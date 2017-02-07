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

package controller

import (
	"github.com/Sirupsen/logrus"
	"github.com/upmc-enterprises/elasticsearch-operator/pkg/snapshot"
	"github.com/upmc-enterprises/elasticsearch-operator/pkg/spec"
	"github.com/upmc-enterprises/elasticsearch-operator/util/k8sutil"
)

// Config defines properties of the controller
type Config struct {
	Namespace string
	k8sclient *k8sutil.K8sutil
}

// Controller object
type Controller struct {
	Config
	clusters map[string]*spec.ElasticSearchCluster
}

// New up a Controller
func New(name, ns string, k8sclient *k8sutil.K8sutil) (*Controller, error) {

	c := &Controller{
		Config: Config{
			Namespace: ns,
			k8sclient: k8sclient,
		},
		clusters: make(map[string]*spec.ElasticSearchCluster),
	}

	return c, nil
}

// Run gets the party started
func (c *Controller) Run() error {

	// Init TPR
	err := c.init()

	if err != nil {
		logrus.Error("Error in init(): ", err)
	}

	// Get existing clusters
	currentClusters, err := c.k8sclient.GetElasticSearchClusters()

	if err != nil {
		logrus.Error("Could not get list of clusters: ", err)
		return err
	}

	for _, cluster := range currentClusters {
		logrus.Infof("Found cluster: %s", cluster.Metadata["name"])

		c.clusters[cluster.Metadata["name"]] = &spec.ElasticSearchCluster{
			Spec: spec.ClusterSpec{
				ClientNodeReplicas: cluster.Spec.ClientNodeReplicas,
				MasterNodeReplicas: cluster.Spec.MasterNodeReplicas,
				DataNodeReplicas:   cluster.Spec.DataNodeReplicas,
				Zones:              cluster.Spec.Zones,
				DataDiskSize:       cluster.Spec.DataDiskSize,
				ElasticSearchImage: cluster.Spec.ElasticSearchImage,
				Snapshot: spec.Snapshot{
					SchedulerEnabled: cluster.Spec.Snapshot.SchedulerEnabled,
					BucketName:       cluster.Spec.Snapshot.BucketName,
					CronSchedule:     cluster.Spec.Snapshot.CronSchedule,
				},
			},
		}
		logrus.Infof("Found %d zones ", len(cluster.Spec.Zones))

		// Setup CronSchedule
		s, _ := snapshot.New(
			cluster.Spec.Snapshot.BucketName,
			cluster.Spec.Snapshot.CronSchedule,
			cluster.Spec.Snapshot.SchedulerEnabled)
		s.Run()
	}

	logrus.Infof("Found %d existing clusters ", len(c.clusters))

	return nil
}

func (c *Controller) init() error {
	err := c.k8sclient.CreateKubernetesThirdPartyResource()
	if err != nil {
		return err
	}

	return nil
}
