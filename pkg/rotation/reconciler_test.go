/*
Copyright 2020 The Kubernetes Authors.

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

package rotation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/client-go/tools/record"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	v1 "k8s.io/api/core/v1"

	. "github.com/onsi/gomega"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"k8s.io/client-go/kubernetes/fake"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/controllers"
	secretsStoreFakeClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/k8s"
	secretsstore "sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store"
	providerfake "sigs.k8s.io/secrets-store-csi-driver/provider/fake"
)

var (
	fakeRecorder = record.NewFakeRecorder(20)
)

func setupScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	return scheme, nil
}

func newTestReconciler(client client.Reader, s *runtime.Scheme, kubeClient kubernetes.Interface, crdClient *secretsStoreFakeClient.Clientset, rotationPollInterval time.Duration, socketPath string, filteredWatchSecret bool) (*Reconciler, error) {
	secretStore, err := k8s.New(kubeClient, 5*time.Second, filteredWatchSecret)
	if err != nil {
		return nil, err
	}

	return &Reconciler{
		providerVolumePath:   socketPath,
		rotationPollInterval: rotationPollInterval,
		providerClients:      secretsstore.NewPluginClientBuilder(socketPath),
		queue:                workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		reporter:             newStatsReporter(),
		eventRecorder:        fakeRecorder,
		kubeClient:           kubeClient,
		crdClient:            crdClient,
		cache:                client,
		secretStore:          secretStore,
	}, nil
}

func TestReconcileError(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name                                  string
		rotationPollInterval                  time.Duration
		secretProviderClassPodStatusToProcess *v1alpha1.SecretProviderClassPodStatus
		secretProviderClassToAdd              *v1alpha1.SecretProviderClass
		podToAdd                              *v1.Pod
		socketPath                            string
		secretToAdd                           *v1.Secret
		expectedObjectVersions                map[string]string
		expectedErr                           bool
		expectedErrorEvents                   bool
	}{
		{
			name:                 "secret provider class not found",
			rotationPollInterval: 60 * time.Second,
			secretProviderClassPodStatusToProcess: &v1alpha1.SecretProviderClassPodStatus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1-default-spc1",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.InternalNodeLabel: "nodeName"},
				},
				Status: v1alpha1.SecretProviderClassPodStatusStatus{
					SecretProviderClassName: "spc1",
					PodName:                 "pod1",
				},
			},
			secretProviderClassToAdd: &v1alpha1.SecretProviderClass{},
			podToAdd:                 &v1.Pod{},
			socketPath:               getTempTestDir(t),
			secretToAdd:              &v1.Secret{},
			expectedErr:              true,
		},
		{
			name:                 "failed to get pod",
			rotationPollInterval: 60 * time.Second,
			secretProviderClassPodStatusToProcess: &v1alpha1.SecretProviderClassPodStatus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1-default-spc1",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.InternalNodeLabel: "nodeName"},
				},
				Status: v1alpha1.SecretProviderClassPodStatusStatus{
					SecretProviderClassName: "spc1",
					PodName:                 "pod1",
				},
			},
			secretProviderClassToAdd: &v1alpha1.SecretProviderClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "spc1",
					Namespace: "default",
				},
				Spec: v1alpha1.SecretProviderClassSpec{
					SecretObjects: []*v1alpha1.SecretObject{
						{
							Data: []*v1alpha1.SecretObjectData{
								{
									ObjectName: "object1",
									Key:        "foo",
								},
							},
						},
					},
				},
			},
			podToAdd:    &v1.Pod{},
			socketPath:  getTempTestDir(t),
			secretToAdd: &v1.Secret{},
			expectedErr: true,
		},
		{
			name:                 "failed to get NodePublishSecretRef secret",
			rotationPollInterval: 60 * time.Second,
			secretProviderClassPodStatusToProcess: &v1alpha1.SecretProviderClassPodStatus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1-default-spc1",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.InternalNodeLabel: "nodeName"},
				},
				Status: v1alpha1.SecretProviderClassPodStatusStatus{
					SecretProviderClassName: "spc1",
					PodName:                 "pod1",
					TargetPath:              getTestTargetPath(t, "foo", "csi-volume"),
				},
			},
			secretProviderClassToAdd: &v1alpha1.SecretProviderClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "spc1",
					Namespace: "default",
				},
				Spec: v1alpha1.SecretProviderClassSpec{
					SecretObjects: []*v1alpha1.SecretObject{
						{
							Data: []*v1alpha1.SecretObjectData{
								{
									ObjectName: "object1",
									Key:        "foo",
								},
							},
						},
					},
					Provider: "provider1",
				},
			},
			podToAdd: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "default",
					UID:       types.UID("foo"),
				},
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "csi-volume",
							VolumeSource: v1.VolumeSource{
								CSI: &v1.CSIVolumeSource{
									Driver:           "secrets-store.csi.k8s.io",
									VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
									NodePublishSecretRef: &v1.LocalObjectReference{
										Name: "secret1",
									},
								},
							},
						},
					},
				},
			},
			socketPath:          getTempTestDir(t),
			secretToAdd:         &v1.Secret{},
			expectedErr:         true,
			expectedErrorEvents: true,
		},
		{
			name:                 "failed to validate targetpath UID",
			rotationPollInterval: 60 * time.Second,
			secretProviderClassPodStatusToProcess: &v1alpha1.SecretProviderClassPodStatus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1-default-spc1",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.InternalNodeLabel: "nodeName"},
				},
				Status: v1alpha1.SecretProviderClassPodStatusStatus{
					SecretProviderClassName: "spc1",
					PodName:                 "pod1",
					TargetPath:              getTestTargetPath(t, "bad-uid", "csi-volume"),
					Objects: []v1alpha1.SecretProviderClassObject{
						{
							ID:      "secret/object1",
							Version: "v1",
						},
					},
				},
			},
			secretProviderClassToAdd: &v1alpha1.SecretProviderClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "spc1",
					Namespace: "default",
				},
				Spec: v1alpha1.SecretProviderClassSpec{
					SecretObjects: []*v1alpha1.SecretObject{
						{
							Data: []*v1alpha1.SecretObjectData{
								{
									ObjectName: "object1",
									Key:        "foo",
								},
							},
						},
					},
					Provider: "provider1",
				},
			},
			podToAdd: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "default",
					UID:       types.UID("foo"),
				},
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "csi-volume",
							VolumeSource: v1.VolumeSource{
								CSI: &v1.CSIVolumeSource{
									Driver:           "secrets-store.csi.k8s.io",
									VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
								},
							},
						},
					},
				},
			},
			socketPath: getTempTestDir(t),
			secretToAdd: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "object1",
					Namespace:       "default",
					ResourceVersion: "rv1",
				},
				Data: map[string][]byte{"foo": []byte("olddata")},
			},
			expectedObjectVersions: map[string]string{"secret/object1": "v2"},
			expectedErr:            true,
			expectedErrorEvents:    false,
		},
		{
			name:                 "failed to validate targetpath volume name",
			rotationPollInterval: 60 * time.Second,
			secretProviderClassPodStatusToProcess: &v1alpha1.SecretProviderClassPodStatus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1-default-spc1",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.InternalNodeLabel: "nodeName"},
				},
				Status: v1alpha1.SecretProviderClassPodStatusStatus{
					SecretProviderClassName: "spc1",
					PodName:                 "pod1",
					TargetPath:              getTestTargetPath(t, "foo", "bad-volume-name"),
					Objects: []v1alpha1.SecretProviderClassObject{
						{
							ID:      "secret/object1",
							Version: "v1",
						},
					},
				},
			},
			secretProviderClassToAdd: &v1alpha1.SecretProviderClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "spc1",
					Namespace: "default",
				},
				Spec: v1alpha1.SecretProviderClassSpec{
					SecretObjects: []*v1alpha1.SecretObject{
						{
							Data: []*v1alpha1.SecretObjectData{
								{
									ObjectName: "object1",
									Key:        "foo",
								},
							},
						},
					},
					Provider: "provider1",
				},
			},
			podToAdd: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "default",
					UID:       types.UID("foo"),
				},
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "csi-volume",
							VolumeSource: v1.VolumeSource{
								CSI: &v1.CSIVolumeSource{
									Driver:           "secrets-store.csi.k8s.io",
									VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
								},
							},
						},
					},
				},
			},
			socketPath: getTempTestDir(t),
			secretToAdd: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "object1",
					Namespace:       "default",
					ResourceVersion: "rv1",
				},
				Data: map[string][]byte{"foo": []byte("olddata")},
			},
			expectedObjectVersions: map[string]string{"secret/object1": "v2"},
			expectedErr:            true,
			expectedErrorEvents:    false,
		},
		{
			name:                 "failed to lookup provider client",
			rotationPollInterval: 60 * time.Second,
			secretProviderClassPodStatusToProcess: &v1alpha1.SecretProviderClassPodStatus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1-default-spc1",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.InternalNodeLabel: "nodeName"},
				},
				Status: v1alpha1.SecretProviderClassPodStatusStatus{
					SecretProviderClassName: "spc1",
					PodName:                 "pod1",
					TargetPath:              getTestTargetPath(t, "foo", "csi-volume"),
				},
			},
			secretProviderClassToAdd: &v1alpha1.SecretProviderClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "spc1",
					Namespace: "default",
				},
				Spec: v1alpha1.SecretProviderClassSpec{
					SecretObjects: []*v1alpha1.SecretObject{
						{
							Data: []*v1alpha1.SecretObjectData{
								{
									ObjectName: "object1",
									Key:        "foo",
								},
							},
						},
					},
					Provider: "wrongprovider",
				},
			},
			podToAdd: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "default",
					UID:       types.UID("foo"),
				},
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "csi-volume",
							VolumeSource: v1.VolumeSource{
								CSI: &v1.CSIVolumeSource{
									Driver:           "secrets-store.csi.k8s.io",
									VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
									NodePublishSecretRef: &v1.LocalObjectReference{
										Name: "secret1",
									},
								},
							},
						},
					},
				},
			},
			socketPath: getTempTestDir(t),
			secretToAdd: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret1",
					Namespace: "default",
				},
				Data: map[string][]byte{"clientid": []byte("clientid")},
			},
			expectedErr:         true,
			expectedErrorEvents: true,
		},
	}

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset(test.podToAdd, test.secretToAdd)
			crdClient := secretsStoreFakeClient.NewSimpleClientset(test.secretProviderClassPodStatusToProcess, test.secretProviderClassToAdd)

			initObjects := []runtime.Object{
				test.podToAdd,
				test.secretToAdd,
				test.secretProviderClassPodStatusToProcess,
				test.secretProviderClassToAdd,
			}
			client := controllerfake.NewFakeClientWithScheme(scheme, initObjects...)

			testReconciler, err := newTestReconciler(client, scheme, kubeClient, crdClient, test.rotationPollInterval, test.socketPath, false)
			g.Expect(err).NotTo(HaveOccurred())

			err = testReconciler.secretStore.Run(wait.NeverStop)
			g.Expect(err).NotTo(HaveOccurred())

			serverEndpoint := fmt.Sprintf("%s/%s.sock", test.socketPath, "provider1")
			defer os.Remove(serverEndpoint)

			server, err := providerfake.NewMocKCSIProviderServer(serverEndpoint)
			g.Expect(err).NotTo(HaveOccurred())
			server.SetObjects(test.expectedObjectVersions)
			server.Start()

			err = testReconciler.reconcile(context.TODO(), test.secretProviderClassPodStatusToProcess)
			g.Expect(err).To(HaveOccurred())
			if test.expectedErrorEvents {
				g.Expect(len(fakeRecorder.Events)).ToNot(BeNumerically("==", 0))
				for len(fakeRecorder.Events) > 0 {
					fmt.Println(<-fakeRecorder.Events)
				}
			}
		})
	}
}

func TestReconcileNoError(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name                            string
		filteredWatchEnabled            bool
		nodePublishSecretRefSecretToAdd *v1.Secret
	}{
		{
			name:                 "filtered watch for nodePublishSecretRef not enabled",
			filteredWatchEnabled: false,
			nodePublishSecretRefSecretToAdd: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret1",
					Namespace: "default",
				},
				Data: map[string][]byte{"clientid": []byte("clientid")},
			},
		},
		{
			name:                 "filtered watch for nodePublishSecretRef enabled",
			filteredWatchEnabled: true,
			nodePublishSecretRefSecretToAdd: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret1",
					Namespace: "default",
					Labels: map[string]string{
						controllers.SecretUsedLabel: "true",
					},
				},
				Data: map[string][]byte{"clientid": []byte("clientid")},
			},
		},
	}

	for _, test := range tests {
		secretProviderClassPodStatusToProcess := &v1alpha1.SecretProviderClassPodStatus{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod1-default-spc1",
				Namespace: "default",
				Labels:    map[string]string{v1alpha1.InternalNodeLabel: "nodeName"},
			},
			Status: v1alpha1.SecretProviderClassPodStatusStatus{
				SecretProviderClassName: "spc1",
				PodName:                 "pod1",
				TargetPath:              getTestTargetPath(t, "foo", "csi-volume"),
				Objects: []v1alpha1.SecretProviderClassObject{
					{
						ID:      "secret/object1",
						Version: "v1",
					},
				},
			},
		}
		secretProviderClassToAdd := &v1alpha1.SecretProviderClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "spc1",
				Namespace: "default",
			},
			Spec: v1alpha1.SecretProviderClassSpec{
				SecretObjects: []*v1alpha1.SecretObject{
					{
						Data: []*v1alpha1.SecretObjectData{
							{
								ObjectName: "object1",
								Key:        "foo",
							},
						},
						SecretName: "foosecret",
						Type:       "Opaque",
					},
				},
				Provider: "provider1",
			},
		}
		podToAdd := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod1",
				Namespace: "default",
				UID:       types.UID("foo"),
			},
			Spec: v1.PodSpec{
				Volumes: []v1.Volume{
					{
						Name: "csi-volume",
						VolumeSource: v1.VolumeSource{
							CSI: &v1.CSIVolumeSource{
								Driver:           "secrets-store.csi.k8s.io",
								VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
								NodePublishSecretRef: &v1.LocalObjectReference{
									Name: "secret1",
								},
							},
						},
					},
				},
			},
		}
		secretToBeRotated := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "foosecret",
				Namespace:       "default",
				ResourceVersion: "12352",
				Labels: map[string]string{
					controllers.SecretManagedLabel: "true",
				},
			},
			Data: map[string][]byte{"foo": []byte("olddata")},
		}

		socketPath := getTempTestDir(t)
		expectedObjectVersions := map[string]string{"secret/object1": "v2"}
		scheme, err := setupScheme()
		g.Expect(err).NotTo(HaveOccurred())

		kubeClient := fake.NewSimpleClientset(podToAdd, test.nodePublishSecretRefSecretToAdd, secretToBeRotated)
		crdClient := secretsStoreFakeClient.NewSimpleClientset(secretProviderClassPodStatusToProcess, secretProviderClassToAdd)

		initObjects := []runtime.Object{
			podToAdd,
			secretToBeRotated,
			test.nodePublishSecretRefSecretToAdd,
			secretProviderClassPodStatusToProcess,
			secretProviderClassToAdd,
		}
		client := controllerfake.NewFakeClientWithScheme(scheme, initObjects...)

		testReconciler, err := newTestReconciler(client, scheme, kubeClient, crdClient, 60*time.Second, socketPath, false)
		g.Expect(err).NotTo(HaveOccurred())
		err = testReconciler.secretStore.Run(wait.NeverStop)
		g.Expect(err).NotTo(HaveOccurred())

		serverEndpoint := fmt.Sprintf("%s/%s.sock", socketPath, "provider1")
		defer os.Remove(serverEndpoint)

		server, err := providerfake.NewMocKCSIProviderServer(serverEndpoint)
		g.Expect(err).NotTo(HaveOccurred())
		server.SetObjects(expectedObjectVersions)
		server.Start()

		err = os.WriteFile(secretProviderClassPodStatusToProcess.Status.TargetPath+"/object1", []byte("newdata"), permission)
		g.Expect(err).NotTo(HaveOccurred())

		err = testReconciler.reconcile(context.TODO(), secretProviderClassPodStatusToProcess)
		g.Expect(err).NotTo(HaveOccurred())

		// validate the secret provider class pod status versions have been updated
		updatedSPCPodStatus, err := crdClient.SecretsstoreV1alpha1().SecretProviderClassPodStatuses(v1.NamespaceDefault).Get(context.TODO(), "pod1-default-spc1", metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(updatedSPCPodStatus.Status.Objects).To(Equal([]v1alpha1.SecretProviderClassObject{{ID: "secret/object1", Version: "v2"}}))

		// validate the secret data has been updated to the latest value
		updatedSecret, err := kubeClient.CoreV1().Secrets(v1.NamespaceDefault).Get(context.TODO(), "foosecret", metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(updatedSecret.Data["foo"]).To(Equal([]byte("newdata")))

		// 2 normal events - one for successfully updating the mounted contents and
		// second for successfully rotating the K8s secret
		g.Expect(len(fakeRecorder.Events)).To(BeNumerically("==", 2))
		for len(fakeRecorder.Events) > 0 {
			<-fakeRecorder.Events
		}

		// test with pod being terminated
		podToAdd.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		kubeClient = fake.NewSimpleClientset(podToAdd, test.nodePublishSecretRefSecretToAdd)
		initObjects = []runtime.Object{
			podToAdd,
			test.nodePublishSecretRefSecretToAdd,
		}
		client = controllerfake.NewFakeClientWithScheme(scheme, initObjects...)
		testReconciler, err = newTestReconciler(client, scheme, kubeClient, crdClient, 60*time.Second, socketPath, false)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(err).NotTo(HaveOccurred())

		err = testReconciler.reconcile(context.TODO(), secretProviderClassPodStatusToProcess)
		g.Expect(err).NotTo(HaveOccurred())

		// test with pod being in succeeded phase
		podToAdd.DeletionTimestamp = nil
		podToAdd.Status.Phase = v1.PodSucceeded
		kubeClient = fake.NewSimpleClientset(podToAdd, test.nodePublishSecretRefSecretToAdd)
		initObjects = []runtime.Object{
			podToAdd,
			test.nodePublishSecretRefSecretToAdd,
		}
		client = controllerfake.NewFakeClientWithScheme(scheme, initObjects...)
		testReconciler, err = newTestReconciler(client, scheme, kubeClient, crdClient, 60*time.Second, socketPath, false)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(err).NotTo(HaveOccurred())

		err = testReconciler.reconcile(context.TODO(), secretProviderClassPodStatusToProcess)
		g.Expect(err).NotTo(HaveOccurred())
	}
}

func TestPatchSecret(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name               string
		secretToAdd        *v1.Secret
		secretName         string
		expectedSecretData map[string][]byte
		expectedErr        bool
	}{
		{
			name:        "secret is not found",
			secretToAdd: &v1.Secret{},
			secretName:  "secret1",
			expectedErr: true,
		},
		{
			name: "secret is found and data already matches",
			secretToAdd: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "secret1",
					Namespace:       "default",
					ResourceVersion: "16172",
					Labels: map[string]string{
						controllers.SecretManagedLabel: "true",
					},
				},
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			secretName:         "secret1",
			expectedSecretData: map[string][]byte{"key1": []byte("value1")},
			expectedErr:        false,
		},
		{
			name: "secret is found and data is updated to latest",
			secretToAdd: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "secret1",
					Namespace:       "default",
					ResourceVersion: "16172",
					Labels: map[string]string{
						controllers.SecretManagedLabel: "true",
					},
				},
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			secretName:         "secret1",
			expectedSecretData: map[string][]byte{"key2": []byte("value2")},
			expectedErr:        false,
		},
		{
			name: "secret is found and new data is appended to existing",
			secretToAdd: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "secret1",
					Namespace:       "default",
					ResourceVersion: "16172",
					Labels: map[string]string{
						controllers.SecretManagedLabel: "true",
					},
				},
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			secretName:         "secret1",
			expectedSecretData: map[string][]byte{"key1": []byte("value1"), "key2": []byte("value2")},
			expectedErr:        false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme, err := setupScheme()
			g.Expect(err).NotTo(HaveOccurred())

			kubeClient := fake.NewSimpleClientset(test.secretToAdd)
			crdClient := secretsStoreFakeClient.NewSimpleClientset()

			initObjects := []runtime.Object{
				test.secretToAdd,
			}
			client := controllerfake.NewFakeClientWithScheme(scheme, initObjects...)

			testReconciler, err := newTestReconciler(client, scheme, kubeClient, crdClient, 60*time.Second, "", false)
			g.Expect(err).NotTo(HaveOccurred())
			err = testReconciler.secretStore.Run(wait.NeverStop)
			g.Expect(err).NotTo(HaveOccurred())

			err = testReconciler.patchSecret(context.TODO(), test.secretName, v1.NamespaceDefault, test.expectedSecretData)
			if test.expectedErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			if !test.expectedErr {
				// check the secret data is what we expect it to
				secret, err := kubeClient.CoreV1().Secrets(v1.NamespaceDefault).Get(context.TODO(), test.secretName, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(secret.Data).To(Equal(test.expectedSecretData))
			}
		})
	}
}

func TestHandleError(t *testing.T) {
	g := NewWithT(t)

	testReconciler, err := newTestReconciler(nil, nil, nil, nil, 60*time.Second, "", false)
	g.Expect(err).NotTo(HaveOccurred())

	testReconciler.handleError(errors.New("failed error"), "key1", false)
	// wait for the object to be requeued
	time.Sleep(11 * time.Second)
	g.Expect(testReconciler.queue.Len()).To(Equal(1))

	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)
		testReconciler.handleError(errors.New("failed error"), "key1", true)
		g.Expect(testReconciler.queue.NumRequeues("key1")).To(Equal(i + 1))

		testReconciler.queue.Get()
		testReconciler.queue.Done("key1")
	}

	// max number of requeues complete for key2, so now it should be removed from queue
	testReconciler.handleError(errors.New("failed error"), "key1", true)
	time.Sleep(1 * time.Second)
	g.Expect(testReconciler.queue.Len()).To(Equal(1))
}

func getTempTestDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "ut")
	if err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}
	return tmpDir
}

func getTestTargetPath(t *testing.T, uid, vol string) string {
	dir := getTempTestDir(t)
	path := filepath.Join(dir, "pods", uid, "volumes", "kubernetes.io~csi", vol, "mount")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}
	return path
}
