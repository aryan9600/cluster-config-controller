/*
Copyright 2021.

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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ref "k8s.io/client-go/tools/reference"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	extensionsv1alpha1 "github.com/aryan9600/cluster-config-controller/api/v1alpha1"
	"github.com/go-logr/logr"
)

// ClusterConfigMapReconciler reconciles a ClusterConfigMap object
type ClusterConfigMapReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	// Requeue channel for creating ConfigMaps upon Namespace events.
	requeueCh chan event.GenericEvent
}

var configMapNameKey = ".metadata.name"

//+kubebuilder:rbac:groups=extensions.toolkit.fluxcd.io,resources=clusterconfigmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=extensions.toolkit.fluxcd.io,resources=clusterconfigmaps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=extensions.toolkit.fluxcd.io,resources=clusterconfigmaps/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
func (r *ClusterConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("ClusterConfigMap", req.NamespacedName)

	// Fetch required ClusterConfigMap according to the req.
	var clusterConfigMap extensionsv1alpha1.ClusterConfigMap
	if err := r.Get(ctx, req.NamespacedName, &clusterConfigMap); err != nil {
		log.Error(err, "could not fetch ClusterConfigMap")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Fetched ClusterConfigMap", "Identidier", req.NamespacedName)

	finalizerName := "extensions.toolkit.fluxcd.io/finalizer"
	// Fetch all the child ConfigMaps created by this ClusterConfigMap.
	var childConfigMaps corev1.ConfigMapList
	if err := r.List(ctx, &childConfigMaps, client.MatchingFields{configMapNameKey: req.Name}); err != nil {
		log.Error(err, "unable to list child config maps")
		return ctrl.Result{}, err
	}

	log.Info("Fetched child ConfigMaps", "Quantity", len(childConfigMaps.Items))

	// Use DeletionTimestamp to determine whether object is being deleted or not.
	if clusterConfigMap.DeletionTimestamp.IsZero() {
		// Add finalizer if it doesn't exist.
		if !containsString(clusterConfigMap.GetFinalizers(), finalizerName) {
			controllerutil.AddFinalizer(&clusterConfigMap, finalizerName)
			if err := r.Update(ctx, &clusterConfigMap); err != nil {
				log.Error(err, "Could not add finalizer to ClusterConfigMap")
				return ctrl.Result{}, err
			}
		}
	} else {
		// ClusterConfigMap is being deleted.
		if containsString(clusterConfigMap.GetFinalizers(), finalizerName) {
			// Delete all child ConfigMaps.
			for _, cm := range childConfigMaps.Items {
				if err := r.Delete(ctx, &cm); err != nil {
					log.Error(err, "unable to delete child config maps")
					return ctrl.Result{}, err
				}
			}
			// Remove finalizer from the object.
			controllerutil.RemoveFinalizer(&clusterConfigMap, finalizerName)
			if err := r.Update(ctx, &clusterConfigMap); err != nil {
				log.Error(err, "unable to remove finalizer from ClusterConfigMap")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Populate the current status by storing the references to all the child ConfigMaps
	clusterConfigMap.Status.ConfigMaps = nil
	for _, childConfigMap := range childConfigMaps.Items {
		childConfigMap := childConfigMap
		configMapRef, err := ref.GetReference(r.Scheme, &childConfigMap)
		if err != nil {
			log.Error(err, "unable to make reference to child config map")
			continue
		}
		clusterConfigMap.Status.ConfigMaps = append(clusterConfigMap.Status.ConfigMaps, *configMapRef)
	}

	// Update the status.
	if err := r.Status().Update(ctx, &clusterConfigMap); err != nil {
		log.Error(err, "could not update ClusterConfigMap CRD status")
		return ctrl.Result{}, err
	}

	spec := clusterConfigMap.Spec

	// Fetch all namespaces defined by the spec.
	var specNamespaces corev1.NamespaceList
	labelSelectors, err := metav1.LabelSelectorAsSelector(&spec.GenerateTo.NamespaceSelectors)
	if err != nil {
		log.Error(err, "could not form LabelSelector to match against Namespaces")
		return ctrl.Result{}, err
	}
	if err := r.List(ctx, &specNamespaces, client.MatchingLabelsSelector{Selector: labelSelectors}); err != nil {
		log.Error(err, "could not list Namespaces with matching labels")
		return ctrl.Result{}, err
	}

	// Namespaces with ConfigMaps in the cluster at the moment(Status).
	var statusNamespaces []corev1.Namespace

	// Loop through the ConfigMaps present in Status and:
	//  1. Form a list of Namespaces that have those ConfigMaps.
	//  2. Delete the ConfigMap if a Namespace is in the cluster Status but not in the Spec.
	for _, statusConfigMap := range clusterConfigMap.Status.ConfigMaps {
		// Fetch the namespaces present in the cluster.
		statusConfigMap := statusConfigMap
		var nameSpace corev1.Namespace
		if err := r.Get(ctx, client.ObjectKey{Name: statusConfigMap.Namespace}, &nameSpace); err != nil {
			log.Error(err, "Could not get Namespace present in CRD status")
			return ctrl.Result{}, err
		}
		statusNamespaces = append(statusNamespaces, nameSpace)

		valid := checkIfValidNameSpace(statusConfigMap.Namespace, specNamespaces.Items)
		if !valid {
			var configMap corev1.ConfigMap
			if err := r.Get(ctx, types.NamespacedName{Namespace: statusConfigMap.Namespace, Name: statusConfigMap.Name}, &configMap); err != nil {
				log.Error(err, "Could not get ConfigMap", "ns", statusConfigMap.Namespace)
				return ctrl.Result{}, err
			}
			if err := r.Delete(ctx, &configMap); err != nil {
				log.Error(err, "Could not delete ConfigMap", "ns", statusConfigMap.Namespace)
				return ctrl.Result{}, err
			}
		}
	}

	// Create Configmap if a Namespace is in the spec but not in cluster Status.
	for _, specNameSpace := range specNamespaces.Items {
		specNameSpace := specNameSpace
		valid := checkIfValidNameSpace(specNameSpace.GetName(), statusNamespaces)
		if !valid {
			configMap := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      make(map[string]string),
					Annotations: make(map[string]string),
					Name:        clusterConfigMap.GetName(),
					Namespace:   specNameSpace.GetName(),
				},
				Data: spec.Data,
			}
			identifier := types.NamespacedName{Namespace: specNameSpace.GetName(), Name: clusterConfigMap.GetName()}
			if err := upsertConfigmap(ctx, identifier, configMap, r.Client); err != nil {
				log.Error(err, "could not upsert cm")
				return ctrl.Result{}, err
			}
		}
	}

	// Update the existing configmaps' data.
	for _, childConfigMap := range childConfigMaps.Items {
		childConfigMap := childConfigMap
		childConfigMap.Data = spec.Data
		if err := r.Update(ctx, &childConfigMap); err != nil {
			log.Error(err, "Could not update config map", "ConfigMap", childConfigMap)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.ConfigMap{}, configMapNameKey, func(o client.Object) []string {
		configMap := o.(*corev1.ConfigMap)
		name := configMap.GetName()
		return []string{name}
	}); err != nil {
		return err
	}
	r.requeueCh = make(chan event.GenericEvent)
	return ctrl.NewControllerManagedBy(mgr).
		For(&extensionsv1alpha1.ClusterConfigMap{}).
		Watches(&source.Channel{
			Source: r.requeueCh,
		}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
