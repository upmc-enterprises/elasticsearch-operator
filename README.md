# elasticsearch operator

[![Build Status](https://travis-ci.org/upmc-enterprises/elasticsearch-operator.svg?branch=master)](https://travis-ci.org/upmc-enterprises/elasticsearch-operator)

### Create certs secret

```
kubectl create secret generic es-certs --from-file=./certs/node-keystore.jks --from-file=./certs/truststore.jks
```

