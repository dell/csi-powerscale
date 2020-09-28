#!/bin/bash
#
# Copyright (c) 2020 Dell Inc., or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#  http://www.apache.org/licenses/LICENSE-2.0

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
DRIVERDIR="${SCRIPTDIR}/../helm"
VERIFYSCRIPT="${SCRIPTDIR}/verify.sh"
SNAPCLASSDIR="${SCRIPTDIR}/beta-snapshot-crd"
PROG="${0}"
NODE_VERIFY=1
VERIFY=1
MODE="install"
# version of Snapshot CRD to install. Default is none ("")
INSTALL_CRD=""

declare -a VALIDDRIVERS

source "$SCRIPTDIR"/common.sh


#
# usage will print command execution help and then exit
function usage() {
  echo
  echo "Help for $PROG"
  echo
  echo "Usage: $PROG options..."
  echo "Options:"
  echo "  Required"
  echo "  --namespace[=]<namespace>                Kubernetes namespace containing the CSI driver"
  echo "  --values[=]<values.yaml>                 Values file, which defines configuration values"

  echo "  Optional"
  echo "  --release[=]<helm release>               Name to register with helm, default value will match the driver name"
  echo "  --upgrade                                Perform an upgrade of the specified driver, default is false"
  echo "  --node-verify-user[=]<username>          Username to SSH to worker nodes as, used to validate node requirements. Default is root"
  echo "  --skip-verify                            Skip the kubernetes configuration verification to use the CSI driver, default will run verification"
  echo "  --skip-verify-node                       Skip worker node verification checks"
  echo "  --snapshot-crd                           Install snapshot CRDs. Default will not install Snapshot classes."
  echo "  -h                                       Help"
  echo

  exit 0
}

# warning, with an option for users to continue
function warning() {
  log separator
  printf "${YELLOW}WARNING:${NC}\n"
  for N in "$@"; do
    echo $N
  done
  echo
  if [ "${ASSUMEYES}" == "true" ]; then
    echo "Continuing as '-Y' argument was supplied"
    return
  fi
  read -n 1 -p "Press 'y' to continue or any other key to exit: " CONT
  echo
  if [ "${CONT}" != "Y" -a "${CONT}" != "y" ]; then
    echo "quitting at user request"
    exit 2
  fi
}


# print header information
function header() {
  log section "Installing CSI Driver: ${DRIVER} on ${kMajorVersion}.${kMinorVersion}"
}

#
# check_for_driver will see if the driver is already installed within the namespace provided
function check_for_driver() {
  log section "Checking to see if CSI Driver is already installed"
  NUM=$(helm list --namespace "${NS}" | grep "^${RELEASE}\b" | wc -l)
  if [ "${1}" == "install" -a "${NUM}" != "0" ]; then
    log error "The CSI Driver is already installed"
  fi
  if [ "${1}" == "upgrade" -a "${NUM}" == "0" ]; then
    log error "The CSI Driver is not installed"
  fi
}

#
# validate_params will validate the parameters passed in
function validate_params() {
  # make sure the driver was specified
  if [ -z "${DRIVER}" ]; then
    echo "No driver specified"
    usage
    exit 1
  fi
  # make sure the driver name is valid
  if [[ ! "${VALIDDRIVERS[@]}" =~ "${DRIVER}" ]]; then
    echo "Driver: ${DRIVER} is invalid."
    echo "Valid options are: ${VALIDDRIVERS[@]}"
    usage
    exit 1
  fi
  # the namespace is required
  if [ -z "${NS}" ]; then
    echo "No namespace specified"
    usage
    exit 1
  fi
  # values file
  if [ -z "${VALUES}" ]; then
    echo "No values file was specified"
    usage
    exit 1
  fi
  if [ ! -f "${VALUES}" ]; then
    echo "Unable to read values file at: ${VALUES}"
    usage
    exit 1
  fi
}

