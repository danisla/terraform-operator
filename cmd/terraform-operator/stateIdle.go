package main

import (
	"fmt"
)

func stateIdleHandler(parentType ParentType, parent *Terraform, status *TerraformControllerStatus, desiredChildren *[]interface{}) (string, error) {
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

	// Generate a new job name for each run, this will be postfixed with a timestamp.
	namePrefix := fmt.Sprintf("%s-", parent.Name)

	// Get the image and pull policy (or default) from the spec.
	image, imagePullPolicy := getImageAndPullPolicy(parent)

	myLog(parent, "DEBUG", fmt.Sprintf("Image: %s, pullPolicy: %s", image, imagePullPolicy))

	// Kustomization data
	tfk := TFKustomization{
		Image:              image,
		ImagePullPolicy:    imagePullPolicy,
		Namespace:          parent.Namespace,
		ConfigMapName:      parent.Spec.Source.ConfigMap.Name,
		ProviderConfigKeys: providerConfigKeys,
		SourceDataKeys:     sourceDataKeys,
		ConfigMapHash:      status.ConfigMapHash,
		BackendBucket:      parent.Spec.BackendBucket,
		BackendPrefix:      parent.Spec.BackendPrefix,
		TFVars:             parent.Spec.TFVars,
	}

	// Make Kustomization
	var kustomizationPath string
	var err error
	switch parentType {
	case ParentPlan:
		kustomizationPath, err = tfk.makeKustomization(PLAN_JOB_BASE_DIR, PLAN_JOB_BASE_NAME, namePrefix, []string{PLAN_JOB_CMD})
	default:
		// This should not happen.
		myLog(parent, "WARN", fmt.Sprintf("Unhandled parentType in StateIdle: %s", parentType))
	}
	if err != nil {
		myLog(parent, "ERROR", fmt.Sprintf("Failed to generate kustomization.yaml: %v", err))
		return status.StateCurrent, nil
	}

	myLog(parent, "DEBUG", fmt.Sprintf("Path to kustomization.yaml: %s", kustomizationPath))

	// Build the kustomization
	build, err := buildKustomization(kustomizationPath)
	if err != nil {
		myLog(parent, "ERROR", fmt.Sprintf("%v", err))
		return status.StateCurrent, nil
	}

	// Save build output to ConfigMap, add as child resource.
	kcmName := makeKustomizeConfigMapName(parent.Name, parentType)
	kcm, err := makeKustomizeBuildConfigMap(kcmName, build)
	if err != nil {
		myLog(parent, "ERROR", fmt.Sprintf("Failed to create ConfigMap for kustomize build output: %v", err))
		return status.StateCurrent, nil
	}
	*desiredChildren = append(*desiredChildren, kcm)
	status.KustomizeBuildConfigMap = kcmName
	myLog(parent, "INFO", fmt.Sprintf("Created Kustomization build ConfigMap: %s", kcmName))

	// Split build into child resources
	resources, err := splitKustomizeBuildOutput(build)
	if err != nil {
		myLog(parent, "ERROR", fmt.Sprintf("Failed to split Kustomize build output into separate resources: %v", err))
		return status.StateCurrent, nil
	}
	// Add each resource to the list of desired children
	for _, resource := range resources {
		kind, name := getResourceKindName(resource)
		myLog(parent, "INFO", fmt.Sprintf("Creating child %s: %s", kind, name))
		*desiredChildren = append(*desiredChildren, resource)
		if kind == "Job" {
			status.JobName = name
		}
	}

	return status.StateCurrent, nil
}
