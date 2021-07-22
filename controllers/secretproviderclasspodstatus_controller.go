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

package controllers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/scheme"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/fileutil"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/k8sutil"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/secretutil"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	SecretManagedLabel         = "secrets-store.csi.k8s.io/managed"
	SecretUsedLabel            = "secrets-store.csi.k8s.io/used"
	secretCreationFailedReason = "FailedToCreateSecret"

	SyncSecretForbiddenWarning = "The secret operation failed with forbidden error. If you installed the CSI driver using helm, ensure syncSecret.enabled=true is set."
)

// SecretProviderClassPodStatusReconciler reconciles a SecretProviderClassPodStatus object
type SecretProviderClassPodStatusReconciler struct {
	client.Client
	mutex         *sync.Mutex
	scheme        *apiruntime.Scheme
	nodeID        string
	reader        client.Reader
	writer        client.Writer
	eventRecorder record.EventRecorder
}

// New creates a new SecretProviderClassPodStatusReconciler
func New(mgr manager.Manager, nodeID string) (*SecretProviderClassPodStatusReconciler, error) {
	eventBroadcaster := record.NewBroadcaster()
	kubeClient := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	eventBroadcaster.StartRecordingToSink(&clientcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "csi-secrets-store-controller"})

	return &SecretProviderClassPodStatusReconciler{
		Client:        mgr.GetClient(),
		mutex:         &sync.Mutex{},
		scheme:        mgr.GetScheme(),
		nodeID:        nodeID,
		reader:        mgr.GetCache(),
		writer:        mgr.GetClient(),
		eventRecorder: recorder,
	}, nil
}

func (r *SecretProviderClassPodStatusReconciler) RunPatcher(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Patcher(ctx); err != nil {
				klog.ErrorS(err, "failed to patch secret owner ref")
			}
		}
	}
}

func (r *SecretProviderClassPodStatusReconciler) Patcher(ctx context.Context) error {
	klog.V(10).Infof("patcher started")
	r.mutex.Lock()
	defer r.mutex.Unlock()

	spcPodStatusList := &v1alpha1.SecretProviderClassPodStatusList{}
	spcMap := make(map[string]v1alpha1.SecretProviderClass)
	secretOwnerMap := make(map[types.NamespacedName][]metav1.OwnerReference)
	// get a list of all spc pod status that belong to the node
	err := r.reader.List(ctx, spcPodStatusList, r.ListOptionsLabelSelector())
	if err != nil {
		return fmt.Errorf("failed to list secret provider class pod status, err: %+v", err)
	}

	spcPodStatuses := spcPodStatusList.Items
	for i := range spcPodStatuses {
		spcName := spcPodStatuses[i].Status.SecretProviderClassName
		spc := &v1alpha1.SecretProviderClass{}
		namespace := spcPodStatuses[i].Namespace

		if val, exists := spcMap[namespace+"/"+spcName]; exists {
			spc = &val
		} else {
			if err := r.reader.Get(ctx, client.ObjectKey{Namespace: namespace, Name: spcName}, spc); err != nil {
				return fmt.Errorf("failed to get spc %s, err: %+v", spcName, err)
			}
			spcMap[namespace+"/"+spcName] = *spc
		}
		// get the pod and check if the pod has a owner reference
		pod := &v1.Pod{}
		err = r.reader.Get(ctx, client.ObjectKey{Namespace: namespace, Name: spcPodStatuses[i].Status.PodName}, pod)
		if err != nil {
			return fmt.Errorf("failed to fetch pod during patching, err: %+v", err)
		}
		var ownerRefs []metav1.OwnerReference
		for _, ownerRef := range pod.GetOwnerReferences() {
			ownerRefs = append(ownerRefs, metav1.OwnerReference{
				APIVersion: ownerRef.APIVersion,
				Kind:       ownerRef.Kind,
				UID:        ownerRef.UID,
				Name:       ownerRef.Name,
			})
		}
		// If a pod has no owner references, then it's a static pod and
		// doesn't belong to a replicaset. In this case, use the spcps as
		// owner reference just like we do it today
		if len(ownerRefs) == 0 {
			// Create a new owner ref.
			gvk, err := apiutil.GVKForObject(&spcPodStatuses[i], r.scheme)
			if err != nil {
				return err
			}
			ref := metav1.OwnerReference{
				APIVersion: gvk.GroupVersion().String(),
				Kind:       gvk.Kind,
				UID:        spcPodStatuses[i].GetUID(),
				Name:       spcPodStatuses[i].GetName(),
			}
			ownerRefs = append(ownerRefs, ref)
		}

		for _, secret := range spc.Spec.SecretObjects {
			key := types.NamespacedName{Name: secret.SecretName, Namespace: namespace}
			val, exists := secretOwnerMap[key]
			if exists {
				secretOwnerMap[key] = append(val, ownerRefs...)
			} else {
				secretOwnerMap[key] = ownerRefs
			}
		}
	}

	for secret, owners := range secretOwnerMap {
		patchFn := func() (bool, error) {
			if err := r.patchSecretWithOwnerRef(ctx, secret.Name, secret.Namespace, owners...); err != nil {
				if !apierrors.IsConflict(err) || !apierrors.IsTimeout(err) {
					klog.ErrorS(err, "failed to set owner ref for secret", "secret", klog.ObjectRef{Namespace: secret.Namespace, Name: secret.Name})
				}
				// syncSecret.enabled is set to false by default in the helm chart for installing the driver in v0.0.23+
				// that would result in a forbidden error, so generate a warning that can be helpful for debugging
				if apierrors.IsForbidden(err) {
					klog.Warning(SyncSecretForbiddenWarning)
				}
				return false, nil
			}
			return true, nil
		}
		if err := wait.ExponentialBackoff(wait.Backoff{
			Steps:    5,
			Duration: 1 * time.Millisecond,
			Factor:   1.0,
			Jitter:   0.1,
		}, patchFn); err != nil {
			return err
		}
	}

	klog.V(10).Infof("patcher completed")
	return nil
}

