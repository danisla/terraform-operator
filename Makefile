TAG = dev

KANIKO_SA_KEY := ${HOME}/.kaniko-sa-key.json

all: install

install:
	go install

image:
	docker build -t gcr.io/cloud-solutions-group/terraform-operator:$(TAG) .

push: image
	docker push gcr.io/cloud-solutions-group/terraform-operator:$(TAG)

install-metacontroller:
	helm install --name metacontroller --namespace metacontroller charts/metacontroller

kaniko-secret: $(KANIKO_SA_KEY)
	kubectl create secret generic kaniko-secret --from-file=kaniko-secret=$(KANIKO_SA_KEY)

metalogs:
	kubectl -n metacontroller logs --tail=200 -f metacontroller-metacontroller-0

include test.mk