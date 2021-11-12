#!/bin/bash
#
# Copyright (c) 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#  http://www.apache.org/licenses/LICENSE-2.0

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
DRIVERDIR="${SCRIPTDIR}/../helm"
PROG="${0}"

# export the name of the debug log, so child processes will see it
export DEBUGLOG="${SCRIPTDIR}/uninstall-debug.log"

declare -a VALIDDRIVERS

source "$SCRIPTDIR"/common.sh

if [ -f "${DEBUGLOG}" ]; then
  rm -f "${DEBUGLOG}"
fi

#
# usage will print command execution help and then exit
function usage() {
    decho "Help for $PROG"
    decho
    decho "Usage: $PROG options..."
    decho "Options:"
    decho "  Required"
    decho "  --namespace[=]<namespace>  Kubernetes namespace to uninstall the CSI driver from"

    decho "  Optional"
    decho "  --release[=]<helm release> Name to register with helm, default value will match the driver name"
    decho "  -h                         Help"
    decho

    exit 0
}



#
# validate_params will validate the parameters passed in
function validate_params() {
    # make sure the driver was specified
    if [ -z "${DRIVER}" ]; then
        decho "No driver specified"
        exit 1
    fi
    # make sure the driver name is valid
    if [[ ! "${VALIDDRIVERS[@]}" =~ "${DRIVER}" ]]; then
        decho "Driver: ${DRIVER} is invalid."
        decho "Valid options are: ${VALIDDRIVERS[@]}"
        exit 1
    fi
    # the namespace is required
    if [ -z "${NAMESPACE}" ]; then
        decho "No namespace specified"
        usage
        exit 1
    fi
}


# check_for_driver will see if the driver is installed within the namespace provided
function check_for_driver() {
    NUM=$(run_command helm list --namespace "${NAMESPACE}" | grep "^${RELEASE}\b" | wc -l)
    if [ "${NUM}" == "0" ]; then
        log error "The CSI Driver is not installed."
        exit 1
    fi
}

# get the list of valid CSI Drivers, this will be the list of directories in drivers/ that contain helm charts
get_drivers "${DRIVERDIR}"

# if only one driver was found, set the DRIVER to that one
if [ ${#VALIDDRIVERS[@]} -eq 1 ]; then
  DRIVER="${VALIDDRIVERS[0]}"
fi

while getopts ":h-:" optchar; do
  case "${optchar}" in
  -)
    case "${OPTARG}" in
      # NAMESPACE
      namespace)
        NAMESPACE="${!OPTIND}"
        OPTIND=$((OPTIND + 1))
        ;;
      namespace=*)
        NAMESPACE=${OPTARG#*=}
        ;;
      # RELEASE
      release)
        RELEASE="${!OPTIND}"
        OPTIND=$((OPTIND + 1))
        ;;
      release=*)
        RELEASE=${OPTARG#*=}
        ;;
      *)
        decho "Unknown option --${OPTARG}"
        decho "For help, run $PROG -h"
        exit 1
        ;;
      esac
    ;;
    h)
      usage
    ;;
    *)
      decho "Unknown option -${OPTARG}"
      decho "For help, run $PROG -h"
      exit 1
      ;;
  esac
done

# by default the NAME of the helm release of the driver is the same as the driver name
RELEASE=$(get_release_name "${DRIVER}")

# validate the parameters passed in
validate_params

check_for_driver
run_command helm delete -n "${NAMESPACE}" "${RELEASE}"
if [ $? -ne 0 ]; then
    decho "Removal of the CSI Driver was unsuccessful"
    exit 1
fi

decho "Removal of the CSI Driver is in progress."
decho "It may take a few minutes for all pods to terminate."

