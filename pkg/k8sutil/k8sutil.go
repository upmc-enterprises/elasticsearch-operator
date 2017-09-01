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

package k8sutil

import (
	"fmt"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	myspec "github.com/upmc-enterprises/elasticsearch-operator/pkg/spec"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	apps "k8s.io/client-go/pkg/apis/apps/v1beta1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	tprName = "elasticsearch-cluster.enterprises.upmc.com"
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

	kibanaDeploymentName = "kibana"

	secretName = "es-certs"
)

// K8sutil defines the kube object
type K8sutil struct {
	Config     *rest.Config
	TprClient  *rest.RESTClient
	Kclient    kubernetes.Interface
	MasterHost string
}

// New creates a new instance of k8sutil
func New(kubeCfgFile, masterHost string) (*K8sutil, error) {

	client, tprclient, err := newKubeClient(kubeCfgFile)

	if err != nil {
		logrus.Fatalf("Could not init Kubernetes client! [%s]", err)
	}

	k := &K8sutil{
		Kclient:    client,
		TprClient:  tprclient,
		MasterHost: masterHost,
	}

	return k, nil
}

func buildConfig(kubeCfgFile string) (*rest.Config, error) {
	if kubeCfgFile != "" {
		logrus.Infof("Using OutOfCluster k8s config with kubeConfigFile: %s", kubeCfgFile)
		return clientcmd.BuildConfigFromFlags("", kubeCfgFile)
	}

	logrus.Info("Using InCluster k8s config")
	return rest.InClusterConfig()
}

func configureTPRClient(config *rest.Config) {
	groupversion := schema.GroupVersion{
		Group:   "enterprises.upmc.com",
		Version: "v1",
	}

	config.GroupVersion = &groupversion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}

	schemeBuilder := runtime.NewSchemeBuilder(
		func(scheme *runtime.Scheme) error {
			scheme.AddKnownTypes(
				schema.GroupVersion{Group: "enterprises.upmc.com", Version: "v1"},
				&myspec.ElasticsearchCluster{},
				&myspec.ElasticsearchClusterList{},
				&api.ListOptions{},
				&api.DeleteOptions{},
			)
			return nil
		})

	schemeBuilder.AddToScheme(api.Scheme)
}

func newKubeClient(kubeCfgFile string) (kubernetes.Interface, *rest.RESTClient, error) {

	// Create the client config. Use kubeconfig if given, otherwise assume in-cluster.
	Config, err := buildConfig(kubeCfgFile)
	if err != nil {
		panic(err)
	}

	client, err := kubernetes.NewForConfig(Config)
	if err != nil {
		panic(err)
	}

	// make a new config for our extension's API group, using the first config as a baseline
	var tprconfig *rest.Config
	tprconfig = Config

	configureTPRClient(tprconfig)

	tprclient, err := rest.RESTClientFor(tprconfig)
	if err != nil {
		logrus.Error(err.Error())
		logrus.Error("can not get client to TPR")
		os.Exit(2)
	}

	return client, tprclient, nil
}

// GetElasticSearchClusters returns a list of custom clusters defined
func (k *K8sutil) GetElasticSearchClusters() ([]myspec.ElasticsearchCluster, error) {
	// var resp *http.Response
	elasticSearchList := myspec.ElasticsearchClusterList{}
	var err error

	for {
		err = k.TprClient.Get().Resource("ElasticsearchClusters").Do().Into(&elasticSearchList)

		if err != nil {
			logrus.Error("error getting elasticsearch clusters")
			logrus.Error(err)
			time.Sleep(5 * time.Second)
			continue
		}
		break
	}

	return elasticSearchList.Items, nil
}

// MonitorElasticSearchEvents watches for new or removed clusters
func (k *K8sutil) MonitorElasticSearchEvents(stopchan chan struct{}) (<-chan *myspec.ElasticsearchCluster, <-chan error) {
	events := make(chan *myspec.ElasticsearchCluster)
	errc := make(chan error, 1)

	source := cache.NewListWatchFromClient(k.TprClient, "elasticsearchclusters", v1.NamespaceAll, fields.Everything())

	createAddHandler := func(obj interface{}) {
		event := obj.(*myspec.ElasticsearchCluster)
		event.Type = "ADDED"
		events <- event
	}

	createDeleteHandler := func(obj interface{}) {
		event := obj.(*myspec.ElasticsearchCluster)
		event.Type = "DELETED"
		events <- event
	}

	updateHandler := func(old interface{}, obj interface{}) {
		event := obj.(*myspec.ElasticsearchCluster)
		event.Type = "MODIFIED"
		events <- event
	}

	_, controller := cache.NewInformer(
		source,
		&myspec.ElasticsearchCluster{},
		time.Minute*60,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    createAddHandler,
			UpdateFunc: updateHandler,
			DeleteFunc: createDeleteHandler,
		})

	go controller.Run(stopchan)

	return events, errc
}

