package main

import (
	"fmt"

	tftype "github.com/danisla/terraform-operator/pkg/types"
)

func getTFVarsFrom(parent *tftype.Terraform) (TerraformInputVars, error) {
	tfVars := make(TerraformInputVars, 0)

	for _, varsFrom := range parent.Spec.TFVarsFrom {
		if varsFrom.TFApply != "" {
			tfApplyName := varsFrom.TFApply

			tfapply, err := getTerraformApply(parent.Namespace, tfApplyName)
			if err != nil {
				return tfVars, fmt.Errorf("Waiting for tfvarsFrom TerraformApply: %s", tfApplyName)
			}

			if len(tfapply.Spec.TFVars) > 0 {
				myLog(parent, "INFO", fmt.Sprintf("Including %d tfvars from TerraformApply %s", len(tfapply.Spec.TFVars), tfApplyName))
				for k, v := range tfapply.Spec.TFVars {
					tfVars[k] = v
				}
			} else {
				myLog(parent, "WARN", fmt.Sprintf("No TFVars found on tfvarsFrom TerraformApply spec: %s", tfApplyName))
			}
		}
	}

	return tfVars, nil
}
