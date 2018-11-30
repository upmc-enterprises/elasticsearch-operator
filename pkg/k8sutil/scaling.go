/*

 */

package k8sutil

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	myspec "github.com/upmc-enterprises/elasticsearch-operator/pkg/apis/elasticsearchoperator/v1"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/api/core/v1"
	"strconv"
	"strings"
	"time"
)

const (
	MAX_DataNodePodRestartTime = 400  // in seconds
	MAX_EsCommunicationTime    = 180  // in seconds
	MAX_EsWaitForDataNode      = "6m" // time for the master to wait for data node to reboot before it rebalances the shards
)
func compare_pod_vs_statefulset(namespace, statefulSetName,podName string, k *K8sutil) bool {
	resourceMatch := true
	logrus.Infof("Scaling: Compare Spec POD Vs statefulset  : ",podName)
	statefulSet, err := k.Kclient.AppsV1beta2().StatefulSets(namespace).Get(statefulSetName, metav1.GetOptions{})
	if (err != nil){
		return resourceMatch
	}
	pod, err := k.Kclient.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if (err != nil){
		return resourceMatch
	}

   if pod.Spec.Containers[0].Resources.Requests["cpu"] != statefulSet.Spec.Template.Spec.Containers[0].Resources.Requests["cpu"] {
		logrus.Infof("Scaling: Compare POD Vs SS  cpu changed  by user: ", pod.Spec.Containers[0].Resources.Requests["cpu"], " current value: ", statefulSet.Spec.Template.Spec.Containers[0].Resources.Requests["cpu"])
		resourceMatch = false
		return resourceMatch
	}
	/*if memory != statefulSet.Spec.Template.Spec.Containers[0].Resources.Requests["memory"] {
		resourceMatch = false
		return resourceMatch
	}*/
	javaOptions:=""
	for _, env := range pod.Spec.Containers[0].Env {
		if env.Name == "ES_JAVA_OPTS" {
			javaOptions = env.Value
			break
		}
	}
	for _, env := range statefulSet.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "ES_JAVA_OPTS" {
			if env.Value != javaOptions {
				logrus.Infof("Scaling: Compare POD Vs SS java_opts Changed by user: ", javaOptions, " current value: ", env.Value, " name: ", podName)
				resourceMatch = false
				return resourceMatch
			}
			break
		}
	}

	return resourceMatch
}

