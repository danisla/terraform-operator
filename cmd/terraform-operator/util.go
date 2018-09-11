package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/buger/jsonparser"
	tftype "github.com/danisla/terraform-operator/pkg/types"
)

func parseTerraformPlan(planfile string) (tftype.TerraformPlanFileSummary, error) {
	summary := tftype.TerraformPlanFileSummary{}

	tfplanjson, err := runTFJson(planfile)
	if err != nil {
		return summary, err
	}
	err = summarizeTFPlan([]byte(tfplanjson), &summary)
	return summary, err
}

func summarizeTFPlan(data []byte, summary *tftype.TerraformPlanFileSummary) error {
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
