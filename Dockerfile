FROM openjdk:8-jdk-alpine
MAINTAINER  Steve Sloka <steve@stevesloka.com>

RUN apk add --update ca-certificates openssl && \
  rm -rf /var/cache/apk/*

RUN mkdir -p /tmp/certs/config
RUN mkdir -p /tmp/certs/certs
ADD https://pkg.cfssl.org/R1.2/cfssl_linux-amd64 /usr/local/bin/cfssl
ADD https://pkg.cfssl.org/R1.2/cfssljson_linux-amd64 /usr/local/bin/cfssljson
RUN chmod +x /usr/local/bin/cfssl
RUN chmod +x /usr/local/bin/cfssljson

ADD _output/bin/elasticsearch-operator /usr/local/bin

CMD ["/bin/sh", "-c", "/usr/local/bin/elasticsearch-operator"]