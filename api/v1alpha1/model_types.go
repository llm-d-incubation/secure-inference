/*
Copyright 2025.

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

const (
	// ModelTypeBase is the type value for base models.
	ModelTypeBase = "BaseModel"
	// ModelTypeLora is the type value for LoRA adapter models.
	ModelTypeLora = "LoRA"
)

type ModelAccessPolicy struct {
	UserAttributes map[string][]string `json:"userAttributes"`
}

type ModelSelectionPolicy struct {
	KeyWords     []string `json:"keyWords,omitempty"`
	Descriptions []string `json:"descriptions,omitempty"`
}

// ModelSpec defines the desired state of Model.
type ModelSpec struct {
	Id string `json:"id"`
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:Enum=BaseModel;LoRA
	Type        string `json:"type"`
	BaseModelId string `json:"baseModelId,omitempty"`
	//+kubebuilder:validation:Required
	AccessPolicy    ModelAccessPolicy    `json:"accessPolicy"`
	SelectionPolicy ModelSelectionPolicy `json:"selectionPolicy,omitempty"`
}

// ModelStatus defines the observed state of Model.
type ModelStatus struct{}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Model is the Schema for the models API.
type Model struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelSpec   `json:"spec,omitempty"`
	Status ModelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ModelList contains a list of Model.
type ModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Model `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Model{}, &ModelList{})
}
