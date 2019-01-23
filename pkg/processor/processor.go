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
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOW CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
*/

package processor

import (
	"fmt"
	"sync"

	"github.com/upmc-enterprises/elasticsearch-operator/pkg/elasticsearchutil"

	"github.com/upmc-enterprises/elasticsearch-operator/pkg/snapshot"

	"github.com/Sirupsen/logrus"
	myspec "github.com/upmc-enterprises/elasticsearch-operator/pkg/apis/elasticsearchoperator/v1"
	"github.com/upmc-enterprises/elasticsearch-operator/pkg/k8sutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// processorLock ensures that reconciliation and event processing does
// not happen at the same time.
var (
	processorLock = &sync.Mutex{}
)

type Cluster struct {
	ESCluster *myspec.ElasticsearchCluster
	Scheduler *snapshot.Scheduler
}

// Processor object
type Processor struct {
	k8sclient *k8sutil.K8sutil
	baseImage string
	clusters  map[string]Cluster
}

// New creates new instance of Processor
func New(kclient *k8sutil.K8sutil, baseImage string) (*Processor, error) {
	p := &Processor{
		k8sclient: kclient,
		baseImage: baseImage,
		clusters:  make(map[string]Cluster),
	}

	return p, nil
}

// Run starts the processor
func (p *Processor) Run() error {

	p.refreshClusters()
	logrus.Infof("Found %d existing clusters ", len(p.clusters))

	return nil
}

// WatchElasticSearchClusterEvents watches for changes to tpr elasticsearch events
func (p *Processor) WatchElasticSearchClusterEvents(done chan struct{}, wg *sync.WaitGroup) {
	events, watchErrs := p.k8sclient.MonitorElasticSearchEvents(done)
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

// WatchDataPodEvents watches for changes to pods
func (p *Processor) WatchDataPodEvents(done chan struct{}, wg *sync.WaitGroup) {
	events, watchErrs := p.k8sclient.MonitorDataPods(done)
	go func() {
		for {
			select {
			case event := <-events:
				err := p.processPodEvent(event)
				if err != nil {
					logrus.Errorln(err)
				}
			case err := <-watchErrs:
				logrus.Errorln(err)
			case <-done:
				wg.Done()
				logrus.Println("Stopped data pod event watcher.")
				return
			}
		}
	}()
}

func (p *Processor) defaultUseSSL(specUseSSL *bool) bool {
	// Default to true
	if specUseSSL == nil {
		logrus.Infof("use-ssl not specified, defaulting to UseSSL=true")
		return true
	} else {
		logrus.Infof("use-ssl %v", *specUseSSL)
		return *specUseSSL
	}
}

func (p *Processor) refreshClusters() error {

	for key, cluster := range p.clusters {
		logrus.Infof("-----> Stop scheduler %s", key)
		cluster.Scheduler.Stop()
	}

	//Reset
	p.clusters = make(map[string]Cluster)

	// Get existing clusters
	currentClusters, err := p.k8sclient.CrdClient.EnterprisesV1().ElasticsearchClusters(v1.NamespaceAll).List(metav1.ListOptions{})

	if err != nil {
		logrus.Error("Could not get list of clusters: ", err)
		return err
	}

	for _, cluster := range currentClusters.Items {
		logrus.Infof("Found cluster: %s", cluster.ObjectMeta.Name)
		useSSL := p.defaultUseSSL(cluster.Spec.UseSSL)
		p.clusters[fmt.Sprintf("%s-%s", cluster.ObjectMeta.Name, cluster.ObjectMeta.Namespace)] = Cluster{
			ESCluster: &myspec.ElasticsearchCluster{
				Spec: myspec.ClusterSpec{
					ClientNodeReplicas:  cluster.Spec.ClientNodeReplicas,
					MasterNodeReplicas:  cluster.Spec.MasterNodeReplicas,
					DataNodeReplicas:    cluster.Spec.DataNodeReplicas,
					Zones:               cluster.Spec.Zones,
					DataDiskSize:        cluster.Spec.DataDiskSize,
					JavaOptions:         cluster.Spec.JavaOptions,
					ClientJavaOptions:   cluster.Spec.ClientJavaOptions,
					DataJavaOptions:     cluster.Spec.DataJavaOptions,
					MasterJavaOptions:   cluster.Spec.MasterJavaOptions,
					NetworkHost:         cluster.Spec.NetworkHost,
					KeepSecretsOnDelete: cluster.Spec.KeepSecretsOnDelete,
					Snapshot: myspec.Snapshot{
						SchedulerEnabled: cluster.Spec.Snapshot.SchedulerEnabled,
						RepoType:         cluster.Spec.Snapshot.RepoType,
						BucketName:       cluster.Spec.Snapshot.BucketName,
						CronSchedule:     cluster.Spec.Snapshot.CronSchedule,
						RepoRegion:       cluster.Spec.Snapshot.RepoRegion,
					},
					Storage: myspec.Storage{
						StorageType:            cluster.Spec.Storage.StorageType,
						StorageClassProvisoner: cluster.Spec.Storage.StorageClassProvisoner,
						StorageClass:           cluster.Spec.Storage.StorageClass,
						VolumeReclaimPolicy:    cluster.Spec.Storage.VolumeReclaimPolicy,
					},
					Scheduler: myspec.Scheduler{
						RepoType:     cluster.Spec.Snapshot.RepoType,
						BucketName:   cluster.Spec.Snapshot.BucketName,
						CronSchedule: cluster.Spec.Snapshot.CronSchedule,
						Enabled:      cluster.Spec.Snapshot.SchedulerEnabled,
						Auth: myspec.SchedulerAuthentication{
							UserName: cluster.Spec.Snapshot.Authentication.UserName,
							Password: cluster.Spec.Snapshot.Authentication.Password,
						},
						RepoAuth: myspec.RepoSchedulerAuthentication{
							RepoAccessKey: cluster.Spec.Snapshot.RepoAuthentication.RepoAccessKey,
							RepoSecretKey: cluster.Spec.Snapshot.RepoAuthentication.RepoSecretKey,
						},
						RepoRegion:  cluster.Spec.Snapshot.RepoRegion,
						UseSSL:      useSSL,
						ElasticURL:  k8sutil.GetESURL(p.k8sclient.GetClientServiceNameFullDNS(cluster.ObjectMeta.Name, cluster.ObjectMeta.Namespace), cluster.Spec.UseSSL),
						ClusterName: cluster.ObjectMeta.Name,
						Namespace:   cluster.ObjectMeta.Namespace,
					},
					Resources: myspec.Resources{
						Limits: myspec.MemoryCPU{
							Memory: cluster.Spec.Resources.Limits.Memory,
							CPU:    cluster.Spec.Resources.Limits.CPU,
						},
						Requests: myspec.MemoryCPU{
							Memory: cluster.Spec.Resources.Requests.Memory,
							CPU:    cluster.Spec.Resources.Requests.CPU,
						},
					},
					Instrumentation: myspec.Instrumentation{
						StatsdHost: cluster.Spec.Instrumentation.StatsdHost,
					},
					Kibana: myspec.Kibana{
						Image:              cluster.Spec.Kibana.Image,
						ImagePullPolicy:    cluster.Spec.Kibana.ImagePullPolicy,
						ServiceAccountName: cluster.Spec.Kibana.ServiceAccountName,
					},
					Cerebro: myspec.Cerebro{
						Image:              cluster.Spec.Cerebro.Image,
						ImagePullPolicy:    cluster.Spec.Cerebro.ImagePullPolicy,
						ServiceAccountName: cluster.Spec.Cerebro.ServiceAccountName,
					},
					NodeSelector:       cluster.Spec.NodeSelector,
					Tolerations:        cluster.Spec.Tolerations,
					Affinity:           cluster.Spec.Affinity,
					UseSSL:             &useSSL,
					ServiceAccountName: cluster.Spec.ServiceAccountName,
				},
			},
			Scheduler: snapshot.New(
				cluster.Spec.Snapshot.RepoType,
				cluster.Spec.Snapshot.BucketName,
				cluster.Spec.Snapshot.CronSchedule,
				cluster.Spec.Snapshot.SchedulerEnabled,
				useSSL,
				cluster.Spec.Snapshot.Authentication.UserName,
				cluster.Spec.Snapshot.Authentication.Password,
				cluster.Spec.Snapshot.Image,
				p.k8sclient.GetClientServiceNameFullDNS(cluster.ObjectMeta.Name, cluster.ObjectMeta.Namespace),
				cluster.ObjectMeta.Name,
				cluster.ObjectMeta.Namespace,
				cluster.Spec.Snapshot.RepoAuthentication.RepoAccessKey,
				cluster.Spec.Snapshot.RepoAuthentication.RepoSecretKey,
				cluster.Spec.Snapshot.RepoRegion,
				p.k8sclient.Kclient,
			),
		}
	}

	return nil
}

func (p *Processor) processElasticSearchClusterEvent(c *myspec.ElasticsearchCluster) error {
	processorLock.Lock()
	defer processorLock.Unlock()
	logrus.Infof("Process Elasticsearch Event %v", c.Type)
	switch {
	case c.Type == "ADDED" || c.Type == "MODIFIED":
		return p.processElasticSearchCluster(c)
	case c.Type == "DELETED":
		p.deleteElasticSearchCluster(c)
	}
	return nil
}

func (p *Processor) processPodEvent(c *v1.Pod) error {
	processorLock.Lock()
	defer processorLock.Unlock()

	role := c.Labels["role"]
	switch role {
	case "data":
		return p.processDataPodEvent(c)
	case "master":
		return p.processMasterPodEvent(c)
	}
	return nil
}

func (p *Processor) processDataPodEvent(c *v1.Pod) error {
	// Set the policy to retain
	name := c.Labels["component"]
	name = name[14:len(name)]

	p.k8sclient.UpdateVolumeReclaimPolicy(p.clusters[fmt.Sprintf("%s-%s", name, c.ObjectMeta.Namespace)].ESCluster.Spec.Storage.VolumeReclaimPolicy, c.ObjectMeta.Namespace, name)

	return nil
}

func (p *Processor) processMasterPodEvent(c *v1.Pod) error {
	name := c.Labels["component"]
	name = name[14:len(name)]
	// get all ready master nodes
	m, err := p.k8sclient.GetMasterNodes(c.ObjectMeta.Namespace, name)
	if err != nil {
		return err
	}
	readyMasterPods := 0
	for _, pod := range m.Items {
		for _, cond := range pod.Status.Conditions {
			if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
				readyMasterPods++
			}
		}
	}
	logrus.Debugf("Found %d ready master pods", readyMasterPods)
	// calc min master nodes
	minMasterNodes := elasticsearchutil.MinMasterNodes(readyMasterPods)
	// set min master node value in ES
	cluster, ok := p.clusters[fmt.Sprintf("%s-%s", name, c.ObjectMeta.Namespace)]
	if !ok {
		return fmt.Errorf("No elasticsearch cluster with name %s found", name)
	}
	esHost := cluster.ESCluster.Spec.Scheduler.ElasticURL
	if cluster.ESCluster.Spec.MasterNodeReplicas == readyMasterPods {
		logrus.Infof("All %d master nodes ready. Setting 'discovery.zen.minimum_master_nodes' to %d", readyMasterPods, minMasterNodes)
		err := elasticsearchutil.UpdateDiscoveryMinMasterNodes(esHost, minMasterNodes)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Processor) processElasticSearchCluster(c *myspec.ElasticsearchCluster) error {
	logrus.Println("--------> Received ElasticSearch Event!")

	// Refresh
	if err := p.refreshClusters(); err != nil {
		logrus.Error("Error refreshing cluster ", err)
		return err
	}

	// Is a base image defined in the custom cluster?
	var baseImage = p.calcBaseImage(p.baseImage, c.Spec.ElasticSearchImage)

	logrus.Infof("Using [%s] as image for es cluster", baseImage)

	// Default UseSSL to true
	useSSL := p.defaultUseSSL(c.Spec.UseSSL)
	c.Spec.UseSSL = &useSSL

	if useSSL && p.k8sclient.CertsSecretExists(c.ObjectMeta.Namespace, c.ObjectMeta.Name) == false {
		// Create certs
		logrus.Info("Creating new certs!")
		if err := p.k8sclient.GenerateCerts("/tmp/certs/config", "/tmp/certs/certs", c.ObjectMeta.Namespace, c.ObjectMeta.Name); err != nil {
			return err
		}

		if err := p.k8sclient.CreateCertsSecret(c.ObjectMeta.Namespace, c.ObjectMeta.Name, "/tmp/certs/certs"); err != nil {
			return err
		}
	}

	// Create Services
	if err := p.k8sclient.CreateDiscoveryService(c.ObjectMeta.Name, c.ObjectMeta.Namespace); err != nil {
		logrus.Error("Error creating discovery service ", err)
		return err
	}

	if err := p.k8sclient.CreateDataService(c.ObjectMeta.Name, c.ObjectMeta.Namespace); err != nil {
		logrus.Error("Error creating data service ", err)
		return err
	}

	if err := p.k8sclient.CreateClientService(c.ObjectMeta.Name, c.ObjectMeta.Namespace, c.Spec.NodePort); err != nil {
		logrus.Error("Error creating client service ", err)
		return err
	}

	if err := p.k8sclient.CreateClientDeployment(baseImage, &c.Spec.ClientNodeReplicas, c.Spec.JavaOptions, c.Spec.ClientJavaOptions,
		c.Spec.Resources, c.Spec.ImagePullSecrets, c.Spec.ImagePullPolicy, c.Spec.ServiceAccountName, c.ObjectMeta.Name, c.Spec.Instrumentation.StatsdHost, c.Spec.NetworkHost, c.ObjectMeta.Namespace, c.Spec.UseSSL, c.Spec.Affinity); err != nil {
		logrus.Error("Error creating client deployment ", err)
		return err
	}

	zoneCount := 0
	if len(c.Spec.Zones) != 0 {
		zoneCount = len(c.Spec.Zones)

		// Create Storage Classes
		for _, sc := range c.Spec.Zones {
			if err := p.k8sclient.CreateStorageClass(sc, c.Spec.Storage.StorageClassProvisoner, c.Spec.Storage.StorageType, c.ObjectMeta.Name, c.Spec.Storage.Encrypted); err != nil {
				logrus.Error("Error creating storage class ", err)
				return err
			}
		}

		zoneDistributionData := p.calculateZoneDistribution(c.Spec.DataNodeReplicas, zoneCount)
		zoneDistributionMaster := p.calculateZoneDistribution(c.Spec.MasterNodeReplicas, zoneCount)

		// Create Master Nodes
		for index, count := range zoneDistributionMaster {
			if err := p.k8sclient.CreateDataNodeDeployment("master", &count, baseImage, c.Spec.Zones[index], c.Spec.DataDiskSize, c.Spec.Resources,
				c.Spec.ImagePullSecrets, c.Spec.ImagePullPolicy, c.Spec.ServiceAccountName, c.ObjectMeta.Name, c.Spec.Instrumentation.StatsdHost, c.Spec.NetworkHost,
				c.ObjectMeta.Namespace, c.Spec.JavaOptions, c.Spec.MasterJavaOptions, c.Spec.DataJavaOptions, c.Spec.UseSSL, c.Spec.Scheduler.ElasticURL, c.Spec.NodeSelector, c.Spec.Tolerations); err != nil {
				logrus.Error("Error creating master node deployment ", err)
				return err
			}
		}

		// Create Data Nodes
		for index, count := range zoneDistributionData {
			if err := p.k8sclient.CreateDataNodeDeployment("data", &count, baseImage, c.Spec.Zones[index], c.Spec.DataDiskSize, c.Spec.Resources,
				c.Spec.ImagePullSecrets, c.Spec.ImagePullPolicy, c.Spec.ServiceAccountName, c.ObjectMeta.Name, c.Spec.Instrumentation.StatsdHost, c.Spec.NetworkHost,
				c.ObjectMeta.Namespace, c.Spec.JavaOptions, c.Spec.MasterJavaOptions, c.Spec.DataJavaOptions, c.Spec.UseSSL, c.Spec.Scheduler.ElasticURL, c.Spec.NodeSelector, c.Spec.Tolerations); err != nil {
				logrus.Error("Error creating data node deployment ", err)

				return err
			}
		}
	} else {
		// No zones defined, rely on current provisioning logic which may break. Other strategy is to use emptyDir?
		// NOTE: Issue with dynamic PV provisioning (https://github.com/kubernetes/kubernetes/issues/34583)
		if len(c.Spec.Storage.StorageClass) == 0 {
			c.Spec.Storage.StorageClass = "default"
		}

		// Create Master Nodes
		if err := p.k8sclient.CreateDataNodeDeployment("master", func() *int32 { i := int32(c.Spec.MasterNodeReplicas); return &i }(), baseImage, c.Spec.Storage.StorageClass,
			c.Spec.DataDiskSize, c.Spec.Resources, c.Spec.ImagePullSecrets, c.Spec.ImagePullPolicy, c.Spec.ServiceAccountName, c.ObjectMeta.Name,
			c.Spec.Instrumentation.StatsdHost, c.Spec.NetworkHost, c.ObjectMeta.Namespace, c.Spec.JavaOptions, c.Spec.MasterJavaOptions, c.Spec.DataJavaOptions, c.Spec.UseSSL, c.Spec.Scheduler.ElasticURL, c.Spec.NodeSelector, c.Spec.Tolerations); err != nil {
			logrus.Error("Error creating master node deployment ", err)

			return err
		}

		// Create Data Nodes
		if err := p.k8sclient.CreateDataNodeDeployment("data", func() *int32 { i := int32(c.Spec.DataNodeReplicas); return &i }(), baseImage, c.Spec.Storage.StorageClass,
			c.Spec.DataDiskSize, c.Spec.Resources, c.Spec.ImagePullSecrets, c.Spec.ImagePullPolicy, c.Spec.ServiceAccountName, c.ObjectMeta.Name,
			c.Spec.Instrumentation.StatsdHost, c.Spec.NetworkHost, c.ObjectMeta.Namespace, c.Spec.JavaOptions, c.Spec.MasterJavaOptions, c.Spec.DataJavaOptions, c.Spec.UseSSL, c.Spec.Scheduler.ElasticURL, c.Spec.NodeSelector, c.Spec.Tolerations); err != nil {
			logrus.Error("Error creating data node deployment ", err)
			return err
		}
	}

	// Deploy Kibana
	if c.Spec.Kibana.Image != "" {

		if err := p.k8sclient.CreateKibanaDeployment(c.Spec.Kibana.Image, c.ObjectMeta.Name, c.ObjectMeta.Namespace, c.Spec.ImagePullSecrets, c.Spec.Kibana.ImagePullPolicy, c.Spec.Kibana.ServiceAccountName, c.Spec.UseSSL); err != nil {
			logrus.Error("Error creating kibana deployment ", err)
			return err
		}

		if err := p.k8sclient.CreateMgmtService("kibana", c.ObjectMeta.Name, c.ObjectMeta.Namespace); err != nil {
			logrus.Error("Error creating kibana mgmt service ", err)
			return err
		}
	}

	// Deploy Cerebro
	if c.Spec.Cerebro.Image != "" {
		host := fmt.Sprintf("elasticsearch-%s", c.ObjectMeta.Name)
		cerebroConf := p.k8sclient.CreateCerebroConfiguration(host, c.Spec.UseSSL)
		name := fmt.Sprintf("%s-%s", c.ObjectMeta.Name, "cerebro")

		// create/update cerebro configMap
		if p.k8sclient.ConfigmapExists(c.ObjectMeta.Namespace, name) {
			if err := p.k8sclient.UpdateConfigMap(c.ObjectMeta.Namespace, name, cerebroConf); err != nil {
				logrus.Error("Error updating configmap ", err)
				return err
			}
		} else {
			if err := p.k8sclient.CreateConfigMap(c.ObjectMeta.Namespace, name, cerebroConf); err != nil {
				logrus.Error("Error creating configmaop ", err)
				return err
			}
		}

		if err := p.k8sclient.CreateCerebroDeployment(c.Spec.Cerebro.Image, c.ObjectMeta.Name, c.ObjectMeta.Namespace, name, c.Spec.ImagePullSecrets, c.Spec.Cerebro.ImagePullPolicy, c.Spec.Cerebro.ServiceAccountName, c.Spec.UseSSL); err != nil {
			logrus.Error("Error creating cerebro deployment ", err)
			return err
		}

		if err := p.k8sclient.CreateMgmtService("cerebro", c.ObjectMeta.Name, c.ObjectMeta.Namespace); err != nil {
			logrus.Error("Error creating cerebro mgmt service ", err)
			return err
		}

	}

	// Setup CronSchedule
	p.clusters[fmt.Sprintf("%s-%s", c.ObjectMeta.Name, c.ObjectMeta.Namespace)].Scheduler.Init()
	logrus.Println("--------> ElasticSearch Event finished!")

	return nil
}

func (p *Processor) deleteElasticSearchCluster(c *myspec.ElasticsearchCluster) {
	logrus.Println("--------> Deleting elasticSearch Cluster ...removing all components...")

	if err := p.k8sclient.DeleteDeployment(c.ObjectMeta.Name, c.ObjectMeta.Namespace, "client"); err != nil {
		logrus.Errorf("Could not delete client deployment %s: %v", c.ObjectMeta.Name, err)
	}

	if err := p.k8sclient.DeleteDeployment(c.ObjectMeta.Name, c.ObjectMeta.Namespace, "kibana"); err != nil {
		logrus.Errorf("Could not delete kibana deployment %s: %v", c.ObjectMeta.Name, err)
	}

	if err := p.k8sclient.DeleteDeployment(c.ObjectMeta.Name, c.ObjectMeta.Namespace, "cerebro"); err != nil {
		logrus.Errorf("Could not delete cerebro deployment %s: %v", c.ObjectMeta.Name, err)
	}

	if err := p.k8sclient.DeleteStatefulSet("master", c.ObjectMeta.Name, c.ObjectMeta.Namespace); err != nil {
		logrus.Errorf("Could not delete master deployment %s: %v", c.ObjectMeta.Name, err)
	}

	if err := p.k8sclient.DeleteStatefulSet("data", c.ObjectMeta.Name, c.ObjectMeta.Namespace); err != nil {
		logrus.Errorf("Could not delete stateful set %s: %v", c.ObjectMeta.Name, err)
	}

	if err := p.k8sclient.DeleteServices(c.ObjectMeta.Name, c.ObjectMeta.Namespace); err != nil {
		logrus.Errorf("Could not delete service %s : %v", c.ObjectMeta.Name, err)
	}

	if err := p.k8sclient.DeleteStorageClasses(c.ObjectMeta.Name); err != nil {
		logrus.Errorf("Could not delete storage class %s: %v", c.ObjectMeta.Name, err)
	}

	p.clusters[fmt.Sprintf("%s-%s", c.ObjectMeta.Name, c.ObjectMeta.Namespace)].Scheduler.Stop()

	if !c.Spec.KeepSecretsOnDelete {
		if err := p.k8sclient.DeleteCertsSecret(c.ObjectMeta.Namespace, c.ObjectMeta.Name); err != nil {
			logrus.Errorf("Could not delete cert secret %s: %v", c.ObjectMeta.Name, err)
		}
	} else {
		logrus.Println("Keeping existing cert secrets...")
	}

	logrus.Println("--------> ElasticSearch Cluster deleted")

}

func (p *Processor) calculateZoneDistribution(dataReplicas, zonesCount int) []int32 {
	if zonesCount == 0 {
		zonesCount = 1
	}

	zoneDistribution := make([]int32, zonesCount)

	index := 0
	for i := 0; i < dataReplicas; i++ {
		if index >= zonesCount {
			index = 0
		}

		zoneDistribution[index]++
		index++
	}

	return zoneDistribution
}

func (p *Processor) calcBaseImage(baseImage, customImage string) string {
	if len(customImage) > 0 {
		baseImage = customImage
	}

	return baseImage
}
