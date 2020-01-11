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
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateConfigMap creates a new configMap
func (k *K8sutil) CreateConfigMap(namespace string, name string, data map[string]string) error {

	cf := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: data,
	}

	if _, err := k.Kclient.CoreV1().ConfigMaps(namespace).Create(cf); err != nil {
		logrus.Error(fmt.Sprintf("Could not create configmap %s:", name), err)
		return err
	}

	return nil

}

// ConfigmapExists returns true if configmap exists
func (k *K8sutil) ConfigmapExists(namespace, name string) bool {
	// Check if configmaps exists
	cf, err := k.Kclient.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})

	if err != nil {
		return false
	}

	if len(cf.Name) > 0 {
		return true
	}

	return false
}

// UpdateConfigMap update a existent configmap
func (k *K8sutil) UpdateConfigMap(namespace string, name string, data map[string]string) error {

	cf := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: data,
	}

	if _, err := k.Kclient.CoreV1().ConfigMaps(namespace).Update(cf); err != nil {
		logrus.Error(fmt.Sprintf("Could not update configmap %s:", name), err)
		return err
	}

	return nil

}
