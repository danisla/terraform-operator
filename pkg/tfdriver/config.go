package tfdriver

import (
	"fmt"
	"log"
	"os"
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

const (
	DEFAULT_TF_PROVIDER_SECRET = "tf-provider-google"
)

// TerraformDriverConfig is the Terraform driver config
type TerraformDriverConfig struct {
	Image                      string
	ImagePullPolicy            corev1.PullPolicy
	PodServiceAccount          string
	BackendBucket              string
	BackendPrefix              string
	MaxAttempts                int32
	BackoffScale               float64
	GoogleProviderConfigSecret string
	PodCmdPlan                 string
	PodCmdApply                string
	PodCmdDestroy              string
	PodCmdGCSTarball           string
}

func (c *TerraformDriverConfig) LoadAndValidate(project string) error {

	// TF_IMAGE is optional
	c.Image, _ = os.LookupEnv("TF_IMAGE")

	// TF_IMAGE_PULL_POLICY is optional
	if pullPolicy, ok := os.LookupEnv("TF_IMAGE_PULL_POLICY"); ok == true {
		c.ImagePullPolicy = corev1.PullPolicy(pullPolicy)
	} else {
		c.ImagePullPolicy = corev1.PullIfNotPresent
	}

	// TF_POD_SERVICE_ACCOUNT is optional
	if serviceAccount, ok := os.LookupEnv("TF_POD_SERVICE_ACCOUNT"); ok == true {
		c.PodServiceAccount = serviceAccount
	} else {
		c.PodServiceAccount = "terraform"
	}

	// TF_BACKEND_BUCKET is required
	if backendBucket, ok := os.LookupEnv("TF_BACKEND_BUCKET"); ok == true {
		c.BackendBucket = backendBucket
	} else {
		// Create bucket name from project name.
		c.BackendBucket = fmt.Sprintf("%s-terraform-operator", project)
		log.Printf("[INFO] No TF_BACKEND_BUCKET given, using canonical bucket name: %s", c.BackendBucket)
	}

	// TF_BACKEND_PREFIX is required
	if backendPrefix, ok := os.LookupEnv("TF_BACKEND_PREFIX"); ok == true {
		c.BackendPrefix = backendPrefix
	} else {
		// Use default prefix.
		c.BackendPrefix = "terraform"
		log.Printf("[INFO] No TF_BACKEND_PREFIX given, using default prefix of: %s", c.BackendPrefix)
	}

	// TF_MAX_ATTEMPTS is optional
	if maxAttempts, ok := os.LookupEnv("TF_MAX_ATTEMPTS"); ok == true {
		i, err := strconv.Atoi(maxAttempts)
		if err != nil {
			return fmt.Errorf("Invalid number for TF_MAX_ATTEMPTS: %s, must be positive integer", maxAttempts)
		}
		if i <= 0 {
			return fmt.Errorf("Invalid number for TF_MAX_ATTEMPTS: %s, must be positive integer", maxAttempts)
		}

		c.MaxAttempts = int32(i)
	} else {
		c.MaxAttempts = 4
		log.Printf("[INFO] No TF_MAX_ATTEMPTS given, using default count of: %d", c.MaxAttempts)
	}

	// TF_BACKOFF_SCALE is optional
	if backoffScale, ok := os.LookupEnv("TF_BACKOFF_SCALE"); ok == true {
		f, err := strconv.ParseFloat(backoffScale, 64)
		if err != nil {
			return fmt.Errorf("Invalid float for TF_BACKOFF_SCALE: %s, must be a valid float", backoffScale)
		}
		if f < 1 {
			return fmt.Errorf("Invalid float for TF_BACKOFF_SCALE: %s, must greater than or equal to 1.0", backoffScale)
		}

		c.BackoffScale = f
	} else {
		c.BackoffScale = 5.0
	}

	// TF_POD_PLAN_CMD is optional
	if podCmd, ok := os.LookupEnv("TF_POD_PLAN_CMD"); ok == true {
		c.PodCmdPlan = podCmd
	} else {
		c.PodCmdPlan = "/run-terraform-plan.sh"
	}

	// TF_POD_APPLY_CMD is optional
	if podCmd, ok := os.LookupEnv("TF_POD_APPLY_CMD"); ok == true {
		c.PodCmdApply = podCmd
	} else {
		c.PodCmdApply = "/run-terraform-apply.sh"
	}

	// TF_POD_DESTROY_CMD is optional
	if podCmd, ok := os.LookupEnv("TF_POD_DESTROY_CMD"); ok == true {
		c.PodCmdDestroy = podCmd
	} else {
		c.PodCmdDestroy = "/run-terraform-destroy.sh"
	}

	// TF_POD_GCS_TARBALL_CMD is optional
	if podCmd, ok := os.LookupEnv("TF_POD_GCS_TARBALL_CMD"); ok == true {
		c.PodCmdGCSTarball = podCmd
	} else {
		c.PodCmdGCSTarball = "/get-gcs-tarball.sh"
	}

	if googleConfigSecret, ok := os.LookupEnv("TF_GOOGLE_PROVIDER_SECRET"); ok == true {
		c.GoogleProviderConfigSecret = googleConfigSecret
	} else {
		c.GoogleProviderConfigSecret = DEFAULT_TF_PROVIDER_SECRET
		log.Printf("[INFO] No TF_GOOGLE_PROVIDER_SECRET given, using default: %s", c.GoogleProviderConfigSecret)
	}

	return nil
}
