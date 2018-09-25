package main

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Pod is a copy of corev1.Pod with the exception of the status field.
// This is used when marshaling so that the Status field does not interfere.
type Pod struct {
	metav1.TypeMeta `json:",inline"`
	ObjectMeta      `json:"metadata,omitempty"`
	Spec            corev1.PodSpec `json:"spec,omitempty"`
}

// ObjectMeta is a copy of metav1.ObjectMeta with only the fields the operater is interested in.
type ObjectMeta struct {
	Name        string            `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Namespace   string            `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
	Labels      map[string]string `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`
}
