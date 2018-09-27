package main

import (
	"fmt"
	"strings"

	tfv1 "github.com/danisla/terraform-operator/pkg/types"
)

func reconcileTFInputsReady(condition *tfv1.Condition, parent *tfv1.Terraform, status *tfv1.TerraformOperatorStatus, children *TerraformChildren, desiredChildren *[]interface{}) (tfv1.ConditionStatus, TerraformInputVars) {
	newStatus := tfv1.ConditionFalse
	allFound := true
	reasons := make([]string, 0)

	tfInputVars := make(TerraformInputVars, 0)

	if parent.Spec.TFInputs != nil {
		for _, tfinput := range *parent.Spec.TFInputs {
			tfapply, err := getTerraform(tfv1.TFKindApply, parent.GetNamespace(), tfinput.Name)
			if err != nil {
				allFound = false
				reasons = append(reasons, fmt.Sprintf("%s/%s: WAITING", tfv1.TFKindApply, tfinput.Name))
			} else {
				varsFound := true
				for _, srcVar := range tfinput.VarMap {
					if tfapply.Status.TFOutput == nil || len(*tfapply.Status.TFOutput) == 0 {
						varsFound = false
						reasons = append(reasons, fmt.Sprintf("%s/%s: Waiting for output vars", tfv1.TFKindApply, tfinput.Name))
					} else {
						found := false
						for _, k := range *tfapply.Status.TFOutput {
							if k.Name == srcVar.Source {
								found = true
								tfInputVars[srcVar.Dest] = k.Value
								break
							}
						}
						if !found {
							varsFound = false
							reasons = append(reasons, fmt.Sprintf("%s/%s: Output not found: %s", tfv1.TFKindApply, tfinput.Name, srcVar))
						}
					}
				}
				if varsFound {
					ready := true
					if tfinput.WaitForReady {
						for _, c := range tfapply.Status.Conditions {
							if c.Type == tfv1.ConditionReady {
								if c.Status != tfv1.ConditionTrue {
									ready = false
								}
							}
						}
					}
					if ready {
						reasons = append(reasons, fmt.Sprintf("%s/%s: OUTPUTS READY", tfv1.TFKindApply, tfinput.Name))
					} else {
						reasons = append(reasons, fmt.Sprintf("%s/%s: Waiting for condition %s", tfv1.TFKindApply, tfinput.Name, tfv1.ConditionReady))
					}
				} else {
					allFound = false
				}
			}
		}
	}

	if allFound {
		newStatus = tfv1.ConditionTrue
	}

	condition.Reason = strings.Join(reasons, ",")

	return newStatus, tfInputVars
}
