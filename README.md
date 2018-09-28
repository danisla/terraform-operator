# Terraform Operator

This is not an official Google product.

## Intro

Implementation of a [CompositeController metacontroller](https://github.com/GoogleCloudPlatform/metacontroller) to operate the Terraform lifecycle.

This controller utilizes the following major components:
- [Custom Resource Definitions (CRD)](https://kubernetes.io/docs/concepts/api-extension/custom-resources/): Used to represent the new custom resources.
- [metacontroller](https://github.com/GoogleCloudPlatform/metacontroller): Implements the CompositeController interface for the Custom Resource Definition.

## Prerequisites

1. Create GKE cluster:

```
ZONE=us-central1-b
CLUSTER_VERSION=$(gcloud beta container get-server-config --zone ${ZONE} --format='value(validMasterVersions[0])')

gcloud container clusters create dev \
  --cluster-version ${CLUSTER_VERSION} \
  --machine-type n1-standard-4 \
  --num-nodes 3 \
  --scopes=cloud-platform \
  --zone ${ZONE}
```

## Install metacontroller

1. Install metacontroller:

```
kubectl create clusterrolebinding ${USER}-cluster-admin-binding --clusterrole=cluster-admin --user=$(gcloud config get-value account)

kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller.yaml
```

## Install the operator

1. Create secrets containing google credentials used by the operator and terraform pod named `tf-operator-sa-key` and `tf-provider-google` respectively:

```
make -e NAMESPACE=default secrets
```

> Note that the `tf-provider-google` secret will be created in the default namespace. Change this if you want to run terraform from another namespace.

2. Modify the `terraform-operator-rbac.yaml` file if you want to run terraform from a namespace other than `default`.

3. Deploy the manifest files for the operator:

```
kubectl apply -f manifests/terraform-operator-rbac.yaml
kubectl apply -f manifests/terraform-operator.yaml
```