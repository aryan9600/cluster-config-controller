package controllers

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func checkIfValidNameSpace(nameSpace string, nameSpaceItems []corev1.Namespace) bool {
	for _, nameSpaceItem := range nameSpaceItems {
		if nameSpaceItem.ObjectMeta.Name == nameSpace {
			return true
		}
	}
	return false
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

// https://stackoverflow.com/a/67904962/10745226
func isMapSubset(mapSet interface{}, mapSubset interface{}) bool {
	mapSetValue := reflect.ValueOf(mapSet)
	mapSubsetValue := reflect.ValueOf(mapSubset)

	if fmt.Sprintf("%T", mapSet) != fmt.Sprintf("%T", mapSubset) {
		return false
	}
	if len(mapSetValue.MapKeys()) < len(mapSubsetValue.MapKeys()) {
		return false
	}
	if len(mapSubsetValue.MapKeys()) == 0 {
		return true
	}

	iterMapSubset := mapSubsetValue.MapRange()
	for iterMapSubset.Next() {
		k := iterMapSubset.Key()
		v := iterMapSubset.Value()

		value := mapSetValue.MapIndex(k)

		if !value.IsValid() || v.Interface() != value.Interface() {
			return false
		}
	}

	return true
}

// Checks if the same ConfigMap already exists before creating it.
func upsertConfigmap(ctx context.Context, identifier types.NamespacedName, configMap corev1.ConfigMap, client client.Client) error {
	var cm corev1.ConfigMap
	// Check if ConfigMap already exists in the Namespace
	if err := client.Get(ctx, identifier, &cm); err != nil {
		// If Configmap not found in Namespace, create the Configmap.
		if apierrors.IsNotFound(err) {
			if err := client.Create(ctx, &configMap); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}
