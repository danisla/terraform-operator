TEST_PLAN_ARTIFACTS := job1-cm.yaml job1-cm-tfplan.yaml job2-src-tfplan.yaml job3-cm-tfplan-inputs.yaml
TEST_APPLY_ARTIFACTS := job1-cm.yaml job1-cm-tfapply.yaml job1-cm-tfapply-tfplan.yaml job2-src-tfapply.yaml
TEST_DESTROY_ARTIFACTS := job1-cm.yaml job1-cm-tfdestroy.yaml job2-src-tfdestroy.yaml

IMAGE := "gcr.io/cloud-solutions-group/terraform-pod:latest"

TEST_ARTIFACTS := $(TEST_PLAN_ARTIFACTS) $(TEST_APPLY_ARTIFACTS) $(TEST_DESTROY_ARTIFACTS)

GOOGLE_CREDENTIALS_SA_KEY := $(HOME)/.tf-google-sa-key.json
GOOGLE_PROVIDER_SECRET_NAME := tf-google

project:
	$(eval PROJECT := $(shell gcloud config get-value project))

backend_bucket: project
	$(eval BACKEND_BUCKET := $(PROJECT)-terraform-operator)

define TEST_CM
apiVersion: v1
kind: ConfigMap 
metadata: 
  name: {{NAME}}
data: 
  terraform.tfvars: |-
    region = "us-central1"
  main.tf: |-
    variable "region" {
      default = "us-central1"
    }
    provider "google" {
      region = "$${var.region}"
    }
    resource "google_compute_project_metadata_item" "default" {
      key = "tf-job-test"
      value = "tf-operator-test"
    }
    data "google_client_config" "current" {}
    output "project" {
      value = "$${data.google_client_config.current.project}"
    }
    output "region" {
      value = "$${var.region}"
    }
    output "metadata_key" {
      value = "$${google_compute_project_metadata_item.default.key}"
    }
    output "metadata_value" {
      value = "$${google_compute_project_metadata_item.default.value}"
    }
endef

define TEST_JOB_SRC
apiVersion: ctl.isla.solutions/v1
kind: {{KIND}}
metadata:
  name: {{NAME}}
spec:
  image: {{IMAGE}}
  imagePullPolicy: Always
  backendBucket: {{BACKEND_BUCKET}}
  backendPrefix: {{BACKEND_PREFIX}}
  providerConfig:
    google:
      secretName: {{GOOGLE_PROVIDER_SECRET_NAME}}
  source:
    embedded: |-
      variable "region" {
        default = "us-central1"
      }
      provider "google" {
        region = "$${var.region}"
      }
      resource "google_compute_project_metadata_item" "default" {
        key = "tf-job-test"
        value = "tf-operator-test"
      }
      data "google_client_config" "current" {}
      output "project" {
        value = "$${data.google_client_config.current.project}"
      }
      output "region" {
        value = "$${var.region}"
      }
      output "metadata_key" {
        value = "$${google_compute_project_metadata_item.default.key}"
      }
      output "metadata_value" {
        value = "$${google_compute_project_metadata_item.default.value}"
      }
  tfvars:
    region: us-central1
endef

define TEST_JOB_CM
apiVersion: ctl.isla.solutions/v1
kind: {{KIND}}
metadata:
  name: {{NAME}}
spec:
  image: {{IMAGE}}
  imagePullPolicy: Always
  backendBucket: {{BACKEND_BUCKET}}
  backendPrefix: {{BACKEND_PREFIX}}
  providerConfig:
    google:
      secretName: {{GOOGLE_PROVIDER_SECRET_NAME}}
  source:
    configMap:
      name: {{CM_NAME}}
      trigger: true
  configMapName: {{CM_NAME}}
  {{TFPLAN}}
  tfvars:
    region: us-central1
endef

define TEST_JOB_CM_INPUTS
apiVersion: ctl.isla.solutions/v1
kind: {{KIND}}
metadata:
  name: {{NAME}}
spec:
  image: {{IMAGE}}
  imagePullPolicy: Always
  backendBucket: {{BACKEND_BUCKET}}
  backendPrefix: {{BACKEND_PREFIX}}
  providerConfig:
    google:
      secretName: {{GOOGLE_PROVIDER_SECRET_NAME}}
  source:
    configMap:
      name: {{CM_NAME}}
      trigger: true
  configMapName: {{CM_NAME}}
  tfinputs:
  - name: {{TFAPPLY_NAME}}
    varMap:
      {{SRC_VAR}}: {{DEST_VAR}}
  tfvars:
    region: us-central1
