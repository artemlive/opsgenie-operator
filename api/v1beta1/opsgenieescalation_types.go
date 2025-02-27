package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpsgenieEscalationSpec defines the desired state of an Opsgenie Escalation
type OpsgenieEscalationSpec struct {
	// Name of the escalation
	Name string `json:"name"`

	// Description of the escalation
	// +optional
	Description string `json:"description,omitempty"`

	// Direct Team ID reference
	// +optional
	TeamID string `json:"teamId,omitempty"`

	// Reference to an OpsgenieTeam CR
	// +optional
	TeamRef *corev1.LocalObjectReference `json:"teamRef,omitempty"`

	// Rules defining the escalation
	Rules []OpsgenieEscalationRule `json:"rules"`
}

// OpsgenieEscalationRule defines how the escalation should work
type OpsgenieEscalationRule struct {
	// Condition for escalation (e.g., if no action is taken)
	// +optional
	Condition string `json:"condition,omitempty"`

	// Notification type (e.g., Default, SMS, Email)
	// +optional
	NotifyType string `json:"notifyType,omitempty"`

	// Time delay before executing the action (in minutes)
	// +optional
	DelayMin int `json:"delayMin,omitempty"`

	// Recipient (user, team, or role)
	Recipient OpsgenieEscalationRecipient `json:"recipient"`
}

// OpsgenieEscalationRecipient defines who receives the escalation
type OpsgenieEscalationRecipient struct {
	// Type of recipient (team, user, schedule)
	// Valid values: team, user, schedule
	Type string `json:"type"`

	// ID of the recipient (Team ID, User ID, or Schedule ID)
	// +optional
	ID string `json:"id"`

	// Username (for user type recipients)
	// +optional
	Username string `json:"username,omitempty"`

	// Reference to an OpsgenieSchedule
	// +optional
	ScheduleRef *corev1.LocalObjectReference `json:"scheduleRef,omitempty"`

	// Reference to an OpsgenieTeam
	// +optional
	TeamRef *corev1.LocalObjectReference `json:"teamRef,omitempty"`
}

// OpsgenieEscalationStatus defines the observed state of an Opsgenie Escalation
type OpsgenieEscalationStatus struct {
	// ID of the created Opsgenie Escalation
	EscalationID string `json:"escalationId,omitempty"`

	// Resolved Team ID (either from direct field or reference)
	ResolvedTeamID string `json:"resolvedTeamId,omitempty"`

	// Current status of the escalation
	Status string `json:"status,omitempty"`

	// Last time the escalation was synced
	LastSyncedTime metav1.Time `json:"lastSyncedTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="EscalationID",type=string,JSONPath=".status.escalationId"
// +kubebuilder:printcolumn:name="ResolvedTeamID",type=string,JSONPath=".status.resolvedTeamId"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=".status.status"
// +kubebuilder:printcolumn:name="Last Synced",type=string,JSONPath=".status.lastSyncedTime"
// OpsgenieEscalation is the Schema for the Opsgenie escalations API.
type OpsgenieEscalation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpsgenieEscalationSpec   `json:"spec,omitempty"`
	Status OpsgenieEscalationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpsgenieEscalationList contains a list of OpsgenieEscalation.
type OpsgenieEscalationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpsgenieEscalation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpsgenieEscalation{}, &OpsgenieEscalationList{})
}
