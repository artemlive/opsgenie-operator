package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpsgenieServiceSpec defines the desired state of an Opsgenie Service
type OpsgenieServiceSpec struct {
	// Name of the Opsgenie service
	Name string `json:"name"`

	// Description of the service
	// +optional
	Description string `json:"description,omitempty"`

	// TeamID allows direct specification of an Opsgenie Team ID
	// +optional
	TeamID string `json:"teamId,omitempty"`

	// TeamRef allows referencing an OpsgenieTeam resource instead of using a hardcoded team ID
	// +optional
	TeamRef *corev1.LocalObjectReference `json:"teamRef,omitempty"`

	// Tags for categorization
	// +optional
	Tags []string `json:"tags,omitempty"`
}

// OpsgenieServiceStatus defines the observed state of an Opsgenie Service
type OpsgenieServiceStatus struct {
	ServiceID      string      `json:"serviceId,omitempty"`
	Status         string      `json:"status,omitempty"`
	ResolvedTeamID string      `json:"resolvedTeamId,omitempty"`
	LastSyncedTime metav1.Time `json:"lastSyncedTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ServiceID",type=string,JSONPath=".status.serviceId"
// +kubebuilder:printcolumn:name="ResolvedTeamID",type=string,JSONPath=".status.resolvedTeamId"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=".status.status"
// +kubebuilder:printcolumn:name="Last Synced",type=string,JSONPath=".status.lastSyncedTime"
// OpsgenieService is the Schema for the Opsgenie services API.
type OpsgenieService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpsgenieServiceSpec   `json:"spec,omitempty"`
	Status OpsgenieServiceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpsgenieServiceList contains a list of OpsgenieService.
type OpsgenieServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpsgenieService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpsgenieService{}, &OpsgenieServiceList{})
}
