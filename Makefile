TAG = latest

all: image

image:
	gcloud builds submit --config cloudbuild.yaml --project cloud-solutions-group --substitutions=TAG_NAME=$(TAG)

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
