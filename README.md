# Isilon CSI

This repo contains [Container Storage Interface(CSI)]
(<https://github.com/container-storage-interface/>) Isilon CSI driver for DellEMC.

## Overview

Isilon CSI plugins implement an interface between CSI enabled Container Orchestrator(CO) and Isilon Storage Array. It allows dynamically provisioning Isilon volumes and attaching them to workloads.

## Introduction
The CSI Driver For Dell EMC Isilon conforms to CSI spec 1.1
   * Support for Kubernetes 1.14 and 1.16
   * Will add support for other orchestrators over time
   * The CSI specification is documented here: https://github.com/container-storage-interface/spec. The driver uses CSI v1.1.

## CSI Driver For Dell EMC Isilon Capabilities

| Capability | Supported | Not supported |
|------------|-----------| --------------|
|Provisioning | Static and dynamic provisioning of volumes, Volume expansion| 
|Export, Mount | Mount volume as file system | Raw volumes, Topology|
|Data protection | Creation of snapshots*, Create volume from snapshots*| Volume Cloning|
|Installer | Helm2, Helm3, Dell CSI Operator (for OpenShift platform) | |
|Access mode | SINGLE_NODE_WRITER , MULTI_NODE_READER_ONLY , MULTI_NODE_MULTI_WRITER|
|Kubernetes | v1.14, v1.16 | v1.13 or previous versions, v1.17 or higher versions|
|OS | RHEL 7.6, 7.7 | Ubuntu, other Linux variants|
|Isilon | OneFS 8.1, 8.2 and 9.0 | Previous versions|
|Protocol | NFS | SMB, CIFS|
|OpenShift| 4.2, 4.3 |

Note: Snapshots related operations are not supported on OpenShift platform with CSI-Isilon driver.

## Installation overview

The Helm chart installs CSI Driver for Isilon using a shell script (helm/install.isilon). This script installs the CSI driver container image along with the required Kubernetes sidecar containers.

The controller section of the Helm chart installs the following components in a Stateful Set in the namespace isilon:

* CSI Driver for Isilon
* Kubernetes Provisioner, which provisions the provisioning volumes
* Kubernetes Attacher, which attaches the volumes to the containers
* Kubernetes Snapshotter, which provides snapshot support
* Kubernetes Resizer, which provides resize support

The node section of the Helm chart installs the following component in a Daemon Set in the namespace isilon:

* CSI Driver for Isilon
* Kubernetes Registrar, which handles the driver registration

### Prerequisites

Before you install CSI Driver for Isilon, verify the requirements that are mentioned in this topic are installed and configured.

#### Requirements

* Install Kubernetes.
* Enable the Kubernetes feature gates
* Configure Docker service
* Install Helm v2 with Tiller with a service account or Helm v3
* Deploy Isilon using Helm

## Enable Kubernetes feature gates

The Kubernetes feature gates must be enabled before installing CSI Driver for Isilon.

#### About Enabling Kubernetes feature gates

The Feature Gates section of Kubernetes home page lists the Kubernetes feature gates. The following Kubernetes feature gates must be enabled:

* VolumeSnapshotDataSource
* CSINodeInfo
* CSIDriverRegistry

### Procedure

 1. On each master and node of Kubernetes, edit /var/lib/kubelet/config.yaml and append the following lines at the end to set feature-gate settings for the kubelets:
    */var/lib/kubelet/config.yaml*

    ```
    VolumeSnapshotDataSource: true
    CSINodeInfo: true
    CSIDriverRegistry: true
    ```

2. On the master node, set the feature gate settings of the kube-apiserver.yaml, kube-controllermanager.yaml and kube-scheduler.yaml file as follows:

    */etc/kubernetes/manifests/kube-apiserver.yaml
    /etc/kubernetes/manifests/kube-controller-manager.yaml
    /etc/kubernetes/manifests/kube-scheduler.yaml*

    ```
    - --feature-gates=VolumeSnapshotDataSource=true,CSINodeInfo=true,CSIDriverRegistry=true
    ```

3. On each node (including master), edit the variable **KUBELET_KUBECONFIG_ARGS** of /usr/lib/systemd/system/kubelet.service.d/10-kubeadm.conf file as follows:

    ```
    Environment="KUBELET_KUBECONFIG_ARGS=--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf --feature-gates=VolumeSnapshotDataSource=true,CSINodeInfo=true,CSIDriverRegistry=true" 
    ```

4. Restart the kublet on all nodes. 

    ```
    systemctl daemon-reload
    systemctl restart kubelet 
    ```

## Configure Docker service

The mount propagation in Docker must be configured on all Kubernetes nodes before installing CSI Driver for Isilon.

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

## Install CSI Driver for Isilon

Install CSI Driver for Isilon using this procedure.

*Before you begin*
 * You must have the downloaded files, including the Helm chart from the source [git repository](https://github.com/dell/csi-isilon), ready for this procedure.
 * In the top-level helm directory, there should be two shell scripts, *install.isilon* and *uninstall.isilon*. These scripts handle some of the pre and post operations that cannot be performed in the helm chart, such as creating Custom Resource Definitions (CRDs), if needed.

Procedure

1. Collect information from the Isilon Systems like IP address, username  and password. Make a note of the value for these parameters as they must be entered in the secret.yaml and myvalues.yaml file.

2. Copy the csi-isilon/values.yaml into a file in the same directory as the install.isilon named myvalues.yaml, to customize settings for installation.

3. Edit myvalues.yaml to set the following parameters for your installation:
    
    The following table lists the primary configurable parameters of the Isilon driver chart and their default values. More detailed information can be found in the [`values.yaml`](helm/csi-isilon/values.yaml) file in this repository.
    
    | Parameter | Description | Required | Default |
    | --------- | ----------- | -------- |-------- |    
    | isiIP | "isiIP" defines the HTTPs endpoint of the Isilon OneFS API server | true | - |
    | isiPort | "isiPort" defines the HTTPs port number of the Isilon OneFS API server | false | 8080 |
    | isiInsecure | "isiInsecure" specifies whether the Isilon OneFS API server's certificate chain and host name should be verified. | false | true |
    | isiAccessZone | The name of the access zone a volume can be created in | false | System |
    | volumeNamePrefix | "volumeNamePrefix" defines a string prepended to each volume created by the CSI driver. | false | k8s |
    | controllerCount | "controllerCount" defines the number of csi-isilon controller nodes to deploy to the Kubernetes release.| false | 1 |
    | enableDebug | Indicates whether debug level logs should be logged | false | false |
    | verbose | Indicates what content of the OneFS REST API message should be logged in debug level logs | false | false |
    | enableQuota | Indicates whether the provisioner should attempt to set (later unset) quota on a newly provisioned volume. This requires SmartQuotas to be enabled.| false | false |
    | noProbeOnStart | Indicates whether the controller/node should probe during initialization | false | false |
    | isiPath | The default base path for the volumes to be created, this will be used if a storage class does not have the IsiPath parameter specified| false | /ifs/data/csi |
    | autoProbe |  Enable auto probe. | false | true |
    | nfsV3 | Specify whether to set the version to v3 when mounting an NFS export. If the value is "false", then the default version supported will be used (i.e. the mount command will not explicitly specify "-o vers=3" option) | false | false |
    | ***Storage Class parameters*** | Following parameters are related to Storage Class |
    | name | "storageClass.name" defines the name of the storage class to be defined. | false | isilon |
    | isDefault | "storageClass.isDefault" defines whether the primary storage class should be the default. | false | true |    
    | reclaimPolicy |    "storageClass.reclaimPolicy" defines what will happen when a volume is removed from the Kubernetes API. Valid values are "Retain" and "Delete".| false | Delete |
    | accessZone | The Access Zone where the Volume would be created | false | System |
    | AzServiceIP | Access Zone service IP if different from isiIP, specify here and refer in storageClass | false |  |
    | rootClientEnabled |  When a PVC is being created, it takes the storage class' value of "storageclass.rootClientEnabled"| false | false |

    Note: User should provide all boolean values with double quotes. This applicable only for myvalues.yaml. Ex: "true"/"false"
   
4. Create a secret file for the OneFS credentials by editing the secret.yaml present under helm directory. Replace the values for the username and password parameters.

    Use the following command to convert username/password to base64 encoded string
    ```
    echo -n 'admin' | base64
    echo -n 'password' | base64 
    ```

    Run `kubectl create namespace isilon` to create the isilon namespace.
    Run `kubectl create -f secret.yaml` to create the secret.
    
    
4. Run the `sh install.isilon` command to proceed with the installation.

    A successful installation should emit messages that look similar to the following samples:
    ```
    NAME                  READY   STATUS             RESTARTS   AGE
    isilon-controller-0   5/5     Running              0         20s
    isilon-node-97fph     2/2     Running              0         20s
    CSIDrivers:
    NAME     CREATED AT
    isilon   2020-05-28T02:55:33Z
    CSINodes:
    NAME       CREATED AT
    lglou233   2020-04-07T11:07:21Z
    StorageClasses:
    NAME               PROVISIONER              AGE
    isilon (default)   csi-isilon.dellemc.com   20s
    crd VolumeSnapshotClass was already created
    installing volumesnapshotclass instance isilon-snapclass
    volumesnapshotclass.snapshot.storage.k8s.io/isilon-snapclass created
    ```

    Results
    At the end of the script, the kubectl get pods -n isilon is called to GET the status of the pods and you will see the following:
    * isilon-controller-0 with 5/5 containers ready, and status displayed as Running.
    * Agent pods with 2/2 containers and the status displayed as Running.

    Finally, the script lists the created storageclasses such as, "isilon". Additional storage classes can be created for different combinations of file system types and Isilon storage pools. The script also creates volumesnapshotclass "isilon-snapclass".

## Certificate validation for Unisphere REST API calls 

The CSI driver exposes an install parameter 'isiInsecure' which determines if the driver
performs client-side verification of the OneFS certificates. The 'isiInsecure' parameter is set to true by default and the driver does not verify the OneFS certificates.

If the isiInsecure is set to false, then the secret isilon-certs must contain the CA certificate for OneFS. 
If this secret is an empty secret, then the validation of the certificate fails, and the driver fails to start.

If the isiInsecure parameter is set to false and a previous installation attempt created the empty secret, then this secret must be deleted and re-created using the CA certs.
If the OneFS certificate is self-signed, then perform the following steps:

### Procedure

1. To fetch the certificate, run `openssl s_client -showcerts -connect <OneFS
IP> </dev/null 2>/dev/null | openssl x509 -outform PEM > ca_cert.pem`

2. To create the secret, run `kubectl create secret generic isilon-certs --from-file=ca_cert.pem -n isilon`

## Upgrade CSI Driver for DELL EMC Isilon from previous versions
The CSI Driver for Dell EMC Isilon prior to v1.2.0 supports only Helm 2. Users can upgrade the driver using Helm 2 or Helm 3.
Use one of the following two approaches to upgrade the CSI Driver for Dell EMC Isilon:
* Upgrade CSI Driver for Dell EMC Isilon v1.1.0 to v1.2.0 using Helm 2
* Migrate from Helm 2 to Helm 3

### Upgrade CSI Driver for Dell EMC Isilon v1.1.0 to v1.2.0 using Helm 2
1. Get the latest code from github.com/dell/csi-isilon (v1.2.0)
2. Uninstall the existing v1.1.0 driver using uninstall.isilon under csi-isilon/helm
3. Prepare myvalues.yaml
4. Run the ./install.isilon command to upgrade the driver
5. List the pods with the following command (to verify the status)

   `kubectl get pods -n isilon`

### Migrating from Helm 2 to Helm 3 (applicable for CSI Driver for Dell EMC Isilon v1.2.0 or higher)
1. Get the latest code from github.com/dell/csi-isilon by executing the following command.

    `git clone -b <VERSION> https://github.com/dell/csi-isilon.git`
    where <VERSION> is the version of CSI Driver for Dell EMC Isilon
2. Uninstall the CSI Driver for Dell EMC Isilon using the uninstall.isilon script under csi-isilon/helm using Helm 2.
3. Go to https://helm.sh/docs/topics/v2_v3_migration/ and follow the instructions to migrate from Helm 2 to Helm 3.
4. Once Helm 3 is ready, install the CSI Driver for Dell EMC Isilon using install.isilon script under csi-isilon/helm.
5. List the pods with the following command (to verify the status)

   `kubectl get pods -n isilon`

## Test deploying a simple pod with Isilon storage
Test the deployment workflow of a simple pod on Isilon storage.

1. **Creating a volume:**

    Create a file `pvc.yaml` using sample yaml files located at test/sample_files/


    Execute the following command to create volume
    ```
    kubectl create -f $PWD/pvc.yaml
    ```

    Result: After executing the above command PVC will be created in the default namespace, and the user can see the pvc by executing `kubectl get pvc`. 
    Note: Verify  system for the new volume

3. **Attach the volume to Host**

    To attach a volume to a host, create a new application(Pod) and use the PVC created above in the Pod. This scenario is explained using the Nginx application. Create `nginx.yaml` using sample yaml files located at test/sample_files/.

    Execute the following command to mount the volume to kubernetes node
    ```
    kubectl create -f $PWD/nginx.yaml
    ```

    Result: After executing the above command, new nginx pod will be successfully created and started in the default namespace.
    Note: Verify Isilon system for host to be part of clients/rootclients field of export created for volume and used by nginx application.

4. **Create Snapshot**

    The following procedure will create a snapshot of the volume in the container using VolumeSnapshot objects defined in snap.yaml. The sample file for snapshot creation is located at test/sample_files/
    
    Execute the following command to create snapshot
    ```
    kubectl create -f $PWD/snap.yaml
    ```
    
    The spec.source section contains the volume that will be snapped in the default namespace. For example, if the volume to be snapped is testvolclaim1, then the created snapshot is named testvolclaim1-snap1. Verify the Isilon system for new created snapshot.
    
    Note:
    
    * User can see the snapshots using `kubectl get volumesnapshot`
    * Notice that this VolumeSnapshot class has a reference to a snapshotClassName:isilon-snapclass. The CSI Driver for Isilon installation creates this class as its default snapshot class. 
    * You can see its definition using `kubectl get volumesnapshotclasses isilon-snapclass -o yaml`.

5. **Create Volume from Snapshot**

    The following procedure will create a new volume from a given snapshot which is specified in spec dataSource field.
    
    The sample file for volume creation from snapshot is located at test/sample_files/
    
    Execute the following command to create snapshot
    ```
    kubectl create -f $PWD/volume_from_snap.yaml
    ```

    Verify the Isilon system for new created volume from snapshot.

6. **Delete Snapshot**

    Execute the following command to delete the snapshot
    
    ```
    kubectl get volumesnapshot
    kubectl delete volumesnapshot testvolclaim1-snap1
    ```
7.  **To Unattach the volume from Host**

    Delete the nginx application to unattach the volume from host
    
    `kubectl delete -f nginx.yaml`
8. **To delete the volume**

    ```
    kubectl get pvc
    kubectl delete pvc testvolclaim1
    kubectl get pvc
    ```

## Install CSI-Isilon driver using dell-csi-operator in OpenShift
CSI Driver for Dell EMC Isilon can also be installed via the new Dell EMC Storage Operator.

The Dell EMC Storage CSI Operator is a Kubernetes Operator, which can be used to install and manage the CSI Drivers provided by Dell EMC for various storage platforms. This operator is available as a community operator for upstream Kubernetes and can be deployed using OperatorHub.io. It is also available as a community operator for OpenShift clusters and can be deployed using OpenShift Container Platform. Both these methods of installation use OLM (Operator Lifecycle Manager).
 
The operator can also be deployed directly by following the instructions available here - https://github.com/dell/dell-csi-operator .
 
There are sample manifests provided which can be edited to do an easy installation of the driver. Please note that the deployment of the driver using the operator doesn’t use any Helm charts and the installation & configuration parameters will be slightly different from the ones specified via the Helm installer.

Kubernetes Operators make it easy to deploy and manage entire lifecycle of complex Kubernetes applications. Operators use Custom Resource Definitions (CRD) which represents the application and use custom controllers to manage them.

### Listing CSI-Isilon drivers
User can query for csi-isilon driver using the following command
`kubectl get csiisilon --all-namespaces`

### Procedure to create new CSI-Isilon driver

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
 
3. Create a CR (Custom Resource) for isilon using the sample files provided 
at test/sample_files/CR/ .

4.  Execute the following command to create isilon custom resource
    ```kubectl create -f <input_sample_file.yaml>``` .
    The above command will deploy the csi-isilon driver.
 
5. User can configure the following parameters in CR
       
   The following table lists the primary configurable parameters of the Isilon driver and their default values.
   
   | Parameter | Description | Required | Default |
   | --------- | ----------- | -------- |-------- |
   | ***Common parameters for node and controller*** |
   | CSI_ENDPOINT | Specifies the HTTP endpoint for Isilon. | No | /var/run/csi/csi.sock |
   | X_CSI_DEBUG | To enable debug mode | No | false |
   | X_CSI_ISI_ENDPOINT | HTTPs endpoint of the Isilon OneFS API server | Yes | |
   | X_CSI_ISI_INSECURE | Specifies whether SSL security needs to be enabled for communication between Isilon and CSI Driver | No | true |
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
           
## Support
The CSI Driver for Dell EMC Isilon image available on Dockerhub is officially supported by Dell EMC.
 
The source code available on Github is unsupported and provided solely under the terms of the license attached to the source code. For clarity, Dell EMC does not provide support for any source code modifications.
 
For any CSI driver setup, configuration issues, questions or feedback, join the Dell EMC Container community at https://www.dell.com/community/Containers/bd-p/Containers
 
For any Dell EMC storage issues, please contact Dell support at: https://www.dell.com/support.
