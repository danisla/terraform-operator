FROM golang:1.10-alpine AS build
RUN apk add --update ca-certificates bash curl git
RUN curl https://raw.githubusercontent.com/golang/dep/v0.5.0/install.sh | sh

COPY . /go/src/github.com/danisla/tfjson-service/
WORKDIR /go/src/github.com/danisla/tfjson-service/cmd/tfjson-service
RUN dep ensure && go install

FROM google/cloud-sdk:alpine
RUN apk add --update ca-certificates bash curl
RUN curl -sfSL https://storage.googleapis.com/kubernetes-release/release/v1.11.0/bin/linux/amd64/kubectl > /usr/bin/kubectl && chmod +x /usr/bin/kubectl
COPY --from=build /go/bin/tfjson-service /usr/bin/
CMD ["/usr/bin/tfjson-service"]