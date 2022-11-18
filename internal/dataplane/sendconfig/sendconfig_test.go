package sendconfig

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"testing"

	"github.com/kong/deck/file"
	deckutils "github.com/kong/deck/utils"
	"github.com/kong/go-kong/kong"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kong/kubernetes-ingress-controller/v2/internal/metrics"
)

func TestParseEntityErrors(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		want    []EntityError
		wantErr bool
	}{
		{
			name: "dunno",
			want: []EntityError{
				{
					Name:      "scallion",
					Namespace: "default",
					Kind:      "Ingress",
					Problems: map[string]string{
						"methods": "cannot set 'methods' when 'protocols' is 'grpc' or 'grpcs'",
					},
				},
				{
					Name:      "turnip",
					Namespace: "default",
					Kind:      "Ingress",
					Problems: map[string]string{
						"strip_path": "cannot set 'strip_path' when 'protocols' is 'grpc' or 'grpcs'",
					},
				},
				{
					Name:      "radish",
					Namespace: "default",
					Kind:      "Service",
					Problems: map[string]string{
						"read_timeout": "expected an integer",
					},
				},
			},
			wantErr: false,
			body: []byte(`{
    "code": 14,
    "fields": {
        "entity_metadata": {},
        "routes": [
            null,
            {
                "entity_metadata": {
                    "id": "aeacf6da-6954-45e8-a2a3-470fabcb738f",
                    "tags": [
						"k8s-name:scallion",
						"k8s-namespace:default",
						"k8s-kind:Ingress"
                    ]
                },
                "methods": "cannot set 'methods' when 'protocols' is 'grpc' or 'grpcs'"
            },
            {
                "entity_metadata": {
                    "id": "d0da7fd2-9f5c-5a9d-81c8-e6463ce7b068",
                    "name": "default.demo.01",
                    "tags": [
						"k8s-name:turnip",
						"k8s-namespace:default",
						"k8s-kind:Ingress"
                    ]
                },
                "strip_path": "cannot set 'strip_path' when 'protocols' is 'grpc' or 'grpcs'"
            }
        ],
        "services": [
            {
                "entity_metadata": {
                    "id": "b8aa692c-6d8d-580e-a767-a7dbc1f58344",
                    "name": "default.echo.pnum-80",
                    "tags": [
						"k8s-name:radish",
						"k8s-namespace:default",
						"k8s-kind:Service"
                    ]
                },
                "read_timeout": "expected an integer"
            }
        ]
    },
    "message": "declarative config is invalid: {entity_metadata={},routes={[2]={entity_metadata={id=\"aeacf6da-6954-45e8-a2a3-470fabcb738f\",tags={\"big-bad-tag\"}},methods=\"cannot set 'methods' when 'protocols' is 'grpc' or 'grpcs'\"},[3]={entity_metadata={id=\"d0da7fd2-9f5c-5a9d-81c8-e6463ce7b068\",name=\"default.demo.01\"},strip_path=\"cannot set 'strip_path' when 'protocols' is 'grpc' or 'grpcs'\"}},services={{entity_metadata={id=\"b8aa692c-6d8d-580e-a767-a7dbc1f58344\",name=\"default.echo.pnum-80\"},read_timeout=\"expected an integer\"}}}",
    "name": "invalid declarative configuration"
}
`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEntityErrors(bytes.NewBuffer(tt.body))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEntityErrors() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, got, tt.want)
		})
	}
}

