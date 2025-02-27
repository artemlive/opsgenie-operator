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

package controller

import (
	"context"
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	opsgeniev1beta1 "github.com/artemlive/opsgenie-operator/api/v1beta1"
	"github.com/opsgenie/opsgenie-go-sdk-v2/escalation"
	"github.com/opsgenie/opsgenie-go-sdk-v2/og"
)

// OpsgenieEscalationReconciler reconciles a OpsgenieEscalation object
type OpsgenieEscalationReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	OpsgenieClient *escalation.Client
}

// +kubebuilder:rbac:groups=opsgenie.macpaw.dev,resources=opsgenieescalations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=opsgenie.macpaw.dev,resources=opsgenieescalations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=opsgenie.macpaw.dev,resources=opsgenieescalations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpsgenieEscalation object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *OpsgenieEscalationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the OpsgenieEscalation resource
	var escCR opsgeniev1beta1.OpsgenieEscalation
	if err := r.Get(ctx, req.NamespacedName, &escCR); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Resolve Team ID (either from direct field or reference)
	teamID, err := r.resolveTeamID(ctx, &escCR)
	if err != nil {
		logger.Error(err, "Failed to resolve team ID")
		return ctrl.Result{}, nil
	}
	escCR.Status.ResolvedTeamID = teamID

	mapOfRules, err := r.mapEscalationRules(ctx, &escCR)
	if err != nil {
		logger.Error(err, "Failed to map escalation rules")
		return ctrl.Result{}, err
	}
	// If escalation already exists, fetch it from Opsgenie
	if escCR.Status.EscalationID != "" {
		existingEscalation, err := r.OpsgenieClient.Get(ctx, &escalation.GetRequest{
			IdentifierType: escalation.Id,
			Identifier:     escCR.Status.EscalationID,
		})
		if err != nil {
			logger.Error(err, "Failed to fetch existing escalation from Opsgenie")
			return ctrl.Result{}, err
		}

		// Check if an update is needed
		if r.isUpdateRequired(&escCR, existingEscalation) {
			logger.Info("Updating existing Opsgenie escalation", "escalationID", escCR.Status.EscalationID)
			updateReq := &escalation.UpdateRequest{
				IdentifierType: escalation.Id,
				Identifier:     escCR.Status.EscalationID,
				Name:           escCR.Spec.Name,
				Description:    escCR.Spec.Description,
				Rules:          mapOfRules,
				OwnerTeam: &og.OwnerTeam{
					Id: teamID,
				},
			}

			if _, err := r.OpsgenieClient.Update(ctx, updateReq); err != nil {
				logger.Error(err, "Failed to update Opsgenie escalation")
				return ctrl.Result{}, err
			}

			// Update status
			escCR.Status.LastSyncedTime = metav1.Now()
			if err := r.Status().Update(ctx, &escCR); err != nil {
				logger.Error(err, "Failed to update escalation status after update")
				return ctrl.Result{}, err
			}

			logger.Info("Successfully updated Opsgenie escalation", "escalationID", escCR.Status.EscalationID)
		}
	} else {
		// Create a new Opsgenie escalation
		logger.Info("Creating new Opsgenie escalation", "name", escCR.Spec.Name)

		createReq := &escalation.CreateRequest{
			Name:        escCR.Spec.Name,
			Description: escCR.Spec.Description,
			Rules:       mapOfRules,
			OwnerTeam:   &og.OwnerTeam{Id: teamID},
		}

		createResp, err := r.OpsgenieClient.Create(ctx, createReq)
		if err != nil {
			logger.Error(err, "Failed to create Opsgenie escalation")
			return ctrl.Result{}, err
		}

		// Update status with Escalation ID
		escCR.Status.EscalationID = createResp.Id
		escCR.Status.Status = "Active"
		escCR.Status.LastSyncedTime = metav1.Now()
		if err := r.Status().Update(ctx, &escCR); err != nil {
			logger.Error(err, "Failed to update status after escalation creation")
			return ctrl.Result{}, err
		}

		logger.Info("Successfully created Opsgenie escalation", "escalationID", createResp.Id)
	}

	// Requeue after 5 minutes to keep in sync
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// resolveTeamID retrieves the team ID from either TeamID or TeamRef
func (r *OpsgenieEscalationReconciler) resolveTeamID(ctx context.Context, escCR *opsgeniev1beta1.OpsgenieEscalation) (string, error) {
	if escCR.Spec.TeamID != "" {
		return escCR.Spec.TeamID, nil
	}

	if escCR.Spec.TeamRef != nil {
		var teamCR opsgeniev1beta1.OpsgenieTeam
		err := r.Get(ctx, client.ObjectKey{Name: escCR.Spec.TeamRef.Name, Namespace: escCR.Namespace}, &teamCR)
		if err != nil {
			return "", err
		}
		return teamCR.Status.TeamID, nil
	}

	return "", errors.New("either teamId or teamRef must be specified")
}

