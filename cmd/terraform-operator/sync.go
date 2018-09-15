package main

import (
	"fmt"

	tftype "github.com/danisla/terraform-operator/pkg/types"
)

func sync(parentType ParentType, parent *tftype.Terraform, children *TerraformOperatorRequestChildren) (*tftype.TerraformOperatorStatus, *[]interface{}, error) {
	status := makeStatus(parent, children)
	currState := status.StateCurrent
	desiredChildren := make([]interface{}, 0)
	nextState := currState[0:1] + currState[1:] // string copy of currState

	var err error
	switch currState {
	case StateNone, StateIdle, StateWaitComplete, StateSpecFromPending, StateSourcePending, StateTFPlanPending, StateTFInputPending, StateTFVarsFromPending, StateProviderConfigPending:
		nextState, err = stateIdle(parentType, parent, status, children, &desiredChildren)

	case StatePodRunning:
		nextState, err = statePodRunning(parentType, parent, status, children, &desiredChildren)

	case StateRetry:
		nextState, err = stateRetry(parentType, parent, status, children, &desiredChildren)

	}

	if err != nil {
		return status, &desiredChildren, err
	}

	// Claim the configmaps.
	for _, o := range children.ConfigMaps {
		desiredChildren = append(desiredChildren, o)
	}

	// Claim the Pods.
	for _, o := range children.Pods {
		desiredChildren = append(desiredChildren, o)
	}

	// Claim the Secrets.
	for _, o := range children.Secrets {
		desiredChildren = append(desiredChildren, o)
	}

	// Advance the state
	if status.StateCurrent != nextState {
		myLog(parent, "INFO", fmt.Sprintf("State %s -> %s", status.StateCurrent, nextState))
	}
	status.StateCurrent = nextState

	return status, &desiredChildren, nil
}
