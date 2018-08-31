# Terraform Operator Basic Example

[![button](http://gstatic.com/cloudssh/images/open-btn.png)](https://console.cloud.google.com/cloudshell/open?git_repo=https://github.com/danisla/terraform-operator&working_dir=examples/basic&page=shell&tutorial=README.md)

Basic example showing how to apply a Terraform config using the operator.

## Change to the example directory

```
[[ `basename $PWD` != basic ]] && cd examples/basic
```

## Set up the environment

1. Set the project, replace `YOUR_PROJECT` with your project ID:

```
PROJECT=YOUR_PROJECT
```

```
gcloud config set project ${PROJECT}
```

2. Create GKE cluster:

```
ZONE=us-central1-b
CLUSTER_VERSION=$(gcloud beta container get-server-config --zone ${ZONE} --format='value(validMasterVersions[0])')

gcloud container clusters create tf-tutorial \
  --cluster-version ${CLUSTER_VERSION} \
  --machine-type n1-standard-4 \
  --num-nodes 3 \
  --scopes=cloud-platform \
  --zone ${ZONE}
```

## Install the terraform-operator controller

1. Install metacontroller:

```
kubectl create clusterrolebinding ${USER}-cluster-admin-binding --clusterrole=cluster-admin --user=$(gcloud config get-value account)

kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller.yaml
```

2. Install terraform-operator:

```
kubectl apply -f https://raw.githubusercontent.com/danisla/terraform-operator/master/manifests/terraform-operator-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/danisla/terraform-operator/master/manifests/terraform-operator.yaml
```

## Create the provider credentials secret

1. Create a service account and credentials JSON for terraform:

```
export SA_KEY=${HOME}/.terraform-sa-key.json
export PROJECT=$(gcloud config get-value project)
export SA_EMAIL=terraform@${PROJECT}.iam.gserviceaccount.com
[[ ! -e ${SA_KEY} ]] && \
gcloud iam service-accounts create terraform \
    --display-name "Terraform admin account" && \
gcloud iam service-accounts keys create ${SA_KEY} \
    --iam-account ${SA_EMAIL} && \
gcloud projects add-iam-policy-binding ${PROJECT} \
    --role roles/editor --member serviceAccount:${SA_EMAIL}
```

2. Create new secret name `tf-google` with `GOOGLE_CREDENTIALS` and `GOOGLE_PROJECT`:

```
kubectl create secret generic tf-google \
  --from-file=GOOGLE_CREDENTIALS=${SA_KEY} \
  --from-literal=GOOGLE_PROJECT=$(gcloud config get-value project)
```

> NOTE: This secret is referenced in each of your terraform operator specs.

## Create the backend bucket for remote state

1. Create the remote state backend bucket:

```
BACKEND_BUCKET="$(gcloud config get-value project)-terraform-operator"
gsutil mb gs://${BACKEND_BUCKET}
```

## Create the example terraform apply file

1. Create the `example-tfapply.yaml` file:

```
cat > example-tfapply.yaml <<'EOF'
apiVersion: ctl.isla.solutions/v1
kind: TerraformApply
metadata:
  name: example
spec:
  image: gcr.io/cloud-solutions-group/terraform-pod:latest
  imagePullPolicy: Always
  backendBucket: {{BACKEND_BUCKET}}
  backendPrefix: terraform
  providerConfig:
    google:
      secretName: tf-google
  sources:
  - embedded: |-
      variable "region" {}
      provider "google" {
        region = "${var.region}"
      }
      resource "google_compute_project_metadata_item" "default" {
        key = "tf-job-test"
        value = "tf-operator-test"
      }
      data "google_client_config" "current" {}
      output "project" {
        value = "${data.google_client_config.current.project}"
      }
      output "region" {
        value = "${var.region}"
      }
      output "metadata_key" {
        value = "${google_compute_project_metadata_item.default.key}"
      }
      output "metadata_value" {
        value = "${google_compute_project_metadata_item.default.value}"
      }
  tfvars:
    region: us-central1
EOF

BACKEND_BUCKET="$(gcloud config get-value project)-terraform-operator"
sed -i -e "s|{{BACKEND_BUCKET}}|${BACKEND_BUCKET}|g" example-tfapply.yaml
cat example-tfapply.yaml
```

## Create the TerraformApply resource

1. Create the `TerraformApply` resource by applying the yaml spec:

```
kubectl apply -f example-tfapply.yaml
```

2. Get the output of the terraform operation:

```
POD=$(kubectl get tfapply example -o jsonpath='{.status.podName}')
kubectl logs -f $POD
```

3. View the resource status with `kubectl describe`:

```
kubectl describe tfapply example
```

## Create the example terraform destroy file

1. Create the `example-tfdestroy.yaml` file from the contents of the `example-tfapply.yaml` file:

```
sed 's/kind: TerraformApply/kind: TerraformDestroy/g' example-tfapply.yaml > example-tfdestroy.yaml
```

## Create the TerraformDestroy resource

1. Create the `TerraformDestroy` resource by applying the yaml spec:

```
kubectl apply -f example-tfdestroy.yaml
```

2. Get the output of the terraform operation:

```
POD=$(kubectl get tfdestroy example -o jsonpath='{.status.podName}')
kubectl logs -f $POD
```

3. View the resource status with `kubectl describe`:

```
kubectl describe tfdestroy example
```

## Cleanup

1. Remove the `TerraformApply` and `TerraformDestroy` resources:

```
kubectl delete tfapply,tfdestroy example
```

2. Delete the GKE cluster:

```
gcloud container clusters delete tf-tutorial --zone us-central1-b
```