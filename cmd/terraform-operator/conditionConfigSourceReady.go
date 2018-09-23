package main

import (
	"fmt"
	"path/filepath"
	"strings"

	tfv1 "github.com/danisla/terraform-operator/pkg/types"
)

func reconcileConfigSourceReady(condition *tfv1.TerraformCondition, parent *tfv1.Terraform, status *tfv1.TerraformOperatorStatus, children *TerraformChildren, desiredChildren *[]interface{}) (tfv1.ConditionStatus, TerraformConfigSourceData) {
	newStatus := tfv1.ConditionFalse
	allFound := true
	reasons := make([]string, 0)

	podName := makeOrdinalPodName(parent, children)

	// Map of ConfigMap source names to content hashes.
	configMapHashes := make(map[string]tfv1.ConfigMapHash, 0)

	// Map of ConfigMap source names to list of keys.
	// Keys are in the order the source appears in the spec.
	// List is a tuple containing the (configmap name , key name)
	configMapKeys := make(tfv1.ConfigMapKeys, 0)

	gcsObjects := make(tfv1.GCSObjects, 0)

	embeddedConfigMaps := make(tfv1.EmbeddedConfigMaps, 0)

	sourceData := TerraformConfigSourceData{
		ConfigMapHashes:    &configMapHashes,
		ConfigMapKeys:      &configMapKeys,
		GCSObjects:         &gcsObjects,
		EmbeddedConfigMaps: &embeddedConfigMaps,
	}

	// Wait for all sources to become available.
	for _, source := range *parent.Spec.Sources {
		if source.ConfigMap.Name != "" {
			configMapName := source.ConfigMap.Name

			configMapData, err := getConfigMapSourceData(parent.GetNamespace(), configMapName)
			if err != nil {
				allFound = false
				reasons = append(reasons, fmt.Sprintf("ConfigMap/%s: WAITING", configMapName))
			} else {
				if err := configMapData.Validate(); err != nil {
					allFound = false
					reasons = append(reasons, fmt.Sprintf("ConfigMap/%s: INVALID: %v", configMapName, err))
				} else {
					configMapHashes[configMapName] = tfv1.ConfigMapHash{
						Name: configMapName,
						Hash: configMapData.GetHash(),
					}
					for k := range configMapData {
						tuple := []string{configMapName, k}
						configMapKeys = append(configMapKeys, tuple)
					}
					reasons = append(reasons, fmt.Sprintf("ConfigMap/%s: READY", configMapName))
				}
			}
		}

		if source.Embedded != "" {
			configMapHash := toSha1(source.Embedded)
			configMapName := fmt.Sprintf("%s-%s", podName, configMapHash[0:4])
			configMapHashes[configMapName] = tfv1.ConfigMapHash{
				Name: configMapName,
				Hash: configMapHash,
			}
			configMap := makeTerraformSourceConfigMap(configMapName, source.Embedded)
			children.claimChildAndGetCurrent(configMap, desiredChildren)
			embeddedConfigMaps = append(embeddedConfigMaps, configMapName)
			for k := range configMap.Data {
				tuple := []string{configMapName, k}
				configMapKeys = append(configMapKeys, tuple)
			}
			reasons = append(reasons, fmt.Sprintf("Embedded ConfigMap/%s: CREATED", configMapName))
		}

		if source.GCS != "" {
			gcsObjects = append(gcsObjects, source.GCS)
			reasons = append(reasons, fmt.Sprintf("GCS/%s", filepath.Base(source.GCS)))
		}

		if source.TFApply != "" || source.TFPlan != "" {
			var tf *tfv1.Terraform

			tfapply, tfapplyErr := getTerraform("tfapply", parent.GetNamespace(), source.TFApply)
			tfplan, tfplanErr := getTerraform("tfplan", parent.GetNamespace(), source.TFPlan)

			if source.TFApply != "" && source.TFPlan != "" && tfapplyErr != nil && tfplanErr != nil {
				// no source available yet.
				allFound = false
				reasons = append(reasons, fmt.Sprintf("%s/%s || %s/%s: WAITING", tfv1.TFKindApply, source.TFApply, tfv1.TFKindPlan, source.TFPlan))
			} else {
				sourceKind := tfv1.TFKindApply
				sourceName := source.TFApply
				if tfapplyErr == nil {
					// Prefer tfapply if both were specified.
					tf = &tfapply
				} else if tfplanErr == nil {
					tf = &tfplan
					sourceKind = tfv1.TFKindPlan
					sourceName = source.TFPlan
				} else {
					if source.TFPlan != "" {
						allFound = false
						reasons = append(reasons, fmt.Sprintf("%s/%s: WAITING", tfv1.TFKindPlan, source.TFPlan))
					} else if source.TFApply != "" {
						allFound = false
						reasons = append(reasons, fmt.Sprintf("%s/%s: WAITING", tfv1.TFKindApply, source.TFApply))
					}
				}

				if tf != nil {
					// Copy ConfigMaps generated from embedded source.
					for _, configMapName := range tf.Status.Sources.EmbeddedConfigMaps {
						configMapData, err := getConfigMapSourceData(tf.GetNamespace(), configMapName)
						if err != nil {
							// Wait for configmap to become available.
							allFound = false
							reasons = append(reasons, fmt.Sprintf("ConfigMap/%s: WAITING", configMapName))
						} else {
							configMapHashes[configMapName] = tfv1.ConfigMapHash{
								Name: configMapName,
								Hash: configMapData.GetHash(),
							}
							for k := range configMapData {
								tuple := []string{configMapName, k}
								configMapKeys = append(configMapKeys, tuple)
							}
							reasons = append(reasons, fmt.Sprintf("ConfigMap/%s: from %s/%s", configMapName, sourceKind, sourceName))
						}
					}

					for _, tfsource := range *tf.Spec.Sources {

						// ConfigMap source
						if tfsource.ConfigMap.Name != "" {
							configMapName := tfsource.ConfigMap.Name
							configMapData, err := getConfigMapSourceData(parent.GetNamespace(), configMapName)
							if err != nil {
								allFound = false
								reasons = append(reasons, fmt.Sprintf("ConfigMap/%s: WAITING", configMapName))
							} else {
								if err := configMapData.Validate(); err != nil {
									allFound = false
									reasons = append(reasons, fmt.Sprintf("ConfigMap/%s: INVALID: %v", configMapName, err))
								} else {
									configMapHashes[configMapName] = tfv1.ConfigMapHash{
										Name: configMapName,
										Hash: configMapData.GetHash(),
									}
									for k := range configMapData {
										tuple := []string{configMapName, k}
										configMapKeys = append(configMapKeys, tuple)
									}
									reasons = append(reasons, fmt.Sprintf("ConfigMap/%s: READY", configMapName))
								}
							}
							reasons = append(reasons, fmt.Sprintf("ConfigMap/%s: from %s/%s", configMapName, sourceKind, sourceName))
						}

						// GCS source
						if tfsource.GCS != "" {
							gcsObjects = append(gcsObjects, tfsource.GCS)
							reasons = append(reasons, fmt.Sprintf("GCS/%s", filepath.Base(tfsource.GCS)))
						}
					}
				}
			}
		}
	}

	if allFound {
		newStatus = tfv1.ConditionTrue
	}

	condition.Reason = strings.Join(reasons, ",")

	return newStatus, sourceData
}
