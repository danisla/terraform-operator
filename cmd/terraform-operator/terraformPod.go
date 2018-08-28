package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PLAN_POD_CMD       = "/run-terraform-plan.sh"
	PLAN_POD_BASE_NAME = ParentPlan
)

// TFPod contains the data needed to create the Terraform Pod
type TFPod struct {
	Image              string
	ImagePullPolicy    corev1.PullPolicy
	Namespace          string
	ProjectID          string
	ConfigMapName      string
	ProviderConfigKeys map[string][]string
	SourceDataKeys     []string
	ConfigMapHash      string
	BackendBucket      string
	BackendPrefix      string
	TFVars             map[string]string
}

func (tfp *TFPod) makeTerraformPod(podName string, cmd []string) (corev1.Pod, error) {
	var pod corev1.Pod

	envVars := tfp.makeEnvVars(podName)

	volumeMounts := tfp.makeVolumeMounts()

	pod = corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"tf-config-map-hash": tfp.ConfigMapHash,
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: DEFAULT_POD_SERVICE_ACCOUNT,

			// Treating this pod like a job, so no restarts.
			RestartPolicy: corev1.RestartPolicyNever,

			Containers: []corev1.Container{
				corev1.Container{
					Name:            "terraform",
					Image:           tfp.Image,
					Command:         cmd,
					ImagePullPolicy: tfp.ImagePullPolicy,
					Env:             envVars,
					VolumeMounts:    volumeMounts,
				},
			},
			Volumes: []corev1.Volume{
				corev1.Volume{
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: tfp.ConfigMapName,
							},
						},
					},
				},
				corev1.Volume{
					Name: "state",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: corev1.StorageMediumMemory,
						},
					},
				},
			},
		},
	}
	return pod, nil
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
		Value: fmt.Sprintf("%s-%s", tfp.Namespace, podName),
	})

	// Output module
	envVars = append(envVars, corev1.EnvVar{
		Name:  "OUTPUT_MODULE",
		Value: "root",
	})

	// TFVars
	var tfVarEnv = regexp.MustCompile(`^TF_VAR_.*$`)
	for k, v := range tfp.TFVars {
		varName := v
		if !tfVarEnv.MatchString(v) {
			varName = fmt.Sprintf("TF_VAR_%s", v)
		}
		envVars = append(envVars, corev1.EnvVar{
			Name:  k,
			Value: varName,
		})
	}

	return envVars
}

func (tfp *TFPod) makeVolumeMounts() []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0)

	// State
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "state",
		MountPath: "/opt/terraform/.terraform",
	})

	// Mount each entity in the config
	for _, k := range tfp.SourceDataKeys {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "config",
			MountPath: filepath.Join("/opt/terraform/", k),
			SubPath:   filepath.Base(k),
		})
	}

	return volumeMounts
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

func makeOrdinalPodName(baseName string, parent *Terraform, children *TerraformControllerRequestChildren) string {
	// Expected format is PARENT_NAME-BASENAME-INDEX
	var validName = regexp.MustCompile(`^([a-z][a-z0-9]+)-([a-z][a-z0-9]+)-([0-9]+)$`)

	i := -1
	for name := range children.Pods {
		if validName.MatchString(name) {
			toks := strings.Split(name, "-")
			num, _ := strconv.Atoi(toks[2])
			if num > i {
				i = num
			}
		} else {
			myLog(parent, "WARN", fmt.Sprintf("Found pod in children list that does not match ordinal pattern: %s", name))
		}
	}
	i++
	return fmt.Sprintf("%s-%s-%d", parent.Name, baseName, i)
}

func makeTerraformSourceConfigMap(name string, data string) (string, corev1.ConfigMap) {
	cm := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			"main.tf": data,
		},
	}

	hash, _ := toSha1(data)
	return hash, cm
}
