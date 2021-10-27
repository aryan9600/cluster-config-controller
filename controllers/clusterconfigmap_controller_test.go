package controllers

import (
	"context"
	"time"

	extensionsv1alpha1 "github.com/aryan9600/cluster-config-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
				NamespaceSelectors: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/context": "testing",
					},
				},
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
				c := 0
				for _, cm := range cmList.Items {
					if cm.GetName() == ccmName {
						c += 1
					}
				}
				return c, nil
			}, timeout, interval).Should(Equal(2))
		})
	})

	Context("When updating `.spec.data` of a ClusterConfigMap", func() {
		It("Should update the `.spec.data` of all of it's child ConfigMaps.", func() {
			By("Updating `.spec.data` of the existing ClusterConfigMap")
			var clusterConfigMap extensionsv1alpha1.ClusterConfigMap
			k8sClient.Get(context.Background(), ccmObjectKey, &clusterConfigMap)
			clusterConfigMap.Spec.Data = map[string]string{"this": "is not the way"}
			Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := k8sClient.Update(context.Background(), &clusterConfigMap); err != nil {
					return err
				}
				return nil
			})).Should(Succeed())

			Eventually(func() bool {
				updatedCCM := extensionsv1alpha1.ClusterConfigMap{}
				if err := k8sClient.Get(context.Background(), ccmObjectKey, &updatedCCM); err != nil {
					return false
				}
				if len(updatedCCM.Spec.Data) == 1 && updatedCCM.Spec.Data["this"] == "is not the way" {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Checking if the `.data` got updated of all child ConfigMaps")
			Eventually(func() (int, error) {
				var cmList corev1.ConfigMapList
				c := 0
				if err := k8sClient.List(context.Background(), &cmList); err != nil {
					return 0, err
				}
				for _, cm := range cmList.Items {
					if cm.GetName() == ccmName && cm.Data["this"] == "is not the way" {
						// GinkgoWriter.Write([]byte(cm.GetName()))
						// GinkgoWriter.Write([]byte("\n"))
						// GinkgoWriter.Write([]byte(fmt.Sprint(c)))
						// GinkgoWriter.Write([]byte("\n"))
						// GinkgoWriter.Write([]byte(cm.GetNamespace()))
						// GinkgoWriter.Write([]byte("\n"))
						c += 1
					}
				}
				return c, nil
			}, timeout, interval).Should(Equal(2))
		})
	})

	Context("When updating `.spec.generateTo.namespaceSelectors` of a ClusterConfigMap", func() {
		It("Should create/delete it's ConfigMpas accordingly.", func() {
			By("Updating `.spec.generateTo.namespaceSelectors` of the existing ClusterConfigMap")
			var clusterConfigMap extensionsv1alpha1.ClusterConfigMap
			k8sClient.Get(context.Background(), ccmObjectKey, &clusterConfigMap)
			clusterConfigMap.Spec.GenerateTo.NamespaceSelectors.MatchLabels = map[string]string{"app.kubernetes.io/context": "prod"}
			Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := k8sClient.Update(context.Background(), &clusterConfigMap); err != nil {
					return err
				}
				return nil
			})).Should(Succeed())

			By("Checking if the existing ConfigMaps in Namespaces 'ns1' and 'ns2' got deleted.")
			Eventually(func() (bool, error) {
				var nsList corev1.NamespaceList
				labelSelector, err := metav1.LabelSelectorAsSelector(&LabelSelector)
				if err != nil {
					return false, err
				}
				if err := k8sClient.List(context.Background(), &nsList, client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
					return false, err
				}
				for _, ns := range nsList.Items {
					var cm corev1.ConfigMap
					if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns.Name, Name: ccmName}, &cm); err != nil {
						if !apierrors.IsNotFound(err) {
							return false, err
						}
					}
				}
				return true, nil
			}, timeout, interval).Should(BeTrue())

			By("Checking if the a new ConfigMap in Namespaces 'ns3' got created.")
			Eventually(func() (bool, error) {
				var nsList corev1.NamespaceList
				var labels client.MatchingLabels
				labels = map[string]string{"app.kubernetes.io/context": "prod"}
				if err := k8sClient.List(context.Background(), &nsList, labels); err != nil {
					return false, err
				}
				for _, ns := range nsList.Items {
					var cm corev1.ConfigMap
					if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns.Name, Name: ccmName}, &cm); err != nil {
						if apierrors.IsNotFound(err) {
							return false, err
						}
					}
				}
				return true, nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When creating a new Namespace with label selectors as specified in the ClusterConfigMap", func() {
		It("Should create a ConfigMap in the new Namespace", func() {
			By("Creating a new Namespace")
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: make(map[string]string),
					Labels: map[string]string{
						"app.kubernetes.io/context": "prod",
					},
					Name: "ns4",
				},
			}
			Expect(k8sClient.Create(context.Background(), &ns)).Should(Succeed())

			By("Checking if a new ConfigMap got created in the 'ns4'")
			Eventually(func() (bool, error) {
				var cm corev1.ConfigMap
				if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: "ns4", Name: ccmName}, &cm); err != nil {
					return false, err
				}
				return true, nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When modifying the label selectors of a Namespace", func() {
		It("Should create a ConfigMap in the Namespace if the label match the ones specified in the ClusterConfigMap", func() {
			By("Modifying the label selctors of a Namespace")
			var ns corev1.Namespace
			Eventually(func() (bool, error) {
				if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: "ns2"}, &ns); err != nil {
					return false, err
				}
				return true, nil
			}, timeout, interval).Should(BeTrue())
			ns.SetLabels(map[string]string{
				"app.kubernetes.io/context": "prod",
			})
			Expect(k8sClient.Update(context.Background(), &ns)).Should(Succeed())

			By("Checking if a new ConfigMap got created in the 'ns2'")
			Eventually(func() (bool, error) {
				var cm corev1.ConfigMap
				if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: "ns2", Name: ccmName}, &cm); err != nil {
					return false, err
				}
				return true, nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When deleting a ClusterConfigMap", func() {
		It("Should delete all it's child ConfigMaps", func() {
			By("Deleting the ClusterConfigMap")
			var ccm extensionsv1alpha1.ClusterConfigMap
			Eventually(func() bool {
				if err := k8sClient.Get(context.Background(), ccmObjectKey, &ccm); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(k8sClient.Delete(context.Background(), &ccm)).Should(Succeed())

			By("Checking the existence of all it's child ConfigMaps")
			Eventually(func() (bool, error) {
				var configMaps corev1.ConfigMapList
				if err := k8sClient.List(context.Background(), &configMaps); err != nil {
					return false, err
				}
				for _, cm := range configMaps.Items {
					if cm.GetName() == ccmName {
						return false, nil
					}
				}
				return true, nil
			})
		})
	})
})
