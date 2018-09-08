package main

import (
	"fmt"
	"math"
	"time"

	tftype "github.com/danisla/terraform-operator/pkg/types"
)

const DEFAULT_RETRY_BACKOFF_SCALE = 5.0

func stateRetry(parentType ParentType, parent *tftype.Terraform, status *tftype.TerraformOperatorStatus, children *TerraformOperatorRequestChildren, desiredChildren *[]interface{}) (tftype.TerraformOperatorState, error) {

	finishedAt, err := time.Parse(time.RFC3339, status.FinishedAt)
	if err != nil {
		myLog(parent, "ERROR", fmt.Sprintf("Failed to parse status.FinishedAt: %v", err))
		return StateRetry, nil
	}

	backoff := computeExponentialBackoff(status.RetryCount, DEFAULT_RETRY_BACKOFF_SCALE)

	timeSinceFinished := time.Since(finishedAt)
	if timeSinceFinished.Seconds() >= backoff {
		// Done waiting for backoff.
		return StateWaitComplete, nil
	}

	nextAttemptTime := finishedAt.Add(time.Second * time.Duration(int64(backoff)))
	status.RetryNextAt = nextAttemptTime.Format(time.RFC3339)

	return StateRetry, nil
}

func computeExponentialBackoff(retryCount int, scaleFactor float64) float64 {
	return ((math.Pow(2, float64(retryCount+1)) - 1) / 2.0) * scaleFactor
}
