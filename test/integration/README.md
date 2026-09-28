# Integration test for CSI PowerScale driver

This test is run on a Kubernetes node, this will make real calls to the
PowerScale.

## Environment Scripts

There are four scripts to set environment variables:

| Script | Purpose |
|--------|---------|
| `env_Quota_Enabled.sh` | Quota-enabled array testing |
| `env_Quota_notEnabled.sh` | Quota-disabled array testing |
| `env_nodeIP1.sh` | Mock different node IPs for NodeStageVolume with different accessModes |
| `env_nodeIP2.sh` | Mock different node IPs for NodeStageVolume with different accessModes |

## Feature Files

| Feature File | Purpose |
|--------------|---------|
| `main_integration.feature` | Most integration scenarios |
| `mock_different_nodeIPs.feature` | Multi-node access mode testing |
| `mtls_basic.feature` | Basic mTLS transport security testing (ER-K8S-BR99506-001-powerscale-mtls-nfs-transport) |

## Configuration

There is a config file to set secrets details, either this file can be updated or the path for the same can be updated in all the environment files, under the variable name 'X_CSI_ISI_CONFIG_PATH'

## Running Tests

To launch the integration test, just run `make integration-test` from csi-powerscale root directory. Whichever environment script, feature file and tag needed can be specified in this script.
