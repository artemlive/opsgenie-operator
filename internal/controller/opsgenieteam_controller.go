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
	"time"

	"github.com/opsgenie/opsgenie-go-sdk-v2/team"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	opsgeniev1beta1 "github.com/artemlive/opsgenie-operator/api/v1beta1"
)

// OpsgenieTeamReconciler reconciles a OpsgenieTeam object
type OpsgenieTeamReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	OpsgenieClient *team.Client
}

// +kubebuilder:rbac:groups=opsgenie.macpaw.dev,resources=opsgenieteams,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=opsgenie.macpaw.dev,resources=opsgenieteams/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=opsgenie.macpaw.dev,resources=opsgenieteams/finalizers,verbs=update
func (r *OpsgenieTeamReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the OpsgenieTeam resource
	var teamCR opsgeniev1beta1.OpsgenieTeam
	if err := r.Get(ctx, req.NamespacedName, &teamCR); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// If the team already exists, fetch it from Opsgenie
	if teamCR.Status.TeamID != "" {
		existingTeam, err := r.OpsgenieClient.Get(ctx, &team.GetTeamRequest{
			IdentifierType:  team.Id,
			IdentifierValue: teamCR.Status.TeamID,
		})
		if err != nil {
			logger.Error(err, "Failed to fetch existing team from Opsgenie")
			return ctrl.Result{}, err
		}

		// Check if an update is needed
		if r.isUpdateRequired(&teamCR, existingTeam) {
			logger.Info("Updating existing Opsgenie team", "teamID", teamCR.Status.TeamID)

			updateReq := &team.UpdateTeamRequest{
				Id:          teamCR.Status.TeamID,
				Name:        teamCR.Spec.Name,
				Description: teamCR.Spec.Description,
			}

			if _, err := r.OpsgenieClient.Update(ctx, updateReq); err != nil {
				logger.Error(err, "Failed to update Opsgenie team")
				return ctrl.Result{}, err
			}

			// Update status
			teamCR.Status.LastSyncedTime = metav1.Now()
			if err := r.Status().Update(ctx, &teamCR); err != nil {
				logger.Error(err, "Failed to update team status after update")
				return ctrl.Result{}, err
			}

			logger.Info("Successfully updated Opsgenie team", "teamID", teamCR.Status.TeamID)
		}

		// Check if members need an update
		if r.isMembersUpdateRequired(&teamCR, existingTeam) {
			logger.Info("Updating team members", "teamID", teamCR.Status.TeamID)

			updateMembersReq := &team.UpdateTeamRequest{
				Id:      teamCR.Status.TeamID,
				Members: r.mapTeamMembers(teamCR.Spec.Members),
			}

			if _, err := r.OpsgenieClient.Update(ctx, updateMembersReq); err != nil {
				logger.Error(err, "Failed to update Opsgenie team members")
				return ctrl.Result{}, err
			}

			// Update last synced timestamp
			teamCR.Status.LastSyncedTime = metav1.Now()
			if err := r.Status().Update(ctx, &teamCR); err != nil {
				logger.Error(err, "Failed to update team status after members update")
				return ctrl.Result{}, err
			}

			logger.Info("Successfully updated Opsgenie team members", "teamID", teamCR.Status.TeamID)
		}

	} else {
		// Create a new Opsgenie team
		logger.Info("Creating new Opsgenie team", "name", teamCR.Spec.Name)

		createReq := &team.CreateTeamRequest{
			Name:        teamCR.Spec.Name,
			Description: teamCR.Spec.Description,
			Members:     r.mapTeamMembers(teamCR.Spec.Members),
		}

		createResp, err := r.OpsgenieClient.Create(ctx, createReq)
		if err != nil {
			logger.Error(err, "Failed to create Opsgenie team")
			return ctrl.Result{}, err
		}

		// Update status with Team ID
		teamCR.Status.TeamID = createResp.Id
		teamCR.Status.Status = "Active"
		teamCR.Status.LastSyncedTime = metav1.Now()
		if err := r.Status().Update(ctx, &teamCR); err != nil {
			logger.Error(err, "Failed to update status after team creation")
			return ctrl.Result{}, err
		}

		logger.Info("Successfully created Opsgenie team", "teamID", createResp.Id)
	}

	// Requeue after 5 minutes to keep in sync
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *OpsgenieTeamReconciler) mapTeamMembers(members []opsgeniev1beta1.OpsgenieTeamMember) []team.Member {
	var opsgenieMembers []team.Member

	for _, m := range members {
		opsgenieMembers = append(opsgenieMembers, team.Member{
			User: team.User{Username: m.UserID},
			Role: m.Role,
		})
	}

	return opsgenieMembers
}

func (r *OpsgenieTeamReconciler) isMembersUpdateRequired(cr *opsgeniev1beta1.OpsgenieTeam, existing *team.GetTeamResult) bool {
	if len(cr.Spec.Members) != len(existing.Members) {
		return true
	}

	existingMembersMap := make(map[string]string)
	for _, member := range existing.Members {
		existingMembersMap[member.User.Username] = member.Role
	}

	for _, newMember := range cr.Spec.Members {
		if existingRole, exists := existingMembersMap[newMember.UserID]; !exists || existingRole != newMember.Role {
			return true
		}
	}

	return false
}

// isUpdateRequired checks if the Opsgenie team needs an update
func (r *OpsgenieTeamReconciler) isUpdateRequired(cr *opsgeniev1beta1.OpsgenieTeam, existing *team.GetTeamResult) bool {
	return cr.Spec.Name != existing.Name ||
		cr.Spec.Description != existing.Description
}

// SetupWithManager initializes the controller
func (r *OpsgenieTeamReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsgeniev1beta1.OpsgenieTeam{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named("opsgenie-team").
		Complete(r)
}
