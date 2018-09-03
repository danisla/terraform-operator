TAG = latest

KANIKO_SA_KEY := ${HOME}/.kaniko-sa-key.json

all: image

image:
	docker build -t gcr.io/cloud-solutions-group/terraform-operator:$(TAG) .

push: image
	docker push gcr.io/cloud-solutions-group/terraform-operator:$(TAG)

install-metacontroller:
	-kubectl create clusterrolebinding $(USER)-cluster-admin-binding --clusterrole=cluster-admin --user=$(shell gcloud config get-value account)

	kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller-rbac.yaml
	kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller.yaml

kaniko-secret: $(KANIKO_SA_KEY)
	kubectl create secret generic kaniko-secret --from-file=kaniko-secret=$(KANIKO_SA_KEY)

metalogs:
	kubectl -n metacontroller logs --tail=200 -f metacontroller-0

include test.mk
