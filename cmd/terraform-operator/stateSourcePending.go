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

		if source.TFApply != "" {
			if source.TFApply == parent.Name && parent.Kind == "TerraformApply" {
				return sourceData, fmt.Errorf("Circular reference to TerraformApply source[%d]: %s", i, source.TFApply)
			}

			myLog(parent, "INFO", fmt.Sprintf("Including TerraformApply source[%d]: %s", i, source.TFApply))
			tfapply, err := getTerraformApply(parent.Namespace, source.TFApply)
			if err != nil {
				return sourceData, fmt.Errorf("Waiting for source TerraformApply: %s", source.TFApply)
			}

			// ConfigMaps generated from embedded source.
			for _, configMapName := range tfapply.Status.Sources.EmbeddedConfigMaps {
				configMapData, err := getConfigMapSourceData(tfapply.ObjectMeta.Namespace, configMapName)
				if err != nil {
					// Wait for configmap to become available.
					return sourceData, fmt.Errorf("Waiting for TerraformApply %s source embedded ConfigMap: %s", source.TFApply, configMapName)
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

				myLog(parent, "INFO", fmt.Sprintf("Including TerraformApply %s embedded ConfigMap source with %d keys: %s", source.TFApply, len(configMapData), configMapName))
			}

			for j, tfsource := range tfapply.Spec.Sources {

				// ConfigMap source
				if tfsource.ConfigMap.Name != "" {
					configMapName := tfsource.ConfigMap.Name

					configMapData, err := getConfigMapSourceData(tfapply.ObjectMeta.Namespace, configMapName)
					if err != nil {
						// Wait for configmap to become available.
						return sourceData, fmt.Errorf("Waiting for TerraformApply %s source ConfigMap: %s", source.TFApply, configMapName)
					}

					err = validateConfigMapSource(configMapData)
					if err != nil {
						return sourceData, fmt.Errorf("TerraformApply %s ConfigMap source %s data is invalid: %v", source.TFApply, configMapName, err)
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

					myLog(parent, "INFO", fmt.Sprintf("Including TerraformApply %s ConfigMap source[%d] with %d keys: %s", source.TFApply, j, len(configMapData), configMapName))
				}

				// GCS source
				if tfsource.GCS != "" {
					myLog(parent, "INFO", fmt.Sprintf("Including TerraformApply %s GCS source[%d]: %s", source.TFApply, j, tfsource.GCS))
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
