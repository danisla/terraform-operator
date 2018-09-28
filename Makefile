SHELL := /bin/bash

git-tag = $(shell COMMIT=$$(git rev-parse HEAD) && echo $${COMMIT:0:5})

TAG := $(call git-tag)
TFJSON_TAG := $(TAG)
NAMESPACE := default

SA_JSON := sa_key.json

all: image

tag:
	@echo $(TAG)

project:
	$(eval PROJECT := $(shell gcloud config get-value project 2>/dev/null))

image:
	gcloud -q builds submit --config cloudbuild.yaml --substitutions _TFJSON_TAG=$(TFJSON_TAG),_TAG=$(TAG),_PROJECT=cloud-solutions-group --machine-type=n1-highcpu-32

terraform-pod-image:
	cd images/terraform-pod && \
	  gcloud builds submit -q --tag gcr.io/cloud-solutions-group/terraform-pod:$(TAG) --machine-type=n1-highcpu-32

tfjson-service-image:
	cd images/tfjson-service && \
	  gcloud builds submit -q --tag gcr.io/cloud-solutions-group/tfjson-service:$(TAG) --machine-type=n1-highcpu-32

ci-image:
	cd images/ci-image && \
	  gcloud builds submit -q --tag gcr.io/cloud-solutions-group/terraform-operator-ci:$(TAG) --machine-type=n1-highcpu-32

install-metacontroller:
	-kubectl create clusterrolebinding $(USER)-cluster-admin-binding --clusterrole=cluster-admin --user=$(shell gcloud config get-value account)

	kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller-rbac.yaml
	kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller.yaml

$(SA_JSON): project
ifeq (,$(wildcard $(SA_JSON)))
	echo "Creating service account key: $@" && \
	  TS=$$(date +%s) && \
	  SA_NAME=terraform-operator-$${TS} && \
	  SA_EMAIL=$${SA_NAME}@$(PROJECT).iam.gserviceaccount.com && \
	  gcloud iam service-accounts create $${SA_NAME} \
	    --display-name "Terraform Operator admin account" && \
	  gcloud iam service-accounts keys create $@ \
	    --iam-account $${SA_EMAIL} && \
	  gcloud projects add-iam-policy-binding $(PROJECT) \
	    --role roles/editor --member serviceAccount:$${SA_EMAIL} > /dev/null
endif

secrets: $(SA_JSON) project
	kubectl -n metacontroller create --save-config=true secret generic tf-operator-sa-key --from-file=sa_key.json=$(SA_JSON) --dry-run -o yaml | kubectl apply -f -
	kubectl -n $(NAMESPACE) create --save-config=true secret generic tf-provider-google --from-literal=GOOGLE_PROJECT=$(PROJECT) --from-file=GOOGLE_CREDENTIALS=$(SA_JSON) --dry-run -o yaml | kubectl apply -f -

lpods:
	kubectl -n metacontroller get pods
	
metalogs:
	kubectl -n metacontroller logs --tail=200 -f metacontroller-0

rollpod:
	kubectl -n metacontroller delete pod -l app=terraform-operator

podlogs:
	POD=$(shell kubectl get pod -n metacontroller -l app=terraform-operator -o name | tail -1) && \
	kubectl -n metacontroller wait $$POD --for=condition=Ready && \
	  kubectl -n metacontroller logs -f $$POD

include kaniko.mk
include test.mk
