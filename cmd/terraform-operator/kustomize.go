package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ktypes "github.com/danisla/terraform-operator/pkg/types"

	"github.com/ghodss/yaml"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PLAN_JOB_CMD       = "/run-terraform-plan.sh"
	PLAN_JOB_BASE_DIR  = "/config/terraform-plan-job"
	PLAN_JOB_BASE_NAME = ParentPlan

	PATCH_IMAGE         = "patch-image.yaml"
	PATCH_PROVIDER      = "patch-provider.yaml"
	PATCH_BACKEND       = "patch-backend.yaml"
	PATCH_CONFIG_VOLUME = "patch-config-volume.yaml"
	PATCH_TFVARS        = "patch-tfvars.yaml"
)

type TFKustomization struct {
	Image              string
	ImagePullPolicy    corev1.PullPolicy
	Namespace          string
	ConfigMapName      string
	ProviderConfigKeys map[string][]string
	SourceDataKeys     []string
	ConfigMapHash      string
	BackendBucket      string
	BackendPrefix      string
	TFVars             map[string]string
}

func (tfk *TFKustomization) makeKustomization(baseDir, baseJobName, namePrefix string, cmd []string) (string, error) {
	var k ktypes.Kustomization
	var kustomizationPath string
	var data []byte
	var err error

	dir, err := ioutil.TempDir("", "terraform-operator-")
	if err != nil {
		return kustomizationPath, err
	}

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return kustomizationPath, err
	}

	// Create the Terraform provider patch
	data, err = makePatchProvider(baseJobName, tfk.ProviderConfigKeys)
	if err != nil {
		return kustomizationPath, fmt.Errorf("Error generating provider patch: %v", err)
	}
	patchProvider := filepath.Join(dir, PATCH_PROVIDER)
	err = ioutil.WriteFile(patchProvider, data, os.ModePerm)

	// Create the image patch
	data, err = makePatchImage(baseJobName, tfk.Image, tfk.ImagePullPolicy, cmd)
	if err != nil {
		return kustomizationPath, fmt.Errorf("Error generating image patch: %v", err)
	}
	patchImage := filepath.Join(dir, PATCH_IMAGE)
	err = ioutil.WriteFile(patchImage, data, os.ModePerm)

	// Create the backend patch
	data, err = makePatchBackend(baseJobName, tfk.Namespace, tfk.BackendBucket, tfk.BackendPrefix)
	if err != nil {
		return kustomizationPath, fmt.Errorf("Error generating backend patch: %v", err)
	}
	patchBackend := filepath.Join(dir, PATCH_BACKEND)
	err = ioutil.WriteFile(patchBackend, data, os.ModePerm)

	// Create the config volume patch
	data, err = makePatchConfigVolume(baseJobName, tfk.ConfigMapName, tfk.ConfigMapHash, tfk.SourceDataKeys)
	if err != nil {
		return kustomizationPath, fmt.Errorf("Error generating config volume patch: %v", err)
	}
	patchConfigVolume := filepath.Join(dir, PATCH_CONFIG_VOLUME)
	err = ioutil.WriteFile(patchConfigVolume, data, os.ModePerm)

	// Create the tfvars patch
	data, err = makePatchTFVars(baseJobName, tfk.TFVars)
	if err != nil {
		return kustomizationPath, fmt.Errorf("Error generating tfvars patch: %v", err)
	}
	patchTFVars := filepath.Join(dir, PATCH_TFVARS)
	err = ioutil.WriteFile(patchTFVars, data, os.ModePerm)

	k = ktypes.Kustomization{
		NamePrefix: namePrefix,
		Bases:      []string{baseAbs},
		Patches: []string{
			patchImage,
			patchProvider,
			patchBackend,
			patchConfigVolume,
			patchTFVars,
		},
	}

	kustomizationPath = filepath.Join(dir, "kustomization.yaml")
	f, err := os.Create(kustomizationPath)
	defer f.Close()

	data, err = yaml.Marshal(k)
	if err != nil {
		return kustomizationPath, err
	}

	err = ioutil.WriteFile(kustomizationPath, data, os.ModePerm)

	return kustomizationPath, err
}

func makeKustomizeConfigMapName(name string, parentType ParentType) string {
	return fmt.Sprintf("%s-%s-kustomize-build", name, parentType)
}

func buildKustomization(srcPath string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("kustomize", "build", filepath.Dir(srcPath))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stdout.String(), fmt.Errorf("Failed to run kustomize: %s\n%v", stderr.String(), err)
	}
	return stdout.String(), nil
}

func makeKustomizeBuildConfigMap(name, build string) (corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			"build.yaml": build,
		},
	}

	return cm, nil
}

func splitKustomizeBuildOutput(build string) ([]interface{}, error) {
	resources := make([]interface{}, 0)

	for _, r := range strings.Split(build, "---") {
		var resource interface{}
		err := yaml.Unmarshal([]byte(r), &resource)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}

	return resources, nil
}

func getResourceKindName(resource interface{}) (string, string) {
	kind := resource.(map[string]interface{})["kind"].(string)
	name := resource.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)
	return kind, name
}

func getImageAndPullPolicy(parent *Terraform) (string, corev1.PullPolicy) {
	var image string
	var pullPolicy corev1.PullPolicy

	if parent.Spec.Image != "" {
		image = parent.Spec.Image
	} else {
		image = DEFAULT_IMAGE
	}

	if parent.Spec.ImagePullPolicy != "" {
		pullPolicy = parent.Spec.ImagePullPolicy
	} else {
		pullPolicy = DEFAULT_IMAGE_PULL_POLICY
	}

	return image, pullPolicy
}
