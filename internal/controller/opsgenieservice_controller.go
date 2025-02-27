package controller

import (
	"context"
	"errors"
	"time"

	"github.com/opsgenie/opsgenie-go-sdk-v2/service"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	opsgeniev1beta1 "github.com/artemlive/opsgenie-operator/api/v1beta1"
)

// OpsgenieServiceReconciler reconciles an OpsgenieService object
type OpsgenieServiceReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	OpsgenieClient *service.Client
}

// resolveTeamID retrieves the team ID from either TeamID or TeamRef
func (r *OpsgenieServiceReconciler) resolveTeamID(ctx context.Context, serviceCR *opsgeniev1beta1.OpsgenieService) (string, error) {
	if serviceCR.Spec.TeamID != "" {
		return serviceCR.Spec.TeamID, nil
	}

	if serviceCR.Spec.TeamRef != nil {
		var teamCR opsgeniev1beta1.OpsgenieTeam
		err := r.Get(ctx, client.ObjectKey{Name: serviceCR.Spec.TeamRef.Name, Namespace: serviceCR.Namespace}, &teamCR)
		if err != nil {
			return "", err
		}
		return teamCR.Status.TeamID, nil
	}

	return "", errors.New("either teamId or teamRef must be specified")
}

// Reconcile syncs Kubernetes OpsgenieService CRs with Opsgenie
func (r *OpsgenieServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the OpsgenieService resource
	var serviceCR opsgeniev1beta1.OpsgenieService
	if err := r.Get(ctx, req.NamespacedName, &serviceCR); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Resolve Team ID (either from direct field or reference)
	teamID, err := r.resolveTeamID(ctx, &serviceCR)
	if err != nil {
		logger.Error(err, "Failed to resolve team ID")
		return ctrl.Result{}, nil
	}
	serviceCR.Status.ResolvedTeamID = teamID

	// If the service already exists, fetch it from Opsgenie
	if serviceCR.Status.ServiceID != "" {
		existingService, err := r.OpsgenieClient.Get(ctx, &service.GetRequest{Id: serviceCR.Status.ServiceID})
		if err != nil {
			logger.Error(err, "Failed to fetch existing service from Opsgenie")
			return ctrl.Result{}, err
		}

		// Check if an update is needed
		if r.isUpdateRequired(&serviceCR, existingService) {
			logger.Info("Updating existing Opsgenie service", "serviceID", serviceCR.Status.ServiceID)

			updateReq := &service.UpdateRequest{
				Id:          serviceCR.Status.ServiceID,
				Name:        serviceCR.Spec.Name,
				Description: serviceCR.Spec.Description,
				Tags:        serviceCR.Spec.Tags,
			}

			if _, err := r.OpsgenieClient.Update(ctx, updateReq); err != nil {
				logger.Error(err, "Failed to update Opsgenie service")
				return ctrl.Result{}, err
			}

			// Update status
			serviceCR.Status.LastSyncedTime = metav1.Now()
			if err := r.Status().Update(ctx, &serviceCR); err != nil {
				logger.Error(err, "Failed to update service status after update")
				return ctrl.Result{}, err
			}

			logger.Info("Successfully updated Opsgenie service", "serviceID", serviceCR.Status.ServiceID)
		}
	} else {
		// Create a new Opsgenie service
		logger.Info("Creating new Opsgenie service", "name", serviceCR.Spec.Name)
		createReq := &service.CreateRequest{
			Name:        serviceCR.Spec.Name,
			Description: serviceCR.Spec.Description,
			Tags:        serviceCR.Spec.Tags,
			TeamId:      teamID,
		}

		createResp, err := r.OpsgenieClient.Create(ctx, createReq)
		if err != nil {
			logger.Error(err, "Failed to create Opsgenie service")
			return ctrl.Result{}, err
		}

		// Update status with Service ID
		serviceCR.Status.ServiceID = createResp.Id
		serviceCR.Status.Status = "Active"
		serviceCR.Status.LastSyncedTime = metav1.Now()
		if err := r.Status().Update(ctx, &serviceCR); err != nil {
			logger.Error(err, "Failed to update status after service creation")
			return ctrl.Result{}, err
		}

		logger.Info("Successfully created Opsgenie service", "serviceID", createResp.Id)
	}

	// Requeue after 5 minutes to keep in sync
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// isUpdateRequired checks if the Opsgenie service needs an update
func (r *OpsgenieServiceReconciler) isUpdateRequired(cr *opsgeniev1beta1.OpsgenieService, existing *service.GetResult) bool {
	return cr.Spec.Name != existing.Service.Name ||
		cr.Spec.Description != existing.Service.Description ||
		!equalStringSlices(cr.Spec.Tags, existing.Service.Tags)
}

// equalStringSlices checks if two slices contain the same elements
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]bool)
	for _, val := range a {
		aMap[val] = true
	}
	for _, val := range b {
		if !aMap[val] {
			return false
		}
	}
	return true
}

// SetupWithManager initializes the controller
func (r *OpsgenieServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsgeniev1beta1.OpsgenieService{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named("opsgenie-service").
		Complete(r)
}
