

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


/* TODO-1 :   This speeds up scaling.
"persistent" : {
 "cluster.routing.allocation.node_concurrent_recoveries": 20,     from 2 to 20
 }

TODO-2:
 set translog durabilty to request:  index.translog.durability

After scaling operation it can be converted to async.
*/

func es_set_delaytimeout(es_ip , duration string)(error){  
	var ret error
	ret = errors.New("Scaling: error in setting delay timeout")

	logrus.Infof("Scaling: setting ES delay timeout to ", duration)
	client := &http.Client{
	}
	body:="{ "
	body = body + "\"settings\": { \"index.unassigned.node_left.delayed_timeout\": \""+duration+"\"  }"
	body = body +  " }"

	//fmt.Println("set BODY :",body,":")
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
func es_checkForGreen(es_ip string)(error){ /* TODO */
	logrus.Infof("Scaling: ES checking for green  ")
	return es_checkForShards(es_ip , "", 180)
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
	fmt.Println(nodename,"  shards: ",node_count," UNASSIGNED shards: ",unassigned_count," initialising shards: ",initializing_count)
	return node_count,unassigned_count+initializing_count
}

func es_checkForShards(es_ip string, nodeName string, waitSeconds int)(error) {
	var ret error
	ret = errors.New("still unassigned shards are there")
	
	logrus.Infof("Scaling: ES checking for shards")
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

	 logrus.Infof("Scaling: ES checking if the data node joined master")
 	 for i := 0; i < waitSeconds; i++ {
		 response, err := http.Get("http://" + es_ip + "/_cat/nodes?v&h=n,ip,v")
		 if err != nil {
			 fmt.Printf("The HTTP request failed with error %s\n", err)
			 ret = err
		 } else {
			 data, _ := ioutil.ReadAll(response.Body)
			 str_ret := strings.Contains(string(data), nodeName)
			 if (str_ret){
				 //time.Sleep(1 * time.Second)
				 //fmt.Println("http output: nodename: ",nodeName," DATA: ", string(data))
				 ret = nil
			 	 return ret;
			 }
		 }
		 time.Sleep(1 * time.Second)
	 }

	 return ret;
 }
/*
es_flush: response:  &{409 Conflict 409 HTTP/1.1 1 1 map[Content-Type:[application/json; charset=UTF-8]] 0xc420017a60 -1 [] false true map[] 0xc4200d9600 <nil>}  BODY:  {"_shards":{"total":32,"successful":31,"failed":1},
"indexer_tenant_1":{"total":16,"successful":16,"failed":0},"indexer_tenant_0":{"total":16,"successful":15,"failed":1,"failures":[{"shard":4,"reason":"pending operations","routing":{"state":"STARTED","primary":false,"node":"pGFMewaYSG-acz3GHywEzQ","relocating_node":null,"shard":4,"index":"indexer_tenant_0","allocation_id":{"id":"fu9axLfdQZqbwBKXd4DSPw"}}}]}}
 */

func es_flush(es_ip string)(bool){
	ret := false

	client := &http.Client{
	}

	req, _ := http.NewRequest("POST", "http://"+es_ip+"/_all/_flush/synced", bytes.NewBufferString(""))
	req.Header.Add("Content-Type", `application/json`)
	resp, _ := client.Do(req)
	data, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("es_flush: response: ",resp," BODY: ",string(data))
	if (resp.StatusCode == 200){
		ret = true
		time.Sleep(1 * time.Second)
	}
	return ret;
}

func es_set_readonly(es_ip string)(bool){
	ret := false
//	return true

	client := &http.Client{
	}
	body:="{ "
	//body= body +"{ \"persistent\": { \"cluster.blocks.read_only\": \"true\" } }"
	body = body + "\"transient\": { \"cluster.blocks.read_only\": \"true\"  }"
//	body = body + ",  \"cluster.routing.allocation.enable\": \"none\" } "
	body = body +  " }"

	//fmt.Println("set BODY :",body,":")
	req, _ := http.NewRequest("PUT", "http://"+es_ip+"/_cluster/settings", bytes.NewBufferString(body))
	req.Header.Add("Content-Type", `application/json`)
	resp, _ := client.Do(req)
	fmt.Println("es_flush: response: ",resp, resp.Body)
	if (resp.StatusCode == 200){
		ret = true
		time.Sleep(1 * time.Second)
	}
	es_flush(es_ip)

	return ret;
}
func es_unset_readonly(es_ip string)(bool){
	ret := false
//	return true

	client := &http.Client{
	}
	body:="{ "
	//body= body +"{ \"persistent\": { \"cluster.blocks.read_only\": \"false\" } }"
	body = body + "\"transient\": { \"cluster.blocks.read_only\": \"false\" }"
	//body = body + ",  \"cluster.routing.allocation.enable\": \"all\" } "
	body = body +  " }"

	fmt.Println("unset BODY :",body,":")
	req, _ := http.NewRequest("PUT", "http://"+es_ip+"/_cluster/settings", bytes.NewBufferString(body))
	req.Header.Add("Content-Type", `application/json`)
	resp, _ := client.Do(req)
	fmt.Println("es_unset: response: ",resp)
	if (resp.StatusCode == 200){
		ret = true
		time.Sleep(1 * time.Second)
	}

	return ret;
}