TAG = dev

all: install

install:
	go install

image:
	docker build -t gcr.io/cloud-solutions-group/terraform-operator:$(TAG) .

push: image
	docker push gcr.io/cloud-solutions-group/terraform-operator:$(TAG)

install-metacontroller:
	helm install --name metacontroller --namespace metacontroller charts/metacontroller

metalogs:
	kubectl -n metacontroller logs --tail=200 -f metacontroller-metacontroller-0

include test.mk