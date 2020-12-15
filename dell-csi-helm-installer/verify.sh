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
PROG="${0}"
source "$SCRIPTDIR"/common.sh

if [ -z "${DEBUGLOG}" ]; then
  export DEBUGLOG="${SCRIPTDIR}/install-debug.log"
  if [ -f "${DEBUGLOG}" ]; then
    rm -f "${DEBUGLOG}"
  fi
fi

declare -a VALIDDRIVERS

# verify-csi-powermax method
function verify-csi-powermax() {
  verify_k8s_versions "1.17" "1.20"
  verify_openshift_versions "4.5" "4.6"
  verify_namespace "${NS}"
  verify_required_secrets "${RELEASE}-creds"
  verify_optional_secrets "${RELEASE}-certs"
  verify_optional_secrets "csirevproxy-tls-secret"
  verify_alpha_snap_resources
  verify_beta_snap_requirements
  verify_iscsi_installation
  verify_helm_3
}

#
# verify-csi-isilon method
function verify-csi-isilon() {
  verify_k8s_versions "1.17" "1.20"
  verify_openshift_versions "4.5" "4.6"
  verify_namespace "${NS}"
  verify_required_secrets "${RELEASE}-creds"
  verify_optional_secrets "${RELEASE}-certs"
  verify_alpha_snap_resources
  verify_beta_snap_requirements
  verify_helm_3
}

#
# verify-csi-vxflexos method
function verify-csi-vxflexos() {
  verify_k8s_versions "1.17" "1.20"
  verify_openshift_versions "4.5" "4.6"
  verify_namespace "${NS}"
  verify_required_secrets "${RELEASE}-creds"
  verify_sdc_installation
  verify_alpha_snap_resources
  verify_beta_snap_requirements
  verify_helm_3
}

# verify-csi-powerstore method
function verify-csi-powerstore() {
  verify_k8s_versions "1.17" "1.20"
  verify_openshift_versions "4.5" "4.6"
  verify_namespace "${NS}"
  verify_required_secrets "${RELEASE}-creds"
  verify_alpha_snap_resources
  verify_beta_snap_requirements
  verify_powerstore_node_configuration
  verify_helm_3
}

# verify-csi-unity method
function verify-csi-unity() {
  verify_k8s_versions "1.17" "1.20"
  verify_openshift_versions "4.5" "4.6"
  verify_namespace "${NS}"
  verify_required_secrets "${RELEASE}-creds"
  verify_required_secrets "${RELEASE}-certs-0"
  verify_alpha_snap_resources
  verify_unity_protocol_installation
  verify_beta_snap_requirements  
  verify_helm_3
}

# if testing routines are found, source them for possible execution
if [ -f "${SCRIPTDIR}/test-functions.sh" ]; then
  source "${SCRIPTDIR}/test-functions.sh"
fi

#
# verify-driver will call the proper method to verify a specific driver
function verify-driver() {
  if [ -z "${1}" ]; then
    decho "Expected one argument, the driver name, to verify-driver. Received none."
    exit $EXIT_ERROR
  fi
  local D="${1}"
  # check if a verify-$DRIVER function exists
  # if not, error and exit
  # if yes, check to see if it should be run and run it
  FNTYPE=$(type -t verify-$D)
  if [ "$FNTYPE" != "function" ]; then
    decho "ERROR: verify-$D function does not exist"
    exit $EXIT_ERROR
  else
    header
    log step "Driver: ${D}"
    decho
    verify-$D
    summary
  fi
}

# Print usage information
function usage() {
  decho
  decho "Help for $PROG"
  decho
  decho "Usage: $PROG options..."
  decho "Options:"
  decho "  Required"
  decho "  --namespace[=]<namespace>       Kubernetes namespace to install the CSI driver"
  decho "  --values[=]<values.yaml>        Values file, which defines configuration values"

  decho "  Optional"
  decho "  --skip-verify-node              Skip worker node verification checks"
  decho "  --release[=]<helm release>      Name to register with helm, default value will match the driver name"
  decho "  --node-verify-user[=]<username> Username to SSH to worker nodes as, used to validate node requirements. Default is root"
  decho "  --snapshot-crd                  Signifies that the Snapshot CRDs will be installed as part of installation."
  decho "  -h                              Help"
  decho

  exit $EXIT_WARNING
}

