/*
Copyright (c) 2016, UPMC Enterprises
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

package processor

import (
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/upmc-enterprises/elasticsearch-operator/pkg/spec"
	"github.com/upmc-enterprises/elasticsearch-operator/util/k8sutil"
)

// processorLock ensures that reconciliation and event processing does
// not happen at the same time.
var processorLock = &sync.Mutex{}

// Processor object
type Processor struct {
	k8sclient *k8sutil.K8sutil
	baseImage string
}

// New creates new instance of Processor
func New(kclient *k8sutil.K8sutil, baseImage string) (*Processor, error) {
	p := &Processor{
		k8sclient: kclient,
		baseImage: baseImage,
	}

	return p, nil
}

// WatchElasticSearchClusterEvents watches for changes to tpr elasticsearch events
func (p *Processor) WatchElasticSearchClusterEvents(done chan struct{}, wg *sync.WaitGroup) {
	events, watchErrs := p.k8sclient.MonitorElasticSearchEvents()
	go func() {
		for {
			select {
			case event := <-events:
				err := p.processElasticSearchClusterEvent(event)
				if err != nil {
					logrus.Errorln(err)
				}
			case err := <-watchErrs:
				logrus.Errorln(err)
			case <-done:
				wg.Done()
				logrus.Println("Stopped elasticsearch event watcher.")
				return
			}
		}
	}()
}

func (p *Processor) processElasticSearchClusterEvent(c k8sutil.ElasticSearchEvent) error {
	processorLock.Lock()
	defer processorLock.Unlock()
	switch {
	case c.Type == "ADDED":
		return p.processElasticSearchCluster(c.Object)
	case c.Type == "DELETED":
		return p.deleteElasticSearchCluster(c.Object)
	}
	return nil
}

func (p *Processor) processElasticSearchCluster(c k8sutil.ElasticSearchCluster) error {
	logrus.Println("--------> ES Event!")

	cluster := &spec.ElasticSearchCluster{
		Spec: spec.ClusterSpec{
			ClientNodeSize: c.Spec.ClientNodeSize,
			MasterNodeSize: c.Spec.MasterNodeSize,
			DataNodeSize:   c.Spec.DataNodeSize,
			Zones:          c.Spec.Zones,
		},
	}

	p.k8sclient.CreateDiscoveryService()
	p.k8sclient.CreateDataService()
	p.k8sclient.CreateClientService()

	// Create Storage Classes
	for _, sc := range cluster.Spec.Zones {
		p.k8sclient.CreateStorageClass(sc)
	}

	p.k8sclient.CreateClientMasterDeployment("client", p.baseImage, &cluster.Spec.ClientNodeSize)
	p.k8sclient.CreateClientMasterDeployment("master", p.baseImage, &cluster.Spec.MasterNodeSize)

	var dataReplicas = cluster.Spec.DataNodeSize
	var zonesCount = len(cluster.Spec.Zones) - 1
	// const totalZones = 3
	var zoneDistribution [3]int32

	// for dataReplicas != 0 {
	// 	zoneDistribution[zonesCount] = zoneDistribution[zonesCount] + 1
	// 	if zonesCount == 0 {
	// 		zonesCount = len(cluster.Spec.Zones) - 1
	// 	} else {
	// 		zonesCount = zonesCount - 1
	// 	}
	// 	dataReplicas = dataReplicas - 1
	// }

	index := 0
	for i := 0; i < dataReplicas; i++ {
		if index > zonesCount {
			index = 0
		}

		zoneDistribution[index]++
		index++
	}

	for index, count := range zoneDistribution {
		p.k8sclient.CreateDataNodeDeployment(&count, p.baseImage, cluster.Spec.Zones[index])
	}

	return nil
}

func (p *Processor) deleteElasticSearchCluster(c k8sutil.ElasticSearchCluster) error {
	logrus.Println("--------> ES Deleted!!")
	return nil
}
