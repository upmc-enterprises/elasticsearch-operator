# Installing

```
$ helm repo add es-operator https://raw.githubusercontent.com/upmc-enterprises/elasticsearch-operator/master/charts/
$ helm install --name=elasticsearch es-operator/elasticsearch --set kibana.enabled=True --set cerebro.enabled=True --set zones="{eu-west-1a,eu-west-1b}" --namespace logging 
$ helm install --name elasticsearch-operator es-operator/elasticsearch-operator --set rbac.enabled=True --namespace logging 
```

# Bumping version

1. Increase the version in the Chart.yaml file
2. Package the chart
```
$ cd charts
$ helm package elasticsearch elasticsearch-operator
```
3. Recreate the index.yaml
```
$ helm repo index --merge index.yaml .
```
4. Add the generated package to git
```
$git add elasticsearch-0.1.1.tgz
```