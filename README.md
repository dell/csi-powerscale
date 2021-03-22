# CSI Driver for Dell EMC PowerScale

[![Go Report Card](https://goreportcard.com/badge/github.com/dell/csi-isilon?style=flat-square)](https://goreportcard.com/report/github.com/dell/csi-isilon)
[![License](https://img.shields.io/github/license/dell/csi-isilon?style=flat-square&color=blue&label=License)](https://github.com/dell/csi-isilon/blob/master/LICENSE)
[![Docker](https://img.shields.io/docker/pulls/dellemc/csi-isilon.svg?logo=docker&style=flat-square&label=Pulls)](https://hub.docker.com/r/dellemc/csi-isilon)
[![Last Release](https://img.shields.io/github/v/release/dell/csi-isilon?label=Latest&style=flat-square&logo=go)](https://github.com/dell/csi-isilon/releases)

**Repository for CSI Driver for Dell EMC PowerScale**

## Description
CSI Driver for Dell EMC PowerScale is a Container Storage Interface ([CSI](https://github.com/container-storage-interface/spec)) driver that provides support for provisioning persistent storage using Dell EMC PowerScale storage array. 

It supports CSI specification version 1.2.

This project may be compiled as a stand-alone binary using Golang that, when run, provides a valid CSI endpoint. It also can be used as a precompiled container image.

## Support
The CSI Driver for Dell EMC PowerScale image, which is the built driver code, is available on Dockerhub and is officially supported by Dell EMC.

The source code for CSI Driver for Dell EMC PowerScale available on Github is unsupported and provided solely under the terms of the license attached to the source code.

For clarity, Dell EMC does not provide support for any source code modifications.

For any CSI driver issues, questions or feedback, join the [Dell EMC Container community](<https://www.dell.com/community/Containers/bd-p/Containers/>)

## Building
This project is a Go module (see golang.org Module information for explanation).
The dependencies for this project are in the go.mod file.

To build the source, execute `make clean build`.

To run unit tests, execute `make unit-test`.

To build a podman based image, execute `make podman-build`.

You can run an integration test on a Linux system by populating the env files at `test/integration/` with values for your Dell EMC PowerScale systems and then run "`make integration-test`".

## Runtime Dependencies
Both the Controller and the Node portions of the driver can only be run on nodes which have network connectivity to a “`PowerScale Cluster`” (which is used by the driver).

## Driver Installation
Please consult the [Installation Guide](https://dell.github.io/storage-plugin-docs/docs/installation/)

## Using Driver
A number of test helm charts and scripts are found in the directory test/helm. Please refer to the section `Testing Drivers` in the [Documentation](https://dell.github.io/storage-plugin-docs/docs/installation/test/) for more info.

## Documentation
For more detailed information on the driver, please refer to [Dell Storage Documentation](https://dell.github.io/storage-plugin-docs/docs/) 

For a detailed set of information on supported platforms and driver capabilities, please refer to the [Features and Capabilities Documentation](https://dell.github.io/storage-plugin-docs/docs/dell-csi-driver/) 