// MonitorDataPods watches for new or changed data node pods
func (k *K8sutil) MonitorDataPods(stopchan chan struct{}) (<-chan *v1.Pod, <-chan error) {
	events := make(chan *v1.Pod)
	errc := make(chan error, 1)

	// create the pod watcher
	podListWatcher := cache.NewListWatchFromClient(k.Kclient.Core().RESTClient(), "pods", v1.NamespaceAll, fields.Everything())

	createAddHandler := func(obj interface{}) {
		event := obj.(*v1.Pod)

		for k, v := range event.ObjectMeta.Labels {
			if k == "role" && v == "data" {
				events <- event
				break
			}
		}
	}

	updateHandler := func(old interface{}, obj interface{}) {
		event := obj.(*v1.Pod)
		for k, v := range event.ObjectMeta.Labels {
			if k == "role" && v == "data" {
				events <- event
				break
			}
		}
	}

	_, controller := cache.NewIndexerInformer(podListWatcher, &v1.Pod{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc:    createAddHandler,
		UpdateFunc: updateHandler,
		DeleteFunc: func(obj interface{}) {},
	}, cache.Indexers{})

	go controller.Run(stopchan)

	return events, errc
}

// CreateKubernetesThirdPartyResource checks if ElasticSearch TPR exists. If not, create
func (k *K8sutil) CreateKubernetesThirdPartyResource() error {

	tpr, err := k.Kclient.ExtensionsV1beta1().ThirdPartyResources().Get(tprName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			tpr := &v1beta1.ThirdPartyResource{
				ObjectMeta: metav1.ObjectMeta{
					Name: tprName,
				},
				Versions: []v1beta1.APIVersion{
					{Name: "v1"},
				},
				Description: "Managed elasticsearch clusters",
			}

			_, err := k.Kclient.ExtensionsV1beta1().ThirdPartyResources().Create(tpr)
			if err != nil {
				panic(err)
			}
			logrus.Info("Created missing TPR")
		} else {
			panic(err)
		}
	} else {
		logrus.Infof("SKIPPING: already exists %#v\n", tpr.ObjectMeta.Name)
	}

	return nil
}

// DeleteStatefulSet deletes the data statefulset
func (k *K8sutil) DeleteStatefulSet(deploymentType, clusterName, namespace string) error {

	labelSelector := ""
	if deploymentType == "data" {
		labelSelector = "component=elasticsearch" + "-" + clusterName + ",role=data"
	} else if deploymentType == "master" {
		labelSelector = "component=elasticsearch" + "-" + clusterName + ",role=master"
	}

	// Get list of data type statefulsets
	statefulsets, err := k.Kclient.AppsV1beta1().StatefulSets(namespace).List(metav1.ListOptions{LabelSelector: labelSelector})

	if err != nil {
		logrus.Error("Could not get stateful sets! ", err)
	}

	for _, statefulset := range statefulsets.Items {
		//Scale the statefulset down to zero (https://github.com/kubernetes/client-go/issues/91)
		statefulset.Spec.Replicas = new(int32)
		statefulset, err := k.Kclient.AppsV1beta1().StatefulSets(namespace).Update(&statefulset)

		if err != nil {
			logrus.Errorf("Could not scale statefulset: %s ", statefulset.Name)
		} else {
			logrus.Infof("Scaled statefulset: %s to zero", statefulset.Name)
		}

		err = k.Kclient.AppsV1beta1().StatefulSets(namespace).Delete(statefulset.Name, &metav1.DeleteOptions{
			PropagationPolicy: func() *metav1.DeletionPropagation {
				foreground := metav1.DeletePropagationForeground
				return &foreground
			}(),
		})

		if err != nil {
			logrus.Errorf("Could not delete statefulset: %s ", statefulset.Name)
		} else {
			logrus.Infof("Deleted statefulset: %s", statefulset.Name)
		}
	}

	return nil
}

func TemplateImagePullSecrets(ips []myspec.ImagePullSecrets) []v1.LocalObjectReference {
	var outSecrets []v1.LocalObjectReference

	for _, s := range ips {
		outSecrets = append(outSecrets, v1.LocalObjectReference{
			Name: s.Name,
		})
	}
	return outSecrets
}