endef

credentials: $(GOOGLE_CREDENTIALS_SA_KEY) project
	kubectl create secret generic $(GOOGLE_PROVIDER_SECRET_NAME) --from-literal=GOOGLE_PROJECT=$(PROJECT) --from-file=GOOGLE_CREDENTIALS=$(GOOGLE_CREDENTIALS_SA_KEY)

export TEST_CM
tests/job%-cm.yaml: project
	@mkdir -p tests
	@echo "$${TEST_CM}" | \
	sed -e "s/{{NAME}}/job$*-tf/g" \
        -e "s/{{PROJECT}}/$(PROJECT)/g" \
	> $@

### BEGIN Tests with ConfigMap source ###
export TEST_JOB_CM
tests/job%-cm-tfplan.yaml: backend_bucket
	@mkdir -p tests
	@echo "$${TEST_JOB_CM}" | \
	sed -e "s/{{KIND}}/TerraformPlan/g" \
	    -e "s/{{NAME}}/job$*/g" \
	    -e "s|{{IMAGE}}|$(IMAGE)|g" \
	    -e "s/{{BACKEND_BUCKET}}/$(BACKEND_BUCKET)/g" \
	    -e "s/{{BACKEND_PREFIX}}/terraform/g" \
	    -e "s/{{GOOGLE_PROVIDER_SECRET_NAME}}/$(GOOGLE_PROVIDER_SECRET_NAME)/g" \
	    -e "s/{{CM_NAME}}/job$*-tf/g" \
	    -e "s/{{TFPLAN}}//g" \
	> $@

export TEST_JOB_CM
tests/job%-cm-tfapply.yaml: backend_bucket
	@mkdir -p tests
	@echo "$${TEST_JOB_CM}" | \
	sed -e "s/{{KIND}}/TerraformApply/g" \
	    -e "s/{{NAME}}/job$*/g" \
	    -e "s|{{IMAGE}}|$(IMAGE)|g" \
	    -e "s/{{BACKEND_BUCKET}}/$(BACKEND_BUCKET)/g" \
	    -e "s/{{BACKEND_PREFIX}}/terraform/g" \
	    -e "s/{{GOOGLE_PROVIDER_SECRET_NAME}}/$(GOOGLE_PROVIDER_SECRET_NAME)/g" \
	    -e "s/{{CM_NAME}}/job$*-tf/g" \
	    -e "s/{{TFPLAN}}//g" \
	> $@

export TEST_JOB_CM
tests/job%-cm-tfapply-tfplan.yaml: backend_bucket
	@mkdir -p tests
	@echo "$${TEST_JOB_CM}" | \
	sed -e "s/{{KIND}}/TerraformApply/g" \
	    -e "s/{{NAME}}/job$*/g" \
	    -e "s|{{IMAGE}}|$(IMAGE)|g" \
	    -e "s/{{BACKEND_BUCKET}}/$(BACKEND_BUCKET)/g" \
	    -e "s/{{BACKEND_PREFIX}}/terraform/g" \
	    -e "s/{{GOOGLE_PROVIDER_SECRET_NAME}}/$(GOOGLE_PROVIDER_SECRET_NAME)/g" \
	    -e "s/{{CM_NAME}}/job$*-tf/g" \
	    -e "s/{{TFPLAN}}/tfplan: job$*/g" \
	> $@

export TEST_JOB_CM
tests/job%-cm-tfdestroy.yaml: backend_bucket
	@mkdir -p tests
	@echo "$${TEST_JOB_CM}" | \
	sed -e "s/{{KIND}}/TerraformDestroy/g" \
	    -e "s/{{NAME}}/job$*/g" \
	    -e "s|{{IMAGE}}|$(IMAGE)|g" \
	    -e "s/{{BACKEND_BUCKET}}/$(BACKEND_BUCKET)/g" \
	    -e "s/{{BACKEND_PREFIX}}/terraform/g" \
	    -e "s/{{GOOGLE_PROVIDER_SECRET_NAME}}/$(GOOGLE_PROVIDER_SECRET_NAME)/g" \
	    -e "s/{{CM_NAME}}/job$*-tf/g" \
	    -e "s/{{TFPLAN}}//g" \
	> $@
### END Tests with ConfigMap source ###


