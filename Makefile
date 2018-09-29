TAG = latest

all: image

image:
	gcloud builds submit -q --config cloudbuild.yaml --project cloud-solutions-group --substitutions=TAG_NAME=$(TAG) --machine-type=n1-highcpu-32

terraform-pod-image:
	cd images/terraform-pod && \
	  gcloud builds submit -q --project cloud-solutions-group --tag gcr.io/cloud-solutions-group/terraform-pod:$(TAG) --machine-type=n1-highcpu-32

tfjson-service-image:
	cd images/tfjson-service && \
	  gcloud builds submit -q --project cloud-solutions-group --tag gcr.io/cloud-solutions-group/tfjson-service:$(TAG) --machine-type=n1-highcpu-32

install-metacontroller:
	-kubectl create clusterrolebinding $(USER)-cluster-admin-binding --clusterrole=cluster-admin --user=$(shell gcloud config get-value account)

	kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller-rbac.yaml
	kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller.yaml

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
