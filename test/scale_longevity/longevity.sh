#!/bin/bash


# longevity.sh
# This script will kick off a test designed to run forever, validiting the longevity of the driver.
#
# The test will continue to run until a file named 'stop' is placed in the script directory.

NOW=`date +"%Y%m%d-%H%M%S"`
SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

LONGREPLICAS=""
NAMESPACE=""
STORAGECLASS1=""
STORAGECLASS2=""
STORAGECLASS3=""
NUMBER_VOLUMES=1

# Usage information
function usage {
   echo
   echo "`basename ${0}`"
   echo "    -n namespace      - Namespace in which to place the test."
   echo "    -r replicas       - Number of replicas to create"
   echo "    -v number_volumes - Number of volumes for each pod. Default is: ${NUMBER_VOLUMES}"
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

cd "${SCRIPTDIR}"

# Parse the options passed on the command line
while getopts "n:r:v:" opt; do
  case $opt in
    n)
      NAMESPACE="${OPTARG}"
      ;;
    r)
      LONGREPLICAS="${OPTARG}"
      ;;
    v)
      NUMBER_VOLUMES="${OPTARG}"
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
export LOGFILE="longevity-${DRIVER}-${NOW}.log"
export CSVFILE="longevity-${DRIVER}-${NOW}.csv" 

# print a header in the CSV file 
echo "pods, creating, terminating, pvcs" > ${CSVFILE}

# remove stop file
rm -f stop

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

echo "Controllers: ${NODES_CONT}"
echo "Workers: ${NODES_WORK[@]}"

deployPods() {
	echo "Deploying pods, replicas: $LONGREPLICAS, target $TARGET, number of volumes: $NUMBER_VOLUMES"
	helm install --set "name=isilon1,replicas=$LONGREPLICAS,numberOfVolumes=$NUMBER_VOLUMES,storageClass=$STORAGECLASS1,namespace=${NAMESPACE}"  -n isilon1 --namespace ${NAMESPACE} ${LONGTEST}
	helm install --set "name=isilon2,replicas=$LONGREPLICAS,numberOfVolumes=$NUMBER_VOLUMES,storageClass=$STORAGECLASS2,namespace=${NAMESPACE}"  -n isilon2 --namespace ${NAMESPACE} ${LONGTEST}
	helm install --set "name=isilon3,replicas=$LONGREPLICAS,numberOfVolumes=$NUMBER_VOLUMES,storageClass=$STORAGECLASS3,namespace=${NAMESPACE}"  -n isilon3 --namespace ${NAMESPACE} ${LONGTEST}
}

rescalePods() {
	echo "Rescaling pods, replicas: $LONGREPLICAS, target $TARGET"
	helm upgrade --set "name=isilon1,replicas=$1,numberOfVolumes=$NUMBER_VOLUMES,storageClass=$STORAGECLASS1,namespace=${NAMESPACE}" isilon1 --namespace ${NAMESPACE} ${LONGTEST}
	helm upgrade --set "name=isilon2,replicas=$1,numberOfVolumes=$NUMBER_VOLUMES,storageClass=$STORAGECLASS2,namespace=${NAMESPACE}" isilon2 --namespace ${NAMESPACE} ${LONGTEST}
	helm upgrade --set "name=isilon3,replicas=$1,numberOfVolumes=$NUMBER_VOLUMES,storageClass=$STORAGECLASS3,namespace=${NAMESPACE}" isilon3 --namespace ${NAMESPACE} ${LONGTEST}
}

helmDelete() {
	echo "Deleting helm charts"
	helm delete --purge isilon1
	helm delete --purge isilon2
	helm delete --purge isilon3
}

