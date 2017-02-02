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

package k8sutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	appsType "k8s.io/client-go/kubernetes/typed/apps/v1beta1"
	coreType "k8s.io/client-go/kubernetes/typed/core/v1"
	extensionsType "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	storageType "k8s.io/client-go/kubernetes/typed/storage/v1beta1"
	"k8s.io/client-go/pkg/api/resource"
	"k8s.io/client-go/pkg/api/v1"
	apps "k8s.io/client-go/pkg/apis/apps/v1beta1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	storage "k8s.io/client-go/pkg/apis/storage/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	namespace                  = os.Getenv("NAMESPACE")
	tprName                    = "elasticsearch-cluster.enterprises.upmc.com"
	elasticSearchEndpoint      = fmt.Sprintf("/apis/enterprises.upmc.com/v1/namespaces/%s/elasticsearchclusters", namespace)
	elasticSearchWatchEndpoint = fmt.Sprintf("/apis/enterprises.upmc.com/v1/watch/namespaces/%s/elasticsearchclusters", namespace)
	tprEndpoint                = "/apis/extensions/v1beta1/thirdpartyresources"
)

const (
	dataDir    = "/data"
	backupFile = "/var/elastic/latest.backup"

	discoveryServiceName = "elasticsearch-discovery"
	dataServiceName      = "es-data-svc"
	clientServiceName    = "elasticsearch"

	clientDeploymentName = "es-client"
	masterDeploymentName = "es-master"
	dataDeploymentName   = "es-data"

	secretName                 = "es-certs"
	awsStorageClassProvisioner = "kubernetes.io/aws-ebs"
)

// KubeInterface abstracts the kubernetes client
type KubeInterface interface {
	Services(namespace string) coreType.ServiceInterface
	ThirdPartyResources() extensionsType.ThirdPartyResourceInterface
	Deployments(namespace string) extensionsType.DeploymentInterface
	StatefulSets(namespace string) appsType.StatefulSetInterface
	StorageClasses() storageType.StorageClassInterface
}

// K8sutil defines the kube object
type K8sutil struct {
	Kclient    KubeInterface
	MasterHost string
}

// ThirdPartyResource in Kubernetes
type ThirdPartyResource struct {
	APIVersion  string               `json:"apiVersion"`
	Kind        string               `json:"kind"`
	Description string               `json:"description"`
	Metadata    map[string]string    `json:"metadata"`
	Versions    [1]map[string]string `json:"versions,omitempty"`
}

// ElasticSearchEvent stores when a ES needs created
type ElasticSearchEvent struct {
	Type   string               `json:"type"`
	Object ElasticSearchCluster `json:"object"`
}

// ElasticSearchCluster represents a custom ES object
type ElasticSearchCluster struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   map[string]string `json:"metadata"`
	Spec       ElasticSearchSpec `json:"spec"`
}

// ElasticSearchSpec represents the custom data of the object
type ElasticSearchSpec struct {
	ClusterName        string   `json: "clustername"`
	ClientNodeReplicas int32    `json:"client-node-replicas"`
	MasterNodeReplicas int32    `json:"master-node-replicas"`
	DataNodeReplicas   int      `json:"data-node-replicas"`
	Zones              []string `json:"zones"`
	DataDiskSize       string   `json:"data-volume-size"`
}

// ElasticSearchList represents a list of ES Clusters
type ElasticSearchList struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   map[string]string      `json:"metadata"`
	Items      []ElasticSearchCluster `json:"items"`
}

// ElasticSearch represents a Kubernetes ES type
type ElasticSearch struct {
	Kind       string            `json:"kind"`
	APIVersion string            `json:"apiVersion"`
	Metadata   map[string]string `json:"metadata"`
	Data       map[string]string `json:"data"`
	Type       string            `json:"type"`
}

// New creates a new instance of k8sutil
func New(kubeCfgFile, masterHost string) (*K8sutil, error) {

	client, err := newKubeClient(kubeCfgFile)

	if err != nil {
		logrus.Fatalf("Could not init Kubernetes client! [%s]", err)
	}

	k := &K8sutil{
		Kclient:    client,
		MasterHost: masterHost,
	}

	return k, nil

}

