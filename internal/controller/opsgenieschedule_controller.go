/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	opsgeniev1beta1 "github.com/artemlive/opsgenie-operator/api/v1beta1"
	"github.com/go-logr/logr"
	"github.com/opsgenie/opsgenie-go-sdk-v2/og"
	"github.com/opsgenie/opsgenie-go-sdk-v2/schedule"
)

// OpsgenieScheduleReconciler reconciles an OpsgenieSchedule object
type OpsgenieScheduleReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	OpsgenieClient *schedule.Client
}

// +kubebuilder:rbac:groups=opsgenie.macpaw.dev,resources=opsgenieschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=opsgenie.macpaw.dev,resources=opsgenieschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=opsgenie.macpaw.dev,resources=opsgenieschedules/finalizers,verbs=update

func (r *OpsgenieScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var scheduleCR opsgeniev1beta1.OpsgenieSchedule
	if err := r.Get(ctx, req.NamespacedName, &scheduleCR); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	opsgenieRotations, err := r.convertRotations(scheduleCR.Spec.Rotations)
	if err != nil {
		logger.Error(err, "Failed to parse rotations")
		return ctrl.Result{}, err
	}

	if scheduleCR.Status.ScheduleID != "" {
		return r.updateSchedule(ctx, &scheduleCR, opsgenieRotations, logger)
	}

	return r.createSchedule(ctx, &scheduleCR, opsgenieRotations, logger)
}

