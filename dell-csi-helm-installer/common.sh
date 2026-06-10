#!/bin/bash
# Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied
# See the License for the specific language governing permissions and
# limitations under the License

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
  uninstall_error)
    log separator
    printf "${RED}Error: $2\n"
    printf "${RED}Uninstallation cannot continue${NC}\n"
    debuglog_only "Error: $2"
    debuglog_only "Uninstallation cannot continue"
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

# detect_helm_version detects the installed Helm client major.minor.patch version.
# Sets HELM_MAJOR_VERSION, HELM_MINOR_VERSION, HELM_PATCH_VERSION, HELM_VERSION_STRING.
# Exits 1 if helm is not found or version cannot be parsed.
function detect_helm_version() {
  local raw
  if ! command -v helm >/dev/null 2>&1; then
    echo "helm not found on PATH. Install Helm from https://helm.sh/docs/intro/install/" >&2
    debuglog_only "helm not found on PATH"
    exit 1
  fi
  raw=$(helm version --short 2>/dev/null)
  if [[ $? -ne 0 || -z "${raw}" ]]; then
    echo "helm found on PATH but 'helm version --short' failed. Check your Helm installation." >&2
    debuglog_only "helm version check failed"
    exit 1
  fi
  if [[ "${raw}" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+) ]]; then
    HELM_MAJOR_VERSION="${BASH_REMATCH[1]}"
    HELM_MINOR_VERSION="${BASH_REMATCH[2]}"
    HELM_PATCH_VERSION="${BASH_REMATCH[3]}"
    HELM_VERSION_STRING="v${HELM_MAJOR_VERSION}.${HELM_MINOR_VERSION}.${HELM_PATCH_VERSION}"
    debuglog_only "helm_version_detected: ${HELM_VERSION_STRING} (major=${HELM_MAJOR_VERSION})"
    return 0
  else
    echo "Unable to parse Helm version from: ${raw}" >&2
    debuglog_only "Unable to parse Helm version from: ${raw}"
    exit 1
  fi
}

# validate_helm_version checks that the detected Helm major version is supported (>= 3).
# Argument $1: major version integer.
# Exits 1 with stderr message if unsupported (<= 2).
function validate_helm_version() {
  local major="${1}"
  local version_str="${HELM_VERSION_STRING:-unknown}"
  if [[ -z "${major}" || "${major}" -lt 3 ]]; then
    echo "Unsupported Helm version detected: ${version_str}. Minimum supported: v3.x. Upgrade Helm: https://helm.sh/docs/intro/install/" >&2
    debuglog_only "Unsupported helm version: ${version_str}"
    exit 1
  fi
}

# record_helm_telemetry writes a structured telemetry entry to DEBUGLOG.
# Arguments: $1=operation (install|upgrade|uninstall), $2=driver_name, $3=result (pending|success|failure)
function record_helm_telemetry() {
  local op="${1}"
  local driver="${2}"
  local result="${3}"
  debuglog_only "helm_telemetry: version=${HELM_VERSION_STRING} major=${HELM_MAJOR_VERSION} operation=${op} driver=${driver} result=${result} timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}

# helm_registry_login performs helm registry login with Helm v3/v4 format normalization.
# Arguments: $1=registry_url (with or without scheme), $2=username, $3=registry auth value, $4=use_plain_http (optional)
# Auth value is passed via stdin only — never in command args or logs.
function helm_registry_login() {
  local registry="${1}"
  local username="${2}"
  local registry_auth="${3}"
  local use_plain_http="${4:-false}"
  if [[ -z "${registry}" ]]; then
    echo "helm_registry_login: registry URL argument is required" >&2
    return 1
  fi
  registry="${registry#https://}"
  registry="${registry#http://}"
  debuglog_only "helm_registry_login: registry=${registry} plain_http=${use_plain_http}"
  
  local plain_http_flag=""
  if [[ "${use_plain_http}" == "true" ]]; then
    plain_http_flag="--plain-http"
  fi
  
  echo "${registry_auth}" | helm registry login "${registry}" -u "${username}" --password-stdin "${plain_http_flag}"
  local rc=$?
  if [[ "${rc}" -ne 0 ]]; then
    echo "Registry login failed for ${registry}" >&2
    return 1
  fi
  return 0
}

# detect_ssa_conflict_in_output parses a Helm v4 stderr output file for SSA field ownership conflicts.
# Emits a structured error message to stderr when a conflict is detected.
# Argument $1: path to the file containing helm error output.
function detect_ssa_conflict_in_output() {
  local output_file="${1}"
  if grep -q "Apply failed with" "${output_file}" 2>/dev/null; then
    local field manager resource namespace
    field=$(grep -oP '(?<=\.)[a-zA-Z.]+(?= is immutable| owned by)' "${output_file}" 2>/dev/null | head -1)
    manager=$(grep -oP '(?<=manager ")([^"]+)(?=")' "${output_file}" 2>/dev/null | head -1)
    resource=$(grep -oP '(?<=resource ")[^"]+' "${output_file}" 2>/dev/null | head -1)
    namespace=$(grep -oP '(?<=namespace ")([^"]+)(?=")' "${output_file}" 2>/dev/null | head -1)
    echo "SSA conflict: field=${field}, conflicting_manager=${manager}, resource=${resource}, namespace=${namespace}. See Helm v4 migration guide for resolution steps." >&2
    debuglog_only "ssa_conflict: field=${field} manager=${manager} resource=${resource} namespace=${namespace}"
  fi
}

