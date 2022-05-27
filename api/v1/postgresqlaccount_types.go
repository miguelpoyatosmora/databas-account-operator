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

// PostgreSQLAccountSpec defines the desired state of PostgreSQLAccount
type PostgreSQLAccountSpec struct {
	PostgreSQLDatabaseName string `json:"postgreSQLDatabaseName,omitempty"`
	Name                   string `json:"name,omitempty"`
	Password               string `json:"password,omitempty"`
	ValidUntil             string `json:"valid_until,omitempty"`
}

// PostgreSQLAccountStatus defines the observed state of PostgreSQLAccount
type PostgreSQLAccountStatus struct {
	Ready bool   `json:"ready"`
	Error string `json:"error"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PostgreSQLAccount is the Schema for the postgresqlaccounts API
type PostgreSQLAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgreSQLAccountSpec   `json:"spec,omitempty"`
	Status PostgreSQLAccountStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PostgreSQLAccountList contains a list of PostgreSQLAccount
type PostgreSQLAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgreSQLAccount `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgreSQLAccount{}, &PostgreSQLAccountList{})
}
