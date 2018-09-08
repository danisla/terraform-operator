package main

import (
	"encoding/json"
	"fmt"
	"time"

	tftype "github.com/danisla/terraform-operator/pkg/types"
	corev1 "k8s.io/api/core/v1"
)

func statePodRunning(parentType ParentType, parent *tftype.Terraform, status *tftype.TerraformOperatorStatus, children *TerraformOperatorRequestChildren, desiredChildren *[]interface{}) (tftype.TerraformOperatorState, error) {
	pod, ok := children.Pods[status.PodName]
	if ok == false {
		myLog(parent, "WARN", fmt.Sprintf("Pod not found in children while in state %s", status.StateCurrent))
		return StatePodRunning, nil
	}

	// Check status of init containers
	for _, cStatus := range pod.Status.InitContainerStatuses {
		switch cStatus.Name {
		case GCS_TARBALL_CONTAINER_NAME:
			switch pod.Status.Phase {
			case corev1.PodFailed:
				setFinalPodStatus(parent, status, cStatus, pod)
				status.PodStatus = PodStatusFailed
				myLog(parent, "ERROR", fmt.Sprintf("%s init container failed: %s", cStatus.Name, cStatus.State.Terminated.Message))

				// Attempt retry
				status.RetryCount++

				if status.RetryCount >= getPodMaxAttempts(parent) {
					myLog(parent, "WARN", "Max retry attempts exceeded")

					status.PodStatus = PodStatusFailed

					status.RetryCount = 0
					status.RetryNextAt = ""

					return StateIdle, nil
				}

				backoff := computeExponentialBackoff(status.RetryCount, DEFAULT_RETRY_BACKOFF_SCALE)
				myLog(parent, "WARN", fmt.Sprintf("Attempting retry %d with backoff of %0.2fs", status.RetryCount, backoff))

				// Transition to StateRetry
				return StateRetry, nil
			}
		default:
			myLog(parent, "WARN", fmt.Sprintf("Found unexpected init container in terraform pod: %s", cStatus.Name))
		}
	}

	// Check pod containers
	for _, cStatus := range pod.Status.ContainerStatuses {
		switch cStatus.Name {
		case TERRAFORM_CONTAINER_NAME:

			switch pod.Status.Phase {
			case corev1.PodSucceeded:
				// Passed
				setFinalPodStatus(parent, status, cStatus, pod)

				// Extract terraform plan path from annotation.
				switch parentType {
				case ParentPlan:
					// Populate status.TFPlan from completed pod annotation.
					if plan, ok := pod.Annotations["terraform-plan"]; ok == true {
						status.TFPlan = plan

						// Parse the plan
						summary, err := parseTerraformPlan(plan)
						if err != nil {
							myLog(parent, "ERROR", fmt.Sprintf("Failed to parse terraform plan file: %s: %v", plan, err))
							return StateIdle, nil
						}

						status.TFPlanDiff = &summary

					} else {
						myLog(parent, "ERROR", fmt.Sprintf("terraform-plan annotation not found on successful pod completion: %s", pod.Name))
					}
				case ParentApply:
					// Populate status.TFOutput map from completed pod annotation.
					if output, ok := pod.Annotations["terraform-output"]; ok == true {
						outputVars, err := makeOutputVars(output)
						if err != nil {
							myLog(parent, "ERROR", fmt.Sprintf("Failed to parse output vars on pod: %s: %v", pod.Name, err))
							return StateIdle, nil
						}
						status.TFOutput = outputVars

						myLog(parent, "INFO", fmt.Sprintf("Extracted %d output variables.", len(outputVars)))

					} else {
						myLog(parent, "ERROR", fmt.Sprintf("terraform-plan annotation not found on successful pod completion: %s", pod.Name))
					}
				}

				status.PodStatus = PodStatusPassed
				status.RetryCount = 0
				status.RetryNextAt = ""

				// Back to StateIdle
				return StateIdle, nil

			case corev1.PodFailed:
				// Failed
				setFinalPodStatus(parent, status, cStatus, pod)

				// Non-zero exit code. Perform retry if attempts has not been exdeeded.
				myLog(parent, "INFO", fmt.Sprintf("%s container failed: %s", pod.Name, cStatus.State.Terminated.Message))

				// Attempt retry
				status.RetryCount++

				if status.RetryCount >= getPodMaxAttempts(parent) {
					myLog(parent, "WARN", "Max retry attempts exceeded")

					status.PodStatus = PodStatusFailed

					status.RetryCount = 0
					status.RetryNextAt = ""

					return StateIdle, nil
				}

				backoff := computeExponentialBackoff(status.RetryCount, DEFAULT_RETRY_BACKOFF_SCALE)
				myLog(parent, "WARN", fmt.Sprintf("Attempting retry %d with backoff of %0.2fs", status.RetryCount, backoff))

				finishedAt, err := time.Parse(time.RFC3339, status.FinishedAt)
				if err != nil {
					myLog(parent, "ERROR", fmt.Sprintf("Failed to parse status.FinishedAt: %v", err))
					return StateRetry, nil
				}

				nextAttemptTime := finishedAt.Add(time.Second * time.Duration(int64(backoff)))
				status.RetryNextAt = nextAttemptTime.Format(time.RFC3339)

				// Transition to StateRetry
				return StateRetry, nil

			default:
				// Active
				if cStatus.State.Running != nil {
					status.StartedAt = cStatus.State.Running.StartedAt.Format(time.RFC3339)
				}

				status.RetryNextAt = ""
				status.PodStatus = PodStatusRunning

				myLog(parent, "INFO", fmt.Sprintf("Waiting for %s to complete.", pod.Name))
			}

		default:
			myLog(parent, "WARN", fmt.Sprintf("Found unexpected container in terraform pod: %s", cStatus.Name))
		}
	}

	return StatePodRunning, nil
}

func getPodMaxAttempts(parent *tftype.Terraform) int {
	maxAttempts := DEFAULT_POD_MAX_ATTEMPTS
	if parent.Spec.MaxAttempts != 0 {
		maxAttempts = parent.Spec.MaxAttempts
	}
	return maxAttempts
}

func makeOutputVars(data string) (map[string]tftype.TerraformOutputVar, error) {
	var outputVars map[string]tftype.TerraformOutputVar
	err := json.Unmarshal([]byte(data), &outputVars)
	return outputVars, err
}

func setFinalPodStatus(parent *tftype.Terraform, status *tftype.TerraformOperatorStatus, cStatus corev1.ContainerStatus, pod corev1.Pod) {
	status.FinishedAt = cStatus.State.Terminated.FinishedAt.Format(time.RFC3339)

	if status.StartedAt == "" {
		status.StartedAt = pod.Status.StartTime.Format(time.RFC3339)
	}

	// Set Duration in seconds
	startTime, _ := time.Parse(time.RFC3339, status.StartedAt)
	duration := cStatus.State.Terminated.FinishedAt.Sub(startTime)
	status.Duration = fmt.Sprintf("%02.0fm%02.0fs", duration.Minutes(), duration.Seconds())

	// Set the parent signature
	// If parent changes from here on, we'll go back through the idle state, triggering new pods.
	status.LastAppliedSig = calcParentSig(parent, "")
}
