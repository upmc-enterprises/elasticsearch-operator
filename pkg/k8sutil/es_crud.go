

package k8sutil

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
	"github.com/Sirupsen/logrus"
)


/* 
TODO:
  Assuming orginal settings are default values like delayed_timeout is 1m and translog.durability is async, incase if the user is changed then 
  user changed settings will be lost after scaling operations.
  this can be corrected by reading the setting before change.
*/

func es_change_settings(es_ip , duration,translog_durability string)(error){  
	var ret error
	ret = errors.New("Scaling: error in setting delay timeout")
	
	logrus.Infof("Scaling: change ES setting:  delay timeout:%s  and translog durability: %s", duration, translog_durability)
	client := &http.Client{
	}
	body:="{ "
	body = body + "\"settings\": { \"index.unassigned.node_left.delayed_timeout\": \""+duration+"\" ,"
	body = body + "\"index.translog.durability\": \""+translog_durability +"\" }"
	body = body +  " }"

	req, _ := http.NewRequest("PUT", "http://"+es_ip+"/_all/_settings", bytes.NewBufferString(body))
	req.Header.Add("Content-Type", `application/json`)
	resp, _ := client.Do(req)
	data, _ := ioutil.ReadAll(resp.Body)

	if (resp.StatusCode == 200){
		ret = nil
	}else{
		if (strings.Contains(string(data), "index_not_found_exception")){
			ret = nil  /* if there are no indexes , then this function need po pass */
			fmt.Println("WARNING in setting delaytimeout: response: ",resp, string(data))
			return ret
		}
		//string str:= "Scaling: setting delaytimeout: response: " + resp + string(data)
		ret = errors.New("Scaling: setting delaytimeout: response: ")
	}
	return ret;
}
func es_checkForGreen(es_ip string)(error){ 
	logrus.Infof("Scaling: ES checking for green  ")
	return es_checkForShards(es_ip , "", MAX_EsCommunicationTime)
}
func util_wordcount(input string, nodename string) (int,int) {
	node_count := 0
	unassigned_count :=0
	initializing_count :=0
	words := strings.Fields(input)
	for _, word := range words {
		if (nodename != "" && word == nodename){
			node_count++
		} else if (word == "UNASSIGNED"){
			unassigned_count++
		}
		if (word == "INITIALIZING"){
			initializing_count++
		}
	}
	//fmt.Println(nodename,"  shards: ",node_count," UNASSIGNED shards: ",unassigned_count," initialising shards: ",initializing_count)
	return node_count,unassigned_count+initializing_count
}

func es_checkForShards(es_ip string, nodeName string, waitSeconds int)(error) {
	var ret error
	ret = errors.New("still unassigned shards are there")
	
	logrus.Infof("Scaling: ES checking for shards name: %s",nodeName)
	for i := 0; i < waitSeconds; i++ {
		response, err := http.Get("http://" + es_ip + "/_cat/shards")
		if err != nil {
			fmt.Printf("The HTTP request failed with error %s\n", err)
		} else {
			data, _ := ioutil.ReadAll(response.Body)
			_,unassigned_count := util_wordcount(string(data), nodeName)
			if (unassigned_count>0) {
				time.Sleep(1 * time.Second)
				continue
			} else {
				ret = nil
				break
			}
		}
		time.Sleep(1 * time.Second)
	}

	return ret;
}

 func es_checkForNodeUp(es_ip string, nodeName string, waitSeconds int)(error){
	 var ret error
	 ret = errors.New("ES node is not up")

	 logrus.Infof("Scaling: ES checking if the data node: %s joined master ",nodeName)
 	 for i := 0; i < waitSeconds; i++ {
		 response, err := http.Get("http://" + es_ip + "/_cat/nodes?v&h=n,ip,v")
		 if err != nil {
			 fmt.Printf("Scaling: The HTTP request failed with error %s\n", err)
			 ret = err
		 } else {
			 data, _ := ioutil.ReadAll(response.Body)
			 str_ret := strings.Contains(string(data), nodeName)
			 if (str_ret){
				 ret = nil
			 	 return ret;
			 }
		 }
		 time.Sleep(1 * time.Second)
	 }

	 return ret;
 }


