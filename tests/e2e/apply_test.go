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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	workapi "sigs.k8s.io/work-api/pkg/apis/v1alpha1"
)

const (
	eventuallyTimeout  = 60 // seconds
	eventuallyInterval = 1  // seconds
)

var (
	appliedWork            *workapi.AppliedWork
	createdWork            *workapi.Work
	createError            error
	deleteError            error
	getError               error
	updateError            error
	manifests              []string
	manifestMetaName       []string
	manifestMetaNamespaces []string
	workName               string
	workNamespace          string

	_ = Describe("Work", func() {
		manifestMetaName = []string{"test-nginx", "test-nginx", "test-configmap"} //Todo - Unmarshal raw file bytes into JSON, extract key / values programmatically.
		manifestMetaNamespaces = []string{"default", "default", "default"}        //Todo - Unmarshal raw file bytes into JSON, extract key / values programmatically.

		// The Manifests' ordinal must be respected; some tests reference by ordinal.
		manifests = []string{
			"testmanifests/test-deployment.yaml",
			"testmanifests/test-service.yaml",
			"testmanifests/test-configmap.yaml",
		}

		BeforeEach(func() {
			workName = "work-" + utilrand.String(5)
			workNamespace = "default"

			createdWork, createError = createWork(workName, workNamespace, manifests)
			Expect(createError).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			deleteError = safeDeleteWork(createdWork)
		})

		Describe("created on the Hub", func() {
			// Todo - It would be better to have context of N manifest, and let the type be programmatically determined.
			Context("with a Service & Deployment & Configmap manifest", func() {
				It("should have a Work resource in the hub", func() {
					Eventually(func() error {
						_, err := retrieveWork(createdWork.Namespace, createdWork.Name)

						return err
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
				})
				It("should have an AppliedWork resource in the spoke ", func() {
					Eventually(func() error {
						appliedWork := workapi.AppliedWork{}
						err := spokeClient.Get(context.Background(), types.NamespacedName{
							Namespace: workNamespace,
							Name:      workName,
						}, &appliedWork)

						return err
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
				})
				It("should have created a Kubernetes deployment", func() {
					Eventually(func() error {
						_, err := spokeKubeClient.AppsV1().Deployments(manifestMetaNamespaces[0]).Get(context.Background(), manifestMetaName[0], metav1.GetOptions{})

						return err
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
				})
				It("should have created a Kubernetes service", func() {
					Eventually(func() error {
						_, err := spokeKubeClient.CoreV1().Services(manifestMetaNamespaces[1]).Get(context.Background(), manifestMetaName[1], metav1.GetOptions{})

						return err
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
				})
				It("should have created a ConfigMap", func() {
					Eventually(func() error {
						_, err := spokeKubeClient.CoreV1().ConfigMaps(manifestMetaNamespaces[2]).Get(context.Background(), manifestMetaName[2], metav1.GetOptions{})

						return err
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
				})
			})
		})

		Describe("updated on the Hub", func() {
			Context("with two new configmap & namespace manifests, where the configmap is dependent upon the namespace", func() {
				// The order of these appended manifests is intentional. The test ensures that the configmap
				// will eventually get created. The first attempt will fail as the namespace is created after.
				manifests = append(manifests, "testmanifests/test-testns.configmap.yaml")
				manifestMetaName = append(manifestMetaName, "test-configmap")
				manifestMetaNamespaces = append(manifestMetaNamespaces, "test-namespace")

				manifests = append(manifests, "testmanifests/test-namespace.yaml")
				manifestMetaName = append(manifestMetaName, "test-namespace")
				manifestMetaNamespaces = append(manifestMetaNamespaces, "")

				BeforeEach(func() {
					var work *workapi.Work
					var err error

					By("getting the existing Work resource on the hub")
					Eventually(func() error {
						work, err = retrieveWork(createdWork.Namespace, createdWork.Name)
						return err
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())

					By("adding the new manifest to the Work resource and updating it on the hub")
					Eventually(func() error {
						addManifestsToWorkSpec([]string{manifests[3], manifests[4]}, &work.Spec)
						_, err = updateWork(work)
						return err
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
				})

				It("should have created the namespace", func() {
					Eventually(func() error {
						_, err := spokeKubeClient.CoreV1().Namespaces().Get(context.Background(), manifestMetaName[4], metav1.GetOptions{})

						return err
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
				})
				It("should have created the ConfigMap in the new namespace", func() {
					Eventually(func() error {
						_, err := spokeKubeClient.CoreV1().ConfigMaps(manifestMetaNamespaces[3]).Get(context.Background(), manifestMetaName[3], metav1.GetOptions{})

						return err
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
				})
			})
			Context("with a modified manifest", func() {
				// Todo, refactor this context to not use "over complicated structure".
				var newDataKey string
				var newDataValue string
				var configMapName string
				var configMapNamespace string

				BeforeEach(func() {
					var cm v1.ConfigMap
					configManifest := createdWork.Spec.Workload.Manifests[2]
					// Unmarshal the data into a struct, modify and then update it.
					err := json.Unmarshal(configManifest.Raw, &cm)
					Expect(err).ToNot(HaveOccurred())
					configMapName = cm.Name
					configMapNamespace = cm.Namespace

					err = spokeKubeClient.CoreV1().ConfigMaps(configMapNamespace).Delete(context.Background(), configMapName, metav1.DeleteOptions{})
					Expect(err).ToNot(HaveOccurred())

					// Add random new key value pair into map.
					newDataKey = utilrand.String(5)
					newDataValue = utilrand.String(5)
					cm.Data[newDataKey] = newDataValue
					rawManifest, err := json.Marshal(cm)
					Expect(err).ToNot(HaveOccurred())
					updatedManifest := workapi.Manifest{}
					updatedManifest.Raw = rawManifest
					createdWork.Spec.Workload.Manifests[2] = updatedManifest

					By("updating a manifest specification within the existing Work on the hub")
					createdWork, updateError = updateWork(createdWork)
					Expect(updateError).ToNot(HaveOccurred())
				})

				It("should reapply the manifest.", func() {
					Eventually(func() bool {
						configMap, _ := spokeKubeClient.CoreV1().ConfigMaps(configMapNamespace).Get(context.Background(), configMapName, metav1.GetOptions{})

						return configMap.Data[newDataKey] == newDataValue
					}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
				})
			})
			Context("with a added, modified, and removed manifest", func() {
				// Todo, refactor this context to not use "over complicated structure".
				var cm v1.ConfigMap
				var err error
				var newDataKey string
				var newDataValue string
				var configMapName string
				var configMapNamespace string
				var namespaceToDelete string

				BeforeEach(func() {
					err = spokeKubeClient.CoreV1().ConfigMaps("default").Delete(context.Background(), "test-configmap", metav1.DeleteOptions{})
					Expect(err).ToNot(HaveOccurred())
				})

				It("should create a secret, modify the existing configmap, and remove the second configmap, from within the spoke", func() {
					By("getting the existing Work resource on the hub")
					Eventually(func() error {
						createdWork, err = retrieveWork(createdWork.Namespace, createdWork.Name)
						return err
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())

					By("removing the test-namespace manifest")
					namespaceToDelete = manifestMetaName[4]
					manifests = manifests[:4]
					manifestMetaName = manifestMetaName[:4]
					manifestMetaNamespaces = manifestMetaNamespaces[:4]
					createdWork.Spec.Workload.Manifests = createdWork.Spec.Workload.Manifests[:4]

					By("modifying the existing configmap's manifest values")
					configManifest := createdWork.Spec.Workload.Manifests[2]
					// Unmarshal the data into a struct, modify and then update it.
					err := json.Unmarshal(configManifest.Raw, &cm)
					Expect(err).ToNot(HaveOccurred())
					configMapName = cm.Name
					configMapNamespace = cm.Namespace

					// Add random new key value pair into map.
					newDataKey = utilrand.String(5)
					newDataValue = utilrand.String(5)
					cm.Data[newDataKey] = newDataValue
					rawManifest, err := json.Marshal(cm)
					Expect(err).ToNot(HaveOccurred())
					updatedManifest := workapi.Manifest{}
					updatedManifest.Raw = rawManifest
					createdWork.Spec.Workload.Manifests[2] = updatedManifest

					By("adding a secret manifest")
					manifests = append(manifests, "testmanifests/test-secret.yaml")
					manifestMetaName = append(manifestMetaName, "test-secret")
					manifestMetaNamespaces = append(manifestMetaNamespaces, "default")
					addManifestsToWorkSpec([]string{manifests[4]}, &createdWork.Spec)

					By("updating all manifest changes to the Work resource in the hub")
					createdWork, updateError = updateWork(createdWork)
					Expect(updateError).ToNot(HaveOccurred())

					By("verifying that modified configmap was updated in the spoke")
					Eventually(func() bool {
						configMap, _ := spokeKubeClient.CoreV1().ConfigMaps(configMapNamespace).Get(context.Background(), configMapName, metav1.GetOptions{})

						return configMap.Data[newDataKey] == newDataValue
					}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())

					By("verifying that the test-namespace was deleted")
					Eventually(func() error {
						_, err := spokeKubeClient.CoreV1().Namespaces().Get(context.Background(), namespaceToDelete, metav1.GetOptions{})

						return err
					}, eventuallyTimeout, eventuallyInterval).Should(HaveOccurred())

					By("verifying that new secret was created in the spoke")
					Eventually(func() error {
						_, err := spokeKubeClient.CoreV1().Secrets(manifestMetaNamespaces[4]).Get(context.Background(), manifestMetaName[4], metav1.GetOptions{})

						return err
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
				})
			})
			Context("with a CRD manifest", func() {
				It("should create the CRD on the spoke", func() {
					manifests = append(manifests, "testmanifests/test-crd.yaml")
					manifestMetaName = append(manifestMetaName, "testcrds.multicluster.x-k8s.io")
					manifestMetaNamespaces = append(manifestMetaNamespaces, "default")

					By("getting the existing Work resource on the hub")
					Eventually(func() error {
						createdWork, getError = retrieveWork(createdWork.Namespace, createdWork.Name)

						return getError
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())

					By("adding the new manifest to the Work resource and updating it on the hub")
					Eventually(func() error {
						addManifestsToWorkSpec([]string{manifests[5]}, &createdWork.Spec)
						_, updateError = updateWork(createdWork)

						return updateError
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())

					Eventually(func() error {
						_, getError = spokeApiExtensionClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), manifestMetaName[5], metav1.GetOptions{})

						return getError
					}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
				})
			})
			Context("with all new manifests", func() {
				It("should delete all previously applied resources", func() {
					manifests = []string{"testmanifests/test-serviceaccount.yaml"}
					manifestMetaName = []string{"test-serviceaccount"}
					manifestMetaNamespaces = []string{"default"}
					aResourceStillExists := true

					// Get the current AppliedWork, so we can verify the previously applied resources have been deleted.
					time.Sleep(3 * time.Second) // Time for the AppliedWork to be updated by the reconciler.
					appliedWork, getError = spokeWorkClient.MulticlusterV1alpha1().AppliedWorks().Get(context.Background(), createdWork.Name, metav1.GetOptions{})

					// Get the Work resource and update replace its manifests.
					createdWork, getError = hubWorkClient.MulticlusterV1alpha1().Works(createdWork.Namespace).Get(context.Background(), createdWork.Name, metav1.GetOptions{})
					Expect(getError).ShouldNot(HaveOccurred())

					createdWork.Spec.Workload.Manifests = nil
					addManifestsToWorkSpec([]string{manifests[0]}, &createdWork.Spec)
					createdWork, updateError = hubWorkClient.MulticlusterV1alpha1().Works(createdWork.Namespace).Update(context.Background(), createdWork, metav1.UpdateOptions{})
					Expect(updateError).ShouldNot(HaveOccurred())

					By("checking to see if all previously applied works have been deleted", func() {
						Eventually(func() bool {
							for aResourceStillExists == true {
								for _, ar := range appliedWork.Status.AppliedResources {
									gvr := schema.GroupVersionResource{
										Group:    ar.Group,
										Version:  ar.Version,
										Resource: ar.Resource,
									}

									_, getError = spokeDynamicClient.Resource(gvr).Namespace(ar.Namespace).Get(context.Background(), ar.Name, metav1.GetOptions{})
									if getError != nil {
										aResourceStillExists = false
									} else {
										aResourceStillExists = true
									}
								}
							}
							return aResourceStillExists
						}, eventuallyTimeout, eventuallyInterval).ShouldNot(BeTrue())
					})

					By("verifying the new manifest was applied", func() {
						Eventually(func() error {
							_, getError = spokeKubeClient.CoreV1().ServiceAccounts(manifestMetaNamespaces[0]).Get(context.Background(), manifestMetaName[0], metav1.GetOptions{})

							return getError
						}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())
					})
				})
			})
		})

		Describe("deleted from the Hub", func() {
			BeforeEach(func() {
				// Todo - Replace with Eventually.
				time.Sleep(2 * time.Second) // Give time for AppliedWork to be created.
				// Grab the AppliedWork, so resource garbage collection can be verified.
				appliedWork, getError = retrieveAppliedWork(createdWork.Name)
				Expect(getError).ToNot(HaveOccurred())
				deleteError = safeDeleteWork(createdWork)
				Expect(deleteError).ToNot(HaveOccurred())
			})

			It("should have deleted the Work resource on the hub", func() {
				Eventually(func() error {
					_, err := retrieveWork(workNamespace, workName)

					return err
				}, eventuallyTimeout, eventuallyInterval).Should(HaveOccurred())
			})
			It("should have deleted the resources from the spoke", func() {
				Eventually(func() bool {
					garbageCollectionComplete := true
					for _, resourceMeta := range appliedWork.Status.AppliedResources {
						gvr := schema.GroupVersionResource{
							Group:    resourceMeta.Group,
							Version:  resourceMeta.Version,
							Resource: resourceMeta.Resource,
						}
						_, err := spokeDynamicClient.Resource(gvr).Get(context.Background(), resourceMeta.Name, metav1.GetOptions{})

						if err == nil {
							garbageCollectionComplete = false
							break
						}
					}

					return garbageCollectionComplete
				}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
			})
			It("should have deleted the AppliedWork resource from the spoke", func() {
				Eventually(func() error {
					_, err := retrieveAppliedWork(createdWork.Name)

					return err
				}, eventuallyTimeout, eventuallyInterval).Should(HaveOccurred())
			})
		})
	})
)

func addManifestsToWorkSpec(manifestFileRelativePaths []string, workSpec *workapi.WorkSpec) {
	for _, file := range manifestFileRelativePaths {
		fileRaw, err := testManifestFiles.ReadFile(file)
		Expect(err).ToNot(HaveOccurred())

		obj, _, err := genericCodec.Decode(fileRaw, nil, nil)
		Expect(err).ToNot(HaveOccurred())

		workSpec.Workload.Manifests = append(
			workSpec.Workload.Manifests, workapi.Manifest{
				RawExtension: runtime.RawExtension{Object: obj},
			})
	}
}
func createWork(workName string, workNamespace string, manifestFiles []string) (*workapi.Work, error) {
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

	newWork := workapi.Work{}
	createError = hubWorkClient.Create(context.Background(), work)
	_ = hubWorkClient.Get(context.Background(), types.NamespacedName{
		Namespace: workNamespace,
		Name:      workName,
	}, &newWork)

	return &newWork, createError
}
func retrieveAppliedWork(resourceName string) (*workapi.AppliedWork, error) {
	retrievedAppliedWork := workapi.AppliedWork{}
	err := spokeClient.Get(context.Background(), types.NamespacedName{Name: resourceName}, &retrievedAppliedWork)
	if err != nil {
		return &retrievedAppliedWork, err
	}
	return &retrievedAppliedWork, nil
}
func retrieveWork(workNamespace string, workName string) (*workapi.Work, error) {
	workRetrieved := workapi.Work{}
	err := hubWorkClient.Get(context.Background(), types.NamespacedName{Namespace: workNamespace, Name: workName}, &workRetrieved)
	if err != nil {
		return nil, err
	}
	return &workRetrieved, nil
}
func safeDeleteWork(work *workapi.Work) error {
	// ToDo - Replace with proper Eventually.
	time.Sleep(1 * time.Second)
	currentWork := workapi.Work{}
	err := hubWorkClient.Get(context.Background(), types.NamespacedName{Name: work.Name, Namespace: work.Namespace}, &currentWork)
	if err == nil {
		err = hubWorkClient.Delete(context.Background(), &currentWork)
		if err != nil {
			return err
		}
		return nil
	}
	return err
}
func updateWork(work *workapi.Work) (*workapi.Work, error) {
	err := hubWorkClient.Update(context.Background(), work)
	if err != nil {
		return nil, err
	}

	updatedWork, err := retrieveWork(work.Namespace, work.Name)
	if err != nil {
		return nil, err
	}
	return updatedWork, err
}
