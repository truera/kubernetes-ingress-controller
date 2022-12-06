package sendconfig

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/kong/deck/diff"
	"github.com/kong/deck/dump"
	"github.com/kong/deck/file"
	"github.com/kong/deck/state"
	deckutils "github.com/kong/deck/utils"
	"github.com/kong/go-kong/kong"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kong/kubernetes-ingress-controller/v2/internal/dataplane/deckgen"
	"github.com/kong/kubernetes-ingress-controller/v2/internal/dataplane/parser"
	"github.com/kong/kubernetes-ingress-controller/v2/internal/metrics"
)

const initialHash = "00000000000000000000000000000000"

// -----------------------------------------------------------------------------
// Sendconfig - Public Functions
// -----------------------------------------------------------------------------

// PerformUpdate writes `targetContent` and `customEntities` to Kong Admin API specified by `kongConfig`.
func PerformUpdate(ctx context.Context,
	log logrus.FieldLogger,
	kongConfig *Kong,
	inMemory bool,
	reverseSync bool,
	skipCACertificates bool,
	targetContent *file.Content,
	selectorTags []string,
	customEntities []byte,
	oldSHA []byte,
	promMetrics *metrics.CtrlFuncMetrics,
) ([]byte, error, []parser.TranslationFailure) {
	newSHA, err := deckgen.GenerateSHA(targetContent, customEntities)
	if err != nil {
		return oldSHA, err, []parser.TranslationFailure{}
	}
	// disable optimization if reverse sync is enabled
	if !reverseSync {
		// use the previous SHA to determine whether or not to perform an update
		if equalSHA(oldSHA, newSHA) {
			if !hasSHAUpdateAlreadyBeenReported(newSHA) {
				log.Debugf("sha %s has been reported", hex.EncodeToString(newSHA))
			}
			// we assume ready as not all Kong versions provide their configuration hash, and their readiness state
			// is always unknown
			ready := true
			status, err := kongConfig.Client.Status(ctx)
			if err != nil {
				log.WithError(err).Error("checking config status failed")
				log.Debug("configuration state unknown, skipping sync to kong")
				return oldSHA, nil, []parser.TranslationFailure{}
			}
			if status.ConfigurationHash == initialHash {
				ready = false
			}
			if ready {
				log.Debug("no configuration change, skipping sync to kong")
				return oldSHA, nil, []parser.TranslationFailure{}
			}
		}
	}

	var metricsProtocol string
	timeStart := time.Now()
	var errParseErr error
	var entityErrors []EntityError
	if inMemory {
		metricsProtocol = metrics.ProtocolDBLess
		err, entityErrors, errParseErr = onUpdateInMemoryMode(ctx, log, targetContent, customEntities, kongConfig)
	} else {
		metricsProtocol = metrics.ProtocolDeck
		err = onUpdateDBMode(ctx, targetContent, kongConfig, selectorTags, skipCACertificates)
	}
	timeEnd := time.Now()

	if err != nil {
		// TODO the collector model doesn't make much sense here since we generate all errors in one go and then toss
		// the instance--no immediate need to retain it, but you could. having it in parser is also a bit awkward, it
		// needs its own package. the translation name is no longer correct either
		failuresCollector, tfcErr := parser.NewTranslationFailuresCollector(log)
		if errParseErr != nil {
			log.WithError(errParseErr).Error("could not parse error response from Kong")
		} else {
			if tfcErr != nil {
				log.WithError(errParseErr).Error("could not parse error response from Kong")
			}
			for _, ee := range entityErrors {
				obj := metav1.PartialObjectMetadata{
					TypeMeta: metav1.TypeMeta{
						Kind:       ee.Kind,
						APIVersion: ee.APIVersion,
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ee.Namespace,
						Name:      ee.Name,
						UID:       types.UID(ee.UID),
					},
				}
				for field, problem := range ee.Problems {
					// TODO this object is incomplete and therefore breaks events a bit. they'll show up in the event
					// list with short fields populated, but won't appear in "describe resource" output. this requires
					// the UID in the reference, so we either need to get the object using the info given or store
					// the UID in tags.
					failuresCollector.PushTranslationFailure(fmt.Sprintf("invalid %s: %s", field, problem), &obj)
					log.Info(fmt.Sprintf("adding failure for %s: %s = %s", ee.Name, field, problem)) // TODO remove
				}
			}
		}

		promMetrics.ConfigPushCount.With(prometheus.Labels{
			metrics.SuccessKey:       metrics.SuccessFalse,
			metrics.ProtocolKey:      metricsProtocol,
			metrics.FailureReasonKey: pushFailureReason(err),
		}).Inc()
		promMetrics.ConfigPushDuration.With(prometheus.Labels{
			metrics.SuccessKey:  metrics.SuccessFalse,
			metrics.ProtocolKey: metricsProtocol,
		}).Observe(float64(timeEnd.Sub(timeStart).Milliseconds()))
		return nil, err, failuresCollector.PopTranslationFailures()
	}

	promMetrics.ConfigPushCount.With(prometheus.Labels{
		metrics.SuccessKey:       metrics.SuccessTrue,
		metrics.ProtocolKey:      metricsProtocol,
		metrics.FailureReasonKey: "",
	}).Inc()
	promMetrics.ConfigPushDuration.With(prometheus.Labels{
		metrics.SuccessKey:  metrics.SuccessTrue,
		metrics.ProtocolKey: metricsProtocol,
	}).Observe(float64(timeEnd.Sub(timeStart).Milliseconds()))
	log.Info("successfully synced configuration to kong.")
	return newSHA, nil, []parser.TranslationFailure{}
}

