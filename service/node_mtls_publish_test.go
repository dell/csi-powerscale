// Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package service

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	mtlsTestFQDN         = "zone1.smartconnect.example.com"
	mtlsTestPVCName      = "test-pvc"
	mtlsTestPVCNamespace = "test-namespace"
)

// newMTLSNodeService builds a node service with a single cluster whose isiSvc is
// already initialized so that NodePublishVolume skips probing.
func newMTLSNodeService() *service {
	isiClusters := new(sync.Map)
	isiClusters.Store("TestCluster", &IsilonClusterConfig{
		ClusterName:   "TestCluster",
		Endpoint:      "http://testendpoint",
		EndpointPort:  "8080",
		MountEndpoint: "http://mountendpoint",
		IsiPath:       "/ifs/data",
		isiSvc: &isiService{
			endpoint: "http://testendpoint:8080",
			client:   &isi.Client{},
		},
	})

	return &service{
		defaultIsiClusterName: "TestCluster",
		isiClusters:           isiClusters,
		nodeID:                "TestNodeID",
		nodeIP:                "1.2.3.4",
		k8sclient:             fake.NewSimpleClientset(),
		opts:                  Opts{AccessZone: "System"},
	}
}

// newMTLSPublishRequest builds a NodePublishVolume request for an mTLS volume.
func newMTLSPublishRequest(azServiceIP, fqdn string, mountFlags []string) *csi.NodePublishVolumeRequest {
	return &csi.NodePublishVolumeRequest{
		VolumeId: "123",
		VolumeCapability: &csi.VolumeCapability{
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{MountFlags: mountFlags},
			},
		},
		VolumeContext: map[string]string{
			"Path":                              "/ifs/data/volname",
			"Name":                              "volname",
			"AccessZone":                        "System",
			AzServiceIPParam:                    azServiceIP,
			constants.SmartConnectZoneFQDNParam: fqdn,
			constants.NFSTransportSecurityParam: constants.NFSTransportSecurityMTLS,
			csiPersistentVolumeClaimName:        mtlsTestPVCName,
			csiPersistentVolumeClaimNamespace:   mtlsTestPVCNamespace,
		},
		TargetPath: "/tmp/mtls-target",
	}
}

