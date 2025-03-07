glob: lint main.go index.html
	@go build -o glob

.PHONY: lint
lint:
	@go fmt ./...

.PHONY: run
run:
	@DEBUG=1 go run ./...


# Install ko
ifeq (,$(shell which ko))
KO=$(GOBIN)/ko
else
KO=$(shell which ko)
endif
$(KO):
	go install github.com/google/ko

# Build image
.PHONY: start-kind
start-kind:
	kind create cluster

.PHONY: ko-build
build-ko:
	KO_DOCKER_REPO='ko.local/baarsgaard/glob' $(KO) build

export KIND_CLUSTER_NAME ?= kind
.PHONY: ko-build-kind
ko-build-kind:
	KO_DOCKER_REPO='kind.local/baarsgaard/glob'	$(KO) build
