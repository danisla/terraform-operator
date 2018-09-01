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

    local name=$1
    local type=$2
    local crc=${3:0:4}
    local ts=$(date +%s)
    local bundle="kterraform-${name}-${type}-${crc}-${ts}.tgz"
    echo "gs://${BACKEND_BUCKET}/sources/${bundle}"
}

function makeBundle() {
    local tmpdir=$(mktemp -d)
    local src=$1
    local bundle="${tmpdir}/bundle.tgz"
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

    local name=$1
    local type=$2
    local src=$3
    local bundle=$(makeBundle "${src}")
    local crc=$(makeChecksum "${bundle}")
    local dest="$(makeBundleName "${name}" "${type}" "${crc}")"

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
}

function dateToUnixTime() {
    local srcDate=$1
    local platform=$(uname)
    local datecmd="date"
    case ${platform,,} in
    darwin)
        date -jf "%FT%TZ" "${srcDate}" +%s
        ;;
    *)
        date -d "${srcDate}" +%s
    esac
}

function tailLogs() {
    local type=$1
    local jobName=$2
    local startTimeMin=${3:-$(date +%FT%TZ)}
    local namespace=${KUBECTL_PLUGINS_CURRENT_NAMESPACE:-"default"}

    startTimeMinUnix=$(dateToUnixTime $startTimeMin)
    podStart=$startTimeMinUnix
    count=0
    until [[ $podStart -gt $startTimeMinUnix || count -ge 10 ]]; do
        startedAt=$(kubectl -n ${namespace} get ${type} ${jobName} -o jsonpath='{.status.startedAt}' 2>/dev/null)
        if [[ -n "${startedAt}" ]]; then
            podStart=$(dateToUnixTime $startedAt)
        fi
        sleep 2
        ((count=count+1))
    done

    local POD=$(kubectl -n ${namespace} get ${type} ${jobName} -o jsonpath='{.status.podName}')
    kubectl -n ${namespace} logs -f $POD
}

function invalidName() {
    [[ ! "$1" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]
}

function makeJobName() {
    echo "kterraform-${1}"
}

function terraformPlan() {
    local src=$1
    local name=$2
    local type="TerraformPlan"
    local bundle=$(uploadTarball "${name}" "tfplan" "${src}")
    local jobName=$(makeJobName "${name}")
    local startTime=$(date +%FT%TZ)
    makeTF "${type}" "${src}" "${bundle}" "${jobName}"
    tailLogs "${type}" "${jobName}" "${startTime}"
}

function terraformApply() {
    local src=$1
    local name=$2
    local jobName=$(makeJobName "${name}")
    local type="TerraformApply"
    local namespace=${KUBECTL_PLUGINS_CURRENT_NAMESPACE:-"default"}
    local bundle=$(uploadTarball "${name}" "tfapply" "${src}")

    # Run terraform plan
    (cd "${src}" && kubectl plugin terraform plan)
    
    while [[ -z "${input}" ]]; do
        printf "\n  "
        read -p "Run terraform apply? (yes/no): " input
    done

    if [[ "${input,,}" == "yes" ]]; then
        local startTime=$(date +%FT%TZ)
        makeTF "${type}" "${src}" "${bundle}" "${jobName}" "${jobName}"
        tailLogs "${type}" "${jobName}" "${startTime}"
    else
        echo "Aborting"
    fi
}

function terraformDestroy() {
    local src=$1
    local name=$2
    local jobName=$(makeJobName "${name}")
    local type="TerraformDestroy"

    while [[ -z "${input}" ]]; do
        read -p "Run terraform destroy? (yes/no): " input
    done

    if [[ "${input,,}" == "yes" ]]; then
        local bundle=$(uploadTarball "${name}" "tfdestroy" "${src}")
        local startTime=$(date +%FT%TZ)
        makeTF "${type}" "${src}" "${bundle}" "${jobName}"
        tailLogs "${type}" "${jobName}" "${startTime}"
    else
        echo "Aborting"
    fi
}

function usage() {
    echo "USAGE: kubectl plugin terraform <configure|plan [<name|default>]|apply [<name>|default]|destroy [<name>|default]>" >&2
}

function getUserCwd() {
    lsof -p $PPID | awk '/cwd/{ print $9 }'
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    action=$1
    name=${2:-"default"}
    invalidName "${name}" && echo "ERROR: '$name' is an invalid name, must be a DNS-1123 name in the form of: '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'" >&2 && exit 1

    case "$action" in
        configure)
            configure
            ;;
        plan)
            terraformPlan $(getUserCwd) "${name}"
            ;;
        apply)
            terraformApply $(getUserCwd) "${name}"
            ;;
        destroy)
            terraformDestroy $(getUserCwd) "${name}"
            ;;
        "")
            usage
            ;;
        *)
            echo "ERROR: Unknown operation: $action"
            usage
            ;;
    esac
fi