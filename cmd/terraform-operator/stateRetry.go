package main

import (
	"fmt"
	"math"
	"time"
)

const DEFAULT_RETRY_BACKOFF_SCALE = 5.0

func stateRetryHandler(parentType ParentType, parent *Terraform, status *TerraformControllerStatus, children *TerraformControllerRequestChildren, desiredChildren *[]interface{}) (string, error) {

	finishedAt, err := time.Parse(time.RFC3339, status.FinishedAt)
	if err != nil {
		myLog(parent, "ERROR", fmt.Sprintf("Failed to parse status.FinishedAt: %v", err))
		return status.StateCurrent, nil
	}

	backoff := computeExponentialBackoff(status.RetryCount, DEFAULT_RETRY_BACKOFF_SCALE)

	timeSinceFinished := time.Since(finishedAt)
	if timeSinceFinished.Seconds() >= backoff {
		// Done waiting for backoff.
		// Call StateIdle handler to trigger recreation of new pod
		return stateIdleHandler(parentType, parent, status, children, desiredChildren)
	}

	return status.StateCurrent, nil
}

func computeExponentialBackoff(retryCount int, scaleFactor float64) float64 {
	return ((math.Pow(2, float64(retryCount+1)) - 1) / 2.0) * scaleFactor
}
