package main

import (
	"fmt"

	tfv1 "github.com/danisla/terraform-operator/pkg/types"
)

func reconcileTFPlanReady(condition *tfv1.Condition, parent *tfv1.Terraform, status *tfv1.TerraformOperatorStatus, children *TerraformChildren, desiredChildren *[]interface{}) (tfv1.ConditionStatus, string) {
	var tfplanfile string
	var tfplan tfv1.Terraform
	var err error
	newStatus := tfv1.ConditionFalse

	if parent.Spec.TFPlan != "" {
		tfplan, err = getTerraform(tfv1.TFKindPlan, parent.GetNamespace(), parent.Spec.TFPlan)
		if err != nil {
			condition.Reason = fmt.Sprintf("%s/%s: WAITING", tfv1.TFKindPlan, parent.Spec.TFPlan)
		} else {
			tfplanfile = tfplan.Status.TFPlan
		}
	}

	if tfplanfile != "" {
		newStatus = tfv1.ConditionTrue
		condition.Reason = fmt.Sprintf("%s/%s: READY", tfv1.TFKindPlan, tfplan.Name)
	}

	return newStatus, tfplanfile
}
