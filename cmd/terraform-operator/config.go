package main

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Config is the configuration structure used by the controller.
type Config struct {
	clientset *kubernetes.Clientset
}

func (c *Config) loadAndValidate() error {
	var err error

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return err
	}
	c.clientset = clientset

	return nil
}