# print header information
function header() {
  log section "Verifying Kubernetes and driver configuration"
  echo "|- Kubernetes Version: ${kMajorVersion}.${kMinorVersion}"
}

# Check if the SDC is installed and the kernel module loaded
function verify_sdc_installation() {
  if [ ${NODE_VERIFY} -eq 0 ]; then
    return
  fi
  log step "Verifying the SDC installation"

  local SDC_MINION_NODES=$(run_command kubectl get nodes -o wide | grep -v -e master -e INTERNAL -e infra | awk ' { print $6; }')

  error=0
  missing=()
  for node in $SDC_MINION_NODES; do
    # check is the scini kernel module is loaded
    run_command ssh ${NODEUSER}@$node "/sbin/lsmod | grep scini" >/dev/null 2>&1
    rv=$?
    if [ $rv -ne 0 ]; then
      missing+=($node)
      error=1
      found_warning "SDC was not found on node: $node"
    fi
  done
  check_error error
}

function verify_powerstore_node_configuration() {
  if [ ${NODE_VERIFY} -eq 0 ]; then
    return
  fi

  log step "Verifying PowerStore node configuration"
  decho

  if ls "${VALUES}" >/dev/null; then
    if grep -c "scsiProtocol:[[:blank:]]\+FC" "${VALUES}" >/dev/null; then
      log arrow
      verify_fc_installation
    elif grep -c "scsiProtocol:[[:blank:]]\+ISCSI" "${VALUES}" >/dev/null; then
      log arrow
      verify_iscsi_installation "small"
    elif grep -c "scsiProtocol:[[:blank:]]\+auto" "${VALUES}" >/dev/null; then
      log arrow
      verify_iscsi_installation "small"
      log arrow
      verify_fc_installation "small"
    elif grep -c "scsiProtocol:[[:blank:]]\+None" "${VALUES}" >/dev/null; then
      log step_warning
      found_warning "Neither FC nor iSCSI connection is activated, please be sure that NFS settings are correct"
    else
      log step_failure
      found_error "Incorrect scsiProtocol value, must be 'FC', 'ISCSI', 'auto' or 'None'"
    fi
  else
    log step_failure
    found_error "${VALUES} doesn't exists"
  fi
}

# Check if the iSCSI client is installed
function verify_iscsi_installation() {
  if [ ${NODE_VERIFY} -eq 0 ]; then
    return
  fi

  log smart_step "Verifying iSCSI installation" "$1"

  error=0
  for node in $MINION_NODES; do
    # check if the iSCSI client is installed
    run_command ssh ${NODEUSER}@"${node}" "cat /etc/iscsi/initiatorname.iscsi" >/dev/null 2>&1
    rv=$?
    if [ $rv -ne 0 ]; then
      error=1
      found_warning "Either iSCSI client was not found on node: $node or not able to verify"
    fi
    run_command ssh ${NODEUSER}@"${node}" pgrep iscsid &>/dev/null
    rv=$?
    if [ $rv -ne 0 ]; then
      error=1
      found_warning "Either iscsid service is not running on node: $node or not able to verify"
    fi
  done

  check_error error
}


function verify_unity_protocol_installation() {
if [ ${NODE_VERIFY} -eq 0 ]; then
    return
  fi

  log smart_step "Verifying sshpass installation.."
  SSHPASS=$(which sshpass)
  if [ -z "$SSHPASS" ]; then
   found_warning "sshpass is not installed. It is mandatory to have ssh pass software for multi node kubernetes setup."
  fi
  
   
  log smart_step "Verifying iSCSI installation" "$1"

  error=0
  for node in $MINION_NODES; do
    # check if the iSCSI client is installed
    echo
    echo -n "Enter the ${NODEUSER} password of ${node}: "
    read -s nodepassword
    echo
    echo "$nodepassword" > protocheckfile
    chmod 0400 protocheckfile
    unset nodepassword
    run_command sshpass -f protocheckfile ssh -o StrictHostKeyChecking=no ${NODEUSER}@"${node}" "cat /etc/iscsi/initiatorname.iscsi" > /dev/null 2>&1
    rv=$?
    if [ $rv -ne 0 ]; then
      error=1
      found_warning "iSCSI client is either not found on node: $node or not able to verify"
    fi
    run_command sshpass -f protocheckfile ssh -o StrictHostKeyChecking=no ${NODEUSER}@"${node}" "pgrep iscsid" > /dev/null 2>&1
    rv1=$?
    if [ $rv1 -ne 0 ]; then
      error=1
      found_warning "iscsid service is either not running on node: $node or not able to verify"
    fi
    rm -f protocheckfile
  done
  check_error error
}

