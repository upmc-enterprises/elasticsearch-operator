FROM alpine

ADD _output/bin/elasticsearch-operator /usr/local/bin

CMD ["/bin/sh", "-c", "/usr/local/bin/elasticsearch-operator"]
