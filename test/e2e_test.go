// +build integration

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

package test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"text/template"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	c "github.com/upmc-enterprises/elasticsearch-operator/test/framework/cluster"
	"github.com/upmc-enterprises/elasticsearch-operator/test/framework/image"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type esCluster struct {
	Namespace        string
	manifest         string
	DataNodeReplicas int32
	KibanaImage      string
	CerebroImage     string
	EsImage          string
	SnapshotImage    string
}

func newEsCluster(namespace string) esCluster {
	return esCluster{
		Namespace:        namespace,
		manifest:         "manifests/e2e-cluster.yaml",
		DataNodeReplicas: 1,
		KibanaImage:      "docker.elastic.co/kibana/kibana-oss:6.1.3",
		CerebroImage:     "upmcenterprises/cerebro:0.6.8",
		EsImage:          "upmcenterprises/docker-elasticsearch-kubernetes:6.1.3_1",
		SnapshotImage:    "upmcenterprises/elasticsearch-cron:0.0.4",
	}
}

var (
	cluster    *c.Cluster
	client     kubernetes.Interface
	esClusters chan esCluster
)

const (
	namespace      = "operator"
	controllerName = "elasticsearch-operator"
)

func init() {
	config.DefaultReporterConfig.Verbose = true
	config.GinkgoConfig.EmitSpecProgress = true
}

var _ = SynchronizedBeforeSuite(func() []byte {
	fmt.Fprintln(os.Stderr, "Building Docker image")
	image.Build()

	fmt.Fprintln(os.Stderr, "Exporting image")
	image, err := image.Export()
	Expect(err).NotTo(HaveOccurred())

	fmt.Fprintln(os.Stderr, "Starting new cluster")
	cluster = c.NewCluster()
	cluster.Start()
	client = cluster.KubeClient()

	fmt.Fprintln(os.Stderr, "Sideloading image")
	err = cluster.Sideload(image)
	Expect(err).NotTo(HaveOccurred())

	return []byte{}
}, func(data []byte) {})

var _ = SynchronizedAfterSuite(func() {
	//cluster.Stop()
}, func() {})

func TestCluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ElasticSearch Operator Suite")
}

var _ = Describe("ElasticSearch Operator", func() {

	It("deploy the operator", func() {
		By("manually creating the namespace")

		cluster.Kubectl("create", "namespace", namespace)

		By("manually installing the operator")

		cluster.Kubectl("apply", "-f", "../example/controller.yaml")

		deployments := client.AppsV1().Deployments(namespace)

		Eventually(
			func() (*appsv1.Deployment, error) {
				return deployments.Get(controllerName, metav1.GetOptions{})
			},
			2*time.Minute, 5*time.Second,
		).Should(haveReplicas(1))
	})

	ossCluster := newEsCluster("es-oss")
	ossCluster.KibanaImage = "docker.elastic.co/kibana/kibana-oss:6.4.3"
	ossCluster.EsImage = "randomvariable/docker-elasticsearch-kubernetes:latest"

	for _, testCase := range []esCluster{
		//newEsCluster("example-es-cluster-hostpath"),
		ossCluster,
	} {
		esc := testCase
		It(fmt.Sprintf("manage ElasticSearch cluster %s", esc.Namespace), func() {

			esNamespace := esc.Namespace
			manifest := esc.manifest
			dataNodeReplicas := esc.DataNodeReplicas

			By("manually creating a target namespace")
			cluster.Kubectl("create", "namespace", esNamespace)

			volumeFile, err := ioutil.ReadFile("manifests/volumes.yaml")
			Expect(err).NotTo(HaveOccurred())

			tmpl, err := template.New("volumes").Parse(string(volumeFile))
			Expect(err).NotTo(HaveOccurred())
			var buf bytes.Buffer
			err = tmpl.Execute(&buf, esc)
			Expect(err).NotTo(HaveOccurred())

			By("manually creating storage classes and volumes")
			err = cluster.KubectlApply(esNamespace, buf.String())
			Expect(err).NotTo(HaveOccurred())

			By("manually defining an ElasticSearch cluster")
			manifestFile, err := ioutil.ReadFile(manifest)
			escTmpl, err := template.New("cluster").Parse(string(manifestFile))
			Expect(err).NotTo(HaveOccurred())
			var escBuf bytes.Buffer
			err = escTmpl.Execute(&escBuf, esc)
			Expect(err).NotTo(HaveOccurred())
			fmt.Fprintln(GinkgoWriter, escBuf.String())

			Eventually(
				func() error {
					return cluster.KubectlApply(esNamespace, escBuf.String())
				},
				2*time.Minute, 5*time.Second,
			).Should(BeNil())
			//err = cluster.KubectlApply(esNamespace, escBuf.String())

			clusterDeployments := client.AppsV1().Deployments(esNamespace)
			clusterStatefulSets := client.AppsV1().StatefulSets(esNamespace)

			By("the operator deploying cerebro")
			Eventually(
				func() (*appsv1.Deployment, error) {
					return clusterDeployments.Get("cerebro-example-es-cluster", metav1.GetOptions{})
				},
				2*time.Minute, 5*time.Second,
			).Should(haveReplicas(1))

			By("the operator deploying an ElasticSearch client node")
			Eventually(
				func() (*appsv1.Deployment, error) {
					return clusterDeployments.Get("es-client-example-es-cluster", metav1.GetOptions{})
				},
				2*time.Minute, 5*time.Second,
			).Should(haveReplicas(1))

			By("the operator deploying Kibana")
			Eventually(
				func() (*appsv1.Deployment, error) {
					return clusterDeployments.Get("kibana-example-es-cluster", metav1.GetOptions{})
				},
				2*time.Minute, 5*time.Second,
			).Should(haveReplicas(1))

			By(fmt.Sprintf("the operator deploying %d data nodes", dataNodeReplicas))
			Eventually(
				func() (*appsv1.StatefulSet, error) {
					return clusterStatefulSets.Get("es-data-example-es-cluster-hostpath-storage", metav1.GetOptions{})
				},
				2*time.Minute, 5*time.Second,
			).Should(haveReplicas(dataNodeReplicas))

			By("the operator deploying 1 primary node")
			Eventually(
				func() (*appsv1.StatefulSet, error) {
					return clusterStatefulSets.Get("es-master-example-es-cluster-hostpath-storage", metav1.GetOptions{})
				},
				2*time.Minute, 5*time.Second,
			).Should(haveReplicas(1))

			By("remove the cluster")
			cluster.Kubectl("delete", "-f", manifest, "-n", esNamespace)

			By("remove the volumes")
			cluster.Kubectl("delete", "-f", "manifests/volumes.yaml", "-n", esNamespace)
		})
	}
})

func haveReplicas(i int32) types.GomegaMatcher {
	return PointTo(
		MatchFields(IgnoreExtras, Fields{
			"Status": MatchFields(IgnoreExtras, Fields{
				"Replicas":      Equal(i),
				"ReadyReplicas": Equal(i),
			}),
		}),
	)
}
