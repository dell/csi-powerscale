Feature: Multi-NIC NFS network selection (ER-K8S-BR67074-001-multi-nic-nfs-selection)
    As an operator of the PowerScale CSI driver
    I want to select single or multi NFS client IPs per node
    So that AI/HPC workloads can use multiple network interfaces per node

  # FR-1.1: Mode Environment Variable
  Scenario: Default mode when unset (AC-002)
    Given X_CSI_ALLOWED_NETWORKS_MODE is not set
    When the driver initializes service options
    Then the mode is set to "single"
    And the driver starts successfully

  Scenario: Valid single mode (AC-001)
    Given X_CSI_ALLOWED_NETWORKS_MODE is set to "single"
    When the driver initializes service options
    Then the mode is set to "single"
    And the driver starts successfully

  Scenario: Valid multi mode with allowedNetworks (AC-001)
    Given X_CSI_ALLOWED_NETWORKS_MODE is set to "multi"
    And X_CSI_ALLOWED_NETWORKS is set to "10.0.0.0/24"
    When the driver initializes service options
    Then the mode is set to "multi"
    And the driver starts successfully

  Scenario: Invalid mode value (AC-006)
    Given X_CSI_ALLOWED_NETWORKS_MODE is set to "round-robin"
    When the driver initializes service options
    Then the driver returns an error containing "invalid"
    And the driver fails to start

  Scenario: Multi mode without allowedNetworks (NEW-Q2)
    Given X_CSI_ALLOWED_NETWORKS_MODE is set to "multi"
    And X_CSI_ALLOWED_NETWORKS is empty
    When the driver initializes service options
    Then the driver returns an error containing "requires"
    And the driver fails to start

  # FR-2.2: GetNFSClientIPs (GetAllNFSClientIPs) is verified via Go
  # table-driven tests in csi-utils/csiutils_test.go::TestGetAllNFSClientIPs
  # (Multiple match, No match, Multiple CIDRs, Warning threshold exceeded,
  # Exactly at warning threshold, Loopback and IPv6 excluded), since the
  # interface-mocking hooks it depends on are private to the csiutils package.

  # FR-2.3: Controller-Side IP Filtering (getIpsFromAllowedNetworks)
  Scenario: Node labels match CIDRs (AC-003, AC-005)
    Given Node "worker-1" has az-labels for "10.0.0.1,10.0.0.2"
    And allowedNetworks is set to "10.0.0.0/24"
    When getIpsFromAllowedNetworks is called for "worker-1"
    Then getIpsFromAllowedNetworks returns IPs "10.0.0.1,10.0.0.2"

  Scenario: Partial match across multiple CIDRs (AC-004)
    Given Node "worker-1" has az-labels for "10.0.0.1,192.168.1.5,172.16.0.9"
    And allowedNetworks is set to "10.0.0.0/24,192.168.1.0/24"
    When getIpsFromAllowedNetworks is called for "worker-1"
    Then getIpsFromAllowedNetworks returns IPs "10.0.0.1,192.168.1.5"
    And "172.16.0.9" is not included in the result

  Scenario: No matching labels
    Given Node "worker-1" has az-labels for "172.16.0.9"
    And allowedNetworks is set to "10.0.0.0/24"
    When getIpsFromAllowedNetworks is called for "worker-1"
    Then getIpsFromAllowedNetworks returns an error containing "no IPs in node labels match"

  Scenario: Kubernetes API error
    Given the Kubernetes API returns an error for GetNodeLabelsWithName
    And allowedNetworks is set to "10.0.0.0/24"
    When getIpsFromAllowedNetworks is called for "worker-1"
    Then getIpsFromAllowedNetworks returns an error containing "k8s API unavailable"

  # FR-3.1: Add-All Export Semantics (AddExportClientByIPWithZone)
  Scenario: All IPs added successfully (AC-003)
    Given the node has matching IPs "10.0.0.1,10.0.0.2"
    And allowedNetworksMode is set to "multi"
    When AddExportClientByIPWithZone is called with the matching IPs
    Then all matching IPs are added to the export client list
    And AddExportClientByIPWithZone returns success

  Scenario: Partial IP addition failure
    Given the node has matching IPs "10.0.0.1,10.0.0.2,10.0.0.3"
    And allowedNetworksMode is set to "multi"
    And "10.0.0.1" fails to be added
    When AddExportClientByIPWithZone is called with the matching IPs
    Then 2 of 3 IPs are added to the export client list
    And AddExportClientByIPWithZone returns success

  Scenario: All IPs fail to be added
    Given the node has matching IPs "10.0.0.1,10.0.0.2"
    And allowedNetworksMode is set to "multi"
    And all addClientFunc calls fail
    When AddExportClientByIPWithZone is called with the matching IPs
    Then AddExportClientByIPWithZone returns an error containing "failed to add any of clients"

  Scenario: Empty IP list
    Given the node has no matching IPs
    And allowedNetworksMode is set to "multi"
    When AddExportClientByIPWithZone is called with the matching IPs
    Then AddExportClientByIPWithZone returns an error containing "failed to add any of clients"

  # FR-3.3: ControllerPublishVolume Multi-Mode Path
  Scenario: Multi-mode publish adds all IPs (AC-003)
    Given allowedNetworksMode is set to "multi"
    And allowedNetworks is set to "10.0.0.0/24"
    And Node "worker-1" has az-labels for "10.0.0.1,10.0.0.2"
    When ControllerPublishVolume is called for node "worker-1"
    Then getIpsFromAllowedNetworks is invoked during publish

  Scenario: Single-mode publish uses existing behavior (AC-002)
    Given allowedNetworksMode is set to "single"
    And allowedNetworks is set to "10.0.0.0/24"
    And Node "worker-1" has az-labels for "10.0.0.1,10.0.0.2"
    When ControllerPublishVolume is called for node "worker-1"
    Then getIpsFromAllowedNetworks is not invoked during publish

  # FR-3.4: ControllerUnpublishVolume Multi-Mode Path
  Scenario: Multi-mode unpublish removes all IPs
    Given allowedNetworksMode is set to "multi"
    And allowedNetworks is set to "10.0.0.0/24"
    And Node "worker-1" has az-labels for "10.0.0.1,10.0.0.2"
    When ControllerUnpublishVolume is called for node "worker-1"
    Then getIpsFromAllowedNetworks is invoked during unpublish

  Scenario: Single-mode unpublish uses existing behavior
    Given allowedNetworksMode is set to "single"
    And allowedNetworks is set to "10.0.0.0/24"
    And Node "worker-1" has az-labels for "10.0.0.1,10.0.0.2"
    When ControllerUnpublishVolume is called for node "worker-1"
    Then getIpsFromAllowedNetworks is not invoked during unpublish

  # FR-4.1: Stable Node ID in multi mode (getPowerScaleNodeID)
  Scenario: Multi-mode uses management IP for Node ID
    Given allowedNetworksMode is set to "multi"
    And X_CSI_NODE_IP is set to "192.168.1.100"
    And allowedNetworks is set to "10.0.0.0/24"
    When getPowerScaleNodeID is called
    Then the Node ID contains "192.168.1.100"

  Scenario: Single-mode uses existing behavior for Node ID
    Given allowedNetworksMode is set to "single"
    And allowedNetworks is set to "127.0.0.0/8"
    And X_CSI_NODE_IP is set to "192.168.1.100"
    When getPowerScaleNodeID is called
    Then the Node ID does not contain "192.168.1.100"
    And the Node ID contains "127.0.0.1"

  Scenario: Multi-mode with GetCSINodeIP failure
    Given allowedNetworksMode is set to "multi"
    And X_CSI_NODE_IP is not set
    When getPowerScaleNodeID is called
    Then getPowerScaleNodeID returns an error
