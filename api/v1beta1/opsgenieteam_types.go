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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpsgenieTeamSpec defines the desired state of an Opsgenie Team
type OpsgenieTeamSpec struct {
	// Name of the Opsgenie team
	Name string `json:"name"`

	// Description of the team
	// +optional
	Description string `json:"description,omitempty"`

	// Members of the team
	// Each member requires a user ID and a role (admin, user, etc.)
	// +optional
	Members []OpsgenieTeamMember `json:"members,omitempty"`
}

// OpsgenieTeamMember defines a user belonging to the team
type OpsgenieTeamMember struct {
	// UserID is the unique identifier for the user
	UserID string `json:"userId"`

	// Role in the team (e.g., "admin", "user")
	Role string `json:"role"`
}

// OpsgenieTeamStatus defines the observed state of an Opsgenie Team
type OpsgenieTeamStatus struct {
	// TeamID is the unique identifier in Opsgenie
	TeamID string `json:"teamId,omitempty"`

	// Status reflects the current state of the team
	Status string `json:"status,omitempty"`

	// LastSyncedTime records the last update timestamp
	LastSyncedTime metav1.Time `json:"lastSyncedTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="TeamID",type=string,JSONPath=".status.teamId"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=".status.status"
// +kubebuilder:printcolumn:name="Last Synced",type=string,JSONPath=".status.lastSyncedTime"
// OpsgenieTeam is the Schema for the Opsgenie teams API.
type OpsgenieTeam struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpsgenieTeamSpec   `json:"spec,omitempty"`
	Status OpsgenieTeamStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpsgenieTeamList contains a list of OpsgenieTeam.
type OpsgenieTeamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpsgenieTeam `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpsgenieTeam{}, &OpsgenieTeamList{})
}
