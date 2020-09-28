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

function log() {
  case $1 in
  separator)
    echo "------------------------------------------------------"
    ;;
  error)
    echo
    log separator
    printf "${RED}Error: $2\n"
    printf "${RED}Installation cannot continue${NC}\n"
    exit 1
    ;;
  step)
    printf "|\n|- %-65s" "$2"
    ;;
  small_step)
    printf "%-61s" "$2"
    ;;
  section)
    log separator
    printf "> %s\n" "$2"
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
    DDD=$(echo $line | awk -F '/' '{print $(NF-1)}')
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
    echo "${RELEASE}"
    return
  fi

  local PREFIX="csi-"
  R=${D#"$PREFIX"}
  echo "${R}"
}
