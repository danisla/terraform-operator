package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

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
		parent.Log("ERROR", fmt.Sprintf("Unhandled parent kind in switch: %s", parent.GetTFKind()))
		return condition.Status
	}
	if err != nil {
		reasons = append(reasons, fmt.Sprintf("Pod/%s: Failed to create pod: %v", podName, err))
		return condition.Status
	}

	pod, ok := children.Pods[status.PodName]
	if ok == false {
		// Pod not yet created.
		status.PodName = podName
		children.claimChildAndGetCurrent(newPod, desiredChildren)
		condition.Reason = fmt.Sprintf("Pod/%s: CREATED", podName)
		parent.Log("INFO", "Creating Pod/%s", podName)
		return newStatus
	}

	// Set podName to existing pod name, rather than new ordinal name
	podName = pod.GetName()

	// Claim the pod
	*desiredChildren = append(*desiredChildren, pod)

	// Check status of init containers
	for _, cStatus := range pod.Status.InitContainerStatuses {
		switch cStatus.Name {
		case GCS_TARBALL_CONTAINER_NAME:
			switch pod.Status.Phase {
			case corev1.PodFailed:
				setFinalPodStatus(parent, status, cStatus, pod, tfv1.PodStatusFailed)
				maxRetry := getPodMaxAttempts(parent)

				// Attempt retry
				status.RetryCount++

				reasons = append(reasons, fmt.Sprintf("Pod/%s.%s: Attempt (%d/%d): %s", podName, cStatus.Name, status.RetryCount, maxRetry, cStatus.State.Terminated.Message))

				if status.RetryCount >= maxRetry {
					// Retries exceeded, reset and attept again. This is a continuous retry loop with exponential backoff.
					status.RetryCount = 0
					status.RetryNextAt = ""
				} else {
					finishedAt, err := time.Parse(time.RFC3339, status.FinishedAt)
					if err != nil {
						parent.Log("ERROR", "Failed to parse time: %v", err)
						reasons = append(reasons, "Internal error")
					} else {
						backoff := computeExponentialBackoff(status.RetryCount, tfDriverConfig.BackoffScale)
						timeSinceFinished := time.Since(finishedAt)
						if timeSinceFinished.Seconds() >= backoff {
							// Done waiting for backoff.

							// Generate a new ordinal pod child
							status.PodName = podName
							children.claimChildAndGetCurrent(newPod, desiredChildren)
							parent.Log("INFO", "Creating Pod/%s", podName)
						}
						nextAttemptTime := finishedAt.Add(time.Second * time.Duration(int64(backoff)))
						status.RetryNextAt = nextAttemptTime.Format(time.RFC3339)
					}
				}
			}
		}
	} // End init container check.

	// Check pod containers
	for _, cStatus := range pod.Status.ContainerStatuses {
		switch cStatus.Name {
		case TERRAFORM_CONTAINER_NAME:

			switch pod.Status.Phase {
			case corev1.PodSucceeded:
				// Passed
				setFinalPodStatus(parent, status, cStatus, pod, tfv1.PodStatusPassed)
				status.RetryCount = 0
				status.RetryNextAt = ""

				// Extract terraform plan path from annotation.
				switch parent.GetTFKind() {
				case tfv1.TFKindPlan:
					// Populate status.TFPlan from completed pod annotation.
					if plan, ok := pod.Annotations["terraform-plan"]; ok == true {
						status.TFPlan = plan

						// Parse the plan
						summary, err := parseTerraformPlan(plan)
						if err != nil {
							parent.Log("ERROR", "Failed to parse plan: %s, %v", plan, err)
							condition.Reason = "Internal error"
							return condition.Status
						}

						status.TFPlanDiff = &summary

					} else {
						parent.Log("ERROR", "Pod/%s is missing terraform-plan annotation", podName)
						condition.Reason = "Internal error"
						return condition.Status
					}
				case tfv1.TFKindApply:
					// Populate status.TFOutput map from completed pod annotation.
					if output, ok := pod.Annotations["terraform-output"]; ok == true {
						outputVars, err := makeOutputVars(output)
						if err != nil {
							parent.Log("ERROR", "Pod/%s: Failed to parse output vars: %v", podName, err)
							condition.Reason = "Internal error"
							return condition.Status
						}
						status.TFOutput = outputVars

						// Create Secret with output var map
						secretName := fmt.Sprintf("%s-tfapply-outputs", parent.GetName())
						secret := makeOutputVarsSecret(secretName, parent.GetNamespace(), outputVars)
						children.claimChildAndGetCurrent(secret, desiredChildren)
						status.TFOutputSecret = secret.GetName()
					} else {
						parent.Log("ERROR", "Pod/%s is missing terraform-output annotation", podName)
						condition.Reason = "Internal error"
						return condition.Status
					}
				}

			case corev1.PodFailed:
				// Failed
				setFinalPodStatus(parent, status, cStatus, pod, tfv1.PodStatusFailed)
				maxRetry := getPodMaxAttempts(parent)

				// Attempt retry
				status.RetryCount++

				reasons = append(reasons, fmt.Sprintf("Pod/%s.%s: Attempt (%d/%d): %s", podName, cStatus.Name, status.RetryCount, maxRetry, cStatus.State.Terminated.Message))

				if status.RetryCount >= maxRetry {
					// Retries exceeded, reset and attept again. This is a continuous retry loop with exponential backoff.
					status.RetryCount = 0
					status.RetryNextAt = ""
				} else {
					finishedAt, err := time.Parse(time.RFC3339, status.FinishedAt)
					if err != nil {
						parent.Log("ERROR", "Failed to parse time: %v", err)
						reasons = append(reasons, "Internal error")
					} else {
						backoff := computeExponentialBackoff(status.RetryCount, tfDriverConfig.BackoffScale)
						timeSinceFinished := time.Since(finishedAt)
						if timeSinceFinished.Seconds() >= backoff {
							// Done waiting for backoff.

							// Generate a new ordinal pod child
							status.PodName = podName
							children.claimChildAndGetCurrent(newPod, desiredChildren)
							parent.Log("INFO", "Creating Pod/%s", podName)
						}
						nextAttemptTime := finishedAt.Add(time.Second * time.Duration(int64(backoff)))
						status.RetryNextAt = nextAttemptTime.Format(time.RFC3339)
					}
				}

			default:
				// Active
				if cStatus.State.Running != nil {
					status.StartedAt = cStatus.State.Running.StartedAt.Format(time.RFC3339)
				}

				status.RetryNextAt = ""
				status.PodStatus = tfv1.PodStatusRunning
				reasons = append(reasons, fmt.Sprintf("Pod/%s: RUNNING", podName))
			}
		}
	} // End container checks

	condition.Reason = strings.Join(reasons, ",")

	return newStatus
}