func newKubeClient(kubeCfgFile string) (KubeInterface, error) {

	var client *kubernetes.Clientset
	// Should we use in cluster or out of cluster config
	if len(kubeCfgFile) == 0 {
		logrus.Info("Using InCluster k8s config")
		cfg, err := rest.InClusterConfig()

		if err != nil {
			return nil, err
		}

		client, err = kubernetes.NewForConfig(cfg)

		if err != nil {
			return nil, err
		}
	} else {
		logrus.Infof("Using OutOfCluster k8s config with kubeConfigFile: %s", kubeCfgFile)
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeCfgFile)

		if err != nil {
			logrus.Error("Got error trying to create client: ", err)
			return nil, err
		}

		client, err = kubernetes.NewForConfig(cfg)

		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

// GetElasticSearchClusters returns a list of custom clusters defined
func (k *K8sutil) GetElasticSearchClusters() ([]ElasticSearchCluster, error) {
	var resp *http.Response
	var err error
	for {
		resp, err = http.Get(k.MasterHost + elasticSearchEndpoint)
		if err != nil {
			logrus.Error(err)
			time.Sleep(5 * time.Second)
			continue
		}
		break
	}

	var elasticSearchList ElasticSearchList
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&elasticSearchList)
	if err != nil {
		logrus.Error("Could not get list of elasticsearch clusters: ", err)
		return nil, err
	}

	return elasticSearchList.Items, nil
}

// MonitorElasticSearchEvents watches for new or removed clusters
func (k *K8sutil) MonitorElasticSearchEvents() (<-chan ElasticSearchEvent, <-chan error) {
	// Validate Namespace exists
	if len(namespace) == 0 {
		logrus.Errorln("WARNING: No namespace found! Events will not be able to be watched!")
	}

	events := make(chan ElasticSearchEvent)
	errc := make(chan error, 1)
	go func() {
		for {
			resp, err := http.Get(k.MasterHost + elasticSearchWatchEndpoint)
			if err != nil {
				errc <- err
				time.Sleep(5 * time.Second)
				continue
			}
			if resp.StatusCode != 200 {
				errc <- errors.New("Invalid status code: " + resp.Status)
				time.Sleep(5 * time.Second)
				continue
			}

			decoder := json.NewDecoder(resp.Body)
			for {
				var event ElasticSearchEvent
				err = decoder.Decode(&event)
				if err != nil {
					errc <- err
					break
				}
				events <- event
			}
		}
	}()

	return events, errc
}

// CreateKubernetesThirdPartyResource checks if ElasticSearch TPR exists. If not, create
func (k *K8sutil) CreateKubernetesThirdPartyResource() error {
	tprResult, _ := k.Kclient.ThirdPartyResources().Get(tprName)

	if len(tprResult.Name) == 0 {
		logrus.Info("ElasticSearchCluster ThirdPartyResource not found, creating...")

		tpr := &v1beta1.ThirdPartyResource{
			ObjectMeta: v1.ObjectMeta{
				Name: tprName,
			},
			Versions: []v1beta1.APIVersion{
				{Name: "v1"},
			},
			Description: "Managed elasticsearch clusters",
		}

		_, err := k.Kclient.ThirdPartyResources().Create(tpr)
		if err != nil {
			logrus.Error("Error creating ThirdPartyResource: ", err)
			return err
		}
	} else {
		logrus.Info("Elastic Search TPR already existing...")
	}

	return nil
}

// CreateDiscoveryService creates the discovery service
func (k *K8sutil) CreateDiscoveryService() error {

	// Check if service exists
	svc, err := k.Kclient.Services(namespace).Get(discoveryServiceName)

	// Service missing, create
	if len(svc.Name) == 0 {
		logrus.Info("Discovery Service not found, creating...")

		discoverySvc := &v1.Service{
			ObjectMeta: v1.ObjectMeta{
				Name: discoveryServiceName,
				Labels: map[string]string{
					"component": "elasticsearch",
					"role":      "master",
				},
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"component": "elasticsearch",
					"role":      "master",
				},
				Ports: []v1.ServicePort{
					v1.ServicePort{
						Name:     "transport",
						Port:     9300,
						Protocol: "TCP",
					},
				},
			},
		}

		_, err := k.Kclient.Services(namespace).Create(discoverySvc)

		if err != nil {
			logrus.Error("Could not create discovery service! ", err)
			return err
		}
	} else if err != nil {
		logrus.Error("Could not get discovery service! ", err)
		return err
	}

	return nil
}

