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
	"github.com/Sirupsen/logrus"
	"k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateStorageClass creates a storage class
// NOTE: Right now only creating AWS EBS volumes type gp2
func (k *K8sutil) CreateStorageClass(zone, storageClassProvisioner, storageType string, clusterName string, useEncryption string) error {

	component := "elasticsearch" + "-" + clusterName
	// Check if storage class exists
	storageClass, err := k.Kclient.StorageV1beta1().StorageClasses().Get(zone, metav1.GetOptions{})

	// Default encryption to true
	if useEncryption == "" {
		useEncryption = "true"
	}

	if len(storageClass.Name) == 0 {
		logrus.Infof("StorageClass %s not found, creating...", zone)

		class := &storage.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: zone,
				Labels: map[string]string{
					"component": component,
					"cluster":   clusterName,
				},
			},
			Provisioner: storageClassProvisioner,
			Parameters: map[string]string{
				"type":      storageType,
				"encrypted": useEncryption,
			},
		}

		if zone != "es-default" {
			class.Parameters["zone"] = zone
		}

		_, err := k.Kclient.Storage().StorageClasses().Create(class)

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
	if err := k.Kclient.StorageV1beta1().StorageClasses().DeleteCollection(&metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: component}); err != nil {
		logrus.Error("Could not delete storageclasses: ", err)
	}
	logrus.Info("Deleted storageclasses")

	return nil
}

// UpdateVolumeReclaimPolicy updates the policy of the volume after it's created:
// See: https://github.com/kubernetes/kubernetes/issues/38192
func (k *K8sutil) UpdateVolumeReclaimPolicy(policy, namespace string, name string) {

	var policyType v1.PersistentVolumeReclaimPolicy

	switch policy {
	case "Delete":
		policyType = v1.PersistentVolumeReclaimDelete
		break
	case "Retain":
		policyType = v1.PersistentVolumeReclaimRetain
		break
	}

	// pvc, err := k.Kclient.CoreV1().PersistentVolumeClaims(namespace).List(metav1.ListOptions{
	// 	LabelSelector: "component=elasticsearch-example-es-cluster",
	// })
	pvc, err := k.Kclient.CoreV1().PersistentVolumeClaims(namespace).List(metav1.ListOptions{
		LabelSelector: "component=elasticsearch-" + name,
	})

	if err != nil {
		return
	}

	for _, v := range pvc.Items {
		pv, err := k.Kclient.CoreV1().PersistentVolumes().Get(v.Spec.VolumeName, metav1.GetOptions{})

		if err != nil {
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
