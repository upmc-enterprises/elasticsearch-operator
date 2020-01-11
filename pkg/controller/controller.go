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

package controller

import (
	"github.com/sirupsen/logrus"
	"github.com/upmc-enterprises/elasticsearch-operator/pkg/k8sutil"
)

// Config defines properties of the controller
type Config struct {
	k8sclient *k8sutil.K8sutil
}

// Controller object
type Controller struct {
	Config
}

// New up a Controller
func New(name string, k8sclient *k8sutil.K8sutil) (*Controller, error) {

	c := &Controller{
		Config: Config{
			k8sclient: k8sclient,
		},
	}

	return c, nil
}

// Run gets the party started
func (c *Controller) Run() error {

	// Init TPR
	err := c.init()

	if err != nil {
		logrus.Error("Error in init(): ", err)
		return err
	}

	return nil
}

func (c *Controller) init() error {
	err := c.k8sclient.CreateKubernetesCustomResourceDefinition()
	if err != nil {
		return err
	}

	if c.k8sclient.EnableInitDaemonset {
		err = c.k8sclient.CreateNodeInitDaemonset()
		if err != nil {
			return err
		}
	}

	return nil
}
