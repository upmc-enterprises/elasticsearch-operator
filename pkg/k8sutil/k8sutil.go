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
	storage "k8s.io/client-go/pkg/apis/storage/v1beta1"
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

// DeleteServices creates the discovery service
func (k *K8sutil) DeleteServices(clusterName, namespace string) {

	fullDiscoveryServiceName := fmt.Sprintf("%s-%s", discoveryServiceName, clusterName)
	err := k.Kclient.CoreV1().Services(namespace).Delete(fullDiscoveryServiceName, &metav1.DeleteOptions{})
	if err != nil {
		logrus.Error("Could not delete service "+fullDiscoveryServiceName+":", err)
	} else {
		logrus.Infof("Delete service: %s", fullDiscoveryServiceName)
	}

	fullDataServiceName := dataServiceName + "-" + clusterName
	err = k.Kclient.CoreV1().Services(namespace).Delete(fullDataServiceName, &metav1.DeleteOptions{})
	if err != nil {
		logrus.Error("Could not delete service "+fullDataServiceName+":", err)
	} else {
		logrus.Infof("Delete service: %s", fullDataServiceName)
	}

	fullClientServiceName := clientServiceName + "-" + clusterName
	err = k.Kclient.CoreV1().Services(namespace).Delete(fullClientServiceName, &metav1.DeleteOptions{})
	if err != nil {
		logrus.Error("Could not delete service "+fullClientServiceName+":", err)
	} else {
		logrus.Infof("Delete service: %s", fullClientServiceName)
	}

}

