package konnect

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"

	"github.com/kong/kubernetes-ingress-controller/v2/internal/adminapi"
	"github.com/kong/kubernetes-ingress-controller/v2/internal/dataplane"
	"github.com/kong/kubernetes-ingress-controller/v2/internal/util"
)

const (
	MinRefreshNodePeriod     = 30 * time.Second
	DefaultRefreshNodePeriod = 60 * time.Second
)

type GatewayClientsProvider interface {
	GatewayClients() []*adminapi.Client
	SubscribeToGatewayClientsChanges() (<-chan struct{}, bool)
}

// NodeAgent gets the running status of KIC node and controlled kong gateway nodes,
// and update their statuses to konnect.
type NodeAgent struct {
	Hostname string
	Version  string

	Logger logr.Logger

	konnectClient *NodeAPIClient
	refreshPeriod time.Duration

	configStatus           atomic.Uint32
	configStatusSubscriber dataplane.ConfigStatusSubscriber
	gatewayClientsProvider GatewayClientsProvider
}

// NewNodeAgent creates a new node agent.
// hostname and version are hostname and version of KIC.
func NewNodeAgent(
	hostname string,
	version string,
	refreshPeriod time.Duration,
	logger logr.Logger,
	client *NodeAPIClient,
	configStatusSubscriber dataplane.ConfigStatusSubscriber,
	gatewayClientsProvider GatewayClientsProvider,
) *NodeAgent {
	if refreshPeriod < MinRefreshNodePeriod {
		refreshPeriod = MinRefreshNodePeriod
	}
	a := &NodeAgent{
		Hostname: hostname,
		Version:  version,
		Logger: logger.
			WithName("konnect-node").WithValues("runtime_group_id", client.RuntimeGroupID),
		konnectClient:          client,
		refreshPeriod:          refreshPeriod,
		configStatusSubscriber: configStatusSubscriber,
		gatewayClientsProvider: gatewayClientsProvider,
	}
	a.configStatus.Store(uint32(dataplane.ConfigStatusOK))
	return a
}

// Start runs the process of maintaining and uploading of KIC and kong gateway nodes.
func (a *NodeAgent) Start(ctx context.Context) error {
	err := a.updateNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to run initial update of nodes, agent abort")
	}

	go a.updateNodeLoop(ctx)
	go a.subscribeConfigStatus(ctx)
	go a.subscribeToGatewayClientsChanges(ctx)

	// We're waiting here as that's the manager.Runnable interface requirement to block until the context is done.
	<-ctx.Done()
	return nil
}

// NeedLeaderElection implements LeaderElectionRunnable interface to ensure that the node agent is run only when
// the KIC instance is elected a leader.
func (a *NodeAgent) NeedLeaderElection() bool {
	return true
}

// sortNodesByLastPing sort nodes by descending order of last ping time
// so that nodes are sorted by the newest order.
func sortNodesByLastPing(nodes []*NodeItem) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].LastPing > nodes[j].LastPing
	})
}

// subscribeConfigStatus subscribes and updates KIC status on translating and applying configurations to kong gateway.
func (a *NodeAgent) subscribeConfigStatus(ctx context.Context) {
	ch := a.configStatusSubscriber.SubscribeConfigStatus()
	chDone := ctx.Done()

	for {
		select {
		case <-chDone:
			a.Logger.Info("subscribe config status loop stopped", "message", ctx.Err().Error())
			return
		case configStatus := <-ch:
			a.configStatus.Store(uint32(configStatus))
		}
	}
}

func (a *NodeAgent) subscribeToGatewayClientsChanges(ctx context.Context) {
	gatewayClientsChangedCh, changesAreExpected := a.gatewayClientsProvider.SubscribeToGatewayClientsChanges()
	if !changesAreExpected {
		// There are no changes of gateway clients going to happen, we don't have to watch them.
		return
	}

	for {
		select {
		case <-ctx.Done():
			a.Logger.Info("subscribe gateway clients changes loop stopped", "message", ctx.Err().Error())
			return
		case <-gatewayClientsChangedCh:
			if err := a.updateNodes(ctx); err != nil {
				a.Logger.Error(err, "failed to update nodes after gateway clients changed")
			}
		}
	}
}

