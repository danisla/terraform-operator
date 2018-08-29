package main

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func stateIdle(parentType ParentType, parent *Terraform, status *TerraformControllerStatus, children *TerraformControllerRequestChildren, desiredChildren *[]interface{}) (string, error) {

	if active, _, _, _ := getPodStatus(children.Pods); active > 0 {
		// Pods should only be active in the StatePodRunning or StateRetry states.
		return StateNone, fmt.Errorf("pods active in StateIdle, re-sync collision")
	}

	// Generate new ordinal pod name
	podName := makeOrdinalPodName(parentType, parent, children)

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
			return StateIdle, nil
		}
		configMapHash, err = toSha1(sourceData)
		if err != nil {
			return StateIdle, err
		}
	} else if parent.Spec.Source.Embedded != "" {
		myLog(parent, "INFO", "Using Terraform source embedded in spec")

		configMapHash, _ = toSha1(parent.Spec.Source.Embedded)

		configMapName = fmt.Sprintf("%s-%s", podName, configMapHash[0:4])

		cm := makeTerraformSourceConfigMap(configMapName, parent.Spec.Source.Embedded)

		for k := range cm.Data {
			sourceDataKeys = append(sourceDataKeys, k)
		}

		*desiredChildren = append(*desiredChildren, cm)

		myLog(parent, "INFO", fmt.Sprintf("Created ConfigMap: %s", configMapName))

	} else {
		myLog(parent, "WARN", "No terraform source defined. Deadend.")
		return StateIdle, nil
	}

	// Check for TFInputs
	tfInputVars := make(map[string]string, 0)
	if len(parent.Spec.TFInputs) > 0 {
		allFound := true
		for _, tfinput := range parent.Spec.TFInputs {
			tfapply, err := getTerraformApply(parent.ObjectMeta.Namespace, tfinput.Name)
			if err != nil {
				// Wait for TerraformApply to become available
				return StateTFInputPending, nil
			}
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
				return StateTFInputPending, nil
			}
			if !allFound {
				return StateTFInputPending, nil
			}

			for src, dest := range tfinput.VarMap {
				myLog(parent, "DEBUG", fmt.Sprintf("Creating var mapping from %s/%s -> %s", tfinput.Name, src, dest))
				tfInputVars[dest] = tfapply.Status.TFOutput[src].Value
			}
		}
	}

	// Check for TFPlan
	var tfplanFile string
	if parent.Spec.TFPlan != "" {
		tfplan, err := getTerraformPlan(parent.ObjectMeta.Namespace, parent.Spec.TFPlan)
		if err != nil {
			// Wait for TerraformPlan to become available
			return StateTFPlanPending, nil
		}
		if tfplan.Status.PodStatus != PodStatusPassed || tfplan.Status.TFPlan == "" {
			myLog(parent, "INFO", fmt.Sprintf("Waiting for TerraformPlan/%s TFPlan", tfplan.Name))
			return StateTFPlanPending, nil
		}
		tfplanFile = tfplan.Status.TFPlan

		myLog(parent, "INFO", fmt.Sprintf("Using plan from TerraformPlan/%s: '%s'", tfplan.Name, tfplanFile))
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
		ConfigMapName:      configMapName,
		ProviderConfigKeys: providerConfigKeys,
		SourceDataKeys:     sourceDataKeys,
		ConfigMapHash:      configMapHash,
		BackendBucket:      parent.Spec.BackendBucket,
		BackendPrefix:      parent.Spec.BackendPrefix,
		TFPlan:             tfplanFile,
		TFInputs:           tfInputVars,
		TFVars:             parent.Spec.TFVars,
	}

	status.ConfigMapHash = configMapHash

	// Make Terraform Pod
	var pod corev1.Pod
	var err error
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
