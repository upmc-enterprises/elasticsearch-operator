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
	"time"
	"strconv"
	"strings"
)

const (
	MAX_DataNodePodRestartTime    = 400  // in seconds
	MAX_EsCommunicationTime = 180 // in seconds
	MAX_EsWaitForDataNode = "6m" // time for the master to wait for data node to reboot before it rebalances the shards
)

func k8_check_DataNodeRestarted(namespace, statefulSetName string, k *K8sutil) error {
	ret := fmt.Errorf("Scaling: POD is not up: ",statefulSetName)
	dnodename := statefulSetName + "-0"
	newstatefulset, _ := k.Kclient.AppsV1beta2().StatefulSets(namespace).Get(statefulSetName, metav1.GetOptions{})
	
	ss_version , _ := strconv.Atoi(newstatefulset.ObjectMeta.ResourceVersion)
	pod, _ := k.Kclient.CoreV1().Pods(namespace).Get(dnodename, metav1.GetOptions{})
	logrus.Infof("Scaling: checking Datanode if it restarted or not by k8: %s",statefulSetName)
	waitSeconds := MAX_DataNodePodRestartTime
	for i := 0; i < waitSeconds; i++ {
		pod, _ = k.Kclient.CoreV1().Pods(namespace).Get(dnodename, metav1.GetOptions{})
		if (pod == nil){
			//logrus.Infof("Scaling: POD Terminated ")
		}else{
			pod_version , _ := strconv.Atoi(pod.ObjectMeta.ResourceVersion)
			if pod_version > ss_version {
				//logrus.Infof("Scaling: POD started pod_version:",pod_version," dd_version:",ss_version," state: ",string(pod.Status.ContainerStatuses))
	
				status := ""
				if (len(pod.Status.ContainerStatuses) > 0) {
					if (pod.Status.ContainerStatuses[0].State.Running != nil) {
						status = status + pod.Status.ContainerStatuses[0].State.Running.String()
					}
					if (pod.Status.ContainerStatuses[0].State.Waiting != nil) {
						status = status + pod.Status.ContainerStatuses[0].State.Waiting.String()
					}
					if (pod.Status.ContainerStatuses[0].State.Terminated != nil) {
						status = status + pod.Status.ContainerStatuses[0].State.Terminated.String()
					}
				}
				if strings.Contains(status,"ContainerStateRunning") {
					logrus.Infof("Scaling: POD started pod_version: %d",pod_version," ss_version: %d",ss_version," Status: %s",status)
				
					ret = nil
				    break
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
	return ret
}

func scale_datanode(k *K8sutil, namespace, statefulSetName string, resources myspec.Resources, javaOptions string, statefulSet *v1beta2.StatefulSet, masterip string) error {
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
				logrus.Infof("Scaling: java_opts Changed by user: ", javaOptions, " current value: ", env.Value, " name: ",statefulSetName)
				resourceMatch = false
				statefulSet.Spec.Template.Spec.Containers[0].Env[index].Value = javaOptions
			}
			break
		}
	}

	if !resourceMatch { 
		logrus.Infof("Scaling:STARTED scaling with new resources ... : %s", statefulSetName)
		// TODO : only memory request is updated, memory limit as need to be updated.
		statefulSet.Spec.Template.Spec.Containers[0].Resources.Requests["memory"] = memory		
		
// Step-3: ES-chanage: check if ES cluster is green state, suppose if one of the data node is down and state is yellow then do not proceed with scaling.
		if err := es_checkForGreen(masterip); err != nil { 
			err = fmt.Errorf("Scaling: ES cluster is not in green state", " name: ",statefulSetName)
			return err
		}
		
// Step-2: ES-chanage:  change default time from 1 min to 3 to n min to avoid copying of shards belonging to the data node that is going to be scaled.
		if err := es_change_settings(masterip, MAX_EsWaitForDataNode, "request");err != nil { // TODO before overwriting save the orginal setting 
			return err
		}
		
// Step-4: scale the Data node by updating the new resources in the stateful set
		_, err := k.Kclient.AppsV1beta2().StatefulSets(namespace).Update(statefulSet)
		if err != nil {
			logrus.Error("ERROR: Scaling: Could not scale statefulSet: ", err)
			return err
		}
		dnodename := statefulSetName + "-0"

// Step-5: check if the POD is restarted and in running state from k8 point of view.
		k8_check_DataNodeRestarted(namespace, statefulSetName, k)

// Step-6: check if the POD is up from the ES point of view.
		if err = es_checkForNodeUp(masterip, dnodename, MAX_EsCommunicationTime); err != nil {
			return err
		}
	
// Step-7: check if all shards are registered with Master, At this ES should turn in to green from yellow, now it is safe to scale next data node.
		if err = es_checkForShards(masterip, dnodename, MAX_EsCommunicationTime); err != nil {
			return err
		}
// Step-8: Undo the timeout settings
		if err := es_change_settings(masterip, "1m", "async");err != nil { 
			return err
		}
		logrus.Infof("Scaling:------------- sucessfully Completed  for: %s :--------------------------------", statefulSetName)
	}
	return nil
}
