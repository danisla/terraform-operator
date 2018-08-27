package main

import (
	"fmt"
	"path/filepath"

	"github.com/ghodss/yaml"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makePatchImage(jobName, imageName string, imagePullPolicy corev1.PullPolicy, cmd []string) ([]byte, error) {
	job := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{Name: jobName},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name:            "terraform",
							Image:           imageName,
							Command:         cmd,
							ImagePullPolicy: imagePullPolicy,
						},
					},
				},
			},
		},
	}
	return yaml.Marshal(job)
}

func makePatchProvider(jobName string, providerConfigKeys map[string][]string) ([]byte, error) {
	var envVars []corev1.EnvVar

	for secretName, keys := range providerConfigKeys {
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

	job := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{Name: jobName},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name: "terraform",
							Env:  envVars,
						},
					},
				},
			},
		},
	}
	return yaml.Marshal(job)
}

func makePatchBackend(jobName, namespace, backendBucket, backendPrefix string) ([]byte, error) {
	envVars := []corev1.EnvVar{
		corev1.EnvVar{
			Name:  "BACKEND_BUCKET",
			Value: backendBucket,
		},
		corev1.EnvVar{
			Name:  "BACKEND_PREFIX",
			Value: backendPrefix,
		},
		corev1.EnvVar{
			Name:  "WORKSPACE",
			Value: fmt.Sprintf("%s-%s", namespace, jobName),
		},
	}

	job := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{Name: jobName},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name: "terraform",
							Env:  envVars,
						},
					},
				},
			},
		},
	}
	return yaml.Marshal(job)
}

func makePatchOutputs(dir string) (string, error) {
	patchPath := filepath.Join(dir, "patch-outputs.yaml")
	return patchPath, nil
}

func makePatchConfigVolume(jobName, configMapName, configMapHash string, sourceDataKeys []string) ([]byte, error) {
	volumeMounts := make([]corev1.VolumeMount, 0)
	for _, k := range sourceDataKeys {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "config",
			MountPath: filepath.Join("/opt/terraform/", k),
			SubPath:   filepath.Base(k),
		})
	}

	job := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"terraform-config-hash": configMapHash,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name:         "terraform",
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: []corev1.Volume{
						corev1.Volume{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return yaml.Marshal(job)
}

func makePatchTFVars(jobName string, tfvars map[string]string) ([]byte, error) {
	envVars := make([]corev1.EnvVar, 0)

	for k, v := range tfvars {
		envVars = append(envVars, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	job := batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{Name: jobName},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name: "terraform",
							Env:  envVars,
						},
					},
				},
			},
		},
	}
	return yaml.Marshal(job)
}
