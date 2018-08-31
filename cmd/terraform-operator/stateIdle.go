package main

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func stateIdle(parentType ParentType, parent *Terraform, status *TerraformOperatorStatus, children *TerraformOperatorRequestChildren, desiredChildren *[]interface{}) (TerraformOperatorState, error) {
	var err error

	if status.StateCurrent == StateIdle && !changeDetected(parent, children, status) {
		return StateIdle, nil
	}

	if active, _, _, _ := getPodStatus(children.Pods); active > 0 {
		// Pods should only be active in the StatePodRunning or StateRetry states.
		return StateNone, fmt.Errorf("pods active in StateIdle, re-sync collision")
	}

	// Generate new ordinal pod name
	podName := makeOrdinalPodName(parentType, parent, children)

	// Map of provider config secret names to list of key names.
	providerConfigKeys := make(map[string][]string, 0)

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

	// Wait for all config sources
	sourceData, err := getSourceData(parent, desiredChildren, podName)
	if err != nil {
		myLog(parent, "WARN", fmt.Sprintf("%v", err))
		return StateSourcePending, nil
	}

	// Wait for any TFInputs
	tfInputVars, err := getTFInputs(parent)
	if err != nil {
		myLog(parent, "WARN", fmt.Sprintf("%v", err))
		return StateTFInputPending, nil
	}

	// Wait for any TerraformPlan
	tfplanFile, err := getTFPlanFile(parent)
	if err != nil {
		myLog(parent, "WARN", fmt.Sprintf("%v", err))
		return StateTFPlanPending, nil
	}

	// Get the image and pull policy (or default) from the spec.
	image, imagePullPolicy := getImageAndPullPolicy(parent)

	// Terraform Pod data
	tfp := TFPod{
		Image:              image,
		ImagePullPolicy:    imagePullPolicy,
		Namespace:          parent.Namespace,
		ProjectID:          config.Project,
		Workspace:          fmt.Sprintf("%s-%s", parent.Namespace, parent.Name),
		SourceData:         sourceData,
		ProviderConfigKeys: providerConfigKeys,
		BackendBucket:      parent.Spec.BackendBucket,
		BackendPrefix:      parent.Spec.BackendPrefix,
		TFParent:           parent.Name,
		TFPlan:             tfplanFile,
		TFInputs:           tfInputVars,
		TFVars:             parent.Spec.TFVars,
	}

	status.Sources.ConfigMapHashes = *sourceData.ConfigMapHashes

	// Make Terraform Pod
	var pod corev1.Pod
	switch parentType {
	case ParentPlan:
		pod, err = tfp.makeTerraformPod(podName, []string{PLAN_POD_CMD})
	case ParentApply:
		pod, err = tfp.makeTerraformPod(podName, []string{APPLY_POD_CMD})
	case ParentDestroy:
		pod, err = tfp.makeTerraformPod(podName, []string{DESTROY_POD_CMD})
	default:
		// This should not happen.
		myLog(parent, "WARN", fmt.Sprintf("Unhandled parentType in StateIdle: %s", parentType))
	}
	if err != nil {
		myLog(parent, "ERROR", fmt.Sprintf("Failed to generate terraform pod: %v", err))
		return StateIdle, nil
	}

	*desiredChildren = append(*desiredChildren, pod)

	status.PodName = pod.Name
	status.Workspace = tfp.Workspace
	status.StateFile = makeStateFilePath(tfp.BackendBucket, tfp.BackendPrefix, tfp.Workspace)
	status.TFPlan = ""
	status.TFOutput = make(map[string]TerraformOutputVar, 0)
	status.StartedAt = ""
	status.FinishedAt = ""
	status.Duration = ""
	status.PodStatus = ""

	myLog(parent, "INFO", fmt.Sprintf("Created Pod: %s", pod.Name))

	// Transition to StatePodRunning
	return StatePodRunning, nil
}

func getPodStatus(pods map[string]corev1.Pod) (int, int, int, string) {
	lastActiveName := ""
	active := 0
	succeeded := 0
	failed := 0
	for _, pod := range pods {
		switch pod.Status.Phase {
		case corev1.PodSucceeded:
			succeeded++
		case corev1.PodFailed:
			failed++
		default:
			lastActiveName = pod.Name
			active++
		}
	}
	return active, succeeded, failed, lastActiveName
}
