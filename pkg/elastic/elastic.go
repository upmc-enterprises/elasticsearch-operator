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

package elastic

import "time"

var (
	caFile = "ca.pem"
)

// Interface abstracts out the implementation of elastic
type Interface interface {
	MonitorElasticClusterStatus(stopchan chan struct{}) (<-chan *StatusEvent, <-chan error)
}

// Client gives a health interface to elastic
type Client struct {
	elasticURL string
	HealthStatus
	checkInterval time.Duration
}

// HealthStatus represents the response from Elastic health endpoint
type HealthStatus struct {
	ActivePrimaryShards         int     `json:"active_primary_shards"`
	ActiveShards                int     `json:"active_shards"`
	ActibeShardsPercentAsNumber float32 `json:"active_shards_percent_as_number"`
	ClusterName                 string  `json:"cluster_name"`
	DelayedUnassignedShards     int     `json:"delayed_unassigned_shards"`
	InitializingShards          int     `json:"initializing_shards"`
	NumberDataNodes             int     `json:"number_of_data_nodes"`
	NumberInFlightFetch         int     `json:"number_of_in_flight_fetch"`
	NumberNodes                 int     `json:"number_of_nodes"`
	NumberPendingTasks          int     `json:"number_of_pending_tasks"`
	ReloactingShards            int     `json:"relocating_shards"`
	Status                      string  `json:"status"`
	TaskMaxWaitingInQueueMilis  int     `json:"task_max_waiting_in_queue_millis"`
	TimedOut                    bool    `json:"timed_out"`
	UnassignedShards            int     `json:"unassigned_shards"`
}

// New creates a new instance of k8sutil
func New() (*Client, error) {

	k := &Client{
		elasticURL:    "https://elasticsearch-example-es-cluster.default:9200",
		HealthStatus:  HealthStatus{},
		checkInterval: time.Second * 5,
	}
	return k, nil
}
