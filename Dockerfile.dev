# This is the same as Dockerfile, but skips `dep ensure`.
# It assumes you already ran that locally.
FROM golang:1.10-alpine AS build

COPY . /go/src/github.com/danisla/terraform-operator/
WORKDIR /go/src/github.com/danisla/terraform-operator/cmd/terraform-operator
RUN go install

FROM gcr.io/cloud-solutions-group/tfjson-service:latest AS tfjson

FROM google/cloud-sdk:alpine
RUN apk add --update ca-certificates bash curl
RUN curl -sfSL https://storage.googleapis.com/kubernetes-release/release/v1.11.0/bin/linux/amd64/kubectl > /usr/bin/kubectl && chmod +x /usr/bin/kubectl
COPY --from=build /go/bin/terraform-operator /usr/bin/
COPY --from=tfjson /usr/bin/tfjson-service /usr/bin/
CMD ["/usr/bin/terraform-operator"]