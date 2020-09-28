#!/bin/sh
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

