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

	"github.com/Sirupsen/logrus"
	myspec "github.com/upmc-enterprises/elasticsearch-operator/pkg/spec"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

const (
	elasticsearchCertspath = "/elasticsearch/config/certs"
)

// TODO just mount the secret needed by each deployment
// DeleteDeployment deletes a deployment
func (k *K8sutil) DeleteDeployment(clusterName, namespace, deploymentType string) error {

	labelSelector := fmt.Sprintf("component=elasticsearch-%s,role=%s", clusterName, deploymentType)

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

// CreateClientDeployment creates the client deployment
func (k *K8sutil) CreateClientDeployment(baseImage string, replicas *int32, javaOptions string, enableSSL bool,
	resources myspec.Resources, imagePullSecrets []myspec.ImagePullSecrets, clusterName, statsdEndpoint, networkHost, namespace string) error {

	component := fmt.Sprintf("elasticsearch-%s", clusterName)
	discoveryServiceNameCluster := fmt.Sprintf("%s-%s", discoveryServiceName, clusterName)

	deploymentName := fmt.Sprintf("%s-%s", clientDeploymentName, clusterName)
	isNodeMaster := "false"
	role := "client"

	// Check if deployment exists
	deployment, err := k.Kclient.ExtensionsV1beta1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})

	if len(deployment.Name) == 0 {
		logrus.Infof("Deployment %s not found, creating...", deploymentName)

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
					"cluster":   clusterName,
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
							"cluster":   clusterName,
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
										Value: "true",
									},
									v1.EnvVar{
										Name:  "SEARCHGUARD_SSL_TRANSPORT_ENABLED",
										Value: strconv.FormatBool(enableSSL),
									},
									v1.EnvVar{
										Name:  "SEARCHGUARD_SSL_HTTP_ENABLED",
										Value: strconv.FormatBool(enableSSL),
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
										Name:      fmt.Sprintf("%s-%s", secretName, clusterName),
										MountPath: elasticsearchCertspath,
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

			if _, err := k.Kclient.ExtensionsV1beta1().Deployments(namespace).Update(deployment); err != nil {
				logrus.Error("Could not scale deployment: ", err)
				return err
			}
		}
	}

	return nil
}

// CreateKibanaDeployment creates a deployment of Kibana
func (k *K8sutil) CreateKibanaDeployment(baseImage, clusterName, namespace string, enableSSL bool, imagePullSecrets []myspec.ImagePullSecrets) error {

	replicaCount := int32(1)

	component := fmt.Sprintf("elasticsearch-%s", clusterName)

	deploymentName := fmt.Sprintf("%s-%s", kibanaDeploymentName, clusterName)

	// Check if deployment exists
	deployment, err := k.Kclient.ExtensionsV1beta1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})

	if len(deployment.Name) == 0 {
		logrus.Infof("%s not found, creating...", deploymentName)

		deployment := &v1beta1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: deploymentName,
				Labels: map[string]string{
					"component": component,
					"role":      "kibana",
					"name":      deploymentName,
				},
			},
			Spec: v1beta1.DeploymentSpec{
				Replicas: &replicaCount,
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"component": component,
							"role":      "kibana",
							"name":      deploymentName,
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							v1.Container{
								Name:            deploymentName,
								Image:           baseImage,
								ImagePullPolicy: "Always",
								Env: []v1.EnvVar{
									v1.EnvVar{
										Name:  "ELASTICSEARCH_URL",
										Value: GetESURL(component, enableSSL),
									},
									v1.EnvVar{
										Name:  "ELASTICSEARCH_SSL_CERTIFICATEAUTHORITIES",
										Value: fmt.Sprintf("%s/ca.pem", elasticsearchCertspath),
									},
									v1.EnvVar{
										Name:  "SERVER_SSL_ENABLED",
										Value: strconv.FormatBool(enableSSL),
									},
									v1.EnvVar{
										Name:  "SERVER_SSL_KEY",
										Value: fmt.Sprintf("%s/kibana-key.pem", elasticsearchCertspath),
									},
									v1.EnvVar{
										Name:  "SERVER_SSL_CERTIFICATE",
										Value: fmt.Sprintf("%s/kibana.pem", elasticsearchCertspath),
									},
									v1.EnvVar{
										Name:  "NODE_DATA",
										Value: "false",
									},
								},
								Ports: []v1.ContainerPort{
									v1.ContainerPort{
										Name:          "http",
										ContainerPort: 5601,
										Protocol:      v1.ProtocolTCP,
									},
								},
								VolumeMounts: []v1.VolumeMount{
									v1.VolumeMount{
										Name:      fmt.Sprintf("%s-%s", secretName, clusterName),
										MountPath: elasticsearchCertspath,
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
			},
		}

		_, err := k.Kclient.ExtensionsV1beta1().Deployments(namespace).Create(deployment)

		if err != nil {
			logrus.Error("Could not create kibana deployment: ", err)
			return err
		}
	} else {
		if err != nil {
			logrus.Error("Could not get kibana deployment! ", err)
			return err
		}
	}

	return nil
}

// CreateCerebroDeployment creates a deployment of Cerebro
func (k *K8sutil) CreateCerebroDeployment(baseImage, clusterName, namespace, cert string, enableSSL bool, imagePullSecrets []myspec.ImagePullSecrets) error {
	replicaCount := int32(1)
	component := fmt.Sprintf("elasticsearch-%s", clusterName)
	deploymentName := fmt.Sprintf("%s-%s", cerebroDeploymentName, clusterName)

	// Check if deployment exists
	deployment, err := k.Kclient.ExtensionsV1beta1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})

	if len(deployment.Name) == 0 {
		logrus.Infof("Deployment %s not found, creating...", deploymentName)

		deployment := &v1beta1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: deploymentName,
				Labels: map[string]string{
					"component": component,
					"role":      "cerebro",
					"name":      deploymentName,
				},
			},
			Spec: v1beta1.DeploymentSpec{
				Replicas: &replicaCount,
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"component": component,
							"role":      "cerebro",
							"name":      deploymentName,
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							v1.Container{
								Name:            deploymentName,
								Image:           baseImage,
								ImagePullPolicy: "Always",
								Command: []string{
									"bin/cerebro",
									"-Dconfig.file=/usr/local/cerebro/cfg/application.conf",
								},
								Ports: []v1.ContainerPort{
									v1.ContainerPort{
										Name:          "http",
										ContainerPort: 9000,
										Protocol:      v1.ProtocolTCP,
									},
								},
								VolumeMounts: []v1.VolumeMount{
									v1.VolumeMount{
										Name:      fmt.Sprintf("%s-%s", secretName, clusterName),
										MountPath: elasticsearchCertspath,
									},
									v1.VolumeMount{
										Name:      cert,
										MountPath: "/usr/local/cerebro/cfg",
									},
								},
							},
						},
						Volumes: []v1.Volume{
							v1.Volume{
								Name: cert,
								VolumeSource: v1.VolumeSource{
									ConfigMap: &v1.ConfigMapVolumeSource{
										LocalObjectReference: v1.LocalObjectReference{
											Name: cert,
										},
									},
								},
							},
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
			},
		}

		if _, err := k.Kclient.ExtensionsV1beta1().Deployments(namespace).Create(deployment); err != nil {
			logrus.Error("Could not create cerebro deployment: ", err)
			return err
		}
	} else {
		if err != nil {
			logrus.Error("Could not get cerebro deployment! ", err)
			return err
		}
	}

	return nil
}
