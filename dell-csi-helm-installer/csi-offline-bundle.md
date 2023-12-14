# Offline Installation of Dell EMC CSI Storage Providers

## Description

The `csi-offline-bundle.sh` script can be used to create a package usable for offline installation of the Dell EMC CSI Storage Providers, via Helm. 

This includes the following drivers:
* [PowerFlex](https://github.com/dell/csi-vxflexos)
* [PowerMax](https://github.com/dell/csi-powermax)
* [PowerScale](https://github.com/dell/csi-powerscale)
* [PowerStore](https://github.com/dell/csi-powerstore)
* [Unity](https://github.com/dell/csi-unity)

the `csm-offline-bundle.sh` script can be used to create a package usable for offline installation of the Dell EMC CSI Storage Providers, via the CSM Operator.
* [Dell CSM Operator](https://github.com/dell/csm-operator)

## Dependencies

Multiple linux based systems may be required to create and process an offline bundle for use.
* One linux based system, with internet access, will be used to create the bundle. This involved the user cloning a git repository hosted on github.com and then invoking a script that utilizes `docker` or `podman` to pull and save container images to file.
* One linux based system, with access to an image registry, to invoke a script that uses `docker` or `podman` to restore container images from file and push them to a registry

If one linux system has both internet access and access to an internal registry, that system can be used for both steps.

Preparing an offline bundle requires the following utilities:

| Dependency            | Usage |
| --------------------- | ----- |
| `docker` or `podman`  | `docker` or `podman` will be used to pull images from public image registries, tag them, and push them to a private registry.  |
|                       | One of these will be required on both the system building the offline bundle as well as the system preparing for installation. |
|                       | Tested version(s) are `docker` 19.03+ and `podman` 1.6.4+
| `git`                 | `git` will be used to manually clone one of the above repos in order to create and offline bundle.
|                       | This is only needed on the system preparing the offline bundle.
|                       | Tested version(s) are `git` 1.8+ but any version should work.

## Workflow

To perform an offline installation of a driver or the Operator, the following steps should be performed:
1. Build an offline bundle
2. Unpacking an offline bundle and preparing for installation
3. Perform either a Helm installation or Operator installation

### Building an offline bundle

This needs to be performed on a linux system with access to the internet as a git repo will need to be cloned, and container images pulled from public registries.

The build an offline bundle, the following steps are needed:
1. Perform a `git clone` of the desired repository. For a helm based install, the specific driver repo should be cloned. For an Operator based deployment, the Dell CSM Operator repo should be cloned
2. Run the offline bundle script with an argument of `-c` in order to create an offline bundle
  - For Helm installs, the `csi-offline-bundle.sh` script will be found in the `dell-csi-helm-installer` directory
  - For Operator installs, the `csm-offline-bundle.sh` script will be found in the `scripts` directory

The script will perform the following steps:
  - Determine required images by parsing either the driver Helm charts (if run from a cloned CSI Driver git repository) or the Dell CSM Operator configuration files (if run from a clone of the Dell CSM Operator repository)
  - Perform an image `pull` of each image required
  - Save all required images to a file by running `docker save` or `podman save`
  - Build a `tar.gz` file containing the images as well as files required to installer the driver and/or Operator

The resulting offline bundle file can be copied to another machine, if necessary, to gain access to the desired image registry.

For example, here is the output of a request to build an offline bundle for the Dell CSM Operator:
```
[user@anothersystem /home/user]# git clone https://github.com/dell/csm-operator.git

```
```
[user@anothersystem /home/user]# cd csm-operator
```
```
[user@system /home/user/csm-operator]# bash scripts/csm-offline-bundle.sh -c

*
* Building image manifest file

   Processing file /root/csm-operator/operatorconfig/driverconfig/common/default.yaml
   Processing file /root/csm-operator/bundle/manifests/dell-csm-operator.clusterserviceversion.yaml

*
* Pulling and saving container images

   dellemc/csi-isilon:v2.8.0
   dellemc/csi-metadata-retriever:v1.6.0
   dellemc/csipowermax-reverseproxy:v2.6.0
   dellemc/csi-powermax:v2.9.0
   dellemc/csi-powerstore:v2.9.0
   dellemc/csi-unity:v2.8.0
   dellemc/csi-vxflexos:v2.9.0
   dellemc/csm-authorization-sidecar:v1.9.0
   dellemc/csm-metrics-powerflex:v1.5.0
   dellemc/csm-metrics-powerscale:v1.2.0
   dellemc/csm-topology:v1.5.0
   dellemc/dell-csi-replicator:v1.7.0
   dellemc/dell-replication-controller:v1.7.0
   dellemc/sdc:4.5
   docker.io/dellemc/dell-csm-operator:v1.3.0
   gcr.io/kubebuilder/kube-rbac-proxy:v0.8.0
   nginxinc/nginx-unprivileged:1.20
   otel/opentelemetry-collector:0.42.0
   registry.k8s.io/sig-storage/csi-attacher:v4.3.0
   registry.k8s.io/sig-storage/csi-external-health-monitor-controller:v0.9.0
   registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.8.0
   registry.k8s.io/sig-storage/csi-provisioner:v3.5.0
   registry.k8s.io/sig-storage/csi-resizer:v1.8.0
   registry.k8s.io/sig-storage/csi-snapshotter:v6.2.2

*
* Copying necessary files

 /root/csm-operator/deploy
 /root/csm-operator/operatorconfig
 /root/csm-operator/samples
 /root/csm-operator/scripts
 /root/csm-operator/README.md
 /root/csm-operator/LICENSE

*
* Compressing release

dell-csm-operator-bundle/
dell-csm-operator-bundle/deploy/
dell-csm-operator-bundle/deploy/operator.yaml
dell-csm-operator-bundle/deploy/crds/
dell-csm-operator-bundle/deploy/crds/storage.dell.com_containerstoragemodules.yaml
dell-csm-operator-bundle/deploy/olm/
dell-csm-operator-bundle/deploy/olm/operator_community.yaml
...
...
dell-csm-operator-bundle/README.md
dell-csm-operator-bundle/LICENSE

*
* Complete

Offline bundle file is: /root/csm-operator/dell-csm-operator-bundle.tar.gz

```

### Unpacking an offline bundle and preparing for installation

This needs to be performed on a linux system with access to an image registry that will host container images. If the registry requires `login`, that should be done before proceeding.

To prepare for driver or Operator installation, the following steps need to be performed:
1. Copy the offline bundle file to a system with access to an image registry available to your Kubernetes/OpenShift cluster
2. Expand the bundle file by running `tar xvfz <filename>`
3. Run the `csm-offline-bundle.sh` script and supply the `-p` option as well as the path to the internal registry with the `-r` option

The script will then perform the following steps:
  - Load the required container images into the local system
  - Tag the images according to the user supplied registry information
  - Push the newly tagged images to the registry
  - Modify the Helm charts or Operator configuration to refer to the newly tagged/pushed images


An example of preparing the bundle for installation:
```
[user@anothersystem /tmp]# tar xvfz dell-csm-operator-bundle.tar.gz
dell-csm-operator-bundle/
dell-csm-operator-bundle/deploy/
dell-csm-operator-bundle/deploy/operator.yaml
dell-csm-operator-bundle/deploy/crds/
dell-csm-operator-bundle/deploy/crds/storage.dell.com_containerstoragemodules.yaml
dell-csm-operator-bundle/deploy/olm/
dell-csm-operator-bundle/deploy/olm/operator_community.yaml
...
...
dell-csm-operator-bundle/README.md
dell-csm-operator-bundle/LICENSE
```
```
[user@anothersystem /tmp]# cd dell-csm-operator-bundle
```
```
[user@anothersystem /tmp/dell-csm-operator-bundle]# bash scripts/csm-offline-bundle.sh -p -r localregistry:5000/dell-csm-operator/
Preparing a offline bundle for installation

*
* Loading docker images

Loaded image: docker.io/dellemc/csi-powerstore:v2.9.0
Loaded image: docker.io/dellemc/csi-isilon:v2.9.0
...
...
Loaded image: registry.k8s.io/sig-storage/csi-resizer:v1.9.2
Loaded image: registry.k8s.io/sig-storage/csi-snapshotter:v6.3.2

*
* Tagging and pushing images

   dellemc/csi-isilon:v2.9.0 -> localregistry:5000/dell-csm-operator/csi-isilon:v2.9.0
   dellemc/csi-metadata-retriever:v1.6.0 -> localregistry:5000/dell-csm-operator/csi-metadata-retriever:v1.6.0
   ...
   ...
   registry.k8s.io/sig-storage/csi-resizer:v1.9.2 -> localregistry:5000/dell-csm-operator/csi-resizer:v1.9.2
   registry.k8s.io/sig-storage/csi-snapshotter:v6.3.2 -> localregistry:5000/dell-csm-operator/csi-snapshotter:v6.3.2

*
* Preparing files within /root/dell-csm-operator-bundle

   changing: dellemc/csi-isilon:v2.9.0 -> localregistry:5000/dell-csm-operator/csi-isilon:v2.9.0
   changing: dellemc/csi-metadata-retriever:v1.6.0 -> localregistry:5000/dell-csm-operator/csi-metadata-retriever:v1.6.0
   ...
   ...
   changing: registry.k8s.io/sig-storage/csi-resizer:v1.9.2 -> localregistry:5000/dell-csm-operator/csi-resizer:v1.9.2
   changing: registry.k8s.io/sig-storage/csi-snapshotter:v6.3.2 -> localregistry:5000/dell-csm-operator/csi-snapshotter:v6.3.2

*
* Complete

```

### Perform either a Helm installation or Operator installation

Now that the required images have been made available and the Helm Charts/Operator configuration updated, installation can proceed by following the instructions that are documented within the driver or Operator repo.
