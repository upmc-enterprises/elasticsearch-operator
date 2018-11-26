
# Patch Details:

The following are  changes present in scaling patch

- Every Data node will have stateful set: currently all data nodes are part of one Statefule set, scaling needs every data node to be seperate statefulset with one replica,  every datanode resource are updated independently to corresponding statefulset, so that the control will be in the hands of operator instead of k8.
- Scaling is optional feature: Seperate section is defined for scaling as shown in below example spec. If the scaling section is not present then entire scaling feature will be disabled.
- Added changes to support local disk, but it is not tested for multiple nodes, MAY need to add affinity between POD and node. 
- when Scaling will be triggered: Scaling will be triggered if there is any change in the one of the following 3 fields inside the scaling section: javaoptions,cpu and memory. javaoptions and resources entries corresponds only to non-data nodes incase scaling section is present. If scaling section is abscent then it corresponds to all nodes.
    - JavaOptions:  
    - CPU inside resources : number of cpu cores.
    - Memory inside resources : Memory size.
 - Steps involved in vertical scaling of  Elastic cluster: Repeating the following steps for each data node one after another, if there is any failure  then scaling will be halted for the rest of data nodes:
    - Step-1: check if there is any changes in  3-resources inside scaling section: javaoptions,cpu and memory. 
    - Step-2: ES-setting-change:  change default time from 1 min to 6 min to avoid copying of shards belonging to the data node that is going to be scaled.
    - step-2: ES-setting-change:  change translong durability from async to sync/request basis, this is to make no data is loss.
    - Step-3: ES-change: check if ES cluster is green state, suppose if one of the data node is down and state is yellow then do not proceed with scaling.
    - Step-4: scale the Data node by updating the new resources in the statefull set. Here the Data node will be restarted. 
    - Step-5: check if the POD is restarted and in running state from k8 point of view.
    - Step-6: check if the POD is up from the ES point of view. means Data node is register with the master.
    - Step-7: check if all shards are registered with Master, At this ES should turn in to green from yellow, now it is safe to scale next data node.
    - Step-8: Undo the  settings done in step-2.
 - Future Enhancements:
    - Horizontal scaling of Data nodes: increasing the "data-node-replica" will function as expected, but if "data-node-replica" is decreased by more then 2 then the Elastic search cluster can enter in to red state and there will be data loss, this can prevented by executing similar to vertical scaling one after another without entering into red state.
    - Picking "masternodeip" from services k8 objects instead from user.
    - Vertical scaling improvements: periodic scaling: currently vertical scaling is triggered from user, instead it can be triggered based on time automatically.
    - Multiple threads: Currently vertical for each elasticsearch cluster takes considerable amount of time, during this time other elastic cluster MAY not be done, this can be parallelised by running scaling operation in multiple threads.
    - Dependency on ElasticSearch Version: It it depends on elasticsearch version as 6.3, if there is lot of changes in  api's then scaling operation will fail. 
    - Local disk support: added changes to support local disk, but not tested with multiple nodes, it need to check that the pod will have affinity to the node, this can be enforced during the statefull set creation..
  - Usecases:
    - case1: scale down during start of non-peak time , and scale-up during the start of peak time every day. doing scaling operation twice in a day, this can done using a cron job without interrupting the service.
    - case2: scale down and scale up once in a while.
 

```
Example Spec containing optional scaling section

spec:
  client-node-replicas: 3
  data-node-replicas: 3
  data-volume-size: 10Gi
  java-options: -Xms256m -Xmx256m
  master-node-replicas: 2
  scaling:
    java-options: -Xms1052m -Xmx1052m
    resources:
      limits:
        cpu: 2m
        memory: 2048Mi
      requests:
        cpu: 1m
        memory: 1024Mi
  zones:
  - us-east-1a
  - us-east-1b
  - us-east-1c
```

# Log :

Below is actual log when there is a trigger for scaling, This is for a four data node cluster.

