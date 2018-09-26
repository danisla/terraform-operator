package types

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TFKind is an enum on the different kind of resources.
type TFKind string

const (
	TFKindPlan    TFKind = "TerraformPlan"
	TFKindApply   TFKind = "TerraformApply"
	TFKindDestroy TFKind = "TerraformDestroy"
)

// TFKindShort is the abbreviated name of the resource, defined in the CRD
type TFKindShort string

const (
	TFKindShortPlan    TFKindShort = "tfplan"
	TFKindShortApply   TFKindShort = "tfapply"
	TFKindShortDestroy TFKindShort = "tfdestroy"
)

// GetShort translates a long kind to a short kind.
func (k *TFKind) GetShort() TFKindShort {
	var s TFKindShort
	switch *k {
	case TFKindPlan:
		s = TFKindShortPlan
	case TFKindApply:
		s = TFKindShortApply
	case TFKindDestroy:
		s = TFKindShortDestroy
	}
	return s
}

// TerraformOperatorState represents the string mapping of the possible controller states. See the const definition below for enumerated states.
type TerraformOperatorState string

// Terraform is the custom resource definition structure.
type Terraform struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              *TerraformSpec          `json:"spec,omitempty"`
	SpecFrom          *TerraformSpecFrom      `json:"specFrom,omitempty"`
	Status            TerraformOperatorStatus `json:"status"`
}

// GetTFKind converts the object type to a TFKind
func (parent *Terraform) GetTFKind() TFKind {
	return TFKind(parent.Kind)
}

// GetTFKindShort converts the object type to a TFKindShort
func (parent *Terraform) GetTFKindShort() TFKindShort {
	k := TFKind(parent.Kind)
	return k.GetShort()
}

// Verify checks the top level required fields.
func (parent *Terraform) Verify() error {
	if parent.Spec == nil && parent.SpecFrom == nil {
		return fmt.Errorf("Missing spec or specFrom, either one must be provided")
	}

	if parent.Spec != nil {
		return parent.Spec.Verify()
	}

	// Verify no cycles in TF sources
	for _, s := range parent.Spec.Sources {
		if s.TFApply != "" {
			if s.TFApply == parent.GetName() && parent.GetTFKind() == TFKindApply {
				return fmt.Errorf("source.tfapply %s/%s: CYCLE", parent.GetTFKind(), s.TFApply)
			}
		}
		if s.TFPlan != "" {
			if s.TFPlan == parent.GetName() && parent.GetTFKind() == TFKindPlan {
				return fmt.Errorf("source.tfplan %s/%s: CYCLE", parent.GetTFKind(), s.TFPlan)
			}
		}
	}

	// Verify vars were given for TFInputs
	if parent.Spec.TFInputs != nil {
		for _, tfinput := range *parent.Spec.TFInputs {
			if len(tfinput.VarMap) == 0 {
				return fmt.Errorf("source.tfinputs.varMap is empty")
			}
		}
	}

	return nil
}

// Log is a conventional log method to print the parent name and kind before the log message.
func (parent *Terraform) Log(level, msgfmt string, fmtargs ...interface{}) {
	log.Printf("[%s][%s][%s] %s", level, parent.Kind, parent.Name, fmt.Sprintf(msgfmt, fmtargs...))
}

