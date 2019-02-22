# Makefile for the Docker image upmcenterprises/elasticsearch-operator
# MAINTAINER: Steve Sloka <slokas@upmc.edu>

.PHONY: all build container push clean test

TAG ?= 0.2.0
PREFIX ?= upmcenterprises
pkgs = $(shell go list ./... | grep -v /vendor/ | grep -v /test/)
# go source files, ignore vendor directory
SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

all: container

.PHONY: build
build: $(GOFILES)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -o _output/bin/elasticsearch-operator --ldflags '-w' ./cmd/operator/main.go

.PHONY: container
container: build
	docker build -t $(PREFIX)/elasticsearch-operator:$(TAG) .

.PHONY: push
push:
	docker push $(PREFIX)/elasticsearch-operator:$(TAG)

.PHONY: clean
clean:
	rm -f _output/$(PREFIX)-elasticsearch-operator-$(TAG).tar
	rm -f elasticsearch-operator

.PHONY: format
format:
	go fmt $(pkgs)

.PHONY: format
check:
	@go tool vet ${SRC}

.PHONY: helm-package
helm-package:
	helm package charts/{elasticsearch,elasticsearch-operator} -d charts
	helm repo index --merge charts/index.yaml charts

.PHONY: test
test: clean
	go test $$(go list ./... | grep -v /vendor/)

.PHONY: e2e
e2e:
	go test ./test/... -count=1 -tags=integration -ginkgo.v -v -ginkgo.progress