// CreateDataService creates the data service
func (k *K8sutil) CreateDataService() error {

	// Check if service exists
	svc, err := k.Kclient.Services(namespace).Get(dataServiceName)

	// Service missing, create
	if len(svc.Name) == 0 {
		logrus.Infof("%s not found, creating...", dataServiceName)

		dataService := &v1.Service{
			ObjectMeta: v1.ObjectMeta{
				Name: dataServiceName,
				Labels: map[string]string{
					"service.alpha.kubernetes.io/tolerate-unready-endpoints": "true",
				},
				Annotations: map[string]string{
					"component": "elasticsearch",
					"name":      dataServiceName,
				},
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"component": "elasticsearch",
					"role":      "data",
				},
				Ports: []v1.ServicePort{
					v1.ServicePort{
						Name:     "transport",
						Port:     9300,
						Protocol: "TCP",
					},
				},
			},
		}

		_, err := k.Kclient.Services(namespace).Create(dataService)

		if err != nil {
			logrus.Error("Could not create data service", err)
			return err
		}
	} else if err != nil {
		logrus.Error("Could not get data service! ", err)
		return err
	}

	return nil
}

// CreateClientService creates the client service
func (k *K8sutil) CreateClientService() error {

	// Check if service exists
	svc, err := k.Kclient.Services(namespace).Get(clientServiceName)

	// Service missing, create
	if len(svc.Name) == 0 {
		logrus.Infof("%s not found, creating...", clientServiceName)

		clientSvc := &v1.Service{
			ObjectMeta: v1.ObjectMeta{
				Name: clientServiceName,
				Labels: map[string]string{
					"component": "elasticsearch",
					"role":      "client",
				},
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"component": "elasticsearch",
					"role":      "client",
				},
				Ports: []v1.ServicePort{
					v1.ServicePort{
						Name:     "http",
						Port:     9200,
						Protocol: "TCP",
					},
				},
			},
		}

		_, err := k.Kclient.Services(namespace).Create(clientSvc)

		if err != nil {
			logrus.Error("Could not create client service", err)
			return err
		}
	} else if err != nil {
		logrus.Error("Could not get client service! ", err)
		return err
	}

	return nil
}

