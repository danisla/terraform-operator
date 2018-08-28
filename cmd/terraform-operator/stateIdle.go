package main

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func stateIdleHandler(parentType ParentType, parent *Terraform, status *TerraformControllerStatus, children *TerraformControllerRequestChildren, desiredChildren *[]interface{}) (string, error) {

	if active, _, _, _ := getPodStatus(children.Pods); active > 0 {
		// Pods should only be active in the StatePodRunning or StateRetry states.
		return StateNone, fmt.Errorf("re-sync collision")
	}

	// Map of provider config secret names to list of key names.
	providerConfigKeys := make(map[string][]string, 0)

	// Map of sourceData key names, used to mount as paths in container.
	sourceDataKeys := make([]string, 0)

	configMapName := ""

	configMapHash := status.ConfigMapHash

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
		myLog(parent, "INFO", fmt.Sprintf("Using Terraform source from ConfigMap: %s", parent.Spec.Source.ConfigMap.Name))
		configMapName = parent.Spec.Source.ConfigMap.Name

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
		configMapHash, err = toSha1(sourceData)
		if err != nil {
			return status.StateCurrent, err
		}
	} else if parent.Spec.Source.Embedded != "" {
		myLog(parent, "INFO", "Using Terraform source embedded in spec")

		// ConfigMap name in the form of: PARENT_NAME-PARENT_TYPE-src
		configMapName = fmt.Sprintf("%s-%s-src", parent.Name, parentType)

		var cm corev1.ConfigMap
		configMapHash, cm = makeTerraformSourceConfigMap(configMapName, parent.Spec.Source.Embedded)

		*desiredChildren = append(*desiredChildren, cm)

		myLog(parent, "INFO", fmt.Sprintf("Created ConfigMap: %s", cm.Name))

	} else {
		myLog(parent, "WARN", "No terraform source defined. Deadend.")
		return status.StateCurrent, nil
	}

	// Get the image and pull policy (or default) from the spec.
	image, imagePullPolicy := getImageAndPullPolicy(parent)

	// Terraform Pod data
	tfp := TFPod{
		Image:              image,
		ImagePullPolicy:    imagePullPolicy,
		Namespace:          parent.Namespace,
		ProjectID:          config.Project,
		ConfigMapName:      configMapName,
		ProviderConfigKeys: providerConfigKeys,
		SourceDataKeys:     sourceDataKeys,
		ConfigMapHash:      configMapHash,
		BackendBucket:      parent.Spec.BackendBucket,
		BackendPrefix:      parent.Spec.BackendPrefix,
		TFVars:             parent.Spec.TFVars,
	}

	status.ConfigMapHash = configMapHash

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
