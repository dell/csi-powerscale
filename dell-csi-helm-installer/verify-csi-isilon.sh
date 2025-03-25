#!/bin/bash
# Copyright © 2020-2025 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License

#
# verify-csi-isilon method
function verify-csi-isilon() {
  verify_k8s_versions "1.27" "1.33"
  verify_openshift_versions "4.18" "4.19"
  verify_namespace "${NS}"
  verify_required_secrets "${RELEASE}-creds"
  verify_optional_secrets "${RELEASE}-certs"
  verify_alpha_snap_resources
  verify_snap_requirements
  verify_helm_3
  verify_helm_values_version "${DRIVER_VERSION}"
  verify_authorization_proxy_server
}
