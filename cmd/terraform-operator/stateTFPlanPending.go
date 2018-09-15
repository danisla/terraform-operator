package main

import (
	"fmt"

	tftype "github.com/danisla/terraform-operator/pkg/types"
)

func getTFPlanFile(parent *tftype.Terraform) (string, error) {
	// Wait for tfplan

	var tfplanFile string
	var tfplan tftype.Terraform
	var err error

	if parent.Spec.TFPlan != "" {

		tfplan, err = getTerraform("tfplan", parent.ObjectMeta.Namespace, parent.Spec.TFPlan)
		if err != nil {
			return tfplanFile, fmt.Errorf("Waiting for TerraformPlan/%s", parent.Spec.TFPlan)
		}

		if tfplan.Status.PodStatus != tftype.PodStatusPassed || tfplan.Status.TFPlan == "" {
			return tfplanFile, fmt.Errorf("Waiting for TerraformPlan/%s TFPlan", tfplan.Name)
		}

		tfplanFile = tfplan.Status.TFPlan

		myLog(parent, "INFO", fmt.Sprintf("Using plan from TerraformPlan/%s: '%s'", tfplan.Name, tfplanFile))
	}

	return tfplanFile, nil
}
