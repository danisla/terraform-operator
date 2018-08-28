package main

import (
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
)

func statePodRunning(parentType ParentType, parent *Terraform, status *TerraformControllerStatus, children *TerraformControllerRequestChildren, desiredChildren *[]interface{}) (string, error) {

	if pod, ok := children.Pods[status.PodName]; ok == true {

		switch pod.Status.Phase {
		case corev1.PodSucceeded, corev1.PodFailed:
			cStatus := pod.Status.ContainerStatuses[0]

			status.FinishedAt = cStatus.State.Terminated.FinishedAt.Format(time.RFC3339)

			// Set Duration in seconds
			startTime, _ := time.Parse(time.RFC3339, status.StartedAt)
			duration := cStatus.State.Terminated.FinishedAt.Sub(startTime)
			status.Duration = fmt.Sprintf("%02.0fm%02.0fs", duration.Minutes(), duration.Seconds())

			// Set the parent signature
			// If parent changes from here on, we'll go back through the idle state, triggering new pods.
			status.LastAppliedSig = calcParentSig(parent, "")

		default:
			// Active
			for _, cStatus := range pod.Status.ContainerStatuses {
				if cStatus.Name == "terraform" {
					if cStatus.State.Running != nil {
						status.StartedAt = cStatus.State.Running.StartedAt.Format(time.RFC3339)
					}
				}
			}
		}

		switch pod.Status.Phase {
		case corev1.PodSucceeded:
			// Extract terraform plan path from annotation.
			switch parentType {
			case ParentPlan:
				// Populate status.TFPlan from completed pod annotation.
				if plan, ok := pod.Annotations["terraform-plan"]; ok == true {
					myLog(parent, "INFO", fmt.Sprintf("Terraform plan from %s: %s", pod.Name, plan))
					status.TFPlan = plan
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

			// Back to StateIdle
			return StateIdle, nil

		case corev1.PodFailed:
			// Non-zero exit code. Perform retry if attempts has not been exdeeded.
			exitCode := pod.Status.ContainerStatuses[0].State.Terminated.ExitCode
			myLog(parent, "INFO", fmt.Sprintf("%s container failed with exit code: %d", pod.Name, exitCode))

			// Attempt retry
			status.RetryCount++

			if status.RetryCount >= getPodMaxAttempts(parent) {
				myLog(parent, "WARN", "Max retry attempts exceeded")

				status.PodStatus = PodStatusFailed

				status.RetryCount = 0

				return StateIdle, nil
			}

			backoff := computeExponentialBackoff(status.RetryCount, DEFAULT_RETRY_BACKOFF_SCALE)
			myLog(parent, "WARN", fmt.Sprintf("Attempting retry %d with backoff of %0.2fs", status.RetryCount, backoff))

			// Transition to StateRetry
			return StateRetry, nil
		default:
			myLog(parent, "INFO", fmt.Sprintf("Waiting for %s to complete.", pod.Name))
		}
	} else {
		myLog(parent, "WARN", fmt.Sprintf("Pod not found in children while in state %s", status.StateCurrent))
	}

	return status.StateCurrent, nil
}

func getPodMaxAttempts(parent *Terraform) int {
	maxAttempts := DEFAULT_POD_MAX_ATTEMPTS
	if parent.Spec.MaxAttempts != 0 {
		maxAttempts = parent.Spec.MaxAttempts
	}
	return maxAttempts
}

func makeOutputVars(data string) (map[string]TerraformOutputVar, error) {
	var outputVars map[string]TerraformOutputVar
	err := json.Unmarshal([]byte(data), &outputVars)
	return outputVars, err
}
