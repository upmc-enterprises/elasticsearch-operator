# Makefile for the Docker image upmcenterprises/elasticsearch-controller
# MAINTAINER: Steve Sloka <slokas@upmc.edu>

.PHONY: all build container push clean test

TAG ?= 0.0.3-1
PREFIX ?= innoq

all: container

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -o _output/bin/elasticsearch-operator --ldflags '-w' ./cmd/operator/main.go

container: build
	docker build -t $(PREFIX)/elasticsearch-operator:$(TAG) .

push:
	docker push $(PREFIX)/elasticsearch-operator:$(TAG)

clean:
	rm -f elasticsearch-operator

test: clean
	go test $$(go list ./... | grep -v /vendor/)