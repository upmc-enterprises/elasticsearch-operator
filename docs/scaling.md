
# Patch Details:

The following are  changes present in scaling patch

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

apiVersion: enterprises.upmc.com/v1
kind: ElasticsearchCluster
metadata:
  clusterName: ""
  creationTimestamp: 2018-11-19T08:49:23Z
  generation: 1
  name: es-cluster
  namespace: default
  resourceVersion: "1636643"
  selfLink: /apis/enterprises.upmc.com/v1/namespaces/default/elasticsearchclusters/es-cluster
  uid: 03267c65-ebd8-11e8-8e6a-000d3a000cf8
spec:
  cerebro:
    image: hub.docker.prod.walmart.com/upmcenterprises/cerebro:0.6.8
  client-node-replicas: 1
  data-node-replicas: 4
  data-volume-size: 10Gi
  elastic-search-image: quay.docker.prod.walmart.com/pires/docker-elasticsearch-kubernetes:6.3.2
  java-options: -Xms1052m -Xmx1052m
  master-node-replicas: 1
  network-host: 0.0.0.0
  resources:
    limits:
      cpu: 4m
      memory: 3048Mi
    requests:
      cpu: 3m
      memory: 2024Mi
  scaling:
    java-options: -Xms1078m -Xmx1078m
    resources:
      limits:
        cpu: 5m
        memory: 2048Mi
      requests:
        cpu: 4m
        memory: 1024Mi
  storage:
    storage-class: standard-disk
  zones: []
```

# TestCases :

# Testcase-1 : Normal scaling operation on Network block storage.
 - Description:  Scaling operation can be triggered by changing the scaling parameters in ElasticSearch Cluster configuration,this can be done using "kubectl edit ..". If there is any change in jvm-heap memory in java-options or cpu cores inside the scaling section then scaling operation will be triggered. This can be observed in elastic search operator log. During scaling operation, the following should be full filled: a) At any time only one data node should be restarted, b) The State of ElasticSearch cluster should not be entered into Red state anytime during scaling operation. In case if the Elasticsearch cluster enter in to Redstate then the scaling operation will be automatically halted by the elasaticsearch opeartor. 
- Steps to Reproduce the test:
    - step-1: edit the configuration of elastic cluster, this can done using "kubectl edit,..", change the parameters like jvm heap memory size or cpu cores inside the scaling section and save the file. 
    - step-2:  After step-1, elastic search operator will receive the signal about the change in configuration of cluster, then the scaling code will check if there is any change in parameters related to scaling, this triggers scaling operation, this can be monitored in operator log.
    - step-3: elasticsearch operator will update the data node pods one after another, and making sure the elastic search cluster is in yellow/green. 
- Test Status: Passed.

# Testcase-2 : Stop and start the ES operator during scaling operation.
- Description:   when the scaling operation is halfway, stop the elasticsearch operator.  example: out 20 nodes, 10 nodes have completed scaling, during that time stop the elastic operator, and start the elasticsearch operator after few minutes, when the elasticsearch operator is  started then scaling of the rest of nodes should  continue where it was stopped last time instead of starting from the beginning. 
- Steps to Reproduce the test:
    - step-1: edit the configuration of elastic cluster, this can done using "kubectl edit,..", change the parameters like jvm heap memory size or cpu cores inside the scaling section and save the file.
    - step-2: After step-1, elastic search operator will receive the signal about the change in configuration of cluster, then the scaling code will check if there is any change in parameters related to scaling, this triggers scaling operation, this can be monitored in operator log.
    - step-3: Elasticsearch operator will update the data node pods one after another, and making sure the elastic search cluster is in yellow/green. 
    - step-4: After scaling is half way of the data nodes, stop the elastic search operator.example: out of 20 nodes, after completing 10 nodes, stop the ES operator.
    - step-5: Wait for 10 min, and start ES operator 
    - step-6: When the ES operator is started, rest of the datanodes will get scaled without starting from the start.
    - step-7: check in elasticsearch cluster the starting time of each data node. half of the data nodes would have started 10 min later.
 - Test Status: Passed.

# Testcase-3 : Trigger scaling operation when Elastic cluster is in "Yellow/Red" state
- Description:  When the Elastic cluster in non-green state, and if scaling operation is started it  should not start scaling operation. means not single data pod should be restarted. Scaling operation should be started only if the Elastic cluster is in green state.
- Steps to Reproduce the test:
    - step-1: make the elastic search cluster to change in to yellow state by stopping one of the data node or by other means.
    - step-2: edit the configuration of elastic cluster, this can done using "kubectl edit,..", change the parameters like jvm heap memory size or cpu cores inside the scaling section and save the file.
    - step-3: After step-2, elastic search operator will receive the signal about the change in configuration of cluster, then the scaling code will check if there is any change in parameters related to scaling, this triggers scaling operation, this can be monitored in operator log.
    - step-4: scaling of Data nodes will started, but immedietly it will generate an error saying ES cluster is in yellow state and scaling operation is aborted.
- Test Status : Passed.

# Testcase-4 :  Trigger of scaling operation: changes in Non-scaling parameter
- Description: If there is any change in non-scalar parameter, then scaling operation should not be triggered. Scaling operation should be triggered only of there is change in Java-options, cpu or memory inside scaling section.  To trigger scaling atleast any one of jvm heap or cpu is required.
- Steps to Reproduce the test:
    - step-1: edit the configuration of elastic cluster, this can done using "kubectl edit,..", change the parameters not related to scaling.
    - step-2: After step-1, elastic search operator will receive the signal about the change in configuration of cluster, but scaling operaton of data nodes will not started saying there is no change in memory and cpu.
- Test Status: Passed

# Testcase-5 : Normal scaling operation on local/nfs storage.
- Description: Similar to Testcase-1 except localdisk is set as storage-class instead of network block storage.
- Steps to Reproduce the test:
    - step-1: edit the configuration of elastic cluster, this can done using "kubectl edit,..", change the parameters like jvm heap memory size or cpu cores inside the scaling section and save the file. 
    - step-2:  After step-1, elastic search operator will receive the signal about the change in configuration of cluster, then the scaling code will check if there is any change in parameters related to scaling, this triggers scaling operation, this can be monitored in operator log.
    - step-3: elasticsearch operator will update the data node pods one after another, and making sure the elastic search cluster is in yellow/green.
- Test status : Failed .
- Reason for Failure: Since all data nodes share same statefull set, the mount point for all data nodes is same, due to this second data nodes will not comesup, this need to addressed in future.
  
    
          