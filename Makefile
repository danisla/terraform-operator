TAG = latest

all: image

image:
	docker build -t gcr.io/cloud-solutions-group/terraform-operator:$(TAG) .

push: image
	docker push gcr.io/cloud-solutions-group/terraform-operator:$(TAG)

install-metacontroller:
	-kubectl create clusterrolebinding $(USER)-cluster-admin-binding --clusterrole=cluster-admin --user=$(shell gcloud config get-value account)

	kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller-rbac.yaml
	kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller.yaml

lpods:
	kubectl -n metacontroller get pods
	
metalogs:
	kubectl -n metacontroller logs --tail=200 -f metacontroller-0

include kaniko.mk
include test.mk
