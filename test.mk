SHELL := /bin/bash

ALL_TESTS := Test.*

GOTEST_ARGS := -v -parallel 8 -cpu 8
TEST_ARGS :=
DELETE_ARG := -delete=true

test: $(ALL_TESTS)

test-keep: $(addprefix Keep,$(ALL_TESTS))

test-list:
	cd test && gotest -list Test*

test-project:
	$(eval PROJECT := $(shell gcloud config get-value project 2>/dev/null))

gotest:
	$(eval GOTEST := $(shell command -v gotest || go get -u github.com/rakyll/gotest))

Test%: gotest test-project
	@export GOOGLE_PROJECT=$(PROJECT) && cd test && gotest -run 'Test$*$$' $(GOTEST_ARGS) -args $(DELETE_ARG) $(TEST_ARGS)

Keep%: gotest test-project
	@export GOOGLE_PROJECT=$(PROJECT) && cd test && gotest -run '$*$$' $(GOTEST_ARGS) -args -delete=false $(TEST_ARGS)
