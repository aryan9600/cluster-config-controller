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
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	extensionsv1alpha1 "github.com/aryan9600/cluster-config-controller/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var LabelSelector metav1.LabelSelector

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = extensionsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	ccmReconciler := ClusterConfigMapReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("ClusterConfigMap"),
	}

	LabelSelector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/context": "testing",
		},
	}

	var nameSpaces []corev1.Namespace
	nameSpaces = append(nameSpaces, corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: make(map[string]string),
			Labels:      LabelSelector.MatchLabels,
			Name:        "ns1",
		},
	})
	nameSpaces = append(nameSpaces, corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: make(map[string]string),
			Labels:      LabelSelector.MatchLabels,
			Name:        "ns2",
		},
	})
	nameSpaces = append(nameSpaces, corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: make(map[string]string),
			Labels: map[string]string{
				"app.kubernetes.io/context": "prod",
			},
			Name: "ns3",
		},
	})

	for _, ns := range nameSpaces {
		ns := ns
		k8sClient.Create(context.Background(), &ns)
	}

	err = ccmReconciler.SetupWithManager(k8sManager)

	err = (&NamespaceReconicler{
		Client:        k8sManager.GetClient(),
		Scheme:        k8sManager.GetScheme(),
		Log:           ctrl.Log.WithName("controllers").WithName("Namespace"),
		CCMReconciler: &ccmReconciler,
	}).SetupWithManager(k8sManager)

	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
