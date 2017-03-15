# elasticsearch operator

[![Build Status](https://travis-ci.org/upmc-enterprises/elasticsearch-operator.svg?branch=master)](https://travis-ci.org/upmc-enterprises/elasticsearch-operator)

The ElasticSearch operator is designed to manage one or more elastic search clusters. Included in the project (initially) is the ability to create the Elastic cluster, deploy the `data nodes` across zones in your Kubernetes cluster, and snapshot indexes to AWS S3. 

# Requirements

## Kubernetes

The operator was built and tested on a 1.5.X Kubernetes cluster and is the only version supported currently. This is because it is utilizing [`StatefulSets`](https://kubernetes.io/docs/concepts/abstractions/controllers/statefulsets/) which are not available in previous versions of Kubernetes. 

## Cloud

The operator was also _currently_ designed to leverage [Amazon AWS S3](https://aws.amazon.com/s3/) for snapshot / restore to the  elastic cluster. The goal of this project is to extend to support additional clouds and scenarios to make it fully featured. 

By swapping out the storage types, this can be used in GKE, but snapshots won't work at the moment. 


# Demo
Watch a demo here:<br>
[![Elasticsearch Operator Demo](http://img.youtube.com/vi/3HnV7NfgP6A/0.jpg)](http://www.youtube.com/watch?v=3HnV7NfgP6A)<br>
[https://www.youtube.com/watch?v=3HnV7NfgP6A](https://www.youtube.com/watch?v=3HnV7NfgP6A)

# Usage

The operator is built using the controller + third party resource model. Once the controller is deployed to your cluster, it will automatically create the ThirdPartyResource. Next create a Kubernetes object type `elasticsearchCluster` to deploy the elastic cluster based upon the TPR. 

## ThirdPartyResource

Following parameters are available to customize the elastic cluster:

- client-node-replicas: Number of client node replicas
- master-node-replicas: Number of client node replicas
- data-node-replicas: Number of data node replicas
- zones: Define which zones to deploy data nodes to for high availability (_Note: Zones are evenly distributed based upon number of data-node-replicas defined_)
- data-volume-size: Size of persistent volume to attach to data nodes
- [snapshot](https://www.elastic.co/guide/en/elasticsearch/reference/current/modules-snapshots.html)
  - scheduler-enabled: If the cron scheduler should be running to enable snapshotting
  - bucket-name: Name of S3 bucket to dump snaptshots
  - cron-schedule: Cron task definition for intervals to do snapshots
- [storage](https://kubernetes.io/docs/user-guide/persistent-volumes/)
  - type: Defines the type of storage to provision based upon cloud (e.g. `gp2`)
  - storage-class-provisioner: Defines which type of provisioner to use (e.g. `kubernetes.io/aws-ebs`)


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
$ kubectl create -f https://raw.githubusercontent.com/upmc-enterprises/elasticsearch-operator/master/example/controller.yaml
```

# Create Example ElasticSearch Cluster

```bash
$ kubectl create -f https://raw.githubusercontent.com/upmc-enterprises/elasticsearch-operator/master/example/example-es-cluster.json
```
_NOTE: Creating a custom cluster requires the creation of a ThirdPartyResource. This happens automatically after the controller is created._

# Create Example ElasticSearch Cluster (Minikube)

To run the operator on minikube, this sample file is setup to do that. It sets lower Java memory contraints as well as uses the default storage class in Minikube which writes to hostPath.

```bash
$ kubectl create -f https://raw.githubusercontent.com/upmc-enterprises/elasticsearch-operator/master/example/example-es-cluster-minikube.json
```
_NOTE: Creating a custom cluster requires the creation of a ThirdPartyResource. This happens automatically after the controller is created._

# Resize ElasticSearch Cluster

`kubectl apply` doesn't work for TPR for the moment. See [kubernetes/#29542](https://github.com/kubernetes/kubernetes/issues/29542). As a workaround, we use curl to resize the cluster.

First update the default example configuration, then send a `PUT` request to the Kubernetes API server. _NOTE: The API is acesssed the API service in this example via [kubectl proxy](https://kubernetes.io/docs/user-guide/kubectl/kubectl_proxy/)._ 

```bash
curl -H 'Content-Type: application/json' -X PUT --data @example/example-es-cluster.json http://127.0.0.1:9005/apis/enterprises.upmc.com/v1/namespaces/default/elasticsearchclusters/example-es-cluster
```

# Snapshot

Elasticsearch can snapshot it's indexes for easy backup / recovery of the cluster. Currently there's an integration to Amazon S3 as the backup respository for snapshots. The `upmcenterprises` docker images include the [S3 Plugin](https://www.elastic.co/guide/en/elasticsearch/plugins/current/repository-s3.html) which enables this feature in AWS. 

## Schedule

Snapshots can be scheduled via a Cron syntax by defining the cron schedule in your elastic cluster. See: [https://godoc.org/github.com/robfig/cron](https://godoc.org/github.com/robfig/cron)

_NOTE: Be sure to enable the scheduler as well by setting `scheduler-enabled=true`_ 

## AWS Setup

To enable the snapshots create a bucket in S3, then apply the following IAM permissions to your EC2 instances replacing `{!YOUR_BUCKET!}` with the correct bucket name. 

```
{
    "Statement": [
        {
            "Action": [
                "s3:ListBucket",
                "s3:GetBucketLocation",
                "s3:ListBucketMultipartUploads",
                "s3:ListBucketVersions"
            ],
            "Effect": "Allow",
            "Resource": [
                "arn:aws:s3:::{!YOUR_BUCKET!}"
            ]
        },
        {
            "Action": [
                "s3:GetObject",
                "s3:PutObject",
                "s3:DeleteObject",
                "s3:AbortMultipartUpload",
                "s3:ListMultipartUploadParts"
            ],
            "Effect": "Allow",
            "Resource": [
                "arn:aws:s3:::{!YOUR_BUCKET!}/*"
            ]
        }
    ],
    "Version": "2012-10-17"
}
```

# Access Cluster

Once deployed and all pods are running, the cluster can be accessed internally via https://elasticsearch:9200/ or https://${ELASTICSEARCH_SERVICE_HOST}:9200/

![alt text](docs/images/running-cluster.png "Running Cluster")

# About
Built by UPMC Enterprises in Pittsburgh, PA. http://enterprises.upmc.com/
