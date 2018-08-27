#!/usr/bin/env bash

set -x
set -e
set -o pipefail

terraform version

cat > backend.tf <<EOF
terraform {
  backend "gcs" {
    bucket = "${BACKEND_BUCKET}"
    prefix = "${BACKEND_PREFIX}"
  }
}
EOF

terraform init -upgrade=true
terraform workspace select ${WORKSPACE} || terraform workspace new ${WORKSPACE}
terraform plan -input=false -out terraform.tfplan

# Write plan to configmap as binary blob.
function publishPlan() {
    local module=$1
    local JSON=""
    case $module in
    root)
        JSON=$(terraform output -json | jq -r -c '.')
        ;;
    *)
        JSON=$(terraform output -module $module -json | jq -r -c '.')
    esac

    if [[ -n ${JOB_NAME+x} ]]; then
        echo "INFO: Updating job with outputs from $module module."
        PATCH=$(echo "{}" | jq -r -c --arg data "${JSON}" '[{op: "add", path: "/metadata/annotations/terraform-output", value: $data}]')
        kubectl patch job "${JOB_NAME}" --type json -p="${PATCH}"
    else
      echo "ERROR: Missing JOB_NAME env var, should have been provided from downward API."
      return 1
    fi
}

if [[ -n ${OUTPUT_MODULE+x} ]]; then
    echo "INFO: Publishing outputs from module: $OUTPUT_MODULE"
    publishOutputs $OUTPUT_MODULE
else
    echo "WARN: No outputs requested in OUTPUT_MODULE env var."
fi