// CreateDataNodeDeployment creates the data node deployment
func (k *K8sutil) CreateDataNodeDeployment(deploymentType string, replicas *int32, baseImage, storageClass string, dataDiskSize string, resources myspec.Resources,
	imagePullSecrets []myspec.ImagePullSecrets, clusterName, statsdEndpoint, networkHost, namespace, javaOptions string) error {

	var deploymentName, role, isNodeMaster, isNodeData string

	if deploymentType == "data" {
		deploymentName = fmt.Sprintf("%s-%s", dataDeploymentName, clusterName)
		isNodeMaster = "false"
		role = "data"
		isNodeData = "true"
	} else if deploymentType == "master" {
		deploymentName = fmt.Sprintf("%s-%s", masterDeploymentName, clusterName)
		isNodeMaster = "true"
		role = "master"
		isNodeData = "false"
	}

	component := fmt.Sprintf("elasticsearch-%s", clusterName)
	discoveryServiceNameCluster := fmt.Sprintf("%s-%s", discoveryServiceName, clusterName)
	statefulSetName := fmt.Sprintf("%s-%s", deploymentName, storageClass)

	// Check if StatefulSet exists
	statefulSet, err := k.Kclient.AppsV1beta1().StatefulSets(namespace).Get(statefulSetName, metav1.GetOptions{})

	if len(statefulSet.Name) == 0 {
		volumeSize, _ := resource.ParseQuantity(dataDiskSize)

		// Parse CPU / Memory
		// limitCPU, _ := resource.ParseQuantity(resources.Limits.CPU)
		// limitMemory, _ := resource.ParseQuantity(resources.Limits.Memory)
		requestCPU, _ := resource.ParseQuantity(resources.Requests.CPU)
		requestMemory, _ := resource.ParseQuantity(resources.Requests.Memory)

		logrus.Infof("StatefulSet %s not found, creating...", statefulSetName)

		statefulSet := &apps.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: statefulSetName,
				Labels: map[string]string{
					"component": component,
					"role":      role,
					"name":      statefulSetName,
				},
			},
			Spec: apps.StatefulSetSpec{
				Replicas:    replicas,
				ServiceName: statefulSetName,
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"component": component,
							"role":      role,
							"name":      statefulSetName,
						},
						Annotations: map[string]string{
							"pod.beta.kubernetes.io/init-containers": "[ { \"name\": \"sysctl\", \"image\": \"busybox\", \"imagePullPolicy\": \"IfNotPresent\", \"command\": [\"sysctl\", \"-w\", \"vm.max_map_count=262144\"], \"securityContext\": { \"privileged\": true } }]",
						},
					},
					Spec: v1.PodSpec{
						Affinity: &v1.Affinity{
							PodAntiAffinity: &v1.PodAntiAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []v1.WeightedPodAffinityTerm{
									{
										Weight: 100,
										PodAffinityTerm: v1.PodAffinityTerm{
											LabelSelector: &metav1.LabelSelector{
												MatchExpressions: []metav1.LabelSelectorRequirement{
													{
														Key:      "role",
														Operator: metav1.LabelSelectorOpIn,
														Values:   []string{role},
													},
												},
											},
											TopologyKey: "kubernetes.io/hostname",
										},
									},
								},
							}},
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
										Value: isNodeMaster,
									},
									v1.EnvVar{
										Name:  "NODE_DATA",
										Value: isNodeData,
									},
									v1.EnvVar{
										Name:  "HTTP_ENABLE",
										Value: "false",
									},
									v1.EnvVar{
										Name:  "ES_JAVA_OPTS",
										Value: javaOptions,
									},
									v1.EnvVar{
										Name:  "STATSD_HOST",
										Value: statsdEndpoint,
									},
									v1.EnvVar{
										Name:  "DISCOVERY_SERVICE",
										Value: discoveryServiceNameCluster,
									},
									v1.EnvVar{
										Name:  "NETWORK_HOST",
										Value: networkHost,
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
										Name:      fmt.Sprintf("%s-%s", secretName, clusterName),
										MountPath: "/elasticsearch/config/certs",
									},
								},
								Resources: v1.ResourceRequirements{
									// Limits: v1.ResourceList{
									// 	"cpu":    limitCPU,
									// 	"memory": limitMemory,
									// },
									Requests: v1.ResourceList{
										"cpu":    requestCPU,
										"memory": requestMemory,
									},
								},
							},
						},
						Volumes: []v1.Volume{
							v1.Volume{
								Name: fmt.Sprintf("%s-%s", secretName, clusterName),
								VolumeSource: v1.VolumeSource{
									Secret: &v1.SecretVolumeSource{
										SecretName: fmt.Sprintf("%s-%s", secretName, clusterName),
									},
								},
							},
						},
						ImagePullSecrets: TemplateImagePullSecrets(imagePullSecrets),
					},
				},
				VolumeClaimTemplates: []v1.PersistentVolumeClaim{
					v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "es-data",
							Annotations: map[string]string{
								"volume.beta.kubernetes.io/storage-class": storageClass,
							},
							Labels: map[string]string{
								"component": "elasticsearch",
								"role":      role,
								"name":      statefulSetName,
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

		_, err := k.Kclient.AppsV1beta1().StatefulSets(namespace).Create(statefulSet)

		if err != nil {
			logrus.Error("Could not create stateful set: ", err)
			return err
		}
	} else {
		if err != nil {
			logrus.Error("Could not get stateful set! ", err)
			return err
		}

		//scale replicas?
		if statefulSet.Spec.Replicas != replicas {
			statefulSet.Spec.Replicas = replicas

			_, err := k.Kclient.AppsV1beta1().StatefulSets(namespace).Update(statefulSet)

			if err != nil {
				logrus.Error("Could not scale statefulSet: ", err)
			}
		}
	}

	return nil
}
