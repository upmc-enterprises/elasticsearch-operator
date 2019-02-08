/*
Copyright (c) 2018, UPMC Enterprises
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
	"github.com/Sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	esOperatorSysctlName = "elasticsearch-operator-sysctl"
)

// CreateNodeInitDaemonset creates the node init daemonset
func (k *K8sutil) CreateNodeInitDaemonset() error {

	ds, err := k.Kclient.ExtensionsV1beta1().DaemonSets(k.InitDaemonsetNamespace).Get(esOperatorSysctlName, metav1.GetOptions{})

	if err != nil && len(ds.Name) == 0 {

		logrus.Infof("Daemonset %s/%s not found, creating...", ds.Namespace, ds.Name)

		resourceCPU, _ := resource.ParseQuantity("10m")
		resourceMemory, _ := resource.ParseQuantity("50Mi")

		daemonset := &v1beta1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: esOperatorSysctlName,
				Labels: map[string]string{
					"k8s-app": "elasticsearch-operator",
				},
				Namespace: k.InitDaemonsetNamespace,
			},
			Spec: v1beta1.DaemonSetSpec{
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"k8s-app": "elasticsearch-operator",
						},
					},
					Spec: v1.PodSpec{
						NodeSelector: map[string]string{
							"beta.kubernetes.io/os": "linux",
						},
						Containers: []v1.Container{
							v1.Container{
								Name:  "sysctl-conf",
								Image: k.BusyboxImage,
								Command: []string{
									"sh",
									"-c",
									"sysctl -w vm.max_map_count=262166 && while true; do sleep 86400; done",
								},
								Resources: v1.ResourceRequirements{
									Limits: v1.ResourceList{
										"cpu":    resourceCPU,
										"memory": resourceMemory,
									},
									Requests: v1.ResourceList{
										"cpu":    resourceCPU,
										"memory": resourceMemory,
									},
								},
								SecurityContext: &v1.SecurityContext{
									Privileged: &[]bool{true}[0],
								},
							},
						},
						HostPID: true,
					},
				},
			},
		}

		_, err = k.Kclient.ExtensionsV1beta1().DaemonSets(k.InitDaemonsetNamespace).Create(daemonset)

	} else {
		logrus.Infof("Daemonset %s/%s already exist, skipping creation ...", ds.Namespace, ds.Name)
	}

	return err
}