# Check if the fc is installed
function verify_fc_installation() {
  if [ ${NODE_VERIFY} -eq 0 ]; then
    return
  fi

  log smart_step "Verifying FC installation" "$1"

  error=0
  for node in $MINION_NODES; do
    # check if FC hosts are available
   run_command ssh ${NODEUSER}@${node} 'ls --hide=* /sys/class/fc_host/* 1>/dev/null' &>/dev/null
    rv=$?
    if [[ ${rv} -ne 0 ]]; then
      error=1
      found_warning "can't find any FC hosts on node: $node"
    fi
  done

  check_error error
}

# verify secrets exist
function verify_required_secrets() {
  log step "Verifying that required secrets have been created"

  error=0
  for N in "${@}"; do
    # Make sure the secret has already been established
    run_command kubectl get secrets -n "${NS}" 2>/dev/null | grep "${N}" --quiet
    if [ $? -ne 0 ]; then
      error=1
      found_error "Required secret, ${N}, does not exist."
    fi
  done
  check_error error
}

function verify_optional_secrets() {
  log step "Verifying that optional secrets have been created"

  error=0
  for N in "${@}"; do
    # Make sure the secret has already been established
    run_command kubectl get secrets -n "${NS}" 2>/dev/null | grep "${N}" --quiet
    if [ $? -ne 0 ]; then
      error=1
      found_warning "Optional secret, ${N}, does not exist."
    fi
  done
  check_error error
}

# verify minimum and maximum k8s versions
function verify_k8s_versions() {
  if [ "${OPENSHIFT}" == "true" ]; then
    return
  fi
  log step "Verifying Kubernetes versions"
  decho

  local MIN=${1}
  local MAX=${2}
  local V="${kMajorVersion}.${kMinorVersion}"
  # check minimum
  log arrow
  log smart_step "Verifying minimum Kubernetes version" "small"
  error=0
  if [[ ${V} < ${MIN} ]]; then
    error=1
    found_error "Kubernetes version, ${V}, is too old. Minimum required version is: ${MIN}"
  fi
  check_error error

  # check maximum
  log arrow
  log smart_step "Verifying maximum Kubernetes version" "small"
  error=0
  if [[ ${V} > ${MAX} ]]; then
    error=1
    found_warning "Kubernetes version, ${V}, is newer than has been tested. Latest tested version is: ${MAX}"
  fi
  check_error error

}

# verify minimum and maximum openshift versions
function verify_openshift_versions() {
  if [ "${OPENSHIFT}" != "true" ]; then
    return
  fi
  log step "Verifying OpenShift versions"
  decho

  local MIN=${1}
  local MAX=${2}
  local V=$(OpenShiftVersion)
  # check minimum
  log arrow
  log smart_step "Verifying minimum OpenShift version" "small"
  error=0
  if [[ ${V} < ${MIN} ]]; then
    error=1
    found_error "OpenShift version, ${V}, is too old. Minimum required version is: ${MIN}"
  fi
  check_error error

  # check maximum
  log arrow
  log smart_step "Verifying maximum OpenShift version" "small"
  error=0
  if [[ ${V} > ${MAX} ]]; then
    error=1
    found_warning "OpenShift version, ${V}, is newer than has been tested. Latest tested version is: ${MAX}"
  fi
  check_error error
}

