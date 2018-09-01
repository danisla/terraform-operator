# Terraform Operator Plugin Example

[![button](http://gstatic.com/cloudssh/images/open-btn.png)](https://console.cloud.google.com/cloudshell/open?git_repo=https://github.com/danisla/terraform-operator&working_dir=examples/plugin&page=shell&tutorial=README.md)

Example showing how to configure and use the Terraform Operator as a Kubectl Plugin.

## Change to the example directory

```
[[ `basename $PWD` != plugin ]] && cd examples/plugin
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

## Install the plugin

1. Clone the repo into the `~/.kube/plugin` directory:

```
(
    mkdir -p ${HOME}/.kube/plugin
    cd ${HOME}/.kube/plugin
    git clone https://github.com/danisla/terraform-operator.git
)
```

2. Verify that __terraform__ apears in the list of __Available Commands__`:

```
kubectl plugin
```

Expected output:

```
...
Available Commands:
  terraform     Plan, apply, and destroy terraform configs with terraform-operator
...
```

## Configure the plugin

1. Configure the plugin with your service account key and project info:

```
kubectl plugin terraform configure
```

2. Follow the prompts to configure the plugin.

Example prompt:

```
GCS bucket for remote state and bundle uploads:
  Enter bucket name or ENTER for default: YOUR_PROJECT_ID-terraform-operator:
```

Example prompt:

```
Project editor service account Key:
  Enter path to service account JSON file, or ENTER to generate (${HOME}/.terraform-operator-sa-key-YOUR_PROJECT_ID.json):
```

> When complete, the plugin will have installed the provider secret, metacontroller and terraform-operator to your cluster.

3. Create alias to plugin:

```
alias kterraform='kubectl plugin terraform'
```

## Run kterraform plan

1. Create terraform plan for example source:

```
kterraform plan
```

> You will see the output of `terraform plan` after the pod has been started, this may take a few seconds the first time.

Example output:

```
...
Plan: 7 to add, 0 to change, 0 to destroy.

------------------------------------------------------------------------

This plan was saved to: terraform.tfplan
...
```

> NOTE: the plan is saved to the GCS bucket configured when you ran `kubectl plugin terraform configure`

> NOTE: you don't have to run `kterraform plan` and can just run `kterraform apply` however you will not get to see the plan before applying it.

## Run kterraform apply

1. Apply the plan created in the previous section:

```
kterraform apply
```

> The plan will be run again and yhen you will be prompted to apply that plan.

Example prompt:

```
  Run terraform apply? (yes/no):
```

After entering __yes__ to the prompt, the terraform apply command will run and apply the plan.

## Run kterraform destroy

1. Destroy the resources created in the previous section:

```
kterraform destroy
```

## Cleanup

1. Delete the terraform-operator objects created:

```
kubectl delete tfplan,tfapply,tfdestroy kterraform-default
```

2. Delete the GKE cluster:

```
gcloud container clusters delete tf-tutorial --zone us-central1-b
```
