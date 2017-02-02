# Makefile for the Docker image upmcenterprises/elasticsearch-controller
# MAINTAINER: Steve Sloka <slokas@upmc.edu>

.PHONY: all build container push clean test

TAG ?= 0.0.0
PREFIX ?= upmcenterprises

all: container

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -o _output/bin/elasticsearch-operator --ldflags '-w' ./cmd/operator/main.go

container: build
	docker build -t $(PREFIX)/elasticsearch-controller:$(TAG) .

push:
	docker push $(PREFIX)/elasticsearch-controller:$(TAG)

clean:
	rm -f elasticsearch-controller

test: clean
	godep go test -v --vmodule=*=4