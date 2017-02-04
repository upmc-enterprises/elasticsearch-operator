FROM alpine
MAINTAINER  Steve Sloka <slokas@upmc.edu>

RUN apk add --update ca-certificates && \
  rm -rf /var/cache/apk/*

ADD _output/bin/elasticsearch-operator /usr/local/bin

CMD ["/bin/sh", "-c", "/usr/local/bin/elasticsearch-operator"]
