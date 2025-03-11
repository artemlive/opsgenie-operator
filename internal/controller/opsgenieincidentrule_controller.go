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
	"github.com/opsgenie/opsgenie-go-sdk-v2/alert"
	"github.com/opsgenie/opsgenie-go-sdk-v2/og"
	"github.com/opsgenie/opsgenie-go-sdk-v2/service"
)

// OpsgenieIncidentRuleReconciler reconciles an OpsgenieIncidentRule object
type OpsgenieIncidentRuleReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	OpsgenieClient *service.Client
}

// +kubebuilder:rbac:groups=opsgenie.macpaw.dev,resources=opsgenieincidentrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=opsgenie.macpaw.dev,resources=opsgenieincidentrules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=opsgenie.macpaw.dev,resources=opsgenieincidentrules/finalizers,verbs=update
func (r *OpsgenieIncidentRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the OpsgenieIncidentRule resource
	var ruleCR opsgeniev1beta1.OpsgenieIncidentRule
	if err := r.Get(ctx, req.NamespacedName, &ruleCR); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Resolve Service ID
	serviceID, err := r.resolveServiceID(ctx, &ruleCR)
	if err != nil {
		logger.Error(err, "Failed to resolve service ID")
		return ctrl.Result{}, err
	}

	// Fetch existing incident rules
	existingRules, err := r.OpsgenieClient.GetIncidentRules(ctx, &service.GetIncidentRulesRequest{ServiceId: serviceID})
	if err != nil {
		logger.Error(err, "Failed to fetch existing incident rules from Opsgenie")
		return ctrl.Result{}, err
	}

	// Check if the rule already exists
	var existingRule *service.IncidentRuleResult
	for _, rule := range existingRules.IncidentRule {
		if rule.Id == ruleCR.Status.RuleID {
			existingRule = &rule
			break
		}
	}

	if existingRule != nil {
		// **Update existing rule**
		logger.Info("Updating existing Opsgenie incident rule", "ruleID", ruleCR.Status.RuleID)

		updateReq := r.mapUpdateIncidentRule(&ruleCR, serviceID, ruleCR.Status.RuleID)
		if _, err := r.OpsgenieClient.UpdateIncidentRule(ctx, updateReq); err != nil {
			logger.Error(err, "Failed to update Opsgenie incident rule")
			return ctrl.Result{}, err
		}

		ruleCR.Status.LastSyncedTime = metav1.Now()
		if err := r.Status().Update(ctx, &ruleCR); err != nil {
			logger.Error(err, "Failed to update rule status after update")
			return ctrl.Result{}, err
		}

		logger.Info("Successfully updated Opsgenie incident rule", "ruleID", ruleCR.Status.RuleID)
	} else {
		// **Create new rule**
		logger.Info("Creating new Opsgenie incident rule", "name", ruleCR.Spec.ServiceRef.Name)

		createReq := r.mapCreateIncidentRule(&ruleCR, serviceID)
		createResp, err := r.OpsgenieClient.CreateIncidentRule(ctx, createReq)
		if err != nil {
			logger.Error(err, "Failed to create Opsgenie incident rule")
			return ctrl.Result{}, err
		}

		ruleCR.Status.RuleID = createResp.Id
		ruleCR.Status.Status = "Active"
		ruleCR.Status.LastSyncedTime = metav1.Now()
		if err := r.Status().Update(ctx, &ruleCR); err != nil {
			logger.Error(err, "Failed to update status after rule creation")
			return ctrl.Result{}, err
		}

		logger.Info("Successfully created Opsgenie incident rule", "ruleID", createResp.Id)
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// Resolve service ID from reference
func (r *OpsgenieIncidentRuleReconciler) resolveServiceID(ctx context.Context, ruleCR *opsgeniev1beta1.OpsgenieIncidentRule) (string, error) {
	if ruleCR.Spec.ServiceRef != nil {
		var serviceCR opsgeniev1beta1.OpsgenieService
		err := r.Get(ctx, client.ObjectKey{Name: ruleCR.Spec.ServiceRef.Name, Namespace: ruleCR.Namespace}, &serviceCR)
		if err != nil {
			return "", err
		}
		return serviceCR.Status.ServiceID, nil
	}

	return "", errors.New("serviceRef must be specified")
}

// SetupWithManager sets up the controller with the Manager
func (r *OpsgenieIncidentRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsgeniev1beta1.OpsgenieIncidentRule{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named("opsgenieincidentrule").
		Complete(r)
}

func (r *OpsgenieIncidentRuleReconciler) mapIncidentProperties(ruleCR *opsgeniev1beta1.OpsgenieIncidentRule) service.IncidentProperties {
	return service.IncidentProperties{
		Message:     ruleCR.Spec.IncidentProperties.Message,
		Tags:        ruleCR.Spec.IncidentProperties.Tags,
		Details:     ruleCR.Spec.IncidentProperties.Details,
		Description: ruleCR.Spec.IncidentProperties.Description,
		Priority:    alert.Priority(ruleCR.Spec.IncidentProperties.Priority),
		StakeholderProperties: service.StakeholderProperties{
			Enable:      ruleCR.Spec.IncidentProperties.StakeholderProperties.Enable,
			Message:     ruleCR.Spec.IncidentProperties.StakeholderProperties.Message,
			Description: ruleCR.Spec.IncidentProperties.StakeholderProperties.Description,
		},
	}
}

func (r *OpsgenieIncidentRuleReconciler) mapConditions(ruleCR *opsgeniev1beta1.OpsgenieIncidentRule) []og.Condition {
	var conditions []og.Condition
	for _, cond := range ruleCR.Spec.Conditions {
		conditions = append(conditions, og.Condition{
			Field:         og.ConditionFieldType(cond.Field),
			Operation:     og.ConditionOperation(cond.Operation),
			ExpectedValue: cond.ExpectedValue,
		})
	}
	return conditions
}

func (r *OpsgenieIncidentRuleReconciler) mapCreateIncidentRule(ruleCR *opsgeniev1beta1.OpsgenieIncidentRule, serviceID string) *service.CreateIncidentRuleRequest {
	return &service.CreateIncidentRuleRequest{
		ServiceId:          serviceID,
		Conditions:         r.mapConditions(ruleCR),
		ConditionMatchType: og.ConditionMatchType(ruleCR.Spec.ConditionMatchType),
		IncidentProperties: r.mapIncidentProperties(ruleCR),
	}
}

func (r *OpsgenieIncidentRuleReconciler) mapUpdateIncidentRule(ruleCR *opsgeniev1beta1.OpsgenieIncidentRule, serviceID, ruleID string) *service.UpdateIncidentRuleRequest {
	return &service.UpdateIncidentRuleRequest{
		ServiceId:          serviceID,
		IncidentRuleId:     ruleID,
		Conditions:         r.mapConditions(ruleCR),
		ConditionMatchType: og.ConditionMatchType(ruleCR.Spec.ConditionMatchType),
		IncidentProperties: r.mapIncidentProperties(ruleCR),
	}
}
