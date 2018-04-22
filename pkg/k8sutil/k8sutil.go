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
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	elasticsearchoperator "github.com/upmc-enterprises/elasticsearch-operator/pkg/apis/elasticsearchoperator"
	elasticsearchoperatorv1 "github.com/upmc-enterprises/elasticsearch-operator/pkg/apis/elasticsearchoperator/v1"
	clientset "github.com/upmc-enterprises/elasticsearch-operator/pkg/client/clientset/versioned"
	genclient "github.com/upmc-enterprises/elasticsearch-operator/pkg/client/clientset/versioned"
	apps "k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	dataDir    = "/data"
	backupFile = "/var/elastic/latest.backup"

	discoveryServiceName = "elasticsearch-discovery"
	dataServiceName      = "es-data-svc"
	clientServiceName    = "elasticsearch"
	kibanaService        = "kibana"
	cerebroService       = "cerebro"

	clientDeploymentName = "es-client"
	masterDeploymentName = "es-master"
	dataDeploymentName   = "es-data"

	kibanaDeploymentName  = "kibana"
	cerebroDeploymentName = "cerebro"

	secretName = "es-certs"
)

var (
	initContainerClusterVersionMin = []int{1, 8}
	mgmtServices                   = map[string]int{"cerebro": 9000, "kibana": 5601}
)

// K8sutil defines the kube object
type K8sutil struct {
	Config     *rest.Config
	CrdClient  genclient.Interface
	Kclient    kubernetes.Interface
	KubeExt    apiextensionsclient.Interface
	K8sVersion []int
	MasterHost string
}

// New creates a new instance of k8sutil
func New(kubeCfgFile, masterHost string) (*K8sutil, error) {

	crdClient, kubeClient, kubeExt, k8sVersion, err := newKubeClient(kubeCfgFile)

	if err != nil {
		logrus.Fatalf("Could not init Kubernetes client! [%s]", err)
	}

	k := &K8sutil{
		Kclient:    kubeClient,
		MasterHost: masterHost,
		K8sVersion: k8sVersion,
		CrdClient:  crdClient,
		KubeExt:    kubeExt,
	}

	return k, nil
}

func buildConfig(kubeCfgFile string) (*rest.Config, error) {
	if kubeCfgFile != "" {
		logrus.Infof("Using OutOfCluster k8s config with kubeConfigFile: %s", kubeCfgFile)
		config, err := clientcmd.BuildConfigFromFlags("", kubeCfgFile)
		if err != nil {
			panic(err.Error())
		}

		return config, nil
	}

	logrus.Info("Using InCluster k8s config")
	return rest.InClusterConfig()
}

func newKubeClient(kubeCfgFile string) (genclient.Interface, kubernetes.Interface, apiextensionsclient.Interface, []int, error) {

	// Create the client config. Use kubeconfig if given, otherwise assume in-cluster.
	Config, err := buildConfig(kubeCfgFile)
	if err != nil {
		panic(err)
	}

	// Create the kubernetes client
	clientSet, err := clientset.NewForConfig(Config)
	if err != nil {
		panic(err)
	}

	kubeClient, err := kubernetes.NewForConfig(Config)
	if err != nil {
		panic(err)
	}

	kubeExtCli, err := apiextensionsclient.NewForConfig(Config)
	if err != nil {
		panic(err)
	}

	version, err := kubeClient.ServerVersion()
	if err != nil {
		logrus.Error("Could not get version from api server:", err)
	}

	majorVer, _ := strconv.Atoi(version.Major)
	minorVer, _ := strconv.Atoi(version.Minor)
	k8sVersion := []int{majorVer, minorVer}

	return clientSet, kubeClient, kubeExtCli, k8sVersion, nil
}

