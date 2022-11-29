#!/bin/bash
# Copyright © 2019 Dell Inc. or its subsidiaries. All Rights Reserved.
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

# scaletest
# This script will kick off a test designed to stress the limits of the driver
# It will install a user supplied helm chart 3 times, each with a user supplied number of replicas.
# Each replica will contain a number of volumes
# The test will continue to run until all replicas have been started, volumes created, and mapped

NOW=`date +"%Y%m%d-%H%M%S"`
SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

SCALEREPLICAS=""
NAMESPACE=""
STORAGECLASS1=""
STORAGECLASS2=""
STORAGECLASS3=""
SCALE_NUMBER_VOLUMES=1

# Usage information
function usage {
   echo
   echo "`basename ${0}`"
   echo "    -n namespace      - Namespace in which to place the test."
   echo "    -r replicas       - Number of replicas to create"
   echo "    -v number_volumes - Number of volumes for each pod. Default is: ${SCALE_NUMBER_VOLUMES}"
   echo
   echo "Default values for everything are set in the driver specific file at env.sh"
   exit 1
}

validateNameSpace() {
  # validate that the service account exists
  kubectl describe namespace "${1}" >/dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo "Creating Namespace"
    kubectl create namespace "${1}"
  fi
}

validateStorageClass() {
  # validate that thestorage exists
  kubectl describe storageclass -n "${NAMESPACE}" "${1}" >/dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo "StorageClass ${1} does not exist in the ${NAMESPACE} namespace."
    echo "Create it before running this test"
    exit 1
  fi
}

printVmsize() {
    for N in "${NODES_CONT[@]}"; do
        echo -n "$N " | tee -a ${LOGFILE}
        kubectl exec ${N} -n ${DRIVERNAMESPACE} --container driver -- ps -eo cmd,vsz,rss | grep csi-${DRIVER} | tee -a ${LOGFILE}
        kubectl logs ${N} -n ${DRIVERNAMESPACE} driver | grep statistics | tee -a ${LOGFILE}
    done
    for N in "${NODES_WORK[@]}"; do
        echo -n "$N "  | tee -a ${LOGFILE}
        kubectl exec ${N} -n ${DRIVERNAMESPACE} --container driver -- ps -eo cmd,vsz,rss | grep csi-${DRIVER} | tee -a ${LOGFILE}
        kubectl logs ${N} -n ${DRIVERNAMESPACE} driver | grep statistics | tee -a ${LOGFILE}
    done
}

deletePvcs() {
	FORCE=""
	PVCS=$(kubectl get pvc -n "${NAMESPACE}" | awk '/pvol/ { print $1; }')
	echo deleting... $PVCS
	for P in $PVCS; do
		if [ "$FORCE" == "yes" ]; then
			echo kubectl delete --force --grace-period=0 pvc $P -n "${NAMESPACE}"
			kubectl delete --force --grace-period=0 pvc $P -n "${NAMESPACE}"
		else
			echo kubectl delete pvc $P -n "${NAMESPACE}" | tee -a ${LOGFILE}
			kubectl delete pvc $P -n "${NAMESPACE}"
		fi
	done
}

cd "${SCRIPTDIR}"

# Parse the options passed on the command line
while getopts "n:r:v:" opt; do
  case $opt in
    n)
      NAMESPACE="${OPTARG}"
      ;;
    r)
      SCALEREPLICAS="${OPTARG}"
      ;;
    v)
      SCALE_NUMBER_VOLUMES="${OPTARG}"
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

source "./env.sh"

export NAMESPACE="${NAMESPACE}"
export STORAGECLASS1="${STORAGECLASS1}"
export STORAGECLASS2="${STORAGECLASS2}"
export STORAGECLASS3="${STORAGECLASS3}"

export LOGFILE="scalability-${NOW}.log"
export CSVFILE="scalability-${NOW}.csv"
# print a header in the CSV file
echo "pods, creating, pvcs" > ${CSVFILE}

