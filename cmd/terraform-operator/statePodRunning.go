package main

import (
	"fmt"
)

func statePodRunning(parentType ParentType, parent *Terraform, status *TerraformControllerStatus, children *TerraformControllerRequestChildren, desiredChildren *[]interface{}) (string, error) {

	if pod, ok := children.Pods[status.PodName]; ok == true {
		for _, cStatus := range pod.Status.ContainerStatuses {
			if cStatus.Name == "terraform" {
				if cStatus.State.Terminated != nil {
					// Container is no longer running. Check status.
					if cStatus.State.Terminated.ExitCode == 0 {
						// Success

						// Extract terraform plan path from annotation.
						if plan, ok := pod.Annotations["terraform-plan"]; ok == true {
							myLog(parent, "INFO", fmt.Sprintf("Terraform plan from %s: %s", pod.Name, plan))
							status.TFPlan = plan
						}

						// Back to StateIdle
						return StateIdle, nil

					} else {
						// Non-zero exit code. Perform retry if attempts has not been exdeeded.
						myLog(parent, "INFO", fmt.Sprintf("%s container failed with exit code: %d", pod.Name, cStatus.State.Terminated.ExitCode))

						return StateIdle, nil
					}
				}
			}
		}

		myLog(parent, "INFO", fmt.Sprintf("Waiting for %s to complete.", pod.Name))
	}

	return status.StateCurrent, nil
}
