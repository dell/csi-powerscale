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

force=no
NAMESPACE="test"

# Usage information
function usage {
   echo
   echo "`basename ${0}`"
   echo "    -n namespace    - Namespace in which the release is running. Default is: ${NAMESPACE}"
   echo "    -f              - force delete"
   exit 1
}

# Parse the options passed on the command line
while getopts "fn:" opt; do
  case $opt in
    n)
      NAMESPACE="${OPTARG}"
      ;;
    f)
      force=yes
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

pvcs=$(kubectl get pvc -n $NAMESPACE | awk '/pvol/ { print $1; }')
echo deleting... $pvcs
for pvc in $pvcs
do
if [ $force == "yes" ];
then
	echo kubectl delete --force --grace-period=0 pvc $pvc -n $NAMESPACE
	kubectl delete --force --grace-period=0 pvc $pvc -n $NAMESPACE
else
	echo kubectl delete pvc $pvc -n $NAMESPACE
	kubectl delete pvc $pvc -n $NAMESPACE
fi
done

