package main

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/ghodss/yaml"
)

func stateTFPlanPending(parentType ParentType, parent *Terraform, status *TerraformControllerStatus, desiredChildren *[]interface{}) (string, error) {
	// Wait for tfplan
	var tfplan Terraform
	var err error
	tfplan, err = getTerraformPlan(parent.ObjectMeta.Namespace, parent.Spec.TFPlan)
	if err != nil {
		myLog(parent, "INFO", fmt.Sprintf("Waiting for TerraformPlan/%s", tfplan.Name))
		return StateTFPlanPending, nil
	}

	if tfplan.Status.PodStatus != PodStatusPassed || tfplan.Status.TFPlan == "" {
		myLog(parent, "INFO", fmt.Sprintf("Waiting for TerraformPlan/%s TFPlan", tfplan.Name))
		return StateTFPlanPending, nil
	}

	return StateWaitComplete, nil
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
