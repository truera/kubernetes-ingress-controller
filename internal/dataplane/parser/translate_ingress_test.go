package parser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kong/kubernetes-ingress-controller/v2/internal/annotations"
	"github.com/kong/kubernetes-ingress-controller/v2/internal/dataplane/kongstate"
	"github.com/kong/kubernetes-ingress-controller/v2/internal/dataplane/parser/translators"
	"github.com/kong/kubernetes-ingress-controller/v2/internal/store"
)

func TestFromIngressV1(t *testing.T) {
	ingressList := []*netv1.Ingress{
		// 0
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "foo-namespace",
				Annotations: map[string]string{
					annotations.IngressClassKey: annotations.DefaultIngressClass,
				},
			},
			Spec: netv1.IngressSpec{
				Rules: []netv1.IngressRule{
					{
						Host: "example.com",
						IngressRuleValue: netv1.IngressRuleValue{
							HTTP: &netv1.HTTPIngressRuleValue{
								Paths: []netv1.HTTPIngressPath{
									{
										Path: "/",
										Backend: netv1.IngressBackend{
											Service: &netv1.IngressServiceBackend{
												Name: "foo-svc",
												Port: netv1.ServiceBackendPort{Number: 80},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		// 1
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ing-with-tls",
				Namespace: "bar-namespace",
				Annotations: map[string]string{
					annotations.IngressClassKey: annotations.DefaultIngressClass,
				},
			},
			Spec: netv1.IngressSpec{
				TLS: []netv1.IngressTLS{
					{
						Hosts: []string{
							"1.example.com",
							"2.example.com",
						},
						SecretName: "sooper-secret",
					},
					{
						Hosts: []string{
							"3.example.com",
							"4.example.com",
						},
						SecretName: "sooper-secret2",
					},
				},
				Rules: []netv1.IngressRule{
					{
						Host: "example.com",
						IngressRuleValue: netv1.IngressRuleValue{
							HTTP: &netv1.HTTPIngressRuleValue{
								Paths: []netv1.HTTPIngressPath{
									{
										Path: "/",
										Backend: netv1.IngressBackend{
											Service: &netv1.IngressServiceBackend{
												Name: "foo-svc",
												Port: netv1.ServiceBackendPort{Number: 80},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		// 2
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ing-with-default-backend",
				Namespace: "bar-namespace",
				Annotations: map[string]string{
					annotations.IngressClassKey: annotations.DefaultIngressClass,
				},
			},
			Spec: netv1.IngressSpec{
				DefaultBackend: &netv1.IngressBackend{
					Service: &netv1.IngressServiceBackend{
						Name: "default-svc",
						Port: netv1.ServiceBackendPort{Number: 80},
					},
				},
			},
		},
		// 3
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "foo-namespace",
				Annotations: map[string]string{
					annotations.IngressClassKey: annotations.DefaultIngressClass,
				},
			},
			Spec: netv1.IngressSpec{
				Rules: []netv1.IngressRule{
					{
						Host: "example.com",
						IngressRuleValue: netv1.IngressRuleValue{
							HTTP: &netv1.HTTPIngressRuleValue{
								Paths: []netv1.HTTPIngressPath{
									{
										Path: "/.well-known/acme-challenge/yolo",
										Backend: netv1.IngressBackend{
											Service: &netv1.IngressServiceBackend{
												Name: "cert-manager-solver-pod",
												Port: netv1.ServiceBackendPort{Number: 80},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		// 4
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "foo-namespace",
				Annotations: map[string]string{
					annotations.IngressClassKey: annotations.DefaultIngressClass,
				},
			},
			Spec: netv1.IngressSpec{
				Rules: []netv1.IngressRule{
					{
						Host: "example.com",
						IngressRuleValue: netv1.IngressRuleValue{
							HTTP: &netv1.HTTPIngressRuleValue{
								Paths: []netv1.HTTPIngressPath{
									{
										Backend: netv1.IngressBackend{
											Service: &netv1.IngressServiceBackend{
												Name: "foo-svc",
												Port: netv1.ServiceBackendPort{Number: 80},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		// 5
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "baz",
				Namespace: "foo-namespace",
				Annotations: map[string]string{
					annotations.IngressClassKey: annotations.DefaultIngressClass,
				},
			},
			Spec: netv1.IngressSpec{
				Rules: []netv1.IngressRule{
					{
						Host:             "example.com",
						IngressRuleValue: netv1.IngressRuleValue{},
					},
				},
			},
		},
		// 6
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "foo-namespace",
				Annotations: map[string]string{
					annotations.IngressClassKey: annotations.DefaultIngressClass,
				},
			},
			Spec: netv1.IngressSpec{
				Rules: []netv1.IngressRule{
					{
						Host: "example.com",
						IngressRuleValue: netv1.IngressRuleValue{
							HTTP: &netv1.HTTPIngressRuleValue{
								Paths: []netv1.HTTPIngressPath{
									{
										Backend: netv1.IngressBackend{
											Service: &netv1.IngressServiceBackend{
												Name: "foo-svc",
												Port: netv1.ServiceBackendPort{Number: 80},
											},
										},
									},
								},
							},
						},
					},
					{
						Host: "example.net",
						IngressRuleValue: netv1.IngressRuleValue{
							HTTP: &netv1.HTTPIngressRuleValue{
								Paths: []netv1.HTTPIngressPath{
									{
										Backend: netv1.IngressBackend{
											Service: &netv1.IngressServiceBackend{
												Name: "foo-svc",
												Port: netv1.ServiceBackendPort{Number: 8000},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		// 7
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid-path",
				Namespace: "foo-namespace",
				Annotations: map[string]string{
					annotations.IngressClassKey: annotations.DefaultIngressClass,
				},
			},
			Spec: netv1.IngressSpec{
				Rules: []netv1.IngressRule{
					{
						Host: "example.com",
						IngressRuleValue: netv1.IngressRuleValue{
							HTTP: &netv1.HTTPIngressRuleValue{
								Paths: []netv1.HTTPIngressPath{
									{
										Path: "/foo//bar",
										Backend: netv1.IngressBackend{
											Service: &netv1.IngressServiceBackend{
												Name: "foo-svc",
												Port: netv1.ServiceBackendPort{Number: 80},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		// 8
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "foo-namespace",
				Annotations: map[string]string{
					annotations.IngressClassKey: annotations.DefaultIngressClass,
				},
			},
			Spec: netv1.IngressSpec{
				Rules: []netv1.IngressRule{
					{
						Host: "example.com",
						IngressRuleValue: netv1.IngressRuleValue{
							HTTP: &netv1.HTTPIngressRuleValue{
								Paths: []netv1.HTTPIngressPath{
									{
										Path: "/",
										Backend: netv1.IngressBackend{
											Service: &netv1.IngressServiceBackend{
												Name: "foo-svc",
												Port: netv1.ServiceBackendPort{Name: "http"},
											},
										},
									},
									{
										Path: "/ws",
										Backend: netv1.IngressBackend{
											Service: &netv1.IngressServiceBackend{
												Name: "foo-svc",
												Port: netv1.ServiceBackendPort{Name: "ws"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		// 9
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "regex-prefix",
				Namespace: "foo-namespace",
				Annotations: map[string]string{
					annotations.IngressClassKey: annotations.DefaultIngressClass,
				},
			},
			Spec: netv1.IngressSpec{
				Rules: []netv1.IngressRule{
					{
						Host: "example.com",
						IngressRuleValue: netv1.IngressRuleValue{
							HTTP: &netv1.HTTPIngressRuleValue{
								Paths: []netv1.HTTPIngressPath{
									{
										Path: translators.ControllerPathRegexPrefix + "/foo/\\d{3}",
										Backend: netv1.IngressBackend{
											Service: &netv1.IngressServiceBackend{
												Name: "foo-svc",
												Port: netv1.ServiceBackendPort{Number: 80},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		// 10
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo-2",
				Namespace: "foo-namespace",
				Annotations: map[string]string{
					annotations.IngressClassKey: annotations.DefaultIngressClass,
				},
			},
			Spec: netv1.IngressSpec{
				Rules: []netv1.IngressRule{
					{
						Host: "example.com",
						IngressRuleValue: netv1.IngressRuleValue{
							HTTP: &netv1.HTTPIngressRuleValue{
								Paths: []netv1.HTTPIngressPath{
									{
										Path: "/",
										Backend: netv1.IngressBackend{
											Service: &netv1.IngressServiceBackend{
												Name: "foo-svc",
												Port: netv1.ServiceBackendPort{Number: 80},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	t.Run("CombinedRoutes=off", func(t *testing.T) {
		t.Run("no ingress returns empty info", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{},
			})
			require.NoError(t, err)
			p := mustNewParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Equal(t, ingressRules{
				ServiceNameToServices: make(map[string]kongstate.Service),
				ServiceNameToParent:   make(map[string]client.Object),
				SecretNameToSNIs:      newSecretNameToSNIs(),
			}, parsedInfo)
		})
		t.Run("simple ingress rule is parsed", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[0],
				},
			})
			require.NoError(t, err)
			p := mustNewParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Len(t, parsedInfo.ServiceNameToServices, 1)
			assert.Equal(t, "foo-svc.foo-namespace.80.svc", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"].Host)
			assert.Equal(t, 80, *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"].Port)

			assert.Equal(t, "/", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"].Routes[0].Paths[0])
			assert.Equal(t, "example.com", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"].Routes[0].Hosts[0])
		})
		t.Run("ingress rule with default backend", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[0],
					ingressList[2],
				},
			})
			require.NoError(t, err)
			p := mustNewParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Len(t, parsedInfo.ServiceNameToServices, 2)
			assert.Equal(t, "foo-svc.foo-namespace.80.svc", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"].Host)
			assert.Equal(t, 80, *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"].Port)

			assert.Equal(t, "/", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"].Routes[0].Paths[0])
			assert.Equal(t, "example.com", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"].Routes[0].Hosts[0])

			assert.Len(t, parsedInfo.ServiceNameToServices["bar-namespace.default-svc.80"].Routes, 1)
			assert.Equal(t, "/", *parsedInfo.ServiceNameToServices["bar-namespace.default-svc.80"].Routes[0].Paths[0])
			assert.Empty(t, parsedInfo.ServiceNameToServices["bar-namespace.default-svc.80"].Routes[0].Hosts)
		})
		t.Run("ingress rule with TLS", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[1],
				},
			})
			require.NoError(t, err)
			p := mustNewParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Len(t, parsedInfo.SecretNameToSNIs.Hosts("bar-namespace/sooper-secret"), 2)
			assert.Len(t, parsedInfo.SecretNameToSNIs.Hosts("bar-namespace/sooper-secret2"), 2)
		})
		t.Run("ingress rule with ACME like path has strip_path set to false", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[3],
				},
			})
			require.NoError(t, err)
			p := mustNewParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Len(t, parsedInfo.ServiceNameToServices, 1)
			assert.Equal(t, "cert-manager-solver-pod.foo-namespace.80.svc",
				*parsedInfo.ServiceNameToServices["foo-namespace.cert-manager-solver-pod.pnum-80"].Host)
			assert.Equal(t, 80, *parsedInfo.ServiceNameToServices["foo-namespace.cert-manager-solver-pod.pnum-80"].Port)

			assert.Equal(t, "/.well-known/acme-challenge/yolo",
				*parsedInfo.ServiceNameToServices["foo-namespace.cert-manager-solver-pod.pnum-80"].Routes[0].Paths[0])
			assert.Equal(t, "example.com",
				*parsedInfo.ServiceNameToServices["foo-namespace.cert-manager-solver-pod.pnum-80"].Routes[0].Hosts[0])
			assert.False(t, *parsedInfo.ServiceNameToServices["foo-namespace.cert-manager-solver-pod.pnum-80"].Routes[0].StripPath)
		})
		t.Run("ingress with empty path is correctly parsed", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[4],
				},
			})
			require.NoError(t, err)
			p := mustNewParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Equal(t, "/", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"].Routes[0].Paths[0])
			assert.Equal(t, "example.com", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"].Routes[0].Hosts[0])
		})
		t.Run("empty Ingress rule doesn't cause a panic", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[5],
				},
			})
			require.NoError(t, err)
			p := mustNewParser(t, store)

			assert.NotPanics(t, func() {
				p.ingressRulesFromIngressV1()
			})
		})
		t.Run("Ingress rules with multiple ports for one Service use separate hostnames for each port", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[6],
				},
			})
			require.NoError(t, err)
			p := mustNewParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Equal(t, "foo-svc.foo-namespace.80.svc",
				*parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"].Host)
			assert.Equal(t, "foo-svc.foo-namespace.8000.svc",
				*parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-8000"].Host)
		})
		t.Run("Ingress rule with ports defined by name", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[9],
				},
			})
			require.NoError(t, err)
			p := mustNewParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			_, ok := parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"]
			assert.True(t, ok)
		})
		t.Run("Ingress rule with regex prefixed path creates route with Kong regex prefix", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[9],
				},
			})
			require.NoError(t, err)
			p := mustNewParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Equal(t, translators.KongPathRegexPrefix+"/foo/\\d{3}", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"].Routes[0].Paths[0])
		})
	})

	t.Run("CombinedRoutes=on", func(t *testing.T) {
		setupParser := func(t *testing.T, store store.Storer) *Parser {
			p := mustNewParser(t, store)
			p.featureFlags.CombinedServiceRoutes = true
			return p
		}

		t.Run("no ingress returns empty info", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Equal(t, ingressRules{
				ServiceNameToServices: make(map[string]kongstate.Service),
				ServiceNameToParent:   make(map[string]client.Object),
				SecretNameToSNIs:      newSecretNameToSNIs(),
			}, parsedInfo)
		})
		t.Run("simple ingress rule is parsed", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[0],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Len(t, parsedInfo.ServiceNameToServices, 1)
			assert.Equal(t, "foo-svc.foo-namespace.80.svc", *parsedInfo.ServiceNameToServices["foo-namespace.foo.foo-svc.80"].Host)
			assert.Equal(t, 80, *parsedInfo.ServiceNameToServices["foo-namespace.foo.foo-svc.80"].Port)

			assert.Equal(t, "/", *parsedInfo.ServiceNameToServices["foo-namespace.foo.foo-svc.80"].Routes[0].Paths[0])
			assert.Equal(t, "example.com", *parsedInfo.ServiceNameToServices["foo-namespace.foo.foo-svc.80"].Routes[0].Hosts[0])
		})
		t.Run("ingress rule with default backend", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[0],
					ingressList[2],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Len(t, parsedInfo.ServiceNameToServices, 2)
			assert.Equal(t, "foo-svc.foo-namespace.80.svc", *parsedInfo.ServiceNameToServices["foo-namespace.foo.foo-svc.80"].Host)
			assert.Equal(t, 80, *parsedInfo.ServiceNameToServices["foo-namespace.foo.foo-svc.80"].Port)

			assert.Equal(t, "/", *parsedInfo.ServiceNameToServices["foo-namespace.foo.foo-svc.80"].Routes[0].Paths[0])
			assert.Equal(t, "example.com", *parsedInfo.ServiceNameToServices["foo-namespace.foo.foo-svc.80"].Routes[0].Hosts[0])

			assert.Len(t, parsedInfo.ServiceNameToServices["bar-namespace.default-svc.80"].Routes, 1)
			assert.Equal(t, "/", *parsedInfo.ServiceNameToServices["bar-namespace.default-svc.80"].Routes[0].Paths[0])
			assert.Empty(t, parsedInfo.ServiceNameToServices["bar-namespace.default-svc.80"].Routes[0].Hosts)
		})
		t.Run("ingress rule with TLS", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[1],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Len(t, parsedInfo.SecretNameToSNIs.Hosts("bar-namespace/sooper-secret"), 2)
			assert.Len(t, parsedInfo.SecretNameToSNIs.Hosts("bar-namespace/sooper-secret2"), 2)
		})
		t.Run("ingress rule with ACME like path has strip_path set to false", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[3],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Len(t, parsedInfo.ServiceNameToServices, 1)
			assert.Equal(t, "cert-manager-solver-pod.foo-namespace.80.svc",
				*parsedInfo.ServiceNameToServices["foo-namespace.foo.cert-manager-solver-pod.80"].Host)
			assert.Equal(t, 80, *parsedInfo.ServiceNameToServices["foo-namespace.foo.cert-manager-solver-pod.80"].Port)

			assert.Equal(t, "/.well-known/acme-challenge/yolo",
				*parsedInfo.ServiceNameToServices["foo-namespace.foo.cert-manager-solver-pod.80"].Routes[0].Paths[0])
			assert.Equal(t, "example.com",
				*parsedInfo.ServiceNameToServices["foo-namespace.foo.cert-manager-solver-pod.80"].Routes[0].Hosts[0])
			assert.False(t, *parsedInfo.ServiceNameToServices["foo-namespace.foo.cert-manager-solver-pod.80"].Routes[0].StripPath)
		})
		t.Run("ingress with empty path is correctly parsed", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[4],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Equal(t, "/", *parsedInfo.ServiceNameToServices["foo-namespace.foo.foo-svc.80"].Routes[0].Paths[0])
			assert.Equal(t, "example.com", *parsedInfo.ServiceNameToServices["foo-namespace.foo.foo-svc.80"].Routes[0].Hosts[0])
		})
		t.Run("empty Ingress rule doesn't cause a panic", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[5],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			assert.NotPanics(t, func() {
				p.ingressRulesFromIngressV1()
			})
		})
		t.Run("Ingress rules with multiple ports for one Service use separate hostnames for each port", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[6],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Equal(t, "foo-svc.foo-namespace.80.svc",
				*parsedInfo.ServiceNameToServices["foo-namespace.foo.foo-svc.80"].Host)
			assert.Equal(t, "foo-svc.foo-namespace.8000.svc",
				*parsedInfo.ServiceNameToServices["foo-namespace.foo.foo-svc.8000"].Host)
		})
		t.Run("Ingress rule with ports defined by name", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[9],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			_, ok := parsedInfo.ServiceNameToServices["foo-namespace.regex-prefix.foo-svc.80"]
			assert.True(t, ok)
		})
		t.Run("Ingress rule with regex prefixed path creates route with Kong regex prefix", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[9],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Equal(t, translators.KongPathRegexPrefix+"/foo/\\d{3}", *parsedInfo.ServiceNameToServices["foo-namespace.regex-prefix.foo-svc.80"].Routes[0].Paths[0])
		})
		t.Run("single service in multiple ingresses generates multiple kong services", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[0],
					ingressList[10],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			require.Len(t, parsedInfo.ServiceNameToServices, 2)
			assert.Equal(t, *parsedInfo.ServiceNameToServices["foo-namespace.foo.foo-svc.80"].Host, "foo-svc.foo-namespace.80.svc")
			assert.Equal(t, *parsedInfo.ServiceNameToServices["foo-namespace.foo-2.foo-svc.80"].Host, "foo-svc.foo-namespace.80.svc")
		})
	})

	t.Run("CombinedRoutes=on && CombinedServices=on", func(t *testing.T) {
		setupParser := func(t *testing.T, store store.Storer) *Parser {
			p := mustNewParser(t, store)
			p.featureFlags.CombinedServiceRoutes = true
			p.featureFlags.CombinedServices = true
			return p
		}

		t.Run("single service in multiple ingresses generates single kong service", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[0],
					ingressList[10],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			require.Len(t, parsedInfo.ServiceNameToServices, 1)
			kongSvc, ok := parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"]
			require.True(t, ok)
			assert.Equal(t, *kongSvc.Host, "foo-svc.foo-namespace.80.svc")
			assert.Len(t, kongSvc.Routes, 2)
		})

		t.Run("no ingress returns empty info", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Equal(t, ingressRules{
				ServiceNameToServices: make(map[string]kongstate.Service),
				ServiceNameToParent:   make(map[string]client.Object),
				SecretNameToSNIs:      newSecretNameToSNIs(),
			}, parsedInfo)
		})

		t.Run("simple ingress rule is parsed", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[0],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Len(t, parsedInfo.ServiceNameToServices, 1)
			assert.Equal(t, "foo-svc.foo-namespace.80.svc", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"].Host)
			assert.Equal(t, 80, *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"].Port)

			assert.Equal(t, "/", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"].Routes[0].Paths[0])
			assert.Equal(t, "example.com", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"].Routes[0].Hosts[0])
		})

		t.Run("ingress rule with default backend", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[0],
					ingressList[2],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Len(t, parsedInfo.ServiceNameToServices, 2)
			assert.Equal(t, "foo-svc.foo-namespace.80.svc", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"].Host)
			assert.Equal(t, 80, *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"].Port)

			assert.Equal(t, "/", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"].Routes[0].Paths[0])
			assert.Equal(t, "example.com", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"].Routes[0].Hosts[0])

			assert.Len(t, parsedInfo.ServiceNameToServices["bar-namespace.default-svc.80"].Routes, 1)
			assert.Equal(t, "/", *parsedInfo.ServiceNameToServices["bar-namespace.default-svc.80"].Routes[0].Paths[0])
			assert.Empty(t, parsedInfo.ServiceNameToServices["bar-namespace.default-svc.80"].Routes[0].Hosts)
		})

		t.Run("ingress rule with TLS", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[1],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Len(t, parsedInfo.SecretNameToSNIs.Hosts("bar-namespace/sooper-secret"), 2)
			assert.Len(t, parsedInfo.SecretNameToSNIs.Hosts("bar-namespace/sooper-secret2"), 2)
		})

		t.Run("ingress rule with ACME like path has strip_path set to false", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[3],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Len(t, parsedInfo.ServiceNameToServices, 1)
			assert.Equal(t, "cert-manager-solver-pod.foo-namespace.80.svc",
				*parsedInfo.ServiceNameToServices["foo-namespace.cert-manager-solver-pod.80"].Host)
			assert.Equal(t, 80, *parsedInfo.ServiceNameToServices["foo-namespace.cert-manager-solver-pod.80"].Port)

			assert.Equal(t, "/.well-known/acme-challenge/yolo",
				*parsedInfo.ServiceNameToServices["foo-namespace.cert-manager-solver-pod.80"].Routes[0].Paths[0])
			assert.Equal(t, "example.com",
				*parsedInfo.ServiceNameToServices["foo-namespace.cert-manager-solver-pod.80"].Routes[0].Hosts[0])
			assert.False(t, *parsedInfo.ServiceNameToServices["foo-namespace.cert-manager-solver-pod.80"].Routes[0].StripPath)
		})
		t.Run("ingress with empty path is correctly parsed", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[4],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Equal(t, "/", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"].Routes[0].Paths[0])
			assert.Equal(t, "example.com", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"].Routes[0].Hosts[0])
		})

		t.Run("empty Ingress rule doesn't cause a panic", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[5],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			assert.NotPanics(t, func() {
				p.ingressRulesFromIngressV1()
			})
		})

		t.Run("Ingress rules with multiple ports for one Service use separate hostnames for each port", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[6],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Equal(t, "foo-svc.foo-namespace.80.svc",
				*parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"].Host)
			assert.Equal(t, "foo-svc.foo-namespace.8000.svc",
				*parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.8000"].Host)
		})

		t.Run("Ingress rule with ports defined by name", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[9],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			_, ok := parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"]
			assert.True(t, ok)
		})

		t.Run("Ingress rule with regex prefixed path creates route with Kong regex prefix", func(t *testing.T) {
			store, err := store.NewFakeStore(store.FakeObjects{
				IngressesV1: []*netv1.Ingress{
					ingressList[9],
				},
			})
			require.NoError(t, err)
			p := setupParser(t, store)

			parsedInfo := p.ingressRulesFromIngressV1()
			assert.Equal(t, translators.KongPathRegexPrefix+"/foo/\\d{3}", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.80"].Routes[0].Paths[0])
		})
	})
}

func TestFromIngressV1_RegexPrefix(t *testing.T) {
	assert := assert.New(t)
	pathTypeExact := netv1.PathTypeExact
	ingressList := []*netv1.Ingress{
		// 0
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "foo-namespace",
				Annotations: map[string]string{
					annotations.IngressClassKey: annotations.DefaultIngressClass,
				},
			},
			Spec: netv1.IngressSpec{
				Rules: []netv1.IngressRule{
					{
						Host: "example.com",
						IngressRuleValue: netv1.IngressRuleValue{
							HTTP: &netv1.HTTPIngressRuleValue{
								Paths: []netv1.HTTPIngressPath{
									{
										Path:     "/whatever",
										PathType: &pathTypeExact,
										Backend: netv1.IngressBackend{
											Service: &netv1.IngressServiceBackend{
												Name: "foo-svc",
												Port: netv1.ServiceBackendPort{Number: 80},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	t.Run("exact rule results in prefixed regex", func(t *testing.T) {
		store, err := store.NewFakeStore(store.FakeObjects{
			IngressesV1: []*netv1.Ingress{
				ingressList[0],
			},
		})
		require.NoError(t, err)
		p := mustNewParser(t, store)
		p.featureFlags.RegexPathPrefix = true

		parsedInfo := p.ingressRulesFromIngressV1()
		assert.Equal("~/whatever$", *parsedInfo.ServiceNameToServices["foo-namespace.foo-svc.pnum-80"].Routes[0].Paths[0])
	})
}

func TestGetDefaultBackendService(t *testing.T) {
	someIngress := func(creationTimestamp time.Time, serviceName string) netv1.Ingress {
		return netv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "foo",
				Namespace:         "foo-namespace",
				CreationTimestamp: metav1.NewTime(creationTimestamp),
			},
			Spec: netv1.IngressSpec{
				DefaultBackend: &netv1.IngressBackend{
					Service: &netv1.IngressServiceBackend{
						Name: serviceName,
						Port: netv1.ServiceBackendPort{Number: 80},
					},
				},
			},
		}
	}

	now := time.Now()
	testCases := []struct {
		name                       string
		ingresses                  []netv1.Ingress
		expressionRoutes           bool
		expectedHaveBackendService bool
		expectedServiceName        string
		expectedServiceHost        string
	}{
		{
			name:                       "no ingresses",
			ingresses:                  []netv1.Ingress{},
			expressionRoutes:           false,
			expectedHaveBackendService: false,
		},
		{
			name:                       "no ingresses with expression routes",
			ingresses:                  []netv1.Ingress{},
			expressionRoutes:           true,
			expectedHaveBackendService: false,
		},
		{
			name:                       "one ingress with default backend",
			ingresses:                  []netv1.Ingress{someIngress(now, "foo-svc")},
			expressionRoutes:           false,
			expectedHaveBackendService: true,
			expectedServiceName:        "foo-namespace.foo-svc.80",
			expectedServiceHost:        "foo-svc.foo-namespace.80.svc",
		},
		{
			name:                       "one ingress with default backend and expression routes enabled",
			ingresses:                  []netv1.Ingress{someIngress(now, "foo-svc")},
			expressionRoutes:           true,
			expectedHaveBackendService: true,
			expectedServiceName:        "foo-namespace.foo-svc.80",
			expectedServiceHost:        "foo-svc.foo-namespace.80.svc",
		},
		{
			name: "multiple ingresses with default backend",
			ingresses: []netv1.Ingress{
				someIngress(now.Add(time.Second), "newer"),
				someIngress(now, "older"),
			},
			expressionRoutes:           false,
			expectedHaveBackendService: true,
			expectedServiceName:        "foo-namespace.older.80",
			expectedServiceHost:        "older.foo-namespace.80.svc",
		},
		{
			name: "multiple ingresses with default backend and expression routes enabled",
			ingresses: []netv1.Ingress{
				someIngress(now.Add(time.Second), "newer"),
				someIngress(now, "older"),
			},
			expressionRoutes:           true,
			expectedHaveBackendService: true,
			expectedServiceName:        "foo-namespace.older.80",
			expectedServiceHost:        "older.foo-namespace.80.svc",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			svc, ok := getDefaultBackendService(tc.ingresses, tc.expressionRoutes)
			require.Equal(t, tc.expectedHaveBackendService, ok)
			if tc.expectedHaveBackendService {
				require.Equal(t, tc.expectedServiceName, *svc.Name)
				require.Equal(t, tc.expectedServiceHost, *svc.Host)
				require.Len(t, svc.Routes, 1)
				route := svc.Routes[0]
				if tc.expressionRoutes {
					require.Equal(t, `(http.path ^= "/") && ((net.protocol == "http") || (net.protocol == "https"))`, *route.Expression)
					require.Equal(t, translators.IngressDefaultBackendPriority, *route.Priority)
				} else {
					require.Len(t, route.Paths, 1)
					require.Equal(t, *route.Paths[0], "/")
				}
			}
		})
	}
}
