# Integration test for CSI Isilon driver

This test is run on a Kubernetes node, this will make real calls to the
Isilon.

There are four scripts to set environment variables, env_Quota_Enabled.sh, env_Quota_notEnabled.sh, env_nodeIP1.sh, env_nodeIP2.sh. All files should be populated with values for Isilon. env_Quota_Enabled.sh is used for Quota enabled, env_Quota_notEnabled.sh is for Quota not enabled. The file env_nodeIP1.sh and file env_nodeIP2.sh added to mock different node IPs for NodeStageVolume with different accessModes, the corresponding feature file is mock_different_nodeIPs.feature. The file main_integration.feature is used to test most scenarios.

To launch the integration test, just run run.sh. Which environment script, feature file and tag needed can be specified in this script.