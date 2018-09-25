SHELL := /bin/bash

ALL_TESTS := Test.*

GOTEST_ARGS := -v -parallel 8 -cpu 8
DELETE_ARG := -delete=true

test: $(ALL_TESTS)

test-keep: $(addprefix Keep,$(ALL_TESTS))

project:
	$(eval PROJECT := $(shell gcloud config get-value project 2>/dev/null))

gotest:
	$(eval GOTEST := $(shell command -v gotest || go get -u github.com/rakyll/gotest))

Test%: gotest project
	@export GOOGLE_PROJECT=$(PROJECT) && cd test && gotest -run 'Test$*$$' $(GOTEST_ARGS) -args $(DELETE_ARG)

Keep%: gotest project
	@export GOOGLE_PROJECT=$(PROJECT) && cd test && gotest -run '$*$$' -args -delete=false