package controllers

import (
	"context"
	"errors"
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
	labelSelectors := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/context": "test",
		},
	}

	Context("When updating ClusterConfigMap status", func() {
		It("Should add the created ConfigMaps to ClusterConfigMap.Status.Namespaces when it's created.", func() {
			By("Creating a new ClusterConfigMap")
			ctx := context.Background()
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
						"this is": "the way",
					},
					GenerateTo: extensionsv1alpha1.ClusterConfigMapGenerateTo{
						NamespaceSelectors: labelSelectors,
					},
				},
			}
			Expect(k8sClient.Create(ctx, &ccm)).Should(Succeed())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ccmNs, Name: ccmName}, &ccm); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Checking whether a ConfigMap with the same data got created in the matching namespace.")
			Eventually(func() ([]string, error) {
				var cmList corev1.ConfigMapList
				if err := k8sClient.List(ctx, &cmList); err != nil {
					return nil, err
				}
				for _, cm := range cmList.Items {
					GinkgoWriter.Write([]byte(cm.Name))
				}
				return nil, errors.New("")
			})
		})
	})
})
