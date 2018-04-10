# Makefile for the Docker image upmcenterprises/elasticsearch-operator
# MAINTAINER: Steve Sloka <slokas@upmc.edu>

.PHONY: all build container push clean test

TAG ?= 0.0.11
PREFIX ?= upmcenterprises
pkgs = $(shell go list ./... | grep -v /vendor/ | grep -v /test/)
# go source files, ignore vendor directory
SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

ifeq ($(GOBIN),)
GOBIN :=${GOPATH}/bin
endif

all: container

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -o _output/bin/elasticsearch-operator --ldflags '-w' ./cmd/operator/main.go

container: build
	docker build -t $(PREFIX)/elasticsearch-operator:$(TAG) .

push:
	docker push $(PREFIX)/elasticsearch-operator:$(TAG)

clean:
	rm -f elasticsearch-operator

$(GOBIN)/deepcopy-gen:
	go get -u -v -d k8s.io/code-generator/cmd/deepcopy-gen
	cd $(GOPATH)/src/k8s.io/code-generator; git checkout release-1.9
	go install k8s.io/code-generator/cmd/deepcopy-gen
	
deepcopy-gen: $(GOBIN)/deepcopy-gen

generate-deepcopy: deepcopy-gen
	$(GOBIN)/deepcopy-gen -i github.com/upmc-enterprises/elasticsearch-operator/pkg/apis/elasticsearchoperator/v1 --go-header-file="$(GOPATH)/src/github.com/upmc-enterprises/elasticsearch-operator/.header" -v=4 --logtostderr --bounding-dirs "github.com/upmc-enterprises/elasticsearch-operator/pkg/client" --output-file-base zz_generated.deepcopy

format:
	go fmt $(pkgs)

check:
	@go tool vet ${SRC}

helm-package:
	helm package charts/{elasticsearch,elasticsearch-operator} -d charts
	helm repo index --merge charts/index.yaml charts

test: clean
	go test $$(go list ./... | grep -v /vendor/)