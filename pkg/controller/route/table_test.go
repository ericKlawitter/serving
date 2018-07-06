/*
Copyright 2018 The Knative Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package route

import (
	"testing"

	istiov1alpha3 "github.com/knative/serving/pkg/apis/istio/v1alpha3"
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/knative/serving/pkg/controller"
	"github.com/knative/serving/pkg/controller/route/config"
	"github.com/knative/serving/pkg/controller/route/resources"
	"github.com/knative/serving/pkg/controller/route/traffic"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgotesting "k8s.io/client-go/testing"

	. "github.com/knative/serving/pkg/controller/testing"
)

// This is heavily based on the way the OpenShift Ingress controller tests its reconciliation method.
func TestReconcile(t *testing.T) {
	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "configuration not yet ready",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{simpleRunLatest("default", "first-reconcile", "not-ready", nil)},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{simpleNotReadyConfig("default", "not-ready")},
			},
			Revision: &RevisionLister{
				Items: []*v1alpha1.Revision{
					simpleNotReadyRevision("default",
						// Use the Revision name from the config.
						simpleNotReadyConfig("default", "not-ready").Status.LatestCreatedRevisionName,
					),
				},
			},
		},
		WantCreates: []metav1.Object{
			resources.MakeK8sService(simpleRunLatest("default", "first-reconcile", "not-ready", nil)),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: addConfigLabel(
				simpleNotReadyConfig("default", "not-ready"),
				// The Route controller attaches our label to this Configuration.
				"serving.knative.dev/route", "first-reconcile",
			),
		}, {
			Object: simpleRunLatest("default", "first-reconcile", "not-ready", &v1alpha1.RouteStatus{
				Domain: "first-reconcile.default.example.com",
				// TODO(#1494): We currently report bad status for this case.
				Conditions: []v1alpha1.RouteCondition{{
					Type:    v1alpha1.RouteConditionAllTrafficAssigned,
					Status:  corev1.ConditionFalse,
					Reason:  "ConfigurationMissing",
					Message: `Referenced Configuration "not-ready" not found`,
				}, {
					Type:    v1alpha1.RouteConditionReady,
					Status:  corev1.ConditionFalse,
					Reason:  "ConfigurationMissing",
					Message: `Referenced Configuration "not-ready" not found`,
				}},
			}),
		}},
		WantErr: true,
		Key:     "default/first-reconcile",
	}, {
		Name: "simple route becomes ready",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{simpleRunLatest("default", "becomes-ready", "config", nil)},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{simpleReadyConfig("default", "config")},
			},
			Revision: &RevisionLister{
				Items: []*v1alpha1.Revision{
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
					),
				},
			},
		},
		WantCreates: []metav1.Object{
			resources.MakeVirtualService(
				setDomain(simpleRunLatest("default", "becomes-ready", "config", nil), "becomes-ready.default.example.com"),
				&traffic.TrafficConfig{
					Targets: map[string][]traffic.RevisionTarget{
						"": []traffic.RevisionTarget{{
							TrafficTarget: v1alpha1.TrafficTarget{
								// Use the Revision name from the config.
								RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
								Percent:      100,
							},
							Active: true,
						}},
					},
				},
			),
			resources.MakeK8sService(simpleRunLatest("default", "becomes-ready", "config", nil)),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: addConfigLabel(
				simpleReadyConfig("default", "config"),
				// The Route controller attaches our label to this Configuration.
				"serving.knative.dev/route", "becomes-ready",
			),
		}, {
			Object: simpleRunLatest("default", "becomes-ready", "config", &v1alpha1.RouteStatus{
				Domain: "becomes-ready.default.example.com",
				Conditions: []v1alpha1.RouteCondition{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					ConfigurationName: "config",
					RevisionName:      "config-00001",
					Percent:           100,
				}},
			}),
		}},
		Key: "default/becomes-ready",
	}, {
		Name: "steady state",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{
					simpleRunLatest("default", "steady-state", "config", &v1alpha1.RouteStatus{
						Domain: "steady-state.default.example.com",
						Conditions: []v1alpha1.RouteCondition{{
							Type:   v1alpha1.RouteConditionAllTrafficAssigned,
							Status: corev1.ConditionTrue,
						}, {
							Type:   v1alpha1.RouteConditionReady,
							Status: corev1.ConditionTrue,
						}},
						Traffic: []v1alpha1.TrafficTarget{{
							ConfigurationName: "config",
							RevisionName:      "config-00001",
							Percent:           100,
						}},
					}),
				},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{
					addConfigLabel(
						simpleReadyConfig("default", "config"),
						// The Route controller attaches our label to this Configuration.
						"serving.knative.dev/route", "steady-state",
					),
				},
			},
			Revision: &RevisionLister{
				Items: []*v1alpha1.Revision{
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
					),
				},
			},
			VirtualService: &VirtualServiceLister{
				Items: []*istiov1alpha3.VirtualService{
					resources.MakeVirtualService(
						setDomain(simpleRunLatest("default", "steady-state", "config", nil), "steady-state.default.example.com"),
						&traffic.TrafficConfig{
							Targets: map[string][]traffic.RevisionTarget{
								"": []traffic.RevisionTarget{{
									TrafficTarget: v1alpha1.TrafficTarget{
										// Use the Revision name from the config.
										RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
										Percent:      100,
									},
									Active: true,
								}},
							},
						},
					),
				},
			},
			K8sService: &K8sServiceLister{
				Items: []*corev1.Service{
					resources.MakeK8sService(simpleRunLatest("default", "steady-state", "config", nil)),
				},
			},
		},
		Key: "default/steady-state",
	}, {
		Name: "different labels, different domain",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{
					addRouteLabel(
						simpleRunLatest("default", "different-domain", "config", &v1alpha1.RouteStatus{
							Domain: "different-domain.default.another-example.com",
							Conditions: []v1alpha1.RouteCondition{{
								Type:   v1alpha1.RouteConditionAllTrafficAssigned,
								Status: corev1.ConditionTrue,
							}, {
								Type:   v1alpha1.RouteConditionReady,
								Status: corev1.ConditionTrue,
							}},
							Traffic: []v1alpha1.TrafficTarget{{
								ConfigurationName: "config",
								RevisionName:      "config-00001",
								Percent:           100,
							}},
						}),
						"app", "prod",
					),
				},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{
					addConfigLabel(
						simpleReadyConfig("default", "config"),
						// The Route controller attaches our label to this Configuration.
						"serving.knative.dev/route", "different-domain",
					),
				},
			},
			Revision: &RevisionLister{
				Items: []*v1alpha1.Revision{
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
					),
				},
			},
			VirtualService: &VirtualServiceLister{
				Items: []*istiov1alpha3.VirtualService{
					resources.MakeVirtualService(
						setDomain(simpleRunLatest("default", "different-domain", "config", nil), "different-domain.default.another-example.com"),
						&traffic.TrafficConfig{
							Targets: map[string][]traffic.RevisionTarget{
								"": []traffic.RevisionTarget{{
									TrafficTarget: v1alpha1.TrafficTarget{
										// Use the Revision name from the config.
										RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
										Percent:      100,
									},
									Active: true,
								}},
							},
						},
					),
				},
			},
			K8sService: &K8sServiceLister{
				Items: []*corev1.Service{
					resources.MakeK8sService(simpleRunLatest("default", "different-domain", "config", nil)),
				},
			},
		},
		Key: "default/different-domain",
	}, {
		Name: "new latest created revision",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{
					simpleRunLatest("default", "new-latest-created", "config", &v1alpha1.RouteStatus{
						Domain: "new-latest-created.default.example.com",
						Conditions: []v1alpha1.RouteCondition{{
							Type:   v1alpha1.RouteConditionAllTrafficAssigned,
							Status: corev1.ConditionTrue,
						}, {
							Type:   v1alpha1.RouteConditionReady,
							Status: corev1.ConditionTrue,
						}},
						Traffic: []v1alpha1.TrafficTarget{{
							ConfigurationName: "config",
							RevisionName:      "config-00001",
							Percent:           100,
						}},
					}),
				},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{
					setLatestCreatedRevision(
						addConfigLabel(
							simpleReadyConfig("default", "config"),
							// The Route controller attaches our label to this Configuration.
							"serving.knative.dev/route", "new-latest-created",
						),
						"config-00002",
					),
				},
			},
			Revision: &RevisionLister{
				Items: []*v1alpha1.Revision{
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
					),
					// This is the name of the new revision we're referencing above.
					simpleNotReadyRevision("default", "config-00002"),
				},
			},
			VirtualService: &VirtualServiceLister{
				Items: []*istiov1alpha3.VirtualService{
					resources.MakeVirtualService(
						setDomain(simpleRunLatest("default", "new-latest-created", "config", nil), "new-latest-created.default.example.com"),
						&traffic.TrafficConfig{
							Targets: map[string][]traffic.RevisionTarget{
								"": []traffic.RevisionTarget{{
									TrafficTarget: v1alpha1.TrafficTarget{
										// Use the Revision name from the config.
										RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
										Percent:      100,
									},
									Active: true,
								}},
							},
						},
					),
				},
			},
			K8sService: &K8sServiceLister{
				Items: []*corev1.Service{
					resources.MakeK8sService(simpleRunLatest("default", "new-latest-created", "config", nil)),
				},
			},
		},
		// A new LatestCreatedRevisionName on the Configuration alone should result in no changes to the Route.
		Key: "default/new-latest-created",
	}, {
		Name: "new latest ready revision",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{
					simpleRunLatest("default", "new-latest-ready", "config", &v1alpha1.RouteStatus{
						Domain: "new-latest-ready.default.example.com",
						Conditions: []v1alpha1.RouteCondition{{
							Type:   v1alpha1.RouteConditionAllTrafficAssigned,
							Status: corev1.ConditionTrue,
						}, {
							Type:   v1alpha1.RouteConditionReady,
							Status: corev1.ConditionTrue,
						}},
						Traffic: []v1alpha1.TrafficTarget{{
							ConfigurationName: "config",
							RevisionName:      "config-00001",
							Percent:           100,
						}},
					}),
				},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{
					setLatestReadyRevision(setLatestCreatedRevision(
						addConfigLabel(
							simpleReadyConfig("default", "config"),
							// The Route controller attaches our label to this Configuration.
							"serving.knative.dev/route", "new-latest-ready",
						),
						"config-00002",
					)),
				},
			},
			Revision: &RevisionLister{
				Items: []*v1alpha1.Revision{
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
					),
					// This is the name of the new revision we're referencing above.
					simpleReadyRevision("default", "config-00002"),
				},
			},
			VirtualService: &VirtualServiceLister{
				Items: []*istiov1alpha3.VirtualService{
					resources.MakeVirtualService(
						setDomain(simpleRunLatest("default", "new-latest-ready", "config", nil), "new-latest-ready.default.example.com"),
						&traffic.TrafficConfig{
							Targets: map[string][]traffic.RevisionTarget{
								"": []traffic.RevisionTarget{{
									TrafficTarget: v1alpha1.TrafficTarget{
										// Use the Revision name from the config.
										RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
										Percent:      100,
									},
									Active: true,
								}},
							},
						},
					),
				},
			},
			K8sService: &K8sServiceLister{
				Items: []*corev1.Service{
					resources.MakeK8sService(simpleRunLatest("default", "new-latest-ready", "config", nil)),
				},
			},
		},
		// A new LatestReadyRevisionName on the Configuration should result in the new Revision being rolled out.
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: resources.MakeVirtualService(
				setDomain(simpleRunLatest("default", "new-latest-ready", "config", nil), "new-latest-ready.default.example.com"),
				&traffic.TrafficConfig{
					Targets: map[string][]traffic.RevisionTarget{
						"": []traffic.RevisionTarget{{
							TrafficTarget: v1alpha1.TrafficTarget{
								// This is the new config we're making become ready.
								RevisionName: "config-00002",
								Percent:      100,
							},
							Active: true,
						}},
					},
				},
			),
		}, {
			Object: simpleRunLatest("default", "new-latest-ready", "config", &v1alpha1.RouteStatus{
				Domain: "new-latest-ready.default.example.com",
				Conditions: []v1alpha1.RouteCondition{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					ConfigurationName: "config",
					RevisionName:      "config-00002",
					Percent:           100,
				}},
			}),
		}},
		Key: "default/new-latest-ready",
	}, {
		Name: "reconcile service mutation",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{
					simpleRunLatest("default", "svc-mutation", "config", &v1alpha1.RouteStatus{
						Domain: "svc-mutation.default.example.com",
						Conditions: []v1alpha1.RouteCondition{{
							Type:   v1alpha1.RouteConditionAllTrafficAssigned,
							Status: corev1.ConditionTrue,
						}, {
							Type:   v1alpha1.RouteConditionReady,
							Status: corev1.ConditionTrue,
						}},
						Traffic: []v1alpha1.TrafficTarget{{
							ConfigurationName: "config",
							RevisionName:      "config-00001",
							Percent:           100,
						}},
					}),
				},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{
					addConfigLabel(
						simpleReadyConfig("default", "config"),
						// The Route controller attaches our label to this Configuration.
						"serving.knative.dev/route", "svc-mutation",
					),
				},
			},
			Revision: &RevisionLister{
				Items: []*v1alpha1.Revision{
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
					),
				},
			},
			VirtualService: &VirtualServiceLister{
				Items: []*istiov1alpha3.VirtualService{
					resources.MakeVirtualService(
						setDomain(simpleRunLatest("default", "svc-mutation", "config", nil), "svc-mutation.default.example.com"),
						&traffic.TrafficConfig{
							Targets: map[string][]traffic.RevisionTarget{
								"": []traffic.RevisionTarget{{
									TrafficTarget: v1alpha1.TrafficTarget{
										// Use the Revision name from the config.
										RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
										Percent:      100,
									},
									Active: true,
								}},
							},
						},
					),
				},
			},
			K8sService: &K8sServiceLister{
				Items: []*corev1.Service{
					mutateService(resources.MakeK8sService(simpleRunLatest("default", "svc-mutation", "config", nil))),
				},
			},
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: resources.MakeK8sService(simpleRunLatest("default", "svc-mutation", "config", nil)),
		}},
		Key: "default/svc-mutation",
	}, {
		Name: "allow cluster ip",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{
					simpleRunLatest("default", "cluster-ip", "config", &v1alpha1.RouteStatus{
						Domain: "cluster-ip.default.example.com",
						Conditions: []v1alpha1.RouteCondition{{
							Type:   v1alpha1.RouteConditionAllTrafficAssigned,
							Status: corev1.ConditionTrue,
						}, {
							Type:   v1alpha1.RouteConditionReady,
							Status: corev1.ConditionTrue,
						}},
						Traffic: []v1alpha1.TrafficTarget{{
							ConfigurationName: "config",
							RevisionName:      "config-00001",
							Percent:           100,
						}},
					}),
				},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{
					addConfigLabel(
						simpleReadyConfig("default", "config"),
						// The Route controller attaches our label to this Configuration.
						"serving.knative.dev/route", "cluster-ip",
					),
				},
			},
			Revision: &RevisionLister{
				Items: []*v1alpha1.Revision{
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
					),
				},
			},
			VirtualService: &VirtualServiceLister{
				Items: []*istiov1alpha3.VirtualService{
					resources.MakeVirtualService(
						setDomain(simpleRunLatest("default", "cluster-ip", "config", nil), "cluster-ip.default.example.com"),
						&traffic.TrafficConfig{
							Targets: map[string][]traffic.RevisionTarget{
								"": []traffic.RevisionTarget{{
									TrafficTarget: v1alpha1.TrafficTarget{
										// Use the Revision name from the config.
										RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
										Percent:      100,
									},
									Active: true,
								}},
							},
						},
					),
				},
			},
			K8sService: &K8sServiceLister{
				Items: []*corev1.Service{
					setClusterIP(resources.MakeK8sService(simpleRunLatest("default", "cluster-ip", "config", nil)), "127.0.0.1"),
				},
			},
		},
		Key: "default/cluster-ip",
	}, {
		Name: "reconcile virtual service mutation",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{
					simpleRunLatest("default", "virt-svc-mutation", "config", &v1alpha1.RouteStatus{
						Domain: "virt-svc-mutation.default.example.com",
						Conditions: []v1alpha1.RouteCondition{{
							Type:   v1alpha1.RouteConditionAllTrafficAssigned,
							Status: corev1.ConditionTrue,
						}, {
							Type:   v1alpha1.RouteConditionReady,
							Status: corev1.ConditionTrue,
						}},
						Traffic: []v1alpha1.TrafficTarget{{
							ConfigurationName: "config",
							RevisionName:      "config-00001",
							Percent:           100,
						}},
					}),
				},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{
					addConfigLabel(
						simpleReadyConfig("default", "config"),
						// The Route controller attaches our label to this Configuration.
						"serving.knative.dev/route", "virt-svc-mutation",
					),
				},
			},
			Revision: &RevisionLister{
				Items: []*v1alpha1.Revision{
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
					),
				},
			},
			VirtualService: &VirtualServiceLister{
				Items: []*istiov1alpha3.VirtualService{
					mutateVirtualService(resources.MakeVirtualService(
						setDomain(simpleRunLatest("default", "virt-svc-mutation", "config", nil), "virt-svc-mutation.default.example.com"),
						&traffic.TrafficConfig{
							Targets: map[string][]traffic.RevisionTarget{
								"": []traffic.RevisionTarget{{
									TrafficTarget: v1alpha1.TrafficTarget{
										// Use the Revision name from the config.
										RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
										Percent:      100,
									},
									Active: true,
								}},
							},
						},
					)),
				},
			},
			K8sService: &K8sServiceLister{
				Items: []*corev1.Service{
					resources.MakeK8sService(simpleRunLatest("default", "virt-svc-mutation", "config", nil)),
				},
			},
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: resources.MakeVirtualService(
				setDomain(simpleRunLatest("default", "virt-svc-mutation", "config", nil), "virt-svc-mutation.default.example.com"),
				&traffic.TrafficConfig{
					Targets: map[string][]traffic.RevisionTarget{
						"": []traffic.RevisionTarget{{
							TrafficTarget: v1alpha1.TrafficTarget{
								// Use the Revision name from the config.
								RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
								Percent:      100,
							},
							Active: true,
						}},
					},
				},
			),
		}},
		Key: "default/virt-svc-mutation",
	}, {
		Name: "config labelled by another route",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{
					simpleRunLatest("default", "licked-cookie", "config", &v1alpha1.RouteStatus{
						Domain: "licked-cookie.default.example.com",
						Conditions: []v1alpha1.RouteCondition{{
							Type:   v1alpha1.RouteConditionAllTrafficAssigned,
							Status: corev1.ConditionTrue,
						}, {
							Type:   v1alpha1.RouteConditionReady,
							Status: corev1.ConditionTrue,
						}},
						Traffic: []v1alpha1.TrafficTarget{{
							ConfigurationName: "config",
							RevisionName:      "config-00001",
							Percent:           100,
						}},
					}),
				},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{
					addConfigLabel(
						simpleReadyConfig("default", "config"),
						// This configuration is being referenced by another Route.
						"serving.knative.dev/route", "this-cookie-has-been-licked",
					),
				},
			},
			Revision: &RevisionLister{
				Items: []*v1alpha1.Revision{
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
					),
				},
			},
			VirtualService: &VirtualServiceLister{
				Items: []*istiov1alpha3.VirtualService{
					resources.MakeVirtualService(
						setDomain(simpleRunLatest("default", "licked-cookie", "config", nil), "licked-cookie.default.example.com"),
						&traffic.TrafficConfig{
							Targets: map[string][]traffic.RevisionTarget{
								"": []traffic.RevisionTarget{{
									TrafficTarget: v1alpha1.TrafficTarget{
										// Use the Revision name from the config.
										RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
										Percent:      100,
									},
									Active: true,
								}},
							},
						},
					),
				},
			},
			K8sService: &K8sServiceLister{
				Items: []*corev1.Service{
					resources.MakeK8sService(simpleRunLatest("default", "licked-cookie", "config", nil)),
				},
			},
		},
		WantErr: true,
		Key:     "default/licked-cookie",
	}, {
		Name: "switch to a different config",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{
					// The status reflects "oldconfig", but the spec "newconfig".
					simpleRunLatest("default", "change-configs", "newconfig", &v1alpha1.RouteStatus{
						Domain: "change-configs.default.example.com",
						Conditions: []v1alpha1.RouteCondition{{
							Type:   v1alpha1.RouteConditionAllTrafficAssigned,
							Status: corev1.ConditionTrue,
						}, {
							Type:   v1alpha1.RouteConditionReady,
							Status: corev1.ConditionTrue,
						}},
						Traffic: []v1alpha1.TrafficTarget{{
							ConfigurationName: "oldconfig",
							RevisionName:      "oldconfig-00001",
							Percent:           100,
						}},
					}),
				},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{
					// Both configs exist, but only "oldconfig" is labelled.
					addConfigLabel(
						simpleReadyConfig("default", "oldconfig"),
						// The Route controller attaches our label to this Configuration.
						"serving.knative.dev/route", "change-configs",
					),
					simpleReadyConfig("default", "newconfig"),
				},
			},
			Revision: &RevisionLister{
				Items: []*v1alpha1.Revision{
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "oldconfig").Status.LatestReadyRevisionName,
					),
					simpleReadyRevision("default",
						// Use the Revision name from the config.
						simpleReadyConfig("default", "newconfig").Status.LatestReadyRevisionName,
					),
				},
			},
			VirtualService: &VirtualServiceLister{
				Items: []*istiov1alpha3.VirtualService{
					resources.MakeVirtualService(
						setDomain(simpleRunLatest("default", "change-configs", "oldconfig", nil), "change-configs.default.example.com"),
						&traffic.TrafficConfig{
							Targets: map[string][]traffic.RevisionTarget{
								"": []traffic.RevisionTarget{{
									TrafficTarget: v1alpha1.TrafficTarget{
										// Use the Revision name from the config.
										RevisionName: simpleReadyConfig("default", "oldconfig").Status.LatestReadyRevisionName,
										Percent:      100,
									},
									Active: true,
								}},
							},
						},
					),
				},
			},
			K8sService: &K8sServiceLister{
				Items: []*corev1.Service{
					resources.MakeK8sService(simpleRunLatest("default", "change-configs", "oldconfig", nil)),
				},
			},
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			// The label is removed from "oldconfig"
			Object: simpleReadyConfig("default", "oldconfig"),
		}, {
			// The label is added to "newconfig"
			Object: addConfigLabel(
				simpleReadyConfig("default", "newconfig"),
				// The Route controller attaches our label to this Configuration.
				"serving.knative.dev/route", "change-configs",
			),
		}, {
			// Updated to point to "newconfig" things.
			Object: resources.MakeVirtualService(
				setDomain(simpleRunLatest("default", "change-configs", "newconfig", nil), "change-configs.default.example.com"),
				&traffic.TrafficConfig{
					Targets: map[string][]traffic.RevisionTarget{
						"": []traffic.RevisionTarget{{
							TrafficTarget: v1alpha1.TrafficTarget{
								// Use the Revision name from the config.
								RevisionName: simpleReadyConfig("default", "newconfig").Status.LatestReadyRevisionName,
								Percent:      100,
							},
							Active: true,
						}},
					},
				},
			),
		}, {
			// Status updated to "newconfig"
			Object: simpleRunLatest("default", "change-configs", "newconfig", &v1alpha1.RouteStatus{
				Domain: "change-configs.default.example.com",
				Conditions: []v1alpha1.RouteCondition{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					ConfigurationName: "newconfig",
					RevisionName:      "newconfig-00001",
					Percent:           100,
				}},
			}),
		}},
		Key: "default/change-configs",
	}, {
		Name: "configuration missing",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{simpleRunLatest("default", "config-missing", "not-found", nil)},
			},
		},
		WantCreates: []metav1.Object{
			resources.MakeK8sService(simpleRunLatest("default", "config-missing", "not-found", nil)),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: simpleRunLatest("default", "config-missing", "not-found", &v1alpha1.RouteStatus{
				Domain: "config-missing.default.example.com",
				Conditions: []v1alpha1.RouteCondition{{
					Type:    v1alpha1.RouteConditionAllTrafficAssigned,
					Status:  corev1.ConditionFalse,
					Reason:  "ConfigurationMissing",
					Message: `Referenced Configuration "not-found" not found`,
				}, {
					Type:    v1alpha1.RouteConditionReady,
					Status:  corev1.ConditionFalse,
					Reason:  "ConfigurationMissing",
					Message: `Referenced Configuration "not-found" not found`,
				}},
			}),
		}},
		WantErr: true,
		Key:     "default/config-missing",
	}, {
		Name: "revision missing (direct)",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{simplePinned("default", "missing-revision-direct", "not-found", nil)},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{simpleReadyConfig("default", "config")},
			},
		},
		WantCreates: []metav1.Object{
			resources.MakeK8sService(simpleRunLatest("default", "missing-revision-direct", "config", nil)),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			// TODO(#1496): Even without adding the label we see an update because of #1496
			// (we remove the non-existent label).
			Object: simpleReadyConfig("default", "config"),
		}, {
			Object: simplePinned("default", "missing-revision-direct", "not-found", &v1alpha1.RouteStatus{
				Domain: "missing-revision-direct.default.example.com",
				Conditions: []v1alpha1.RouteCondition{{
					Type:    v1alpha1.RouteConditionAllTrafficAssigned,
					Status:  corev1.ConditionFalse,
					Reason:  "RevisionMissing",
					Message: `Referenced Revision "not-found" not found`,
				}, {
					Type:    v1alpha1.RouteConditionReady,
					Status:  corev1.ConditionFalse,
					Reason:  "RevisionMissing",
					Message: `Referenced Revision "not-found" not found`,
				}},
			}),
		}},
		WantErr: true,
		Key:     "default/missing-revision-direct",
	}, {
		Name: "revision missing (indirect)",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{simpleRunLatest("default", "missing-revision-indirect", "config", nil)},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{simpleReadyConfig("default", "config")},
			},
		},
		WantCreates: []metav1.Object{
			resources.MakeK8sService(simpleRunLatest("default", "missing-revision-indirect", "config", nil)),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: addConfigLabel(
				simpleReadyConfig("default", "config"),
				// The Route controller attaches our label to this Configuration.
				"serving.knative.dev/route", "missing-revision-indirect",
			),
		}, {
			Object: simpleRunLatest("default", "missing-revision-indirect", "config", &v1alpha1.RouteStatus{
				Domain: "missing-revision-indirect.default.example.com",
				// TODO(#1494): We currently report bad status for this case.
				Conditions: []v1alpha1.RouteCondition{{
					Type:    v1alpha1.RouteConditionAllTrafficAssigned,
					Status:  corev1.ConditionFalse,
					Reason:  "ConfigurationMissing",
					Message: `Referenced Configuration "config" not found`,
				}, {
					Type:    v1alpha1.RouteConditionReady,
					Status:  corev1.ConditionFalse,
					Reason:  "ConfigurationMissing",
					Message: `Referenced Configuration "config" not found`,
				}},
			}),
		}},
		WantErr: true,
		Key:     "default/missing-revision-indirect",
	}, {
		Name: "pinned route becomes ready",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{simplePinned("default", "pinned-becomes-ready",
					// Use the Revision name from the config
					simpleReadyConfig("default", "config").Status.LatestReadyRevisionName, nil)},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{simpleReadyConfig("default", "config")},
			},
			Revision: &RevisionLister{
				Items: []*v1alpha1.Revision{
					addOwnerRef(
						simpleReadyRevision("default",
							// Use the Revision name from the config.
							simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
						),
						or("Configuration", "config"),
					),
				},
			},
		},
		WantCreates: []metav1.Object{
			resources.MakeVirtualService(
				setDomain(simpleRunLatest("default", "pinned-becomes-ready", "config", nil), "pinned-becomes-ready.default.example.com"),
				&traffic.TrafficConfig{
					Targets: map[string][]traffic.RevisionTarget{
						"": []traffic.RevisionTarget{{
							TrafficTarget: v1alpha1.TrafficTarget{
								// Use the Revision name from the config.
								RevisionName: simpleReadyConfig("default", "config").Status.LatestReadyRevisionName,
								Percent:      100,
							},
							Active: true,
						}},
					},
				},
			),
			resources.MakeK8sService(simpleRunLatest("default", "pinned-becomes-ready", "config", nil)),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			// TODO(#1496): Even without adding the label we see an update because of #1496
			// (we remove the non-existent label).
			Object: simpleReadyConfig("default", "config"),
			// TODO(#1495): The parent configuration isn't labeled because it's established through
			// labels instead of owner references.
			// addConfigLabel(
			// 	simpleReadyConfig("default", "config"),
			// 	// The Route controller attaches our label to this Configuration.
			// 	"serving.knative.dev/route", "pinned-becomes-ready",
			// ),
		}, {
			Object: simplePinned("default", "pinned-becomes-ready",
				// Use the config's revision name.
				simpleReadyConfig("default", "config").Status.LatestReadyRevisionName, &v1alpha1.RouteStatus{
					Domain: "pinned-becomes-ready.default.example.com",
					Conditions: []v1alpha1.RouteCondition{{
						Type:   v1alpha1.RouteConditionAllTrafficAssigned,
						Status: corev1.ConditionTrue,
					}, {
						Type:   v1alpha1.RouteConditionReady,
						Status: corev1.ConditionTrue,
					}},
					Traffic: []v1alpha1.TrafficTarget{{
						// TODO(#1495): This is established thru labels instead of OwnerReferences.
						// ConfigurationName: "config",
						RevisionName: "config-00001",
						Percent:      100,
					}},
				}),
		}},
		Key: "default/pinned-becomes-ready",
	}, {
		Name: "traffic split becomes ready",
		Listers: Listers{
			Route: &RouteLister{
				Items: []*v1alpha1.Route{
					routeWithTraffic("default", "named-traffic-split", nil,
						v1alpha1.TrafficTarget{
							ConfigurationName: "blue",
							Percent:           50,
						}, v1alpha1.TrafficTarget{
							ConfigurationName: "green",
							Percent:           50,
						}),
				},
			},
			Configuration: &ConfigurationLister{
				Items: []*v1alpha1.Configuration{
					simpleReadyConfig("default", "blue"),
					simpleReadyConfig("default", "green"),
				},
			},
			Revision: &RevisionLister{
				Items: []*v1alpha1.Revision{
					addOwnerRef(
						simpleReadyRevision("default",
							// Use the Revision name from the config.
							simpleReadyConfig("default", "blue").Status.LatestReadyRevisionName,
						),
						or("Configuration", "blue"),
					),
					addOwnerRef(
						simpleReadyRevision("default",
							// Use the Revision name from the config.
							simpleReadyConfig("default", "green").Status.LatestReadyRevisionName,
						),
						or("Configuration", "green"),
					),
				},
			},
		},
		WantCreates: []metav1.Object{
			resources.MakeVirtualService(
				setDomain(routeWithTraffic("default", "named-traffic-split", nil,
					v1alpha1.TrafficTarget{
						ConfigurationName: "blue",
						Percent:           50,
					}, v1alpha1.TrafficTarget{
						ConfigurationName: "green",
						Percent:           50,
					}), "named-traffic-split.default.example.com"),
				&traffic.TrafficConfig{
					Targets: map[string][]traffic.RevisionTarget{
						"": []traffic.RevisionTarget{{
							TrafficTarget: v1alpha1.TrafficTarget{
								// Use the Revision name from the config.
								RevisionName: simpleReadyConfig("default", "blue").Status.LatestReadyRevisionName,
								Percent:      50,
							},
							Active: true,
						}, {
							TrafficTarget: v1alpha1.TrafficTarget{
								// Use the Revision name from the config.
								RevisionName: simpleReadyConfig("default", "green").Status.LatestReadyRevisionName,
								Percent:      50,
							},
							Active: true,
						}},
					},
				},
			),
			resources.MakeK8sService(routeWithTraffic("default", "named-traffic-split", nil,
				v1alpha1.TrafficTarget{
					ConfigurationName: "blue",
					Percent:           50,
				}, v1alpha1.TrafficTarget{
					ConfigurationName: "green",
					Percent:           50,
				})),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: addConfigLabel(
				simpleReadyConfig("default", "blue"),
				// The Route controller attaches our label to this Configuration.
				"serving.knative.dev/route", "named-traffic-split",
			),
		}, {
			Object: addConfigLabel(
				simpleReadyConfig("default", "green"),
				// The Route controller attaches our label to this Configuration.
				"serving.knative.dev/route", "named-traffic-split",
			),
		}, {
			Object: routeWithTraffic("default", "named-traffic-split", &v1alpha1.RouteStatus{
				Domain: "named-traffic-split.default.example.com",
				Conditions: []v1alpha1.RouteCondition{{
					Type:   v1alpha1.RouteConditionAllTrafficAssigned,
					Status: corev1.ConditionTrue,
				}, {
					Type:   v1alpha1.RouteConditionReady,
					Status: corev1.ConditionTrue,
				}},
				Traffic: []v1alpha1.TrafficTarget{{
					ConfigurationName: "blue",
					RevisionName:      "blue-00001",
					Percent:           50,
				}, {
					ConfigurationName: "green",
					RevisionName:      "green-00001",
					Percent:           50,
				}},
			}, v1alpha1.TrafficTarget{
				ConfigurationName: "blue",
				Percent:           50,
			}, v1alpha1.TrafficTarget{
				ConfigurationName: "green",
				Percent:           50,
			}),
		}},
		Key: "default/named-traffic-split",
	}}

	// TODO(mattmoor): Revision inactive (direct reference)
	// TODO(mattmoor): Revision inactive (indirect reference)
	// TODO(mattmoor): Multiple inactive Revisions

	table.Test(t, func(listers *Listers, opt controller.Options) controller.Interface {
		return &Controller{
			Base:                 controller.NewBase(opt, controllerAgentName, "Routes"),
			routeLister:          listers.GetRouteLister(),
			configurationLister:  listers.GetConfigurationLister(),
			revisionLister:       listers.GetRevisionLister(),
			serviceLister:        listers.GetK8sServiceLister(),
			virtualServiceLister: listers.GetVirtualServiceLister(),
			domainConfig: &config.Domain{
				Domains: map[string]*config.LabelSelector{
					"example.com": &config.LabelSelector{},
					"another-example.com": &config.LabelSelector{
						Selector: map[string]string{"app": "prod"},
					},
				},
			},
		}
	})
}

func mutateVirtualService(vs *istiov1alpha3.VirtualService) *istiov1alpha3.VirtualService {
	// Thor's Hammer
	vs.Spec = istiov1alpha3.VirtualServiceSpec{}
	return vs
}

func mutateService(svc *corev1.Service) *corev1.Service {
	// Thor's Hammer
	svc.Spec = corev1.ServiceSpec{}
	return svc
}

func setClusterIP(svc *corev1.Service, ip string) *corev1.Service {
	svc.Spec.ClusterIP = ip
	return svc
}

func routeWithTraffic(namespace, name string, status *v1alpha1.RouteStatus, traffic ...v1alpha1.TrafficTarget) *v1alpha1.Route {
	route := &v1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1alpha1.RouteSpec{
			Traffic: traffic,
		},
	}
	if status != nil {
		route.Status = *status
	}
	return route
}

func simplePinned(namespace, name, revision string, status *v1alpha1.RouteStatus) *v1alpha1.Route {
	return routeWithTraffic(namespace, name, status, v1alpha1.TrafficTarget{
		RevisionName: revision,
		Percent:      100,
	})
}

func simpleRunLatest(namespace, name, config string, status *v1alpha1.RouteStatus) *v1alpha1.Route {
	return routeWithTraffic(namespace, name, status, v1alpha1.TrafficTarget{
		ConfigurationName: config,
		Percent:           100,
	})
}

func setDomain(route *v1alpha1.Route, domain string) *v1alpha1.Route {
	route.Status.Domain = domain
	return route
}

func simpleNotReadyConfig(namespace, name string) *v1alpha1.Configuration {
	cfg := &v1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	cfg.Status.InitializeConditions()
	cfg.Status.SetLatestCreatedRevisionName(name + "-00001")
	return cfg
}

func simpleReadyConfig(namespace, name string) *v1alpha1.Configuration {
	return setLatestReadyRevision(simpleNotReadyConfig(namespace, name))
}

func setLatestCreatedRevision(cfg *v1alpha1.Configuration, name string) *v1alpha1.Configuration {
	cfg.Status.SetLatestCreatedRevisionName(name)
	return cfg
}

func setLatestReadyRevision(cfg *v1alpha1.Configuration) *v1alpha1.Configuration {
	cfg.Status.SetLatestReadyRevisionName(cfg.Status.LatestCreatedRevisionName)
	return cfg
}

func addRouteLabel(route *v1alpha1.Route, key, value string) *v1alpha1.Route {
	if route.Labels == nil {
		route.Labels = make(map[string]string)
	}
	route.Labels[key] = value
	return route
}

func addConfigLabel(config *v1alpha1.Configuration, key, value string) *v1alpha1.Configuration {
	if config.Labels == nil {
		config.Labels = make(map[string]string)
	}
	config.Labels[key] = value
	return config
}

func simpleReadyRevision(namespace, name string) *v1alpha1.Revision {
	return &v1alpha1.Revision{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: v1alpha1.RevisionStatus{
			Conditions: []v1alpha1.RevisionCondition{{
				Type:   v1alpha1.RevisionConditionReady,
				Status: corev1.ConditionTrue,
			}},
		},
	}
}

func simpleNotReadyRevision(namespace, name string) *v1alpha1.Revision {
	return &v1alpha1.Revision{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: v1alpha1.RevisionStatus{
			Conditions: []v1alpha1.RevisionCondition{{
				Type:   v1alpha1.RevisionConditionReady,
				Status: corev1.ConditionTrue,
			}},
		},
	}
}

func addOwnerRef(rev *v1alpha1.Revision, o []metav1.OwnerReference) *v1alpha1.Revision {
	rev.OwnerReferences = o
	return rev
}

// or builds OwnerReferences for a child of a Service
func or(kind, name string) []metav1.OwnerReference {
	boolTrue := true
	return []metav1.OwnerReference{{
		APIVersion:         v1alpha1.SchemeGroupVersion.String(),
		Kind:               kind,
		Name:               name,
		Controller:         &boolTrue,
		BlockOwnerDeletion: &boolTrue,
	}}
}