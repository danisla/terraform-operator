package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/terraform/terraform"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: tfjson terraform.tfplan")
		os.Exit(1)
	}

	j, err := tfjson(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println(j)
}

// Basd on code from: https://github.com/palantir/tfjson

type output map[string]interface{}

func tfjson(planfile string) (string, error) {
	srcPlanFile := planfile

	if planfile[0:5] == "gs://" {
		dir, err := ioutil.TempDir("", "tfplan")
		if err != nil {
			log.Fatal(err)
		}
		defer os.RemoveAll(dir) // clean up

		srcPlanFile = filepath.Join(dir, filepath.Base(planfile))

		// Download plan using gsutil
		if err := getGCSFile(planfile, srcPlanFile); err != nil {
			log.Fatal(err)
		}
	}

	f, err := os.Open(srcPlanFile)
	if err != nil {
		return "", err
	}
	defer f.Close()

	plan, err := terraform.ReadPlan(f)
	if err != nil {
		return "", err
	}

	diff := output{}
	for _, v := range plan.Diff.Modules {
		convertModuleDiff(diff, v)
	}

	j, err := json.MarshalIndent(diff, "", "    ")
	if err != nil {
		return "", err
	}

	return string(j), nil
}

func insert(out output, path []string, key string, value interface{}) {
	// if len(path) > 0 && path[0] == "root" {
	// 	path = path[1:]
	// }
	for _, elem := range path {
		switch nested := out[elem].(type) {
		case output:
			out = nested
		default:
			new := output{}
			out[elem] = new
			out = new
		}
	}
	out[key] = value
}

func convertModuleDiff(out output, diff *terraform.ModuleDiff) {
	insert(out, diff.Path, "destroy", diff.Destroy)
	for k, v := range diff.Resources {
		convertInstanceDiff(out, append(diff.Path, k), v)
	}
}

func convertInstanceDiff(out output, path []string, diff *terraform.InstanceDiff) {
	insert(out, path, "destroy", diff.Destroy)
	insert(out, path, "destroy_tainted", diff.DestroyTainted)
	for k, v := range diff.Attributes {
		insert(out, path, k, v.New)
	}
}

func getGCSFile(src, dest string) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("gsutil", "cp", src, dest)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run gsutil: %s\n%v", stderr.String(), err)
	}

	return err
}
