#!/bin/sh

# This should be like 111.222.333.444
export X_CSI_CLUSTER_NAME="cluster1"
export X_CSI_ISI_PATH="/ifs/data/csi/integration"
export X_CSI_ISI_PORT="8080"
export X_CSI_ISI_QUOTA_ENABLED="true"
export X_CSI_NODE_IP=`hostname -I | head -1 | awk ' { print $1; } '`
NODEFQDN=`hostname -f`
NODENAME=`hostname`
SEPARATOR="=#=#="
NODEID=$NODENAME$SEPARATOR$NODEFQDN$SEPARATOR$X_CSI_NODE_IP
export X_CSI_NODE_NAME=$NODEID
export X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION="true"
export X_CSI_ISI_AUTOPROBE="true"
export X_CSI_ISI_NO_PROBE_ON_START="false"
export X_CSI_MODE=""
export X_CSI_ISI_CONFIG_PATH=`pwd`/config
export X_CSI_ISI_ACCESS_ZONE="integration-test-zone"
export X_CSI_ISI_AZ_SERVICE_IP="1.2.3.4"

# Variables for using tests
export CSI_ENDPOINT=`pwd`/unix_sock