func (r *OpsgenieEscalationReconciler) resolveRecipientID(ctx context.Context, recipient opsgeniev1beta1.OpsgenieEscalationRecipient, namespace string) (string, string, error) {
	// If `id` is set, use it directly
	if recipient.ID != "" {
		return recipient.ID, "", nil
	}

	// If `username` is set for `user` type recipients, use it
	if recipient.Type == "user" && recipient.Username != "" {
		return "", recipient.Username, nil
	}

	// If `teamRef` is set, resolve it
	if recipient.TeamRef != nil {
		var teamCR opsgeniev1beta1.OpsgenieTeam
		err := r.Get(ctx, client.ObjectKey{Name: recipient.TeamRef.Name, Namespace: namespace}, &teamCR)
		if err != nil {
			return "", "", err
		}
		return teamCR.Status.TeamID, "", nil
	}

	// If `scheduleRef` is set, resolve it
	if recipient.ScheduleRef != nil {
		var scheduleCR opsgeniev1beta1.OpsgenieSchedule
		err := r.Get(ctx, client.ObjectKey{Name: recipient.ScheduleRef.Name, Namespace: namespace}, &scheduleCR)
		if err != nil {
			return "", "", err
		}
		return scheduleCR.Status.ScheduleID, "", nil
	}

	return "", "", errors.New("either id, username, teamRef, or scheduleRef must be specified")
}

// mapEscalationRules converts Kubernetes CR rules to Opsgenie API format
func (r *OpsgenieEscalationReconciler) mapEscalationRules(ctx context.Context, cr *opsgeniev1beta1.OpsgenieEscalation) ([]escalation.RuleRequest, error) {
	var opsgenieRules []escalation.RuleRequest
	for _, rule := range cr.Spec.Rules {
		// Resolve recipient ID or username
		recipientID, recipientUsername, err := r.resolveRecipientID(ctx, rule.Recipient, cr.Namespace)
		if err != nil {
			return nil, err
		}

		// Build the recipient object
		recipient := og.Participant{
			Type: og.ParticipantType(rule.Recipient.Type),
		}

		// Assign ID or Username based on available values
		if recipientID != "" {
			recipient.Id = recipientID
		} else if recipientUsername != "" {
			recipient.Username = recipientUsername
		}

		opsgenieRules = append(opsgenieRules, escalation.RuleRequest{
			Condition:  og.EscalationCondition(rule.Condition),
			NotifyType: og.NotifyType(rule.NotifyType),
			Recipient:  recipient,
			Delay: escalation.EscalationDelayRequest{
				TimeAmount: uint32(rule.DelayMin),
			},
		})
	}
	return opsgenieRules, nil
}

func (r *OpsgenieEscalationReconciler) isUpdateRequired(cr *opsgeniev1beta1.OpsgenieEscalation, existing *escalation.GetResult) bool {
	// Check if basic fields are different
	if cr.Spec.Name != existing.Name || cr.Spec.Description != existing.Description {
		return true
	}

	// Check if rules have changed
	return !r.compareEscalationRules(cr.Spec.Rules, existing.Rules)
}

// compareEscalationRules checks if rules have changed
func (r *OpsgenieEscalationReconciler) compareEscalationRules(crRules []opsgeniev1beta1.OpsgenieEscalationRule, opsgenieRules []escalation.Rule) bool {
	if len(crRules) != len(opsgenieRules) {
		return false
	}

	for i, rule := range crRules {
		opsRule := opsgenieRules[i]

		// Compare condition
		if og.EscalationCondition(rule.Condition) != opsRule.Condition {
			return false
		}

		// Compare notify type
		if og.NotifyType(rule.NotifyType) != opsRule.NotifyType {
			return false
		}

		// Compare delay (Opsgenie only has `TimeAmount`, no `TimeUnit`)
		if uint32(rule.DelayMin) != opsRule.Delay.TimeAmount {
			return false
		}

		// Compare recipient (fix ID -> Name)
		if og.ParticipantType(rule.Recipient.Type) != opsRule.Recipient.Type || rule.Recipient.ID != opsRule.Recipient.Name {
			return false
		}
	}

	return true
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpsgenieEscalationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsgeniev1beta1.OpsgenieEscalation{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named("opsgenieescalation").
		Complete(r)
}
