glob: lint main.go index.html
	@go build -o glob

.PHONY: lint
lint:
	@go fmt ./...

.PHONY: run
run:
	@DEBUG=1 go run ./...

export PORT ?= 3000
.PHONY: open
open:
	$(BROWSER) http://localhost:$(PORT)/index.html

.PHONY: go_mod
go_mod:
	@go mod download
	@go mod tidy

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

.PHONY: build-ko
build-ko:
	KO_DOCKER_REPO='ko.local/baarsgaard' $(KO) build --base-import-paths

export KIND_CLUSTER_NAME ?= kind
.PHONY: ko-build-kind
ko-build-kind:
	KO_DOCKER_REPO='kind.local/baarsgaard' $(KO) build --base-import-paths
