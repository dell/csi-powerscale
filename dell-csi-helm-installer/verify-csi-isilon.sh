#!/bin/bash
#
# Copyright (c) 2020 Dell Inc., or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#  http://www.apache.org/licenses/LICENSE-2.0

#
# verify-csi-isilon method
function verify-csi-isilon() {
  verify_k8s_versions "1.21" "1.25"
  verify_openshift_versions "4.9" "4.10"
  verify_namespace "${NS}"
  verify_required_secrets "${RELEASE}-creds"
  verify_optional_secrets "${RELEASE}-certs"
  verify_alpha_snap_resources
  verify_snap_requirements
  verify_helm_3
  verify_helm_values_version "2.5.0"
  verify_authorization_proxy_server
}
