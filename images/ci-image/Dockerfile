FROM golang:1.10-alpine

ENV K8S_VERSION v1.11.3

RUN \
  apk add --update --no-progress \
    ca-certificates \
    bash \
    curl \
    git \
    python \
    make && \
  curl https://raw.githubusercontent.com/golang/dep/v0.5.0/install.sh | sh && \
  curl -sfSL https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/linux/amd64/kubectl > /usr/local/bin/kubectl && \
  chmod +x /usr/local/bin/kubectl && \
  go get -u github.com/rakyll/gotest && \
  go get -u github.com/Masterminds/sprig && \
  go get -u k8s.io/apimachinery/pkg/apis/meta/v1 && \
  curl -sfSL https://github.com/kubernetes-sigs/kustomize/releases/download/v1.0.8/kustomize_1.0.8_linux_amd64 > /usr/local/bin/kustomize && \
  chmod +x /usr/local/bin/kustomize && \
  curl https://sdk.cloud.google.com | bash

ENV PATH ${PATH}:/root/google-cloud-sdk/bin/