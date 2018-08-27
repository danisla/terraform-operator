package main

import "fmt"

func sync(parentType ParentType, parent *Terraform, children *TerraformControllerRequestChildren) (*TerraformControllerStatus, *[]interface{}, error) {
	status := makeStatus(parent, children)
	currState := status.StateCurrent
	if currState == "" {
		currState = StateIdle
	}
	desiredChildren := make([]interface{}, 0)
	nextState := currState[0:1] + currState[1:] // string copy of currState

	changed := changeDetected(parent, children, status)

	var err error
	switch currState {
	case StateIdle:
		if changed {
			// Call StateIdle handler
			nextState, err = stateIdleHandler(parentType, parent, status, &desiredChildren)
		}

	case StateProviderConfigPending:
		// Call StateProviderConfigPending handler
		nextState, err = stateProviderConfigPending(parentType, parent, status, &desiredChildren)

	case StateSourcePending:
		// Call StateSourcePending handler
		nextState, err = stateSourcePending(parentType, parent, status, &desiredChildren)

	}

	if err != nil {
		return status, &desiredChildren, err
	}

	if !changed {
		// Claim the ConfigMaps.
		for _, o := range children.ConfigMaps {
			desiredChildren = append(desiredChildren, o)
		}

		// Claim the Jobs.
		for _, o := range children.Jobs {
			desiredChildren = append(desiredChildren, o)
		}
	}

	// Advance the state
	if status.StateCurrent != nextState {
		myLog(parent, "INFO", fmt.Sprintf("Current state: %s", nextState))
	}
	status.StateCurrent = nextState

	return status, &desiredChildren, nil
}
