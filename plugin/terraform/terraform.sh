#!/usr/bin/env bash

export TERRAFORM_OPERATOR_CONFIG="${HOME}/.terraform-operator-plugin.env"
export TERRAFORM_OPERATOR_GOOGLE_SECRET="tf-provider-google"

declare -a GOOGLE_CREDS
export GOOGLE_CREDS

function configure() {
    project=$(gcloud config get-value project 2>/dev/null)
    if [[ -z "${project}" ]]; then
        echo "ERROR: gcloud project not set. Run: 'gcloud config set project YOUR_PROJECT_ID'"
        return 1
    fi

    local defaultBucket="${project}-terraform-operator"

    local defaultSAKey="${HOME}/.terraform-operator-sa-key-${project}.json"

    bucket=""
    while [[ -z "${bucket}" ]]; do
        echo "GCS bucket for remote state and bundle uploads:"
        read -p "  Enter bucket name or ENTER for default: ${defaultBucket}: " input >&2

        if [[ -z "${input}" ]]; then
            bucket="${defaultBucket}"
        else
            bucket="${input}"
        fi
    done

    printf "\nINFO: Using GCS bucket: ${bucket}\n" >&2

    secretName=""
    # Check for existing secret, if found, prompt to use that.
    if checkSecret ${TERRAFORM_OPERATOR_GOOGLE_SECRET}; then
        while [[ ! "${input,,}" =~ [yn] ]]; do
            echo "INFO: secret ${TERRAFORM_OPERATOR_GOOGLE_SECRET} exists, configured for:"
            printf "\tservice account: ${GOOGLE_CREDS[0]}\n\tproject: ${GOOGLE_CREDS[1]}\n" >&2
            read -p  "  Use secret for Terraform Operator? (Y/n): " input >&2
            if [[ -z "${input}" || "${input,,}" == "y" ]]; then
                secretName=${TERRAFORM_OPERATOR_GOOGLE_SECRET}
                break
            fi
        done
    fi

    sa_key=""
    input=""
    confirm=""
    while [[ -z "${secretName}" && -z "${sa_key}" ]]; do
        echo "Project editor service account Key:"
        read -p "  Enter path to service account JSON file, or ENTER to generate (${defaultSAKey}): " input >&2

        if [[ -n "${input}" ]]; then
            if [[ ! -e "${input}" ]]; then
                echo "ERROR: key file not found: $input" >&2
                continue
            fi
            sa_key="${input}"
        else
            if [[ -e "${defaultSAKey}" ]]; then
                confirm=""
                while [[ ! "${confirm,,}" =~ [yn] ]]; do
                    read -p "WARN: ${defaultSAKey} already exists, use existing? (y/n): " confirm >&2
                done
                if [[ "${confirm}" == "y" ]]; then
                    sa_key="${defaultSAKey}"
                    break
                else
                    continue
                fi
            fi
            # Generate new key and add IAM binding
            createEditorSAKey "${defaultSAKey}"

            sa_key="${defaultSAKey}"
        fi
    done

    # Create secret
    if [[ -z "${secretName}" ]]; then
        createGoogleProviderSecret "${TERRAFORM_OPERATOR_GOOGLE_SECRET}" "${sa_key}" "${project}"
    fi

    # Creat the bucket
    makeBucket "${bucket}"

    # Generate the config
    cat > "${TERRAFORM_OPERATOR_CONFIG}" <<EOF
BACKEND_BUCKET=${bucket}
BACKEND_PREFIX=terraform
IMAGE_PULL_POLICY=IfNotPresent
GOOGLE_PROVIDER_SECRET=${TERRAFORM_OPERATOR_GOOGLE_SECRET}
EOF

    echo "INFO: Generated config file: ${TERRAFORM_OPERATOR_CONFIG}" >&2

    echo "INFO: Installing metacontroller" >&2
    installMetacontroller

    echo "INFO: Installing terraform-operator" >&2
    installTerraformOperator

    echo "INFO: Terraform plugin configured" >&2

    printf "\nCreate kterraform alias:\n"

    printf "\n\talias kterraform='kubectl plugin terraform'\n\n"

    usage
}