// ListOptionsLabelSelector returns a ListOptions with a label selector for node name.
func (r *SecretProviderClassPodStatusReconciler) ListOptionsLabelSelector() client.ListOption {
	return client.MatchingLabels(map[string]string{
		v1alpha1.InternalNodeLabel: r.nodeID,
	})
}

// +kubebuilder:rbac:groups=secrets-store.csi.x-k8s.io,resources=secretproviderclasspodstatuses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=secrets-store.csi.x-k8s.io,resources=secretproviderclasspodstatuses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=secrets-store.csi.x-k8s.io,resources=secretproviderclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *SecretProviderClassPodStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	klog.InfoS("reconcile started", "spcps", req.NamespacedName.String())

	spcPodStatus := &v1alpha1.SecretProviderClassPodStatus{}
	if err := r.reader.Get(ctx, req.NamespacedName, spcPodStatus); err != nil {
		if apierrors.IsNotFound(err) {
			klog.InfoS("reconcile complete", "spcps", req.NamespacedName.String())
			return ctrl.Result{}, nil
		}
		klog.ErrorS(err, "failed to get spc pod status", "spcps", req.NamespacedName.String())
		return ctrl.Result{}, err
	}

	// Obtain the full pod metadata. An object reference is needed for sending
	// events and the UID is helpful for validating the SPCPS TargetPath.
	pod := &v1.Pod{}
	if err := r.reader.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: spcPodStatus.Status.PodName}, pod); err != nil {
		klog.ErrorS(err, "failed to get pod", "pod", klog.ObjectRef{Namespace: req.Namespace, Name: spcPodStatus.Status.PodName})
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}
	// skip reconcile if the pod is being terminated
	// or the pod is in succeeded state (for jobs that complete aren't gc yet)
	// or the pod is in a failed state (all containers get terminated)
	if !pod.GetDeletionTimestamp().IsZero() || pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
		klog.V(5).InfoS("pod is being terminated, skipping reconcile", "pod", klog.KObj(pod))
		return ctrl.Result{}, nil
	}

	spcName := spcPodStatus.Status.SecretProviderClassName
	spc := &v1alpha1.SecretProviderClass{}
	if err := r.reader.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: spcName}, spc); err != nil {
		klog.ErrorS(err, "failed to get spc", "spc", spcName)
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	if len(spc.Spec.SecretObjects) == 0 {
		klog.InfoS("no secret objects defined for spc, nothing to reconcile", "spc", klog.KObj(spc), "spcps", klog.KObj(spcPodStatus))
		return ctrl.Result{}, nil
	}

	// determine which pod volume this is associated with
	podVol := k8sutil.SPCVolume(pod, spc.Name)
	if podVol == nil {
		return ctrl.Result{}, fmt.Errorf("failed to find secret provider class pod status volume for pod %s/%s", req.Namespace, spcPodStatus.Status.PodName)
	}

	// validate TargetPath
	if fileutil.GetPodUIDFromTargetPath(spcPodStatus.Status.TargetPath) != string(pod.UID) {
		return ctrl.Result{}, fmt.Errorf("secret provider class pod status targetPath did not match pod UID for pod %s/%s", req.Namespace, spcPodStatus.Status.PodName)
	}
	if fileutil.GetVolumeNameFromTargetPath(spcPodStatus.Status.TargetPath) != podVol.Name {
		return ctrl.Result{}, fmt.Errorf("secret provider class pod status volume name did not match pod Volume for pod %s/%s", req.Namespace, spcPodStatus.Status.PodName)
	}

	files, err := fileutil.GetMountedFiles(spcPodStatus.Status.TargetPath)
	if err != nil {
		r.generateEvent(pod, corev1.EventTypeWarning, secretCreationFailedReason, fmt.Sprintf("failed to get mounted files, err: %+v", err))
		klog.ErrorS(err, "failed to get mounted files", "spc", klog.KObj(spc), "pod", klog.KObj(pod), "spcps", klog.KObj(spcPodStatus))
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}
	errs := make([]error, 0)
	for _, secretObj := range spc.Spec.SecretObjects {
		secretName := strings.TrimSpace(secretObj.SecretName)

		if err = secretutil.ValidateSecretObject(*secretObj); err != nil {
			klog.ErrorS(err, "failed to validate secret object in spc", "spc", klog.KObj(spc), "pod", klog.KObj(pod), "spcps", klog.KObj(spcPodStatus))
			errs = append(errs, fmt.Errorf("failed to validate secret object in spc %s/%s, err: %+v", spc.Namespace, spc.Name, err))
			continue
		}
		exists, err := r.secretExists(ctx, secretName, req.Namespace)
		if err != nil {
			klog.ErrorS(err, "failed to check if secret exists", "secret", klog.ObjectRef{Namespace: req.Namespace, Name: secretName}, "spc", klog.KObj(spc), "pod", klog.KObj(pod), "spcps", klog.KObj(spcPodStatus))
			// syncSecret.enabled is set to false by default in the helm chart for installing the driver in v0.0.23+
			// that would result in a forbidden error, so generate a warning that can be helpful for debugging
			if apierrors.IsForbidden(err) {
				klog.Warning(SyncSecretForbiddenWarning)
			}
			errs = append(errs, fmt.Errorf("failed to check if secret %s exists, err: %+v", secretName, err))
			continue
		}

		var funcs []func() (bool, error)

		if !exists {
			secretType := secretutil.GetSecretType(strings.TrimSpace(secretObj.Type))

			var datamap map[string][]byte
			if datamap, err = secretutil.GetSecretData(secretObj.Data, secretType, files); err != nil {
				r.generateEvent(pod, corev1.EventTypeWarning, secretCreationFailedReason, fmt.Sprintf("failed to get data in spc %s/%s for secret %s, err: %+v", req.Namespace, spcName, secretName, err))
				klog.ErrorS(err, "failed to get data in spc for secret", "spc", klog.KObj(spc), "pod", klog.KObj(pod), "secret", klog.ObjectRef{Namespace: req.Namespace, Name: secretName}, "spcps", klog.KObj(spcPodStatus))
				errs = append(errs, fmt.Errorf("failed to get data in spc %s/%s for secret %s, err: %+v", req.Namespace, spcName, secretName, err))
				continue
			}

			labelsMap := make(map[string]string)
			if secretObj.Labels != nil {
				labelsMap = secretObj.Labels
			}
			annotationsMap := make(map[string]string)
			if secretObj.Annotations != nil {
				annotationsMap = secretObj.Annotations
			}
			// Set secrets-store.csi.k8s.io/managed=true label on the secret that's created and managed
			// by the secrets-store-csi-driver. This label will be used to perform a filtered list watch
			// only on secrets created and managed by the driver
			labelsMap[SecretManagedLabel] = "true"

			createFn := func() (bool, error) {
				if err := r.createK8sSecret(ctx, secretName, req.Namespace, datamap, labelsMap, annotationsMap, secretType); err != nil {
					klog.ErrorS(err, "failed to create Kubernetes secret", "spc", klog.KObj(spc), "pod", klog.KObj(pod), "secret", klog.ObjectRef{Namespace: req.Namespace, Name: secretName}, "spcps", klog.KObj(spcPodStatus))
					// syncSecret.enabled is set to false by default in the helm chart for installing the driver in v0.0.23+
					// that would result in a forbidden error, so generate a warning that can be helpful for debugging
					if apierrors.IsForbidden(err) {
						klog.Warning(SyncSecretForbiddenWarning)
					}
					return false, nil
				}
				return true, nil
			}
			funcs = append(funcs, createFn)
		}

		for _, f := range funcs {
			if err := wait.ExponentialBackoff(wait.Backoff{
				Steps:    5,
				Duration: 1 * time.Millisecond,
				Factor:   1.0,
				Jitter:   0.1,
			}, f); err != nil {
				r.generateEvent(pod, corev1.EventTypeWarning, secretCreationFailedReason, err.Error())
				return ctrl.Result{RequeueAfter: 5 * time.Second}, err
			}
		}
	}

	if len(errs) > 0 {
		return ctrl.Result{Requeue: true}, nil
	}

	klog.InfoS("reconcile complete", "spc", klog.KObj(spc), "pod", klog.KObj(pod), "spcps", klog.KObj(spcPodStatus))
	// requeue the spc pod status again after 5mins to check if secret and ownerRef exists
	// and haven't been modified. If secret doesn't exist, then this requeue will ensure it's
	// created in the next reconcile and the owner ref patched again
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *SecretProviderClassPodStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.SecretProviderClassPodStatus{}).
		WithEventFilter(r.belongsToNodePredicate()).
		Complete(r)
}