// CreateKubernetesCustomResourceDefinition checks if ElasticSearch CRD exists. If not, create
func (k *K8sutil) CreateKubernetesCustomResourceDefinition() error {

	crd, err := k.KubeExt.ApiextensionsV1beta1().CustomResourceDefinitions().Get(elasticsearchoperator.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			crdObject := &apiextensionsv1beta1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: elasticsearchoperator.Name,
				},
				Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
					Group:   elasticsearchoperator.GroupName,
					Version: elasticsearchoperator.Version,
					Scope:   apiextensionsv1beta1.NamespaceScoped,
					Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
						Plural: elasticsearchoperator.ResourcePlural,
						Kind:   elasticsearchoperator.ResourceKind,
					},
				},
			}

			_, err := k.KubeExt.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crdObject)
			if err != nil {
				panic(err)
			}
			logrus.Info("Created missing CRD...waiting for it to be established...")

			// wait for CRD being established
			err = wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
				createdCRD, err := k.KubeExt.ApiextensionsV1beta1().CustomResourceDefinitions().Get(elasticsearchoperator.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				for _, cond := range createdCRD.Status.Conditions {
					switch cond.Type {
					case apiextensionsv1beta1.Established:
						if cond.Status == apiextensionsv1beta1.ConditionTrue {
							return true, nil
						}
					case apiextensionsv1beta1.NamesAccepted:
						if cond.Status == apiextensionsv1beta1.ConditionFalse {
							return false, fmt.Errorf("Name conflict: %v", cond.Reason)
						}
					}
				}
				return false, nil
			})

			if err != nil {
				deleteErr := k.KubeExt.ApiextensionsV1beta1().CustomResourceDefinitions().Delete(elasticsearchoperator.Name, nil)
				if deleteErr != nil {
					return errors.NewAggregate([]error{err, deleteErr})
				}
				return err
			}

			logrus.Info("CRD ready!")
		} else {
			panic(err)
		}
	} else {
		logrus.Infof("SKIPPING: already exists %#v\n", crd.ObjectMeta.Name)
	}

	return nil
}

