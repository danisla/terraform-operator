package main

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func getProviderConfigSecret(namespace string, name string) ([]string, error) {
	secrets := config.clientset.CoreV1().Secrets(namespace)
	secret, err := secrets.Get(name, metav1.GetOptions{})
	var secretKeys []string
	for k := range secret.Data {
		secretKeys = append(secretKeys, k)
	}
	return secretKeys, err
}
