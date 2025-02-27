package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpsgenieScheduleSpec defines the desired state of an Opsgenie Schedule
type OpsgenieScheduleSpec struct {
	// Name of the schedule
	Name string `json:"name"`

	// Description of the schedule
	// +optional
	Description string `json:"description,omitempty"`

	// TimeZone for the schedule (e.g., "UTC", "America/New_York")
	TimeZone string `json:"timeZone"`

	// Rotations define how the schedule is structured
	Rotations []OpsgenieRotation `json:"rotations"`

	// OwnerTeam is the team responsible for the schedule
	OwnerTeam string `json:"ownerTeam"`
}

// OpsgenieRotation defines the rotation configuration
type OpsgenieRotation struct { // Name of the rotation
	// Name of the rotation
	Name string `json:"name"`

	// Type of rotation (daily, weekly, hourly)
	Type string `json:"type"`

	// Start date of the rotation (ISO 8601 format)
	StartDate string `json:"startDate"`

	// End date of the rotation (optional)
	// +optional
	EndDate string `json:"endDate,omitempty"`

	// Length of each shift in the rotation
	Length int `json:"length"`

	// Participants in the rotation
	Participants []OpsgenieParticipant `json:"participants"`

	// Time restrictions for the rotation (optional)
	// +optional
	TimeRestriction *OpsgenieTimeRestriction `json:"timeRestriction,omitempty"`
}

// OpsgenieParticipant defines a user or team in the rotation
type OpsgenieParticipant struct {
	// Type of participant (user, team, escalation, role)
	Type string `json:"type"`

	// ID of the participant
	// +optional
	ID string `json:"id"`

	// Name of the participant (e.g., email for users, team name)
	// +optional
	Name string `json:"name,omitempty"`

	// Username for user participants (optional)
	// +optional
	Username string `json:"username,omitempty"`
}

// OpsgenieTimeRestriction defines active hours for a rotation
type OpsgenieTimeRestriction struct {
	// Type of restriction (weekly, custom)
	Type string `json:"type"`

	// Time intervals when the rotation is active
	Intervals []OpsgenieTimeInterval `json:"intervals"`
}

// OpsgenieTimeInterval defines a time range within a restriction
type OpsgenieTimeInterval struct {
	// Start day (0 = Sunday, 6 = Saturday)
	StartDay int `json:"startDay"`

	// End day (0 = Sunday, 6 = Saturday)
	EndDay int `json:"endDay"`

	// Start hour (0-23)
	StartHour int `json:"startHour"`

	// End hour (0-23)
	EndHour int `json:"endHour"`
}

// OpsgenieScheduleStatus defines the observed state of an Opsgenie Schedule
type OpsgenieScheduleStatus struct {
	// ID of the Opsgenie Schedule
	ScheduleID string `json:"scheduleId,omitempty"`

	// Current status of the schedule
	Status string `json:"status,omitempty"`

	// Last time the schedule was synced
	LastSyncedTime metav1.Time `json:"lastSyncedTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ScheduleID",type=string,JSONPath=".status.scheduleId"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=".status.status"
// +kubebuilder:printcolumn:name="Last Synced",type=string,JSONPath=".status.lastSyncedTime"

// OpsgenieSchedule is the Schema for the Opsgenie Schedules API.
type OpsgenieSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpsgenieScheduleSpec   `json:"spec,omitempty"`
	Status OpsgenieScheduleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpsgenieScheduleList contains a list of OpsgenieSchedule.
type OpsgenieScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpsgenieSchedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpsgenieSchedule{}, &OpsgenieScheduleList{})
}
