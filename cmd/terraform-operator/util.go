package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	tftype "github.com/danisla/terraform-operator/pkg/types"
)

func parseTerraformPlan(planfile string) (tftype.TerraformPlanFileSummary, error) {
	var summary tftype.TerraformPlanFileSummary

	tfplanjson, err := runTFJson(planfile)
	if err != nil {
		return summary, err
	}

	var data map[string]interface{}
	var currMod map[string]interface{}

	err = json.Unmarshal([]byte(tfplanjson), &data)
	if err != nil {
		return summary, fmt.Errorf("Failed to unmarshal tfplan json: %v", err)
	}

	nodes := []string{"root"}
	currMod = data

	destroy := 0
	create := 0
	change := 0

	var n string
	for len(nodes) > 0 {
		n = nodes[0]
		nodes = nodes[1:]

		currMod = currMod[n].(map[string]interface{})

		for k := range currMod {
			toks := strings.Split(k, ".")

			if len(toks) == 2 {

				id := currMod[k].(map[string]interface{})["id"]
				d := currMod[k].(map[string]interface{})["destroy"].(bool)

				if d == true {
					destroy++
				} else if id != nil && id == "" {
					create++
				} else {
					change++
				}

			} else if k == "destroy" {
				// module destroy
			} else {
				// nested module
				nodes = append(nodes, k)
			}
		}
	}

	summary.Added = create
	summary.Changed = change
	summary.Destroyed = destroy

	return summary, nil
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
