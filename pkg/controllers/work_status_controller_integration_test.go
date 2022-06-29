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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	workv1alpha1 "sigs.k8s.io/work-api/pkg/apis/v1alpha1"
)

var _ = Describe("Work Status Controller", func() {
	var resourceName string
	var resourceNamespace string
	var workName string
	var workNamespace string

	const timeout = time.Second * 30
	const interval = time.Second * 1

	BeforeEach(func() {
		workName = utilrand.String(5)
		workNamespace = utilrand.String(5)
		resourceName = utilrand.String(5)
		resourceNamespace = utilrand.String(5)

		wns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: workNamespace,
			},
		}
		_, err := k8sClient.CoreV1().Namespaces().Create(context.Background(), wns, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		rns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: resourceNamespace,
			},
		}
		_, err = k8sClient.CoreV1().Namespaces().Create(context.Background(), rns, metav1.CreateOptions{})
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

		Eventually(func() bool {
			appliedWorkObject, err2 := workClient.MulticlusterV1alpha1().AppliedWorks().Get(context.Background(), workName, metav1.GetOptions{})
			if err2 == nil {
				return appliedWorkObject.Spec.WorkName == workName
			}
			return false
		}, timeout, interval).Should(BeTrue())
	})

	AfterEach(func() {
		err := k8sClient.CoreV1().Namespaces().Delete(context.Background(), workNamespace, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())
	})

	Context("A Work object with manifests has been created", func() {
		It("Resource was removed from the Work Status", func() {
			currentWork, err := workClient.MulticlusterV1alpha1().Works(workNamespace).Get(context.Background(), workName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			currentWork.Status.ManifestConditions = []workv1alpha1.ManifestCondition{}

			_, err = workClient.MulticlusterV1alpha1().Works(workNamespace).Update(context.Background(), currentWork, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				gvr := schema.GroupVersionResource{
					Group:    "core",
					Version:  "v1",
					Resource: "ConfigMap",
				}
				_, err := dynamicClient.Resource(gvr).Namespace(resourceNamespace).Get(context.Background(), resourceName, metav1.GetOptions{})
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
		It("Resource is deleted from the AppliedResources of the AppliedWork", func() {
			currentAppliedWork, err := workClient.MulticlusterV1alpha1().AppliedWorks().Get(context.Background(), workName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			currentAppliedWork.Status.AppliedResources = []workv1alpha1.AppliedResourceMeta{}
			_, err = workClient.MulticlusterV1alpha1().AppliedWorks().Update(context.Background(), currentAppliedWork, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				currentAppliedWork, err := workClient.MulticlusterV1alpha1().AppliedWorks().Get(context.Background(), workName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return len(currentAppliedWork.Status.AppliedResources) != 0
			}, timeout, interval).Should(BeTrue())
		})
	})
})
