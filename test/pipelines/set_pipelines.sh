#!/usr/bin/env bash
READLINK=readlink
[[ "$(uname)" =~ Darwin ]] && READLINK=greadlink
BASE_DIR=$($READLINK -f $(dirname $0))

fly -t tf set-pipeline -p terraform-operator -c ${BASE_DIR}/ci.yaml -l ${BASE_DIR}/values.yaml

fly -t tf expose-pipeline -p terraform-operator