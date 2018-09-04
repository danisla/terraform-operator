#!/usr/bin/env bash

set -e
set -o pipefail

mkdir -p ${PWD}/.terraform

# Decode any *.b64 files
find . -maxdepth 1 -mindepth 1 -name "*.b64" -exec sh -c "base64 -d {} > \$(basename {} .b64)" \;

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