# verify namespace
function verify_namespace() {
  log step "Verifying that required namespaces have been created"

  error=0
  for N in "${@}"; do
    # Make sure the namespace exists
    run_command kubectl describe namespace "${N}" >/dev/null 2>&1
    if [ $? -ne 0 ]; then
      error=1
      found_error "Namespace does not exist: ${N}"
    fi
  done

  check_error error
}

# verify that the no alpha version of volume snapshot resource is present on the system
function verify_alpha_snap_resources() {
  log step "Verifying alpha snapshot resources"
  decho
  log arrow
  log smart_step "Verifying that alpha snapshot CRDs are not installed" "small"

  error=0
  # check for the alpha snapshot CRDs. These shouldn't be present for installation to proceed with
  CRDS=("VolumeSnapshotClasses" "VolumeSnapshotContents" "VolumeSnapshots")
  for C in "${CRDS[@]}"; do
    # Verify that alpha snapshot related CRDs/CRs are not there on the system.
    run_command kubectl explain ${C} 2> /dev/null | grep "^VERSION.*v1alpha1$" --quiet
    if [ $? -eq 0 ]; then
      error=1
      found_error "The alhpa CRD for ${C} is installed. Please uninstall it"
      if [[ $(run_command kubectl get ${C} -A --no-headers 2>/dev/null | wc -l) -ne 0 ]]; then
        found_error " Found CR for alpha CRD ${C}. Please delete it"
      fi
    fi
  done
  check_error error
}

# verify that the requirements for beta snapshot support exist
function verify_beta_snap_requirements() {
  log step "Verifying beta snapshot support"
  decho
  log arrow
  log smart_step "Verifying that beta snapshot CRDs are available" "small"

  error=0
  # check for the CRDs. These are required for installation
  CRDS=("VolumeSnapshotClasses" "VolumeSnapshotContents" "VolumeSnapshots")
  for C in "${CRDS[@]}"; do
    # Verify if snapshot related CRDs are there on the system. If not install them.
    run_command kubectl explain ${C} 2> /dev/null | grep "^VERSION.*v1beta1$" --quiet
    if [ $? -ne 0 ]; then
      error=1
      if [ "${INSTALL_CRD}" == "yes" ]; then
        found_warning "The beta CRD for ${C} is not installed. They will be installed because --snapshot-crd was specified"
      else
        found_error "The beta CRD for ${C} is not installed. These can be installed by specifying --snapshot-crd during installation"
      fi
    fi
  done
  check_error error

  log arrow
  log smart_step "Verifying that beta snapshot controller is available" "small"

  error=0
  # check for the snapshot-controller. These are strongly suggested but not required
  run_command kubectl get pods -A | grep snapshot-controller --quiet
  if [ $? -ne 0 ]; then
    error=1
    found_warning "The Snapshot Controller does not seem to be deployed. The Snapshot Controller should be provided by the Kubernetes vendor or administrator."
  fi

  check_error error
}

# verify that helm is v3 or above
function verify_helm_3() {
  log step "Verifying helm version"

  error=0
  # Check helm installer version
  helm --help >&/dev/null || {
    found_error "helm is required for installation"
    log step_failure
    return
  }

  run_command helm version | grep "v3." --quiet
  if [ $? -ne 0 ]; then
    error=1
    found_error "Driver installation is supported only using helm 3"
  fi

  check_error error
}

# found_error, installation will not continue
function found_error() {
  for N in "$@"; do
    ERRORS+=("${N}")
  done
}

# found_warning, installation can continue
function found_warning() {
  for N in "$@"; do
    WARNINGS+=("${N}")
  done
}