func TestRenderConfigWithCustomEntities(t *testing.T) {
	type args struct {
		state                   *file.Content
		customEntitiesJSONBytes []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "basic sanity test for fast-path",
			args: args{
				state: &file.Content{
					FormatVersion: "1.1",
					Services: []file.FService{
						{
							Service: kong.Service{
								Name: kong.String("foo"),
								Host: kong.String("example.com"),
							},
						},
					},
				},
				customEntitiesJSONBytes: nil,
			},
			want:    []byte(`{"_format_version":"1.1","services":[{"host":"example.com","name":"foo"}]}`),
			wantErr: false,
		},
		{
			name: "does not break with random bytes in the custom entities",
			args: args{
				state: &file.Content{
					FormatVersion: "1.1",
					Services: []file.FService{
						{
							Service: kong.Service{
								Name: kong.String("foo"),
								Host: kong.String("example.com"),
							},
						},
					},
				},
				customEntitiesJSONBytes: []byte("random-bytes"),
			},
			want:    []byte(`{"_format_version":"1.1","services":[{"host":"example.com","name":"foo"}]}`),
			wantErr: false,
		},
		{
			name: "custom entities cannot hijack core entities",
			args: args{
				state: &file.Content{
					FormatVersion: "1.1",
					Services: []file.FService{
						{
							Service: kong.Service{
								Name: kong.String("foo"),
								Host: kong.String("example.com"),
							},
						},
					},
				},
				customEntitiesJSONBytes: []byte(`{"services":[{"host":"rogue.example.com","name":"rogue"}]}`),
			},
			want:    []byte(`{"_format_version":"1.1","services":[{"host":"example.com","name":"foo"}]}`),
			wantErr: false,
		},
		{
			name: "custom entities can be populated",
			args: args{
				state: &file.Content{
					FormatVersion: "1.1",
					Services: []file.FService{
						{
							Service: kong.Service{
								Name: kong.String("foo"),
								Host: kong.String("example.com"),
							},
						},
					},
				},
				customEntitiesJSONBytes: []byte(`{"my-custom-dao-name":` +
					`[{"name":"custom1","key1":"value1"},` +
					`{"name":"custom2","dumb":"test-value","boring-test-value-name":"really?"}]}`),
			},
			want: []byte(`{"_format_version":"1.1",` +
				`"my-custom-dao-name":[{"key1":"value1","name":"custom1"},` +
				`{"boring-test-value-name":"really?","dumb":"test-value","name":"custom2"}]` +
				`,"services":[{"host":"example.com","name":"foo"}]}`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderConfigWithCustomEntities(logrus.New(), tt.args.state, tt.args.customEntitiesJSONBytes)
			if (err != nil) != tt.wantErr {
				t.Errorf("renderConfigWithCustomEntities() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("renderConfigWithCustomEntities() = %v, want %v",
					string(got), string(tt.want))
			}
		})
	}
}

func TestUpdateReportingUtilities(t *testing.T) {
	assert.False(t, hasSHAUpdateAlreadyBeenReported([]byte("fake-sha")))
	assert.True(t, hasSHAUpdateAlreadyBeenReported([]byte("fake-sha")))
	assert.False(t, hasSHAUpdateAlreadyBeenReported([]byte("another-fake-sha")))
	assert.True(t, hasSHAUpdateAlreadyBeenReported([]byte("another-fake-sha")))
	assert.False(t, hasSHAUpdateAlreadyBeenReported([]byte("yet-another-fake-sha")))
	assert.True(t, hasSHAUpdateAlreadyBeenReported([]byte("yet-another-fake-sha")))
	assert.True(t, hasSHAUpdateAlreadyBeenReported([]byte("yet-another-fake-sha")))
	assert.True(t, hasSHAUpdateAlreadyBeenReported([]byte("yet-another-fake-sha")))
}

func TestPushFailureReason(t *testing.T) {
	apiConflictErr := kong.NewAPIError(http.StatusConflict, "conflict api error")
	networkErr := net.UnknownNetworkError("network error")
	genericError := errors.New("generic error")

	testCases := []struct {
		name           string
		err            error
		expectedReason string
	}{
		{
			name:           "generic_error",
			err:            genericError,
			expectedReason: metrics.FailureReasonOther,
		},
		{
			name:           "api_conflict_error",
			err:            apiConflictErr,
			expectedReason: metrics.FailureReasonConflict,
		},
		{
			name:           "api_conflict_error_wrapped",
			err:            fmt.Errorf("wrapped conflict api err: %w", apiConflictErr),
			expectedReason: metrics.FailureReasonConflict,
		},
		{
			name:           "deck_config_conflict_error_empty",
			err:            deckConfigConflictError{},
			expectedReason: metrics.FailureReasonConflict,
		},
		{
			name:           "deck_config_conflict_error_with_generic_error",
			err:            deckConfigConflictError{genericError},
			expectedReason: metrics.FailureReasonConflict,
		},
		{
			name:           "deck_err_array_with_api_conflict_error",
			err:            deckutils.ErrArray{Errors: []error{apiConflictErr}},
			expectedReason: metrics.FailureReasonConflict,
		},
		{
			name:           "wrapped_deck_err_array_with_api_conflict_error",
			err:            fmt.Errorf("wrapped: %w", deckutils.ErrArray{Errors: []error{apiConflictErr}}),
			expectedReason: metrics.FailureReasonConflict,
		},
		{
			name:           "deck_err_array_with_generic_error",
			err:            deckutils.ErrArray{Errors: []error{genericError}},
			expectedReason: metrics.FailureReasonOther,
		},
		{
			name:           "deck_err_array_empty",
			err:            deckutils.ErrArray{Errors: []error{genericError}},
			expectedReason: metrics.FailureReasonOther,
		},
		{
			name:           "network_error",
			err:            networkErr,
			expectedReason: metrics.FailureReasonNetwork,
		},
		{
			name:           "network_error_wrapped_in_deck_config_conflict_error",
			err:            deckConfigConflictError{networkErr},
			expectedReason: metrics.FailureReasonNetwork,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reason := pushFailureReason(tc.err)
			require.Equal(t, tc.expectedReason, reason)
		})
	}
}