// MonitorElasticSearchEvents watches for new or removed clusters
func (k *K8sutil) MonitorElasticSearchEvents(stopchan chan struct{}) (<-chan *elasticsearchoperatorv1.ElasticsearchCluster, <-chan error) {
	events := make(chan *elasticsearchoperatorv1.ElasticsearchCluster)
	errc := make(chan error, 1)

	source := cache.NewListWatchFromClient(k.CrdClient.EnterprisesV1().RESTClient(), elasticsearchoperator.ResourcePlural, v1.NamespaceAll, fields.Everything())

	createAddHandler := func(obj interface{}) {
		event := obj.(*elasticsearchoperatorv1.ElasticsearchCluster)
		event.Type = "ADDED"
		events <- event
	}

	createDeleteHandler := func(obj interface{}) {
		event := obj.(*elasticsearchoperatorv1.ElasticsearchCluster)
		event.Type = "DELETED"
		events <- event
	}

	updateHandler := func(old interface{}, obj interface{}) {
		event := obj.(*elasticsearchoperatorv1.ElasticsearchCluster)
		event.Type = "MODIFIED"
		events <- event
	}

	_, controller := cache.NewInformer(
		source,
		&elasticsearchoperatorv1.ElasticsearchCluster{},
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

func TemplateImagePullSecrets(ips []elasticsearchoperatorv1.ImagePullSecrets) []v1.LocalObjectReference {
	var outSecrets []v1.LocalObjectReference

	for _, s := range ips {
		outSecrets = append(outSecrets, v1.LocalObjectReference{
			Name: s.Name,
		})
	}
	return outSecrets
}

// CreateDataNodeDeployment creates the data node deployment
func (k *K8sutil) CreateDataNodeDeployment(deploymentType, clusterName, namespace string, clusterSpec elasticsearchoperatorv1.ClusterSpec) error {

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
	statefulSetName := fmt.Sprintf("%s-%s", deploymentName, clusterSpec.Storage.StorageClass)

	//TODO remove
	// rReq, err := k.ParseRequirements(clusterSpec.DataSpec)
	// if err != nil {
	// 	return fmt.Errorf("Error parsing data node resources: %v", err)
	// }

	// Check if StatefulSet exists
	statefulSet, err := k.Kclient.AppsV1beta2().StatefulSets(namespace).Get(statefulSetName, metav1.GetOptions{})

	if len(statefulSet.Name) == 0 {
		volumeSize, error := resource.ParseQuantity(clusterSpec.DataSpec.DiskSize)
		if err != nil {
			return fmt.Errorf("Error parsing data node diskSize: %v", error)
		}

		logrus.Infof("StatefulSet %s not found, creating...", statefulSetName)
		scheme := v1.URISchemeHTTP
		if clusterSpec.UseSSL {
			scheme = v1.URISchemeHTTPS
		}
		probe := &v1.Probe{
			TimeoutSeconds:      30,
			InitialDelaySeconds: 10,
			FailureThreshold:    15,
			Handler: v1.Handler{
				HTTPGet: &v1.HTTPGetAction{
					Port:   intstr.FromInt(9200),
					Path:   clusterHealthURL,
					Scheme: scheme,
				},
			},
		}
		statefulSet := &apps.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: statefulSetName,
				Labels: map[string]string{
					"component": component,
					"role":      role,
					"name":      statefulSetName,
					"cluster":   clusterName,
				},
			},
			Spec: apps.StatefulSetSpec{
				Replicas:    &clusterSpec.DataSpec.Replicas,
				ServiceName: statefulSetName,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"component": component,
						"role":      role,
						"name":      statefulSetName,
						"cluster":   clusterName,
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"component": component,
							"role":      role,
							"name":      statefulSetName,
							"cluster":   clusterName,
						},
						Annotations: clusterSpec.ClientSpec.Annotations,
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
								Image:           clusterSpec.ClientSpec.Image,
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
										Value: clusterName,
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
										Value: "true",
									},
									v1.EnvVar{
										Name:  "ES_JAVA_OPTS",
										Value: clusterSpec.DataSpec.JavaOptions,
									},
									v1.EnvVar{
										Name:  "STATSD_HOST",
										Value: clusterSpec.Instrumentation.StatsdHost,
									},
									v1.EnvVar{
										Name:  "DISCOVERY_SERVICE",
										Value: discoveryServiceNameCluster,
									},
									v1.EnvVar{
										Name:  "NETWORK_HOST",
										Value: clusterSpec.DataSpec.NetworkHost,
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
								ReadinessProbe: probe,
								LivenessProbe:  probe,
								VolumeMounts: []v1.VolumeMount{
									v1.VolumeMount{
										Name:      "es-data",
										MountPath: "/data",
									},
									v1.VolumeMount{
										Name:      fmt.Sprintf("%s-%s", secretName, clusterName),
										MountPath: elasticsearchCertspath,
									},
								},
								Resources: clusterSpec.DataSpec.Resources,
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
						ImagePullSecrets: TemplateImagePullSecrets(clusterSpec.ImagePullSecrets),
					},
				},
				VolumeClaimTemplates: []v1.PersistentVolumeClaim{
					v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "es-data",
							Labels: map[string]string{
								"component": "elasticsearch",
								"role":      role,
								"name":      statefulSetName,
								"cluster":   clusterName,
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

		if clusterSpec.Storage.StorageClass != "default" {
			statefulSet.Spec.VolumeClaimTemplates[0].Annotations = map[string]string{
				"volume.beta.kubernetes.io/storage-class": clusterSpec.Storage.StorageClass,
			}
		}

		_, err := k.Kclient.AppsV1beta2().StatefulSets(namespace).Create(statefulSet)

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
		if statefulSet.Spec.Replicas != &clusterSpec.DataSpec.Replicas {
			statefulSet.Spec.Replicas = &clusterSpec.DataSpec.Replicas

			_, err := k.Kclient.AppsV1beta2().StatefulSets(namespace).Update(statefulSet)

			if err != nil {
				logrus.Error("Could not scale statefulSet: ", err)
			}
		}
	}

	return nil
}

func (k *K8sutil) CreateCerebroConfiguration(clusterName string) map[string]string {

	x := map[string]string{}
	x["application.conf"] = fmt.Sprintf(`
play.ws.ssl {
        trustManager = {
                stores = [
				{ type = "PEM", path = "%s/cerebro.pem" },
				{ path: %s/truststore.jks, type: "JKS" }
                ]
        }
}
//play.crypto.secret = "ki:s:[[@=Ag?QIW2jMwkY:eqvrJ]JqoJyi2axj3ZvOv^/KavOT4ViJSv?6YY4[N"
//play.http.secret.key = "ki:s:[[@=Ag?QIW2jMwkY:eqvrJ]JqoJyi2axj3ZvOv^/KavOT4ViJSv?6YY4[N"
secret = "ki:s:[[@=Ag?QIW2jMwkY:eqvrJ]JqoJyi2axj3ZvOv^/KavOT4ViJSv?6YY4[N"
# Application base path
basePath = "/"

# Defaults to RUNNING_PID at the root directory of the app.
# To avoid creating a PID file set this value to /dev/null
#pidfile.path = "/var/run/cerebro.pid"
pidfile.path=/dev/null

# Rest request history max size per user
rest.history.size = 50 // defaults to 50 if not specified

# Path of local database file
#data.path: "/var/lib/cerebro/cerebro.db"
data.path = "./cerebro.db"
hosts = [
{
	host = "%s"
	name = "es-servers"
}
]
		`, elasticsearchCertspath, elasticsearchCertspath, fmt.Sprintf("https://%s:9200",
		fmt.Sprintf(fmt.Sprintf("elasticsearch-%s", clusterName))))
	return x
}
