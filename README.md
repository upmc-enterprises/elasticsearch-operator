# elasticsearch operator

[![Build Status](https://travis-ci.org/upmc-enterprises/elasticsearch-operator.svg?branch=master)](https://travis-ci.org/upmc-enterprises/elasticsearch-operator)

The ElasticSearch operator is designed to manage one or more elastic search clusters. Included in the project (initially) is the ability to create the Elastic cluster, deploy the `data nodes` across zones in your Kubernetes cluster, and snapshot indexes to AWS S3. 

# Requirements

## Kubernetes

The operator was built and tested on a 1.5.X Kubernetes cluster and is the only version supported. This is because it is utilizing [`StatefulSets`](https://kubernetes.io/docs/concepts/abstractions/controllers/statefulsets/) which are not available in previous versions of Kubernetes. 

## Cloud

The operator was also _currently_ designed to leverage [Amazon AWS S3](https://aws.amazon.com/s3/) for snapshots / restores to the  elastic cluster. The goal of this prooject is to extend to support additional clouds and scenarios to make it fully featured. 

# Usage

The operator is built using the controllers + third party resource model. Once the controller is deployed to your cluster, it will automatically create the ThirdPartyResource. Next create a Kubernetes object type `elasticsearchCluster` to deploy the elastic cluster based upon the TPR. 

## ThirdPartyResource

Following parameters are available to customize the elastic cluster:

- client-node-replicas: Number of client node replicas
- master-node-replicas: Number of client node replicas
- data-node-replicas: Number of data node replicas
- zones: Define which zones to deploy data nodes to for high availability (_Note: Zones are evenly distributed based upon number of data-node-replicas defined_)
- data-volume-size: Size of persistent volume to attach to data nodes

## Certs secret

The default image used adds TLS to the Elastic cluster. This secret is used to volume in certs to the cluster dynamically. Provided in this repo are sample certificates, but you are encouraged to change them for your cluster. 

```bash
$ kubectl create secret generic es-certs --from-file=./certs/node-keystore.jks --from-file=./certs/truststore.jks
```
## Base image

The base image used is `upmcenterprises/docker-elasticsearch-kubernetes:5.1.1` which can be overriden by addeding to the custom cluster you create _(See: [ThirdPartyResource](#thirdpartyresource) above)_. 

_NOTE: If no image is specified, the default noted previously is used._

# Deploy Operator

To deploy the operator simply deploy to your cluster:

```bash
$ kubectl create -f example/controller.yaml
```

# Create Example ElasticSearch Cluster

```bash
$ kubectl create -f example/example-es-cluster.json
```
_NOTE: Creating a custom cluster requires the creation of a ThirdPartyResource. This happens automatically after the controller is created._

# Resize ElasticSearch Cluster

`kubectl apply` doesn't work for TPR for the moment. See [kubernetes/#29542](https://github.com/kubernetes/kubernetes/issues/29542). As a workaround, we use curl to resize the cluster.

First update the default example configuration, then send a `PUT` request to the Kubernetes API server. _NOTE: The API is acesssed the API service in this example via [kubectl proxy](https://kubernetes.io/docs/user-guide/kubectl/kubectl_proxy/)._ 

```bash
curl -H 'Content-Type: application/json' -X PUT --data @example/example-es-cluster.json http://127.0.0.1:9005/apis/enterprises.upmc.com/v1/namespaces/default/elasticsearchclusters/example-es-cluster
```

# About
Built by UPMC Enterprises in Pittsburgh, PA. http://enterprises.upmc.com/