#!/bin/bash

# Configuration file for isilon driver
# This file is sourced from the longevity and scalbility test scripts
# It will be executed with 'cwd' set to this directory

# Longevity test helm chart
LONGTEST="${LONGTEST:-volumes}"
LONGREPLICAS="${LONGREPLICAS:-2}"

# Scalability test helm chart
SCALETEST="${SCALETEST:-volumes}"
SCALEREPLICAS="${SCALEREPLICAS:-3}"

# Namespace to run tests in. Will be created if it does not exist
NAMESPACE="${NAMESPACE:-test}"

# Storage Class names. Three of these are required but they can be the same.
# These must exist before starting tests
STORAGECLASS1="${STORAGECLASS1:-isilon}"
STORAGECLASS2="${STORAGECLASS2:-isilon}"
STORAGECLASS3="${STORAGECLASS3:-isilon}"

# The namespace that the driver is running within
DRIVERNAMESPACE="isilon"