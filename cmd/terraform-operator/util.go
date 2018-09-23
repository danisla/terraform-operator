package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"

	"github.com/buger/jsonparser"
	tfv1 "github.com/danisla/terraform-operator/pkg/types"
	"github.com/ghodss/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func parseTerraformPlan(planfile string) (tfv1.TerraformPlanFileSummary, error) {
	summary := tfv1.TerraformPlanFileSummary{}

	tfplanjson, err := runTFJson(planfile)
	if err != nil {
		return summary, err
	}
	err = summarizeTFPlan([]byte(tfplanjson), &summary)
	return summary, err
}

func summarizeTFPlan(data []byte, summary *tfv1.TerraformPlanFileSummary) error {
	modData, vt, _, err := jsonparser.Get(data, []string{}...)
	if err != nil {
		return err
	}
	if vt != jsonparser.Object {
		return fmt.Errorf("Module data was not an object, found type: %v", vt)
	}

	return jsonparser.ObjectEach(modData, func(key []byte, value []byte, vt jsonparser.ValueType, offset int) error {
		toks := strings.Split(string(key), ".")

		if vt == jsonparser.Object {
			if len(toks) == 2 {
				// Resource
				id, err := jsonparser.GetString(value, "id")
				if err != nil {
					return fmt.Errorf("Failed to extract 'id' key as string: %v", err)
				}

				d, err := jsonparser.GetBoolean(value, "destroy")
				if err != nil {
					return fmt.Errorf("Failed to extract 'destroy' key as boolean: %v", err)
				}

				if d == true {
					summary.Destroyed++
				} else if id == "" {
					summary.Added++
				} else {
					summary.Changed++
				}
			} else if string(key) != "destroy" && string(key) != "destroy_tainted" {
				// Nested module
				err := summarizeTFPlan(value, summary)
				if err != nil {
					return err
				}
			}
		}

		return nil
	}, []string{}...)
}

func runTFJson(planfile string) (string, error) {

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("tfjson-service", planfile)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("Failed to run gsutil: %s\n%v", stderr.String(), err)
	}

	return stdout.String(), err
}

func getTerraform(kind tfv1.TFKind, namespace string, name string) (tfv1.Terraform, error) {
	var tfapply tfv1.Terraform
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("kubectl", "get", kind, "-n", namespace, name, "-o", "yaml")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return tfapply, fmt.Errorf("Failed to run kubectl: %s\n%v", stderr.String(), err)
	}

	err = yaml.Unmarshal(stdout.Bytes(), &tfapply)

	return tfapply, err
}

func getSecretKeys(namespace string, name string) ([]string, error) {
	secrets := config.clientset.CoreV1().Secrets(namespace)
	secret, err := secrets.Get(name, metav1.GetOptions{})
	var secretKeys []string
	for k := range secret.Data {
		secretKeys = append(secretKeys, k)
	}
	return secretKeys, err
}

func getConfigMapSourceData(namespace string, name string) (ConfigMapSourceData, error) {
	configMaps := config.clientset.CoreV1().ConfigMaps(namespace)
	configMap, err := configMaps.Get(name, metav1.GetOptions{})
	return configMap.Data, err
}

func toSha1(data string) string {
	h := sha1.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func getVarsFromTF(kind tfv1.TFKind, namespace, name string) (TerraformInputVars, error) {
	tfVars := make(TerraformInputVars, 0)
	tf, err := getTerraform(kind, namespace, name)
	if err != nil {
		return tfVars, err
	}
	if len(tf.Spec.TFVars) > 0 {
		for k, v := range tf.Spec.TFVars {
			tfVars[k] = v
		}
	}
	return tfVars, nil
}
