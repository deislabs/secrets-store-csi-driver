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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Provider enum for all the provider names
type Provider string

const (
	// Azure provider for Azure Key Vault
	Azure Provider = "Azure"
	// Vault provider for Hashicorp Vault
	Vault Provider = "Vault"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SecretObjectData defines the desired state of synced K8s secret object data
type SecretObjectData struct {
	// name of the object to sync
	ObjectName string `json:"objectName,omitempty"`
	// data field to populate
	Key string `json:"key,omitempty"`
}

// SecretObject defines the desired state of synced K8s secret objects
type SecretObject struct {
	// name of the K8s secret object
	SecretName string `json:"secretName,omitempty"`
	// type of K8s secret object
	Type string `json:"type,omitempty"`
	// labels of K8s secret object
	Labels map[string]string `json:"labels,omitempty"`
	// annotations of k8s secret object
	Annotations map[string]string   `json:"annotations,omitempty"`
	Data        []*SecretObjectData `json:"data,omitempty"`
	// SyncAll can sync all secrets defined in the parameters field of SecretProviderClass
	SyncAll bool `json:"syncAll,omitempty"`
}

// SecretProviderClassSpec defines the desired state of SecretProviderClass
type SecretProviderClassSpec struct {
	// Configuration for provider name
	Provider Provider `json:"provider,omitempty"`
	// Configuration for specific provider
	Parameters    map[string]string `json:"parameters,omitempty"`
	SecretObjects []*SecretObject   `json:"secretObjects,omitempty"`
}

// ByPodStatus defines the state of SecretProviderClass as seen by
// an individual controller
type ByPodStatus struct {
	// id of the pod that wrote the status
	ID string `json:"id,omitempty"`
	// namespace of the pod that wrote the status
	Namespace string `json:"namespace,omitempty"`
}

// SecretProviderClassStatus defines the observed state of SecretProviderClass
type SecretProviderClassStatus struct {
	ByPod []*ByPodStatus `json:"byPod,omitempty"`
}

// +kubebuilder:object:root=true
// +genclient

// SecretProviderClass is the Schema for the secretproviderclasses API
type SecretProviderClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretProviderClassSpec   `json:"spec,omitempty"`
	Status SecretProviderClassStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SecretProviderClassList contains a list of SecretProviderClass
type SecretProviderClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretProviderClass `json:"items"`
}