// -----------------------------------------------------------------------------
// Sendconfig - Private Functions
// -----------------------------------------------------------------------------

func renderConfigWithCustomEntities(log logrus.FieldLogger, state *file.Content,
	customEntitiesJSONBytes []byte,
) ([]byte, error) {
	var kongCoreConfig []byte
	var err error

	kongCoreConfig, err = json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("marshaling kong config into json: %w", err)
	}

	// fast path
	if len(customEntitiesJSONBytes) == 0 {
		return kongCoreConfig, nil
	}

	// slow path
	mergeMap := map[string]interface{}{}
	var result []byte
	var customEntities map[string]interface{}

	// unmarshal core config into the merge map
	err = json.Unmarshal(kongCoreConfig, &mergeMap)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling kong config into map[string]interface{}: %w", err)
	}

	// unmarshal custom entities config into the merge map
	err = json.Unmarshal(customEntitiesJSONBytes, &customEntities)
	if err != nil {
		// do not error out when custom entities are messed up
		log.WithError(err).Error("failed to unmarshal custom entities from secret data")
	} else {
		for k, v := range customEntities {
			if _, exists := mergeMap[k]; !exists {
				mergeMap[k] = v
			}
		}
	}

	// construct the final configuration
	result, err = json.Marshal(mergeMap)
	if err != nil {
		err = fmt.Errorf("marshaling final config into JSON: %w", err)
		return nil, err
	}

	return result, nil
}

func onUpdateInMemoryMode(ctx context.Context,
	log logrus.FieldLogger,
	state *file.Content,
	customEntities []byte,
	kongConfig *Kong,
) (error, []EntityError, error) {
	// Kong will error out if this is set
	state.Info = nil
	// Kong errors out if `null`s are present in `config` of plugins
	deckgen.CleanUpNullsInPluginConfigs(state)
	var parseErr error
	var errorsByEntity []EntityError

	config, err := renderConfigWithCustomEntities(log, state, customEntities)
	if err != nil {
		return fmt.Errorf("constructing kong configuration: %w", err), errorsByEntity, parseErr
	}

	req, err := http.NewRequest("POST", kongConfig.URL+"/config",
		bytes.NewReader(config))
	if err != nil {
		return fmt.Errorf("creating new HTTP request for /config: %w", err), errorsByEntity, parseErr
	}
	req.Header.Add("content-type", "application/json")

	queryString := req.URL.Query()
	queryString.Add("check_hash", "1")

	req.URL.RawQuery = queryString.Encode()

	_, err = kongConfig.Client.Do(ctx, req, nil)
	if err != nil {
		if apiError, ok := err.(*kong.APIError); ok {
			errorsByEntity, parseErr = parseFlatEntityErrors(apiError.Raw(), log)
		}
	}

	return err, errorsByEntity, parseErr
}

