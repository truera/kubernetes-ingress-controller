package builder

import (
	"github.com/kong/go-kong/kong"
	"github.com/kong/kubernetes-ingress-controller/v2/internal/dataplane/kongstate"
	"github.com/kong/kubernetes-ingress-controller/v2/internal/util"
)

// KongstateServiceBackendBuilder is a builder for KongstateServiceBackend.
// Primarily used for testing.
// Please note that some methods are not provided yet, as we
// don't need them yet. Feel free to add them as needed.
type KongstateRouteBuilder struct {
	kongstateServiceBackend kongstate.Route
}

func NewKongstateRoute(name string) *KongstateRouteBuilder {
	return &KongstateRouteBuilder{
		kongstateServiceBackend: kongstate.Route{
			Route: kong.Route{
				Name: kong.String(name),
			},
		},
	}
}

func (b *KongstateRouteBuilder) Build() kongstate.Route {
	return b.kongstateServiceBackend
}

// WithHTTPDefaults sets the http and https protocol and preserves host.
func (b *KongstateRouteBuilder) WithHTTPDefaults() *KongstateRouteBuilder {
	return b.WithProtocols("http", "https").WithPreserveHost(true)
}

func (b *KongstateRouteBuilder) WithMethods(methods ...string) *KongstateRouteBuilder {
	for _, method := range methods {
		b.kongstateServiceBackend.Route.Methods = append(b.kongstateServiceBackend.Route.Methods, kong.String(method))
	}
	return b
}

func (b *KongstateRouteBuilder) WithProtocols(protocols ...string) *KongstateRouteBuilder {
	for _, protocol := range protocols {
		b.kongstateServiceBackend.Route.Protocols = append(b.kongstateServiceBackend.Route.Protocols, kong.String(protocol))
	}
	return b
}

func (b *KongstateRouteBuilder) WithHosts(hosts ...string) *KongstateRouteBuilder {
	for _, host := range hosts {
		b.kongstateServiceBackend.Route.Hosts = append(b.kongstateServiceBackend.Route.Hosts, kong.String(host))
	}
	return b
}

func (b *KongstateRouteBuilder) WithPaths(paths ...string) *KongstateRouteBuilder {
	for _, path := range paths {
		b.kongstateServiceBackend.Route.Paths = append(b.kongstateServiceBackend.Route.Paths, kong.String(path))
	}
	return b
}

func (b *KongstateRouteBuilder) WithService(service *kong.Service) *KongstateRouteBuilder {
	b.kongstateServiceBackend.Route.Service = service
	return b
}

func (b *KongstateRouteBuilder) WithIngressObjectInfo(ingress util.K8sObjectInfo) *KongstateRouteBuilder {
	b.kongstateServiceBackend.Ingress = ingress
	return b
}

func (b *KongstateRouteBuilder) WithPreserveHost(preserveHost bool) *KongstateRouteBuilder {
	b.kongstateServiceBackend.Route.PreserveHost = kong.Bool(preserveHost)
	return b
}

func (b *KongstateRouteBuilder) WithStripPath(stripPath bool) *KongstateRouteBuilder {
	b.kongstateServiceBackend.Route.StripPath = kong.Bool(stripPath)
	return b
}

func (b *KongstateRouteBuilder) WithPlugins(plugins ...kong.Plugin) *KongstateRouteBuilder {
	for _, plugin := range plugins {
		b.kongstateServiceBackend.Plugins = append(b.kongstateServiceBackend.Plugins, plugin)
	}
	return b
}

func (b *KongstateRouteBuilder) WithHeader(key string, vals ...string) *KongstateRouteBuilder {
	if b.kongstateServiceBackend.Route.Headers == nil {
		b.kongstateServiceBackend.Route.Headers = make(map[string][]string)
	}
	b.kongstateServiceBackend.Route.Headers[key] = vals
	return b
}
