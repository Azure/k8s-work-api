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
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	workapi "sigs.k8s.io/work-api/pkg/apis/v1alpha1"
)

const (
	eventuallyTimeout  = 60 // seconds
	eventuallyInterval = 1  // seconds
)

var _ = ginkgo.Describe("Apply Work", func() {

	workName := "work-" + utilrand.String(5)
	workNamespace = "default"

	ginkgo.Context("Create a service and deployment", func() {
		ginkgo.It("Should create work successfully", func() {

			manifestFiles := []string{
				"testmanifests/test-deployment.yaml",
				"testmanifests/test-service.yaml",
				"testmanifests/test-configmap.yaml",
			}

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

			_, err := hubWorkClient.MulticlusterV1alpha1().Works(workNamespace).Create(context.Background(), work, metav1.CreateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			gomega.Eventually(func() error {
				_, err := spokeKubeClient.AppsV1().Deployments("default").Get(context.Background(), "test-nginx", metav1.GetOptions{})
				if err != nil {
					return err
				}

				_, err = spokeKubeClient.CoreV1().Services("default").Get(context.Background(), "test-nginx", metav1.GetOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())

			gomega.Eventually(func() error {
				work, err := hubWorkClient.MulticlusterV1alpha1().Works(workNamespace).Get(context.Background(), workName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				if !meta.IsStatusConditionTrue(work.Status.Conditions, "Applied") {
					return fmt.Errorf("Expect the applied contidion of the work is true")
				}

				return nil
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())
		})
	})
	ginkgo.Context("Add a manifest to an existing work", func() {
		ginkgo.It("Should create new resources successfully.", func() {
			manifestFiles := []string{
				"testmanifests/test-deployment2.yaml",
				"testmanifests/test-service2.yaml",
			}

			// Get existing work, then add the new manifests
			work, err := hubWorkClient.MulticlusterV1alpha1().Works(workNamespace).Get(context.Background(), workName, metav1.GetOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(work).ToNot(gomega.BeNil())

			addManifestsToWorkSpec(manifestFiles, &work.Spec)

			_, err = hubWorkClient.MulticlusterV1alpha1().Works(workNamespace).Update(context.Background(), work, metav1.UpdateOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			gomega.Eventually(func() error {
				_, err := spokeKubeClient.AppsV1().Deployments("default").Get(context.Background(), "test-nginx2", metav1.GetOptions{})
				if err != nil {
					return err
				}

				_, err = spokeKubeClient.CoreV1().Services("default").Get(context.Background(), "test-nginx2", metav1.GetOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())

			gomega.Eventually(func() error {
				work, err := hubWorkClient.MulticlusterV1alpha1().Works(workNamespace).Get(context.Background(), workName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				if !meta.IsStatusConditionTrue(work.Status.Conditions, "Applied") {
					return fmt.Errorf("Expect the applied contidion of the work is true")
				}

				return nil
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())
		})
	})
	ginkgo.Context("Modify a field within a manifest", func() {
		ginkgo.It("Should reapply the manifest", func() {
			configMap, err := spokeKubeClient.CoreV1().ConfigMaps("default").Get(context.Background(), "test-configmap", metav1.GetOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			gomega.Expect(configMap).ShouldNot(gomega.BeNil())

			// Retrieve the owner reference for the existing resource, then retrieve the work spec from the hub using its owner reference.
			// Note: The index of 0 can be trusted only due to being in a testing environment.
			cmOwner := configMap.OwnerReferences[0]
			appliedWork, err := spokeWorkClient.MulticlusterV1alpha1().AppliedWorks().Get(context.Background(), cmOwner.Name, metav1.GetOptions{})
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
			work, err := hubWorkClient.MulticlusterV1alpha1().Works(workNamespace).Get(context.Background(), cmOwner.Name, metav1.GetOptions{})
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
			_, updateErr := hubWorkClient.MulticlusterV1alpha1().Works(workNamespace).Update(context.Background(), work, metav1.UpdateOptions{})
			gomega.Expect(updateErr).ToNot(gomega.HaveOccurred())

			// Verify the new ConfigMap manifest is reapplied on the spoke cluster.
			gomega.Eventually(func() bool {
				configMap, err := spokeKubeClient.CoreV1().ConfigMaps("default").Get(context.Background(), "test-configmap", metav1.GetOptions{})
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				return configMap.Data[randomNewKey] == randomNewValue
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
		})
	})
})

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
