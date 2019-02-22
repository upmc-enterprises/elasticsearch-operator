/*
Copyright (c) 2019, VMware
All rights reserved.
Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:
    * Redistributions of source code must retain the above copyright
      notice, this list of conditions and the following disclaimer.
    * Redistributions in binary form must reproduce the above copyright
      notice, this list of conditions and the following disclaimer in the
      documentation and/or other materials provided with the distribution.
    * Neither the name VMware nor the
      names of its contributors may be used to endorse or promote products
      derived from this software without specific prior written permission.
THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL VMWARE BE LIABLE FOR ANY
DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOW CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
*/

package cluster

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/onsi/ginkgo"

	"github.com/onsi/gomega"
	"github.com/upmc-enterprises/elasticsearch-operator/test/framework/command"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// clusterName is the name of the Kind cluster
const defaultClusterName string = "eso-e2e"

// Cluster is a cluster
type Cluster struct {
	clusterName string
	kubeConfig  string
}

func NewCluster() *Cluster {
	return &Cluster{
		clusterName: defaultClusterName,
	}
}

// KubeConfig
func (c *Cluster) KubeConfig() string {
	if c.kubeConfig != "" {
		return c.kubeConfig
	}
	cmd := exec.Command("kind", "get", "kubeconfig-path", fmt.Sprintf("--name=%s", c.clusterName))
	cmd.Stderr = ginkgo.GinkgoWriter
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	kubeConfig := strings.TrimSpace(string(out))
	c.kubeConfig = kubeConfig
	return c.kubeConfig
}

func (c *Cluster) Start() {
	cmd := exec.Command("kind", "create", "cluster", "--name", c.clusterName)
	out, err := cmd.CombinedOutput()
	fmt.Fprintln(ginkgo.GinkgoWriter, string(out))

	if err != nil {
		if strings.Contains(string(out), "already exists") {
			c.Stop()
			c.Start()
		} else {
			gomega.ExpectWithOffset(1, err).To(gomega.BeNil())
		}
	}
}

// RestConfig returns a rest configuration pointed at the provisioned cluster
func (c *Cluster) RestConfig() *restclient.Config {
	cfg, err := clientcmd.BuildConfigFromFlags("", c.KubeConfig())
	gomega.ExpectWithOffset(1, err).To(gomega.BeNil())
	return cfg
}

// KubeClient returns a Kubernetes client pointing at the provisioned cluster
func (c *Cluster) KubeClient() kubernetes.Interface {
	cfg := c.RestConfig()
	client, err := kubernetes.NewForConfig(cfg)
	gomega.ExpectWithOffset(1, err).To(gomega.BeNil())
	return client
}

func (c *Cluster) Stop() {
	cmd := exec.Command("kind", "delete", "cluster", "--name", c.clusterName)
	command.Run(cmd)
}

func (c *Cluster) Kubectl(args ...string) {

	kubeConfigArg := fmt.Sprintf("--kubeconfig=%s", c.KubeConfig())

	kubectlArgs := append(args, kubeConfigArg)

	command.Run(exec.Command("kubectl", kubectlArgs...))
}

func (c *Cluster) KubectlApply(namespace string, manifest string) error {
	kubeConfigArg := fmt.Sprintf("--kubeconfig=%s", c.KubeConfig())
	cmd := exec.Command("kubectl", "apply", "-n", namespace, kubeConfigArg, "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Stderr = ginkgo.GinkgoWriter
	cmd.Stdout = ginkgo.GinkgoWriter
	return cmd.Run()
}

func (c *Cluster) Sideload(pipe io.Reader) error {
	cmd := exec.Command("docker", "exec", "--interactive", fmt.Sprintf("%s-control-plane", c.clusterName), "docker", "load")
	cmd.Stdout = ginkgo.GinkgoWriter
	cmd.Stderr = ginkgo.GinkgoWriter
	cmd.Stdin = pipe
	return cmd.Run()
	//"upmcenterprises/elasticsearch-operator:0.2.0"
}