// GetSig returns a hash of the current parent spec.
func (parent *Terraform) GetSig() string {
	hasher := sha1.New()
	data, err := json.Marshal(&parent.Spec)
	if err != nil {
		parent.Log("ERROR", "Failed to convert parent spec to JSON, this is a bug.")
		return ""
	}
	hasher.Write([]byte(data))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// MakeConditions initializes a new AppDBConditions struct
func (parent *Terraform) MakeConditions(initTime metav1.Time) Conditions {
	conditions := make(map[ConditionType]*Condition, 0)

	// Extract existing conditions from status and copy to conditions map for easier lookup.
	for _, c := range parent.GetConditionOrder() {
		// Search for condition type in conditions.
		found := false
		for _, condition := range parent.Status.Conditions {
			if condition.Type == c {
				found = true
				condition.LastProbeTime = initTime
				condition.Reason = ""
				condition.Message = ""
				conditions[c] = &condition
				break
			}
		}
		if found == false {
			// Initialize condition with unknown state
			conditions[c] = &Condition{
				Type:               c,
				Status:             ConditionUnknown,
				LastProbeTime:      initTime,
				LastTransitionTime: initTime,
			}
		}
	}

	return conditions
}

// GetConditionOrder returns an ordered slice of the conditions as the should appear in the status.
// This is dependent on which fields are provided in the parent spec.
func (parent *Terraform) GetConditionOrder() []ConditionType {
	desiredOrder := []ConditionType{
		ConditionSpecFromReady,
		ConditionProviderConfigReady,
		ConditionConfigSourceReady,
		ConditionInputsReady,
		ConditionVarsFromReady,
		ConditionPlanReady,
		ConditionPodComplete,
		ConditionReady,
	}

	conditionOrder := make([]ConditionType, 0)
	for _, c := range desiredOrder {
		if c == ConditionSpecFromReady && parent.SpecFrom == nil {
			continue
		}

		if parent.Spec != nil {
			// Inputs conditional on spec for inputs.
			if c == ConditionInputsReady && (parent.Spec.TFInputs == nil || len(*parent.Spec.TFInputs) == 0) {
				continue
			}

			// VarsFrom conditional on spec for vars from.
			if c == ConditionVarsFromReady && (parent.Spec.TFVarsFrom == nil || len(*parent.Spec.TFVarsFrom) == 0) {
				continue
			}

			// TFPlan conditional on spec for tfplan.
			if c == ConditionPlanReady && parent.Spec.TFPlan == "" {
				continue
			}
		}

		conditionOrder = append(conditionOrder, c)
	}
	return conditionOrder
}

// TerraformOperatorStatus is the status structure for the custom resource
type TerraformOperatorStatus struct {
	Sources        TerraformOperatorStatusSources `json:"sources,omitempty"`
	PodName        string                         `json:"podName,omitempty"`
	PodStatus      PodStatus                      `json:"podStatus,omitempty"`
	StartedAt      string                         `json:"startedAt,omitempty"`
	FinishedAt     string                         `json:"finishedAt,omitempty"`
	Duration       string                         `json:"duration,omitempty"`
	TFPlan         string                         `json:"planFile,omitempty"`
	TFPlanDiff     *TerraformPlanFileSummary      `json:"planDiff,omitempty"`
	TFOutput       *[]TerraformOutputVar          `json:"outputs,omitempty"`
	TFOutputSecret string                         `json:"outputsSecret,omitempty"`
	RetryCount     int32                          `json:"retryCount,omitempty"`
	RetryNextAt    string                         `json:"retryNextAt,omitempty"`
	Workspace      string                         `json:"workspace,omitempty"`
	StateFile      string                         `json:"stateFile,omitempty"`
	Conditions     []Condition                    `json:"conditions,omitempty"`
}

// Condition defines the format for a status condition element.
type Condition struct {
	Type               ConditionType   `json:"type"`
	Status             ConditionStatus `json:"status"`
	LastProbeTime      metav1.Time     `json:"lastProbeTime,omitempty"`
	LastTransitionTime metav1.Time     `json:"lastTransitionTime,omitempty"`
	Reason             string          `json:"reason,omitempty"`
	Message            string          `json:"message,omitempty"`
}

type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

// Conditions is a map of the condition types to their condition.
type Conditions map[ConditionType]*Condition

// ConditionType is a valid value for TerraformCondition.Type
type ConditionType string

// The condition type constants listed below are in the order they should roughly happen and in the order they
// exist in the status.conditions list. This gives visibility to what the operator is doing.
// Some conditions can be satisfied in parallel with others.
const (
	// ConditionSpecFromReady is True when the given specFrom terraform resource is ready.
	ConditionSpecFromReady ConditionType = "SpecFromReady"
	// ConditionProviderConfigReady is True when the provider config is available and ready.
	ConditionProviderConfigReady ConditionType = "ProviderConfigReady"
	// ConditionConfigSourceReady is True when all config sources are ready.
	ConditionConfigSourceReady ConditionType = "ConfigSourceReady"
	// ConditionInputsReady is True when all var inputs are ready.
	ConditionInputsReady ConditionType = "TFInputsReady"
	// ConditionVarsFromReady is True when all vars from sources are ready.
	ConditionVarsFromReady ConditionType = "TFVarsFromReady"
	// ConditionPlanReady is True when a given tfplan source file path is ready.
	ConditionPlanReady ConditionType = "TFPlanReady"
	// ConditionPodComplete is True when the terraform pod has completed successfully.
	ConditionPodComplete ConditionType = "TFPodComplete"
	// ConditionReady is True when all prior conditions are ready.
	ConditionReady ConditionType = "Ready"
)

// GetDependencies returns a map of condition type names to an ordered slice of dependent condition types.
func (conditionType *ConditionType) GetDependencies() []ConditionType {
	switch *conditionType {
	case ConditionPodComplete:
		return []ConditionType{
			ConditionProviderConfigReady,
			ConditionConfigSourceReady,
			ConditionInputsReady,
			ConditionVarsFromReady,
			ConditionPlanReady,
		}
	}
	return []ConditionType{}
}

// CheckConditions verifies that all given conditions have been met for the given conditionType on the receiving conditions.
func (conditions Conditions) CheckConditions(conditionType ConditionType) error {
	waiting := []string{}
	for _, t := range conditionType.GetDependencies() {
		condition := conditions[t]
		if condition != nil && condition.Status != ConditionTrue {
			waiting = append(waiting, string(t))
		}
	}

	if len(waiting) == 0 {
		return nil
	}

	return fmt.Errorf("Waiting on conditions: %s", strings.Join(waiting, ","))
}

// TerraformSpec is the top level structure of the spec body
type TerraformSpec struct {
	Image           string                         `json:"image,omitempty"`
	ImagePullPolicy corev1.PullPolicy              `json:"imagePullPolicy,omitempty"`
	BackendBucket   string                         `json:"backendBucket,omitempty"`
	BackendPrefix   string                         `json:"backendPrefix,omitempty"`
	ProviderConfig  *[]TerraformSpecProviderConfig `json:"providerConfig,omitempty"`
	Sources         []TerraformConfigSource        `json:"sources,omitempty"`
	TFPlan          string                         `json:"tfplan,omitempty"`
	TFInputs        *[]TerraformConfigInputs       `json:"tfinputs,omitempty"`
	TFVars          *[]TFVar                       `json:"tfvars,omitempty"`
	TFVarsFrom      *[]TerraformConfigVarsFrom     `json:"tfvarsFrom,omitempty"`
	MaxAttempts     *int32                         `json:"maxAttempts,omitempty"`
}

// TerraformSpecFrom is the the top level structure of specifying spec from antoher Terraform resource
type TerraformSpecFrom struct {
	TFPlan    string `json:"tfplan,omitempty"`
	TFApply   string `json:"tfapply,omitempty"`
	TFDestroy string `json:"tfdestroy,omitempty"`
}

// Verify checks all required fields in the spec.
func (spec *TerraformSpec) Verify() error {
	if spec.ProviderConfig == nil {
		return fmt.Errorf("Missing 'spec.providerConfig'")
	}

	if len(spec.Sources) == 0 {
		return fmt.Errorf("Missing 'spec.sources'")
	}

	return nil
}

// TFVar is an element of the TFVars spec
type TFVar struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// TerraformSpecProviderConfig is the structure providing the provider credentials block.
type TerraformSpecProviderConfig struct {
	Name       string `json:"name,omitempty"`
	SecretName string `json:"secretName,omitempty"`
}

// TerraformConfigSource is the structure providing the source for terraform configs.
type TerraformConfigSource struct {
	ConfigMap *TerraformSourceConfigMap `json:"configMap,omitempty"`
	Embedded  string                    `json:"embedded,omitempty"`
	GCS       string                    `json:"gcs,omitempty"`
	TFPlan    string                    `json:"tfplan,omitempty"`
	TFApply   string                    `json:"tfapply,omitempty"`
}

// TerraformSourceConfigMap is the spec defining a config map source for terraform config.
type TerraformSourceConfigMap struct {
	Name    string `json:"name,omitempty"`
	Trigger bool   `json:"trigger,omitempty"`
}

// TerraformConfigVarsFrom is the spec for referencing TFVars from another object.
type TerraformConfigVarsFrom struct {
	TFApply string `json:"tfapply,omitempty"`
	TFPlan  string `json:"tfplan,omitempty"`
}

// TerraformConfigInputs is the structure defining how to use output vars from other TerraformApply resources
type TerraformConfigInputs struct {
	Name   string       `json:"name,omitempty"`
	VarMap []VarMapItem `json:"varMap,omitempty"`
}

// VarMapItem is a config input mapping element
type VarMapItem struct {
	Source string `json:"source,omitempty"`
	Dest   string `json:"dest,omitempty"`
}

// TerraformOperatorStatusSources describes the status.sources structure.
type TerraformOperatorStatusSources struct {
	ConfigMapHashes    []ConfigMapHash    `json:"configMapHashes,omitempty"`
	EmbeddedConfigMaps EmbeddedConfigMaps `json:"embeddedConfigMaps,omitempty"`
}

// TerraformPlanFileSummary summarizes the changes in a terraform plan
type TerraformPlanFileSummary struct {
	Added     int `json:"added"`
	Changed   int `json:"changed"`
	Destroyed int `json:"destroyed"`
}

// ConfigMapKeys is an ordered list of source keys as they appeard in the spec.
// List is a tuple containing the (configmap name , key name)
type ConfigMapKeys [][]string

// GCSObjects is a list of GCS URLs containing terraform source bundles.
type GCSObjects []string

// ConfigMapHash is an element holding the configmap source name and a hash of the data spec.
type ConfigMapHash struct {
	Name string `json:"name,omitempty"`
	Hash string `json:"hash,omitempty"`
}

// EmbeddedConfigMaps is a list of ConfigMap names generated to hold the embedded source.
type EmbeddedConfigMaps []string

// TerraformOutputVar is the structure of a terraform output variable from `terraform output -json`
type TerraformOutputVar struct {
	Name      string `json:"name,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
	Type      string `json:"type,omitempty"`
	Value     string `json:"value,omitempty"`
}

// PodStatus is a const enum
type PodStatus string

// Pod status for reporting pass/fail status of pod
const (
	// PodStatusFailed indicates that the max attempts for retry have failed.
	PodStatusFailed  PodStatus = "FAILED"
	PodStatusPassed  PodStatus = "COMPLETED"
	PodStatusRunning PodStatus = "RUNNING"
	PodStatusUnknown PodStatus = "UNKNOWN"
)
