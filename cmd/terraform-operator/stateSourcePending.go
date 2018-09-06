package main

import (
	"fmt"

	tftype "github.com/danisla/terraform-operator/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getSourceData(parent *tftype.Terraform, desiredChildren *[]interface{}, podName string) (TerraformConfigSourceData, error) {

	// Map of ConfigMap source names to content hashes.
	configMapHashes := make(tftype.ConfigMapHashes, 0)

	// Map of ConfigMap source names to list of keys.
	// Keys are in the order the source appears in the spec.
	// List is a tuple containing the (configmap name , key name)
	configMapKeys := make(tftype.ConfigMapKeys, 0)

	gcsObjects := make(tftype.GCSObjects, 0)

	embeddedConfigMaps := make(tftype.EmbeddedConfigMaps, 0)

	sourceData := TerraformConfigSourceData{
		ConfigMapHashes:    &configMapHashes,
		ConfigMapKeys:      &configMapKeys,
		GCSObjects:         &gcsObjects,
		EmbeddedConfigMaps: &embeddedConfigMaps,
	}

	// At least 1 source is required.
	if len(parent.Spec.Sources) == 0 {
		return sourceData, fmt.Errorf("No terraform source defined. Deadend")
	}

	// Wait for all sources to become available.
	for i, source := range parent.Spec.Sources {
		if source.ConfigMap.Name != "" {
			configMapName := source.ConfigMap.Name

			configMapData, err := getConfigMapSourceData(parent.ObjectMeta.Namespace, configMapName)
			if err != nil {
				// Wait for configmap to become available.
				return sourceData, fmt.Errorf("Waiting for source ConfigMap: %s", configMapName)
			}

			err = validateConfigMapSource(configMapData)
			if err != nil {
				return sourceData, fmt.Errorf("ConfigMap source %s data is invalid: %v", configMapName, err)
			}

			configMapHash, err := toSha1(configMapData)
			if err != nil {
				return sourceData, err
			}

			configMapHashes[configMapName] = configMapHash

			for k := range configMapData {
				tuple := []string{configMapName, k}
				configMapKeys = append(configMapKeys, tuple)
			}

			myLog(parent, "INFO", fmt.Sprintf("Including ConfigMap source[%d] with %d keys: %s", i, len(configMapData), configMapName))
		}

		if source.Embedded != "" {

			configMapHash, err := toSha1(source.Embedded)
			if err != nil {
				return sourceData, err
			}

			configMapName := fmt.Sprintf("%s-%s", podName, configMapHash[0:4])

			configMapHashes[configMapName] = configMapHash

			configMap := makeTerraformSourceConfigMap(configMapName, source.Embedded)

			*desiredChildren = append(*desiredChildren, configMap)

			embeddedConfigMaps = append(embeddedConfigMaps, configMapName)

			for k := range configMap.Data {
				tuple := []string{configMapName, k}
				configMapKeys = append(configMapKeys, tuple)
			}

			myLog(parent, "INFO", fmt.Sprintf("Including embedded source[%d] from spec", i))

		}

		if source.GCS != "" {
			myLog(parent, "INFO", fmt.Sprintf("Including GCS source[%d]: %s", i, source.GCS))
			gcsObjects = append(gcsObjects, source.GCS)
		}

		if source.TFApply != "" || source.TFPlan != "" {

			if source.TFApply == parent.Name && parent.Kind == "TerraformApply" {
				return sourceData, fmt.Errorf("Circular reference to TerraformApply source[%d]: %s", i, source.TFApply)
			}

			if source.TFPlan == parent.Name && parent.Kind == "TerraformPlan" {
				return sourceData, fmt.Errorf("Circular reference to TerraformPlan source[%d]: %s", i, source.TFPlan)
			}

			sourceKind := "TerraformApply"
			sourceName := source.TFApply

			var tf tftype.Terraform

			tfapply, tfapplyErr := getTerraform("tfapply", parent.Namespace, source.TFApply)
			tfplan, tfplanErr := getTerraform("tfplan", parent.Namespace, source.TFPlan)

			if source.TFApply != "" && source.TFPlan != "" && tfapplyErr != nil && tfplanErr != nil {
				// no source available yet.
				return sourceData, fmt.Errorf("Waiting for either source TerraformPlan: '%s', or source TerraformApply: '%s'", source.TFPlan, source.TFApply)
			}

			if tfapplyErr == nil {
				// Prefer tfapply if both were specified.
				tf = tfapply
				myLog(parent, "INFO", fmt.Sprintf("Including %s source[%d]: %s", sourceKind, i, source.TFApply))
			} else if tfplanErr == nil {
				tf = tfplan
				sourceKind = "TerraformPlan"
				sourceName = source.TFPlan
				myLog(parent, "INFO", fmt.Sprintf("Including %s source[%d]: %s", sourceKind, i, source.TFPlan))
			} else {
				if source.TFPlan != "" {
					return sourceData, fmt.Errorf("Waiting for source TerraformPlan: %s", source.TFPlan)
				} else {
					return sourceData, fmt.Errorf("Waiting for source TerraformApply: %s", source.TFApply)
				}
			}

			// ConfigMaps generated from embedded source.
			for _, configMapName := range tf.Status.Sources.EmbeddedConfigMaps {
				configMapData, err := getConfigMapSourceData(tf.ObjectMeta.Namespace, configMapName)
				if err != nil {
					// Wait for configmap to become available.
					return sourceData, fmt.Errorf("Waiting for %s %s source embedded ConfigMap: %s", sourceKind, sourceName, configMapName)
				}

				configMapHash, err := toSha1(configMapData)
				if err != nil {
					return sourceData, err
				}

				configMapHashes[configMapName] = configMapHash

				for k := range configMapData {
					tuple := []string{configMapName, k}
					configMapKeys = append(configMapKeys, tuple)
				}

				myLog(parent, "INFO", fmt.Sprintf("Including %s %s embedded ConfigMap source with %d keys: %s", sourceKind, sourceName, len(configMapData), configMapName))
			}

			for j, tfsource := range tf.Spec.Sources {

				// ConfigMap source
				if tfsource.ConfigMap.Name != "" {
					configMapName := tfsource.ConfigMap.Name

					configMapData, err := getConfigMapSourceData(tf.ObjectMeta.Namespace, configMapName)
					if err != nil {
						// Wait for configmap to become available.
						return sourceData, fmt.Errorf("Waiting for %s %s source ConfigMap: %s", sourceKind, sourceName, configMapName)
					}

					err = validateConfigMapSource(configMapData)
					if err != nil {
						return sourceData, fmt.Errorf("%s %s ConfigMap source %s data is invalid: %v", sourceKind, sourceName, configMapName, err)
					}

					configMapHash, err := toSha1(configMapData)
					if err != nil {
						return sourceData, err
					}

					configMapHashes[configMapName] = configMapHash

					for k := range configMapData {
						tuple := []string{configMapName, k}
						configMapKeys = append(configMapKeys, tuple)
					}

					myLog(parent, "INFO", fmt.Sprintf("Including %s %s ConfigMap source[%d] with %d keys: %s", sourceKind, sourceName, j, len(configMapData), configMapName))
				}

				// GCS source
				if tfsource.GCS != "" {
					myLog(parent, "INFO", fmt.Sprintf("Including %s %s GCS source[%d]: %s", sourceKind, sourceName, j, tfsource.GCS))
					gcsObjects = append(gcsObjects, tfsource.GCS)
				}
			}
		}
	}

	return sourceData, nil
}

func getConfigMapSourceData(namespace string, name string) (map[string]string, error) {
	configMaps := config.clientset.CoreV1().ConfigMaps(namespace)
	configMap, err := configMaps.Get(name, metav1.GetOptions{})
	return configMap.Data, err
}

func validateConfigMapSource(sourceData map[string]string) error {
	if len(sourceData) == 0 {
		return fmt.Errorf("no data found in ConfigMap")
	}
	return nil
}
