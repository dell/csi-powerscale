#!/bin/bash
#
# Copyright (c) 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#  http://www.apache.org/licenses/LICENSE-2.0

# bundle a CSI driver helm chart, installation scripts, and
# container images into a tarball that can be used for offline installations

# display some usage information
usage() {
   echo
   echo "$0"
   echo "Make a package for offline installation of a CSI driver"
   echo
   echo "Arguments:"
   echo "-c             Create an offline bundle"
   echo "-p             Prepare this bundle for installation"
   echo "-r <registry>  Required if preparing offline bundle with '-p'"
   echo "               Supply the registry name/path which will hold the images"
   echo "               For example: my.registry.com:5000/dell/csi"
   echo "-h             Displays this information"
   echo
   echo "Exactly one of '-c' or '-p' needs to be specified"
   echo
}

# status
# echos a brief status sttement to stdout
status() {
  echo
  echo "*"
  echo "* $@"
  echo 
}

# run_command
# runs a shell command
# exits and prints stdout/stderr when a non-zero return code occurs
run_command() {
  CMDOUT=$(eval "${@}" 2>&1)
  local rc=$?

  if [ $rc -ne 0 ]; then
    echo
    echo "ERROR"
    echo "Received a non-zero return code ($rc) from the following comand:"
    echo "  ${@}"
    echo
    echo "Output was:"
    echo "${CMDOUT}"
    echo
    echo "Exiting"
    exit 1
  fi
}

# build_image_manifest
# builds a manifest of all the images referred to by the helm chart
build_image_manifest() {
  local REGEX="([-_./:A-Za-z0-9]{3,}):([-_.A-Za-z0-9]{1,})"

  status "Building image manifest file"
  if [ -e "${IMAGEFILEDIR}" ]; then
    rm -rf "${IMAGEFILEDIR}"
  fi
  if [ -f "${IMAGEMANIFEST}" ]; then
    rm -rf "${IMAGEMANIFEST}"
  fi

  for D in ${DIRS_FOR_IMAGE_NAMES[@]}; do
    echo "   Processing files in ${D}"
    if [ ! -d "${D}" ]; then
      echo "Unable to find directory, ${D}. Skipping"
    else
      # look for strings that appear to be image names, this will
      # - search all files in a diectory looking for strings that make $REGEX
      # - exclude anything with double '//'' as that is a URL and not an image name
      # - make sure at least one '/' is found
      find "${D}" -type f -exec egrep -oh "${REGEX}" {} \; | egrep -v '//' | egrep '/' >> "${IMAGEMANIFEST}.tmp"
    fi
  done

  # sort and uniqify the list
  cat "${IMAGEMANIFEST}.tmp" | sort | uniq > "${IMAGEMANIFEST}"
  rm "${IMAGEMANIFEST}.tmp"
}

# archive_images
# archive the necessary docker images by pulling them locally and then saving them
archive_images() {
  status "Pulling and saving container images"

  if [ ! -d "${IMAGEFILEDIR}" ]; then
    mkdir -p "${IMAGEFILEDIR}"
  fi

  # the images, pull first in case some are not local
  while read line; do
      echo "   $line"
      run_command "${DOCKER}" pull "${line}"
      IMAGEFILE=$(echo "${line}" | sed 's|[/:]|-|g')
      # if we already have the image exported, skip it
      if [ ! -f "${IMAGEFILEDIR}/${IMAGEFILE}.tar" ]; then
        run_command "${DOCKER}" save -o "${IMAGEFILEDIR}/${IMAGEFILE}.tar" "${line}"
      fi
  done < "${IMAGEMANIFEST}"

} 

# restore_images
# load the images from an archive into the local registry
# then push them to the target registry
restore_images() {
  status "Loading docker images"
  find "${IMAGEFILEDIR}" -name \*.tar -exec "${DOCKER}" load -i {} \; 2>/dev/null

  status "Tagging and pushing images"
  while read line; do
      local NEWNAME="${REGISTRY}${line##*/}"
      echo "   $line -> ${NEWNAME}"
      run_command "${DOCKER}" tag "${line}" "${NEWNAME}"
      run_command "${DOCKER}" push "${NEWNAME}"
  done < "${IMAGEMANIFEST}"
}

# copy in any necessary files
copy_files() {
  status "Copying necessary files"
  for f in ${REQUIRED_FILES[@]}; do
    echo " ${f}"
    cp -R "${f}" "${DISTDIR}"
    if [ $? -ne 0 ]; then
      echo "Unable to copy ${f} to the distribution directory"
      exit 1
    fi
  done
}

# fix any references in the helm charts or operator configuration
fixup_files() {

  local ROOTDIR="${HELMDIR}"

  if [ "${MODE}" == "operator" ]; then
    ROOTDIR="${REPODIR}"
  fi

  status "Preparing ${MODE} files within ${ROOTDIR}"

  # for each image in the manifest, replace the old name with the new
  while read line; do
      local NEWNAME="${REGISTRY}${line##*/}"
      echo "   changing: $line -> ${NEWNAME}"
      find "${ROOTDIR}" -type f -not -path "${SCRIPTDIR}/*" -exec sed -i "s|$line|$NEWNAME|g" {} \;
  done < "${IMAGEMANIFEST}"
}

# compress the whole bundle
compress_bundle() {
  status "Compressing release"
  cd "${DISTBASE}" && tar cvfz "${DISTFILE}" "${DRIVERDIR}"
  if [ $? -ne 0 ]; then
    echo "Unable to package build"
    exit 1
  fi
  rm -rf "${DISTDIR}"
}