type EntityError struct {
	Name       string
	Namespace  string
	Kind       string
	APIVersion string
	UID        string
	Problems   map[string]string
}

type EntityErrorRaw struct {
	Meta     EntityMeta
	Problems map[string]string
}

type EntityMeta struct {
	Name string
	ID   string
	Tags []string
}

func parseEntityErrors(body []byte, log logrus.FieldLogger) ([]EntityError, error) {
	var sections map[string]interface{}
	var entityErrors []EntityError

	// "fields" is the top-level key that contains the expanded JSON sections of validation errors
	err := json.Unmarshal([]byte(gjson.Get(string(body), "fields").Raw), &sections)
	if err != nil {
		return entityErrors, err
	}
	// top-level keys under "fields" are "route", "service", etc.--Kong entity types
	for entity, section := range sections {
		// TODO obsolete with proper exclusion of empty metas, correct?
		if entity == "entity_metadata" {
			continue
		}
		// the value of the entity type keys are arrays of objects with problems
		badEntities, ok := section.([]interface{})
		if !ok {
			return entityErrors, fmt.Errorf("%s section has unrecognized structure", entity)
		}
		ee, err := parseEntityList(badEntities, log)
		if err != nil {
			return entityErrors, err
		}
		entityErrors = append(entityErrors, ee...)
	}
	return entityErrors, nil
}

type ConfigError struct {
	Code   int               `json:"code,omitempty" yaml:"code,omitempty"`
	Fields ConfigErrorFields `json:"fields,omitempty" yaml:"fields,omitempty"`
}

type ConfigErrorFields struct {
	Flattened []FlatEntityError `json:"flattened,omitempty" yaml:"flattened,omitempty"`
	Message   string            `json:"message,omitempty" yaml:"message,omitempty"`
	Name      string            `json:"name,omitempty" yaml:"name,omitempty"`
}

type FlatEntityError struct {
	Name   string           `json:"entity_name,omitempty" yaml:"entity_name,omitempty"`
	ID     string           `json:"entity_id,omitempty" yaml:"entity_id,omitempty"`
	Tags   []string         `json:"entity_tags,omitempty" yaml:"entity_tags,omitempty"`
	Errors []FlatFieldError `json:"errors,omitempty" yaml:"errors,omitempty"`
}

type FlatFieldError struct {
	Field    string   `json:"field,omitempty" yaml:"field,omitempty"`
	Message  string   `json:"message,omitempty" yaml:"message,omitempty"`
	Messages []string `json:"messages,omitempty" yaml:"messages,omitempty"`
}

func parseFlatEntityErrors(body []byte, log logrus.FieldLogger) ([]EntityError, error) {
	var entityErrors []EntityError
	var configError ConfigError
	err := json.Unmarshal(body, &configError)
	if err != nil {
		return entityErrors, fmt.Errorf("could not unmarshal config error: %w", err)
	}
	for _, ee := range configError.Fields.Flattened {
		raw := EntityErrorRaw{
			Meta: EntityMeta{
				Name: ee.Name,
				ID:   ee.ID,
				Tags: ee.Tags,
			},
			Problems: map[string]string{},
		}
		for _, p := range ee.Errors {
			if len(p.Message) > 0 && len(p.Messages) > 0 {
				return entityErrors, fmt.Errorf("entity %s has both single and array errors for field %s", ee.Name, p.Field)
			}
			if len(p.Message) > 0 {
				raw.Problems[p.Field] = p.Message
			}
			if len(p.Messages) > 0 {
				for i, message := range p.Messages {
					if len(message) > 0 { // TODO how are the nulls treated?
						raw.Problems[fmt.Sprintf("%s[%d]", p.Field, i)] = message
					}
				}
			}
		}
		parsed, err := parseEntityError(raw)
		if err != nil {
			return entityErrors, fmt.Errorf("could not parse entity %s: %w", ee.Name, err)
		}
		entityErrors = append(entityErrors, parsed)
	}
	return entityErrors, nil
}

