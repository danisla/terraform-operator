package main

import (
	"fmt"
)

func stateProviderConfigPending(parentType ParentType, parent *Terraform, status *TerraformControllerStatus, desiredChildren *[]interface{}) (string, error) {

	// Wait for Secrets.
	allFound := true
	for _, c := range parent.Spec.ProviderConfig {
		if c.SecretName != "" {
			_, err := getProviderConfigSecret(parent.ObjectMeta.Namespace, c.SecretName)
			if err != nil {
				myLog(parent, "INFO", fmt.Sprintf("Waiting for provider Secret named '%s'", c.SecretName))
				allFound = false
			}
		}
	}
	if allFound {
		// All provider secrets found, transition back to StateIdle
		return StateIdle, nil
	}

	return status.StateCurrent, nil
}
