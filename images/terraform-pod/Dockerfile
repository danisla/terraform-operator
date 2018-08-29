FROM google/cloud-sdk:alpine

RUN apk add --update \
    jq \
    curl \
    autossh \
    openssl \
    ca-certificates

RUN mkdir -p /opt/terraform

# Install latest terraform
COPY terraform-install.sh /opt/terraform/terraform-install.sh
RUN chmod +x /opt/terraform/terraform-install.sh
RUN /opt/terraform/terraform-install.sh
RUN cp ${HOME}/bin/terraform /usr/local/bin/terraform

# Install kubectl
RUN curl -sfSL https://storage.googleapis.com/kubernetes-release/release/v1.11.0/bin/linux/amd64/kubectl > /usr/bin/kubectl && chmod +x /usr/bin/kubectl

COPY run-terraform-apply.sh /run-terraform-apply.sh
RUN chmod +x /run-terraform-apply.sh

COPY run-terraform-destroy.sh /run-terraform-destroy.sh
RUN chmod +x /run-terraform-destroy.sh

COPY run-terraform-plan.sh /run-terraform-plan.sh
RUN chmod +x /run-terraform-plan.sh

WORKDIR /opt/terraform

ENTRYPOINT ["/run-terraform-plan.sh"]