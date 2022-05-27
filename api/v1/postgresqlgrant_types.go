/*
Copyright 2022.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PostgreSQLGrantSpec defines the desired state of PostgreSQLGrant
type PostgreSQLGrantSpec struct {
	PostgreSQLDatabaseName string   `json:"postgreSQLDatabaseName,omitempty"`
	Type                   []string `json:"type,omitempty"`
	To                     string   `json:"to,omitempty"`
	Schema                 string   `json:"schema,omitempty"`
}

// PostgreSQLGrantStatus defines the observed state of PostgreSQLGrant
type PostgreSQLGrantStatus struct {
	Ready bool   `json:"ready"`
	Error string `json:"error"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PostgreSQLGrant is the Schema for the postgresqlgrants API
type PostgreSQLGrant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgreSQLGrantSpec   `json:"spec,omitempty"`
	Status PostgreSQLGrantStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PostgreSQLGrantList contains a list of PostgreSQLGrant
type PostgreSQLGrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgreSQLGrant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgreSQLGrant{}, &PostgreSQLGrantList{})
}