// belongsToNodePredicate defines predicates for handlers
func (r *SecretProviderClassPodStatusReconciler) belongsToNodePredicate() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return r.processIfBelongsToNode(e.ObjectNew)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return r.processIfBelongsToNode(e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return r.processIfBelongsToNode(e.Object)
		},
	}
}

// processIfBelongsToNode determines if the secretproviderclasspodstatus belongs to the node based on the
// internal.secrets-store.csi.k8s.io/node-name: <node name> label. If belongs to node, then the spcps is processed.
func (r *SecretProviderClassPodStatusReconciler) processIfBelongsToNode(objMeta metav1.Object) bool {
	node, ok := objMeta.GetLabels()[v1alpha1.InternalNodeLabel]
	if !ok {
		return false
	}
	if !strings.EqualFold(node, r.nodeID) {
		return false
	}
	return true
}

// createK8sSecret creates K8s secret with data from mounted files
// If a secret with the same name already exists in the namespace of the pod, the error is nil.
func (r *SecretProviderClassPodStatusReconciler) createK8sSecret(ctx context.Context, name, namespace string, datamap map[string][]byte, labelsmap map[string]string, annotationsmap map[string]string, secretType corev1.SecretType) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      labelsmap,
			Annotations: annotationsmap,
		},
		Type: secretType,
		Data: datamap,
	}

	err := r.writer.Create(ctx, secret)
	if err == nil {
		klog.InfoS("successfully created Kubernetes secret", "secret", klog.ObjectRef{Namespace: namespace, Name: name})
		return nil
	}
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// patchSecretWithOwnerRef patches the secret owner reference with the spc pod status
func (r *SecretProviderClassPodStatusReconciler) patchSecretWithOwnerRef(ctx context.Context, name, namespace string, ownerRefs ...metav1.OwnerReference) error {
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	if err := r.Client.Get(ctx, secretKey, secret); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(5).InfoS("secret not found for patching", "secret", klog.ObjectRef{Namespace: namespace, Name: name})
			return nil
		}
		return err
	}

	patch := client.MergeFromWithOptions(secret.DeepCopy(), client.MergeFromWithOptimisticLock{})
	needsPatch := false

	secretOwnerRefs := secret.GetOwnerReferences()
	secretOwnerMap := make(map[string]types.UID)
	for _, or := range secretOwnerRefs {
		secretOwnerMap[or.Name] = or.UID
	}

	for i := range ownerRefs {
		if _, exists := secretOwnerMap[ownerRefs[i].Name]; exists {
			continue
		}
		// add to map for tracking
		secretOwnerMap[ownerRefs[i].Name] = ownerRefs[i].UID
		needsPatch = true
		klog.V(5).Infof("Adding %s/%s as owner ref for %s/%s", ownerRefs[i].APIVersion, ownerRefs[i].Name, namespace, name)
		secretOwnerRefs = append(secretOwnerRefs, ownerRefs[i])
	}

	if needsPatch {
		secret.SetOwnerReferences(secretOwnerRefs)
		return r.writer.Patch(ctx, secret, patch)
	}
	return nil
}

// secretExists checks if the secret with name and namespace already exists
func (r *SecretProviderClassPodStatusReconciler) secretExists(ctx context.Context, name, namespace string) (bool, error) {
	o := &v1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := r.Client.Get(ctx, secretKey, o)
	if err == nil {
		return true, nil
	}
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	return false, err
}

// generateEvent generates an event
func (r *SecretProviderClassPodStatusReconciler) generateEvent(obj runtime.Object, eventType, reason, message string) {
	if obj != nil {
		r.eventRecorder.Eventf(obj, eventType, reason, message)
	}
}