func k8_check_DataNodeRestarted(namespace, statefulSetName,dnodename string, k *K8sutil) error {
	ret := fmt.Errorf("Scaling: POD is not up: ")
	statefulset, _ := k.Kclient.AppsV1beta2().StatefulSets(namespace).Get(statefulSetName, metav1.GetOptions{})

	ss_version, _ := strconv.Atoi(statefulset.ObjectMeta.ResourceVersion)
	pod, _ := k.Kclient.CoreV1().Pods(namespace).Get(dnodename, metav1.GetOptions{})
	logrus.Infof("Scaling: checking Datanode if it restarted or not by k8:ss: %s datanode:%s", statefulSetName,dnodename)
	waitSeconds := MAX_DataNodePodRestartTime
	for i := 0; i < waitSeconds; i++ {
		pod, _ = k.Kclient.CoreV1().Pods(namespace).Get(dnodename, metav1.GetOptions{})
		if pod == nil {
			//logrus.Infof("Scaling: POD Terminated ")
		} else {
			pod_version, _ := strconv.Atoi(pod.ObjectMeta.ResourceVersion)
			if pod_version > ss_version {
				//logrus.Infof("Scaling: POD started pod_version:",pod_version," dd_version:",ss_version," state: ",(pod.Status.ContainerStatuses))
				status := ""
				if len(pod.Status.ContainerStatuses) > 0 {
					if pod.Status.ContainerStatuses[0].State.Running != nil {
						status = status + pod.Status.ContainerStatuses[0].State.Running.String()
					}
					if pod.Status.ContainerStatuses[0].State.Waiting != nil {
						status = status + pod.Status.ContainerStatuses[0].State.Waiting.String()
					}
					if pod.Status.ContainerStatuses[0].State.Terminated != nil {
						status = status + pod.Status.ContainerStatuses[0].State.Terminated.String()
					}
				}
				if strings.Contains(status, "ContainerStateRunning") {
					logrus.Infof("Scaling: POD started pod_version: %d", pod_version, " ss_version: %d", ss_version, " Status: %s", status)
					ret = nil
					break
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
	return ret
}
func get_masterIP(k *K8sutil, namespace, clusterName string) string {
	listN, _ := k.Kclient.CoreV1().Services(namespace).List(metav1.ListOptions{})
	for _, s := range listN.Items {
		service_name := "elasticsearch-" + clusterName
		if service_name == s.Name {
			if len(s.Spec.ExternalIPs) > 0 && (len(s.Spec.Ports) > 0) {
				ret := s.Spec.ExternalIPs[0] + ":" + strconv.Itoa(int(s.Spec.Ports[0].NodePort))
				logrus.Infof("Scaling:  MasterIP external port: %s", ret)
				return ret
			}
			if len(s.Spec.ClusterIP) > 0 && (len(s.Spec.Ports) > 0) {
				ret := s.Spec.ClusterIP + ":" + strconv.Itoa(int(s.Spec.Ports[0].Port))
				logrus.Infof("Scaling:  MasterIP Internal port: %s", ret)
				return ret
			}
		}
	}
	return ""
}
func scale_datanode(k *K8sutil, namespace, clusterName, statefulSetName, podName string, resources myspec.Resources, javaOptions string, statefulSet *v1beta2.StatefulSet, pod_index int32, masterip string) error {
	// Step-3: ES-change: check if ES cluster is green state, suppose if one of the data node is down and state is yellow then do not proceed with scaling.
	if err := es_checkForGreen(masterip); err != nil {
		err = fmt.Errorf("Scaling: ES cluster is not in green state")
		return err
	}

	// Step-2: ES-chanage:  change default time from 1 min to 3 to n min to avoid copying of shards belonging to the data node that is going to be scaled.
	if err := es_change_settings(masterip, MAX_EsWaitForDataNode, "request"); err != nil { // TODO before overwriting save the orginal setting
		return err
	}

	// Step-4: scale the Data node by deleting the POD
	deletePolicy := metav1.DeletePropagationForeground
	
	err := k.Kclient.Core().Pods(namespace).Delete(podName, &metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy})
	if err != nil {
		logrus.Error("ERROR: Scaling: Could not delete POD: ", err)
		return err
	}

	// Step-5: check if the POD is restarted and in running state from k8 point of view.
	if err =  k8_check_DataNodeRestarted(namespace, statefulSetName, podName, k); err != nil {
		return err
	}

	// Step-6: check if the POD is up from the ES point of view.
	if err = es_checkForNodeUp(masterip, podName, MAX_EsCommunicationTime); err != nil {
		return err
	}

	// Step-7: check if all shards are registered with Master, At this ES should turn in to green from yellow, now it is safe to scale next data node.
	if err = es_checkForShards(masterip,  podName, MAX_EsCommunicationTime); err != nil {
		return err
	}
	// Step-8: Undo the timeout settings
	if err := es_change_settings(masterip, "1m", "async"); err != nil {
		return err
	}
	logrus.Infof("Scaling:------------- sucessfully Completed  for: %s :--------------------------------", podName)
	return nil
}

func scale_statefulset(k *K8sutil, namespace, clusterName, statefulSetName string, resources myspec.Resources, javaOptions string, statefulSet *v1beta2.StatefulSet, replicas int32) error {
	cpu, _ := resource.ParseQuantity(resources.Requests.CPU)
	memory, _ := resource.ParseQuantity(resources.Requests.Memory)

	// Step-1: check if there is any change in the 3-resources: javaoptions,cpu and memory
	resourceMatch := true
	if cpu != statefulSet.Spec.Template.Spec.Containers[0].Resources.Requests["cpu"] {
		logrus.Infof("Scaling: cpu changed  by user: ", cpu, " current value: ", statefulSet.Spec.Template.Spec.Containers[0].Resources.Requests["cpu"])
		resourceMatch = false
		statefulSet.Spec.Template.Spec.Containers[0].Resources.Requests["cpu"] = cpu
	}
	if memory != statefulSet.Spec.Template.Spec.Containers[0].Resources.Requests["memory"] {
		// TODO: there is a slight mismatch,  Memory Changed USER: %!(EXTRA resource.Quantity={{1073741824 0} {<nil>}  BinarySI}, string= from k8: , resource.Quantity={{1073741824 0} {<nil>} 1Gi BinarySI})
		//logrus.Infof(" Memory Changed USER: ",memory," from k8: ",statefulSet.Spec.Template.Spec.Containers[0].Resources.Requests["memory"].i.value)
		//resourceMatch = false
	}
	for index, env := range statefulSet.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "ES_JAVA_OPTS" {
			if env.Value != javaOptions {
				logrus.Infof("Scaling: java_opts Changed by user: ", javaOptions, " current value: ", env.Value, " name: ", statefulSetName)
				resourceMatch = false
				statefulSet.Spec.Template.Spec.Containers[0].Env[index].Value = javaOptions
			}
			break
		}
	}

	masterip := get_masterIP(k, namespace, clusterName)
	if masterip == "" {
		err := fmt.Errorf("MasterIP is empty")
		return err
	}

	if !resourceMatch {
		logrus.Infof("Scaling:STARTED scaling with new resources ... : %s", statefulSetName)
		// TODO : only memory request is updated, memory limit as need to be updated.
		statefulSet.Spec.Template.Spec.Containers[0].Resources.Requests["memory"] = memory
		
		_, err := k.Kclient.AppsV1beta2().StatefulSets(namespace).Update(statefulSet)
		if err != nil {
			logrus.Error("ERROR: Scaling: Could not update statefulSet: ", err)
			return err
		}
		
		var index int32 = 0
		for index = 0; index < replicas; index++ {
			podName := fmt.Sprintf("%s-%d", statefulSetName, index)
			ret := scale_datanode(k, namespace, clusterName, statefulSetName, podName, resources, javaOptions, statefulSet, index,masterip)
			if ret != nil {
				return ret
			}
		}
	}else{
		// check if Scaling is stopped half the way, this may be because server is stopped or for various reasons */
		var index int32 = 0
		for index = 0; index < replicas; index++ {
			podName := fmt.Sprintf("%s-%d", statefulSetName, index)
			if (compare_pod_vs_statefulset(namespace, statefulSetName, podName, k)){
				continue
			}
			ret := scale_datanode(k, namespace, clusterName, statefulSetName, podName, resources, javaOptions, statefulSet, index,masterip)
			if ret != nil {
				return ret
			}
		}
	}
	return nil
}