# copy_helm_dir
# make a copy of the helm directory if one does not already exist
copy_helm_dir() {
  if [ "${MODE}" != "helm" ]; then
    return
  fi

  status "Ensuring a copy of the helm directory exists"
  if [ -d "${HELMBACKUPDIR}" ]; then
    return
  fi

  mkdir -p "${HELMBACKUPDIR}"
  cp -R "${HELMDIR}"/* "${HELMBACKUPDIR}"
}

# set_mode
# figure out if we are working from:
# - a driver repo and using helm
# - the operator repo which means we are using an operator
set_mode() {
  # default is helm
  MODE="helm"

  if [ ! -d "${HELMDIR}" ]; then
    MODE="operator"
  fi
}

#------------------------------------------------------------------------------
#
# Main script logic starts here
#

# default values, overridable by users
CREATE="false"
PREPARE="false"
REGISTRY=""

# some directories
SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
REPODIR="$( dirname "${SCRIPTDIR}" )"
HELMDIR="${REPODIR}/helm"
HELMBACKUPDIR="${REPODIR}/helm-original"

# mode we are using for install, "helm" or "operator"
set_mode

if [ "${MODE}" == "helm" ]; then
  INSTALLERDIR="${REPODIR}/dell-csi-helm-installer"
  CHARTFILE=$(find "${HELMDIR}" -maxdepth 2 -type f -name Chart.yaml)

  # some output files
  DRIVERNAME=$(grep -oh "^name:\s.*" "${CHARTFILE}" | awk '{print $2}')
  DRIVERNAME=${DRIVERNAME:-"dell-csi-driver"}
  DRIVERVERSION=$(grep -oh "^version:\s.*" "${CHARTFILE}" | awk '{print $2}')
  DRIVERVERSION=${DRIVERVERSION:-unknown}
  DISTBASE="${REPODIR}"
  DRIVERDIR="${DRIVERNAME}-bundle-${DRIVERVERSION}"
  DISTDIR="${DISTBASE}/${DRIVERDIR}"
  DISTFILE="${DISTBASE}/${DRIVERDIR}.tar.gz"
  IMAGEMANIFEST="${INSTALLERDIR}/images.manifest"
  IMAGEFILEDIR="${INSTALLERDIR}/images.tar"

  # directories to search all files for image names
  DIRS_FOR_IMAGE_NAMES=(
    "${HELMDIR}"
  )
  # list of all files to be included
  REQUIRED_FILES=(
    "${HELMDIR}"
    "${INSTALLERDIR}"
    "${REPODIR}/*.md"
    "${REPODIR}/LICENSE"
  )
else
  DRIVERNAME="dell-csi-operator"
  DISTBASE="${REPODIR}"
  DRIVERDIR="${DRIVERNAME}-bundle"
  DISTDIR="${DISTBASE}/${DRIVERDIR}"
  DISTFILE="${DISTBASE}/${DRIVERDIR}.tar.gz"
  IMAGEMANIFEST="${REPODIR}/scripts/images.manifest"
  IMAGEFILEDIR="${REPODIR}/scripts/images.tar"


  # directories to search all files for image names
  DIRS_FOR_IMAGE_NAMES=(
    "${REPODIR}/driverconfig"
    "${REPODIR}/deploy"
    "${REPODIR}/samples"
  )

  # list of all files to be included
  REQUIRED_FILES=(
    "${REPODIR}/driverconfig"
    "${REPODIR}/deploy"
    "${REPODIR}/samples"
    "${REPODIR}/scripts"
    "${REPODIR}/*.md"
    "${REPODIR}/LICENSE"
  )
fi

while getopts "cpr:h" opt; do
  case $opt in
    c)
      CREATE="true"
      ;;
    p)
      PREPARE="true"
      ;;
    r)
      REGISTRY="${OPTARG}"
      ;;
    h)
      usage
      exit 0
      ;;
    \?)
      echo "Invalid option: -$OPTARG" >&2
      exit 1
      ;;
    :)
      echo "Option -$OPTARG requires an argument." >&2
      exit 1
      ;;
  esac
done

# make sure exatly one option for create/prepare was specified
if [ "${CREATE}" == "${PREPARE}" ]; then
  usage
  exit 1
fi

# validate prepare arguments
if [ "${PREPARE}" == "true" ]; then
  if [ "${REGISTRY}" == "" ]; then
    usage
    exit 1
  fi
fi

if [ "${REGISTRY: -1}" != "/" ]; then
  REGISTRY="${REGISTRY}/"
fi

# figure out if we should use docker or podman, preferring docker
DOCKER=$(which docker 2>/dev/null || which podman 2>/dev/null)   
if [ "${DOCKER}" == "" ]; then
  echo "Unable to find either docker or podman in $PATH"
  exit 1
fi

# create a bundle
if [ "${CREATE}" == "true" ]; then
  if [ -d "${DISTDIR}" ]; then
    rm -rf "${DISTDIR}"
  fi
  if [ ! -d "${DISTDIR}" ]; then
    mkdir -p "${DISTDIR}"
  fi
  if [ -f "${DISTFILE}" ]; then
    rm -f "${DISTFILE}"
  fi
  build_image_manifest
  archive_images
  copy_files
  compress_bundle

  status "Complete"
  echo "Offline bundle file is: ${DISTFILE}"
fi

# prepare a bundle for installation
if [ "${PREPARE}" == "true" ]; then
  echo "Preparing a offline bundle for installation"
  restore_images
  copy_helm_dir
  fixup_files

  status "Complete"

  if [ "${MODE}" == "helm" ]; then
    echo "Installation of the ${DRIVERNAME} driver can now be performed via"
    echo "the scripts in ${INSTALLERDIR}"
  fi
fi

echo

exit 0
