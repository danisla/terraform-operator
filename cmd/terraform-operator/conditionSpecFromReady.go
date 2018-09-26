package main

import (
	"fmt"

	tfv1 "github.com/danisla/terraform-operator/pkg/types"
	"github.com/jinzhu/copier"
)

func reconcileSpecFromReady(condition *tfv1.Condition, parent *tfv1.Terraform, status *tfv1.TerraformOperatorStatus, children *TerraformChildren, desiredChildren *[]interface{}) (tfv1.ConditionStatus, *tfv1.TerraformSpec) {
	var spec tfv1.TerraformSpec
	newStatus := tfv1.ConditionFalse

	// Wait for any specFrom resource.
	var specFromType tfv1.TFKind
	var specFromName string
	if parent.SpecFrom != nil {
		if parent.SpecFrom.TFPlan != "" {
			specFromType = tfv1.TFKindPlan
			specFromName = parent.SpecFrom.TFPlan
		} else if parent.SpecFrom.TFApply != "" {
			specFromType = tfv1.TFKindApply
			specFromName = parent.SpecFrom.TFApply
		} else if parent.SpecFrom.TFDestroy != "" {
			specFromType = tfv1.TFKindDestroy
			specFromName = parent.SpecFrom.TFDestroy
		}
	}
	if specFromType != "" {
		specFromTF, err := getTerraform(specFromType, parent.GetNamespace(), specFromName)
		if err != nil {
			condition.Reason = fmt.Sprintf("Waiting for spec from: %s/%s", specFromType, specFromName)
		} else {
			copier.Copy(&spec, specFromTF.Spec)
			newStatus = tfv1.ConditionTrue
			condition.Reason = fmt.Sprintf("Using spec from: %s/%s", specFromType, specFromName)
		}
	} else {
		// Spec from parent.Spec
		newStatus = tfv1.ConditionTrue
	}

	return newStatus, &spec
}