#
# install_driver uses helm to install the driver with a given name
function install_driver() {
  if [ "${1}" == "upgrade" ]; then
    log step "Upgrading Driver"
  else
    log step "Installing Driver"
  fi

  HELMOUTPUT="/tmp/csi-install.$$.out"
  helm ${1} --values "${DRIVERDIR}/${DRIVER}/k8s-${kMajorVersion}.${kMinorVersion}-values.yaml" --values "${DRIVERDIR}/${DRIVER}/driver-image.yaml" --values "${VALUES}" --namespace ${NS} "${RELEASE}" "${DRIVERDIR}/${DRIVER}" >"${HELMOUTPUT}" 2>&1
  if [ $? -ne 0 ]; then
    cat "${HELMOUTPUT}"
    log error "Helm operation failed, output can be found in ${HELMOUTPUT}. The failure should be examined, before proceeding. Additionally, running csi-uninstall.sh may be needed to clean up partial deployments."
  fi
  log step_success
  # wait for the deployment to finish, use the default timeout
  waitOnRunning "${NS}" "statefulset ${RELEASE}-controller,daemonset ${RELEASE}-node"
  if [ $? -eq 1 ]; then
    warning "Timed out waiting for the operation to complete." \
      "This does not indicate a fatal error, pods may take a while to start." \
      "Progress can be checked by running \"kubectl get pods -n ${NS}\""
  fi
}

# Print a nice summary at the end
function summary() {
  log section "Operation complete"
}

# waitOnRunning
# will wait, for a timeout period, for a number of pods to go into Running state within a namespace
# arguments:
#  $1: required: namespace to watch
#  $2: required: comma seperated list of deployment type and name pairs
#      for example: "statefulset mystatefulset,daemonset mydaemonset"
#  $3: optional: timeout value, 300 seconds is the default.
function waitOnRunning() {
  if [ -z "${2}" ]; then
    echo "No namespace and/or list of deployments was supplied. This field is required for waitOnRunning"
    return 1
  fi
  # namespace
  local NS="${1}"
  # pods
  IFS="," read -r -a PODS <<<"${2}"
  # timeout value passed in, or 300 seconds as a default
  local TIMEOUT="300"
  if [ -n "${3}" ]; then
    TIMEOUT="${3}"
  fi

  error=0
  for D in "${PODS[@]}"; do
    log arrow
    log smart_step "Waiting for $D to be ready" "small"
    kubectl -n "${NS}" rollout status --timeout=${TIMEOUT}s ${D} >/dev/null 2>&1
    if [ $? -ne 0 ]; then
      error=1
      log step_failure
    else
      log step_success
    fi
  done

  if [ $error -ne 0 ]; then
    return 1
  fi
  return 0
}

function kubectl_safe() {
  eval "kubectl $1"
  exitcode=$?
  if [[ $exitcode != 0 ]]; then
    echo "$2"
    exit $exitcode
  fi
}

#
# install_snapshot_crds
# Downloads and installs snapshot CRDs
function install_snapshot_crd() {
  if [ "${INSTALL_CRD}" == "" ]; then
    return
  fi
  log step "Checking and installing snapshot crds"

  declare -A SNAPCLASSES=(
    ["volumesnapshotclasses"]="snapshot.storage.k8s.io_volumesnapshotclasses.yaml"
    ["volumesnapshotcontents"]="snapshot.storage.k8s.io_volumesnapshotcontents.yaml"
    ["volumesnapshots"]="snapshot.storage.k8s.io_volumesnapshots.yaml"
  )

  for C in "${!SNAPCLASSES[@]}"; do
    F="${SNAPCLASSES[$C]}"
    # check if custom resource exists
    kubectl_safe "get customresourcedefinitions" "Failed to get crds" | grep "${C}" --quiet

    if [[ $? -ne 0 ]]; then
      # make sure CRD exists
      if [ ! -f "${SNAPCLASSDIR}/${SNAPCLASSES[$C]}" ]; then
        echo "Unable to to find Snapshot Classes at ${SNAPCLASSDIR}"
        exit 1
      fi
      # create the custom resource
      kubectl_safe "create -f ${SNAPCLASSDIR}/${SNAPCLASSES[$C]}" "Failed to create Volume Snapshot Beta CRD: ${C}"
    fi
  done

  sleep 10s
  log step_success
}

