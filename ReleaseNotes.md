# Release Notes - CSI PowerScale v1.5.0
[![Go Report Card](https://goreportcard.com/badge/github.com/dell/csi-isilon)](https://goreportcard.com/report/github.com/dell/csi-isilon)
[![License](https://img.shields.io/github/license/dell/csi-isilon)](https://github.com/dell/csi-isilon/blob/master/LICENSE)
[![Docker](https://img.shields.io/docker/pulls/dellemc/csi-isilon.svg?logo=docker)](https://hub.docker.com/r/dellemc/csi-isilon)

## New Features/Changes
- Added support for Kubernetes v1.20
- Added support for OpenShift 4.7 with RHEL and CoreOS worker nodes
- Added support for Red Hat Enterprise Linux (RHEL) 8.x
- Added multi-cluster support through single instance of driver installation
- Added support for custom networks for NFS I/O traffic
- SSH permissions are no longer required. You can safely revoke the privilege ISI_PRIV_LOGIN_SSH for the CSI driver user

## Fixed Issues
- There are no Fixed issues in this release.

## Known Issues
   | Issue | Resolution or workaround, if known |
   | ----- | ---------------------------------- |
   | Creating snapshot fails if the parameter IsiPath in volume snapshot class and related storage class are not the same. The driver uses the incorrect IsiPath parameter and tries to locate the source volume due to the inconsistency. | Ensure IsiPath in VolumeSnapshotClass yaml and related storageClass yaml are the same. |
   | While deleting a volume, if there are files or folders created on the volume that are owned by different users. If the Isilon credentials used are for a nonprivileged Isilon user, the delete volume action fails. It is due to the limitation in Linux permission control. | To perform the delete volume action, the user account must be assigned a role that has the privilege ISI_PRIV_IFS_RESTORE. The user account must have the following set of privileges to ensure that all the CSI Isilon driver capabilities work properly:<br> * ISI_PRIV_LOGIN_PAPI<br> * ISI_PRIV_NFS<br> * ISI_PRIV_QUOTA<br> * ISI_PRIV_SNAPSHOT<br> * ISI_PRIV_IFS_RESTORE<br> * ISI_PRIV_NS_IFS_ACCESS<br> In some cases, ISI_PRIV_BACKUP is also required, for example, when files owned by other users have mode bits set to 700. |
   | If hostname is mapped to loopback IP in /etc/hosts file, and pods are created using 1.3.0.1 release, after upgrade to 1.4.0 there is a possibility of "localhost" as stale entry in export | We recommend you not to map hostname to loopback IP in /etc/hosts file |
   | If the length of the nodeID exceeds 128 characters, driver fails to update CSINode object and installation fails. This is due to a limitation set by CSI spec which doesn't allow nodeID to be greater than 128 characters. | The CSI PowerScale driver uses the hostname for building the nodeID which is set in CSINode resource object, hence we recommend not having very long hostnames in order to avoid this issue. This current limitation of 128 characters is likely to be relaxed in future kubernetes versions as per this issue in the community: https://github.com/kubernetes-sigs/gcp-compute-persistent-disk-csi-driver/issues/581