// CreateDiscoveryService creates the discovery service
func (k *K8sutil) CreateDiscoveryService(clusterName, namespace string) error {

	fullDiscoveryServiceName := fmt.Sprintf("%s-%s", discoveryServiceName, clusterName)
	component := "elasticsearch" + "-" + clusterName
	// Check if service exists
	svc, err := k.Kclient.CoreV1().Services(namespace).Get(fullDiscoveryServiceName, metav1.GetOptions{})

	// Service missing, create
	if len(svc.Name) == 0 {
		logrus.Info("Discovery Service not found, creating...")

		discoverySvc := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: fullDiscoveryServiceName,
				Labels: map[string]string{
					"component": component,
					"role":      "master",
				},
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"component": component,
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

		_, err := k.Kclient.CoreV1().Services(namespace).Create(discoverySvc)

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
func (k *K8sutil) CreateDataService(clusterName, namespace string) error {
	fullDataServiceName := dataServiceName + "-" + clusterName
	component := "elasticsearch" + "-" + clusterName
	// Check if service exists
	svc, err := k.Kclient.CoreV1().Services(namespace).Get(fullDataServiceName, metav1.GetOptions{})

	// Service missing, create
	if len(svc.Name) == 0 {
		logrus.Infof("%s not found, creating...", fullDataServiceName)

		dataService := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: fullDataServiceName,
				Labels: map[string]string{
					"service.alpha.kubernetes.io/tolerate-unready-endpoints": "true",
				},
				Annotations: map[string]string{
					"component": component,
					"name":      fullDataServiceName,
				},
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"component": component,
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

		_, err := k.Kclient.CoreV1().Services(namespace).Create(dataService)

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

// GetClientServiceNameFullDNS return the full DNS name of the client service
func (k *K8sutil) GetClientServiceNameFullDNS(clusterName, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", k.GetClientServiceName(clusterName), namespace)
}

// GetClientServiceName return the name of the client service
func (k *K8sutil) GetClientServiceName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clientServiceName, clusterName)
}

// CreateClientService creates the client service
func (k *K8sutil) CreateClientService(clusterName, namespace string, nodePort int32) error {

	fullClientServiceName := k.GetClientServiceName(clusterName)
	component := "elasticsearch" + "-" + clusterName
	// Check if service exists
	svc, err := k.Kclient.CoreV1().Services(namespace).Get(fullClientServiceName, metav1.GetOptions{})

	// Service missing, create
	if len(svc.Name) == 0 {
		logrus.Infof("%s not found, creating...", fullClientServiceName)

		clientSvc := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: fullClientServiceName,
				Labels: map[string]string{
					"component": component,
					"role":      "client",
				},
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"component": component,
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
		if nodePort > 0 {
			clientSvc.Spec.Type = v1.ServiceTypeNodePort
			clientSvc.Spec.Ports[0].NodePort = nodePort
		}

		_, err := k.Kclient.CoreV1().Services(namespace).Create(clientSvc)

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

// DeleteClientMasterDeployment deletes the client or master deployment
func (k *K8sutil) DeleteClientMasterDeployment(deploymentType, clusterName, namespace string) error {

	labelSelector := ""

	if deploymentType == "client" {
		labelSelector = "component=elasticsearch" + "-" + clusterName + ",role=client"
	} else if deploymentType == "master" {
		labelSelector = "component=elasticsearch" + "-" + clusterName + ",role=master"
	}

	// Get list of deployments
	deployments, err := k.Kclient.ExtensionsV1beta1().Deployments(namespace).List(metav1.ListOptions{LabelSelector: labelSelector})

	if err != nil {
		logrus.Error("Could not get deployments! ", err)
	}

	for _, deployment := range deployments.Items {
		//Scale the deployment down to zero (https://github.com/kubernetes/client-go/issues/91)
		deployment.Spec.Replicas = new(int32)
		deployment, err := k.Kclient.ExtensionsV1beta1().Deployments(namespace).Update(&deployment)

		if err != nil {
			logrus.Errorf("Could not scale deployment: %s ", deployment.Name)
		} else {
			logrus.Infof("Scaled deployment: %s to zero", deployment.Name)
		}

		err = k.Kclient.ExtensionsV1beta1().Deployments(namespace).Delete(deployment.Name, &metav1.DeleteOptions{})

		if err != nil {
			logrus.Errorf("Could not delete deployments: %s ", deployment.Name)
		} else {
			logrus.Infof("Deleted deployment: %s", deployment.Name)
		}
	}

	// Get list of ReplicaSets
	replicaSets, err := k.Kclient.ExtensionsV1beta1().ReplicaSets(namespace).List(metav1.ListOptions{LabelSelector: labelSelector})

	if err != nil {
		logrus.Error("Could not get replica sets! ", err)
	}

	for _, replicaSet := range replicaSets.Items {
		err := k.Kclient.ExtensionsV1beta1().ReplicaSets(namespace).Delete(replicaSet.Name, &metav1.DeleteOptions{})

		if err != nil {
			logrus.Errorf("Could not delete replica sets: %s ", replicaSet.Name)
		} else {
			logrus.Infof("Deleted replica set: %s", replicaSet.Name)
		}
	}

	return nil
}

// DeleteStatefulSet deletes the data statefulset
func (k *K8sutil) DeleteStatefulSet(clusterName, namespace string) error {

	// Get list of deployments
	statefulsets, err := k.Kclient.AppsV1beta1().StatefulSets(namespace).List(metav1.ListOptions{LabelSelector: "component=elasticsearch" + "-" + clusterName + ",role=data"})

	if err != nil {
		logrus.Error("Could not get stateful sets! ", err)
	}

	for _, statefulset := range statefulsets.Items {
		//Scale the deployment down to zero (https://github.com/kubernetes/client-go/issues/91)
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

// CreateClientMasterDeployment creates the client or master deployment
func (k *K8sutil) CreateClientMasterDeployment(deploymentType, baseImage string, replicas *int32, javaOptions string,
	resources myspec.Resources, imagePullSecrets []myspec.ImagePullSecrets, clusterName, statsdEndpoint, networkHost, namespace string) error {

	component := fmt.Sprintf("elasticsearch-%s", clusterName)
	discoveryServiceNameCluster := fmt.Sprintf("%s-%s", discoveryServiceName, clusterName)

	var deploymentName, role, isNodeMaster, httpEnable string

	if deploymentType == "client" {
		httpEnable = "true"
		deploymentName = clientDeploymentName + "-" + clusterName
		isNodeMaster = "false"
		role = "client"
	} else if deploymentType == "master" {
		httpEnable = "false"
		deploymentName = masterDeploymentName + "-" + clusterName
		isNodeMaster = "true"
		role = "master"
	}

	// Check if deployment exists
	deployment, err := k.Kclient.ExtensionsV1beta1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})

	if len(deployment.Name) == 0 {
		logrus.Infof("%s not found, creating...", deploymentName)

		// Parse CPU / Memory
		limitCPU, _ := resource.ParseQuantity(resources.Limits.CPU)
		limitMemory, _ := resource.ParseQuantity(resources.Limits.Memory)
		requestCPU, _ := resource.ParseQuantity(resources.Requests.CPU)
		requestMemory, _ := resource.ParseQuantity(resources.Requests.Memory)

		deployment := &v1beta1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: deploymentName,
				Labels: map[string]string{
					"component": component,
					"role":      role,
					"name":      deploymentName,
				},
			},
			Spec: v1beta1.DeploymentSpec{
				Replicas: replicas,
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"component": component,
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
								Resources: v1.ResourceRequirements{
									Limits: v1.ResourceList{
										"cpu":    limitCPU,
										"memory": limitMemory,
									},
									Requests: v1.ResourceList{
										"cpu":    requestCPU,
										"memory": requestMemory,
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
						ImagePullSecrets: TemplateImagePullSecrets(imagePullSecrets),
					},
				},
			},
		}

		_, err := k.Kclient.ExtensionsV1beta1().Deployments(namespace).Create(deployment)

		if err != nil {
			logrus.Error("Could not create client deployment: ", err)
			return err
		}
	} else {
		if err != nil {
			logrus.Error("Could not get client deployment! ", err)
			return err
		}

		//scale replicas?
		if deployment.Spec.Replicas != replicas {
			deployment.Spec.Replicas = replicas

			_, err := k.Kclient.ExtensionsV1beta1().Deployments(namespace).Update(deployment)

			if err != nil {
				logrus.Error("Could not scale deployment: ", err)
			}
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
func (k *K8sutil) CreateDataNodeDeployment(replicas *int32, baseImage, storageClass string, dataDiskSize string, resources myspec.Resources,
	imagePullSecrets []myspec.ImagePullSecrets, clusterName, statsdEndpoint, networkHost, namespace string) error {

	fullDataDeploymentName := fmt.Sprintf("%s-%s", dataDeploymentName, clusterName)
	component := fmt.Sprintf("elasticsearch-%s", clusterName)
	discoveryServiceNameCluster := fmt.Sprintf("%s-%s", discoveryServiceName, clusterName)
	statefulSetName := fmt.Sprintf("%s-%s", fullDataDeploymentName, storageClass)

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
					"role":      "data",
					"name":      statefulSetName,
				},
			},
			Spec: apps.StatefulSetSpec{
				Replicas:    replicas,
				ServiceName: "es-data-svc" + "-" + clusterName,
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"component": component,
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
										Name:      "es-certs",
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
								Name: "es-certs",
								VolumeSource: v1.VolumeSource{
									Secret: &v1.SecretVolumeSource{
										SecretName: "es-certs",
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
								"role":      "data",
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
			logrus.Error("Could not create data stateful set: ", err)
			return err
		}
	} else {
		if err != nil {
			logrus.Error("Could not get data stateful set! ", err)
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

// CreateStorageClass creates a storage class
// NOTE: Right now only creating AWS EBS volumes type gp2
func (k *K8sutil) CreateStorageClass(zone, storageClassProvisioner, storageType string, clusterName string) error {

	component := "elasticsearch" + "-" + clusterName
	// Check if storage class exists
	storageClass, err := k.Kclient.StorageV1beta1().StorageClasses().Get(zone, metav1.GetOptions{})

	if len(storageClass.Name) == 0 {
		logrus.Infof("StorgeClass %s not found, creating...", zone)

		class := &storage.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: zone,
				Labels: map[string]string{
					"component": component,
				},
			},
			Provisioner: storageClassProvisioner,
			Parameters: map[string]string{
				"type": storageType,
			},
		}

		if zone != "es-default" {
			class.Parameters["zone"] = zone
		}

		_, err := k.Kclient.StorageV1beta1().StorageClasses().Create(class)

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

// DeleteStorageClasses removes storage classes tied to the operator
func (k *K8sutil) DeleteStorageClasses(clusterName string) error {
	component := "elasticsearch" + "-" + clusterName
	err := k.Kclient.StorageV1beta1().StorageClasses().DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: component})

	if err != nil {
		logrus.Error("Could not delete storageclasses: ", err)
	} else {
		logrus.Info("Deleted storageclasses")
	}

	return nil
}

// UpdateVolumeReclaimPolicy updates the policy of the volume after it's created:
// See: https://github.com/kubernetes/kubernetes/issues/38192
func (k *K8sutil) UpdateVolumeReclaimPolicy(policy, namespace string) {

	var policyType v1.PersistentVolumeReclaimPolicy

	switch policy {
	case "Delete":
		policyType = v1.PersistentVolumeReclaimDelete
		break
	case "Retain":
		policyType = v1.PersistentVolumeReclaimRetain
		break
	}

	pvc, err := k.Kclient.CoreV1().PersistentVolumeClaims(namespace).List(metav1.ListOptions{
		LabelSelector: "role=data",
	})

	if err != nil {
		logrus.Error("Could not get pvc! ", err)
		return
	}

	for _, v := range pvc.Items {
		pv, err := k.Kclient.CoreV1().PersistentVolumes().Get(v.Spec.VolumeName, metav1.GetOptions{})

		if err != nil {
			logrus.Error("Could not get pv! ", err)
			continue
		}

		// Set the policy
		pv.Spec.PersistentVolumeReclaimPolicy = policyType

		_, err = k.Kclient.CoreV1().PersistentVolumes().Update(pv)

		if err != nil {
			logrus.Error("Could not update pv! ", err)
			continue
		}

	}
}
