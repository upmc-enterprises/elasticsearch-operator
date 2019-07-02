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

	"github.com/Sirupsen/logrus"
	myspec "github.com/upmc-enterprises/elasticsearch-operator/pkg/apis/elasticsearchoperator/v1"
	"k8s.io/api/apps/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	elasticsearchCertspath = "/elasticsearch/config/certs"
	clusterHealthURL       = "/_nodes/_local"
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
func (k *K8sutil) CreateClientDeployment(baseImage string, replicas *int32, javaOptions, clientJavaOptions string,
	resources myspec.Resources, imagePullSecrets []myspec.ImagePullSecrets, imagePullPolicy, serviceAccountName, clusterName, statsdEndpoint, networkHost, namespace string, useSSL *bool, affinity v1.Affinity, annotations map[string]string, nodeSelector map[string]string, tolerations []v1.Toleration) error {

	component := fmt.Sprintf("elasticsearch-%s", clusterName)
	discoveryServiceNameCluster := fmt.Sprintf("%s-%s", discoveryServiceName, clusterName)

	deploymentName := fmt.Sprintf("%s-%s", clientDeploymentName, clusterName)
	isNodeMaster := "false"
	role := "client"

	// Check if deployment exists
	deployment, err := k.Kclient.ExtensionsV1beta1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})

	enableSSL := "false"
	if useSSL != nil && *useSSL {
		enableSSL = "true"
	}

	// parse javaOptions and see if client nodes are using different options
	// if using the legacy (global) java-options, then this will be applied to all nodes that dont have custom settings
	esJavaOps := ""

	if clientJavaOptions != "" {
		esJavaOps = clientJavaOptions
	} else {
		esJavaOps = javaOptions
	}

	if len(deployment.Name) == 0 {
		logrus.Infof("Deployment %s not found, creating...", deploymentName)
		clusterSecretName := fmt.Sprintf("%s-%s", secretName, clusterName)

		// Parse CPU / Memory
		limitCPU, _ := resource.ParseQuantity(resources.Limits.CPU)
		limitMemory, _ := resource.ParseQuantity(resources.Limits.Memory)
		requestCPU, _ := resource.ParseQuantity(resources.Requests.CPU)
		requestMemory, _ := resource.ParseQuantity(resources.Requests.Memory)
		scheme := v1.URISchemeHTTP
		if useSSL != nil && *useSSL {
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
						Annotations: annotations,
					},
					Spec: v1.PodSpec{
						Tolerations:  tolerations,
						NodeSelector: nodeSelector,
						Affinity: &affinity,
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
								ImagePullPolicy: v1.PullPolicy(imagePullPolicy),
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
										Value: "false",
									},
									v1.EnvVar{
										Name:  "HTTP_ENABLE",
										Value: "true",
									},
									v1.EnvVar{
										Name:  "SEARCHGUARD_SSL_TRANSPORT_ENABLED",
										Value: enableSSL,
									},
									v1.EnvVar{
										Name:  "SEARCHGUARD_SSL_HTTP_ENABLED",
										Value: enableSSL,
									},
									v1.EnvVar{
										Name:  "ES_JAVA_OPTS",
										Value: esJavaOps,
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
								ReadinessProbe: probe,
								LivenessProbe:  probe,
								VolumeMounts: []v1.VolumeMount{
									v1.VolumeMount{
										Name:      "storage",
										MountPath: "/data",
									},
									v1.VolumeMount{
										Name:      clusterSecretName,
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
								Name: clusterSecretName,
								VolumeSource: v1.VolumeSource{
									Secret: &v1.SecretVolumeSource{
										SecretName: clusterSecretName,
									},
								},
							},
						},
						ImagePullSecrets: TemplateImagePullSecrets(imagePullSecrets),
					},
				},
			},
		}

		if useSSL != nil && !*useSSL {
			// Do not configure Volume and VolumeMount for certs
			volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
			for index, volumeMount := range volumeMounts {
				if volumeMount.Name == clusterSecretName {
					if index < (len(volumeMounts) - 1) {
						volumeMounts = append(volumeMounts[:index], volumeMounts[index+1:]...)
					} else {
						volumeMounts = volumeMounts[:index]
					}
					break
				}
			}
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

			volumes := deployment.Spec.Template.Spec.Volumes
			for index, volume := range volumes {
				if volume.Name == clusterSecretName {
					if index < (len(volumes) - 1) {
						volumes = append(volumes[:index], volumes[index+1:]...)
					} else {
						volumes = volumes[:index]
					}
					break
				}
			}
			deployment.Spec.Template.Spec.Volumes = volumes
		}

		if serviceAccountName != "" {
			deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
		}

		_, err := k.Kclient.AppsV1beta1().Deployments(namespace).Create(deployment)

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
func (k *K8sutil) CreateKibanaDeployment(baseImage, clusterName, namespace string, imagePullSecrets []myspec.ImagePullSecrets, imagePullPolicy string, serviceAccountName string, useSSL *bool) error {

	replicaCount := int32(1)

	component := fmt.Sprintf("elasticsearch-%s", clusterName)

	deploymentName := fmt.Sprintf("%s-%s", kibanaDeploymentName, clusterName)

	enableSSL := "true"
	scheme := v1.URISchemeHTTPS
	if useSSL != nil && !*useSSL {
		enableSSL = "false"
		scheme = v1.URISchemeHTTP
	}

	// Check if deployment exists
	deployment, err := k.Kclient.ExtensionsV1beta1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})
	probe := &v1.Probe{
		TimeoutSeconds:      30,
		InitialDelaySeconds: 1,
		FailureThreshold:    10,
		Handler: v1.Handler{
			HTTPGet: &v1.HTTPGetAction{
				Port:   intstr.FromInt(5601),
				Path:   "/", //TODO since kibana doesn't have a healthcheck url, the root is enough
				Scheme: scheme,
			},
		},
	}
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
								ImagePullPolicy: v1.PullPolicy(imagePullPolicy),
								Env: []v1.EnvVar{
									v1.EnvVar{
										Name:  "ELASTICSEARCH_URL",
										Value: GetESURL(component, useSSL),
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
								LivenessProbe:  probe,
								ReadinessProbe: probe,
							},
						},
						ImagePullSecrets: TemplateImagePullSecrets(imagePullSecrets),
					},
				},
			},
		}

		if *useSSL {
			// SSL config

			deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env,
				v1.EnvVar{
					Name:  "ELASTICSEARCH_SSL_CERTIFICATEAUTHORITIES",
					Value: fmt.Sprintf("%s/ca.pem", elasticsearchCertspath),
				},
				v1.EnvVar{
					Name:  "SERVER_SSL_ENABLED",
					Value: enableSSL,
				},
				v1.EnvVar{
					Name:  "SERVER_SSL_KEY",
					Value: fmt.Sprintf("%s/kibana-key.pem", elasticsearchCertspath),
				},
				v1.EnvVar{
					Name:  "SERVER_SSL_CERTIFICATE",
					Value: fmt.Sprintf("%s/kibana.pem", elasticsearchCertspath),
				},
			)

			// Certs volume
			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, v1.Volume{
				Name: fmt.Sprintf("%s-%s", secretName, clusterName),
				VolumeSource: v1.VolumeSource{
					Secret: &v1.SecretVolumeSource{
						SecretName: fmt.Sprintf("%s-%s", secretName, clusterName),
					},
				},
			})
			// Mount certs
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
				Name:      fmt.Sprintf("%s-%s", secretName, clusterName),
				MountPath: elasticsearchCertspath,
			})
		}

		if serviceAccountName != "" {
			deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
		}

		_, err := k.Kclient.AppsV1beta1().Deployments(namespace).Create(deployment)

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
func (k *K8sutil) CreateCerebroDeployment(baseImage, clusterName, namespace, cert string, imagePullSecrets []myspec.ImagePullSecrets, imagePullPolicy string, serviceAccountName string, useSSL *bool) error {
	replicaCount := int32(1)
	component := fmt.Sprintf("elasticsearch-%s", clusterName)
	deploymentName := fmt.Sprintf("%s-%s", cerebroDeploymentName, clusterName)

	// Check if deployment exists
	deployment, err := k.Kclient.AppsV1beta1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})
	probe := &v1.Probe{
		TimeoutSeconds:      30,
		InitialDelaySeconds: 1,
		FailureThreshold:    10,
		Handler: v1.Handler{
			HTTPGet: &v1.HTTPGetAction{
				Port:   intstr.FromInt(9000),
				Path:   "/#/connect", //TODO since cerebro doesn't have a healthcheck url, this path is enough
				Scheme: v1.URISchemeHTTP,
			},
		},
	}
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
							{
								Name:            deploymentName,
								Image:           baseImage,
								ImagePullPolicy: v1.PullPolicy(imagePullPolicy),
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
								ReadinessProbe: probe,
								LivenessProbe:  probe,
								VolumeMounts: []v1.VolumeMount{
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
						},
						ImagePullSecrets: TemplateImagePullSecrets(imagePullSecrets),
					},
				},
			},
		}

		if *useSSL {
			// Certs volume
			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes,
				v1.Volume{
					Name: fmt.Sprintf("%s-%s", secretName, clusterName),
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: fmt.Sprintf("%s-%s", secretName, clusterName),
						},
					},
				},
			)
			// Mount certs
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts,
				v1.VolumeMount{
					Name:      fmt.Sprintf("%s-%s", secretName, clusterName),
					MountPath: elasticsearchCertspath,
				},
			)
		}

		if serviceAccountName != "" {
			deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
		}

		if _, err := k.Kclient.AppsV1beta1().Deployments(namespace).Create(deployment); err != nil {
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
