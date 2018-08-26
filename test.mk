TEST_PLAN_ARTIFACTS := job1-cm.yaml job1-tfplan.yaml
TEST_APPLY_ARTIFACTS := job1-cm.yaml job1-tfapply.yaml 
TEST_DESTROY_ARTIFACTS := job1-cm.yaml job1-tfdestroy.yaml

TEST_ARTIFACTS := $(TEST_PLAN_ARTIFACTS) $(TEST_APPLY_ARTIFACTS) $(TEST_DESTROY_ARTIFACTS)

CREDENTIALS_SECRET_YAML := $(HOME)/.secrets-tf-common-disla.yaml
CREDENTIALS_SECRET_NAME := common
CREDENTIALS_SECRET_KEY := service_account_json

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
    project = {{PROJECT}}
  main.tf: |-
    variable "region" {
      default = "us-central1"
    }
    provider "google" {
      region = "$${var.region}"
    }
    resource "google_compute_project_metadata_item" "default" {
      key = "{{NAME}}"
      value = "tf-operator-test"
    }
endef

define TEST_JOB
apiVersion: ctl.isla.solutions/v1
kind: {{KIND}}
metadata:
  name: {{NAME}}
spec:
  backendBucket: {{BACKEND_BUCKET}}
  backendPrefix: {{BACKEND_PREFIX}}
  credentialsSecret:
    name: {{CREDS_NAME}}
    key: {{CREDS_KEY}}
  source:
    configMap:
      name: {{CM_NAME}}
      trigger: true
  configMapName: {{CM_NAME}}
  tfvars:
    region: us-central1
endef

credentials: $(CREDENTIALS_SECRET_YAML)
	kubectl apply -f $<

export TEST_CM
job%-cm.yaml: project
	@echo "$${TEST_CM}" | \
	sed -e "s/{{NAME}}/job$*/g" \
        -e "s/{{PROJECT}}/$(PROJECT)/g" \
	> $@

export TEST_JOB
job%-tfplan.yaml: backend_bucket
	@echo "$${TEST_JOB}" | \
	sed -e "s/{{KIND}}/TerraformPlan/g" \
	    -e "s/{{NAME}}/job$*/g" \
	    -e "s/{{BACKEND_BUCKET}}/$(BACKEND_BUCKET)/g" \
	    -e "s/{{BACKEND_PREFIX}}/terraform/g" \
	    -e "s/{{CREDS_NAME}}/$(CREDENTIALS_SECRET_NAME)/g" \
	    -e "s/{{CREDS_KEY}}/$(CREDENTIALS_SECRET_KEY)/g" \
	    -e "s/{{CM_NAME}}/job$*/g" \
	> $@

export TEST_JOB
job%-tfapply.yaml: backend_bucket
	@echo "$${TEST_JOB}" | \
	sed -e "s/{{KIND}}/TerraformApply/g" \
	    -e "s/{{NAME}}/job$*/g" \
	    -e "s/{{BACKEND_BUCKET}}/$(BACKEND_BUCKET)/g" \
	    -e "s/{{BACKEND_PREFIX}}/terraform/g" \
	    -e "s/{{CREDS_NAME}}/$(CREDENTIALS_SECRET_NAME)/g" \
	    -e "s/{{CREDS_KEY}}/$(CREDENTIALS_SECRET_KEY)/g" \
	    -e "s/{{CM_NAME}}/job$*/g" \
	> $@

export TEST_JOB
job%-tfdestroy.yaml: backend_bucket
	@echo "$${TEST_JOB}" | \
	sed -e "s/{{KIND}}/TerraformDestroy/g" \
	    -e "s/{{NAME}}/job$*/g" \
	    -e "s/{{BACKEND_BUCKET}}/$(BACKEND_BUCKET)/g" \
	    -e "s/{{BACKEND_PREFIX}}/terraform/g" \
	    -e "s/{{CREDS_NAME}}/$(CREDENTIALS_SECRET_NAME)/g" \
	    -e "s/{{CREDS_KEY}}/$(CREDENTIALS_SECRET_KEY)/g" \
	    -e "s/{{CM_NAME}}/job$*/g" \
	> $@

test-artifacts: $(TEST_ARTIFACTS)

test: $(TEST_PLAN_ARTIFACTS)
	-@for f in $^; do kubectl apply -f $$f; done

test-plan: $(TEST_PLAN_ARTIFACTS)
	-@for f in $^; do kubectl apply -f $$f; done

test-apply: $(TEST_APPLY_ARTIFACTS)
	-@for f in $^; do kubectl apply -f $$f; done

test-destroy: $(TEST_DESTROY_ARTIFACTS)
	-@for f in $^; do kubectl apply -f $$f; done

test-stop: $(TEST_ARTIFACTS)
	-@for f in $^; do kubectl delete -f $$f; done

test-clean: $(TEST_ARTIFACTS)
	rm -f $^