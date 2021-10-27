package controllers

import (
	"context"
	"reflect"

	"github.com/aryan9600/cluster-config-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type NamespaceReconicler struct {
	client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	CCMReconciler *ClusterConfigMapReconciler
}

//+kubebuilder:rbac:groups=extensions.toolkit.fluxcd.io,resources=clusterconfigmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
func (r *NamespaceReconicler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("Namespace", req.NamespacedName)

	// Fetch required Namespace according to the req.
	var namespace corev1.Namespace
	if err := r.Get(ctx, req.NamespacedName, &namespace); err != nil {
		log.Error(err, "could not fetch namespace")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	namespaceLabels := namespace.GetLabels()

	// Fetch all the ClusterConfigMaps in the cluster.
	var clusterConfigMaps v1alpha1.ClusterConfigMapList
	if err := r.List(ctx, &clusterConfigMaps); err != nil {
		log.Error(err, "Could not list ClusterConfigMaps")
		return ctrl.Result{}, err
	}

	items := clusterConfigMaps.Items
	// Loop through the ClusterConfigMaps, check if the labels are a subset of the Namespace's labels
	// and create the ConfigMap accordingly.
	for i, clusterConfigMap := range items {
		clusterConfigMap := clusterConfigMap
		clusterConfigMapLabels := clusterConfigMap.Spec.GenerateTo.NamespaceSelectors.MatchLabels
		if isMapSubset(namespaceLabels, clusterConfigMapLabels) {
			// requeue the ClusterConfigMap over to its reconciler
			r.CCMReconciler.requeueCh <- event.GenericEvent{Object: &items[i]}
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceReconicler) SetupWithManager(mgr ctrl.Manager) error {
	log := r.Log
	namespacePredicate := predicate.Funcs{
		CreateFunc: func(ce event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(ue event.UpdateEvent) bool {
			if ue.ObjectOld == nil {
				log.Error(nil, "Update event has no old object to update", "event", ue)
				return false
			}
			if ue.ObjectNew == nil {
				log.Error(nil, "Update event has no new object for update", "event", ue)
				return false
			}

			return !reflect.DeepEqual(ue.ObjectNew.GetLabels(), ue.ObjectOld.GetLabels())
		},
		DeleteFunc:  func(de event.DeleteEvent) bool { return false },
		GenericFunc: func(ge event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		WithEventFilter(namespacePredicate).
		Complete(r)
}