```
INFO[0032] Found cluster: es-cluster                    
INFO[0032] use-ssl not specified, defaulting to UseSSL=true 
INFO[0032]  Using [hub.docker.prod.walmart.com/upmcenterprises/docker-elasticsearch-kubernetes:6.1.3_0] as image for es cluster 
INFO[0032] use-ssl not specified, defaulting to UseSSL=true 
INFO[0034]  Current replicas: %!(EXTRA int32=1, string= New replica: , int32=1) 
INFO[0035]   updated Stateful set: %!(EXTRA string=es-master-es-cluster-localdisk-0) 
INFO[0035] Scaling enabled: true  JavaOptions:-Xms1072m -Xmx1072m Resources: {{1024Mi 4m} {2048Mi 6m}} masterIP: 10.12.17.168:30004 
INFO[0035] Scaling: java_opts Changed by user: %!(EXTRA string=-Xms1072m -Xmx1072m, string= current value: , string=-Xms1062m -Xmx1062m, string= name: , string=es-data-es-cluster-localdisk-0) 
INFO[0035] Scaling:STARTED scaling with new resources ... : es-data-es-cluster-localdisk-0 
INFO[0035] Scaling: ES checking for green               
INFO[0035] Scaling: ES checking for shards name:        
INFO[0036] Scaling: change ES setting:  delay timeout:6m  and translog durability: request 
INFO[0042] Scaling: checking Datanode if it restarted or not by k8: es-data-es-cluster-localdisk-0 
INFO[0047] Scaling: POD started pod_version: 107346%!(EXTRA string= ss_version: %d, int=107321, string= Status: %s, string=&ContainerStateRunning{StartedAt:2018-11-23 17:30:54 +0530 IST,}) 
INFO[0047] Scaling: ES checking if the data node: es-data-es-cluster-localdisk-0-0 joined master  
INFO[0057] Scaling: ES checking for shards name: es-data-es-cluster-localdisk-0-0 
INFO[0058] Scaling: change ES setting:  delay timeout:1m  and translog durability: async 
INFO[0058] Scaling:------------- sucessfully Completed  for: es-data-es-cluster-localdisk-0 :-------------------------------- 
INFO[0058] Scaling: java_opts Changed by user: %!(EXTRA string=-Xms1072m -Xmx1072m, string= current value: , string=-Xms1062m -Xmx1062m, string= name: , string=es-data-es-cluster-localdisk-1) 
INFO[0058] Scaling:STARTED scaling with new resources ... : es-data-es-cluster-localdisk-1 
INFO[0058] Scaling: ES checking for green               
INFO[0058] Scaling: ES checking for shards name:        
INFO[0059] Scaling: change ES setting:  delay timeout:6m  and translog durability: request 
INFO[0060] Scaling: checking Datanode if it restarted or not by k8: es-data-es-cluster-localdisk-1 
INFO[0069] Scaling: POD started pod_version: 107396%!(EXTRA string= ss_version: %d, int=107369, string= Status: %s, string=&ContainerStateRunning{StartedAt:2018-11-23 17:31:16 +0530 IST,}) 
INFO[0069] Scaling: ES checking if the data node: es-data-es-cluster-localdisk-1-0 joined master  
INFO[0079] Scaling: ES checking for shards name: es-data-es-cluster-localdisk-1-0 
INFO[0079] Scaling: change ES setting:  delay timeout:1m  and translog durability: async 
INFO[0080] Scaling:------------- sucessfully Completed  for: es-data-es-cluster-localdisk-1 :-------------------------------- 
INFO[0080] Scaling: java_opts Changed by user: %!(EXTRA string=-Xms1072m -Xmx1072m, string= current value: , string=-Xms1062m -Xmx1062m, string= name: , string=es-data-es-cluster-localdisk-2) 
INFO[0080] Scaling:STARTED scaling with new resources ... : es-data-es-cluster-localdisk-2 
INFO[0080] Scaling: ES checking for green               
INFO[0080] Scaling: ES checking for shards name:        
INFO[0080] Scaling: change ES setting:  delay timeout:6m  and translog durability: request 
INFO[0082] Scaling: checking Datanode if it restarted or not by k8: es-data-es-cluster-localdisk-2 
INFO[0089] Scaling: POD started pod_version: 107448%!(EXTRA string= ss_version: %d, int=107421, string= Status: %s, string=&ContainerStateRunning{StartedAt:2018-11-23 17:31:36 +0530 IST,}) 
INFO[0089] Scaling: ES checking if the data node: es-data-es-cluster-localdisk-2-0 joined master  
INFO[0099] Scaling: ES checking for shards name: es-data-es-cluster-localdisk-2-0 
INFO[0099] Scaling: change ES setting:  delay timeout:1m  and translog durability: async 
INFO[0100] Scaling:------------- sucessfully Completed  for: es-data-es-cluster-localdisk-2 :-------------------------------- 
INFO[0100] Scaling: java_opts Changed by user: %!(EXTRA string=-Xms1072m -Xmx1072m, string= current value: , string=-Xms1062m -Xmx1062m, string= name: , string=es-data-es-cluster-localdisk-3) 
INFO[0100] Scaling:STARTED scaling with new resources ... : es-data-es-cluster-localdisk-3 
INFO[0100] Scaling: ES checking for green               
INFO[0100] Scaling: ES checking for shards name:        
INFO[0100] Scaling: change ES setting:  delay timeout:6m  and translog durability: request 
INFO[0102] Scaling: checking Datanode if it restarted or not by k8: es-data-es-cluster-localdisk-3 
INFO[0108] Scaling: POD started pod_version: 107498%!(EXTRA string= ss_version: %d, int=107470, string= Status: %s, string=&ContainerStateRunning{StartedAt:2018-11-23 17:31:56 +0530 IST,}) 
INFO[0108] Scaling: ES checking if the data node: es-data-es-cluster-localdisk-3-0 joined master  
INFO[0118] Scaling: ES checking for shards name: es-data-es-cluster-localdisk-3-0 
INFO[0120] Scaling: change ES setting:  delay timeout:1m  and translog durability: async 
INFO[0120] Scaling:------------- sucessfully Completed  for: es-data-es-cluster-localdisk-3 :-------------------------------- 
INFO[0122] --------> ElasticSearch Event finished!

```
