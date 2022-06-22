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

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"time"

	workapi "sigs.k8s.io/work-api/pkg/apis/v1alpha1"
)

const (
	eventuallyTimeout  = 60 // seconds
	eventuallyInterval = 1  // seconds
)

var _ = ginkgo.Describe("Apply Work", func() {
	defaultManifestFiles := []string{
		"testmanifests/test-deployment.yaml",
		"testmanifests/test-service.yaml",
	}

	ginkgo.Context("Create a service and deployment", func() {
		ginkgo.It("Should create work successfully", func() {
			// Set
			createdWork, err := createWork(defaultManifestFiles)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// Vet
			gomega.Eventually(func() error {
				_, err := spokeKubeClient.AppsV1().Deployments("default").Get(context.Background(), "test-nginx", metav1.GetOptions{})
				if err != nil {
					return err
				}

				_, err = spokeKubeClient.CoreV1().Services("default").Get(context.Background(), "test-nginx", metav1.GetOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())
			gomega.Eventually(func() error {
				work, err := hubWorkClient.MulticlusterV1alpha1().Works(createdWork.Namespace).Get(context.Background(), createdWork.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}

				if !meta.IsStatusConditionTrue(work.Status.Conditions, "Applied") {
					return fmt.Errorf("Expect the applied contidion of the work is true")
				}

				return nil
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())

			// Reset
			err = deleteWork(createdWork)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
		ginkgo.It("Should create new resources successfully.", func() {
			// Set
			createdWork, err := createWork(defaultManifestFiles)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			newManifestFiles := []string{
				"testmanifests/test-deployment2.yaml",
				"testmanifests/test-service2.yaml",
			}

			work, err := hubWorkClient.MulticlusterV1alpha1().Works(createdWork.Namespace).Get(context.Background(), createdWork.Name, metav1.GetOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(work).ToNot(gomega.BeNil())

			addManifestsToWorkSpec(newManifestFiles, &work.Spec)

			_, err = hubWorkClient.MulticlusterV1alpha1().Works(work.Namespace).Update(context.Background(), work, metav1.UpdateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// Vet
			gomega.Eventually(func() error {
				_, err := spokeKubeClient.AppsV1().Deployments("default").Get(context.Background(), "test-nginx2", metav1.GetOptions{})
				if err != nil {
					return err
				}

				_, err = spokeKubeClient.CoreV1().Services("default").Get(context.Background(), "test-nginx2", metav1.GetOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())
			gomega.Eventually(func() error {
				work, err := hubWorkClient.MulticlusterV1alpha1().Works(work.Namespace).Get(context.Background(), work.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}

				if !meta.IsStatusConditionTrue(work.Status.Conditions, "Applied") {
					return fmt.Errorf("Expect the applied contidion of the work is true")
				}

				return nil
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())

			// Reset
			err = deleteWork(createdWork)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
	})
	ginkgo.Context("Deleting a Work resource from the Hub", func() {
		ginkgo.It("should delete: Work resource, AppliedWork resource, and the actual resources created by the Work.spec", func() {
			// Set
			createdWork, err := createWork(defaultManifestFiles)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// Get AppliedWork(s) so we can verify garbage collection.
			var appliedWorks *workapi.AppliedWork
			gomega.Eventually(func() error {
				appliedWorks, err = spokeWorkClient.MulticlusterV1alpha1().AppliedWorks().Get(context.Background(), createdWork.Name, metav1.GetOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())

			err = deleteWork(createdWork)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// Ensure the work resource was deleted from the Hub.
			gomega.Eventually(func() error {
				_, err = hubWorkClient.MulticlusterV1alpha1().Works(createdWork.Namespace).Get(context.Background(), createdWork.Name, metav1.GetOptions{})

				return err
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.HaveOccurred())

			// Ensure the AppliedWork resource was deleted from the spoke.
			gomega.Eventually(func() error {
				_, err = spokeWorkClient.MulticlusterV1alpha1().AppliedWorks().Get(context.Background(), createdWork.Name, metav1.GetOptions{})

				return err
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.HaveOccurred())

			// Ensure the resources are garbage collection on the spoke.
			gomega.Eventually(func() bool {
				garbageCollectionComplete := true
				for _, resourceMeta := range appliedWorks.Status.AppliedResources {
					gvr := schema.GroupVersionResource{
						Group:    resourceMeta.Group,
						Version:  resourceMeta.Version,
						Resource: resourceMeta.Resource,
					}
					_, err := spokeDynamicClient.Resource(gvr).Get(context.Background(), resourceMeta.Name, metav1.GetOptions{})

					// ToDo - Replace all HTTP calls with proper err code expectation.
					if err == nil {
						garbageCollectionComplete = false
						break
					}
				}

				return garbageCollectionComplete
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
		})
	})
	ginkgo.Context("Modify a field within a manifest", func() {
		ginkgo.It("Should reapply the manifest", func() {
			// Setup
			configMapManifest := []string{
				"testmanifests/test-configmap.yaml",
			}

			createdWork, err := createWork(configMapManifest)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// Give the spoke cluster time to create the map.
			time.Sleep(5 * time.Second)
			configMap, err := spokeKubeClient.CoreV1().ConfigMaps("default").Get(context.Background(), "test-configmap", metav1.GetOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			// Retrieve the owner reference for the existing resource, then retrieve the work spec from the hub using its owner reference.
			// Note: The index of 0 can be trusted only due to being in a testing environment.
			cmOwner := configMap.OwnerReferences[0]
			appliedWork, err := spokeWorkClient.MulticlusterV1alpha1().AppliedWorks().Get(context.Background(), cmOwner.Name, metav1.GetOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			awResources := appliedWork.Status.AppliedResources

			// Locate the AppliedResourceMeta by GVK+R details.
			gvk := configMap.GroupVersionKind()
			var matchIndex int
			for i, resourceMeta := range awResources {
				if resourceMeta.Group == gvk.Group &&
					resourceMeta.Version == gvk.Version &&
					resourceMeta.Kind == gvk.Kind &&
					resourceMeta.Namespace == configMap.Namespace &&
					resourceMeta.Name == configMap.Name {
					matchIndex = i
					break
				}
			}

			appliedResourceMeta := awResources[matchIndex]

			// Grab the Work resource that the manifest for the AppliedResource exists within, then extract via the ordinal.
			work, err := hubWorkClient.MulticlusterV1alpha1().Works(createdWork.Namespace).Get(context.Background(), cmOwner.Name, metav1.GetOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			resourceManifest := work.Spec.Workload.Manifests[appliedResourceMeta.Ordinal]

			// Unmarshal the data into a struct, modify and then update it.
			var cm v1.ConfigMap
			err = json.Unmarshal(resourceManifest.Raw, &cm)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// Add random new key value pair into map.
			randomNewKey := utilrand.String(5)
			randomNewValue := utilrand.String(5)
			cm.Data[randomNewKey] = randomNewValue

			// Update the manifest value.
			rawManifest, merr := json.Marshal(cm)
			gomega.Expect(merr).ToNot(gomega.HaveOccurred())
			manifest := workapi.Manifest{}
			manifest.Raw = rawManifest
			work.Spec.Workload.Manifests[appliedResourceMeta.Ordinal] = manifest

			// Update the Work resource.
			_, updateErr := hubWorkClient.MulticlusterV1alpha1().Works(createdWork.Namespace).Update(context.Background(), work, metav1.UpdateOptions{})
			gomega.Expect(updateErr).ToNot(gomega.HaveOccurred())

			// Vet
			gomega.Eventually(func() bool {
				configMap, err := spokeKubeClient.CoreV1().ConfigMaps("default").Get(context.Background(), "test-configmap", metav1.GetOptions{})
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				return configMap.Data[randomNewKey] == randomNewValue
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			// Reset
			err = deleteWork(createdWork)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
	})
})

func createWork(manifestFiles []string) (*workapi.Work, error) {
	workName := "work-" + utilrand.String(5)
	workNamespace := "default"

	work := &workapi.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workName,
			Namespace: workNamespace,
		},
		Spec: workapi.WorkSpec{
			Workload: workapi.WorkloadTemplate{
				Manifests: []workapi.Manifest{},
			},
		},
	}

	addManifestsToWorkSpec(manifestFiles, &work.Spec)
	createdWork, err := hubWorkClient.MulticlusterV1alpha1().Works(work.Namespace).Create(context.Background(), work, metav1.CreateOptions{})
	return createdWork, err
}

func deleteWork(work *workapi.Work) error {
	err := hubWorkClient.MulticlusterV1alpha1().Works(work.Namespace).Delete(context.Background(), work.Name, metav1.DeleteOptions{})
	return err
}

func addManifestsToWorkSpec(manifestFileRelativePaths []string, workSpec *workapi.WorkSpec) {
	for _, file := range manifestFileRelativePaths {
		fileRaw, err := testManifestFiles.ReadFile(file)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		obj, _, err := genericCodec.Decode(fileRaw, nil, nil)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		workSpec.Workload.Manifests = append(
			workSpec.Workload.Manifests, workapi.Manifest{
				RawExtension: runtime.RawExtension{Object: obj},
			})
	}
}
