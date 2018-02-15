# Installing

```
$ helm repo add es-operator https://raw.githubusercontent.com/upmc-enterprises/elasticsearch-operator/master/charts/
$ helm install --name elasticsearch-operator es-operator/elasticsearch-operator --set rbac.enabled=True --namespace logging 
$ helm install --name=elasticsearch es-operator/elasticsearch --set kibana.enabled=True --set cerebro.enabled=True --set zones="{eu-west-1a,eu-west-1b}" --namespace logging 
```

# Bumping version

1. Increase the version in the Chart.yaml file
2. Package the chart
```
$ make helm-packages
```
3. Add the generated package to git
```
$ git add charts/elasticsearch-*.tgz
```
4. Push the changes