#
# verify_kubernetes
# will run a driver specific function to verify environmental requirements
function verify_kubernetes() {
  EXTRA_OPTS=""
  if [ $VERIFY -eq 0 ]; then
    echo "Skipping verification at user request"
  else
    if [ $NODE_VERIFY -eq 0 ]; then
      EXTRA_OPTS="$EXTRA_OPTS --skip-verify-node"
    fi
    if [ "${INSTALL_CRD}" == "yes" ]; then
      EXTRA_OPTS="$EXTRA_OPTS --snapshot-crd"
    fi
    "${VERIFYSCRIPT}" --namespace "${NS}" --release "${RELEASE}" --values "${VALUES}" --node-verify-user "${NODEUSER}" ${EXTRA_OPTS}
    VERIFYRC=$?
    case $VERIFYRC in
    0) ;;

    1)
      warning "Kubernetes validation failed but installation can continue. " \
        "This may affect driver installation."
      ;;
    *)
      log error "Kubernetes validation failed."
      ;;
    esac
  fi
}

#
# main
#
VERIFYOPTS=""
ASSUMEYES="false"

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
    skip-verify)
      VERIFY=0
      ;;
    skip-verify-node)
      NODE_VERIFY=0
      ;;
      # SNAPSHOT_CRD
    snapshot-crd)
      INSTALL_CRD="yes"
      ;;
    upgrade)
      MODE="upgrade"
      ;;
      # NAMESPACE
    namespace)
      NS="${!OPTIND}"
      if [[ -z ${NS} || ${NS} == "--skip-verify" ]]; then
        NS=${DEFAULT_NS}
      else
        OPTIND=$((OPTIND + 1))
      fi
      ;;
    namespace=*)
      NS=${OPTARG#*=}
      if [[ -z ${NS} ]]; then NS=${DEFAULT_NS}; fi
      ;;
      # RELEASE
    release)
      RELEASE="${!OPTIND}"
      OPTIND=$((OPTIND + 1))
      ;;
    release=*)
      RELEASE=${OPTARG#*=}
      ;;
      # VALUES
    values)
      VALUES="${!OPTIND}"
      OPTIND=$((OPTIND + 1))
      ;;
    values=*)
      VALUES=${OPTARG#*=}
      ;;
      # NODEUSER
    node-verify-user)
      NODEUSER="${!OPTIND}"
      OPTIND=$((OPTIND + 1))
      ;;
    node-verify-user=*)
      HODEUSER=${OPTARG#*=}
      ;;
    *)
      echo "Unknown option --${OPTARG}"
      echo "For help, run $PROG -h"
      exit 1
      ;;
    esac
    ;;
  h)
    usage
    ;;
  *)
    echo "Unknown option -${OPTARG}"
    echo "For help, run $PROG -h"
    exit 1
    ;;
  esac
done

# by default the NAME of the helm release of the driver is the same as the driver name
RELEASE=$(get_release_name "${DRIVER}")
# by default, NODEUSER is root
NODEUSER="${NODEUSER:-root}"

# make sure kubectl is available
kubectl --help >&/dev/null || {
  echo "kubectl required for installation... exiting"
  exit 2
}
# make sure helm is available
helm --help >&/dev/null || {
  echo "helm required for installation... exiting"
  exit 2
}

# Get the kubernetes major and minor version numbers.
kMajorVersion=$(kubectl version | grep 'Server Version' | sed -e 's/^.*Major:"//' -e 's/[^0-9].*//g')
kMinorVersion=$(kubectl version | grep 'Server Version' | sed -e 's/^.*Minor:"//' -e 's/[^0-9].*//g')

# validate the parameters passed in
validate_params "${MODE}"

header
check_for_driver "${MODE}"
verify_kubernetes

if [[ "${INSTALL_CRD}" != "" ]]; then
  install_snapshot_crd
fi


# all good, keep processing
install_driver "${MODE}"

summary
