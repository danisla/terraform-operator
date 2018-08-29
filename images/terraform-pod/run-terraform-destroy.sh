#!/usr/bin/env bash

set -x
set -e
set -o pipefail

mkdir -p ${PWD}/.terraform

terraform version

cat > backend.tf <<EOF
terraform {
  backend "gcs" {
    bucket = "${BACKEND_BUCKET}"
    prefix = "${BACKEND_PREFIX}"
  }
}
EOF

tree

terraform init -upgrade=true
terraform workspace select ${WORKSPACE} || terraform workspace new ${WORKSPACE}
terraform destroy -input=false -lock=false -auto-approve