package main

import (
	"fmt"
	"time"
)

func statePodRunning(parentType ParentType, parent *Terraform, status *TerraformControllerStatus, children *TerraformControllerRequestChildren, desiredChildren *[]interface{}) (string, error) {

	if pod, ok := children.Pods[status.PodName]; ok == true {
		for _, cStatus := range pod.Status.ContainerStatuses {
			if cStatus.Name == "terraform" {
				if cStatus.State.Running != nil && status.StartedAt == "" {
					// Set StartedAt time
					status.StartedAt = cStatus.State.Running.StartedAt.Format(time.RFC3339)
				}
				if cStatus.State.Terminated != nil {
					// Set EndedAt time
					status.FinishedAt = cStatus.State.Terminated.FinishedAt.Format(time.RFC3339)

					// Container is no longer running. Check status.
					if cStatus.State.Terminated.ExitCode == 0 {
						// Success

						// Extract terraform plan path from annotation.
						if plan, ok := pod.Annotations["terraform-plan"]; ok == true {
							myLog(parent, "INFO", fmt.Sprintf("Terraform plan from %s: %s", pod.Name, plan))
							status.TFPlan = plan
						}

						status.PodStatus = PodStatusPassed

						status.RetryCount = 0

						// Back to StateIdle
						return StateIdle, nil

					} else {
						// Non-zero exit code. Perform retry if attempts has not been exdeeded.
						myLog(parent, "INFO", fmt.Sprintf("%s container failed with exit code: %d", pod.Name, cStatus.State.Terminated.ExitCode))

						status.RetryCount++

						if status.RetryCount >= getPodMaxAttempts(parent) {
							myLog(parent, "WARN", "Max retry attempts exceeded")
							status.PodStatus = PodStatusFailed
							return StateIdle, nil
						}

						backoff := computeExponentialBackoff(status.RetryCount, DEFAULT_RETRY_BACKOFF_SCALE)
						myLog(parent, "WARN", fmt.Sprintf("Attempting retry %d with backoff of %0.2fs", status.RetryCount, backoff))

						// Transition to StateRetry
						return StateRetry, nil
					}
				}
			}
		}

		myLog(parent, "INFO", fmt.Sprintf("Waiting for %s to complete.", pod.Name))
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
