package main

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tfv1 "github.com/danisla/terraform-operator/pkg/types"
	"github.com/jinzhu/copier"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (tfp *TFPod) makeTerraformPod(podName, namespace string, kind tfv1.TFKind, currPod *corev1.Pod) (Pod, error) {
	var pod Pod

	envVars := tfp.makeEnvVars(podName)

	volumeMounts := tfp.makeVolumeMounts()

	volumes := tfp.makeVolumes()

	labels := tfp.makeLabels()

	annotations := make(map[string]string, 0)

	if currPod != nil {
		for k, v := range currPod.GetAnnotations() {
			if k == "metacontroller.k8s.io/last-applied-configuration" {
				continue
			} else {
				annotations[k] = v
			}
		}
	}

	objectMeta := ObjectMeta{
		Name:        podName,
		Namespace:   namespace,
		Labels:      labels,
		Annotations: annotations,
	}

	var podCmd string
	switch kind {
	case tfv1.TFKindPlan:
		podCmd = tfDriverConfig.PodCmdPlan
	case tfv1.TFKindApply:
		podCmd = tfDriverConfig.PodCmdApply
	case tfv1.TFKindDestroy:
		podCmd = tfDriverConfig.PodCmdDestroy
	}

	podSpec := corev1.PodSpec{
		ServiceAccountName: tfDriverConfig.PodServiceAccount,

		// Treating this pod like a job, so no restarts.
		RestartPolicy: corev1.RestartPolicyNever,

		InitContainers: tfp.makeInitContainers(),

		Containers: []corev1.Container{
			corev1.Container{
				Name:            TERRAFORM_CONTAINER_NAME,
				Image:           tfp.Image,
				Command:         strings.Split(podCmd, " "),
				ImagePullPolicy: tfp.ImagePullPolicy,
				Env:             envVars,
				VolumeMounts:    volumeMounts,
			},
		},
		Volumes: volumes,
	}

	if currPod != nil {
		copier.Copy(&podSpec, currPod.Spec)
	}

	pod = Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: objectMeta,
		Spec:       podSpec,
	}

	return pod, nil
}

func (tfp *TFPod) makeInitContainers() []corev1.Container {
	initContainers := make([]corev1.Container, 0)

	if len(tfp.SourceData.GCSObjects) > 0 {
		envVars := make([]corev1.EnvVar, 0)

		envVars = append(envVars, tfp.makeProviderEnv()...)

		envVars = append(envVars, corev1.EnvVar{
			Name:  "GCS_TARBALLS",
			Value: strings.Join(tfp.SourceData.GCSObjects, ","),
		})

		initContainers = append(initContainers, corev1.Container{
			Name:            GCS_TARBALL_CONTAINER_NAME,
			Image:           tfp.Image,
			Command:         strings.Split(tfDriverConfig.PodCmdGCSTarball, " "),
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
	for k := range tfp.SourceData.ConfigMapHashes {
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
	for _, t := range tfp.SourceData.ConfigMapKeys {
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

func getImageAndPullPolicy(parent *tfv1.Terraform) (string, corev1.PullPolicy) {
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

func getOrdinalIndex(podName string) int {
	// Expected format is PARENT_NAME-PARENT_TYPE-INDEX
	var validName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?-([0-9]+)$`)
	index := 0
	if validName.MatchString(podName) {
		toks := strings.Split(podName, "-")
		index, _ = strconv.Atoi(toks[len(toks)-1])
	} else {
		log.Printf("WARN: could not extract ordinal index from name: %s", podName)
	}
	return index
}

func makeOrdinalPodName(parent *tfv1.Terraform, index int) string {
	// Expected format is PARENT_NAME-PARENT_TYPE-INDEX
	return fmt.Sprintf("%s-%s-%d", parent.GetName(), parent.GetTFKindShort(), index)
}

func makeTerraformSourceConfigMap(name string, data string, filename string) corev1.ConfigMap {
	cmData := strings.TrimSpace(data)

	cm := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{},
		},
		Data: map[string]string{
			filename: cmData,
		},
	}
	return cm
}

func getBackendBucketandPrefix(parent *tfv1.Terraform) (string, string) {
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
	return backendBucket, filepath.Join(backendPrefix, parent.GetName())
}

func makeStateFilePath(backendBucket, backendPrefix, workspace string) string {
	return fmt.Sprintf("gs://%s/%s/%s.tfstate", backendBucket, backendPrefix, workspace)
}

func makeOutputVarsSecret(name string, namespace string, vars []tfv1.TerraformOutputVar) corev1.Secret {
	var secret corev1.Secret

	data := make(map[string][]byte, 0)

	if vars != nil {
		for _, v := range vars {
			data[v.Name] = []byte(v.Value)
		}
	}

	secret = corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		Data: data,
	}

	return secret
}

func getLastPodIndex(pods map[string]corev1.Pod) int {
	index := 0
	for name := range pods {
		podIndex := getOrdinalIndex(name)
		if podIndex > index {
			index = podIndex
		}
	}
	return index
}

func getPodStatus(pods map[string]corev1.Pod) (int, int, int, int) {
	active := 0
	succeeded := 0
	failed := 0
	index := 0
	for _, pod := range pods {
		switch pod.Status.Phase {
		case corev1.PodSucceeded:
			succeeded++
		case corev1.PodFailed:
			failed++
		default:
			podIndex := getOrdinalIndex(pod.GetName())
			if podIndex > index {
				index = podIndex
			}
			active++
		}
	}
	return active, succeeded, failed, index
}
