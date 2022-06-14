/*
Copyright 2021 The Kubernetes Authors.

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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	workv1alpha1 "sigs.k8s.io/work-api/pkg/apis/v1alpha1"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

var _ = Describe("Garbage Collected", func() {
	var appliedWorkName string
	var resourceName string
	var resourceNamespace string
	var workName string
	var workNamespace string

	const timeout = time.Second * 30
	const interval = time.Second * 1

	// BeforeEach test ensure:
	// #1 - A namespace exists for the Work to reside within.
	// #2 - A manifest of some type should be within the Work object.
	BeforeEach(func() {
		workName = "wn-" + utilrand.String(5)
		workNamespace = "wns-" + utilrand.String(5)
		resourceName = "rn-" + utilrand.String(5)
		resourceNamespace = "rns" + utilrand.String(5)
		appliedWorkName = workName

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: workNamespace,
			},
		}
		_, err := k8sClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		// Create the Work object with some type of Manifest resource.
		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: resourceNamespace,
			},
			Data: map[string]string{
				"test": "test",
			},
		}

		work := &workv1alpha1.Work{
			ObjectMeta: metav1.ObjectMeta{
				Name:      workName,
				Namespace: workNamespace,
			},
			Spec: workv1alpha1.WorkSpec{
				Workload: workv1alpha1.WorkloadTemplate{
					Manifests: []workv1alpha1.Manifest{
						{
							RawExtension: runtime.RawExtension{Object: cm},
						},
					},
				},
			},
		}

		_, createWorkErr := workClient.MulticlusterV1alpha1().Works(workNamespace).Create(context.Background(), work, metav1.CreateOptions{})
		Expect(createWorkErr).ToNot(HaveOccurred())
	})

	// AfterEach test ensure:
	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		err := k8sClient.CoreV1().Namespaces().Delete(context.Background(), workNamespace, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())
	})

	Context("A Work object with manifests has been created.", func() {
		It("Should have created an AppliedWork object", func() {
			Eventually(func() error {
				_, err := workClient.MulticlusterV1alpha1().AppliedWorks().Get(context.Background(), workName, metav1.GetOptions{})
				return err
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("A Work object with manifests has been marked for deletion.", func() {
		// Mark the existing work object for deletion.
		BeforeEach(func() {
			work, err := workClient.MulticlusterV1alpha1().Works(workNamespace).Get(context.Background(), workName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			controllerutil.AddFinalizer(work, workFinalizer)
			work.Status.Conditions = []metav1.Condition{}
			_, err = workClient.MulticlusterV1alpha1().Works(workNamespace).UpdateStatus(context.Background(), work, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			err = workClient.MulticlusterV1alpha1().Works(workNamespace).Delete(context.Background(), workName, metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should have deleted the AppliedWork object.", func() {
			Eventually(func() bool {
				_, err := workClient.MulticlusterV1alpha1().AppliedWorks().Get(context.Background(), appliedWorkName, metav1.GetOptions{})

				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})

	})

})
