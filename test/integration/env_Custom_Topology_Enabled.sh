#!/bin/sh

# This should be like 111.222.333.444
export X_CSI_CLUSTER_NAME="cluster1"
export X_CSI_ISI_PATH="/ifs/data/csi/integration"
export X_CSI_ISI_PORT="8080"
export X_CSI_ISI_QUOTA_ENABLED="true"
export X_CSI_CUSTOM_TOPOLOGY_ENABLED="true"
export X_CSI_NODE_IP=`hostname -I | head -1 | awk ' { print $1; } '`
NODENAME=`hostname`
export X_CSI_NODE_NAME=$NODENAME
export X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION="true"
export X_CSI_ISI_AUTOPROBE="true"
export X_CSI_ISI_NO_PROBE_ON_START="false"
export X_CSI_MODE=""
export X_CSI_ISI_CONFIG_PATH=`pwd`/config

# Variables for using tests
export CSI_ENDPOINT=`pwd`/unix_sock