function makeBucket() {
    local name=$1
    gsutil mb "gs://$name"
}

function checkSecret() {
    local name=$1
    local namespace=${KUBECTL_PLUGINS_CURRENT_NAMESPACE:-"default"}
    local EMAIL=$(kubectl -n ${namespace} get secret ${name} -o jsonpath='{.data.GOOGLE_CREDENTIALS}' | base64 --decode | jq -r .client_email)
    local PROJECT=$(kubectl -n ${namespace} get secret ${name} -o jsonpath='{.data.GOOGLE_PROJECT}' | base64 --decode)

    GOOGLE_CREDS[0]="${EMAIL}"
    GOOGLE_CREDS[1]="${PROJECT}"

    [[ -n "${EMAIL}" && -n "${PROJECT}" ]]
}

function createGoogleProviderSecret() {
    local name=$1
    local sa_key=$2
    local project=$3

    namespace=${KUBECTL_PLUGINS_CURRENT_NAMESPACE:-"default"}

    kubectl -n ${namespace} create secret generic ${name} --from-literal=GOOGLE_PROJECT=${project} --from-file=GOOGLE_CREDENTIALS=${sa_key}
}

function createEditorSAKey() {
    local SA_KEY=$1

    local TS=$(date +%s)
    local SA_NAME=terraform-operator-${TS}
    local PROJECT=$(gcloud config get-value project 2>/dev/null)
    local SA_EMAIL=${SA_NAME}@${PROJECT}.iam.gserviceaccount.com
    gcloud iam service-accounts create ${SA_NAME} \
        --display-name "Terraform Operator admin account" && \
    gcloud iam service-accounts keys create ${SA_KEY} \
        --iam-account ${SA_EMAIL} && \
    gcloud projects add-iam-policy-binding ${PROJECT} \
        --role roles/editor --member serviceAccount:${SA_EMAIL} > /dev/null
}

function installMetacontroller() {
    local version=${1:-"master"}
    kubectl create clusterrolebinding ${USER}-cluster-admin-binding --clusterrole=cluster-admin --user=$(gcloud config get-value account)

    kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/${version}/manifests/metacontroller-rbac.yaml
    kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/${version}/manifests/metacontroller.yaml
}

function installTerraformOperator() {
    local version=${1:-"master"}
    kubectl apply -f https://raw.githubusercontent.com/danisla/terraform-operator/${version}/manifests/terraform-operator-rbac.yaml
    kubectl apply -f https://raw.githubusercontent.com/danisla/terraform-operator/${version}/manifests/terraform-operator.yaml
}

function makeBundleName() {
    . "${TERRAFORM_OPERATOR_CONFIG}"

    local crc=${1:0:4}
    local bundle="kterraform-${crc}.tgz"
    echo "gs://${BACKEND_BUCKET}/sources/${bundle}"
}

function makeBundle() {
    local tmpdir=$(mktemp -d)
    local src=$1
    local bundle="${tmpdir}/bundle.tgz"
    local cksums="${tmpdir}/checksums"
    cd ${src}
    tar \
        --exclude .git \
        --exclude .terraform \
        --exclude .terraform.d \
        -zcf "${bundle}" .

    echo "${bundle}"
}

function makeChecksum() {
    src=$1
    CKSUM=$(tar zxf "${src}" -O | cksum)
    echo "${CKSUM:0:4}"
}

function uploadTarball() {
    . "${TERRAFORM_OPERATOR_CONFIG}"

    local bundle=$(makeBundle $1)
    local crc=$(makeChecksum "${bundle}")
    local dest="$(makeBundleName "${crc}")"

    # gsutil cp "${bundle}" "${dest}" >/dev/null 2>&1
    gsutil cp "${bundle}" "${dest}" >/dev/null 2>&1
    echo "${dest}"
}

