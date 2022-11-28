#!/bin/sh
# Copyright © 2019-2022 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License

# This is used for mocking different node IPs in order to test accessModes

# This should be like 111.222.333.444
export X_CSI_CLUSTER_NAME="cluster1"
export X_CSI_ISI_PATH="/ifs/data/csi/integration"
export X_CSI_ISI_PORT="8080"
export X_CSI_ISI_QUOTA_ENABLED="false"
export X_CSI_NODE_IP="1.1.1.2"
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
