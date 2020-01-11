# Makefile for the Docker image upmcenterprises/elasticsearch-operator
# MAINTAINER: Steve Sloka <steve@stevesloka.com>

.PHONY: all build container push clean test

TAG ?= 0.4.0
PREFIX ?= upmcenterprises

all: container

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -o _output/bin/elasticsearch-operator --ldflags '-w' ./cmd/operator/main.go

container: build
	docker build -t $(PREFIX)/elasticsearch-operator:$(TAG) .

dep:
	go mod vendor

push:
	docker push $(PREFIX)/elasticsearch-operator:$(TAG)

clean:
	rm -f elasticsearch-operator

format:
	go fmt ./pkg/... ./cmd/...

check:
	go vet ./pkg/... ./cmd/...

helm-package:
	helm package charts/{elasticsearch,elasticsearch-operator} -d charts
	helm repo index --merge charts/index.yaml charts

test: clean
	go test ./pkg/...

devpreq:
	mkdir -p /tmp/certs/config && mkdir -p /tmp/certs/certs
	go get -u github.com/cloudflare/cfssl/cmd/cfssl
	go get -u github.com/cloudflare/cfssl/cmd/cfssljson
