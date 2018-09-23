package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tftype "github.com/danisla/terraform-operator/pkg/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Paths to scripts in the Terraform Pod container
const (
	PLAN_POD_CMD    = "/run-terraform-plan.sh"
	APPLY_POD_CMD   = "/run-terraform-apply.sh"
	DESTROY_POD_CMD = "/run-terraform-destroy.sh"
	GCS_TARBALL_CMD = "/get-gcs-tarball.sh"
)

// Name of the containers in the Terraform Pod
const (
	TERRAFORM_CONTAINER_NAME   = "terraform"
	GCS_TARBALL_CONTAINER_NAME = "gcs-tarball"
)

// TFPod contains the data needed to create the Terraform Pod
type TFPod struct {
	Image              string
	ImagePullPolicy    corev1.PullPolicy
	Namespace          string
	ProjectID          string
	Workspace          string
	SourceData         TerraformConfigSourceData
	ProviderConfigKeys ProviderConfigKeys
	BackendBucket      string
	BackendPrefix      string
	TFParent           string
	TFPlan             string
	TFInputs           TerraformInputVars
	TFVarsFrom         TerraformInputVars
	TFVars             TerraformInputVars
}

func (tfp *TFPod) makeTerraformPod(podName string, cmd []string) (corev1.Pod, error) {
	var pod corev1.Pod

	envVars := tfp.makeEnvVars(podName)

	volumeMounts := tfp.makeVolumeMounts()

	volumes := tfp.makeVolumes()

	labels := tfp.makeLabels()

	pod = corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   podName,
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: tfDriverConfig.PodServiceAccount,

			// Treating this pod like a job, so no restarts.
			RestartPolicy: corev1.RestartPolicyNever,

			InitContainers: tfp.makeInitContainers(),

			Containers: []corev1.Container{
				corev1.Container{
					Name:            TERRAFORM_CONTAINER_NAME,
					Image:           tfp.Image,
					Command:         cmd,
					ImagePullPolicy: tfp.ImagePullPolicy,
					Env:             envVars,
					VolumeMounts:    volumeMounts,
				},
			},
			Volumes: volumes,
		},
	}
	return pod, nil
}

func (tfp *TFPod) makeInitContainers() []corev1.Container {
	initContainers := make([]corev1.Container, 0)

	if len(*tfp.SourceData.GCSObjects) > 0 {
		envVars := make([]corev1.EnvVar, 0)

		envVars = append(envVars, tfp.makeProviderEnv()...)

		envVars = append(envVars, corev1.EnvVar{
			Name:  "GCS_TARBALLS",
			Value: strings.Join(*tfp.SourceData.GCSObjects, ","),
		})

		initContainers = append(initContainers, corev1.Container{
			Name:            GCS_TARBALL_CONTAINER_NAME,
			Image:           tfp.Image,
			Command:         []string{GCS_TARBALL_CMD},
			ImagePullPolicy: tfp.ImagePullPolicy,
			Env:             envVars,
			VolumeMounts: []corev1.VolumeMount{
				corev1.VolumeMount{
					Name:      "state",
					MountPath: "/opt/terraform/",
				},
			},
		})
	}

	return initContainers
}

func (tfp *TFPod) makeProviderEnv() []corev1.EnvVar {
	envVars := make([]corev1.EnvVar, 0)

	// Project ID
	envVars = append(envVars, corev1.EnvVar{
		Name:  "PROJECT_ID",
		Value: tfp.ProjectID,
	})

	// Provider env
	for secretName, keys := range tfp.ProviderConfigKeys {
		for _, k := range keys {
			envVars = append(envVars, corev1.EnvVar{
				Name: k,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
						Key: k,
					},
				},
			})
		}
	}

	return envVars
}

