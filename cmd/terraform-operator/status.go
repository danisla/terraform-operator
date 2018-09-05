package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"

	tftype "github.com/danisla/terraform-operator/pkg/types"
)

func makeStatus(parent *tftype.Terraform, children *TerraformOperatorRequestChildren) *tftype.TerraformOperatorStatus {
	status := tftype.TerraformOperatorStatus{
		StateCurrent: StateNone,
	}

	changed := false
	sig := calcParentSig(parent, "")

	if parent.Status.LastAppliedSig != "" {
		if parent.Status.StateCurrent == StateIdle && sig != parent.Status.LastAppliedSig {
			changed = true
			status.LastAppliedSig = ""
		} else {
			status.LastAppliedSig = parent.Status.LastAppliedSig
		}
	}

	if parent.Status.StateCurrent != "" && changed == false {
		status.StateCurrent = parent.Status.StateCurrent
	}

	if parent.Status.Sources.ConfigMapHashes != nil {
		status.Sources.ConfigMapHashes = parent.Status.Sources.ConfigMapHashes
	}

	if parent.Status.Sources.EmbeddedConfigMaps != nil {
		status.Sources.EmbeddedConfigMaps = parent.Status.Sources.EmbeddedConfigMaps
	}

	if parent.Status.PodName != "" && changed == false {
		status.PodName = parent.Status.PodName
	}

	if parent.Status.TFPlan != "" && changed == false {
		status.TFPlan = parent.Status.TFPlan
	}

	if parent.Status.RetryCount != 0 && changed == false {
		status.RetryCount = parent.Status.RetryCount
	}

	if parent.Status.PodStatus != "" && changed == false {
		status.PodStatus = parent.Status.PodStatus
	}

	if parent.Status.StartedAt != "" && changed == false {
		status.StartedAt = parent.Status.StartedAt
	}

	if parent.Status.FinishedAt != "" && changed == false {
		status.FinishedAt = parent.Status.FinishedAt
	}

	if parent.Status.Duration != "" && changed == false {
		status.Duration = parent.Status.Duration
	}

	if parent.Status.Workspace != "" && changed == false {
		status.Workspace = parent.Status.Workspace
	}

	if parent.Status.StateFile != "" && changed == false {
		status.StateFile = parent.Status.StateFile
	}

	if parent.Status.TFOutput != nil && changed == false {
		status.TFOutput = parent.Status.TFOutput
	}

	if parent.Status.TFPlanDiff != nil && changed == false {
		status.TFPlanDiff = parent.Status.TFPlanDiff
	}

	return &status
}

func calcParentSig(parent *tftype.Terraform, addStr string) string {
	hasher := sha1.New()
	data, err := json.Marshal(&parent.Spec)
	if err != nil {
		myLog(parent, "ERROR", "Failed to convert parent spec to JSON, this is a bug.")
		return ""
	}
	hasher.Write([]byte(data))
	hasher.Write([]byte(addStr))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func changeDetected(parent *tftype.Terraform, children *TerraformOperatorRequestChildren, status *tftype.TerraformOperatorStatus) bool {
	changed := false

	if status.StateCurrent == StateIdle {

		// Changed if parent spec changes
		if status.LastAppliedSig != "" && status.LastAppliedSig != calcParentSig(parent, "") {
			myLog(parent, "DEBUG", "Changed because parent sig different")
			changed = true
		}

		// Changed if any config map sources change.
		for _, source := range parent.Spec.Sources {
			if source.ConfigMap.Name != "" && source.ConfigMap.Trigger && status.Sources.ConfigMapHashes[source.ConfigMap.Name] != "" {
				specData, err := getConfigMapSourceData(parent.ObjectMeta.Namespace, source.ConfigMap.Name)
				if err != nil {
					changed = true
				}
				shaSum, err := toSha1(specData)
				if err != nil {
					myLog(parent, "ERROR", fmt.Sprintf("Failed to compute shasum of ConfigMap: %v", err))
					changed = true
				}
				if shaSum != status.Sources.ConfigMapHashes[source.ConfigMap.Name] {
					myLog(parent, "DEBUG", "Changed because configmap spec changed")
					changed = true
				}
			}
		}
	}

	return changed
}

func toSha1(data interface{}) (string, error) {
	h := sha1.New()
	var b []byte
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil)), nil
}