// setTLSCapable makes the node appear kTLS capable (kernel module and tlshd present).
func setTLSCapable(t *testing.T, capable bool) {
	originalStat := statFunc
	originalLookPath := lookPathFunc
	t.Cleanup(func() {
		statFunc = originalStat
		lookPathFunc = originalLookPath
	})

	if capable {
		statFunc = func(_ string) (os.FileInfo, error) { return nil, nil }
		lookPathFunc = func(_ string) (string, error) { return "/usr/sbin/tlshd", nil }
		return
	}
	statFunc = func(_ string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	lookPathFunc = func(_ string) (string, error) { return "", errors.New("not found") }
}

// setPublishVolumeResult stubs the NFS mount so no real mount is attempted.
func setPublishVolumeResult(t *testing.T, err error) {
	originalPublish := publishVolumeFunc
	originalGetVolByName := getVolByNameFunc
	t.Cleanup(func() {
		publishVolumeFunc = originalPublish
		getVolByNameFunc = originalGetVolByName
	})

	publishVolumeFunc = func(_ context.Context, _ *csi.NodePublishVolumeRequest, _ string) error { return err }
	getVolByNameFunc = func(_ *service, _ context.Context, _, _ string, _ *IsilonClusterConfig) (isi.Volume, error) {
		return nil, nil
	}
}

// TestNodePublishVolume_MTLSNodeNotTLSCapable verifies the fail-closed behavior
// when the node lacks kernel TLS support or the tlshd daemon.
func TestNodePublishVolume_MTLSNodeNotTLSCapable(t *testing.T) {
	setTLSCapable(t, false)
	setPublishVolumeResult(t, nil)

	svc := newMTLSNodeService()
	resp, err := svc.NodePublishVolume(context.Background(), newMTLSPublishRequest("10.0.0.1", mtlsTestFQDN, nil))

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestNodePublishVolume_MTLSConflictingMountOption verifies that mount options
// conflicting with mTLS are rejected.
func TestNodePublishVolume_MTLSConflictingMountOption(t *testing.T) {
	setTLSCapable(t, true)
	setPublishVolumeResult(t, nil)

	svc := newMTLSNodeService()
	req := newMTLSPublishRequest("10.0.0.1", mtlsTestFQDN, []string{"xprtsec=none"})
	resp, err := svc.NodePublishVolume(context.Background(), req)

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "xprtsec=none")
}

// TestNodePublishVolume_MTLSSuccess verifies a successful mTLS publish when the
// node is TLS capable and the mount target is an FQDN.
func TestNodePublishVolume_MTLSSuccess(t *testing.T) {
	setTLSCapable(t, true)
	setPublishVolumeResult(t, nil)

	svc := newMTLSNodeService()
	req := newMTLSPublishRequest("10.0.0.1", mtlsTestFQDN, []string{"vers=4.1"})
	resp, err := svc.NodePublishVolume(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestNodePublishVolume_DirectMountReceivesResolvedFQDN(t *testing.T) {
	setTLSCapable(t, true)
	originalPublish := publishVolumeFunc
	originalGetVolByName := getVolByNameFunc
	t.Cleanup(func() {
		publishVolumeFunc = originalPublish
		getVolByNameFunc = originalGetVolByName
	})

	svc := newMTLSNodeService()
	configValue, ok := svc.isiClusters.Load("TestCluster")
	require.True(t, ok)
	config := configValue.(*IsilonClusterConfig)
	config.NFSMountFQDN = mtlsTestFQDN

	var publishedContext map[string]string
	publishVolumeFunc = func(_ context.Context, req *csi.NodePublishVolumeRequest, _ string) error {
		publishedContext = req.GetVolumeContext()
		return nil
	}
	getVolByNameFunc = func(_ *service, _ context.Context, _, _ string, _ *IsilonClusterConfig) (isi.Volume, error) {
		return nil, nil
	}

	req := newMTLSPublishRequest("10.0.0.1", "", []string{"vers=4.1"})
	resp, err := svc.NodePublishVolume(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, mtlsTestFQDN, publishedContext[constants.SmartConnectZoneFQDNParam])
}

func TestNodePublishVolume_DoesNotInheritTransportSecurity(t *testing.T) {
	setTLSCapable(t, false)
	setPublishVolumeResult(t, nil)
	t.Setenv("X_CSI_ISI_NFS_TRANSPORT_SECURITY", constants.NFSTransportSecurityMTLS)

	svc := newMTLSNodeService()
	req := newMTLSPublishRequest("10.0.0.1", "", nil)
	delete(req.VolumeContext, constants.NFSTransportSecurityParam)
	resp, err := svc.NodePublishVolume(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestNodePublishVolume_MTLSMountTLSError verifies TLS mount failures are
// classified and surfaced as Internal errors.
func TestNodePublishVolume_MTLSMountTLSError(t *testing.T) {
	setTLSCapable(t, true)
	setPublishVolumeResult(t, errors.New("x509: certificate signed by unknown authority"))

	svc := newMTLSNodeService()
	resp, err := svc.NodePublishVolume(context.Background(), newMTLSPublishRequest("10.0.0.1", mtlsTestFQDN, nil))

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Contains(t, err.Error(), "mTLS mount failed")
}

// TestNodePublishVolume_MTLSMountTimeout verifies TLS handshake timeouts are
// reported as DeadlineExceeded.
func TestNodePublishVolume_MTLSMountTimeout(t *testing.T) {
	setTLSCapable(t, true)
	setPublishVolumeResult(t, &MountError{Operation: "mount", Err: errors.New("i/o timeout"), Timeout: true})

	svc := newMTLSNodeService()
	resp, err := svc.NodePublishVolume(context.Background(), newMTLSPublishRequest("10.0.0.1", mtlsTestFQDN, nil))

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
}
