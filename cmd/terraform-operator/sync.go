package main

import (
	"fmt"
	"strings"
	"time"

	tfv1 "github.com/danisla/terraform-operator/pkg/types"
	"github.com/jinzhu/copier"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func sync(parentType ParentType, parent *tfv1.Terraform, children *TerraformChildren) (*tfv1.TerraformOperatorStatus, *[]interface{}, error) {
	var err error
	var status tfv1.TerraformOperatorStatus
	copier.Copy(&status, &parent.Status)

	desiredChildren := make([]interface{}, 0)

	// Current time used for updating conditions
	tNow := metav1.NewTime(time.Now())

	// Verify required top level fields.
	if err = parent.Spec.Verify(); err != nil {
		parent.Log("ERROR", "Invalid spec: %v", err)
		status.Conditions = append(status.Conditions, tfv1.Condition{
			Type:               tfv1.ConditionReady,
			Status:             tfv1.ConditionFalse,
			LastProbeTime:      tNow,
			LastTransitionTime: tNow,
			Reason:             "Invalid spec",
			Message:            fmt.Sprintf("%v", err),
		})
		return &status, &desiredChildren, nil
	}

	// Map of condition types to conditions, converted to list of conditions after switch statement.
	conditions := parent.MakeConditions(tNow)
	conditionOrder := parent.GetConditionOrder()

	// Variables shared by multiple conditions
	var spec *tfv1.TerraformSpec
	var providerConfigKeys ProviderConfigKeys
	var sourceData TerraformConfigSourceData
	var tfInputVars TerraformInputVars
	var tfVarsFrom TerraformInputVars
	var tfplanfile string

	// Reconcile each condition.
	for _, conditionType := range conditionOrder {
		condition := conditions[conditionType]
		newStatus := condition.Status

		// Skip processing conditions with unmet dependencies.
		if err = conditions.CheckConditions(conditionType); err != nil {
			newStatus = tfv1.ConditionFalse
			condition.Reason = err.Error()
			if condition.Status != newStatus {
				condition.LastTransitionTime = tNow
				condition.Status = newStatus
			}
			continue
		}

		switch conditionType {
		case tfv1.ConditionSpecFromReady:
			newStatus, spec = reconcileSpecFromReady(condition, parent, &status, children, &desiredChildren)
			if spec != nil {
				parent.Spec = spec
			}

		case tfv1.ConditionProviderConfigReady:
			newStatus, providerConfigKeys = reconcileProviderConfigReady(condition, parent, &status, children, &desiredChildren)

		case tfv1.ConditionConfigSourceReady:
			newStatus, sourceData = reconcileConfigSourceReady(condition, parent, &status, children, &desiredChildren)

		case tfv1.ConditionInputsReady:
			newStatus, tfInputVars = reconcileTFInputsReady(condition, parent, &status, children, &desiredChildren)

		case tfv1.ConditionVarsFromReady:
			newStatus, tfVarsFrom = reconcileTFVarsFromReady(condition, parent, &status, children, &desiredChildren)

		case tfv1.ConditionPlanReady:
			newStatus, tfplanfile = reconcileTFPlanReady(condition, parent, &status, children, &desiredChildren)

		case tfv1.ConditionPodComplete:
			newStatus = reconcileTFPodReady(condition, parent, &status, children, &desiredChildren, &providerConfigKeys, &sourceData, &tfInputVars, &tfVarsFrom, tfplanfile)

		case tfv1.ConditionReady:
			newStatus = tfv1.ConditionTrue
			notReady := []string{}
			for _, c := range conditionOrder {
				if c != tfv1.ConditionReady && conditions[c].Status != tfv1.ConditionTrue {
					notReady = append(notReady, string(c))
					newStatus = tfv1.ConditionFalse
				}
			}
			if len(notReady) > 0 {
				condition.Reason = fmt.Sprintf("Waiting for conditions: %s", strings.Join(notReady, ","))
			} else {
				condition.Reason = "All conditions satisfied"
			}
		}

		if condition.Status != newStatus {
			condition.LastTransitionTime = tNow
			condition.Status = newStatus
		}
	}

	// Set the ordered condition status from the conditions map.
	newConditions := make([]tfv1.Condition, 0)
	for _, c := range parent.GetConditionOrder() {
		newConditions = append(newConditions, *conditions[c])
	}
	status.Conditions = newConditions

	return &status, &desiredChildren, nil
}
