package main

import (
	"fmt"

	tfv1 "github.com/danisla/terraform-operator/pkg/types"
)

func reconcileSpecFromReady(condition *tfv1.Condition, parent *tfv1.Terraform, status *tfv1.TerraformOperatorStatus, children *TerraformChildren, desiredChildren *[]interface{}) (tfv1.ConditionStatus, *tfv1.TerraformSpec) {
	var spec *tfv1.TerraformSpec
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
			if specFromTF.SpecFrom != nil {
				// Cannot request spec from another specfrom
				condition.Reason = fmt.Sprintf("%s/%s is also specFrom: cannot reference another specFrom resource.", specFromType, specFromName)
			} else {
				// Wait for ready condition
				if parent.SpecFrom.WaitForReady {
					for _, c := range specFromTF.Status.Conditions {
						if c.Type == tfv1.ConditionReady {
							if c.Status == tfv1.ConditionTrue {
								spec = specFromTF.Spec
								newStatus = tfv1.ConditionTrue
								condition.Reason = fmt.Sprintf("Using spec from: %s/%s", specFromType, specFromName)
							} else {
								condition.Reason = fmt.Sprintf("Waiting for %s/%s condition: %s", string(specFromType), specFromName, tfv1.ConditionReady)
							}
							break
						}
					}
				} else {
					spec = specFromTF.Spec
					newStatus = tfv1.ConditionTrue
					condition.Reason = fmt.Sprintf("Using spec from: %s/%s", specFromType, specFromName)
				}
			}
		}
	} else {
		// Spec from parent.Spec
		newStatus = tfv1.ConditionTrue
	}

	return newStatus, spec
}
