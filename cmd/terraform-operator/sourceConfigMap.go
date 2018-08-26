package main

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getConfigMapSourceData(namespace string, name string) (map[string]string, error) {
	configMaps := config.clientset.CoreV1().ConfigMaps(namespace)
	configMap, err := configMaps.Get(name, metav1.GetOptions{})
	return configMap.Data, err
}

func setConfigMapHash(sourceData map[string]string, parent *Terraform, status *TerraformControllerStatus) error {
	shaSum, err := toSha1(sourceData)
	if err != nil {
		return err
	}
	status.ConfigMapHash = shaSum

	return nil
}

func validateConfigMapSource(sourceData map[string]string) error {
	if len(sourceData) == 0 {
		return fmt.Errorf("no data found in ConfigMap")
	}
	return nil
}
