#!/bin/bash
#
# Copyright (c) 2020 Dell Inc., or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#  http://www.apache.org/licenses/LICENSE-2.0

DRIVERDIR="${SCRIPTDIR}/../helm"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
DARK_GRAY='\033[1;30m'
NC='\033[0m' # No Color

function decho() {
  if [ -n "${DEBUGLOG}" ]; then
    echo "$@" | tee -a "${DEBUGLOG}"
  fi
}

function debuglog_only() {
  if [ -n "${DEBUGLOG}" ]; then
    echo "$@" >> "${DEBUGLOG}"
  fi
}

function log() {
  case $1 in
  separator)
    decho "------------------------------------------------------"
    ;;
  error)
    decho
    log separator
    printf "${RED}Error: $2\n"
    printf "${RED}Installation cannot continue${NC}\n"
    debuglog_only "Error: $2"
    debuglog_only "Installation cannot continue"
    exit 1
    ;;
  step)
    printf "|\n|- %-65s" "$2"
    debuglog_only "${2}"
    ;;
  small_step)
    printf "%-61s" "$2"
    debuglog_only "${2}"
    ;;
  section)
    log separator
    printf "> %s\n" "$2"
    debuglog_only "${2}"
    log separator
    ;;
  smart_step)
    if [[ $3 == "small" ]]; then
      log small_step "$2"
    else
      log step "$2"
    fi
    ;;
  arrow)
    printf "  %s\n  %s" "|" "|--> "
    ;;
  step_success)
    printf "${GREEN}Success${NC}\n"
    ;;
  step_failure)
    printf "${RED}Failed${NC}\n"
    ;;
  step_warning)
    printf "${YELLOW}Warning${NC}\n"
    ;;
  info)
    printf "${DARK_GRAY}%s${NC}\n" "$2"
    ;;
  passed)
    printf "${GREEN}Success${NC}\n"
    ;;
  warnings)
    printf "${YELLOW}Warnings:${NC}\n"
    ;;
  errors)
    printf "${RED}Errors:${NC}\n"
    ;;
  *)
    echo -n "Unknown"
    ;;
  esac
}

function check_error() {
  if [[ $1 -ne 0 ]]; then
    log step_failure
  else
    log step_success
  fi
}

#
# get_drivers will populate an array of drivers found by
# enumerating the directories in drivers/ that contain a helm chart
function get_drivers() {
  D="${1}"
  TTT=$(pwd)
  while read -r line; do
    DDD=$(decho $line | awk -F '/' '{print $(NF-1)}')
    VALIDDRIVERS+=("$DDD")
  done < <(find "${D}" -maxdepth 2 -type f -name Chart.yaml | sort)
}

#
# get_release will determine the helm release name to use
# If ${RELEASE} is set, use that
# Otherwise, use the driver name minus any "csi-" prefix
# argument 1: Driver name
function get_release_name() {
  local D="${1}"
  if [ ! -z "${RELEASE}" ]; then
    decho "${RELEASE}"
    return
  fi

  local PREFIX="csi-"
  R=${D#"$PREFIX"}
  decho "${R}"
}

function run_command() {
  local RC=0
  if [ -n "${DEBUGLOG}" ]; then
    local ME=$(basename "${0}")
    echo "---------------" >> "${DEBUGLOG}"
    echo "${ME}:${BASH_LINENO[0]} - Running command: $@" >> "${DEBUGLOG}"
    debuglog_only "Results:"
    eval "$@" | tee -a "${DEBUGLOG}"
    RC=${PIPESTATUS[0]}
    echo "---------------" >> "${DEBUGLOG}"
  else
    eval "$@"
    RC=$?
  fi
  return $RC
}

# dump out information about a helm chart to the debug file
# takes a few arguments
# $1 the namespace
# $2 the release
function debuglog_helm_status() {
  local NS="${1}"
  local RLS="${2}"

  debuglog_only "Getting information about Helm release: ${RLS}"
  debuglog_only "****************"
  debuglog_only "Helm Status:"
  helm status "${RLS}" -n "${NS}" >> "${DEBUGLOG}"
  debuglog_only "****************"
  debuglog_only "Manifest"
  helm get manifest "${RLS}" -n "${NS}" >> "${DEBUGLOG}"
  debuglog_only "****************"
  debuglog_only "Status of resources"
  helm get manifest "${RLS}" -n "${NS}" | kubectl get -f - >> "${DEBUGLOG}"

}

# determines if the current KUBECONFIG is pointing to an OpenShift cluster
# echos "true" or "false" 
function isOpenShift() {
  # check if the securitycontextconstraints.security.openshift.io crd exists
  run_command kubectl get crd | grep securitycontextconstraints.security.openshift.io --quiet >/dev/null 2>&1
  local O=$?
  if [[ ${O} == 0 ]]; then
    # this is openshift
    echo "true"
  else
    echo "false"
  fi
}

# determines the version of OpenShift 
# echos version, or empty string if not OpenShift
function OpenShiftVersion() {
  # check if this is OpenShift
  local O=$(isOpenShift)
  if [ "${O}" == "false" ]; then
    # this is not openshift
    echo ""
  else
    local V=$(run_command kubectl get clusterversions -o jsonpath="{.items[*].status.desired.version}")
    local MAJOR=$(echo "${V}" | awk -F '.' '{print $1}')
    local MINOR=$(echo "${V}" | awk -F '.' '{print $2}')
    echo "${MAJOR}.${MINOR}"
  fi
}

