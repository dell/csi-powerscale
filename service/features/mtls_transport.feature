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

Feature: mTLS-Protected NFS Transport
	As a platform operator
	I want to provision volumes with mTLS-protected NFS transport
	So that data in transit is encrypted and mutually authenticated

Background:
	Given a Isilon service
	And I have a Node "node1" with AccessZone

@mtls
@v2.0.0
Scenario: Create volume with mTLS transport security
	Given I create a volume with name "mtls-volume-1"
	And I specify CreateVolume AccessZone "System"
	And I specify CreateVolume IsiPath "/ifs/data/csi"
	And I specify CreateVolume SmartConnectZoneFQDN "zone1.smartconnect.example.com"
	And I specify CreateVolume NFSTransportSecurity "mtls"
	When I call CreateVolume
	Then the error contains "none"
	And a valid CreateVolumeResponse is returned
	And the volume context contains "SmartConnectZoneFQDN" with value "zone1.smartconnect.example.com"
	And the volume context contains "NFSTransportSecurity" with value "mtls"

@mtls
@v2.0.0
Scenario: Node publish volume with mTLS using StorageClass FQDN
	Given a controller published volume
	And the volume context contains "SmartConnectZoneFQDN" with value "zone1.smartconnect.example.com"
	And the volume context contains "NFSTransportSecurity" with value "mtls"
	And a capability with voltype "mount" access "single-writer"
	And get Node Publish Volume Request
	When I call NodePublishVolume
	Then the error contains "none"

@mtls
@v2.0.0
Scenario: Node publish volume with mTLS using cluster config FQDN
	Given a controller published volume
	And the cluster config has nfsMountFQDN "cluster.smartconnect.example.com"
	And the volume context contains "NFSTransportSecurity" with value "mtls"
	And a capability with voltype "mount" access "single-writer"
	And get Node Publish Volume Request
	When I call NodePublishVolume
	Then the error contains "none"

@mtls
@v2.0.0
Scenario: Node publish volume with mTLS using environment variable FQDN
	Given a controller published volume
	And the environment variable X_CSI_ISI_NFS_MOUNT_FQDN is set to "env.smartconnect.example.com"
	And the volume context contains "NFSTransportSecurity" with value "mtls"
	And a capability with voltype "mount" access "single-writer"
	And get Node Publish Volume Request
	When I call NodePublishVolume
	Then the error contains "none"

@mtls
@v2.0.0
Scenario Outline: Reject mTLS mount with IP address
	Given a controller published volume
	And the volume context contains "SmartConnectZoneFQDN" with value <ip_address>
	And the volume context contains "NFSTransportSecurity" with value "mtls"
	And a capability with voltype "mount" access "single-writer"
	And get Node Publish Volume Request
	When I call NodePublishVolume
	Then the error contains <error_message>

	Examples:
	| ip_address    | error_message                                      |
	| "192.168.1.100" | "mount target '192.168.1.100' is an IP address"   |
	| "2001:db8::1"   | "mount target '2001:db8::1' is an IP address"     |

@mtls
@v2.0.0
Scenario: Reject mTLS mount with no FQDN configured
	Given a controller published volume
	And the volume context contains "NFSTransportSecurity" with value "mtls"
	And a capability with voltype "mount" access "single-writer"
	And get Node Publish Volume Request
	When I call NodePublishVolume
	Then the error contains "No FQDN configured for mTLS mount"

@mtls
@v2.0.0
Scenario: Reject mTLS mount with conflicting xprtsec=none mount option
	Given a controller published volume
	And the volume context contains "SmartConnectZoneFQDN" with value "zone1.smartconnect.example.com"
	And the volume context contains "NFSTransportSecurity" with value "mtls"
	And a capability with voltype "mount" access "single-writer"
	And the mount options include "xprtsec=none"
	And get Node Publish Volume Request
	When I call NodePublishVolume
	Then the error contains "xprtsec=none conflicts with NFSTransportSecurity: mtls"

@mtls
@v2.0.0
Scenario: Allow non-mTLS mount with IP address
	Given a controller published volume
	And the volume context contains "SmartConnectZoneFQDN" with value "192.168.1.100"
	And a capability with voltype "mount" access "single-writer"
	And get Node Publish Volume Request
	When I call NodePublishVolume
	Then the error contains "none"

@mtls
@v2.0.0
Scenario: Allow non-mTLS mount without FQDN
	Given a controller published volume
	And a capability with voltype "mount" access "single-writer"
	And get Node Publish Volume Request
	When I call NodePublishVolume
	Then the error contains "none"

@mtls
@v2.0.0
Scenario Outline: FQDN precedence tests
	Given a controller published volume
	And the volume context contains "SmartConnectZoneFQDN" with value <sc_fqdn>
	And the cluster config has nfsMountFQDN <cluster_fqdn>
	And the environment variable X_CSI_ISI_NFS_MOUNT_FQDN is set to <env_fqdn>
	And the volume context contains "NFSTransportSecurity" with value "mtls"
	And a capability with voltype "mount" access "single-writer"
	And get Node Publish Volume Request
	When I call NodePublishVolume
	Then the mount target should be <expected_fqdn>

	Examples:
	| sc_fqdn                      | cluster_fqdn                    | env_fqdn                       | expected_fqdn                  |
	| "sc.smartconnect.example.com" | "cluster.smartconnect.example.com" | "env.smartconnect.example.com" | "sc.smartconnect.example.com"  |
	| ""                            | "cluster.smartconnect.example.com" | "env.smartconnect.example.com" | "cluster.smartconnect.example.com" |
	| ""                            | ""                              | "env.smartconnect.example.com" | "env.smartconnect.example.com" |

@mtls
@v2.0.0
Scenario: Validate NFSTransportSecurity parameter values
	Given I create a volume with name "mtls-volume-validation"
	And I specify CreateVolume AccessZone "System"
	And I specify CreateVolume IsiPath "/ifs/data/csi"
	And I specify CreateVolume SmartConnectZoneFQDN "zone1.smartconnect.example.com"
	And I specify CreateVolume NFSTransportSecurity "invalid-value"
	When I call CreateVolume
	Then the error contains "Invalid NFSTransportSecurity value"

@mtls
@v2.0.0
Scenario: Accept valid NFSTransportSecurity values
	Given I create a volume with name "mtls-volume-valid-<value>"
	And I specify CreateVolume AccessZone "System"
	And I specify CreateVolume IsiPath "/ifs/data/csi"
	And I specify CreateVolume SmartConnectZoneFQDN "zone1.smartconnect.example.com"
	And I specify CreateVolume NFSTransportSecurity "<value>"
	When I call CreateVolume
	Then the error contains "none"
	And a valid CreateVolumeResponse is returned

	Examples:
	| value |
	| "none" |
	| "tls" |
	| "mtls" |
	| "" |

@mtls
@v2.0.0
Scenario: NodeGetInfo does not auto-apply TLS capability topology label
	# TLS capability topology label is NOT auto-applied by the driver.
	# Customers manually label mTLS-capable nodes and use StorageClass
	# allowedTopologies for scheduling. Mount-time validation in
	# NodePublishVolume remains as the fail-safe check.
	When I call NodeGetInfo
	Then the topology segments do not contain key "csi-isilon.dellemc.com/tls-capable"
