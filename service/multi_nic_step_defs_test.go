package service

/*
 Copyright (c) 2026 Dell Inc, or its subsidiaries.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// This file implements the godog step definitions backing
// service/features/multi_nic_network_selection.feature, which covers the
// Gherkin scenarios defined in ER-K8S-BR67074-001-multi-nic-nfs-selection
// (FR-1 through FR-4). FR-2.2 (GetNFSClientIPs / GetAllNFSClientIPs) and
// FR-5 (GetExportsCountAttachedToNodeIPs) are covered by Go table-driven
// tests closer to their implementation (csi-utils/csiutils_test.go and
// gopowerscale/exports_test.go respectively) since the mocking hooks they
// rely on are private to those packages.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	isimocks "github.com/Ecosystems/container-storage-modules/src/gopowerscale/mocks"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/cucumber/godog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// multiNICState holds the per-scenario state for the multi-NIC feature steps.
type multiNICState struct {
	// service options under test
	mode            string
	allowedNetworks []string

	// initializeServiceOpts results
	initErr  error
	initOpts Opts

	// node labels backing getIpsFromAllowedNetworks
	nodeLabels    map[string]string
	nodeLabelsErr error

	// getIpsFromAllowedNetworks results
	labelsFetchCount int
	multiIPs         []string
	multiErr         error

	// AddExportClientByIPWithZone
	matchingIPs []string
	failingIPs  map[string]bool
	allFail     bool
	addedIPs    []string
	addErr      error

	// getPowerScaleNodeID
	nodeIDResult string
	nodeIDErr    error

	// ControllerPublishVolume / ControllerUnpublishVolume invocation tracking
	publishInvokedLabels bool
}

func (m *multiNICState) reset() {
	*m = multiNICState{}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// --- FR-1.1: Mode Environment Variable ---

func (m *multiNICState) xCSIAllowedNetworksModeIsNotSet() error {
	return os.Unsetenv(constants.EnvAllowedNetworksMode)
}

func (m *multiNICState) xCSIAllowedNetworksModeIsSetTo(mode string) error {
	return os.Setenv(constants.EnvAllowedNetworksMode, mode)
}

func (m *multiNICState) xCSIAllowedNetworksIsSetTo(networks string) error {
	// X_CSI_ALLOWED_NETWORKS is parsed as a YAML array (e.g. "[10.0.0.0/24]"),
	// so wrap bare CIDR values for convenience in Gherkin scenarios.
	if !strings.HasPrefix(networks, "[") {
		networks = "[" + networks + "]"
	}
	return os.Setenv(constants.EnvAllowedNetworks, networks)
}

func (m *multiNICState) xCSIAllowedNetworksIsEmpty() error {
	return os.Setenv(constants.EnvAllowedNetworks, "")
}

func (m *multiNICState) theDriverInitializesServiceOptions() error {
	defer func() {
		os.Unsetenv(constants.EnvAllowedNetworksMode)
		os.Unsetenv(constants.EnvAllowedNetworks)
	}()

	s := &service{}
	m.initErr = s.initializeServiceOpts(context.Background())
	m.initOpts = s.opts
	return nil
}

func (m *multiNICState) theModeIsSetTo(mode string) error {
	if m.initOpts.allowedNetworksMode != mode {
		return fmt.Errorf("expected mode %q, got %q", mode, m.initOpts.allowedNetworksMode)
	}
	return nil
}

func (m *multiNICState) theDriverStartsSuccessfully() error {
	if m.initErr != nil {
		return fmt.Errorf("expected no error, got: %v", m.initErr)
	}
	return nil
}

func (m *multiNICState) theDriverFailsToStart() error {
	if m.initErr == nil {
		return errors.New("expected the driver to fail to start, but it succeeded")
	}
	return nil
}

func (m *multiNICState) theDriverReturnsAnErrorContaining(substr string) error {
	if m.initErr == nil {
		return fmt.Errorf("expected an error containing %q, got nil", substr)
	}
	if !strings.Contains(m.initErr.Error(), substr) {
		return fmt.Errorf("expected error to contain %q, got %q", substr, m.initErr.Error())
	}
	return nil
}

// --- FR-2.3: Controller-Side IP Filtering (getIpsFromAllowedNetworks) ---

func (m *multiNICState) nodeHasAzLabelsFor(_ string, ips string) error {
	if m.nodeLabels == nil {
		m.nodeLabels = map[string]string{}
	}
	for _, ip := range splitCSV(ips) {
		// Use a /24 label key; the exact prefix length is irrelevant since
		// getIpsFromAllowedNetworks only inspects the captured IP portion.
		key := fmt.Sprintf("%s/az-%s-24-%s", constants.PluginName, ip, ip)
		m.nodeLabels[key] = "true"
	}
	return nil
}

func (m *multiNICState) allowedNetworksIsSetTo(cidrs string) error {
	m.allowedNetworks = splitCSV(cidrs)
	return nil
}

func (m *multiNICState) theKubernetesAPIReturnsAnErrorForGetNodeLabelsWithName() error {
	m.nodeLabelsErr = errors.New("k8s API unavailable")
	return nil
}

func (m *multiNICState) getIpsFromAllowedNetworksIsCalledFor(nodeName string) error {
	original := getNodeLabelsWithNameFunc
	defer func() { getNodeLabelsWithNameFunc = original }()

	getNodeLabelsWithNameFunc = func(_ *service) func(string) (map[string]string, error) {
		return func(string) (map[string]string, error) {
			m.labelsFetchCount++
			return m.nodeLabels, m.nodeLabelsErr
		}
	}

	s := &service{opts: Opts{allowedNetworks: m.allowedNetworks}}
	nodeID := fmt.Sprintf("%s=#=#=%s.domain=#=#=127.0.0.1", nodeName, nodeName)
	m.multiIPs, m.multiErr = s.getIpsFromAllowedNetworks(context.Background(), nodeID)
	return nil
}

func (m *multiNICState) getIpsFromAllowedNetworksReturnsIPs(expected string) error {
	want := splitCSV(expected)
	if m.multiErr != nil {
		return fmt.Errorf("expected no error, got: %v", m.multiErr)
	}
	if !sameStringSet(m.multiIPs, want) {
		return fmt.Errorf("expected IPs %v, got %v", want, m.multiIPs)
	}
	return nil
}

func (m *multiNICState) getIpsFromAllowedNetworksReturnsAnErrorContaining(substr string) error {
	if m.multiErr == nil {
		return fmt.Errorf("expected an error containing %q, got nil", substr)
	}
	if !strings.Contains(m.multiErr.Error(), substr) {
		return fmt.Errorf("expected error to contain %q, got %q", substr, m.multiErr.Error())
	}
	return nil
}

func (m *multiNICState) ipIsNotIncludedInTheResult(ip string) error {
	for _, got := range m.multiIPs {
		if got == ip {
			return fmt.Errorf("expected %q to be excluded, but it was present in %v", ip, m.multiIPs)
		}
	}
	return nil
}

// --- FR-3.1: Add-All Export Semantics (AddExportClientByIPWithZone) ---

func (m *multiNICState) theNodeHasMatchingIPs(ips string) error {
	m.matchingIPs = splitCSV(ips)
	return nil
}

func (m *multiNICState) theNodeHasNoMatchingIPs() error {
	m.matchingIPs = nil
	return nil
}

func (m *multiNICState) ipFailsToBeAdded(ip string) error {
	if m.failingIPs == nil {
		m.failingIPs = map[string]bool{}
	}
	m.failingIPs[ip] = true
	return nil
}

func (m *multiNICState) allAddClientFuncCallsFail() error {
	m.allFail = true
	return nil
}

func (m *multiNICState) addExportClientByIPWithZoneIsCalledWithTheMatchingIPs() error {
	svc := &isiService{}
	addFunc := func(_ context.Context, _ int, _ string, ip string, _ bool) error {
		if m.allFail || m.failingIPs[ip] {
			return fmt.Errorf("simulated failure adding %s", ip)
		}
		m.addedIPs = append(m.addedIPs, ip)
		return nil
	}

	m.addErr = svc.AddExportClientByIPWithZone(context.Background(), "cluster1", 1, "System", "node1", m.matchingIPs, addFunc, nil, m.mode)
	return nil
}

func (m *multiNICState) allMatchingIPsAreAddedToTheExportClientList() error {
	if !sameStringSet(m.addedIPs, m.matchingIPs) {
		return fmt.Errorf("expected all IPs %v to be added, got %v", m.matchingIPs, m.addedIPs)
	}
	return nil
}

func (m *multiNICState) nOfMIPsAreAddedToTheExportClientList(n, total int) error {
	if len(m.addedIPs) != n {
		return fmt.Errorf("expected %d of %d IPs added, got %d (%v)", n, total, len(m.addedIPs), m.addedIPs)
	}
	return nil
}

func (m *multiNICState) addExportClientByIPWithZoneReturnsSuccess() error {
	if m.addErr != nil {
		return fmt.Errorf("expected success, got error: %v", m.addErr)
	}
	return nil
}

func (m *multiNICState) addExportClientByIPWithZoneReturnsAnErrorContaining(substr string) error {
	if m.addErr == nil {
		return fmt.Errorf("expected an error containing %q, got nil", substr)
	}
	if !strings.Contains(m.addErr.Error(), substr) {
		return fmt.Errorf("expected error to contain %q, got %q", substr, m.addErr.Error())
	}
	return nil
}

// --- FR-3.3 / FR-3.4: ControllerPublishVolume / ControllerUnpublishVolume multi-mode paths ---

func (m *multiNICState) allowedNetworksModeIsSetTo(mode string) error {
	m.mode = mode
	return nil
}

func (m *multiNICState) controllerPublishVolumeIsCalledForNode(nodeName string) error {
	original := getNodeLabelsWithNameFunc
	defer func() { getNodeLabelsWithNameFunc = original }()

	getNodeLabelsWithNameFunc = func(_ *service) func(string) (map[string]string, error) {
		return func(string) (map[string]string, error) {
			m.labelsFetchCount++
			return m.nodeLabels, m.nodeLabelsErr
		}
	}

	s := &service{
		opts: Opts{
			allowedNetworksMode: m.mode,
			allowedNetworks:     m.allowedNetworks,
		},
	}

	nodeID := fmt.Sprintf("%s=#=#=%s.domain=#=#=10.0.0.1", nodeName, nodeName)
	// VolumeId is intentionally empty: the multi-NIC IP resolution block in
	// ControllerPublishVolume runs before volume ID validation, so we can
	// observe the branching decision (via labelsFetchCount) without needing
	// to mock the full PowerScale REST API chain.
	req := &csi.ControllerPublishVolumeRequest{
		VolumeId: "",
		NodeId:   nodeID,
	}
	_, _ = s.ControllerPublishVolume(context.Background(), req)
	m.publishInvokedLabels = m.labelsFetchCount > 0
	return nil
}

func (m *multiNICState) getIpsFromAllowedNetworksIsInvokedDuringPublish() error {
	if !m.publishInvokedLabels {
		return errors.New("expected getIpsFromAllowedNetworks to be invoked during publish, but it was not")
	}
	return nil
}

func (m *multiNICState) getIpsFromAllowedNetworksIsNotInvokedDuringPublish() error {
	if m.publishInvokedLabels {
		return errors.New("expected getIpsFromAllowedNetworks NOT to be invoked during publish, but it was")
	}
	return nil
}

func (m *multiNICState) controllerUnpublishVolumeIsCalledForNode(nodeName string) error {
	original := getNodeLabelsWithNameFunc
	defer func() { getNodeLabelsWithNameFunc = original }()

	getNodeLabelsWithNameFunc = func(_ *service) func(string) (map[string]string, error) {
		return func(string) (map[string]string, error) {
			m.labelsFetchCount++
			return m.nodeLabels, m.nodeLabelsErr
		}
	}

	mockClient := &isimocks.Client{}
	// Generic catch-all mocks: these scenarios only assert on the
	// mode-branching decision (whether node labels were fetched), so the
	// downstream export removal is allowed to no-op/fail safely.
	mockClient.On("Get", anyArgs[0:6]...).Return(nil)
	mockClient.On("Put", anyArgs...).Return(nil)

	ignoreUnresolvableHosts := false
	isiConfig := &IsilonClusterConfig{
		ClusterName:             "system",
		IgnoreUnresolvableHosts: &ignoreUnresolvableHosts,
		isiSvc: &isiService{
			client: &isi.Client{API: mockClient},
		},
	}

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "multinicpv"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					VolumeAttributes: map[string]string{},
				},
			},
		},
	}

	isiClusters := &sync.Map{}
	isiClusters.Store("system", isiConfig)

	s := &service{
		k8sclient:             fake.NewSimpleClientset(pv),
		defaultIsiClusterName: "system",
		isiClusters:           isiClusters,
		opts: Opts{
			allowedNetworksMode: m.mode,
			allowedNetworks:     m.allowedNetworks,
		},
	}

	nodeID := fmt.Sprintf("%s=#=#=%s.domain=#=#=10.0.0.1", nodeName, nodeName)
	req := &csi.ControllerUnpublishVolumeRequest{
		VolumeId: "multinicpv=_=_=19=_=_=csi0zone",
		NodeId:   nodeID,
	}
	_, _ = s.ControllerUnpublishVolume(context.Background(), req)
	m.publishInvokedLabels = m.labelsFetchCount > 0
	return nil
}

func (m *multiNICState) getIpsFromAllowedNetworksIsInvokedDuringUnpublish() error {
	return m.getIpsFromAllowedNetworksIsInvokedDuringPublish()
}

func (m *multiNICState) getIpsFromAllowedNetworksIsNotInvokedDuringUnpublish() error {
	return m.getIpsFromAllowedNetworksIsNotInvokedDuringPublish()
}

// --- FR-4.1: Stable Node ID in multi mode (getPowerScaleNodeID) ---

func (m *multiNICState) xCSINodeIPIsSetTo(ip string) error {
	m.nodeIDResult = ip // reuse field temporarily to stash the mgmt IP
	return nil
}

func (m *multiNICState) xCSINodeIPIsNotSet() error {
	m.nodeIDResult = ""
	return nil
}

func (m *multiNICState) getPowerScaleNodeIDIsCalled() error {
	mgmtIP := m.nodeIDResult
	s := &service{
		nodeIP: mgmtIP,
		nodeID: "worker-1",
		opts: Opts{
			allowedNetworks:     m.allowedNetworks,
			allowedNetworksMode: m.mode,
		},
	}
	m.nodeIDResult, m.nodeIDErr = s.getPowerScaleNodeID(context.Background())
	return nil
}

func (m *multiNICState) theNodeIDContains(substr string) error {
	if m.nodeIDErr != nil {
		return fmt.Errorf("expected no error, got: %v", m.nodeIDErr)
	}
	if !strings.Contains(m.nodeIDResult, substr) {
		return fmt.Errorf("expected Node ID to contain %q, got %q", substr, m.nodeIDResult)
	}
	return nil
}

func (m *multiNICState) theNodeIDDoesNotContain(substr string) error {
	if m.nodeIDErr != nil {
		return fmt.Errorf("expected no error, got: %v", m.nodeIDErr)
	}
	if strings.Contains(m.nodeIDResult, substr) {
		return fmt.Errorf("expected Node ID NOT to contain %q, got %q", substr, m.nodeIDResult)
	}
	return nil
}

func (m *multiNICState) getPowerScaleNodeIDReturnsAnError() error {
	if m.nodeIDErr == nil {
		return errors.New("expected an error, got nil")
	}
	return nil
}

// sameStringSet compares two string slices ignoring order.
func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := map[string]int{}
	for _, v := range a {
		counts[v]++
	}
	for _, v := range b {
		counts[v]--
	}
	for _, c := range counts {
		if c != 0 {
			return false
		}
	}
	return true
}

// RegisterMultiNICSteps wires up the step definitions for
// multi_nic_network_selection.feature into the shared godog ScenarioContext.
func RegisterMultiNICSteps(s *godog.ScenarioContext) {
	m := &multiNICState{}
	s.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		m.reset()
		return ctx, nil
	})

	// FR-1.1
	s.Step(`^X_CSI_ALLOWED_NETWORKS_MODE is not set$`, m.xCSIAllowedNetworksModeIsNotSet)
	s.Step(`^X_CSI_ALLOWED_NETWORKS_MODE is set to "([^"]*)"$`, m.xCSIAllowedNetworksModeIsSetTo)
	s.Step(`^X_CSI_ALLOWED_NETWORKS is set to "([^"]*)"$`, m.xCSIAllowedNetworksIsSetTo)
	s.Step(`^X_CSI_ALLOWED_NETWORKS is empty$`, m.xCSIAllowedNetworksIsEmpty)
	s.Step(`^the driver initializes service options$`, m.theDriverInitializesServiceOptions)
	s.Step(`^the mode is set to "([^"]*)"$`, m.theModeIsSetTo)
	s.Step(`^the driver starts successfully$`, m.theDriverStartsSuccessfully)
	s.Step(`^the driver fails to start$`, m.theDriverFailsToStart)
	s.Step(`^the driver returns an error containing "([^"]*)"$`, m.theDriverReturnsAnErrorContaining)

	// FR-2.3
	s.Step(`^Node "([^"]*)" has az-labels for "([^"]*)"$`, m.nodeHasAzLabelsFor)
	s.Step(`^allowedNetworks is set to "([^"]*)"$`, m.allowedNetworksIsSetTo)
	s.Step(`^the Kubernetes API returns an error for GetNodeLabelsWithName$`, m.theKubernetesAPIReturnsAnErrorForGetNodeLabelsWithName)
	s.Step(`^getIpsFromAllowedNetworks is called for "([^"]*)"$`, m.getIpsFromAllowedNetworksIsCalledFor)
	s.Step(`^getIpsFromAllowedNetworks returns IPs "([^"]*)"$`, m.getIpsFromAllowedNetworksReturnsIPs)
	s.Step(`^getIpsFromAllowedNetworks returns an error containing "([^"]*)"$`, m.getIpsFromAllowedNetworksReturnsAnErrorContaining)
	s.Step(`^"([^"]*)" is not included in the result$`, m.ipIsNotIncludedInTheResult)

	// FR-3.1
	s.Step(`^the node has matching IPs "([^"]*)"$`, m.theNodeHasMatchingIPs)
	s.Step(`^the node has no matching IPs$`, m.theNodeHasNoMatchingIPs)
	s.Step(`^"([^"]*)" fails to be added$`, m.ipFailsToBeAdded)
	s.Step(`^all addClientFunc calls fail$`, m.allAddClientFuncCallsFail)
	s.Step(`^AddExportClientByIPWithZone is called with the matching IPs$`, m.addExportClientByIPWithZoneIsCalledWithTheMatchingIPs)
	s.Step(`^all matching IPs are added to the export client list$`, m.allMatchingIPsAreAddedToTheExportClientList)
	s.Step(`^(\d+) of (\d+) IPs are added to the export client list$`, m.nOfMIPsAreAddedToTheExportClientList)
	s.Step(`^AddExportClientByIPWithZone returns success$`, m.addExportClientByIPWithZoneReturnsSuccess)
	s.Step(`^AddExportClientByIPWithZone returns an error containing "([^"]*)"$`, m.addExportClientByIPWithZoneReturnsAnErrorContaining)

	// FR-3.3 / FR-3.4
	s.Step(`^allowedNetworksMode is set to "([^"]*)"$`, m.allowedNetworksModeIsSetTo)
	s.Step(`^ControllerPublishVolume is called for node "([^"]*)"$`, m.controllerPublishVolumeIsCalledForNode)
	s.Step(`^getIpsFromAllowedNetworks is invoked during publish$`, m.getIpsFromAllowedNetworksIsInvokedDuringPublish)
	s.Step(`^getIpsFromAllowedNetworks is not invoked during publish$`, m.getIpsFromAllowedNetworksIsNotInvokedDuringPublish)
	s.Step(`^ControllerUnpublishVolume is called for node "([^"]*)"$`, m.controllerUnpublishVolumeIsCalledForNode)
	s.Step(`^getIpsFromAllowedNetworks is invoked during unpublish$`, m.getIpsFromAllowedNetworksIsInvokedDuringUnpublish)
	s.Step(`^getIpsFromAllowedNetworks is not invoked during unpublish$`, m.getIpsFromAllowedNetworksIsNotInvokedDuringUnpublish)

	// FR-4.1
	s.Step(`^X_CSI_NODE_IP is set to "([^"]*)"$`, m.xCSINodeIPIsSetTo)
	s.Step(`^X_CSI_NODE_IP is not set$`, m.xCSINodeIPIsNotSet)
	s.Step(`^getPowerScaleNodeID is called$`, m.getPowerScaleNodeIDIsCalled)
	s.Step(`^the Node ID contains "([^"]*)"$`, m.theNodeIDContains)
	s.Step(`^the Node ID does not contain "([^"]*)"$`, m.theNodeIDDoesNotContain)
	s.Step(`^getPowerScaleNodeID returns an error$`, m.getPowerScaleNodeIDReturnsAnError)
}
