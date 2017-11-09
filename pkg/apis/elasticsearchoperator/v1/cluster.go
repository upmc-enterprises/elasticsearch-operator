/*
Copyright (c) 2017, UPMC Enterprises
All rights reserved.
Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:
    * Redistributions of source code must retain the above copyright
      notice, this list of conditions and the following disclaimer.
    * Redistributions in binary form must reproduce the above copyright
      notice, this list of conditions and the following disclaimer in the
      documentation and/or other materials provided with the distribution.
    * Neither the name UPMC Enterprises nor the
      names of its contributors may be used to endorse or promote products
      derived from this software without specific prior written permission.
THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL UPMC ENTERPRISES BE LIABLE FOR ANY
DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ElasticsearchCluster defines the cluster
type ElasticsearchCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Type              string      `json:"type"`
	Spec              ClusterSpec `json:"spec"`
	Status            CRDStatus   `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ElasticsearchClusterList represents a list of ES Clusters
type ElasticsearchClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElasticsearchCluster `json:"items"`
}

type CRDStatus struct {
	State   CRDState `json:"state,omitempty"`
	Message string   `json:"message,omitempty"`
}

type CRDState string

// ClusterSpec defines cluster options
type ClusterSpec struct {
	// ClientNodeSize defines how many client nodes to have in cluster
	ClientNodeReplicas int32 `json:"client-node-replicas"`

	// MasterNodeSize defines how many client nodes to have in cluster
	MasterNodeReplicas int `json:"master-node-replicas"`

	// DataNodeSize defines how many client nodes to have in cluster
	DataNodeReplicas int `json:"data-node-replicas"`

	// NodeSelector specifies a map of key-value pairs. For the pod to be eligible
	// to run on a node, the node must have each of the indicated key-value pairs as
	// labels.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Zones specifies a map of key-value pairs. Defines which zones
	// to deploy persistent volumes for data nodes
	Zones []string `json:"zones,omitempty"`

	// DataDiskSize specifies how large the persistent volume should be attached
	// to the data nodes in the ES cluster
	DataDiskSize string `json:"data-volume-size"`

	// DataDiskSize specifies the docker image to use (optional)
	ElasticSearchImage string `json:"elastic-search-image"`

	// Snapshot defines how snapshots are scheduled
	Snapshot Snapshot `json:"snapshot"`

	// Storage defines how volumes are provisioned
	Storage Storage `json:"storage"`

	// JavaOptions defines args passed to elastic nodes
	JavaOptions string `json:"java-options"`

	// ImagePullSecrets defines credentials to pull image from private repository (optional)
	ImagePullSecrets []ImagePullSecrets `json:"image-pull-secrets"`

	// Resources defines memory / cpu constraints
	Resources Resources `json:"resources"`

	// Instrumentation defines metrics for the cluster
	Instrumentation Instrumentation `json:"instrumentation"`

	// Specify how the container binds to network ports
	NetworkHost string `json:"network-host"`

	//NodePort
	NodePort int32 `json:"nodePort"`

	// Kibana
	Kibana Kibana `json:"kibana"`

	//Cerebro
	Cerebro Cerebro `json:"cerebro"`

	Scheduler Scheduler
}

// ImagePullSecrets defines credentials to pull image from private repository
type ImagePullSecrets struct {
	// Name defines the name of the secret file that will be used
	Name string `json:"name"`
}

// Snapshot defines all params to create / store snapshots
type Snapshot struct {
	// Enabled determines if snapshots are enabled
	SchedulerEnabled bool `json:"scheduler-enabled"`

	// BucketName defines the AWS S3 bucket to store snapshots
	BucketName string `json:"bucket-name"`

	// CronSchedule defines how to run the snapshots
	// SEE: https://godoc.org/github.com/robfig/cron
	CronSchedule string `json:"cron-schedule"`

	// Authentication defines credentials for snapshot requests
	Authentication Authentication `json:"authentication"`
}

// Authentication defines credentials for snapshot requests
type Authentication struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

// Storage defines how dynamic volumes are created
// https://kubernetes.io/docs/user-guide/persistent-volumes/
type Storage struct {
	// StorageType is the type of storage to create
	StorageType string `json:"type"`

	// StorageClassProvisoner is the storage provisioner type
	StorageClassProvisoner string `json:"storage-class-provisioner"`

	// StorageClass to use
	StorageClass string `json:"storage-class"`

	// Volume Reclaim Policy on Persistent Volumes
	VolumeReclaimPolicy string `json:"volume-reclaim-policy"`
}

// Resources defines CPU / Memory restrictions on pods
type Resources struct {
	Requests MemoryCPU `json:"requests"`
	Limits   MemoryCPU `json:"limits"`
}

// MemoryCPU defines memory cpu options
type MemoryCPU struct {
	// Memory defines max amount of memory
	Memory string `json:"memory"`

	// CPU defines max amount of CPU
	CPU string `json:"cpu"`
}

// Instrumentation handles all metrics for the cluster
type Instrumentation struct {
	StatsdHost string `json:"statsd-host"`
}

// Kibana properties if wanting operator to deploy for user
type Kibana struct {
	// Defines the image to use for deploying kibana
	Image string `json:"image"`
}

// Cerebro properties if wanting operator to deploy for user
type Cerebro struct {
	// Defines the image to use for deploying Cerebro
	Image         string `json:"image"`
	Configuration string `json:"configuration"`
}

// Scheduler stores info about how to snapshot the cluster
type Scheduler struct {
	S3bucketName string
	CronSchedule string
	Enabled      bool
	Auth         SchedulerAuthentication
	ElasticURL   string
	Namespace    string
	ClusterName  string
}

// SchedulerAuthentication stores credentials used to authenticate against snapshot endpoint
type SchedulerAuthentication struct {
	UserName string
	Password string
}