// CreateClientMasterDeployment creates the client or master deployment
func (k *K8sutil) CreateClientMasterDeployment(deploymentType, baseImage string, replicas *int32) error {

	var deploymentName, role, isNodeMaster, httpEnable string

	if deploymentType == "client" {
		httpEnable = "true"
		deploymentName = clientDeploymentName
		isNodeMaster = "false"
		role = "client"
	} else if deploymentType == "master" {
		httpEnable = "false"
		deploymentName = masterDeploymentName
		isNodeMaster = "true"
		role = "master"
	}

	// Check if deployment exists
	deployment, err := k.Kclient.Deployments(namespace).Get(deploymentName)

	if len(deployment.Name) == 0 {
		logrus.Infof("%s not found, creating...", deploymentName)

		deployment := &v1beta1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name: deploymentName,
				Labels: map[string]string{
					"component": "elasticsearch",
					"role":      role,
					"name":      deploymentName,
				},
			},
			Spec: v1beta1.DeploymentSpec{
				Replicas: replicas,
				Template: v1.PodTemplateSpec{
					ObjectMeta: v1.ObjectMeta{
						Labels: map[string]string{
							"component": "elasticsearch",
							"role":      role,
							"name":      deploymentName,
						},
						Annotations: map[string]string{
							"pod.beta.kubernetes.io/init-containers": "[ { \"name\": \"sysctl\", \"image\": \"busybox\", \"imagePullPolicy\": \"IfNotPresent\", \"command\": [\"sysctl\", \"-w\", \"vm.max_map_count=262144\"], \"securityContext\": { \"privileged\": true } }]",
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							v1.Container{
								Name: deploymentName,
								SecurityContext: &v1.SecurityContext{
									Privileged: &[]bool{true}[0],
									Capabilities: &v1.Capabilities{
										Add: []v1.Capability{
											"IPC_LOCK",
										},
									},
								},
								Image:           baseImage,
								ImagePullPolicy: "Always",
								Env: []v1.EnvVar{
									v1.EnvVar{
										Name: "NAMESPACE",
										ValueFrom: &v1.EnvVarSource{
											FieldRef: &v1.ObjectFieldSelector{
												FieldPath: "metadata.namespace",
											},
										},
									},
									v1.EnvVar{
										Name:  "CLUSTER_NAME",
										Value: "myesdb",
									},
									v1.EnvVar{
										Name:  "NODE_MASTER",
										Value: isNodeMaster,
									},
									v1.EnvVar{
										Name:  "NODE_DATA",
										Value: "false",
									},
									v1.EnvVar{
										Name:  "HTTP_ENABLE",
										Value: httpEnable,
									},
									v1.EnvVar{
										Name:  "ES_JAVA_OPTS",
										Value: "-Xms1024m -Xmx1024m",
									},
								},
								Ports: []v1.ContainerPort{
									v1.ContainerPort{
										Name:          "transport",
										ContainerPort: 9300,
										Protocol:      v1.ProtocolTCP,
									},
									v1.ContainerPort{
										Name:          "http",
										ContainerPort: 9200,
										Protocol:      v1.ProtocolTCP,
									},
								},
								VolumeMounts: []v1.VolumeMount{
									v1.VolumeMount{
										Name:      "storage",
										MountPath: "/data",
									},
									v1.VolumeMount{
										Name:      "es-certs",
										MountPath: "/elasticsearch/config/certs",
									},
								},
							},
						},
						Volumes: []v1.Volume{
							v1.Volume{
								Name: "storage",
								VolumeSource: v1.VolumeSource{
									EmptyDir: &v1.EmptyDirVolumeSource{},
								},
							},
							v1.Volume{
								Name: "es-certs",
								VolumeSource: v1.VolumeSource{
									Secret: &v1.SecretVolumeSource{
										SecretName: "es-certs",
									},
								},
							},
						},
					},
				},
			},
		}

		_, err := k.Kclient.Deployments(namespace).Create(deployment)

		if err != nil {
			logrus.Error("Could not create client deployment: ", err)
			return err
		}
	} else if err != nil {
		logrus.Error("Could not get client deployment! ", err)
		return err
	}

	return nil
}