// updateKICNode updates status of KIC node in konnect.
func (a *NodeAgent) updateKICNode(existingNodes []*NodeItem) error {
	nodesWithSameName := []*NodeItem{}
	for _, node := range existingNodes {
		if node.Type != NodeTypeIngressController {
			continue
		}

		if node.Hostname == a.Hostname {
			// save all nodes with same name as current KIC node, update the latest one and delete others.
			nodesWithSameName = append(nodesWithSameName, node)
		} else {
			// delete the nodes with different name of the current node, since only on KIC node is allowed in the runtime group.
			a.Logger.V(util.DebugLevel).Info("remove outdated KIC node", "node_id", node.ID, "hostname", node.Hostname)
			err := a.konnectClient.DeleteNode(node.ID)
			if err != nil {
				a.Logger.Error(err, "failed to delete KIC node", "node_id", node.ID, "hostname", node.Hostname)
				continue
			}
		}
	}
	// sort nodes by last ping and reserve the latest node.
	sortNodesByLastPing(nodesWithSameName)

	var ingressControllerStatus IngressControllerState
	configStatus := int(a.configStatus.Load())
	switch dataplane.ConfigStatus(configStatus) {
	case dataplane.ConfigStatusOK:
		ingressControllerStatus = IngressControllerStateOperational
	case dataplane.ConfigStatusTranslationErrorHappened:
		ingressControllerStatus = IngressControllerStatePartialConfigFail
	case dataplane.ConfigStatusApplyFailed:
		ingressControllerStatus = IngressControllerStateInoperable
	default:
		ingressControllerStatus = IngressControllerStateUnknown
	}

	// create a new node if there is no existing node with same name as the current KIC node.
	if len(nodesWithSameName) == 0 {
		a.Logger.V(util.DebugLevel).Info("no nodes found for KIC pod, should create one", "hostname", a.Hostname)
		createNodeReq := &CreateNodeRequest{
			Hostname: a.Hostname,
			Version:  a.Version,
			Type:     NodeTypeIngressController,
			LastPing: time.Now().Unix(),
			Status:   string(ingressControllerStatus),
		}
		resp, err := a.konnectClient.CreateNode(createNodeReq)
		if err != nil {
			return fmt.Errorf("failed to create KIC node, hostname %s: %w", a.Hostname, err)
		}
		a.Logger.Info("created KIC node", "node_id", resp.Item.ID, "hostname", a.Hostname)
		return nil
	}

	// update the node with latest last ping time.
	latestNode := nodesWithSameName[0]
	updateNodeReq := &UpdateNodeRequest{
		Hostname: a.Hostname,
		Type:     NodeTypeIngressController,
		Version:  a.Version,
		LastPing: time.Now().Unix(),
		Status:   string(ingressControllerStatus),
	}
	_, err := a.konnectClient.UpdateNode(latestNode.ID, updateNodeReq)
	if err != nil {
		a.Logger.Error(err, "failed to update node for KIC")
		return err
	}
	a.Logger.V(util.DebugLevel).Info("updated last ping time of node for KIC", "node_id", latestNode.ID, "hostname", a.Hostname)

	// treat more nodes with the same name as outdated, and remove them.
	for i := 1; i < len(nodesWithSameName); i++ {
		node := nodesWithSameName[i]
		err := a.konnectClient.DeleteNode(node.ID)
		if err != nil {
			a.Logger.Error(err, "failed to delete outdated KIC node", "node_id", node.ID, "hostname", node.Hostname)
			continue
		}
		a.Logger.V(util.DebugLevel).Info("removed outdated KIC node", "node_id", node.ID, "hostname", node.Hostname)
	}
	return nil
}

