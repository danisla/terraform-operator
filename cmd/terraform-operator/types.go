package main

import (
	tftype "github.com/danisla/terraform-operator/pkg/types"
	corev1 "k8s.io/api/core/v1"
)

// Default image used for terraform pod, can be overridden using spec.Image and spec.ImagePullPolicy
const (
	DEFAULT_IMAGE             = "gcr.io/cloud-solutions-group/terraform-pod:latest"
	DEFAULT_IMAGE_PULL_POLICY = corev1.PullIfNotPresent
)

// ServiceAccount installed with Controller deployment
const (
	DEFAULT_POD_SERVICE_ACCOUNT = "terraform"
)

// Default max retries for failed pods
const (
	DEFAULT_POD_MAX_ATTEMPTS = 4
)

// Pod status for reporting pass/fail status of pod
const (
	// PodStatusFailed indicates that the max attempts for retry have failed.
	PodStatusFailed = "FAILED"
	PodStatusPassed = "COMPLETED"
)

const (
	// StateNone is the inital state for a new spec.
	StateNone = tftype.TerraformOperatorState("NONE")
	// StateIdle means there are no more changes pending
	StateIdle = tftype.TerraformOperatorState("IDLE")
	// StateWaitComplete is used to indicate that a wait is complete and to transition back through the idle handler.
	StateWaitComplete = tftype.TerraformOperatorState("WAIT_COMPLETE")
	// StateSourcePending means the controller is waiting for the source ConfigMap to become available.
	StateSourcePending = tftype.TerraformOperatorState("SOURCE_PENDING")
	// StateProviderConfigPending means the controller is waiting for the credentials Secret to become available.
	StateProviderConfigPending = tftype.TerraformOperatorState("PROVIDER_PENDING")
	// StateTFPlanPending means the controller is waiting for tfplan object.
	StateTFPlanPending = tftype.TerraformOperatorState("TFPLAN_PENDING")
	// StateTFInputPending means the controller is waiting for one or more tfapply objects.
	StateTFInputPending = tftype.TerraformOperatorState("TFINPUT_PENDING")
	// StateTFVarsFromPending means the controller is waiting to read tfvars from another object.
	StateTFVarsFromPending = tftype.TerraformOperatorState("TFVARSFROM_PENDING")
	// StatePodRunning means the controller is waiting for the terraform pod to complete.
	StatePodRunning = tftype.TerraformOperatorState("POD_RUNNING")
	// StateRetry means a pod has failed and is being retried up to MaxAttempts times.
	StateRetry = tftype.TerraformOperatorState("POD_RETRY")
)

// ParentType represents the strign mapping to the possible parent types in the const below.
type ParentType string

const (
	ParentPlan    = "tfplan"
	ParentApply   = "tfapply"
	ParentDestroy = "tfdestroy"
)

// SyncRequest describes the payload from the CompositeController hook
type SyncRequest struct {
	Parent   tftype.Terraform                 `json:"parent"`
	Children TerraformOperatorRequestChildren `json:"children"`
}

// SyncResponse is the CompositeController response structure.
type SyncResponse struct {
	Status   tftype.TerraformOperatorStatus `json:"status"`
	Children []interface{}                  `json:"children"`
}

// TerraformOperatorRequestChildren is the children definition passed by the CompositeController request for the Terraform controller.
type TerraformOperatorRequestChildren struct {
	Pods       map[string]corev1.Pod       `json:"Pod.v1"`
	ConfigMaps map[string]corev1.ConfigMap `json:"ConfigMap.v1"`
}

// TerraformInputVars is a map of output var names from TerraformApply Objects.
type TerraformInputVars map[string]string

// TerraformSpecCredentials is the structure for providing the credentials
type TerraformSpecCredentials struct {
	Name string `json:"name,omitempty"`
	Key  string `json:"key,omitempty"`
}

// TerraformConfigSourceData is the structure of all of the extracted config sources used by the Terraform Pod.
type TerraformConfigSourceData struct {
	ConfigMapHashes    *tftype.ConfigMapHashes
	ConfigMapKeys      *tftype.ConfigMapKeys
	GCSObjects         *tftype.GCSObjects
	EmbeddedConfigMaps *tftype.EmbeddedConfigMaps
}
