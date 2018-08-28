package main

import "fmt"

func sync(parentType ParentType, parent *Terraform, children *TerraformControllerRequestChildren) (*TerraformControllerStatus, *[]interface{}, error) {
	status := makeStatus(parent, children)
	currState := status.StateCurrent
	desiredChildren := make([]interface{}, 0)
	nextState := currState[0:1] + currState[1:] // string copy of currState

	changed := changeDetected(parent, children, status)

	var err error
	switch currState {
	case StateNone:
		// Call StateIdle handler
		nextState, err = stateIdleHandler(parentType, parent, status, children, &desiredChildren)

	case StateIdle:
		if changed {
			// Call StateIdle handler
			nextState, err = stateIdleHandler(parentType, parent, status, children, &desiredChildren)
		}

	case StateProviderConfigPending:
		// Call StateProviderConfigPending handler
		nextState, err = stateProviderConfigPending(parentType, parent, status, &desiredChildren)

	case StateSourcePending:
		// Call StateSourcePending handler
		nextState, err = stateSourcePending(parentType, parent, status, &desiredChildren)

	case StatePodRunning:
		// Call StatePodRunning handler
		nextState, err = statePodRunning(parentType, parent, status, children, &desiredChildren)

	case StateRetry:
		// Call StateRetry handler
		nextState, err = stateRetryHandler(parentType, parent, status, children, &desiredChildren)

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

	// Advance the state
	if status.StateCurrent != nextState {
		myLog(parent, "INFO", fmt.Sprintf("State %s -> %s", status.StateCurrent, nextState))
	}
	status.StateCurrent = nextState

	return status, &desiredChildren, nil
}
