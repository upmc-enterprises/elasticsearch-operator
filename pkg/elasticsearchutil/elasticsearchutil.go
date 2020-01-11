package elasticsearchutil

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/sirupsen/logrus"
)

// MinMasterNodes calculates the minium number of master nodes needed to ensure that the cluster
// does not get into a split-brain state.
// https://www.elastic.co/guide/en/elasticsearch/reference/current/discovery-settings.html
func MinMasterNodes(replicas int) int {
	return (replicas / 2) + 1
}

// UpdateDiscoveryMinMasterNodes sets the 'discovery.zen.minimum_master_nodes' setting in elasticsearch
// to the given minimum of master nodes.
// https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-update-settings.html
func UpdateDiscoveryMinMasterNodes(esHost string, minMasterNodes int) error {
	var jsonStr = generateMinMasterNodesPayload(minMasterNodes)
	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/_cluster/settings", esHost), bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	logrus.Debugf("request to ES: %v", req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf(string(b))
	}
	defer resp.Body.Close()
	return nil
}

func generateMinMasterNodesPayload(minMasterNodes int) []byte {
	json := fmt.Sprintf(`{"transient": {"discovery.zen.minimum_master_nodes": %d}}`, minMasterNodes)
	return []byte(json)
}
