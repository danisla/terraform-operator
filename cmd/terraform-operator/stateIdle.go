package main

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func stateIdleHandler(parentType ParentType, parent *Terraform, status *TerraformControllerStatus, children *TerraformControllerRequestChildren, desiredChildren *[]interface{}) (string, error) {
	status.LastAppliedSig = calcParentSig(parent, "")

	// Map of provider config secret names to list of key names.
	providerConfigKeys := make(map[string][]string, 0)

	// Map of sourceData key names, used to mount as paths in container.
	sourceDataKeys := make([]string, 0)

	// Check for provider config secret. If not yet available, transition to StateProviderConfigPending
	if parent.Spec.ProviderConfig != nil {
		for _, c := range parent.Spec.ProviderConfig {
			if c.SecretName != "" {
				secretKeys, err := getProviderConfigSecret(parent.ObjectMeta.Namespace, c.SecretName)
				if err != nil {
					// Wait for secret to become available
					return StateProviderConfigPending, nil
				}
				providerConfigKeys[c.SecretName] = secretKeys
			}
		}
	}

	// Check for ConfigMap source. If not yet available, transition to StateSourcePending
	if parent.Spec.Source.ConfigMap.Name != "" {
		sourceData, err := getConfigMapSourceData(parent.ObjectMeta.Namespace, parent.Spec.Source.ConfigMap.Name)
		if err != nil {
			// Wait for configmap to become available.
			return StateSourcePending, nil
		}
		for k := range sourceData {
			sourceDataKeys = append(sourceDataKeys, k)
		}
		err = validateConfigMapSource(sourceData)
		if err != nil {
			myLog(parent, "ERROR", fmt.Sprintf("ConfigMap source data is invalid: %v", err))
			return status.StateCurrent, nil
		}
		shaSum, err := toSha1(sourceData)
		if err != nil {
			return status.StateCurrent, err
		}
		status.ConfigMapHash = shaSum
	}

	// Get the image and pull policy (or default) from the spec.
	image, imagePullPolicy := getImageAndPullPolicy(parent)

	// Terraform Pod data
	tfp := TFPod{
		Image:              image,
		ImagePullPolicy:    imagePullPolicy,
		Namespace:          parent.Namespace,
		ProjectID:          config.Project,
		ConfigMapName:      parent.Spec.Source.ConfigMap.Name,
		ProviderConfigKeys: providerConfigKeys,
		SourceDataKeys:     sourceDataKeys,
		ConfigMapHash:      status.ConfigMapHash,
		BackendBucket:      parent.Spec.BackendBucket,
		BackendPrefix:      parent.Spec.BackendPrefix,
		TFVars:             parent.Spec.TFVars,
	}

	// Make Terraform Pod
	var pod corev1.Pod
	var err error
	switch parentType {
	case ParentPlan:
		// Generate new ordinal pod name
		podName := makeOrdinalPodName(PLAN_POD_BASE_NAME, parent, children)

		pod, err = tfp.makeTerraformPod(podName, []string{PLAN_POD_CMD})
	default:
		// This should not happen.
		myLog(parent, "WARN", fmt.Sprintf("Unhandled parentType in StateIdle: %s", parentType))
	}
	if err != nil {
		myLog(parent, "ERROR", fmt.Sprintf("Failed to generate terraform pod: %v", err))
		return status.StateCurrent, nil
	}

	*desiredChildren = append(*desiredChildren, pod)

	status.PodName = pod.Name

	myLog(parent, "INFO", fmt.Sprintf("Created pod: %s", pod.Name))

	// Transition to StatePodRunning
	return StatePodRunning, nil
}
