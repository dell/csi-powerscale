# :lock: **Important Notice**
Starting with the release of **Container Storage Modules v1.16.0**, this repository will no longer be maintained as an open source project. Future development will continue under a closed source model. This change reflects our commitment to delivering even greater value to our customers by enabling faster innovation and more deeply integrated features with the Dell storage portfolio.<br>
For existing customers using Dell’s Container Storage Modules, you will continue to receive:
* **Ongoing Support & Community Engagement**<br>
       You will continue to receive high-quality support through Dell Support and our community channels. Your experience of engaging with the Dell community remains unchanged.
* **Streamlined Deployment & Updates**<br>
        Deployment and update processes will remain consistent, ensuring a smooth and familiar experience.
* **Access to Documentation & Resources**<br>
       All documentation and related materials will remain publicly accessible, providing transparency and technical guidance.
* **Continued Access to Current Open Source Version**<br>
       The current open-source version will remain available under its existing license for those who rely on it.

Moving to a closed source model allows Dell’s development team to accelerate feature delivery and enhance integration across our Enterprise Kubernetes Storage solutions ultimately providing a more seamless and robust experience.<br>
We deeply appreciate the contributions of the open source community and remain committed to supporting our customers through this transition.<br>

For questions or access requests, please contact the maintainers via [Dell Support](https://www.dell.com/support/kbdoc/en-in/000188046/container-storage-interface-csi-drivers-and-container-storage-modules-csm-how-to-get-support).

# CSI Driver for Dell EMC PowerScale

[![Go Report Card](https://goreportcard.com/badge/github.com/dell/csi-isilon?style=flat-square)](https://goreportcard.com/report/github.com/dell/csi-isilon)
[![License](https://img.shields.io/github/license/dell/csi-isilon?style=flat-square&color=blue&label=License)](https://github.com/dell/csi-isilon/blob/main/LICENSE)
[![Docker](https://img.shields.io/docker/pulls/dellemc/csi-isilon.svg?logo=docker&style=flat-square&label=Pulls)](https://hub.docker.com/r/dellemc/csi-isilon)
[![Last Release](https://img.shields.io/github/v/release/dell/csi-isilon?label=Latest&style=flat-square&logo=go)](https://github.com/dell/csi-isilon/releases)

**Repository for CSI Driver for Dell EMC PowerScale**

## Description

CSI Driver for PowerScale is part of the [CSM (Container Storage Modules)](https://github.com/dell/csm) open-source suite of Kubernetes storage enablers for Dell Technology (Dell) products. CSI Driver for PowerScale is a Container Storage Interface (CSI) driver that provides support for provisioning persistent storage using Dell PowerScale storage array.

This project may be compiled as a stand-alone binary using Golang that, when run, provides a valid CSI endpoint. It also can be used as a precompiled container image.

## Table of Contents

* [Code of Conduct](https://github.com/dell/csm/blob/main/docs/CODE_OF_CONDUCT.md)
* [Maintainer Guide](https://github.com/dell/csm/blob/main/docs/MAINTAINER_GUIDE.md)
* [Committer Guide](https://github.com/dell/csm/blob/main/docs/COMMITTER_GUIDE.md)
* [Contributing Guide](https://github.com/dell/csm/blob/main/docs/CONTRIBUTING.md)
* [List of Adopters](https://github.com/dell/csm/blob/main/docs/ADOPTERS.md)
* [Support](#support)
* [Security](https://github.com/dell/csm/blob/main/docs/SECURITY.md)
* [Building](#building)
* [Runtime Dependecies](#runtime-dependencies)
* [Documentation](#documentation)

## Support
For any issues, questions or feedback, please contact [Dell support](https://www.dell.com/support/incidents-online/en-us/contactus/product/container-storage-modules).

## Building

This project is a Go module (see golang.org Module information for explanation).
The dependencies for this project are in the go.mod file.

To build the source, execute `make clean build`.

To run unit tests, execute `make unit-test`.

To build a podman based image, execute `make podman-build`.

You can run an integration test on a Linux system by populating the env files at `test/integration/` with values for your Dell EMC PowerScale systems and then run "`make integration-test`".

## Runtime Dependencies

Both the Controller and the Node portions of the driver can only be run on nodes which have network connectivity to a “`PowerScale Cluster`” (which is used by the driver).

## Documentation

For more detailed information on the driver, please refer to [Container Storage Modules documentation](https://dell.github.io/csm-docs/).