// updateGatewayNodes updates status of controlled kong gateway nodes to konnect.
func (a *NodeAgent) updateGatewayNodes(ctx context.Context, existingNodes []*NodeItem) error {
	const proxyNodeType = NodeTypeKongProxy

	gatewayInstances := a.gatewayClientsProvider.GatewayClients()
	gatewayInstanceMap := make(map[string]struct{})

	existingNodeMap := make(map[string][]*NodeItem)
	for _, node := range existingNodes {
		if node.Type == proxyNodeType {
			existingNodeMap[node.Hostname] = append(existingNodeMap[node.Hostname], node)
		}
	}

	for _, gateway := range gatewayInstances {
		hostname, err := gatewayHostname(gateway)
		if err != nil {
			continue
		}
		gatewayInstanceMap[hostname] = struct{}{}
		nodes, ok := existingNodeMap[hostname]

		gatewayVersion, err := gateway.GetKongVersion(ctx)
		if err != nil {
			continue
		}

		// hostname in existing nodes, should create a new node.
		if !ok || len(nodes) == 0 {
			createNodeReq := &CreateNodeRequest{
				Hostname: hostname,
				Version:  gatewayVersion,
				Type:     proxyNodeType,
				LastPing: time.Now().Unix(),
			}
			newNode, err := a.konnectClient.CreateNode(createNodeReq)
			if err != nil {
				a.Logger.Error(err, "failed to create kong gateway node", "hostname", hostname)
			} else {
				a.Logger.Info("created kong gateway node", "hostname", hostname, "node_id", newNode.Item.ID)
			}
			continue
		}

		// sort the nodes by last ping, and only reserve the latest node.
		sortNodesByLastPing(nodes)
		updateNodeReq := &UpdateNodeRequest{
			Hostname: hostname,
			Version:  gatewayVersion,
			Type:     proxyNodeType,
			LastPing: time.Now().Unix(),
		}
		// update the latest node.
		latestNode := nodes[0]
		_, err = a.konnectClient.UpdateNode(latestNode.ID, updateNodeReq)
		if err != nil {
			a.Logger.Error(err, "failed to update kong gateway node", "hostname", hostname, "node_id", latestNode.ID)
			continue
		}
		a.Logger.V(util.DebugLevel).Info("updated kong gateway node", "hostname", hostname, "node_id", latestNode.ID)
		// succeeded to update node, remove the other outdated nodes.
		for i := 1; i < len(nodes); i++ {
			node := nodes[i]
			err := a.konnectClient.DeleteNode(node.ID)
			if err != nil {
				a.Logger.Error(err, "failed to delete outdated kong gateway node", "node_id", node.ID, "hostname", node.Hostname)
				continue
			}
			a.Logger.V(util.DebugLevel).Info("removed outdated kong gateway node", "node_id", node.ID, "hostname", node.Hostname)
		}

	}

	// delete nodes with no corresponding gateway pod.
	for hostname, nodes := range existingNodeMap {
		if _, ok := gatewayInstanceMap[hostname]; !ok {
			for _, node := range nodes {
				err := a.konnectClient.DeleteNode(node.ID)
				if err != nil {
					a.Logger.Error(err, "failed to delete outdated kong gateway node", "node_id", node.ID, "hostname", node.Hostname)
					continue
				}
				a.Logger.V(util.DebugLevel).Info("removed outdated kong gateway node", "node_id", node.ID, "hostname", node.Hostname)
			}
		}
	}

	return nil
}

// updateNodes updates current status of KIC and controlled kong gateway nodes.
func (a *NodeAgent) updateNodes(ctx context.Context) error {
	existingNodes, err := a.konnectClient.ListAllNodes()
	if err != nil {
		return fmt.Errorf("failed to list existing nodes: %w", err)
	}

	err = a.updateKICNode(existingNodes)
	if err != nil {
		// REVIEW: not return here and continue to update kong gateway nodes?
		return fmt.Errorf("failed to update KIC node: %w", err)
	}

	err = a.updateGatewayNodes(ctx, existingNodes)
	if err != nil {
		return fmt.Errorf("failed to update controlled kong gateway nodes: %w", err)
	}
	return nil
}

// updateNodeLoop runs the loop to update status of KIC and kong gateway nods periodically.
func (a *NodeAgent) updateNodeLoop(ctx context.Context) {
	ticker := time.NewTicker(a.refreshPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			a.Logger.Info("update node loop stopped", "message", err.Error())
			return
		case <-ticker.C:
			err := a.updateNodes(ctx)
			if err != nil {
				a.Logger.Error(err, "failed to update nodes")
			}
		}
	}
}

func gatewayHostname(c *adminapi.Client) (string, error) {
	if podNN, ok := c.PodReference(); ok {
		return podNN.String(), nil
	}

	rootURL := c.BaseRootURL()
	u, err := url.Parse(rootURL)
	if err != nil {
		// this should never happen
		return "", fmt.Errorf("failed to parse client's root url: %w", err)
	}

	// use "gateway_address" as hostname of konnect node.
	// REVIEW: trim ports in addresses? like 127.0.0.1:8444 -> gateway_127.0.0.1
	return "gateway" + "_" + u.Host, nil
}
