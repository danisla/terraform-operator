FROM golang:1.10-alpine AS build
RUN apk add --update ca-certificates bash curl git
RUN curl https://raw.githubusercontent.com/golang/dep/v0.5.0/install.sh | sh

COPY . /go/src/github.com/danisla/terraform-operator/
WORKDIR /go/src/github.com/danisla/terraform-operator/cmd/terraform-operator
RUN dep ensure && go install

FROM alpine:3.7
RUN apk add --update ca-certificates bash curl
RUN curl -sfL -o /usr/local/bin/kustomize https://github.com/kubernetes-sigs/kustomize/releases/download/v1.0.6/kustomize_1.0.6_linux_amd64 && chmod +x /usr/local/bin/kustomize
COPY config /config/
COPY --from=build /go/bin/terraform-operator /usr/bin/
CMD ["/usr/bin/terraform-operator"]