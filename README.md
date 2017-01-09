# elasticsearch operator

### Create certs secret

```
kubectl create secret generic es-certs --from-file=./certs/node-keystore.jks --from-file=./certs/truststore.jks
```