func parseEntityList(entities []interface{}, log logrus.FieldLogger) ([]EntityError, error) {
	var entityErrors []EntityError
	for _, entity := range entities {
		// individual objects contain a key->value map. keys can be one of:
		// - the entity meta object
		// - a field name and its validation failure
		// - a NESTED list of entities with issues, e.g. if you define routes under services in input, you will get
		// a "routes" key here, whose value is another entity
		//
		// these can all apear under the same entity. a service with a bad field and a route with a bad field
		// defined under it will have both a field+validation key AND a nested entity list "routes" key
		objects, ok := entity.(map[string]interface{})
		if !ok {
			if entity == nil {
				continue
			}
			return entityErrors, fmt.Errorf("%s entity has unrecognized structure", entity)
		}
		raw := EntityErrorRaw{
			Meta:     EntityMeta{},
			Problems: map[string]string{},
		}
		for k, v := range objects {
			switch t := v.(type) {
			// fields with issues are "fieldname": "problem description"
			case string:
				raw.Problems[k] = t
			// nested entities are mixed-type arrays, any of metadata, field errors, or nested entities
			case []interface{}:
				ee, err := parseEntityList(t, log)
				if err != nil {
					return entityErrors, fmt.Errorf("TODO couldnt parse sub list: %w", err)
				}
				entityErrors = append(entityErrors, ee...)
			// metadata is a string map
			case map[string]interface{}:
				if k != "entity_metadata" {
					return entityErrors, fmt.Errorf("TODO map not meta")
				}
				metaRaw, err := json.Marshal(t)
				if err != nil {
					return entityErrors, fmt.Errorf("could not marshal %s section metadata: %w", entity, err)
				}
				var meta EntityMeta
				err = json.Unmarshal(metaRaw, &meta)
				if err != nil {
					return entityErrors, fmt.Errorf("could not unmarshal %s section metadata: %w", entity, err)
				}
				raw.Meta = meta
			default:
				return entityErrors, fmt.Errorf("%s section %s key is invalid type", entity, k)
			}
		}
		ee, err := parseEntityError(raw)
		if err != nil {
			log.Info("TODO entity does not have parseable error, maybe only container?")
		}
		entityErrors = append(entityErrors, ee)
	}
	return entityErrors, nil
}

func parseEntityError(raw EntityErrorRaw) (EntityError, error) {
	ee := EntityError{}
	ee.Problems = raw.Problems
	for _, tag := range raw.Meta.Tags {
		// TODO constants
		if strings.HasPrefix(tag, "k8s-name:") {
			ee.Name = strings.TrimPrefix(tag, "k8s-name:")
		}
		if strings.HasPrefix(tag, "k8s-namespace:") {
			ee.Namespace = strings.TrimPrefix(tag, "k8s-namespace:")
		}
		if strings.HasPrefix(tag, "k8s-kind:") {
			ee.Kind = strings.TrimPrefix(tag, "k8s-kind:")
		}
		if strings.HasPrefix(tag, "k8s-apiversion:") {
			ee.APIVersion = strings.TrimPrefix(tag, "k8s-apiversion:")
		}
		if strings.HasPrefix(tag, "k8s-uid:") {
			ee.UID = strings.TrimPrefix(tag, "k8s-uid:")
		}
	}
	if ee.Name == "" {
		return ee, fmt.Errorf("no name")
	}
	if ee.Namespace == "" {
		return ee, fmt.Errorf("no namespace")
	}
	if ee.Kind == "" {
		return ee, fmt.Errorf("no kind")
	}
	// TODO
	//if ee.APIVersion == "" {
	//	return ee, fmt.Errorf("no API version")
	//}
	return ee, nil
}

