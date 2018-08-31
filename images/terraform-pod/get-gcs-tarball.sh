#!/usr/bin/env bash

set -x
set -e
set -o pipefail

mkdir -p ${PWD}/.terraform

if [[ -z ${GCS_TARBALLS+x} ]]; then
    echo "ERROR: GCS_TARBALLS env var not set"
    exit 1
fi

if [[ -z ${GOOGLE_CREDENTIALS+x} ]]; then
    echo "ERROR: GOOGLE_CREDENTIALS containing JSON key data env var not set"
    exit 1
fi

SA_JSON=${PWD}/.terraform/service_account.json
  cat > ${SA_JSON} <<EOF
$GOOGLE_CREDENTIALS
EOF

gcloud auth activate-service-account --key-file=${SA_JSON}
gcloud config set project $PROJECT_ID

IFS=',' read -ra tarballs <<< "${GCS_TARBALLS}"

for tb in ${tarballs[*]}; do
    echo "INFO: Fetching tarball: $tb"

    gsutil cp "${tb}" ./

    tar zxvf $(basename "${tb}")
done

tree

echo "INFO: Done"