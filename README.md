# elasticsearch operator

### Create certs secret

```
kubectl create secret generic es-certs --from-file=node-keystore.jks --from-file=truststore.jks
```

