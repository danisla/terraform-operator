package main

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/ghodss/yaml"
)

func stateTFInputPending(parentType ParentType, parent *Terraform, status *TerraformControllerStatus, desiredChildren *[]interface{}) (string, error) {

	// Wait for tfinputs
	allFound := true
	for _, tfinput := range parent.Spec.TFInputs {
		tfapply, err := getTerraformApply(parent.ObjectMeta.Namespace, tfinput.Name)
		if err != nil {
			myLog(parent, "INFO", fmt.Sprintf("Waiting for TerraformApply/%s", tfinput.Name))
			allFound = false
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
						myLog(parent, "WARN", fmt.Sprintf("Input variable from TerraformApply/%s not found: %s", tfinput.Name, srcVar))
					}
					allFound = allFound && found
				}
			} else {
				myLog(parent, "INFO", fmt.Sprintf("Waiting for output variables from TerraformApply/%s", tfinput.Name))
			}
		}
	}

	if allFound {
		// All intputs found, transition to StateWaitComplete
		myLog(parent, "INFO", "Found all required input variables")
		return StateWaitComplete, nil
	}

	return status.StateCurrent, nil
}

func getTerraformApply(namespace string, name string) (Terraform, error) {
	var tfapply Terraform
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