func computeExponentialBackoff(retryCount int32, scaleFactor float64) float64 {
	return ((math.Pow(2, float64(retryCount+1)) - 1) / 2.0) * scaleFactor
}

func setFinalPodStatus(parent *tfv1.Terraform, status *tfv1.TerraformOperatorStatus, cStatus corev1.ContainerStatus, pod corev1.Pod, podStatus tfv1.PodStatus) {
	status.PodStatus = podStatus
	status.FinishedAt = cStatus.State.Terminated.FinishedAt.Format(time.RFC3339)

	if status.StartedAt == "" {
		status.StartedAt = pod.Status.StartTime.Format(time.RFC3339)
	}

	// Set Duration in seconds
	startTime, _ := time.Parse(time.RFC3339, status.StartedAt)
	duration := cStatus.State.Terminated.FinishedAt.Sub(startTime)
	status.Duration = fmt.Sprintf("%02.0fm%02.0fs", duration.Minutes(), duration.Seconds())
}

func getPodMaxAttempts(parent *tfv1.Terraform) int32 {
	maxAttempts := tfDriverConfig.MaxAttempts
	if parent.Spec.MaxAttempts != nil {
		maxAttempts = *parent.Spec.MaxAttempts
	}
	return maxAttempts
}

func makeOutputVars(data string) (*[]tfv1.TerraformOutputVar, error) {
	var outputVarsMap map[string]tfv1.TerraformOutputVar
	err := json.Unmarshal([]byte(data), &outputVarsMap)
	if err != nil {
		return nil, err
	}
	// Convert map to slice ordered by var name.
	keys := make([]string, 0)
	for k := range outputVarsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	outputVars := make([]tfv1.TerraformOutputVar, 0)
	for _, k := range keys {
		v := outputVarsMap[k]
		v.Name = k
		outputVars = append(outputVars, v)
	}
	return &outputVars, err
}
