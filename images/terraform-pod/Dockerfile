FROM google/cloud-sdk:alpine

RUN apk add --update \
    jq \
    curl \
    autossh \
    openssl \
    ca-certificates \
    tree

COPY * /

RUN curl -sfSL https://storage.googleapis.com/kubernetes-release/release/v1.11.0/bin/linux/amd64/kubectl > /usr/bin/kubectl && chmod +x /usr/bin/kubectl && \
    gcloud components install beta -q && \
    mkdir -p /opt/terraform && \
    chmod +x /terraform-install.sh && \
    /terraform-install.sh && \
    cp ${HOME}/bin/terraform /usr/bin/terraform && \
    chmod +x /run-terraform*.sh && \
    chmod +x /get-gcs-tarball.sh

WORKDIR /opt/terraform

ENTRYPOINT ["/run-terraform-plan.sh"]