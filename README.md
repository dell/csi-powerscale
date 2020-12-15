# CSI Driver for PowerScale

[![Go Report Card](https://goreportcard.com/badge/github.com/dell/csi-isilon)](https://goreportcard.com/report/github.com/dell/csi-isilon)
[![License](https://img.shields.io/github/license/dell/csi-isilon)](https://github.com/dell/csi-isilon/blob/master/LICENSE)
[![Docker](https://img.shields.io/docker/pulls/dellemc/csi-isilon.svg?logo=docker)](https://hub.docker.com/r/dellemc/csi-isilon)
[![Last Release](https://img.shields.io/github/v/release/dell/csi-isilon?label=latest&style=flat-square)](https://github.com/dell/csi-isilon/releases)

## Description
CSI Driver for PowerScale is a Container Storage Interface ([CSI](https://github.com/container-storage-interface/spec))
driver for Dell EMC PowerScale. It supports CSI specification version 1.1.

This project may be compiled as a stand-alone binary using Golang that, when
run, provides a valid CSI endpoint. This project can also be built
as a Golang plug-in in order to extend the functionality of other programs.

## Support
The CSI Driver for Dell EMC PowerScale image, which is the built driver code, is available on Dockerhub and is officially supported by Dell EMC.

The source code for CSI Driver for Dell EMC PowerScale available on Github is unsupported and provided solely under the terms of the license attached to the 
source code. 

For clarity, Dell EMC does not provide support for any source code modifications.

For any CSI driver issues, questions or feedback, join the [Dell EMC Container community](<https://www.dell.com/community/Containers/bd-p/Containers/>)

## Overview

PowerScale CSI plugins implement an interface between CSI enabled Container Orchestrator(CO) and PowerScale Storage Array. It allows dynamically provisioning PowerScale volumes 
and attaching them to workloads.

## Introduction
The CSI Driver For Dell EMC PowerScale conforms to CSI spec 1.1
   * Support for Kubernetes 1.17, 1.18 and 1.19
   * Will add support for other orchestrators over time
   * The CSI specification is documented here: https://github.com/container-storage-interface/spec. The driver uses CSI v1.1.

## CSI Driver For Dell EMC PowerScale Capabilities

|Capability | Supported | Not supported |
|------------|-----------| --------------|
|Provisioning | Static and dynamic provisioning of volumes, Volume expansion, Create volume from snapshots, Volume Cloning, CSI Ephemeral Inline Volumes| Generic Ephemeral Volumes |
|Export, Mount | Mount volume as file system, Topology support for volumes, mount options | Raw volumes|
|Data protection | Creation of snapshots| |
|Installer | Helm3, Dell CSI Operator (for OpenShift platform) | |
|Access mode | SINGLE_NODE_WRITER , MULTI_NODE_READER_ONLY , MULTI_NODE_MULTI_WRITER|
|Kubernetes | v1.17, v1.18, v1.19 | v1.15 or previous versions, v1.20 or higher versions|
|OS | RHEL 7.x, CentOS 7.x, Ubuntu 20.0.4 | other Linux variants|
|PowerScale | OneFS 8.1, 8.2, 9.0 and 9.1 | Previous versions|
|Protocol | NFS | SMB, CIFS|
|OpenShift| 4.5, 4.6 |
|Docker EE| 3.1* |

Note: CoreOS worker nodes are only supported with RedHat OpenShift platform

## Installation overview

Installation in a Kubernetes cluster should be done using the scripts within the `dell-csi-helm-installer` directory. 

For more information, consult the [README.md](dell-csi-helm-installer/README.md)

The controller section of the Helm chart installs the following components in a Stateful Set:

* CSI Driver for PowerScale
* Kubernetes Provisioner, which provisions the provisioning volumes
* Kubernetes Attacher, which attaches the volumes to the containers
* Kubernetes Snapshotter, which provides snapshot support
* Kubernetes Resizer, which provides resize support

The node section of the Helm chart installs the following component in a Daemon Set:

* CSI Driver for PowerScale
* Kubernetes Registrar, which handles the driver registration

### Prerequisites

Before you install CSI Driver for PowerScale, verify the requirements that are mentioned in this topic are installed and configured.

#### Requirements

* Install Kubernetes.
* Configure Docker service
* Install Helm v3
* Install volume snapshot components
* Deploy PowerScale driver using Helm

**Note:** There is no feature gate that needs to be set explicitly for csi drivers from 1.17 onwards. All the required feature gates are either beta/GA.

## Configure Docker service

The mount propagation in Docker must be configured on all Kubernetes nodes before installing CSI Driver for PowerScale.

### Procedure

1. Edit the service section of */etc/systemd/system/multi-user.target.wants/docker.service* file as follows:

    ```
    [Service]
    ...
    MountFlags=shared
    ```
    
2. Restart the Docker service with systemctl daemon-reload and

    ```
    systemctl daemon-reload
    systemctl restart docker
    ```

## Install volume snapshot components

### Install Snapshot Beta CRDs
To install snapshot crds specify `--snapshot-crd` flag to driver installation script `dell-csi-helm-installer/csi-install.sh` during driver installation

### [Install Common Snapshot Controller](<https://kubernetes.io/blog/2019/12/09/kubernetes-1-17-feature-cis-volume-snapshot-beta/#how-do-i-deploy-support-for-volume-snapshots-on-my-kubernetes-cluster>), if not already installed for the cluster

    ```
    kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v3.0.2/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
    kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v3.0.2/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
    ```

## Install CSI Driver for PowerScale

Install CSI Driver for PowerScale using this procedure.

*Before you begin*
 * You must clone the source [git repository](https://github.com/dell/csi-isilon), ready for below procedure.
 * In the `dell-csi-helm-installer` directory, there should be two shell scripts, *csi-install.sh* and *csi-uninstall.sh*. These scripts
   handle some of the pre and post operations that cannot be performed in the helm chart.

Procedure

1. Collect information from the PowerScale Systems like IP address, username  and password. Make a note of the value for these parameters as they must be entered in the secret.yaml and values file.

2. Copy the helm/csi-isilon/values.yaml into a new location with name say *my-isilon-settings.yaml*, to customize settings for installation.

3. Edit *my-isilon-settings.yaml* to set the following parameters for your installation:
    
    The following table lists the primary configurable parameters of the PowerScale driver helm chart and their default values. More detailed information can be
   found in the  [`values.yaml`](helm/csi-isilon/values.yaml) file in this repository.
    
    | Parameter | Description | Required | Default |
    | --------- | ----------- | -------- |-------- |    
    | isiIP | "isiIP" defines the HTTPs endpoint of the PowerScale OneFS API server | true | - |
    | isiPort | "isiPort" defines the HTTPs port number of the PowerScale OneFS API server | false | 8080 |
    | isiInsecure | "isiInsecure" specifies whether the PowerScale OneFS API server's certificate chain and host name should be verified. | false | true |
    | isiAccessZone | The name of the access zone a volume can be created in | false | System |
    | volumeNamePrefix | "volumeNamePrefix" defines a string prepended to each volume created by the CSI driver. | false | k8s |
    | controllerCount | "controllerCount" defines the number of csi-powerscale controller nodes to deploy to the Kubernetes release.| true | 2 |   
    | enableDebug | Indicates whether debug level logs should be logged | false | true |
    | verbose | Indicates what content of the OneFS REST API message should be logged in debug level logs | false | 1 |
    | enableQuota | Indicates whether the provisioner should attempt to set (later unset) quota on a newly provisioned volume. This requires SmartQuotas to be  enabled.| false | true |
    | noProbeOnStart | Indicates whether the controller/node should probe during initialization | false | false |
    | isiPath | The default base path for the volumes to be created, this will be used if a storage class does not have the IsiPath parameter specified| false
    | /ifs/data/csi |
    | autoProbe |  Enable auto probe. | false | true |
    | nfsV3 | Specify whether to set the version to v3 when mounting an NFS export. If the value is "false", then the default version supported will be used (i.e. the mount command will not explicitly specify "-o vers=3" option). This flag has now been deprecated and will be removed in a future release. Please useStorageClass.mountOptions if you want to specify 'vers=3' as a mount option. | false | false |
    | enableCustomTopology | Indicates PowerScale FQDN/IP which will be fetched from node label and the same will be used by controller and node pod to establish connection to Array. This requires enableCustomTopology to be enabled. | false | false |
    | ***Storage Class parameters*** | Following parameters are related to Storage Class |
    | name | "storageClass.name" defines the name of the storage class to be defined. | false | isilon |
    | isDefault | "storageClass.isDefault" defines whether the primary storage class should be the default. | false | true |    
    | reclaimPolicy | "storageClass.reclaimPolicy" defines what will happen when a volume is removed from the Kubernetes API. Valid values are "Retain" and "Delete".| false | Delete |
    | accessZone | The Access Zone where the Volume would be created | false | System |
    | AzServiceIP | Access Zone service IP if different from isiIP, specify here and refer in storageClass | false |  |
    | rootClientEnabled |  When a PVC is being created, it takes the storage class' value of "storageclass.rootClientEnabled"| false | false |
    | ***Controller parameters*** | Set nodeSelector and tolerations for controller |
    | nodeSelector | Define nodeSelector for the controllers, if required | false | |
    | tolerations | Define tolerations for the controllers, if required | false | |
    
    Note: User should provide all boolean values with double quotes. This applicable only for my-isilon-settings.yaml. Ex: "true"/"false"

    Note: controllerCount parameter value should not exceed number of nodes in the kubernetes cluster. Otherwise some of the controller pods will be in "Pending" state till new nodes are available for scheduling. The installer will exit with a WARNING on the same.

4. Create namespace

    Run `kubectl create namespace isilon` to create the isilon namespace. Specify the same namespace name while installing the driver. 
    Note: CSI PowerScale also supports installation of driver in custom namespace.

5. Create a secret file for the OneFS credentials by editing the secret.yaml present under helm directory. Replace the values for the username and password parameters.

    Use the following command to convert username/password to base64 encoded string
    ```
    echo -n 'admin' | base64
    echo -n 'password' | base64 
    ```  
    Run `kubectl create -f secret.yaml` to create the secret.

    Note: The username specified in secret.yaml must be from the authentication providers of PowerScale. The user must have enough privileges to perform the actions. The suggested privileges are as follows:
    ```
    ISI_PRIV_LOGIN_PAPI
    ISI_PRIV_NFS
    ISI_PRIV_QUOTA
    ISI_PRIV_SNAPSHOT
    ISI_PRIV_IFS_RESTORE
    ISI_PRIV_NS_IFS_ACCESS
    ISI_PRIV_LOGIN_SSH
   ```

6. Install OneFS CA certificates by following the instructions from next section, if you want to validate OneFS API server's certificates. If not, create an empty
   secret  using the following command and empty secret should be created for the successful CSI Driver for DELL EMC Powerscale installation.
    ```
    kubectl create -f emptysecret.yaml
    ```

7. Install CSI driver for PowerScale by following the instructions from [README](dell-csi-helm-installer/README.md)


## Certificate validation for OneFS REST API calls 

The CSI driver exposes an install parameter 'isiInsecure' which determines if the driver
performs client-side verification of the OneFS certificates. The 'isiInsecure' parameter is set to true by default and the driver does not verify the OneFS certificates.

If the isiInsecure is set to false, then the secret isilon-certs must contain the CA certificate for OneFS. 
If this secret is an empty secret, then the validation of the certificate fails, and the driver fails to start.

If the isiInsecure parameter is set to false and a previous installation attempt to create the empty secret, then this secret must be deleted and re-created using the CA certs. If the OneFS certificate is self-signed, then perform the following steps:

### Procedure

1. To fetch the certificate, run `openssl s_client -showcerts -connect <OneFS
IP> </dev/null 2>/dev/null | openssl x509 -outform PEM > ca_cert.pem`

2. To create the secret, run `kubectl create secret generic isilon-certs --from-file=ca_cert.pem -n isilon`

## Upgrade CSI Driver for DELL EMC PowerScale from version v1.3.0 to v1.4.0

* Verify that all pre-requisites to install CSI Driver for DELL EMC PowerScale v1.4.0 are fulfilled.

* Clone the repository https://github.com/dell/csi-powerscale , Copy the helm/csi-isilon/values.yaml into a new location with name say my-isilon-settings.yaml,
  to customize settings for installation. Edit my-isilon-settings.yaml as per the requirements.

* Change to directory dell-csi-helm-installer to install the DELL EMC PowerScale 
   `cd dell-csi-helm-installer`

* Upgrade the CSI Driver for DELL EMC PowerScale v1.4.0 using following command.

   ##### `./csi-install.sh --namespace isilon --values ./my-isilon-settings.yaml --upgrade`
 
## Test deploying a simple pod with PowerScale storage

Test the deployment workflow of a simple pod on PowerScale storage.

1. **Creating a volume:**

    Create a file `pvc.yaml` using sample yaml files located at test/sample_files/


    Execute the following command to create volume
    ```
    kubectl create -f $PWD/pvc.yaml
    ```

    Result: After executing the above command PVC will be created in the default namespace, and the user can see the pvc by executing `kubectl get pvc`. 
    Note: Verify system for the new volume

3. **Attach the volume to Host**

    To attach a volume to a host, create a new application(Pod) and use the PVC created above in the Pod. This scenario is explained using the Nginx application. Create `nginx.yaml` 
    using sample yaml files located at test/sample_files/.

    Execute the following command to mount the volume to Kubernetes node
    ```
    kubectl create -f $PWD/nginx.yaml
    ```

    Result: After executing the above command, new nginx pod will be successfully created and started in the default namespace.
    Note: Verify PowerScale system for host to be part of clients/rootclients field of export created for volume and used by nginx application.

4. **Create Snapshot**

    The following procedure will create a snapshot of the volume in the container using VolumeSnapshot objects defined in snap.yaml. The sample file for snapshot creation is located 
    at test/sample_files/
    
    Execute the following command to create snapshot
    ```
    kubectl create -f $PWD/snap.yaml
    ```
    
    The spec.source section contains the volume that will be snapped in the default namespace. For example, if the volume to be snapped is testvolclaim1, then the created snapshot is named testvolclaim1-snap1. Verify the PowerScale system for newly created snapshot.
    
    Note:
    
    * User can see the snapshots using `kubectl get volumesnapshot`
    * Notice that this VolumeSnapshot class has a reference to a snapshotClassName:isilon-snapclass. The CSI Driver for PowerScale installation creates this class 
      as its default snapshot class. 
    * You can see its definition using `kubectl get volumesnapshotclasses isilon-snapclass -o yaml`.

5. **Create Volume from Snapshot**

    The following procedure will create a new volume from a given snapshot which is specified in spec dataSource field.
    
    The sample file for volume creation from snapshot is located under test/sample_files/
    
    Execute the following command to create snapshot
    ```
    kubectl create -f $PWD/volume_from_snap.yaml
    ```

    Verify the PowerScale system for newly created volume from snapshot.

6. **Delete Snapshot**

    Execute the following commands to delete the snapshot
    
    ```
    kubectl get volumesnapshot
    kubectl delete volumesnapshot testvolclaim1-snap1
    ```

7. **Create new volume from existing volume(volume clone)**

    The following procedure will create a new volume from another existing volume which is specified in spec dataSource field.
    
    The sample file for volume creation from volume is located at test/sample_files/
    
    Execute the following command to create snapshot
    ```
    kubectl create -f $PWD/volume_from_volume.yaml
    ```

    Verify the PowerScale system for new created volume from volume.

8.  **To Unattach the volume from Host**

    Delete the nginx application to unattach the volume from host
    
    `kubectl delete -f nginx.yaml`

9.  **To delete the volume**

    ```
    kubectl get pvc
    kubectl delete pvc testvolclaim1
    kubectl get pvc
    ```
## Topology Support 

   From version 1.4.0, the CSI Powerscale driver supports Topology by default which forces volumes to be placed on worker nodes that have connectivity to the backend storage, as a result of which the nodes which have access to PowerScale Array are appropriately labelled. The driver leverages these labels to ensure that the driver components (controller, node) are spawned only on nodes wherein these labels exist. 
  
   This covers use cases where:
 
   The csi-powerscale Driver may not be installed or running on some nodes where Users have chosen to restrict the nodes on accessing the powerscale storage array. 

   We support CustomTopology which enables users to apply labels for nodes - "csi-isilon.dellemc.com/XX.XX.XX.XX=csi-isilon.dellemc.com" and expect the labels to be 
   honored by the driver.
   
   When “enableCustomTopology” is set to “true”, CSI driver fetches custom labels “csi-isilon.dellemc.com/XX.XX.XX.XX=csi-isilon.dellemc.com” applied on worker nodes, and use them to initialize node pod with custom PowerScale FQDN/IP.


## Topology Usage
 
   In order to utilize the Topology feature create a custom storage class with volumeBindingMode set to WaitForFirstConsumer and specify the desired topology labels within allowedTopologies field of this custom storage class. This ensures that pod scheduling takes advantage of the topology and the node selected has access to provisioned volumes.

   A sample manifest file is available at `helm/samples/storageclass/isilon.yaml` to create a storage class with Topology support.
 
For additional information, see the [Kubernetes Topology documentation](https://kubernetes-csi.github.io/docs/topology.html).

## Volume creation from datasource (i.e., from another volume or snapshot)

Volumes can be created by pre-populating data in them from a data source. The data source can be another existing volume or a snapshot.

For volume request from another volume, the PowerScale user must have SSH privilege(ISI_PRIV_LOGIN_SSH) assigned to him.

For READ-WRITE volume request from snapshot, the PowerScale user must have SSH privilege(ISI_PRIV_LOGIN_SSH) assigned to him.

Note: Only one READ-ONLY volume can be created from snapshot at any point in time. This operation is space efficient and fast compared to READ-WRITE volumes. 


## Install CSI-PowerScale driver using dell-csi-operator in OpenShift

CSI Driver for Dell EMC PowerScale can also be installed via the new Dell EMC Storage Operator.

The Dell EMC Storage CSI Operator is a Kubernetes Operator, which can be used to install and manage the CSI Drivers provided by Dell EMC for various storage platforms. This operator is available as a community operator for upstream Kubernetes and can be deployed using OperatorHub.io. It is also available as a community operator for OpenShift clusters and can be deployed using OpenShift Container Platform. Both these methods of installation use OLM (Operator Lifecycle Manager).
 
The operator can also be deployed directly by following the instructions available here - https://github.com/dell/dell-csi-operator.
 
There are sample manifests provided which can be edited to do an easy installation of the driver. Please note that the deployment of the driver using the operator doesn’t use any Helm charts and the installation & configuration parameters will be slightly different from the ones specified via the Helm installer.

Kubernetes Operators make it easy to deploy and manage entire lifecycle of complex Kubernetes applications. Operators use Custom Resource Definitions (CRD) which represents the application and use custom controllers to manage them.

### Listing CSI-PowerScale drivers
User can query for csi-powerscale driver using the following command
`kubectl get csiisilon --all-namespaces`

### Procedure to create new CSI-PowerScale driver

1. Create namespace

   Run `kubectl create namespace isilon` to create the isilon namespace.
   
2. Create *isilon-creds*
   
   Create a file called isilon-creds.yaml with the following content
     ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
      name: isilon-creds
      namespace: isilon
    type: Opaque
    data:
      # set username to the base64 encoded username
      username: <base64 username>
      # set password to the base64 encoded password
      password: <base64 password>
    ```
   
   Replace the values for the username and password parameters. These values can be optioned using base64 encoding as described in the following example:
   ```
   echo -n "myusername" | base64
   echo -n "mypassword" | base64
   ```
   
   Run `kubectl create -f isilon-creds.yaml` command to create the secret.
 
3. Create a CR (Custom Resource) for PowerScale using the sample files provided 
   [here](https://github.com/dell/dell-csi-operator/tree/master/samples) .

4.  Execute the following command to create PowerScale custom resource
    ```kubectl create -f <input_sample_file.yaml>``` .
    The above command will deploy the CSI-PowerScale driver.
 
5. User can configure the following parameters in CR
       
   The following table lists the primary configurable parameters of the PowerScale driver and their default values.
   
   | Parameter | Description | Required | Default |
   | --------- | ----------- | -------- |-------- |
   | ***Common parameters for node and controller*** |
   | CSI_ENDPOINT | The UNIX socket address for handling gRPC calls | No | /var/run/csi/csi.sock |
   | X_CSI_DEBUG | To enable debug mode | No | false |
   | X_CSI_ISI_ENDPOINT | HTTPs endpoint of the PowerScale OneFS API server | Yes | |
   | X_CSI_ISI_INSECURE | Specifies whether SSL security needs to be enabled for communication between PowerScale and CSI Driver | No | true |
   | X_CSI_ISI_PATH | Base path for the volumes to be created | Yes | |
   | X_CSI_ISI_AUTOPROBE | To enable auto probing for driver | No | true |
   | X_CSI_ISILON_NO_PROBE_ON_START | Indicates whether the controller/node should probe during initialization | Yes | |
   | ***Controller parameters*** |
   | X_CSI_MODE   | Driver starting mode  | No | controller |
   | X_CSI_ISI_ACCESS_ZONE | Name of the access zone a volume can be created in | No | System |
   | X_CSI_ISI_QUOTA_ENABLED | To enable SmartQuotas | Yes | |
   | ***Node parameters*** |
   | X_CSI_ISILON_NFS_V3 | Set the version to v3 when mounting an NFS export. If the value is "false", then the default version supported will be used | Yes | |
   | X_CSI_MODE   | Driver starting mode  | No | node |

## Support for Docker EE

The CSI Driver for Dell EMC PowerScale supports Docker EE & deployment on clusters bootstrapped with UCP (Universal Control Plane).

*UCP version 3.3.3 supports kubernetes 1.18 and CSI driver can be installed on UCP 3.1 with Helm. With Docker EE 3.1, we also supports those UCP versions which 
leverage k8s 1.17.

The installation process for the driver on such clusters remains the same as the installation process on upstream clusters.

On UCP based clusters, kubectl may not be installed by default, it is important that [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) is installed prior to the installation of the driver.

The worker nodes in UCP backed clusters may run any of the OSs which we support with upstream clusters.
