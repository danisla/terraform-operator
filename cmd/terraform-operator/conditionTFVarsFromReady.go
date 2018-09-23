package main

import (
	"fmt"
	"strings"

	tfv1 "github.com/danisla/terraform-operator/pkg/types"
)

func reconcileTFVarsFromReady(condition *tfv1.TerraformCondition, parent *tfv1.Terraform, status *tfv1.TerraformOperatorStatus, children *TerraformChildren, desiredChildren *[]interface{}) (tfv1.ConditionStatus, TerraformInputVars) {
	var err error
	newStatus := tfv1.ConditionFalse
	allFound := true
	reasons := make([]string, 0)

	tfVars := make(TerraformInputVars, 0)

	if parent.Spec.TFVarsFrom != nil {
		for _, varsFrom := range *parent.Spec.TFVarsFrom {

			if varsFrom.TFApply != "" && varsFrom.TFPlan != "" {
				// if both TFApply and TFPlan are provided in the varsFrom element, this is an OR condition for waiting and all vars are de-duped and merged.

				foundVars := false

				tfApplyVars, tfApplyErr := getVarsFromTF(tfv1.TFKindApply, parent.GetNamespace(), varsFrom.TFApply)
				tfPlanVars, tfPlanErr := getVarsFromTF(tfv1.TFKindPlan, parent.GetNamespace(), varsFrom.TFPlan)

				if tfApplyErr == nil {
					foundVars = true
					for k, v := range tfApplyVars {
						tfVars[k] = v
					}
					reasons = append(reasons, fmt.Sprintf("%s/%s: %d vars", tfv1.TFKindApply, varsFrom.TFApply, len(tfApplyVars)))
				}

				if tfPlanErr == nil {
					foundVars = true
					for k, v := range tfPlanVars {
						tfVars[k] = v
					}
					reasons = append(reasons, fmt.Sprintf("%s/%s: %d vars", tfv1.TFKindPlan, varsFrom.TFPlan, len(tfPlanVars)))
				}

				if foundVars == false {
					allFound = false
					reasons = append(reasons, fmt.Sprintf("%s/%s || %s/%s: WAITING", tfv1.TFKindApply, varsFrom.TFApply, tfv1.TFKindPlan, varsFrom.TFPlan))
				}
			} else if varsFrom.TFApply != "" {
				// Wait for TerraformApply vars
				tfVars, err = getVarsFromTF(tfv1.TFKindApply, parent.GetNamespace(), varsFrom.TFApply)
				if err != nil {
					allFound = false
					reasons = append(reasons, fmt.Sprintf("%s/%s: WAITING", tfv1.TFKindApply, varsFrom.TFApply))
				}
			} else if varsFrom.TFPlan != "" {
				// Wait for TerraformPlan vars
				tfVars, err = getVarsFromTF(tfv1.TFKindPlan, parent.GetNamespace(), varsFrom.TFPlan)
				if err != nil {
					allFound = false
					reasons = append(reasons, fmt.Sprintf("%s/%s: WAITING", tfv1.TFKindPlan, varsFrom.TFPlan))
				}
			}
		}
	}

	if allFound {
		newStatus = tfv1.ConditionTrue
	}

	condition.Reason = strings.Join(reasons, ",")

	return newStatus, tfVars
}
