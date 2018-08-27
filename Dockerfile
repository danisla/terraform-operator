FROM golang:1.10-alpine AS build
RUN apk add --update ca-certificates bash curl git
RUN curl https://raw.githubusercontent.com/golang/dep/v0.5.0/install.sh | sh

COPY . /go/src/github.com/danisla/terraform-operator/
WORKDIR /go/src/github.com/danisla/terraform-operator/cmd/terraform-operator
RUN dep ensure && go install

FROM alpine:3.7
RUN apk add --update ca-certificates bash curl
COPY --from=build /go/bin/terraform-operator /usr/bin/
CMD ["/usr/bin/terraform-operator"]