# Print a nice summary at the end
function summary() {
  local VERSTATUS="Success"
  if [ "${#WARNINGS[@]}" -ne 0 ]; then
    VERSTATUS="With Warnings"
  fi
  if [ "${#ERRORS[@]}" -ne 0 ]; then
    VERSTATUS="With Errors"
  fi
  decho
  log section "Verification Complete - ${VERSTATUS}"
  # print all the WARNINGS
  NON_CRD_WARNINGS=0
  if [ "${#WARNINGS[@]}" -ne 0 ]; then
    log warnings
    for E in "${WARNINGS[@]}"; do
      decho "- ${E}"
      decho ${E} | grep --quiet "^The beta CRD for VolumeSnapshot"
      if [ $? -ne 0 ]; then
        NON_CRD_WARNINGS=1
      fi
    done
    RC=$EXIT_WARNING
    if [ "${INSTALL_CRD}" == "yes" -a ${NON_CRD_WARNINGS} -eq 0 ]; then
      RC=$EXIT_SUCCESS
    fi
  fi

  # print all the ERRORS
  if [ "${#ERRORS[@]}" -ne 0 ]; then
    log errors
    for E in "${ERRORS[@]}"; do
      decho "- ${E}"
    done
    RC=$EXIT_ERROR
  fi

  return $RC
}

#
# validate_params will validate the parameters passed in
function validate_params() {
  # make sure the driver was specified
  if [ -z "${DRIVER}" ]; then
    decho "No driver specified"
    usage
    exit 1
  fi
  # make sure the driver name is valid
  if [[ ! "${VALIDDRIVERS[@]}" =~ "${DRIVER}" ]]; then
    decho "Driver: ${DRIVER} is invalid."
    decho "Valid options are: ${VALIDDRIVERS[@]}"
    usage
    exit 1
  fi
  # the namespace is required
  if [ -z "${NS}" ]; then
    decho "No namespace specified"
    usage
    exit 1
  fi
  # values file
  if [ -z "${VALUES}" ]; then
    decho "No values file was specified"
    usage
    exit 1
  fi
  if [ ! -f "${VALUES}" ]; then
    decho "Unable to read values file at: ${VALUES}"
    usage
    exit 1
  fi
}

#
# main
#
# default values

NODE_VERIFY=1

# exit codes
EXIT_SUCCESS=0
EXIT_WARNING=1
EXIT_ERROR=99

# arrays of messages
WARNINGS=()
ERRORS=()

INSTALL_CRD="no"

# make sure kubectl is available
kubectl --help >&/dev/null || {
  decho "kubectl required for verification... exiting"
  exit $EXIT_ERROR
}

# Determine the nodes
MINION_NODES=$(run_command kubectl get nodes -o wide | grep -v -e master -e INTERNAL | awk ' { print $6; }')
MASTER_NODES=$(run_command kubectl get nodes -o wide | awk ' /master/{ print $6; }')
# Get the kubernetes major and minor version numbers.
kMajorVersion=$(run_command kubectl version | grep 'Server Version' | sed -e 's/^.*Major:"//' -e 's/[^0-9].*//g')
kMinorVersion=$(run_command kubectl version | grep 'Server Version' | sed -e 's/^.*Minor:"//' -e 's/[^0-9].*//g')

# get the list of valid CSI Drivers, this will be the list of directories in drivers/ that contain helm charts
get_drivers "${SCRIPTDIR}/../helm"
# if only one driver was found, set the DRIVER to that one
if [ ${#VALIDDRIVERS[@]} -eq 1 ]; then
  DRIVER="${VALIDDRIVERS[0]}"
fi

while getopts ":h-:" optchar; do
  case "${optchar}" in
  -)
    case "${OPTARG}" in
    # INSTALL_CRD. Signifies that we were asked to install the CRDs
    snapshot-crd)
      INSTALL_CRD="yes"
      ;;
    skip-verify-node)
      NODE_VERIFY=0
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
      NODEUSER=${OPTARG#*=}
      ;;
    *)
      decho "Unknown option --${OPTARG}"
      decho "For help, run $PROG -h"
      exit $EXIT_ERROR
      ;;
    esac
    ;;
  h)
    usage
    ;;
  *)
    decho "Unknown option -${OPTARG}"
    decho "For help, run $PROG -h"
    exit $EXIT_ERROR
    ;;
  esac
done

# by default the NAME of the helm release of the driver is the same as the driver name
RELEASE=$(get_release_name "${DRIVER}")

#"${RELEASE:-$DRIVER}"
# by default, NODEUSER is root
NODEUSER="${NODEUSER:-root}"

# validate the parameters passed in
validate_params "${MODE}"
OPENSHIFT=$(isOpenShift)

verify-driver "${DRIVER}"
exit $?
