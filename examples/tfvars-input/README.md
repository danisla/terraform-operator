# Terraform Operator Output Variables Example

[![button](http://gstatic.com/cloudssh/images/open-btn.png)](https://console.cloud.google.com/cloudshell/open?git_repo=https://github.com/danisla/terraform-operator&working_dir=examples/tfvars-input&page=shell&tutorial=README.md)

Example showing how to reference a Terraform output variables from another TerraformApply resorce.

## Change to the example directory

```
[[ `basename $PWD` != tfvars-input ]] && cd examples/tfvars-input
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

2. Create new secret name `tf-provider-google` with `GOOGLE_CREDENTIALS` and `GOOGLE_PROJECT`:

```
kubectl create secret generic tf-provider-google \
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

## Create ConfigMaps for the terraform configs

1. Create ConfigMap for Google Compute Network:

```
kubectl create configmap example-tf-network --from-file=network.tf
```

2. Create ConfigMap for Google Compute Managed Instance Group:

```
kubectl create configmap example-tf-mig --from-file=mig.tf --from-file=gceme.sh.tpl
```

## Create the terraform aplpy file for the network

1. Create the `example-network-tfapply.yaml` file:

```
BACKEND_BUCKET="$(gcloud config get-value project)-terraform-operator"
cat > example-network-tfapply.yaml <<EOF
apiVersion: ctl.isla.solutions/v1
kind: TerraformApply
metadata:
  name: example-network
spec:
  backendBucket: ${BACKEND_BUCKET}
  backendPrefix: terraform
  providerConfig:
    google:
      secretName: tf-provider-google
  sources:
  - configMap:
      name: example-tf-network
      trigger: true
  tfvars:
    region: us-central1
    network_name: example-tfvars-input
EOF
cat example-network-tfapply.yaml
```

## Create the terraform apply file for the MIG

1. Create the `example-mig-tfapply.yaml` file:

```
BACKEND_BUCKET="$(gcloud config get-value project)-terraform-operator"
cat > example-mig-tfapply.yaml <<EOF
apiVersion: ctl.isla.solutions/v1
kind: TerraformApply
metadata:
  name: example-mig
spec:
  backendBucket: ${BACKEND_BUCKET}
  backendPrefix: terraform
  providerConfig:
    google:
      secretName: tf-provider-google
  sources:
  - configMap:
      name: example-tf-mig
      trigger: true
  tfinputs:
  - name: example-network
    varMap:
      network_region: region
      network_name: network_name
  tfvars:
    region: "us-central1"
    mig_size: "2"
EOF
cat example-mig-tfapply.yaml
```

> Notice how the `tfinputs.varMap` maps output variables from the network resource.

## Create the Terraform resources

1. Create the `TerraformApply` resources for the network and MIG:

```
kubectl apply -f example-network-tfapply.yaml
kubectl apply -f example-mig-tfapply.yaml
```

2. Get the output of the terraform apply operation for the network:

```
POD=$(kubectl get tfapply example-network -o jsonpath='{.status.podName}')
kubectl logs -f $POD
```

3. Get the output of the terraform apply operation for the MIG:

```
POD=$(kubectl get tfapply example-mig -o jsonpath='{.status.podName}')
kubectl logs -f $POD
```

## Create the example terraform destroy file

1. Create the `example-mig-tfdestroy.yaml` file from the contents of the `example-mig-tfapply.yaml` file:

```
sed 's/kind: TerraformApply/kind: TerraformDestroy/g' example-mig-tfapply.yaml > example-mig-tfdestroy.yaml
```

## Create the TerraformDestroy resource

1. Create the `TerraformDestroy` resource by applying the yaml spec:

```
kubectl apply -f example-mig-tfdestroy.yaml
```

2. Get the output of the terraform operation:

```
POD=$(kubectl get tfdestroy example-mig -o jsonpath='{.status.podName}')
kubectl logs -f $POD
```

3. Wait for the destroy operation to complete before proceeding.


## Create the example terraform destroy file

1. Create the `example-network-tfdestroy.yaml` file from the contents of the `example-network-tfapply.yaml` file:

```
sed 's/kind: TerraformApply/kind: TerraformDestroy/g' example-network-tfapply.yaml > example-network-tfdestroy.yaml
```

## Create the TerraformDestroy resource

1. Create the `TerraformDestroy` resource by applying the yaml spec:

```
kubectl apply -f example-network-tfdestroy.yaml
```

2. Get the output of the terraform operation:

```
POD=$(kubectl get tfdestroy example-network -o jsonpath='{.status.podName}')
kubectl logs -f $POD
```

## Cleanup

1. Remove the `Terraform*` resources:

```
kubectl delete tfapply,tfdestroy example-mig
kubectl delete tfapply,tfdestroy example-network
```

2. Delete the GKE cluster:

```
gcloud container clusters delete tf-tutorial --zone us-central1-b
```