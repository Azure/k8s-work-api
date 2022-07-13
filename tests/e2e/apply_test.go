package e2e

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
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
	workNamePrefix     = "work-"
)

var _ = Describe("Work creation", func() {
	WorkCreatedContext(*generateTestSpec(
		"with a Work resource that has two manifests: Deployment & Service",
		workNamePrefix+utilrand.String(5),
		"default",
		[]string{
			"manifests/test-deployment.yaml",
			"manifests/test-service.yaml",
		}))

	WorkCreatedWithCRDContext(*generateTestSpec(
		"with a CRD manifest",
		workNamePrefix+utilrand.String(5),
		"default",
		[]string{
			"manifests/test-crd.yaml",
		}))
})
var _ = Describe("Work modification", func() {
	WorkUpdateWithDependencyContext(
		*generateTestSpec(
			"with two newly added manifests: configmap & namespace",
			workNamePrefix+utilrand.String(5),
			"default",
			[]string{
				"manifests/test-secret.yaml",
				"manifests/test-configmap.ns.yaml",
				"manifests/test-namespace.yaml",
			}))

	WorkUpdateWithModifiedManifestContext(
		*generateTestSpec(
			"with a modified manifest",
			workNamePrefix+utilrand.String(5),
			"default",
			[]string{
				"manifests/test-configmap.yaml",
			}))
})
var _ = Describe("Work deletion", func() {
	WorkDeletedContext(
		*generateTestSpec(
			"with a deletion request",
			workNamePrefix+utilrand.String(5),
			"default",
			[]string{
				"manifests/test-secret.yaml",
			}))
})

var WorkCreatedContext = func(testSpec testSpec) bool {
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
var WorkCreatedWithCRDContext = func(spec testSpec) bool {
	return Context(spec.Description, func() {
		var err error
		workToCreate := createWorkObjFromTestDetails(spec)

		It("should create a Work resource within the hub", func() {
			By("creating a Work resource")
			_, err = createWorkResource(workToCreate)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have created the CRD within the spoke", func() {
			Eventually(func() error {
				_, err = spokeApiExtensionClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), spec.ManifestDetails[0].ObjMeta.Name, metav1.GetOptions{})

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

		// The initial Work creation should only have 1 manifest, the update will add 2 new manifests.
		initialWorkToCreate.Spec.Workload.Manifests = initialWorkToCreate.Spec.Workload.Manifests[0:1]

		It("should get the existing work and update the spec with the new manifests", func() {
			By("creating the initial work", func() {
				_, err = createWorkResource(initialWorkToCreate)
				Expect(err).ToNot(HaveOccurred())
			})

			By("updating the work with the new manifests", func() {
				Eventually(func() error {
					work, err = retrieveWork(testSpec.WorkNamespace, testSpec.WorkName)
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
var WorkUpdateWithModifiedManifestContext = func(testSpec testSpec) bool {
	return Context(testSpec.Description, func() {
		var work *workapi.Work
		var configMap v1.ConfigMap
		var err error

		newDataKey := utilrand.String(5)
		newDataValue := utilrand.String(5)
		initialWorkToCreate := createWorkObjFromTestDetails(testSpec)

		It("should reapply the manifest's updated spec on the spoke cluster", func() {
			By("creating the initial Work resource on the hub", func() {
				_, err = createWorkResource(initialWorkToCreate)
				Expect(err).ToNot(HaveOccurred())
			})

			By("retrieving the existing work and modifying the manifest", func() {
				Eventually(func() error {
					work, err = retrieveWork(testSpec.WorkNamespace, testSpec.WorkName)

					// Extract and modify the ConfigMap by adding a new key value pair.
					err = json.Unmarshal(work.Spec.Workload.Manifests[0].Raw, &configMap)
					configMap.Data[newDataKey] = newDataValue

					rawUpdatedManifest, _ := json.Marshal(configMap)

					obj, _, _ := genericCodec.Decode(rawUpdatedManifest, nil, nil)

					work.Spec.Workload.Manifests[0].Object = obj
					work.Spec.Workload.Manifests[0].Raw = rawUpdatedManifest

					work, err = hubWorkClient.MulticlusterV1alpha1().Works(testSpec.WorkNamespace).Update(context.Background(), work, metav1.UpdateOptions{})

					return err
				}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
			})

			By("verifying if the manifest was reapplied", func() {
				Eventually(func() bool {
					configMap, _ := spokeKubeClient.CoreV1().ConfigMaps(testSpec.ManifestDetails[0].ObjMeta.Namespace).Get(context.Background(), testSpec.ManifestDetails[0].ObjMeta.Name, metav1.GetOptions{})

					return configMap.Data[newDataKey] == newDataValue
				}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
			})
		})
	})
}
var WorkDeletedContext = func(testSpec testSpec) bool {
	return Context(testSpec.Description, func() {
		var err error

		workToCreate := createWorkObjFromTestDetails(testSpec)
		It("should delete the Work and verify the resource has been garbage collected", func() {
			By("creating a Work resource")
			_, err = createWorkResource(workToCreate)
			Expect(err).ToNot(HaveOccurred())

			By("verifying the manifest was applied", func() {
				_, err = spokeKubeClient.CoreV1().Secrets(testSpec.ManifestDetails[0].ObjMeta.Namespace).Get(context.Background(), testSpec.ManifestDetails[0].ObjMeta.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
			})

			By("deleting the Work resource", func() {
				err = deleteWorkResource(testSpec.WorkNamespace, testSpec.WorkName)
				Expect(err).ToNot(HaveOccurred())
			})

			By("verifying the resource was garbage collected", func() {
				Eventually(func() error {
					err = spokeKubeClient.CoreV1().Secrets(testSpec.ManifestDetails[0].ObjMeta.Namespace).Delete(context.Background(), testSpec.ManifestDetails[0].ObjMeta.Name, metav1.DeleteOptions{})

					return err
				}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
			})
		})

	})
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
func decodeUnstructured(manifest workapi.Manifest) (*unstructured.Unstructured, error) {
	unstructuredObj := &unstructured.Unstructured{}
	err := unstructuredObj.UnmarshalJSON(manifest.Raw)

	return unstructuredObj, err
}
func deleteWorkResource(namespace string, name string) error {
	return hubWorkClient.MulticlusterV1alpha1().Works(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
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
