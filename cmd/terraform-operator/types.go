package main

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// TerraformControllerState represents the string mapping of the possible controller states. See the const definition below for enumerated states.
type TerraformControllerState string

const (
	// StateNone is the inital state for a new spec.
	StateNone = "NONE"
	// StateIdle means there are no more changes pending
	StateIdle = "IDLE"
	// StateSourcePending means the controller is waiting for the source ConfigMap to become available.
	StateSourcePending = "SOURCE_PENDING"
	// StateProviderConfigPending means the controller is waiting for the credentials Secret to become available.
	StateProviderConfigPending = "PROVIDER_PENDING"
	// StatePodRunning means the controller is waiting for the terraform pod to complete.
	StatePodRunning = "POD_RUNNING"
	// StateRetry means a pod has failed and is being retried up to MaxAttempts times.
	StateRetry = "POD_RETRY"
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
	Parent   Terraform                          `json:"parent"`
	Children TerraformControllerRequestChildren `json:"children"`
}

// SyncResponse is the CompositeController response structure.
type SyncResponse struct {
	Status   TerraformControllerStatus `json:"status"`
	Children []interface{}             `json:"children"`
}

// TerraformControllerRequestChildren is the children definition passed by the CompositeController request for the Terraform controller.
type TerraformControllerRequestChildren struct {
	Pods       map[string]corev1.Pod       `json:"Pod.v1"`
	ConfigMaps map[string]corev1.ConfigMap `json:"ConfigMap.v1"`
}

// TerraformControllerStatus is the status structure for the custom resource
type TerraformControllerStatus struct {
	LastAppliedSig string                        `json:"lastAppliedSig"`
	ConfigMapHash  string                        `json:"configMapHash"`
	StateCurrent   string                        `json:"stateCurrent"`
	PodName        string                        `json:"podName"`
	PodStatus      string                        `json:"podStatus"`
	StartedAt      string                        `json:"startedAt"`
	FinishedAt     string                        `json:"finishedAt"`
	Duration       string                        `json:"duration"`
	TFPlan         string                        `json:"planFile"`
	TFOutput       map[string]TerraformOutputVar `json:"outputs"`
	RetryCount     int                           `json:"retryCount"`
	Workspace      string                        `json:"workspace"`
	StateFile      string                        `json:"stateFile"`
}

// Terraform is the custom resource definition structure.
type Terraform struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TerraformSpec             `json:"spec,omitempty"`
	Status            TerraformControllerStatus `json:"status"`
}

// TerraformSpec is the top level structure of the spec body
type TerraformSpec struct {
	Image           string                                 `json:"image",omitempty`
	ImagePullPolicy corev1.PullPolicy                      `json:"imagePullPolicy,omitempty"`
	BackendBucket   string                                 `json:"backendBucket,omitempty"`
	BackendPrefix   string                                 `json:"backendPrefix,omitempty"`
	ProviderConfig  map[string]TerraformSpecProviderConfig `json:"providerConfig,omitempty"`
	Source          TerraformConfigSource                  `json:"source,omitempty"`
	ConfigMapName   string                                 `json:"configMapName,omitempty"`
	TFVars          map[string]string                      `json:"tfvars,omitempty"`
	MaxAttempts     int                                    `json:"maxAttempts,omitempty"`
}

// TerraformSpecProviderConfig is the structure providing the provider credentials block.
type TerraformSpecProviderConfig struct {
	SecretName string `json:"secretName,omitempty"`
}

// TerraformConfigSource is the structure providing the source for terraform configs.
type TerraformConfigSource struct {
	ConfigMap TerraformSourceConfigMap `json:"configMap,omitempty"`
	Embedded  string                   `json:"embedded,omitempty"`
}

// TerraformSourceConfigMap is the spec defining a config map source for terraform config.
type TerraformSourceConfigMap struct {
	Name    string `json:"name,omitempty"`
	Trigger bool   `json:"trigger,omitempty"`
}

// TerraformSpecCredentials is the structure for providing the credentials
type TerraformSpecCredentials struct {
	Name string `json:"name,omitempty"`
	Key  string `json:"key,omitempty"`
}

// TerraformOutputVar is the structure of a terraform output variable from `terraform output -json`
type TerraformOutputVar struct {
	Sensitive bool   `json:"sensitive,omitempty"`
	Type      string `json:"type,omitempty"`
	Value     string `json:"value,omitempty"`
}