func (tfp *TFPod) makeEnvVars(podName string) []corev1.EnvVar {
	envVars := make([]corev1.EnvVar, 0)

	// Project ID
	envVars = append(envVars, corev1.EnvVar{
		Name:  "PROJECT_ID",
		Value: tfp.ProjectID,
	})

	// Pod name from downward API
	envVars = append(envVars, corev1.EnvVar{
		Name: "POD_NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	})

	// Namespace from downward API
	envVars = append(envVars, corev1.EnvVar{
		Name: "NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	})

	// Provider envvars
	envVars = append(envVars, tfp.makeProviderEnv()...)

	// Terraform remote backend
	envVars = append(envVars, corev1.EnvVar{
		Name:  "BACKEND_BUCKET",
		Value: tfp.BackendBucket,
	})
	envVars = append(envVars, corev1.EnvVar{
		Name:  "BACKEND_PREFIX",
		Value: tfp.BackendPrefix,
	})
	envVars = append(envVars, corev1.EnvVar{
		Name:  "WORKSPACE",
		Value: tfp.Workspace,
	})

	// Output module
	envVars = append(envVars, corev1.EnvVar{
		Name:  "OUTPUT_MODULE",
		Value: "root",
	})

	// TFVars
	var tfVarEnv = regexp.MustCompile(`^TF_VAR_.*$`)
	for k, v := range tfp.TFVars {
		varName := k
		if !tfVarEnv.MatchString(v) {
			varName = fmt.Sprintf("TF_VAR_%s", varName)
		}
		envVars = append(envVars, corev1.EnvVar{
			Name:  varName,
			Value: v,
		})
	}

	// TFVarsFrom
	for k, v := range tfp.TFVarsFrom {
		varName := k
		if !tfVarEnv.MatchString(v) {
			varName = fmt.Sprintf("TF_VAR_%s", varName)
		}
		envVars = append(envVars, corev1.EnvVar{
			Name:  varName,
			Value: v,
		})
	}

	// Vars from TerraformApply outputs
	for k, v := range tfp.TFInputs {
		varName := k
		if !tfVarEnv.MatchString(v) {
			varName = fmt.Sprintf("TF_VAR_%s", varName)
		}
		envVars = append(envVars, corev1.EnvVar{
			Name:  varName,
			Value: v,
		})
	}

	// TF Plan var to apply existing plan.
	if tfp.TFPlan != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "TFPLAN",
			Value: tfp.TFPlan,
		})
	}

	return envVars
}

func (tfp *TFPod) makeVolumes() []corev1.Volume {
	volumes := make([]corev1.Volume, 0)

	// State empty dir
	volumes = append(volumes, corev1.Volume{
		Name: "state",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	})

	// ConfigMap volumes
	var defaultMode int32 = 438
	for k := range *tfp.SourceData.ConfigMapHashes {
		volumes = append(volumes, corev1.Volume{
			Name: k,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: k,
					},
					DefaultMode: &defaultMode,
				},
			},
		})
	}

	return volumes
}

func (tfp *TFPod) makeVolumeMounts() []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0)

	// State
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "state",
		MountPath: "/opt/terraform/",
	})

	// Mount each entity in the config
	for _, t := range *tfp.SourceData.ConfigMapKeys {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      t[0],
			MountPath: filepath.Join("/opt/terraform/", t[1]),
			SubPath:   filepath.Base(t[1]),
		})
	}

	return volumeMounts
}

func (tfp *TFPod) makeLabels() map[string]string {
	labels := make(map[string]string, 0)

	labels["terraform-parent"] = tfp.TFParent

	return labels
}

func getImageAndPullPolicy(parent *tftype.Terraform) (string, corev1.PullPolicy) {
	var image string
	var pullPolicy corev1.PullPolicy

	if parent.Spec.Image != "" {
		image = parent.Spec.Image
	} else {
		image = tfDriverConfig.Image
	}

	if parent.Spec.ImagePullPolicy != "" {
		pullPolicy = corev1.PullPolicy(parent.Spec.ImagePullPolicy)
	} else {
		pullPolicy = tfDriverConfig.ImagePullPolicy
	}

	return image, pullPolicy
}

func makeOrdinalPodName(parent *tftype.Terraform, children *TerraformChildren) string {
	// Expected format is PARENT_NAME-PARENT_TYPE-INDEX
	var validName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?-([0-9]+)$`)

	i := -1
	for name := range children.Pods {
		if validName.MatchString(name) {
			toks := strings.Split(name, "-")
			num, _ := strconv.Atoi(toks[len(toks)-1])
			if num > i {
				i = num
			}
		} else {
			parent.Log("WARN", "Found pod in children list that does not match ordinal pattern: %s", name)
		}
	}
	i++
	return fmt.Sprintf("%s-%s-%d", parent.GetName(), parent.GetTFKindShort(), i)
}

func makeTerraformSourceConfigMap(name string, data string) corev1.ConfigMap {
	cmData := strings.TrimSpace(data)
	keyName := "main.tf"

	cm := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			keyName: cmData,
		},
	}
	return cm
}

func getBackendBucketandPrefix(parent *tftype.Terraform) (string, string) {
	backendBucket := parent.Spec.BackendBucket
	if backendBucket == "" {
		// Use default from config.
		backendBucket = tfDriverConfig.BackendBucket
	}
	backendPrefix := parent.Spec.BackendPrefix
	if backendPrefix == "" {
		// Create canonical prefix.
		backendPrefix = tfDriverConfig.BackendPrefix
	}
	return backendBucket, backendPrefix
}

func makeStateFilePath(backendBucket, backendPrefix, workspace string) string {
	return fmt.Sprintf("gs://%s/%s/%s.tfstate", backendBucket, backendPrefix, workspace)
}

func makeOutputVarsSecret(name string, namespace string, vars *[]tftype.TerraformOutputVar) corev1.Secret {
	var secret corev1.Secret

	data := make(map[string]string, 0)

	for _, v := range *vars {
		data[v.Name] = v.Value
	}

	secret = corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: data,
	}

	return secret
}
