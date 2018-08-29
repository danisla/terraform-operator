package main

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/ghodss/yaml"
)

func getTFPlanFile(parent *Terraform) (string, error) {
	// Wait for tfplan

	var tfplanFile string
	var tfplan Terraform
	var err error

	if parent.Spec.TFPlan != "" {

		tfplan, err = getTerraformPlan(parent.ObjectMeta.Namespace, parent.Spec.TFPlan)
		if err != nil {
			return tfplanFile, fmt.Errorf("Waiting for TerraformPlan/%s", parent.Spec.TFPlan)
		}

		if tfplan.Status.PodStatus != PodStatusPassed || tfplan.Status.TFPlan == "" {
			return tfplanFile, fmt.Errorf("Waiting for TerraformPlan/%s TFPlan", tfplan.Name)
		}

		tfplanFile = tfplan.Status.TFPlan

		myLog(parent, "INFO", fmt.Sprintf("Using plan from TerraformPlan/%s: '%s'", tfplan.Name, tfplanFile))
	}

	return tfplanFile, nil
}

func getTerraformPlan(namespace string, name string) (Terraform, error) {
	var tfplan Terraform
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("kubectl", "get", "tfplan", "-n", namespace, name, "-o", "yaml")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return tfplan, fmt.Errorf("Failed to run kubectl: %s\n%v", stderr.String(), err)
	}

	err = yaml.Unmarshal(stdout.Bytes(), &tfplan)

	return tfplan, err
}
