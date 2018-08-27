package main

import (
	"log"

	"cloud.google.com/go/compute/metadata"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Config is the configuration structure used by the controller.
type Config struct {
	Project    string
	ProjectNum string
	clientset  *kubernetes.Clientset
}

func (c *Config) loadAndValidate() error {
	var err error

	if c.Project == "" {
		log.Printf("[INFO] Fetching Project ID from Compute metadata API...")
		c.Project, err = metadata.ProjectID()
		if err != nil {
			return err
		}
	}

	if c.ProjectNum == "" {
		log.Printf("[INFO] Fetching Numeric Project ID from Compute metadata API...")
		c.ProjectNum, err = metadata.NumericProjectID()
		if err != nil {
			return err
		}
	}

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
