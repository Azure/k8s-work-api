package e2e

import (
	"context"
	v1 "k8s.io/api/core/v1"

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

type manifestDetails struct {
	Manifest workapi.Manifest
	GVK      *schema.GroupVersionKind
	ObjMeta  metav1.ObjectMeta
}

const (
	eventuallyTimeout  = 60 // seconds
	eventuallyInterval = 1  // seconds
	workNamePrefix     = "work-"
)

var _ = Describe("Work creation", func() {

	WorkCreatedContext(
		"with a Work resource that has two manifests: Deployment & Service",
		[]string{
			"manifests/test-deployment.yaml",
			"manifests/test-service.yaml",
		})

	WorkCreatedWithCRDContext(
		"with a CRD manifest",
		[]string{
			"manifests/test-crd.yaml",
		})
})

var _ = Describe("Work modification", func() {
	WorkUpdateWithDependencyContext(
		"with two newly added manifests: configmap & namespace",
		[]string{
			"manifests/test-secret.yaml",
			"manifests/test-configmap.ns.yaml",
			"manifests/test-namespace.yaml",
		})

	WorkUpdateWithModifiedManifestContext(
		"with a modified manifest",
		[]string{
			"manifests/test-configmap.yaml",
		})
})

var _ = Describe("Work deletion", func() {
	WorkDeletedContext(
		"with a deletion request",
		[]string{
			"manifests/test-secret.yaml",
		})
})

var WorkCreatedContext = func(description string, manifestFiles []string) bool {
	return Context(description, func() {
		var createdWork *workapi.Work
		var err error
		manifestDetails := generateManifestDetails(manifestFiles)

		BeforeEach(func() {
			workObj := createWorkObj(
				utilrand.String(5),
				"default",
				manifestDetails,
			)

			createdWork, err = createWorkResource(workObj)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err = deleteWorkResource(createdWork.Namespace, createdWork.Name)
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
				_, err := spokeKubeClient.AppsV1().Deployments(manifestDetails[0].ObjMeta.Namespace).
					Get(context.Background(), manifestDetails[0].ObjMeta.Name, metav1.GetOptions{})

				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
		})
		It("should have created a service", func() {
			Eventually(func() error {
				_, err := spokeKubeClient.CoreV1().Services(manifestDetails[1].ObjMeta.Namespace).
					Get(context.Background(), manifestDetails[1].ObjMeta.Name, metav1.GetOptions{})

				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
		})
	})
}

var WorkCreatedWithCRDContext = func(description string, manifestFiles []string) bool {
	return Context(description, func() {
		var createdWork *workapi.Work
		var err error
		manifestDetails := generateManifestDetails(manifestFiles)

		BeforeEach(func() {
			workObj := createWorkObj(
				utilrand.String(5),
				"default",
				manifestDetails,
			)

			createdWork, err = createWorkResource(workObj)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err = deleteWorkResource(createdWork.Namespace, createdWork.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have created the CRD within the spoke", func() {
			Eventually(func() error {
				_, err = spokeApiExtensionClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), manifestDetails[0].ObjMeta.Name, metav1.GetOptions{})

				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
		})
	})
}

var WorkUpdateWithDependencyContext = func(description string, manifestFiles []string) bool {
	return Context(description, func() {
		var createdWork *workapi.Work
		var err error
		initialManifestDetails := generateManifestDetails(manifestFiles[0:1])
		addedManifestDetails := generateManifestDetails(manifestFiles)

		BeforeEach(func() {
			workObj := createWorkObj(
				utilrand.String(5),
				"default",
				initialManifestDetails,
			)

			createdWork, err = createWorkResource(workObj)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err = deleteWorkResource(createdWork.Namespace, createdWork.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have created the ConfigMap in the new namespace", func() {
			By("retrieving the existing work and updating it by adding new manifests", func() {
				Eventually(func() error {
					createdWork, err = retrieveWork(createdWork.Namespace, createdWork.Name)
					Expect(err).ToNot(HaveOccurred())

					createdWork.Spec.Workload.Manifests = append(createdWork.Spec.Workload.Manifests, addedManifestDetails[1].Manifest, addedManifestDetails[2].Manifest)
					createdWork, err = hubWorkClient.MulticlusterV1alpha1().Works(createdWork.Namespace).Update(context.Background(), createdWork, metav1.UpdateOptions{})

					return err
				}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
			})
			By("checking if the new Namespace was created ", func() {
				Eventually(func() error {
					_, err := spokeKubeClient.CoreV1().Namespaces().Get(context.Background(), addedManifestDetails[2].ObjMeta.Name, metav1.GetOptions{})

					return err
				}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
			})

			By("checking if the ConfigMap was created in the new namespace", func() {
				Eventually(func() error {
					_, err := spokeKubeClient.CoreV1().ConfigMaps(addedManifestDetails[1].ObjMeta.Namespace).Get(context.Background(), addedManifestDetails[1].ObjMeta.Name, metav1.GetOptions{})

					return err
				}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
			})
		})
	})
}

var WorkUpdateWithModifiedManifestContext = func(description string, manifestFiles []string) bool {
	return Context(description, func() {
		var configMap v1.ConfigMap
		var createdWork *workapi.Work
		var err error
		newDataKey := utilrand.String(5)
		newDataValue := utilrand.String(5)

		manifestDetails := generateManifestDetails(manifestFiles)

		BeforeEach(func() {
			workObj := createWorkObj(
				utilrand.String(5),
				"default",
				manifestDetails,
			)

			createdWork, err = createWorkResource(workObj)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err = deleteWorkResource(createdWork.Namespace, createdWork.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should reapply the manifest's updated spec on the spoke cluster", func() {
			By("retrieving the existing work and modifying the manifest", func() {
				Eventually(func() error {
					createdWork, err = retrieveWork(createdWork.Namespace, createdWork.Name)

					// Extract and modify the ConfigMap by adding a new key value pair.
					err = json.Unmarshal(createdWork.Spec.Workload.Manifests[0].Raw, &configMap)
					configMap.Data[newDataKey] = newDataValue

					rawUpdatedManifest, _ := json.Marshal(configMap)

					obj, _, _ := genericCodec.Decode(rawUpdatedManifest, nil, nil)

					createdWork.Spec.Workload.Manifests[0].Object = obj
					createdWork.Spec.Workload.Manifests[0].Raw = rawUpdatedManifest

					createdWork, err = hubWorkClient.MulticlusterV1alpha1().Works(createdWork.Namespace).Update(context.Background(), createdWork, metav1.UpdateOptions{})

					return err
				}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
			})

			By("verifying if the manifest was reapplied", func() {
				Eventually(func() bool {
					configMap, _ := spokeKubeClient.CoreV1().ConfigMaps(manifestDetails[0].ObjMeta.Namespace).Get(context.Background(), manifestDetails[0].ObjMeta.Name, metav1.GetOptions{})

					return configMap.Data[newDataKey] == newDataValue
				}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
			})
		})
	})
}

var WorkDeletedContext = func(description string, manifestFiles []string) bool {
	return Context(description, func() {
		var createdWork *workapi.Work
		var err error
		manifestDetails := generateManifestDetails(manifestFiles)

		BeforeEach(func() {
			workObj := createWorkObj(
				utilrand.String(5),
				"default",
				manifestDetails,
			)

			createdWork, err = createWorkResource(workObj)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err = deleteWorkResource(createdWork.Namespace, createdWork.Name)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should delete the Work and verify the resource has been garbage collected", func() {
			By("verifying the manifest was applied", func() {
				Eventually(func() error {
					_, err = spokeKubeClient.CoreV1().Secrets(manifestDetails[0].ObjMeta.Namespace).Get(context.Background(), manifestDetails[0].ObjMeta.Name, metav1.GetOptions{})

					return err
				}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
			})

			By("deleting the Work resource", func() {
				err = deleteWorkResource(createdWork.Namespace, createdWork.Name)
				Expect(err).ToNot(HaveOccurred())
			})

			By("verifying the resource was garbage collected", func() {
				Eventually(func() error {
					err = spokeKubeClient.CoreV1().Secrets(manifestDetails[0].ObjMeta.Namespace).Delete(context.Background(), manifestDetails[0].ObjMeta.Name, metav1.DeleteOptions{})

					return err
				}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
			})
		})

	})
}

func createWorkObj(workName string, workNamespace string, manifestDetails []manifestDetails) *workapi.Work {
	work := &workapi.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workName,
			Namespace: workNamespace,
		},
	}

	for _, detail := range manifestDetails {
		work.Spec.Workload.Manifests = append(work.Spec.Workload.Manifests, detail.Manifest)
	}

	return work
}
func createWorkResource(work *workapi.Work) (*workapi.Work, error) {
	return hubWorkClient.MulticlusterV1alpha1().Works(work.Namespace).Create(context.Background(), work, metav1.CreateOptions{})
}
func decodeUnstructured(manifest workapi.Manifest) (*unstructured.Unstructured, error) {
	unstructuredObj := &unstructured.Unstructured{}
	err := unstructuredObj.UnmarshalJSON(manifest.Raw)

	return unstructuredObj, err
}
func deleteWorkResource(namespace string, name string) error {
	return hubWorkClient.MulticlusterV1alpha1().Works(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

func generateManifestDetails(manifestFiles []string) []manifestDetails {
	var details []manifestDetails

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

func retrieveWork(namespace string, name string) (*workapi.Work, error) {
	return hubWorkClient.MulticlusterV1alpha1().Works(namespace).Get(context.Background(), name, metav1.GetOptions{})
}
