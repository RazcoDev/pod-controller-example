/*


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

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Loading environment variables

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func constructServiceForPod(r *PodReconciler, pod *corev1.Pod) (*corev1.Service, error) {
	name := fmt.Sprintf("%s-svc", pod.Name)
	var servicePortArray []corev1.ServicePort
	driverPort := corev1.ServicePort{
		Name:     "driver-rpc-port",
		Protocol: "TCP",
		Port:     7078,
		TargetPort: intstr.IntOrString{
			IntVal: 7078,
		},
	}
	blockmgrPort := corev1.ServicePort{
		Name:     "blockmanager",
		Protocol: "TCP",
		Port:     7079,
		TargetPort: intstr.IntOrString{
			IntVal: 7079,
		},
	}
	webuiPort := corev1.ServicePort{
		Name:     "webui",
		Protocol: "TCP",
		Port:     4040,
		TargetPort: intstr.IntOrString{
			IntVal: 4040,
		},
	}

	servicePortArray = append(servicePortArray, driverPort, blockmgrPort, webuiPort)
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   pod.Namespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: corev1.ServiceSpec{
			Ports: servicePortArray,
		},
	}
	for k, v := range pod.Labels {
		service.Labels[k] = v
	}

	if err := ctrl.SetControllerReference(pod, service, r.Scheme); err != nil {
		return nil, err
	}
	return service, nil

}

func constructRouteForService(r *PodReconciler, service *corev1.Service, pod *corev1.Pod, clusterName string) (*v1.Route, error) {
	name := fmt.Sprintf("%s-ingress", pod.Name)
	hostname := fmt.Sprintf("%s-ingress.%s", pod.Name, clusterName)
	routePort := &v1.RoutePort{TargetPort: intstr.IntOrString{
		IntVal: service.Spec.Ports[0].Port,
	}}

	route := &v1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind: "route.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pod.Namespace,
			Labels:    make(map[string]string),
		},
		Spec: v1.RouteSpec{
			Host: hostname,
			To: v1.RouteTargetReference{
				Name: service.Name,
			},
			Port: routePort,
		},
	}
	for k, v := range service.Labels {
		route.Labels[k] = v
	}

	if err := ctrl.SetControllerReference(pod, route, r.Scheme); err != nil {
		return nil, err
	}
	return route, nil

}

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=routes,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
func (r *PodReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	labelKey := os.Getenv("label_key")
	if labelKey == "" {

		return ctrl.Result{}, fmt.Errorf("label_key environment variable is missing")
	}
	labelValue := os.Getenv("label_value")
	if labelValue == "" {
		return ctrl.Result{}, fmt.Errorf("label_value environment variable is missing")
	}

	clusterName := os.Getenv("cluster_name")
	if clusterName == "" {
		return ctrl.Result{}, fmt.Errorf("cluster_name environment variable is missing")
	}

	ctx := context.Background()
	log := r.Log.WithValues("pod", req.NamespacedName)
	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		log.Error(err, "Unable to fetch Pod")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if pod.Labels[labelKey] == labelValue {
		if pod.Status.Phase == "Running" {
			var (
				childServices corev1.ServiceList
				childRoutes   v1.RouteList
			)
			if err := r.List(ctx, &childServices, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
				log.Error(err, "Unable to list child services")
				return ctrl.Result{}, err
			}

			if len(childServices.Items) == 0 {
				serviceCreate := func(childServices *corev1.ServiceList, pod corev1.Pod) (*corev1.Service, error) {
					service, err := constructServiceForPod(r, &pod)
					if err != nil {
						log.Error(err, "Unable to construct Service for Pod : %s", pod.Name)
						return nil, err
					}

					if err := r.Create(ctx, service); err != nil {
						log.Error(err, "Unable to create Service for Pod", "service", service)
						return nil, err
					}
					r.Recorder.Event(&pod, corev1.EventTypeNormal, "Created", "Service has been created - "+service.Name)
					log.V(1).Info("Created Service for Pod", "service", service)
					return service, err
				}

				_, err := serviceCreate(&childServices, pod)
				if err != nil {
					log.Error(err, "Unable to create Service for Pod", "pod", pod)
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}

			if err := r.List(ctx, &childRoutes, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
				log.Error(err, "Unable to list child Routes")
				return ctrl.Result{}, err
			}
			if len(childRoutes.Items) != 0 {
				lenS := childRoutes.Items[0]
				log.Info(lenS.Name)
			}
			if len(childRoutes.Items) == 0 {

				routeCreate := func(childRoutes *v1.RouteList, service corev1.Service) (*v1.Route, error) {
					if len(childRoutes.Items) != 0 {
						//log.Info(string(len(childRoutes.Items)))
						return &childRoutes.Items[0], nil
					} else {
						route, err := constructRouteForService(r, &service, &pod, clusterName)
						if err != nil {
							log.Error(err, "Unable to construct Route for Service : %s", service.Name)
							return nil, err
						}

						if err := r.Create(ctx, route); err != nil {
							log.Error(err, "Unable to create Route for Service", "route", route)
							return nil, err
						}
						r.Recorder.Event(&pod, corev1.EventTypeNormal, "Created", "Route has been created - "+route.Name)
						log.V(1).Info("Created Route for Service", "route", route)
						return route, err
					}
				}

				_, err := routeCreate(&childRoutes, childServices.Items[0])
				if err != nil {
					log.Error(err, "Unable to create Route for Service", "service", childServices.Items[0])
					return ctrl.Result{}, err
				}
			}
		}
	}
	return ctrl.Result{}, nil
}

var (
	jobOwnerKey = ".metadata.controller"
	apiGVStr    = corev1.SchemeGroupVersion.String()
)

func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := v1.AddToScheme(mgr.GetScheme()); err != nil {
	}

	if err := mgr.GetFieldIndexer().IndexField(&corev1.Service{}, jobOwnerKey, func(rawObj runtime.Object) []string {
		service := rawObj.(*corev1.Service)
		owner := metav1.GetControllerOf(service)
		if owner == nil {
			return nil
		}

		if owner.APIVersion != apiGVStr || owner.Kind != "Pod" {
			return nil
		}

		return []string{owner.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(&v1.Route{}, jobOwnerKey, func(rawObj runtime.Object) []string {
		route := rawObj.(*v1.Route)

		owner := metav1.GetControllerOf(route)
		if owner == nil {
			return nil
		}

		if owner.APIVersion != apiGVStr || owner.Kind != "Pod" {
			return nil
		}

		return []string{owner.Name}
	}); err != nil {
		return err
	}
	r.Recorder = mgr.GetEventRecorderFor("Raz-Controller")

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Owns(&v1.Route{}).
		Complete(r)
}
