#!/usr/bin/env bash

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
terraform plan -input=false -out terraform.tfplan

# Write plan to configmap as binary blob.
function publishPlan() {
  local tfplan=$1

  SA_JSON=${PWD}/.terraform/service_account.json
  cat > ${SA_JSON} <<EOF
$GOOGLE_CREDENTIALS
EOF

  gcloud auth activate-service-account --key-file=${SA_JSON}
  gcloud config set project $PROJECT_ID

  destPath="gs://${BACKEND_BUCKET}/${BACKEND_PREFIX}/${NAMESPACE}-${POD_NAME}.tfplan"

  echo "INFO: Uploading $tfplan to $destPath"
  # Copy plan to backend bucket
  gsutil cp "${tfplan}" "${destPath}"

  if [[ -n ${POD_NAME+x} ]]; then
      echo "INFO: Updating pod annotation with path to plan file."
      PATCH=$(echo "{}" | jq -r -c --arg data "${destPath}" '[{op: "add", path: "/metadata/annotations/terraform-plan", value: $data}]')
      kubectl patch pod "${POD_NAME}" --type json -p="${PATCH}"
  else
    echo "ERROR: Missing POD_NAME env var, should have been provided from downward API."
    return 1
  fi
}

publishPlan $(readlink -f terraform.tfplan)