// CreateDataNodeDeployment creates the data node deployment
func (k *K8sutil) CreateDataNodeDeployment(replicas *int32, baseImage, storageClass string, dataDiskSize string) error {

	statefulSetName := fmt.Sprintf("%s-%s", dataDeploymentName, storageClass)

	// Check if StatefulSet exists
	statefulSet, err := k.Kclient.StatefulSets(namespace).Get(statefulSetName)

	if len(statefulSet.Name) == 0 {
		volumeSize, _ := resource.ParseQuantity(dataDiskSize)

		logrus.Infof("StatefulSet %s not found, creating...", statefulSetName)

		statefulSet := &apps.StatefulSet{
			ObjectMeta: v1.ObjectMeta{
				Name: statefulSetName,
				Labels: map[string]string{
					"component": "elasticsearch",
					"role":      "data",
					"name":      statefulSetName,
				},
			},
			Spec: apps.StatefulSetSpec{
				Replicas:    replicas,
				ServiceName: "es-data-svc",
				Template: v1.PodTemplateSpec{
					ObjectMeta: v1.ObjectMeta{
						Labels: map[string]string{
							"component": "elasticsearch",
							"role":      "data",
							"name":      statefulSetName,
						},
						Annotations: map[string]string{
							"pod.beta.kubernetes.io/init-containers": "[ { \"name\": \"sysctl\", \"image\": \"busybox\", \"imagePullPolicy\": \"IfNotPresent\", \"command\": [\"sysctl\", \"-w\", \"vm.max_map_count=262144\"], \"securityContext\": { \"privileged\": true } }]",
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							v1.Container{
								Name: statefulSetName,
								SecurityContext: &v1.SecurityContext{
									Privileged: &[]bool{true}[0],
									Capabilities: &v1.Capabilities{
										Add: []v1.Capability{
											"IPC_LOCK",
										},
									},
								},
								Image:           baseImage,
								ImagePullPolicy: "Always",
								Env: []v1.EnvVar{
									v1.EnvVar{
										Name: "NAMESPACE",
										ValueFrom: &v1.EnvVarSource{
											FieldRef: &v1.ObjectFieldSelector{
												FieldPath: "metadata.namespace",
											},
										},
									},
									v1.EnvVar{
										Name:  "CLUSTER_NAME",
										Value: "myesdb",
									},
									v1.EnvVar{
										Name:  "NODE_MASTER",
										Value: "false",
									},
									v1.EnvVar{
										Name:  "HTTP_ENABLE",
										Value: "false",
									},
									v1.EnvVar{
										Name:  "ES_JAVA_OPTS",
										Value: "-Xms1024m -Xmx1024m",
									},
								},
								Ports: []v1.ContainerPort{
									v1.ContainerPort{
										Name:          "transport",
										ContainerPort: 9300,
										Protocol:      v1.ProtocolTCP,
									},
								},
								VolumeMounts: []v1.VolumeMount{
									v1.VolumeMount{
										Name:      "es-data",
										MountPath: "/data",
									},
									v1.VolumeMount{
										Name:      "es-certs",
										MountPath: "/elasticsearch/config/certs",
									},
								},
							},
						},
						Volumes: []v1.Volume{
							v1.Volume{
								Name: "es-certs",
								VolumeSource: v1.VolumeSource{
									Secret: &v1.SecretVolumeSource{
										SecretName: "es-certs",
									},
								},
							},
						},
					},
				},
				VolumeClaimTemplates: []v1.PersistentVolumeClaim{
					v1.PersistentVolumeClaim{
						ObjectMeta: v1.ObjectMeta{
							Name: "es-data",
							Annotations: map[string]string{
								"volume.beta.kubernetes.io/storage-class": storageClass,
							},
						},
						Spec: v1.PersistentVolumeClaimSpec{
							AccessModes: []v1.PersistentVolumeAccessMode{
								v1.ReadWriteOnce,
							},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceStorage: volumeSize,
								},
							},
						},
					},
				},
			},
		}

		_, err := k.Kclient.StatefulSets(namespace).Create(statefulSet)

		if err != nil {
			logrus.Error("Could not create data stateful set: ", err)
			return err
		}
	} else if err != nil {
		logrus.Error("Could not get data stateful set! ", err)
		return err
	}

	return nil
}

// CreatePeristentVolume creates a CreatePeristentVolume
func (k *K8sutil) CreatePeristentVolume(zone string) error {

	return nil
}

// CreateStorageClass creates a storage class
// NOTE: Right now only creating AWS EBS volumes type gp2
func (k *K8sutil) CreateStorageClass(zone string) error {

	// Check if storage class exists
	storageClass, err := k.Kclient.StorageClasses().Get(zone)

	if len(storageClass.Name) == 0 {
		logrus.Infof("StorgeClass %s not found, creating...", zone)

		class := &storage.StorageClass{
			ObjectMeta: v1.ObjectMeta{
				Name: zone,
				Labels: map[string]string{
					"component": "elasticsearch",
				},
			},
			Provisioner: awsStorageClassProvisioner,
			Parameters: map[string]string{
				"type": "gp2",
				"zone": zone,
			},
		}

		_, err := k.Kclient.StorageClasses().Create(class)

		if err != nil {
			logrus.Error("Could not create storage class: ", err)
			return err
		}
	} else if err != nil {
		logrus.Error("Could not get storage class! ", err)
		return err
	}

	return nil
}
