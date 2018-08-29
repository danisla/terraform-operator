package main

import (
	"fmt"
)

func stateSourcePending(parentType ParentType, parent *Terraform, status *TerraformControllerStatus, desiredChildren *[]interface{}) (string, error) {

	// Wait for ConfigMap source.
	_, err := getConfigMapSourceData(parent.ObjectMeta.Namespace, parent.Spec.Source.ConfigMap.Name)
	if err != nil {
		myLog(parent, "INFO", fmt.Sprintf("Waiting for source ConfigMap named '%s'", parent.Spec.Source.ConfigMap.Name))
	} else {
		// Got ConfigMap, transition to StateWaitComplete
		return StateWaitComplete, nil
	}

	return StateSourcePending, nil
}