function makeTF() {
    local type=$1
    local src=$2
    local bundle=$3
    local jobName=$4
    local tfplan=$5
    
    local namespace=${KUBECTL_PLUGINS_CURRENT_NAMESPACE:-"default"}
    local timestamp=$(date +%s)

    [[ -n "${tfplan}" ]] && tfplan="  tfplan: ${tfplan}"

    . "${TERRAFORM_OPERATOR_CONFIG}"

    cat <<EOF | kubectl apply -f - >&2
apiVersion: ctl.isla.solutions/v1
kind: ${type}
metadata:
  name: ${jobName}
  namespace: ${namespace}
  labels:
    kterraform-timestamp: "${timestamp}"
spec:
  image: gcr.io/cloud-solutions-group/terraform-pod:latest
  imagePullPolicy: ${IMAGE_PULL_POLICY:-IfNotPresent}
  backendBucket: ${BACKEND_BUCKET}
  backendPrefix: ${BACKEND_PREFIX}
  providerConfig:
    google:
      secretName: ${GOOGLE_PROVIDER_SECRET}
${tfplan}
  sources:
  - gcs: ${bundle}
EOF
    echo "${jobName}"
}

function tailLogs() {
    local type=$1
    local jobName=$2
    local namespace=${KUBECTL_PLUGINS_CURRENT_NAMESPACE:-"default"}

    kubectl -n ${namespace} wait pod -l terraform-parent=${jobName} --for condition=Ready >/dev/null

    local POD=$(kubectl -n ${namespace} get ${type} ${jobName} -o jsonpath='{.status.podName}')
    kubectl -n ${namespace} logs -f $POD
}

function terraformPlan() {
    local src=$1
    local type="TerraformPlan"
    local bundle=$(uploadTarball "${src}")
    local jobName=$(basename "$bundle" .tgz)
    local jobName=$(makeTF "${type}" "${src}" "${bundle}" "${jobName}")
    tailLogs "${type}" "${jobName}"
}

function terraformApply() {
    local src=$1
    local type="TerraformApply"
    local namespace=${KUBECTL_PLUGINS_CURRENT_NAMESPACE:-"default"}

    # Get logs from existing TerraformPlan
    local bundle=$(uploadTarball "${src}")
    local jobName=$(basename "$bundle" .tgz)
    local POD=$(kubectl -n ${namespace} get TerraformPlan ${jobName} -o jsonpath='{.status.podName}')
    tfplan=""
    if [[ -n "${POD}" ]]; then
        kubectl -n ${namespace} logs $POD
        tfplan=${jobName}
    else
        echo "WARN: No terraform plan run previously." >&2
    fi
    

    while [[ -z "${input}" ]]; do
        printf "\n  "
        read -p "Run terraform apply? (yes/no): " input
    done

    if [[ "${input,,}" == "yes" ]]; then
        local jobName=$(makeTF "${type}" "${src}" "${bundle}" "${jobName}" "${tfplan}")
        tailLogs "${type}" "${jobName}"
    else
        echo "Aborting"
    fi
}

function terraformDestroy() {
    local src=$1
    local type="TerraformDestroy"

    while [[ -z "${input}" ]]; do
        read -p "Run terraform destroy? (yes/no): " input
    done

    if [[ "${input,,}" == "yes" ]]; then
        local bundle=$(uploadTarball "${src}")
        local jobName=$(basename "$bundle" .tgz)
        local jobName=$(makeTF "${type}" "${src}" "${bundle}" "${jobName}")
        tailLogs "${type}" "${jobName}"
    else
        echo "Aborting"
    fi
}

function usage() {
    echo "USAGE: kubectl plugin terraform <plan|apply|destroy>" >&2
}

function getUserCwd() {
    lsof -p $PPID | awk '/cwd/{ print $9 }'
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    case $1 in
        configure)
            configure
            ;;
        plan)
            terraformPlan $(getUserCwd)
            ;;
        apply)
            terraformApply $(getUserCwd)
            ;;
        destroy)
            terraformDestroy $(getUserCwd)
            ;;
        "")
            usage
            ;;
        *)
            echo "ERROR: Unknown operation: $1"
            usage
            ;;
    esac
fi