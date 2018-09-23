package main

import (
	"fmt"
	"strings"

	tfv1 "github.com/danisla/terraform-operator/pkg/types"
	corev1 "k8s.io/api/core/v1"
)

func reconcileTFPodReady(condition *tfv1.TerraformCondition, parent *tfv1.Terraform, status *tfv1.TerraformOperatorStatus, children *TerraformChildren, desiredChildren *[]interface{}, providerConfigKeys *ProviderConfigKeys, sourceData *TerraformConfigSourceData, tfInputVars *TerraformInputVars, tfVarsFrom *TerraformInputVars, tfplanfile string) tfv1.ConditionStatus {
	newStatus := tfv1.ConditionFalse
	reasons := make([]string, 0)
	podName := makeOrdinalPodName(parent, children)

	// Get the image and pull policy (or default) from the spec.
	image, imagePullPolicy := getImageAndPullPolicy(parent)

	// Get the backend bucket and backend prefix (or default) from the spec.
	backendBucket, backendPrefix := getBackendBucketandPrefix(parent)

	// Convert spec TFVars to TerraformInputVars
	tfVars := make(TerraformInputVars, 0)
	if parent.Spec.TFVars != nil {
		for _, v := range *parent.Spec.TFVars {
			tfVars[v.Name] = v.Value
		}
	}

	// Terraform Pod data
	tfp := TFPod{
		Image:              image,
		ImagePullPolicy:    imagePullPolicy,
		Namespace:          parent.GetNamespace(),
		ProjectID:          config.Project,
		Workspace:          fmt.Sprintf("%s-%s", parent.GetNamespace(), parent.GetName()),
		SourceData:         *sourceData,
		ProviderConfigKeys: *providerConfigKeys,
		BackendBucket:      backendBucket,
		BackendPrefix:      backendPrefix,
		TFParent:           parent.GetName(),
		TFPlan:             tfplanfile,
		TFInputs:           *tfInputVars,
		TFVarsFrom:         *tfVarsFrom,
		TFVars:             tfVars,
	}

	// status.Sources.ConfigMapHashes = *sourceData.ConfigMapHashes
	// status.Sources.EmbeddedConfigMaps = *sourceData.EmbeddedConfigMaps

	// Make Terraform Pod
	var newPod corev1.Pod
	var err error
	switch parent.GetTFKind() {
	case tfv1.TFKindPlan:
		newPod, err = tfp.makeTerraformPod(podName, []string{PLAN_POD_CMD})
	case tfv1.TFKindApply:
		newPod, err = tfp.makeTerraformPod(podName, []string{APPLY_POD_CMD})
	case tfv1.TFKindDestroy:
		newPod, err = tfp.makeTerraformPod(podName, []string{DESTROY_POD_CMD})
	default:
		// This should not happen.
		parent.Log("ERROR", fmt.Sprintf("Unhandled parentType in StateIdle: %s", parent.GetTFKind()))
	}
	if err != nil {
		reasons = append(reasons, fmt.Sprintf("Pod/%s: Failed to create pod: %v", podName, err))
	} else {
		if child := children.claimChildAndGetCurrent(newPod, desiredChildren); child != nil {
			// pod := child.(corev1.Pod)

		} else {
			reasons = append(reasons, fmt.Sprintf("Pod/%s: CREATED", podName))
		}
	}

	condition.Reason = strings.Join(reasons, ",")

	return newStatus
}