func onUpdateDBMode(ctx context.Context,
	targetContent *file.Content,
	kongConfig *Kong,
	selectorTags []string,
	skipCACertificates bool,
) error {
	dumpConfig := dump.Config{SelectorTags: selectorTags, SkipCACerts: skipCACertificates}

	cs, err := currentState(ctx, kongConfig, dumpConfig)
	if err != nil {
		return err
	}

	ts, err := targetState(ctx, targetContent, cs, kongConfig, dumpConfig)
	if err != nil {
		return deckConfigConflictError{err}
	}

	syncer, err := diff.NewSyncer(diff.SyncerOpts{
		CurrentState:    cs,
		TargetState:     ts,
		KongClient:      kongConfig.Client,
		SilenceWarnings: true,
	})
	if err != nil {
		return fmt.Errorf("creating a new syncer: %w", err)
	}

	_, errs := syncer.Solve(ctx, kongConfig.Concurrency, false)
	if errs != nil {
		return deckutils.ErrArray{Errors: errs}
	}
	return nil
}

func currentState(ctx context.Context, kongConfig *Kong, dumpConfig dump.Config) (*state.KongState, error) {
	rawState, err := dump.Get(ctx, kongConfig.Client, dumpConfig)
	if err != nil {
		return nil, fmt.Errorf("loading configuration from kong: %w", err)
	}

	return state.Get(rawState)
}

func targetState(ctx context.Context, targetContent *file.Content, currentState *state.KongState, kongConfig *Kong, dumpConfig dump.Config) (*state.KongState, error) {
	rawState, err := file.Get(ctx, targetContent, file.RenderConfig{
		CurrentState: currentState,
		KongVersion:  kongConfig.Version,
	}, dumpConfig, kongConfig.Client)
	if err != nil {
		return nil, err
	}

	return state.Get(rawState)
}

func equalSHA(a, b []byte) bool {
	return reflect.DeepEqual(a, b)
}

var (
	latestReportedSHA []byte
	shaLock           sync.RWMutex
)

// hasSHAUpdateAlreadyBeenReported is a helper function to allow
// sendconfig internals to be aware of the last logged/reported
// update to the Kong Admin API. Given the most recent update SHA,
// it will return true/false whether or not that SHA has previously
// been reported (logged, e.t.c.) so that the caller can make
// decisions (such as staggering or stifling duplicate log lines).
//
// TODO: This is a bit of a hack for now to keep backwards compat,
//
//	but in the future we might configure rolling this into
//	some object/interface which has this functionality as an
//	inherent behavior.
func hasSHAUpdateAlreadyBeenReported(latestUpdateSHA []byte) bool {
	shaLock.Lock()
	defer shaLock.Unlock()
	if equalSHA(latestReportedSHA, latestUpdateSHA) {
		return true
	}
	latestReportedSHA = latestUpdateSHA
	return false
}

// deckConfigConflictError is an error used to wrap deck config conflict errors returned from deck functions
// transforming KongRawState to KongState (e.g. state.Get, dump.Get).
type deckConfigConflictError struct {
	err error
}

func (e deckConfigConflictError) Error() string {
	return e.err.Error()
}

func (e deckConfigConflictError) Is(target error) bool {
	_, ok := target.(deckConfigConflictError)
	return ok
}

func (e deckConfigConflictError) Unwrap() error {
	return e.err
}

// pushFailureReason extracts config push failure reason from an error returned from onUpdateInMemoryMode or onUpdateDBMode.
func pushFailureReason(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return metrics.FailureReasonNetwork
	}

	if isConflictErr(err) {
		return metrics.FailureReasonConflict
	}

	return metrics.FailureReasonOther
}

func isConflictErr(err error) bool {
	var apiErr *kong.APIError
	if errors.As(err, &apiErr) && apiErr.Code() == http.StatusConflict ||
		errors.Is(err, deckConfigConflictError{}) {
		return true
	}

	var deckErrArray deckutils.ErrArray
	if errors.As(err, &deckErrArray) {
		for _, err := range deckErrArray.Errors {
			if isConflictErr(err) {
				return true
			}
		}
	}

	return false
}