func (r *OpsgenieScheduleReconciler) createSchedule(ctx context.Context, scheduleCR *opsgeniev1beta1.OpsgenieSchedule, rotations []og.Rotation, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Creating new Opsgenie schedule", "name", scheduleCR.Spec.Name)

	createReq := &schedule.CreateRequest{
		Name:        scheduleCR.Spec.Name,
		Description: scheduleCR.Spec.Description,
		Timezone:    scheduleCR.Spec.TimeZone,
		OwnerTeam:   &og.OwnerTeam{Name: scheduleCR.Spec.OwnerTeam},
		Rotations:   rotations,
	}

	createResp, err := r.OpsgenieClient.Create(ctx, createReq)
	if err != nil {
		// Check if it's a 409 Conflict (Schedule already exists)
		if strings.Contains(err.Error(), "409") && strings.Contains(err.Error(), "already exists") {
			logger.Info("Schedule already exists, trying to import", "name", scheduleCR.Spec.Name)

			// Fetch existing schedule by name
			existingSchedule, err := r.OpsgenieClient.Get(ctx, &schedule.GetRequest{
				IdentifierType:  schedule.Name,
				IdentifierValue: scheduleCR.Spec.Name,
			})
			if err != nil {
				logger.Error(err, "Failed to fetch existing schedule during implicit import")
				return ctrl.Result{}, err
			}

			// Update CR status with existing schedule ID
			scheduleCR.Status.ScheduleID = existingSchedule.Schedule.Id
			scheduleCR.Status.Status = "Active"
			scheduleCR.Status.LastSyncedTime = metav1.Now()

			if err := r.Status().Update(ctx, scheduleCR); err != nil {
				logger.Error(err, "Failed to update CR status after importing existing schedule")
				return ctrl.Result{}, err
			}

			logger.Info("Successfully imported existing Opsgenie schedule", "scheduleID", existingSchedule.Schedule.Id)
			return ctrl.Result{}, nil
		}

		// If it's another error, return it
		logger.Error(err, "Failed to create Opsgenie schedule")
		return ctrl.Result{}, err
	}

	// If creation succeeds, update CR status with new Schedule ID
	scheduleCR.Status.ScheduleID = createResp.Id
	scheduleCR.Status.Status = "Active"
	scheduleCR.Status.LastSyncedTime = metav1.Now()

	if err := r.Status().Update(ctx, scheduleCR); err != nil {
		logger.Error(err, "Failed to update status after schedule creation")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully created Opsgenie schedule", "scheduleID", createResp.Id)
	return ctrl.Result{RequeueAfter: 60 * time.Minute}, nil
}

func (r *OpsgenieScheduleReconciler) convertRotations(crRotations []opsgeniev1beta1.OpsgenieRotation) ([]og.Rotation, error) {
	var opsgenieRotations []og.Rotation

	for _, r := range crRotations {
		startTime, err := time.Parse(time.RFC3339, r.StartDate)
		if err != nil {
			return nil, fmt.Errorf("invalid startDate format: %s", r.StartDate)
		}

		participants, err := convertParticipants(r.Participants)
		if err != nil {
			return nil, err
		}
		if len(participants) == 0 {
			return nil, fmt.Errorf("rotation '%s' must have at least one participant", r.Name)
		}

		// Convert time restrictions if present
		var timeRestriction *og.TimeRestriction
		if r.TimeRestriction != nil {
			timeRestriction, err = convertTimeRestrictions(*r.TimeRestriction)
			if err != nil {
				return nil, err
			}
		}

		opsgenieRotations = append(opsgenieRotations, og.Rotation{
			Name:            r.Name,
			Type:            og.RotationType(r.Type),
			StartDate:       &startTime,
			Length:          uint32(r.Length),
			Participants:    participants,
			TimeRestriction: timeRestriction,
		})
	}

	return opsgenieRotations, nil
}

func (r *OpsgenieScheduleReconciler) updateSchedule(ctx context.Context, scheduleCR *opsgeniev1beta1.OpsgenieSchedule, rotations []og.Rotation, logger logr.Logger) (ctrl.Result, error) {
	existingSchedule, err := r.OpsgenieClient.Get(ctx, &schedule.GetRequest{
		IdentifierType:  schedule.Id,
		IdentifierValue: scheduleCR.Status.ScheduleID,
	})
	if err != nil {
		logger.Error(err, "Failed to fetch existing schedule from Opsgenie")
		return ctrl.Result{}, err
	}

	if !r.isUpdateRequired(scheduleCR, existingSchedule, rotations) {
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	logger.Info("Updating existing Opsgenie schedule", "scheduleID", scheduleCR.Status.ScheduleID)

	updateReq := &schedule.UpdateRequest{
		IdentifierType:  schedule.Id,
		IdentifierValue: scheduleCR.Status.ScheduleID,
		Name:            scheduleCR.Spec.Name,
		Description:     scheduleCR.Spec.Description,
		OwnerTeam:       &og.OwnerTeam{Name: scheduleCR.Spec.OwnerTeam},
		Timezone:        scheduleCR.Spec.TimeZone,
		Rotations:       rotations,
	}

	if _, err := r.OpsgenieClient.Update(ctx, updateReq); err != nil {
		logger.Error(err, "Failed to update Opsgenie schedule")
		return ctrl.Result{}, err
	}

	scheduleCR.Status.LastSyncedTime = metav1.Now()
	if err := r.Status().Update(ctx, scheduleCR); err != nil {
		logger.Error(err, "Failed to update schedule status after update")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully updated Opsgenie schedule", "scheduleID", scheduleCR.Status.ScheduleID)
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func convertParticipants(crParticipants []opsgeniev1beta1.OpsgenieParticipant) ([]og.Participant, error) {
	var participants []og.Participant

	for _, p := range crParticipants {
		if p.ID == "" && p.Name == "" && p.Username == "" {
			return nil, fmt.Errorf("participant must have at least one of ID, Name, or Username")
		}

		participant := og.Participant{Type: og.ParticipantType(p.Type)}

		// Assign values based on participant type
		switch p.Type {
		case "user":
			if p.ID != "" {
				participant.Id = p.ID
			} else if p.Username != "" {
				participant.Username = p.Username
			} else {
				return nil, fmt.Errorf("for participant type 'user', either ID or Username must be provided")
			}

		case "team", "escalation", "role":
			if p.ID != "" {
				participant.Id = p.ID
			} else {
				participant.Name = p.Name // Teams, escalations, and roles can use names
			}

		default:
			return nil, fmt.Errorf("unsupported participant type: %s", p.Type)
		}

		participants = append(participants, participant)
	}

	return participants, nil
}

func convertTimeRestrictions(crRestriction opsgeniev1beta1.OpsgenieTimeRestriction) (*og.TimeRestriction, error) {
	if crRestriction.Type == "" {
		return nil, fmt.Errorf("time restriction type cannot be empty")
	}

	var restrictions []og.Restriction
	for _, i := range crRestriction.Intervals {
		startHour := uint32(i.StartHour)
		startMin := uint32(0) // Default to 0
		endHour := uint32(i.EndHour)
		endMin := uint32(0) // Default to 0

		startDay, err := mapDayIntToString(i.StartDay)
		if err != nil {
			return nil, err
		}
		endDay, err := mapDayIntToString(i.EndDay)
		if err != nil {
			return nil, err
		}

		restrictions = append(restrictions, og.Restriction{
			StartDay:  startDay,
			EndDay:    endDay,
			StartHour: &startHour,
			StartMin:  &startMin,
			EndHour:   &endHour,
			EndMin:    &endMin,
		})
	}

	return &og.TimeRestriction{
		Type:            og.RestrictionType(crRestriction.Type),
		RestrictionList: restrictions,
	}, nil
}

func mapDayIntToString(day int) (og.Day, error) {
	days := map[int]og.Day{
		1: og.Monday,
		2: og.Tuesday,
		3: og.Wednesday,
		4: og.Thursday,
		5: og.Friday,
		6: og.Saturday,
		7: og.Sunday,
	}

	d, exists := days[day]
	if !exists {
		return "", fmt.Errorf("invalid day value: %d (expected 1-7)", day)
	}
	return d, nil
}

func (r *OpsgenieScheduleReconciler) isUpdateRequired(cr *opsgeniev1beta1.OpsgenieSchedule, existing *schedule.GetResult, newRotations []og.Rotation) bool {
	return cr.Spec.Description != existing.Schedule.Description ||
		cr.Spec.TimeZone != existing.Schedule.Timezone ||
		!r.areRotationsSame(existing.Schedule.Rotations, newRotations)
}

func (r *OpsgenieScheduleReconciler) areRotationsSame(existing []og.Rotation, newRotations []og.Rotation) bool {
	if len(existing) != len(newRotations) {
		return false
	}
	for i := range existing {
		if existing[i].Name != newRotations[i].Name ||
			existing[i].Type != newRotations[i].Type ||
			!existing[i].StartDate.Equal(*newRotations[i].StartDate) ||
			existing[i].Length != newRotations[i].Length {
			return false
		}
	}
	return true
}

func (r *OpsgenieScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsgeniev1beta1.OpsgenieSchedule{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named("opsgenieschedule").
		Complete(r)
}
