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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	workapi "sigs.k8s.io/work-api/pkg/apis/v1alpha1"
)

type testSpec struct {
	Description     string
	WorkName        string
	WorkNamespace   string
	ManifestDetails []manifestDetails
}

type manifestDetails struct {
	Manifest workapi.Manifest
	GVK      *schema.GroupVersionKind
	ObjMeta  metav1.ObjectMeta
}

const (
	eventuallyTimeout  = 60 // seconds
	eventuallyInterval = 1  // seconds
)

var _ = Describe("Test Work creation", func() {

	WorkCreationContext(*generateTestSpec(
		"with a Work resource that has two manifests: Deployment & Service",
		utilrand.String(5),
		"default",
		[]string{
			"manifests/test-deployment.yaml",
			"manifests/test-service.yaml",
		}))
})

var _ = Describe("Test Work modification", func() {
	WorkUpdateWithDependencyContext(
		*generateTestSpec(
			"with a Work resource that has two manifests: Deployment & Service",
			utilrand.String(5),
			"default",
			[]string{
				"manifests/test-secret.yaml",
				"manifests/test-configmap.ns.yaml",
				"manifests/test-namespace.yaml",
			}))
})

var _ = Describe("Test Work removal", func() {

})

var WorkCreationContext = func(testSpec testSpec) bool {
	return Context(testSpec.Description, func() {
		var createdWork *workapi.Work
		var err error

		workToCreate := createWorkObjFromTestDetails(testSpec)
		It("should have created a Work resource within the hub cluster", func() {
			By("creating a Work resource")
			createdWork, err = createWorkResource(workToCreate)
			Expect(err).ToNot(HaveOccurred())
		})
		It("should have created an AppliedWork resource in the spoke ", func() {
			Eventually(func() error {
				_, err := spokeWorkClient.MulticlusterV1alpha1().AppliedWorks().Get(context.Background(), createdWork.Name, metav1.GetOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
		})
		It("should have created a deployment", func() {
			Eventually(func() error {
				_, err := spokeKubeClient.AppsV1().Deployments(testSpec.ManifestDetails[0].ObjMeta.Namespace).
					Get(context.Background(), testSpec.ManifestDetails[0].ObjMeta.Name, metav1.GetOptions{})

				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
		})
		It("should have created a service", func() {
			Eventually(func() error {
				_, err := spokeKubeClient.CoreV1().Services(testSpec.ManifestDetails[1].ObjMeta.Namespace).
					Get(context.Background(), testSpec.ManifestDetails[1].ObjMeta.Name, metav1.GetOptions{})

				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
		})
	})
}

var WorkUpdateWithDependencyContext = func(testSpec testSpec) bool {
	return Context(testSpec.Description, func() {
		var work *workapi.Work
		var err error

		initialWorkToCreate := createWorkObjFromTestDetails(testSpec)
		initialWorkToCreate.Spec.Workload.Manifests = initialWorkToCreate.Spec.Workload.Manifests[0:1]

		It("should get the existing work and update the spec with the new manifests", func() {
			By("creating the initial work", func() {
				_, err = createWorkResource(initialWorkToCreate)
				Expect(err).ToNot(HaveOccurred())
			})

			By("updating the work with the new manifests", func() {
				Eventually(func() error {
					work, err = hubWorkClient.MulticlusterV1alpha1().Works(testSpec.WorkNamespace).Get(context.Background(), testSpec.WorkName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())

					work.Spec.Workload.Manifests = append(work.Spec.Workload.Manifests, testSpec.ManifestDetails[1].Manifest, testSpec.ManifestDetails[2].Manifest)
					work, err = hubWorkClient.MulticlusterV1alpha1().Works(testSpec.WorkNamespace).Update(context.Background(), work, metav1.UpdateOptions{})

					return err
				}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
			})
		})

		It("should have created the ConfigMap in the new namespace", func() {
			Eventually(func() error {
				_, err := spokeKubeClient.CoreV1().ConfigMaps(testSpec.ManifestDetails[1].ObjMeta.Namespace).Get(context.Background(), testSpec.ManifestDetails[1].ObjMeta.Name, metav1.GetOptions{})

				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
		})

		It("should have created the namespace", func() {
			Eventually(func() error {
				_, err := spokeKubeClient.CoreV1().Namespaces().Get(context.Background(), testSpec.ManifestDetails[2].ObjMeta.Name, metav1.GetOptions{})

				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
		})
	})
}

func generateTestSpec(description string, workName string, workNamespace string, manifestFiles []string) *testSpec {
	return &testSpec{
		Description:     description,
		WorkName:        workName,
		WorkNamespace:   workNamespace,
		ManifestDetails: generateManifestDetails(manifestFiles),
	}
}

func generateManifestDetails(manifestFiles []string) []manifestDetails {
	details := []manifestDetails{}

	for _, file := range manifestFiles {
		detail := manifestDetails{}

		// Read files, create manifest
		fileRaw, err := testManifestFiles.ReadFile(file)
		Expect(err).ToNot(HaveOccurred())

		obj, gvk, err := genericCodec.Decode(fileRaw, nil, nil)
		Expect(err).ToNot(HaveOccurred())

		jsonObj, err := json.Marshal(obj)
		Expect(err).ToNot(HaveOccurred())

		detail.Manifest = workapi.Manifest{
			RawExtension: runtime.RawExtension{
				Object: obj,
				Raw:    jsonObj},
		}

		rawObj, err := decodeUnstructured(detail.Manifest)
		Expect(err).ShouldNot(HaveOccurred())

		detail.GVK = gvk
		detail.ObjMeta = metav1.ObjectMeta{
			Name:      rawObj.GetName(),
			Namespace: rawObj.GetNamespace(),
		}

		details = append(details, detail)
	}

	return details
}

func decodeUnstructured(manifest workapi.Manifest) (*unstructured.Unstructured, error) {
	unstructuredObj := &unstructured.Unstructured{}
	err := unstructuredObj.UnmarshalJSON(manifest.Raw)

	return unstructuredObj, err
}

func createWorkObjFromTestDetails(testSpec testSpec) *workapi.Work {
	work := &workapi.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSpec.WorkName,
			Namespace: testSpec.WorkNamespace,
		},
	}

	for _, mDetails := range testSpec.ManifestDetails {
		work.Spec.Workload.Manifests = append(work.Spec.Workload.Manifests, mDetails.Manifest)
	}

	return work
}

func createWorkResource(work *workapi.Work) (*workapi.Work, error) {
	return hubWorkClient.MulticlusterV1alpha1().Works(work.Namespace).Create(context.Background(), work, metav1.CreateOptions{})
}

//func retrieveWork(workNamespace string, workName string) (*workapi.Work, error) {
//	return hubWorkClient.MulticlusterV1alpha1().Works(workNamespace).Get(context.Background(), workName, metav1.GetOptions{})
//}
