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

package spec

// ElasticSearchCluster defines the cluster
type ElasticSearchCluster struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   map[string]string `json:"metadata"`
	Spec       ClusterSpec       `json:"spec"`
}

// ClusterSpec defines cluster options
type ClusterSpec struct {
	// ClientNodeSize defines how many client nodes to have in cluster
	ClientNodeReplicas int32 `json:"client-node-replicas"`

	// MasterNodeSize defines how many client nodes to have in cluster
	MasterNodeReplicas int32 `json:"master-node-replicas"`

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
}