### BEGIN Tests with embedded terraform ###
export TEST_JOB_SRC
tests/job%-src-tfplan.yaml: backend_bucket
	@mkdir -p tests
	@echo "$${TEST_JOB_SRC}" | \
	sed -e "s/{{KIND}}/TerraformPlan/g" \
	    -e "s/{{NAME}}/job$*/g" \
	    -e "s|{{IMAGE}}|$(IMAGE)|g" \
	    -e "s/{{BACKEND_BUCKET}}/$(BACKEND_BUCKET)/g" \
	    -e "s/{{BACKEND_PREFIX}}/terraform/g" \
	    -e "s/{{GOOGLE_PROVIDER_SECRET_NAME}}/$(GOOGLE_PROVIDER_SECRET_NAME)/g" \
	> $@

export TEST_JOB_SRC
tests/job%-src-tfapply.yaml: backend_bucket
	@mkdir -p tests
	@echo "$${TEST_JOB_SRC}" | \
	sed -e "s/{{KIND}}/TerraformApply/g" \
	    -e "s/{{NAME}}/job$*/g" \
	    -e "s|{{IMAGE}}|$(IMAGE)|g" \
	    -e "s/{{BACKEND_BUCKET}}/$(BACKEND_BUCKET)/g" \
	    -e "s/{{BACKEND_PREFIX}}/terraform/g" \
	    -e "s/{{GOOGLE_PROVIDER_SECRET_NAME}}/$(GOOGLE_PROVIDER_SECRET_NAME)/g" \
	> $@

export TEST_JOB_SRC
tests/job%-src-tfdestroy.yaml: backend_bucket
	@mkdir -p tests
	@echo "$${TEST_JOB_SRC}" | \
	sed -e "s/{{KIND}}/TerraformDestroy/g" \
	    -e "s/{{NAME}}/job$*/g" \
	    -e "s|{{IMAGE}}|$(IMAGE)|g" \
	    -e "s/{{BACKEND_BUCKET}}/$(BACKEND_BUCKET)/g" \
	    -e "s/{{BACKEND_PREFIX}}/terraform/g" \
	    -e "s/{{GOOGLE_PROVIDER_SECRET_NAME}}/$(GOOGLE_PROVIDER_SECRET_NAME)/g" \
	> $@

### END Tests with embedded terraform ###

### BEGIN Tests with configmap source and tfapply inputs
export TEST_JOB_CM_INPUTS
tests/job%-cm-tfplan-inputs.yaml: backend_bucket
	@mkdir -p tests
	@echo "$${TEST_JOB_CM_INPUTS}" | \
	sed -e "s/{{KIND}}/TerraformPlan/g" \
	    -e "s/{{NAME}}/job$*/g" \
	    -e "s|{{IMAGE}}|$(IMAGE)|g" \
	    -e "s/{{BACKEND_BUCKET}}/$(BACKEND_BUCKET)/g" \
	    -e "s/{{BACKEND_PREFIX}}/terraform/g" \
	    -e "s/{{GOOGLE_PROVIDER_SECRET_NAME}}/$(GOOGLE_PROVIDER_SECRET_NAME)/g" \
	    -e "s/{{TFAPPLY_NAME}}/job1/g" \
	    -e "s/{{CM_NAME}}/job1-tf/g" \
	    -e "s/{{SRC_VAR}}/region/g" \
	    -e "s/{{DEST_VAR}}/region_input/g" \
	> $@

### END Tests with configmap source and tfapply inputs

test-artifacts: $(addprefix tests/,$(TEST_ARTIFACTS))

test: $(addprefix tests/,$(TEST_PLAN_ARTIFACTS))
	-@for f in $^; do kubectl apply -f $$f; done

test-plan: $(addprefix tests/,$(TEST_PLAN_ARTIFACTS))
	-@for f in $^; do kubectl apply -f $$f; done

test-apply: $(addprefix tests/,$(TEST_APPLY_ARTIFACTS))
	-@for f in $^; do kubectl apply -f $$f; done

test-destroy: $(addprefix tests/,$(TEST_DESTROY_ARTIFACTS))
	-@for f in $^; do kubectl apply -f $$f; done

test-stop: $(addprefix tests/,$(TEST_ARTIFACTS))
	-@for f in $^; do kubectl delete -f $$f; done

test-clean: $(addprefix tests/,$(TEST_ARTIFACTS))
	rm -f $^