validateNameSpace "${NAMESPACE}"
validateStorageClass "${STORAGECLASS1}"
validateStorageClass "${STORAGECLASS2}"
validateStorageClass "${STORAGECLASS3}"

# fill an array of controller and worker node names
declare -a NODES_CONT
declare -a NODES_WORK

while read -r line; do
  P=$(echo $line | awk '{ print $1 }')
  NODES_CONT+=("$P")
done < <(kubectl get pods -n ${DRIVERNAMESPACE} | grep controller)
while read -r line; do
  P=$(echo $line | awk '{ print $1 }')
  NODES_WORK+=("$P")
done < <(kubectl get pods -n ${DRIVERNAMESPACE} | grep node)

TARGET=$(expr $SCALEREPLICAS \* 3)
echo "Targeting replicas: $SCALEREPLICAS"
echo "Targeting pods: $TARGET"

helm install --set "name=isilon1,replicas=$SCALEREPLICAS,numberOfVolumes=$SCALE_NUMBER_VOLUMES,storageClass=$STORAGECLASS1,namespace=${NAMESPACE}"  -n isilon1 --namespace ${NAMESPACE} ${SCALETEST}
helm install --set "name=isilon2,replicas=$SCALEREPLICAS,numberOfVolumes=$SCALE_NUMBER_VOLUMES,storageClass=$STORAGECLASS2,namespace=${NAMESPACE}"  -n isilon2 --namespace ${NAMESPACE} ${SCALETEST}
helm install --set "name=isilon3,replicas=$SCALEREPLICAS,numberOfVolumes=$SCALE_NUMBER_VOLUMES,storageClass=$STORAGECLASS3,namespace=${NAMESPACE}"  -n isilon3 --namespace ${NAMESPACE} ${SCALETEST}

waitOnRunning() {
  if [ "$1" = "" ];
    then echo "arg: target" ;
    exit 2;
  fi

  WAITINGFOR=$1

  RUNNING=$(kubectl get pods -n ${NAMESPACE} | grep "Running" | wc -l)
  while [ $RUNNING -ne $WAITINGFOR ];
  do
	  RUNNING=$(kubectl get pods -n ${NAMESPACE} | grep "Running" | wc -l)
	  CREATING=$(kubectl get pods -n ${NAMESPACE} | grep "ContainerCreating" | wc -l)
	  PVCS=$(kubectl get pvc -n ${NAMESPACE} | grep -v "NAME" | wc -l)
	  date
	  date >>${LOGFILE}
	  echo running $RUNNING creating $CREATING pvcs $PVCS
	  echo running $RUNNING creating $CREATING pvcs $PVCS >>${LOGFILE}
    echo "$RUNNING, $CREATING, $PVCS" >> ${CSVFILE}
    printVmsize
	  sleep 30
  done
}

waitOnRunning $TARGET
sleep 30

echo
echo "*************************************************************************"
echo "Ramp up of volumes/pods complete, rescaling the deployments back to zero."
echo

# rescale the environment back to 0 replicas
helm upgrade --set "name=isilon1,replicas=0,numberOfVolumes=$SCALE_NUMBER_VOLUMES,storageClass=$STORAGECLASS1,namespace=${NAMESPACE}" isilon1 --namespace ${NAMESPACE} ${SCALETEST}
helm upgrade --set "name=isilon2,replicas=0,numberOfVolumes=$SCALE_NUMBER_VOLUMES,storageClass=$STORAGECLASS2,namespace=${NAMESPACE}" isilon2 --namespace ${NAMESPACE} ${SCALETEST}
helm upgrade --set "name=isilon3,replicas=0,numberOfVolumes=$SCALE_NUMBER_VOLUMES,storageClass=$STORAGECLASS3,namespace=${NAMESPACE}" isilon3 --namespace ${NAMESPACE} ${SCALETEST}

waitOnRunning 0
sleep 30

deletePvcs

# delete the helm deployments
helm delete --purge isilon1
helm delete --purge isilon2
helm delete --purge isilon3