package main

import (
	"fmt"
	"strings"

	tfv1 "github.com/danisla/terraform-operator/pkg/types"
)

func reconcileProviderConfigReady(condition *tfv1.TerraformCondition, parent *tfv1.Terraform, status *tfv1.TerraformOperatorStatus, children *TerraformChildren, desiredChildren *[]interface{}) (tfv1.ConditionStatus, ProviderConfigKeys) {
	newStatus := tfv1.ConditionFalse
	allFound := true
	reasons := make([]string, 0)

	// Map of provider config secret names to list of key names.
	providerConfigKeys := make(ProviderConfigKeys, 0)

	// Wait for all provider config secrets.
	if parent.Spec.ProviderConfig != nil {
		for _, c := range *parent.Spec.ProviderConfig {
			if c.SecretName != "" {
				secretKeys, err := getSecretKeys(parent.GetNamespace(), c.SecretName)
				if err != nil {
					// Wait for secret to become available
					allFound = false
				} else {
					providerConfigKeys[c.SecretName] = secretKeys
					reasons = append(reasons, fmt.Sprintf("Secret/%s", c.SecretName))
				}
			}
		}
	}

	if allFound {
		newStatus = tfv1.ConditionTrue
	}

	condition.Reason = strings.Join(reasons, ",")

	return newStatus, providerConfigKeys
}
