#!/bin/bash

# Copyright © 2020-2025 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

#!/bin/sh
# Copyright © 2019-2020 Dell Inc. or its subsidiaries. All Rights Reserved.
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

TEST=""
NAMESPACE="test"

# Usage information
function usage {
   echo
   echo "`basename ${0}`"
   echo "    -t test         - Test to stop"
   echo "    -n namespace    - Namespace in which the release is running. Default is: ${NAMESPACE}"
   exit 1
}

# Parse the options passed on the command line
while getopts "t:n:" opt; do
  case $opt in
    t)
      TEST="${OPTARG}"
      ;;
    n)
      NAMESPACE="${OPTARG}"
      ;;
    \?)
      echo "Invalid option: -$OPTARG" >&2
      usage
      ;;
    :)
      echo "Option -$OPTARG requires an argument." >&2
      usage
      ;;
  esac
done

# Ensure a test was named and that it exists
if [ "${TEST}" == "" ]; then
  echo "The name of a test must be specified"
  usage
fi
if [ ! -d "${TEST}" ]; then
  echo "Unable to find test named: ${TEST}"
  usage
fi

# the helm release name will be the basename of the test
RELEASE=`basename "${TEST}"`

VALUES="__${NAMESPACE}-${RELEASE}__.yaml"

helm version | grep "v3." --quiet
if [ $? -eq 0 ]; then
  helm delete "${RELEASE}"
else
  helm delete --purge "${RELEASE}"
fi

sleep 10
kubectl get pods -n "${NAMESPACE}"
echo "waiting for persistent volumes to be cleaned up"
sleep 90
sh deletepvcs.sh -n "${NAMESPACE}"
kubectl get persistentvolumes -o wide

if [ -f "${VALUES}" ]; then
  rm "${VALUES}"
fi



