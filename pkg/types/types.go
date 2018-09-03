package types

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TerraformOperatorState represents the string mapping of the possible controller states. See the const definition below for enumerated states.
type TerraformOperatorState string

// Terraform is the custom resource definition structure.
type Terraform struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TerraformSpec           `json:"spec,omitempty"`
	Status            TerraformOperatorStatus `json:"status"`
}

// TerraformSpec is the top level structure of the spec body
type TerraformSpec struct {
	Image           string                                 `json:"image",omitempty`
	ImagePullPolicy corev1.PullPolicy                      `json:"imagePullPolicy,omitempty"`
	BackendBucket   string                                 `json:"backendBucket,omitempty"`
	BackendPrefix   string                                 `json:"backendPrefix,omitempty"`
	ProviderConfig  map[string]TerraformSpecProviderConfig `json:"providerConfig,omitempty"`
	Sources         []TerraformConfigSource                `json:"sources,omitempty"`
	TFPlan          string                                 `json:"tfplan,omitempty"`
	TFInputs        []TerraformConfigInputs                `json:"tfinputs,omitempty"`
	TFVars          map[string]string                      `json:"tfvars,omitempty"`
	TFVarsFrom      []TerraformConfigVarsFrom              `json:"tfvarsFrom,omitempty"`
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
	GCS       string                   `json:"gcs,omitempty"`
	TFApply   string                   `json:"tfapply,omitempty"`
}

// TerraformSourceConfigMap is the spec defining a config map source for terraform config.
type TerraformSourceConfigMap struct {
	Name    string `json:"name,omitempty"`
	Trigger bool   `json:"trigger,omitempty"`
}

// TerraformConfigVarsFrom is the spec for referencing TFVars from another object.
type TerraformConfigVarsFrom struct {
	TFApply string `json:"tfapply,omitempty"`
}

// TerraformConfigInputs is the structure defining how to use output vars from other TerraformApply resources
type TerraformConfigInputs struct {
	Name   string            `json:"name,omitempty"`
	VarMap map[string]string `json:"varMap,omitempty"`
}

// TerraformOperatorStatus is the status structure for the custom resource
type TerraformOperatorStatus struct {
	LastAppliedSig string                         `json:"lastAppliedSig"`
	Sources        TerraformOperatorStatusSources `json:"sources"`
	StateCurrent   TerraformOperatorState         `json:"stateCurrent"`
	PodName        string                         `json:"podName"`
	PodStatus      string                         `json:"podStatus"`
	StartedAt      string                         `json:"startedAt"`
	FinishedAt     string                         `json:"finishedAt"`
	Duration       string                         `json:"duration"`
	TFPlan         string                         `json:"planFile"`
	TFOutput       map[string]TerraformOutputVar  `json:"outputs"`
	RetryCount     int                            `json:"retryCount"`
	Workspace      string                         `json:"workspace"`
	StateFile      string                         `json:"stateFile"`
}

// TerraformOperatorStatusSources describes the status.sources structure.
type TerraformOperatorStatusSources struct {
	ConfigMapHashes    ConfigMapHashes    `json:"configMapHashes"`
	EmbeddedConfigMaps EmbeddedConfigMaps `json:"embeddedConfigMaps"`
}

// ConfigMapKeys is an ordered list of source keys as they appeard in the spec.
// List is a tuple containing the (configmap name , key name)
type ConfigMapKeys [][]string

// GCSObjects is a list of GCS URLs containing terraform source bundles.
type GCSObjects []string

// ConfigMapHashes is a map of configmap names to a has of the data spec.
type ConfigMapHashes map[string]string

// EmbeddedConfigMaps is a list of ConfigMap names generated to hold the embedded source.
type EmbeddedConfigMaps []string

// TerraformOutputVar is the structure of a terraform output variable from `terraform output -json`
type TerraformOutputVar struct {
	Sensitive bool   `json:"sensitive,omitempty"`
	Type      string `json:"type,omitempty"`
	Value     string `json:"value,omitempty"`
}
