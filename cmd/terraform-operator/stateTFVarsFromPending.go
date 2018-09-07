package main

import (
	"fmt"

	tftype "github.com/danisla/terraform-operator/pkg/types"
)

func getTFVarsFrom(parent *tftype.Terraform) (TerraformInputVars, error) {
	tfVars := make(TerraformInputVars, 0)

	for _, varsFrom := range parent.Spec.TFVarsFrom {

		if varsFrom.TFApply != "" && varsFrom.TFPlan != "" {
			// if both TFApply and TFPlan are provided in the varsFrom element, this is an OR condition for waiting and all vars are de-duped and merged.

			foundVars := false

			tfApplyVars, tfApplyErr := getVarsFromTF(parent, "TerraformApply", varsFrom.TFApply)
			tfPlanVars, tfPlanErr := getVarsFromTF(parent, "TerraformPlan", varsFrom.TFPlan)

			if tfApplyErr == nil {
				foundVars = true
				for k, v := range tfApplyVars {
					tfVars[k] = v
				}
			}

			if tfPlanErr == nil {
				foundVars = true
				for k, v := range tfPlanVars {
					tfVars[k] = v
				}
			}

			if foundVars == false {
				return tfVars, fmt.Errorf("Waiting for tfvarsFrom either tfapply or tfplan")
			} else {
				return tfVars, nil
			}

		} else if varsFrom.TFApply != "" {
			// Wait for TerraformApply vars
			return getVarsFromTF(parent, "TerraformApply", varsFrom.TFApply)
		} else if varsFrom.TFPlan != "" {
			// Wait for TerraformPlan vars
			return getVarsFromTF(parent, "TerraformPlan", varsFrom.TFPlan)
		}
	}

	return tfVars, nil
}

func getVarsFromTF(parent *tftype.Terraform, kind, name string) (TerraformInputVars, error) {
	tfVars := make(TerraformInputVars, 0)

	tf, err := getTerraform(kind, parent.ObjectMeta.Namespace, name)
	if err != nil {
		return tfVars, fmt.Errorf("Waiting for tfvarsFrom %s: %s", kind, name)
	}

	if len(tf.Spec.TFVars) > 0 {
		myLog(parent, "INFO", fmt.Sprintf("Including %d tfvars from %s %s", len(tf.Spec.TFVars), kind, name))
		for k, v := range tf.Spec.TFVars {
			tfVars[k] = v
		}
	} else {
		myLog(parent, "WARN", fmt.Sprintf("No TFVars found on tfvarsFrom %s spec: %s", kind, name))
	}

	return tfVars, nil
}
