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

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// CreateDataService creates the data service
func (k *K8sutil) CreateDataService(clusterName, namespace string) error {
	fullDataServiceName := dataServiceName + "-" + clusterName
	component := "elasticsearch" + "-" + clusterName
	// Check if service exists
	svc, err := k.Kclient.CoreV1().Services(namespace).Get(fullDataServiceName, metav1.GetOptions{})

	// Service missing, create
	if len(svc.Name) == 0 {
		logrus.Infof("Service %s not found, creating...", fullDataServiceName)

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

		if _, err := k.Kclient.CoreV1().Services(namespace).Create(dataService); err != nil {
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
	component := fmt.Sprintf("elasticsearch-%s", clusterName)
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

		if _, err := k.Kclient.CoreV1().Services(namespace).Create(clientSvc); err != nil {
			logrus.Error("Could not create client service", err)
			return err
		}

	} else if err != nil {
		logrus.Error("Could not get client service! ", err)
		return err
	}

	return nil
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

		if _, err := k.Kclient.CoreV1().Services(namespace).Create(discoverySvc); err != nil {
			logrus.Error("Could not create discovery service! ", err)
			return err
		}

	} else if err != nil {
		logrus.Error("Could not get discovery service! ", err)
		return err
	}

	return nil
}

// CreateMgmtServices creates service for kibana or cerebro
func (k *K8sutil) CreateMgmtService(service string, clusterName, namespace string) error {

	port := mgmtServices[service]
	serviceName := fmt.Sprintf("%s-%s", service, clusterName)

	// Check if service exists
	s, err := k.Kclient.CoreV1().Services(namespace).Get(serviceName, metav1.GetOptions{})

	// Service missing, create
	if len(s.Name) == 0 {
		logrus.Info(fmt.Sprintf("%v not found, creating...", serviceName))

		svc := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: serviceName,
				Labels: map[string]string{
					"role": service,
				},
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"role": service,
				},
				Ports: []v1.ServicePort{
					v1.ServicePort{
						Name:       "http",
						Port:       80,
						Protocol:   "TCP",
						TargetPort: intstr.FromInt(port),
					},
				},
			},
		}

		if _, err := k.Kclient.CoreV1().Services(namespace).Create(svc); err != nil {
			logrus.Errorf("Could not create service %v %v! ", serviceName, err)
			return err
		}

	} else if err != nil {
		logrus.Errorf("Could not get service %v %v! ", serviceName, err)
		return err
	}
	return nil
}

// DeleteServices creates the discovery service
func (k *K8sutil) DeleteServices(clusterName, namespace string) error {

	fullDiscoveryServiceName := fmt.Sprintf("%s-%s", discoveryServiceName, clusterName)
	if err := k.Kclient.CoreV1().Services(namespace).Delete(fullDiscoveryServiceName, &metav1.DeleteOptions{}); err != nil {
		logrus.Error("Could not delete service "+fullDiscoveryServiceName+":", err)
	}
	logrus.Infof("Deleted service: %s", fullDiscoveryServiceName)

	fullDataServiceName := dataServiceName + "-" + clusterName
	if err := k.Kclient.CoreV1().Services(namespace).Delete(fullDataServiceName, &metav1.DeleteOptions{}); err != nil {
		logrus.Error("Could not delete service "+fullDataServiceName+":", err)
	}
	logrus.Infof("Deleted service: %s", fullDataServiceName)

	fullClientServiceName := clientServiceName + "-" + clusterName
	if err := k.Kclient.CoreV1().Services(namespace).Delete(fullClientServiceName, &metav1.DeleteOptions{}); err != nil {
		logrus.Error("Could not delete service "+fullClientServiceName+":", err)
	}
	logrus.Infof("Deleted service: %s", fullClientServiceName)

	for component, _ := range mgmtServices {

		// Check if service exists
		s, _ := k.Kclient.CoreV1().Services(namespace).Get(component, metav1.GetOptions{})

		// Service exists, delete
		if len(s.Name) >= 1 {
			fullClientServiceName := fmt.Sprintf("%s-%s", component, clusterName)

			if err := k.Kclient.CoreV1().Services(namespace).Delete(fullClientServiceName, &metav1.DeleteOptions{}); err != nil {
				logrus.Error("Could not delete service "+fullClientServiceName+":", err)
			}
			logrus.Infof("Deleted service: %s", fullClientServiceName)
		}

	}

	return nil
}
