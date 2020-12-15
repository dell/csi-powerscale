# Helm Installer for Dell EMC CSI Storage Providers

## Description

This directory provides scripts to install, upgrade, uninstall the CSI drivers, and to verify the Kubernetes environment.
These same scripts are present in all Dell EMC Container Storage Interface ([CSI](https://github.com/container-storage-interface/spec)) drivers. This includes the drivers for:
* [PowerFlex](https://github.com/dell/csi-vxflexos)
* [PowerMax](https://github.com/dell/csi-powermax)
* [PowerScale](https://github.com/dell/csi-powerscale)
* [PowerStore](https://github.com/dell/csi-powerstore)
* [Unity](https://github.com/dell/csi-unity)

NOTE: This documentation uses the PowerScale driver as an example. If working with a different driver, substitute the name as appropriate.

## Dependencies

Installing any of the Dell EMC CSI Drivers requires a few utilities to be installed on the system running the installation.

| Dependency    | Usage  |
| ------------- | ----- |
| `kubectl`     | Kubectl is used to validate that the Kubernetes system meets the requirements of the driver. |
| `helm`        | Helm v3 is used as the deployment tool for Charts. See, [Install HELM 3](https://helm.sh/docs/intro/install/) for instructions to install HELM 3. |
| `sshpass`     | sshpass is used to check certain pre-requisities in worker nodes (in chosen drivers). |


In order to use these tools, a valid `KUBECONFIG` is required. Ensure that either a valid configuration is in the default location or that the `KUBECONFIG` environment variable points to a valid confiugration before using these tools.

## Capabilities

This project provides the following capabilitites, each one is discussed in detail later in this document.

* Install a driver. When installing a driver, options are provided to specify the target namespace as well as options to control the types of verifications to be performed on the target system.
* Upgrade a driver. Upgrading a driver is an effective way to either deploy a new version of the driver or to modify the parameters used in an initial deployment.
* Uninstall a driver. This removes the driver and any installed storage classes.
* Verify a Kubernetes system for suitability with a driver. These verification steps differ, slightly, from driver to driver but include verifiying version compatibility, namespace availability, existance of required secrets, and validating worker node compatibility with driver protocols such as iSCSI, Fibre Channel, NFS, etc 


Most of these usages require the creation/specification of a values file. These files specify configuration settings that are passed into the driver and configure it for use. To create one of these files, the following steps should be followed:
1. Copy a template file for the driver to a new location, naming this new file is at the users discretion. The template files are always found within the driver repo at `helm/csi-<drivername>/values.yaml`
2. Edit the file such that it contains the proper configuration settings for the specific environment. These files are yaml formatted so maintaining the file structure is important.

For example, to create a values file for the PowerScale driver the following steps can be executed
```
# cd to  the installation script directory
cd dell-csi-helm-installer

# copy the template file
cp ../helm/csi-isilon/values.yaml ./my-isilon-settings.yaml

# edit the newly created values file
vi my-isilon-settings.yaml
```

These values files can then be archived for later reference or for usage when upgrading the driver.


### Install A Driver

Installing a driver is performed via the `csi-install.sh` script. This script requires a few arguments: the target namespace and the user created values file. By default, this will verify the Kubernetes environment and present a list of warnings and/or errors. Errors must be addressed before installing, warning should be examined for their applicability. For example, in order to install the PowerScale driver into a namespace called "isilon", the following command should be run:
```
./csi-install.sh --namespace isilon --values ./my-isilon-settings.yaml
```

For usage information:
```
[dell-csi-helm-installer]# ./csi-install.sh -h
Help for ./csi-install.sh

Usage: ./csi-install.sh options...
Options:
  Required
  --namespace[=]<namespace>                Kubernetes namespace containing the CSI driver
  --values[=]<values.yaml>                 Values file, which defines configuration values
  Optional
  --release[=]<helm release>               Name to register with helm, default value will match the driver name
  --upgrade                                Perform an upgrade of the specified driver, default is false
  --node-verify-user[=]<username>          Username to SSH to worker nodes as, used to validate node requirements. Default is root
  --skip-verify                            Skip the kubernetes configuration verification to use the CSI driver, default will run verification
  --skip-verify-node                       Skip worker node verification checks
  --snapshot-crd                           Install snapshot CRDs. Default will not install Snapshot classes.
  -h                                       Help
```

### Upgrade A Driver

Upgrading a driver is very similar to installation. The `csi-install.sh` script is run, with the same required arguments, along with a `--upgrade` argument. For example, to upgrade the previously installed PowerScale driver, the following command can be supplied:

```
./csi-install.sh --namespace isilon --values ./my-isilon-settings.yaml --upgrade
```

For usage information:
```
[dell-csi-helm-installer]# ./csi-install.sh -h
Help for ./csi-install.sh

Usage: ./csi-install.sh options...
Options:
  Required
  --namespace[=]<namespace>                Kubernetes namespace containing the CSI driver
  --values[=]<values.yaml>                 Values file, which defines configuration values
  Optional
  --release[=]<helm release>               Name to register with helm, default value will match the driver name
  --upgrade                                Perform an upgrade of the specified driver, default is false
  --node-verify-user[=]<username>          Username to SSH to worker nodes as, used to validate node requirements. Default is root
  --skip-verify                            Skip the kubernetes configuration verification to use the CSI driver, default will run verification
  --skip-verify-node                       Skip worker node verification checks
  --snapshot-crd                           Install snapshot CRDs. Default will not install Snapshot classes.
  -h                                       Help
```

### Uninstall A Driver

To uninstall a driver, the `csi-uninstall.sh` script provides a handy wrapper around the `helm` utility. The only required argument for uninstallation is the namespace name. To uninstall the PowerScale driver:

```
./csi-uninstall.sh --namespace isilon
```

For usage information:
```
[dell-csi-helm-installer]# ./csi-uninstall.sh -h
Help for ./csi-uninstall.sh

Usage: ./csi-uninstall.sh options...
Options:
  Required
  --namespace[=]<namespace>  Kubernetes namespace to uninstall the CSI driver from
  Optional
  --release[=]<helm release> Name to register with helm, default value will match the driver name
  -h                         Help
```

### Verify A Kubernetes Environment

The `verify.sh` script is run, automatically, as part of the installation and upgrade procedures and can also be run by itself. This provides a handy means to validate a Kubernetes system without meaning to actually perform the installation. To verify an environment, run `verify.sh` with the namespace name and values file options.

```
./verify.sh --namespace isilon --values ./my-isilon-settings.yaml
```

For usage information:
```
[dell-csi-helm-installer]# ./verify.sh -h
Help for ./verify.sh

Usage: ./verify.sh options...
Options:
  Required
  --namespace[=]<namespace>       Kubernetes namespace to install the CSI driver
  --values[=]<values.yaml>        Values file, which defines configuration values
  Optional
  --skip-verify-node              Skip worker node verification checks
  --release[=]<helm release>      Name to register with helm, default value will match the driver name
  --node-verify-user[=]<username> Username to SSH to worker nodes as, used to validate node requirements. Default is root
  -h                              Help                           Help
```

