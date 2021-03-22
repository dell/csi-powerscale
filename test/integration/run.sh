#!/bin/sh
# This will run coverage analysis using the integration testing.
# The env.sh must point to a valid Isilon deployment

function runTest() {
    rm -f unix_sock
    source ${1}
    go test -v -coverprofile=c.linux.out -timeout 30m -coverpkg=github.com/dell/csi-isilon/service *test.go -args ${2} ${3}
    wait
}

echo "Quota is enabled"
runTest ./env_Quota_Enabled.sh ./features/main_integration.feature "v1.0"
mv ./Powerscale_integration_test_results.xml Powerscale_integration_test_results_QuotaEnabled.xml

echo "Quota is not enabled"
runTest ./env_Quota_notEnabled.sh ./features/integration.feature "v1.0"
mv ./Powerscale_integration_test_results.xml Powerscale_integration_test_results_QuotaNotEnabled.xml

echo "test accessModes with nodeIP1"
runTest ./env_nodeIP1.sh ./features/mock_different_nodeIPs.feature "first_run"
mv ./Powerscale_integration_test_results.xml Powerscale_integration_test_results_AccessModeIP1.xml

echo "test accessModes with nodeIP2"
runTest ./env_nodeIP2.sh ./features/mock_different_nodeIPs.feature "second_run"
mv ./Powerscale_integration_test_results.xml Powerscale_integration_test_results_AccessModesIP2.xml

echo "Custom Topology is enabled"
runTest ./env_Custom_Topology_Enabled.sh ./features/integration.feature "v1.0"
mv ./Powerscale_integration_test_results.xml Powerscale_integration_test_results_CustomTopology.xml
