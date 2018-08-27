package main

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Default image used for terraform job, can be overridden using spec.Image and spec.ImagePullPolicy
const (
	DEFAULT_IMAGE             = "gcr.io/cloud-solutions-group/terraform-job:latest"
	DEFAULT_IMAGE_PULL_POLICY = corev1.PullIfNotPresent
)

// TerraformControllerState represents the string mapping of the possible controller states. See the const definition below for enumerated states.
type TerraformControllerState string

const (
	// StateIdle means there are no more changes pending
	StateIdle = "IDLE"
	// StateSourcePending means the controller is waiting for the source ConfigMap to become available.
	StateSourcePending = "SOURCE_PENDING"
	// StateProviderConfigPending means the controller is waiting for the credentials Secret to become available.
	StateProviderConfigPending = "PROVIDER_PENDING"
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
	Jobs       map[string]batchv1.Job      `json:"Job.batch/v1"`
	ConfigMaps map[string]corev1.ConfigMap `json:"ConfigMap.v1"`
}

// TerraformControllerStatus is the status structure for the custom resource
type TerraformControllerStatus struct {
	LastAppliedSig          string `json:"lastAppliedSig"`
	ConfigMapHash           string `json:"configMapHash"`
	StateCurrent            string `json:"stateCurrent"`
	KustomizeBuildConfigMap string `json:"kustomizeBuildConfigMap"`
	JobName                 string `json:"jobName"`
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
}

// TerraformSpecProviderConfig is the structure providing the provider credentials block.
type TerraformSpecProviderConfig struct {
	SecretName string `json:"secretName,omitempty"`
}

// TerraformConfigSource is the structure providing the source for terraform configs.
type TerraformConfigSource struct {
	ConfigMap TerraformSourceConfigMap `json:"configMap,omitempty"`
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
