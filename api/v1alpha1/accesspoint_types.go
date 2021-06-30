/*
Copyright 2021.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AccessPointSpec defines the desired state of AccessPoint
type AccessPointSpec struct {

	//+kubebuilder:validation:Required
	//+kubebuilder:validation:Format:=string

	// SSID for this AccessPoint
	Ssid string `json:"ssid,omitempty"`

	//+kubebuilder:validation:Required
	//+kubebuilder:validation:Format:=string

	// Password for this AccessPoint
	Password string `json:"password,omitempty"` // TODO -> Secret

	//+kubebuilder:validation:Required
	//+kubebuilder:validation:Format:=string

	// The Network Interface name
	Interface string `json:"interface,omitempty"`

	//+kubebuilder:validation:Format:=string

	// If the interface needs bridge to connect to the Internet, you want to set the bridge name.
	Bridge string `json:"bridge,omitempty"`

	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// This ap will deploy onto this devices.
	Devices []string `json:"devices,omitempty"`
}

// AccessPointStatus defines the observed state of AccessPoint
type AccessPointStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AccessPoint is the Schema for the accesspoints API
type AccessPoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AccessPointSpec   `json:"spec,omitempty"`
	Status AccessPointStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AccessPointList contains a list of AccessPoint
type AccessPointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccessPoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AccessPoint{}, &AccessPointList{})
}
