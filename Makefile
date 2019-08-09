# Makefile for the Docker image upmcenterprises/elasticsearch-operator
# MAINTAINER: Steve Sloka <steve@stevesloka.com>

.PHONY: all build container push clean test

PREFIX ?= upmcenterprises
TAG ?= 0.4.1
pkgs = $(shell go list ./... | grep -v /vendor/ | grep -v /test/)
# go source files, ignore vendor directory
SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

all: container

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -o _output/bin/elasticsearch-operator --ldflags '-w' ./cmd/operator/main.go

container: build
	docker build -t $(PREFIX)/elasticsearch-operator:$(TAG) .

push:
	docker push $(PREFIX)/elasticsearch-operator:$(TAG)

clean:
	rm -f elasticsearch-operator

format:
	go fmt $(pkgs)

check:
	@go tool vet ${SRC}

helm-package:
	helm package charts/{elasticsearch,elasticsearch-operator} -d charts
	helm repo index --merge charts/index.yaml charts

test: clean
	go test $$(go list ./... | grep -v /vendor/)

devpreq:
	mkdir -p /tmp/certs/config && mkdir -p /tmp/certs/certs
	go get -u github.com/cloudflare/cfssl/cmd/cfssl
	go get -u github.com/cloudflare/cfssl/cmd/cfssljson
