package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpsgenieIncidentRuleSpec defines the desired state of OpsgenieIncidentRule.
type OpsgenieIncidentRuleSpec struct {
	// Reference to the Opsgenie Service this rule applies to.
	ServiceRef *corev1.LocalObjectReference `json:"serviceRef"`

	// Conditions that trigger this incident rule.
	Conditions []OpsgenieIncidentCondition `json:"conditions"`

	// Match type for the conditions (e.g., "match-all-conditions", "match-any-condition").
	ConditionMatchType string `json:"conditionMatchType"`
	// Properties of the incident created by this rule.
	IncidentProperties OpsgenieIncidentProperties `json:"incidentProperties"`
}

// OpsgenieIncidentCondition defines the conditions for triggering an incident rule.
type OpsgenieIncidentCondition struct {
	// Field to evaluate (e.g., "message", "alias", "tags").
	Field string `json:"field"`

	// Comparison operation (e.g., "equals", "contains", "startsWith").
	Operation string `json:"operation"`

	// Expected value to match.
	ExpectedValue string `json:"expectedValue"`
}

type OpsgenieIncidentProperties struct {
	// Message describing the incident.
	Message string `json:"message"`

	// Tags attached to the incident.
	Tags []string `json:"tags,omitempty"`

	// Additional details as key-value pairs.
	Details map[string]string `json:"details,omitempty"`

	// Longer description of the incident.
	Description string `json:"description,omitempty"`

	// Priority level (e.g., P1, P2, P3).
	Priority string `json:"priority"`

	// Stakeholder notification settings.
	StakeholderProperties OpsgenieStakeholderProperties `json:"stakeholderProperties"`
}

type OpsgenieStakeholderProperties struct {
	// Whether to enable stakeholder notifications.
	Enable *bool `json:"enable,omitempty"`

	// Message sent to stakeholders.
	Message string `json:"message"`

	// Description for stakeholders.
	Description string `json:"description,omitempty"`
}

// OpsgenieIncidentResponder defines the recipient of the incident action.
type OpsgenieIncidentResponder struct {
	// Type of responder (team, user, schedule).
	Type string `json:"type"`

	// ID of the responder (Opsgenie ID).
	// +optional
	ID string `json:"id,omitempty"`

	// Reference to an OpsgenieSchedule.
	// +optional
	ScheduleRef *corev1.LocalObjectReference `json:"scheduleRef,omitempty"`

	// Reference to an OpsgenieTeam.
	// +optional
	TeamRef *corev1.LocalObjectReference `json:"teamRef,omitempty"`
}

// OpsgenieIncidentRuleStatus defines the observed state of an OpsgenieIncidentRule.
type OpsgenieIncidentRuleStatus struct {
	// ID of the created Opsgenie Incident Rule.
	RuleID string `json:"ruleId,omitempty"`

	// Status of the incident rule (e.g., "Active").
	Status string `json:"status,omitempty"`

	// Last time the rule was synced.
	LastSyncedTime metav1.Time `json:"lastSyncedTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="RuleID",type=string,JSONPath=".status.ruleId"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=".status.status"
// +kubebuilder:printcolumn:name="Last Synced",type=string,JSONPath=".status.lastSyncedTime"

// OpsgenieIncidentRule is the Schema for the Opsgenie incident rules API.
type OpsgenieIncidentRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpsgenieIncidentRuleSpec   `json:"spec,omitempty"`
	Status OpsgenieIncidentRuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpsgenieIncidentRuleList contains a list of OpsgenieIncidentRule.
type OpsgenieIncidentRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpsgenieIncidentRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpsgenieIncidentRule{}, &OpsgenieIncidentRuleList{})
}