printVmsize() {
    for N in "${NODES_CONT[@]}"; do
        echo -n "$N " | tee -a ${LOGFILE}
        kubectl exec ${N} -n ${DRIVERNAMESPACE} --container driver -- ps -eo cmd,vsz,rss | grep csi-${DRIVER} | tee -a ${LOGFILE}
    done
    for N in "${NODES_WORK[@]}"; do
        echo -n "$N " | tee -a ${LOGFILE}
        kubectl exec ${N} -n ${DRIVERNAMESPACE} --container driver -- ps -eo cmd,vsz,rss | grep csi-${DRIVER} | tee -a ${LOGFILE}
    done
}

waitOnRunning() {
  if [ "$1" = "" ]; 
    then echo "arg: target" ; 
    exit 2; 
  fi

  WAITINGFOR=$1
  
  RUNNING=$(kubectl get pods -n "${NAMESPACE}" | grep "Running" | wc -l)
  while [ $RUNNING -ne $WAITINGFOR ];
  do
	  RUNNING=$(kubectl get pods -n "${NAMESPACE}" | grep "Running" | wc -l)
	  CREATING=$(kubectl get pods -n "${NAMESPACE}" | grep "ContainerCreating" | wc -l)
	  TERMINATING=$(kubectl get pods -n "${NAMESPACE}" | grep "Terminating" | wc -l)
	  PVCS=$(kubectl get pvc -n "${NAMESPACE}" | grep -v "NAME" | wc -l)
	  date | tee -a ${LOGFILE}
	  echo running $RUNNING creating $CREATING terminating $TERMINATING pvcs $PVCS | tee -a ${LOGFILE}
      echo "$RUNNING, $CREATING, $TERMINATING, $PVCS" >> ${CSVFILE}
	  printVmsize
	  sleep 30
  done
}

waitOnNoPods() {
	COUNT=$(kubectl get pods -n "${NAMESPACE}" | grep -v "NAME" | wc -l)
	while [ $COUNT -gt 0 ];
	do
		echo "Waiting on all $COUNT pods to be deleted" | tee -a ${LOGFILE}
		sleep 30
		COUNT=$(kubectl get pods -n "${NAMESPACE}" | grep -v "NAME" | wc -l)
		echo pods $COUNT
	done
}

waitOnNoVolumeAttachments() {
	COUNT=$(kubectl get volumeattachments -n "${NAMESPACE}" | grep -v "NAME" | wc -l)
	while [ $COUNT -gt 0 ];
	do
		echo "Waiting on all volume attachments to be deleted: $COUNT" | tee -a ${LOGFILE}
		sleep 30
		COUNT=$(kubectl get volumeattachments -n "${NAMESPACE}" | grep -v "NAME" | wc -l)
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


validateNameSpace "${NAMESPACE}"
validateStorageClass "${STORAGECLASS1}"
validateStorageClass "${STORAGECLASS2}"
validateStorageClass "${STORAGECLASS3}"

TARGET=$(expr $LONGREPLICAS \* 3)
echo "Targeting replicas: $LONGREPLICAS"
echo "Targeting pods: $TARGET"

#
# Longevity test loop. Runs until a "stop" file is found.
#
ITER=1
while true;
do
	TS=$(date)
	echo "Longevity test iteration $ITER replicas $LONGREPLICAS target $TARGET $TS" | tee -a ${LOGFILE}

	echo "deploying pods" | tee -a ${LOGFILE}
	deployPods
	echo "waiting on running $target"  | tee -a ${LOGFILE}
	waitOnRunning $TARGET
	echo "rescaling pods 0"  | tee -a ${LOGFILE}
	rescalePods 0
	echo "waiting on running 0"  | tee -a ${LOGFILE}
	waitOnRunning 0
	echo "waiting on no pods"  | tee -a ${LOGFILE}
	waitOnNoPods

	waitOnNoVolumeAttachments
	deletePvcs
	helmDelete

	echo "Longevity test iteration $ITER completed $TS"  | tee -a ${LOGFILE}
	
	if [ -f stop ]; 
	then
		echo "stop detected... exiting"
		exit 0
	fi
	ITER=$(expr $ITER \+ 1)
done
exit 0