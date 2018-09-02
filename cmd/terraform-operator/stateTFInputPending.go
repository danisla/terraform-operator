package main

import (
	"bytes"
	"fmt"
	"os/exec"

	tftype "github.com/danisla/terraform-operator/pkg/types"
	"github.com/ghodss/yaml"
)

func getTFInputs(parent *tftype.Terraform) (TerraformInputVars, error) {
	tfInputVars := make(TerraformInputVars, 0)

	// Wait for tfinputs
	for _, tfinput := range parent.Spec.TFInputs {
		tfapply, err := getTerraformApply(parent.ObjectMeta.Namespace, tfinput.Name)
		if err != nil {
			return tfInputVars, fmt.Errorf("Waiting for TerraformApply/%s", tfinput.Name)
		} else {
			if len(tfinput.VarMap) > 0 {
				for srcVar := range tfinput.VarMap {
					found := false
					for k := range tfapply.Status.TFOutput {
						if k == srcVar {
							found = true
							break
						}
					}
					if !found {
						return tfInputVars, fmt.Errorf("Input variable from TerraformApply/%s not found: %s", tfinput.Name, srcVar)
					}
				}
			} else {
				return tfInputVars, fmt.Errorf("Waiting for output variables from TerraformApply/%s", tfinput.Name)
			}

			for src, dest := range tfinput.VarMap {
				myLog(parent, "DEBUG", fmt.Sprintf("Creating var mapping from %s/%s -> %s", tfinput.Name, src, dest))
				tfInputVars[dest] = tfapply.Status.TFOutput[src].Value
			}
		}
	}

	return tfInputVars, nil
}

func getTerraformApply(namespace string, name string) (tftype.Terraform, error) {
	var tfapply tftype.Terraform
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("kubectl", "get", "tfapply", "-n", namespace, name, "-o", "yaml")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return tfapply, fmt.Errorf("Failed to run kubectl: %s\n%v", stderr.String(), err)
	}

	err = yaml.Unmarshal(stdout.Bytes(), &tfapply)

	return tfapply, err
}
