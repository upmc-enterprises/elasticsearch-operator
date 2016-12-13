/*
Copyright (c) 2016, UPMC Enterprises
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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/1.4/kubernetes"
	"k8s.io/client-go/1.4/pkg/api/v1"
	"k8s.io/client-go/1.4/pkg/apis/extensions/v1beta1"
)

var (
	namespace                  = os.Getenv("NAMESPACE")
	tprName                    = "elasticsearch-cluster.enterprises.upmc.com"
	elasticSearchEndpoint      = fmt.Sprintf("/apis/enterprises.upmc.com/v1/namespaces/%s/elasticsearchclusters", namespace)
	elasticSearchWatchEndpoint = fmt.Sprintf("/apis/enterprises.upmc.com/v1/namespaces/%s/elasticsearchclusters?watch=true", namespace)
	tprEndpoint                = "/apis/extensions/v1beta1/thirdpartyresources"
)

const (
	dataDir    = "/data"
	backupFile = "/var/elastic/latest.backup"
)

// ThirdPartyResource in Kubernetes
type ThirdPartyResource struct {
	APIVersion  string               `json:"apiVersion"`
	Kind        string               `json:"kind"`
	Description string               `json:"description"`
	Metadata    map[string]string    `json:"metadata"`
	Versions    [1]map[string]string `json:"versions,omitempty"`
}

// ElasticSearchEvent stores when a ES needs created
type ElasticSearchEvent struct {
	Type   string        `json:"type"`
	Object ElasticSearch `json:"object"`
}

// ElasticSearchCluster represents a custom ES object
type ElasticSearchCluster struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   map[string]string `json:"metadata"`
	Spec       ElasticSearchSpec `json:"spec"`
}

// ElasticSearchSpec represents the custom data of the object
type ElasticSearchSpec struct {
	ClusterName    string `json: "cluster-name"`
	ClientNodeSize int    `json:"client-node-size"`
	MasterNodeSize int    `json:"master-node-size"`
	DataNodeSize   int    `json:"data-node-size"`
}

// ElasticSearchList represents a list of ES Clusters
type ElasticSearchList struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   map[string]string      `json:"metadata"`
	Items      []ElasticSearchCluster `json:"items"`
}

// ElasticSearch represents a Kubernetes ES type
type ElasticSearch struct {
	Kind       string            `json:"kind"`
	APIVersion string            `json:"apiVersion"`
	Metadata   map[string]string `json:"metadata"`
	Data       map[string]string `json:"data"`
	Type       string            `json:"type"`
}

// GetElasticSearchClusters returns a list of custom clusters defined
func GetElasticSearchClusters(apiHost string) ([]ElasticSearchCluster, error) {
	var resp *http.Response
	var err error
	for {
		resp, err = http.Get(apiHost + elasticSearchEndpoint)
		if err != nil {
			logrus.Error(err)
			time.Sleep(5 * time.Second)
			continue
		}
		break
	}

	var elasticSearchList ElasticSearchList
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&elasticSearchList)
	if err != nil {
		logrus.Error("Could not get list of elasticsearch clusters: ", err)
		return nil, err
	}

	return elasticSearchList.Items, nil
}

// MonitorElasticSearchEvents watches for new or removed clusters
func MonitorElasticSearchEvents(apiHost string) (<-chan ElasticSearchEvent, <-chan error) {
	events := make(chan ElasticSearchEvent)
	errc := make(chan error, 1)
	go func() {
		for {
			resp, err := http.Get(apiHost + elasticSearchWatchEndpoint)
			if err != nil {
				errc <- err
				time.Sleep(5 * time.Second)
				continue
			}
			if resp.StatusCode != 200 {
				errc <- errors.New("Invalid status code: " + resp.Status)
				time.Sleep(5 * time.Second)
				continue
			}

			decoder := json.NewDecoder(resp.Body)
			for {
				var event ElasticSearchEvent
				err = decoder.Decode(&event)
				if err != nil {
					errc <- err
					break
				}
				events <- event
			}
		}
	}()

	return events, errc
}

// CreateKubernetesThirdPartyResource checks if ElasticSearch TPR exists. If not, create
func CreateKubernetesThirdPartyResource(kclient *kubernetes.Clientset, apiHost string) error {
	tprResult, _ := kclient.ThirdPartyResources().Get(tprName)

	if len(tprResult.Name) == 0 {
		logrus.Info("ElasticSearchCluster ThirdPartyResource not found, creating...")

		tpr := &v1beta1.ThirdPartyResource{
			ObjectMeta: v1.ObjectMeta{
				Name: tprName,
			},
			Versions: []v1beta1.APIVersion{
				{Name: "v1"},
			},
			Description: "Managed elasticsearch clusters",
		}

		_, err := kclient.ThirdPartyResources().Create(tpr)
		if err != nil {
			logrus.Error("Error creating ThirdPartyResource: ", err)
			return err
		}
	} else {
		logrus.Info("Elastic Search TPR already existing...")
	}

	return nil
}
