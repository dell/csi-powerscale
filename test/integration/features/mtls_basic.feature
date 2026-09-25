# Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

Feature: mTLS Basic Integration Tests
  As a platform operator
  I want to verify basic mTLS functionality against a real PowerScale array
  So that I can ensure the driver correctly handles mTLS configuration

  Background:
    Given a Isilon service

  @mtls-basic
  Scenario: Create volume with basic configuration
    Given a basic volume request "mtls-test-volume" "8"
    When I call CreateVolume
    Then there is a directory "mtls-test-volume"
    And there is an export "mtls-test-volume"
    When I call DeleteVolume
    Then there is not a directory "mtls-test-volume"
    And there is not an export "mtls-test-volume"

  @mtls-basic
  Scenario: Create volume with mTLS StorageClass parameters
    Given a basic volume request "mtls-params-volume" "8"
    And StorageClass with NFSTransportSecurity: "mtls"
    And StorageClass with SmartConnectZoneFQDN: "powerscale.test.local"
    When I call CreateVolume
    Then there is a directory "mtls-params-volume"
    And there is an export "mtls-params-volume"
    When I call DeleteVolume
    Then there is not a directory "mtls-params-volume"
    And there is not an export "mtls-params-volume"

  @mtls-basic
  Scenario: Create volume with TLS transport security
    Given a basic volume request "tls-volume" "8"
    And StorageClass with NFSTransportSecurity: "tls"
    And StorageClass with SmartConnectZoneFQDN: "powerscale.test.local"
    When I call CreateVolume
    Then there is a directory "tls-volume"
    And there is an export "tls-volume"
    When I call DeleteVolume
    Then there is not a directory "tls-volume"
    And there is not an export "tls-volume"

  @mtls-basic
  Scenario: Create volume with custom mount options
    Given a basic volume request "mount-opts-volume" "8"
    And StorageClass mountOptions include "rw"
    And StorageClass mountOptions include "hard"
    When I call CreateVolume
    Then there is a directory "mount-opts-volume"
    And there is an export "mount-opts-volume"
    When I call DeleteVolume
    Then there is not a directory "mount-opts-volume"
    And there is not an export "mount-opts-volume"

  @mtls-basic
  Scenario: Create volume without SmartConnectZoneFQDN
    Given a basic volume request "no-fqdn-volume" "8"
    And StorageClass SmartConnectZoneFQDN is not set
    When I call CreateVolume
    Then there is a directory "no-fqdn-volume"
    And there is an export "no-fqdn-volume"
    When I call DeleteVolume
    Then there is not a directory "no-fqdn-volume"
    And there is not an export "no-fqdn-volume"
