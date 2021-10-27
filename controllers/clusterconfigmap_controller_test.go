package controllers

import (
	"context"
	"time"

	extensionsv1alpha1 "github.com/aryan9600/cluster-config-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ClusterConfigMap Controller", func() {
	const (
		ccmName = "test-ccm"
		ccmNs   = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	ccm := extensionsv1alpha1.ClusterConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions.toolkit.fluxcd.io/v1alpha1",
			Kind:       "ClusterConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ccmName,
			Namespace: ccmNs,
		},
		Spec: extensionsv1alpha1.ClusterConfigMapSpec{
			Data: map[string]string{
				"this": "is the way",
			},
			GenerateTo: extensionsv1alpha1.ClusterConfigMapGenerateTo{
				NamespaceSelectors: LabelSelector,
			},
		},
	}
	ccmObjectKey := types.NamespacedName{Namespace: ccmNs, Name: ccmName}

	Context("When creating a new ClusterConfigMap", func() {
		It("Should created the required ConfigMap in all valid Namespaces.", func() {
			By("Creating a new ClusterConfigMap")
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, &ccm)).Should(Succeed())
			Eventually(func() bool {
				createdCCM := extensionsv1alpha1.ClusterConfigMap{}
				if err := k8sClient.Get(ctx, ccmObjectKey, &createdCCM); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Checking whether a ConfigMap with the same name got created in the matching namespace.")
			Eventually(func() (int, error) {
				var cmList corev1.ConfigMapList
				if err := k8sClient.List(ctx, &cmList); err != nil {
					return 0, err
				}
				var c int
				for _, cm := range cmList.Items {
					if cm.GetName() == ccmName && (cm.GetNamespace() == "ns1" || cm.GetNamespace() == "ns2") {
						c += 1
					}
				}
				return c, nil
			}).Should(Equal(2))
		})
	})

	Context("When updating `.spec.data` of a ClusterConfigMap", func() {
		It("Should update the `.spec.data` of all of it's child ConfigMaps.", func() {
			By("Updating `.spec.data` of the existing ClusterConfigMap")
			var clusterConfigMap extensionsv1alpha1.ClusterConfigMap
			k8sClient.Get(context.Background(), ccmObjectKey, &clusterConfigMap)
			clusterConfigMap.Spec.Data = map[string]string{"this": "is not the way"}
			Expect(k8sClient.Update(context.Background(), &clusterConfigMap)).Should(Succeed())
			Eventually(func() bool {
				var updatedCCM extensionsv1alpha1.ClusterConfigMap
				if err := k8sClient.Get(context.Background(), ccmObjectKey, &ccm); err != nil {
					if len(updatedCCM.Spec.Data) == 1 && updatedCCM.Spec.Data["this"] == "is not the way" {
						return true
					}
					return false
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Checking if the `.spec.data` got updated of all child ConfigMaps")
			Eventually(func() (int, error) {
				var cmList corev1.ConfigMapList
				var c int
				if err := k8sClient.List(context.Background(), &cmList); err != nil {
					for _, cm := range cmList.Items {
						if cm.GetName() == ccmName && cm.Data["this"] == "is not the way" {
							c += 1
						}
					}
					return c, nil
				} else {
					return 0, err
				}
			}).Should(Equal(2))
		})
	})
})
