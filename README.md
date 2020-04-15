# CSI Driver for Dell EMC Isilon
**Repository for CSI Driver for Dell EMC Isilon development project**

## Description
CSI Driver for Dell EMC Isilon is a Container Storage Interface ([CSI](https://github.com/container-storage-interface/spec)) driver that provides support for provisioning persistent storage using Dell EMC Isilon.

CSI driver for Dell EMC Isilon  
    * supports CSI specification version 1.1  
    * support for Kubernetes 1.14  
    * supports Red Hat Enterprise Linux 7.6 host operating system  
    * supports Isilon OneFS versions 8.1 and 8.2  
  
The project may be compiled as a stand-alone binary using Golang that, when run, provides a valid CSI endpoint. It may also be built as a Golang plug-in in order to extend the functionality of other programs.

## Building

This project is a Go module (see golang.org Module information for explanation).
The dependencies for this project are listed in the go.mod file.

To build the source, execute `make clean build`. Before building the source, three variables called REPO_NAME, IMAGE_NAME, and IMAGE_TAG should be defined. For example, run `export REPO_NAME=` in Linux to define the variable REPO_NAME. The image URL will be combined as $(REPO_NAME)/$(IMAGE_NAME):$(IMAGE_TAG).

To run unit tests, execute `make unit-test`.

To build a docker image, execute `make docker`.

You can also run integration tests on a Linux system by populating `env.sh` file with values for your Dell EMC Isilon system and then running `make integration-test`.

## Runtime Dependencies
Both the Controller and the Node portions of the driver can only be run on nodes with network connectivity to a Dell EMC Isilon server (which is used by the driver).

The Node portion of the driver can only be run on nodes that have the "mount" installed.

You can verify said runtime dependencies by running the `verify.kubernetes` script located inside of the helm directory.

## Installation

Installation in Kubernetes should be done using the `install.isilon` script and accompanying Helm chart in the helm directory.

For more detailed installation information, please refer to `CSI Driver for Dell EMC Isilon Product Guide and Release Notes v1.0.pdf`.

The driver will be started in Kubernetes as a result of executing the installation script.


## Using driver

A number of test helm charts and scripts are found in the directory test/helm.
Product Guide provides descriptions of how to run these and explains how they work.

## Capable operational modes
The CSI spec defines a set of AccessModes that a volume can have. The CSI Driver for Dell EMC Isilon supports the following modes for volumes that will be mounted as a filesystem:

```go
// Can only be published once as read/write on a single node,
// at any given time.
SINGLE_NODE_WRITER = 1;

// Can be published as readonly at multiple nodes simultaneously.
MULTI_NODE_READER_ONLY = 3;

// Can be published as read/write at multiple nodes
// simultaneously.
MULTI_NODE_MULTI_WRITER = 5;
```

This means that volumes can be mounted to either single node at a time, with read-write, or can be mounted on multiple nodes, with read-only or read-write permissions.

## Removing driver

Installed driver in Kubernetes can be uninstalled using the `uninstall.isilon` script and accompanying Helm chart in the helm directory.

## Support
The CSI Driver for Dell EMC Isilon image available on Dockerhub is officially supported by Dell EMC.

The source code available on Github is unsupported and provided solely under the terms of the license attached to the source code. For clarity, Dell EMC does not provide support for any source code modifications.

For any CSI driver setup, configuration issues, questions or feedback, join the Dell EMC Container community at https://www.dell.com/community/Containers/bd-p/Containers

For any Dell EMC storage issues, please contact Dell support at: https://www.dell.com/support.