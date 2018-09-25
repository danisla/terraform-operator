package main

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Pod is a copy of corev1.Pod with the exception of the status field.
// This is used when marshaling so that the Status field does not interfere.
type Pod struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              corev1.PodSpec `json:"spec,omitempty"`
}
