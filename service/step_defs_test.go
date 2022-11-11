package service

/*
 Copyright (c) 2019 Dell Inc, or its subsidiaries.

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
import (
	context2 "context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dell/csi-isilon/service/mock/k8s"

	"github.com/dell/csi-isilon/common/utils"

	"github.com/dell/csi-isilon/common/constants"
	csiext "github.com/dell/dell-csi-extensions/replication"
	"google.golang.org/grpc"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/cucumber/godog"
	commonext "github.com/dell/dell-csi-extensions/common"
	podmon "github.com/dell/dell-csi-extensions/podmon"
	"github.com/dell/gocsi"
	"github.com/dell/gofsutil"
	"golang.org/x/net/context"
	"google.golang.org/grpc/metadata"
)

type feature struct {
	nGoRoutines                             int
	server                                  *httptest.Server
	service                                 *service
	err                                     error // return from the preceeding call
	getPluginInfoResponse                   *csi.GetPluginInfoResponse
	getPluginCapabilitiesResponse           *csi.GetPluginCapabilitiesResponse
	probeResponse                           *csi.ProbeResponse
	createVolumeResponse                    *csi.CreateVolumeResponse
	publishVolumeResponse                   *csi.ControllerPublishVolumeResponse
	unpublishVolumeResponse                 *csi.ControllerUnpublishVolumeResponse
	nodeGetInfoResponse                     *csi.NodeGetInfoResponse
	nodeGetCapabilitiesResponse             *csi.NodeGetCapabilitiesResponse
	deleteVolumeResponse                    *csi.DeleteVolumeResponse
	getCapacityResponse                     *csi.GetCapacityResponse
	controllerGetCapabilitiesResponse       *csi.ControllerGetCapabilitiesResponse
	validateVolumeCapabilitiesResponse      *csi.ValidateVolumeCapabilitiesResponse
	createSnapshotResponse                  *csi.CreateSnapshotResponse
	createVolumeRequest                     *csi.CreateVolumeRequest
	createRemoteVolumeRequest               *csiext.CreateRemoteVolumeRequest
	createRemoteVolumeResponse              *csiext.CreateRemoteVolumeResponse
	createStorageProtectionGroupRequest     *csiext.CreateStorageProtectionGroupRequest
	createStorageProtectionGroupResponse    *csiext.CreateStorageProtectionGroupResponse
	deleteStorageProtectionGroupRequest     *csiext.DeleteStorageProtectionGroupRequest
	deleteStorageProtectionGroupResponse    *csiext.DeleteStorageProtectionGroupResponse
	publishVolumeRequest                    *csi.ControllerPublishVolumeRequest
	unpublishVolumeRequest                  *csi.ControllerUnpublishVolumeRequest
	deleteVolumeRequest                     *csi.DeleteVolumeRequest
	controllerExpandVolumeRequest           *csi.ControllerExpandVolumeRequest
	controllerExpandVolumeResponse          *csi.ControllerExpandVolumeResponse
	controllerGetVolumeRequest              *csi.ControllerGetVolumeRequest
	controllerGetVolumeResponse             *csi.ControllerGetVolumeResponse
	listVolumesRequest                      *csi.ListVolumesRequest
	listVolumesResponse                     *csi.ListVolumesResponse
	listSnapshotsRequest                    *csi.ListSnapshotsRequest
	listSnapshotsResponse                   *csi.ListSnapshotsResponse
	listedVolumeIDs                         map[string]bool
	listVolumesNextTokenCache               string
	wrongCapacity, wrongStoragePool         bool
	accessZone                              string
	capability                              *csi.VolumeCapability
	capabilities                            []*csi.VolumeCapability
	nodeStageVolumeRequest                  *csi.NodeStageVolumeRequest
	nodeStageVolumeResponse                 *csi.NodeStageVolumeResponse
	nodeUnstageVolumeRequest                *csi.NodeUnstageVolumeRequest
	nodeUnstageVolumeResponse               *csi.NodeUnstageVolumeResponse
	nodePublishVolumeRequest                *csi.NodePublishVolumeRequest
	nodeUnpublishVolumeRequest              *csi.NodeUnpublishVolumeRequest
	nodeUnpublishVolumeResponse             *csi.NodeUnpublishVolumeResponse
	nodeGetVolumeStatsRequest               *csi.NodeGetVolumeStatsRequest
	nodeGetVolumeStatsResponse              *csi.NodeGetVolumeStatsResponse
	deleteSnapshotRequest                   *csi.DeleteSnapshotRequest
	deleteSnapshotResponse                  *csi.DeleteSnapshotResponse
	createSnapshotRequest                   *csi.CreateSnapshotRequest
	getStorageProtectionGroupStatusResponse *csiext.GetStorageProtectionGroupStatusResponse
	getStorageProtectionGroupStatusRequest  *csiext.GetStorageProtectionGroupStatusRequest
	executeActionRequest                    *csiext.ExecuteActionRequest
	executeActionResponse                   *csiext.ExecuteActionResponse
	getReplicationCapabilityRequest         *csiext.GetReplicationCapabilityRequest
	getReplicationCapabilityResponse        *csiext.GetReplicationCapabilityResponse
	validateVolumeHostConnectivityResp      *podmon.ValidateVolumeHostConnectivityResponse
	ProbeControllerRequest                  *commonext.ProbeControllerRequest
	ProbeControllerResponse                 *commonext.ProbeControllerResponse
	volumeIDList                            []string
	snapshotIDList                          []string
	groupIDList                             []string
	snapshotIndex                           int
	rootClientEnabled                       string
	createVolumeRequestTest                 *csi.CreateVolumeRequest
	createVolumeResponseTest                *csi.CreateVolumeResponse
}

var inducedErrors struct {
	badVolumeIdentifier  bool
	invalidVolumeID      bool
	noVolumeID           bool
	differentVolumeID    bool
	noNodeName           bool
	noNodeID             bool
	omitVolumeCapability bool
	omitAccessMode       bool
	useAccessTypeMount   bool
	noIsiService         bool
	autoProbeNotEnabled  bool
	volumePathNotFound   bool
}

const (
	Volume1      = "d0f055a700000000"
	datafile     = "test/tmp/datafile"
	datadir      = "test/tmp/datadir"
	datafile2    = "test/tmp/datafile2"
	datadir2     = "test/tmp/datadir2"
	clusterName1 = "cluster1"
	logLevel     = constants.DefaultLogLevel
)

func (f *feature) aIsilonService() error {
	f.checkGoRoutines("start aIsilonService")

	f.err = nil
	f.getPluginInfoResponse = nil
	f.volumeIDList = f.volumeIDList[:0]
	f.snapshotIDList = f.snapshotIDList[:0]

	// configure gofsutil; we use a mock interface
	gofsutil.UseMockFS()
	gofsutil.GOFSMock.InduceBindMountError = false
	gofsutil.GOFSMock.InduceMountError = false
	gofsutil.GOFSMock.InduceGetMountsError = false
	gofsutil.GOFSMock.InduceDevMountsError = false
	gofsutil.GOFSMock.InduceUnmountError = false
	gofsutil.GOFSMock.InduceFormatError = false
	gofsutil.GOFSMock.InduceGetDiskFormatError = false
	gofsutil.GOFSMock.InduceGetDiskFormatType = ""
	gofsutil.GOFSMockMounts = gofsutil.GOFSMockMounts[:0]

	// set induced errors
	inducedErrors.badVolumeIdentifier = false
	inducedErrors.invalidVolumeID = false
	inducedErrors.noVolumeID = false
	inducedErrors.differentVolumeID = false
	inducedErrors.noNodeName = false
	inducedErrors.noNodeID = false
	inducedErrors.omitVolumeCapability = false
	inducedErrors.omitAccessMode = false

	// initialize volume and export existence status
	stepHandlersErrors.ExportNotFoundError = true
	stepHandlersErrors.VolumeNotExistError = true

	// Get the httptest mock handler. Only set
	// a new server if there isn't one already.
	handler := getHandler()
	// Get or reuse the cached service
	f.getService()
	clusterConfig := f.service.getIsilonClusterConfig(clusterName1)
	if handler != nil && os.Getenv("CSI_ISILON_ENDPOINT") == "" {
		if f.server == nil {
			f.server = httptest.NewServer(handler)
		}
		log.Printf("server url: %s\n", f.server.URL)
		clusterConfig.EndpointURL = f.server.URL
		//f.service.opts.EndpointURL = f.server.URL
	} else {
		f.server = nil
	}
	isiSvc, _ := f.service.GetIsiService(context.Background(), clusterConfig, logLevel)
	updatedClusterConfig, _ := f.service.isiClusters.Load(clusterName1)
	updatedClusterConfig.(*IsilonClusterConfig).isiSvc = isiSvc
	f.service.isiClusters.Store(clusterName1, updatedClusterConfig)
	f.checkGoRoutines("end aIsilonService")
	f.service.logServiceStats()
	return nil
}

func (f *feature) renderOneFSAPIUnreachable() error {
	testControllerHasNoConnection = true
	testNodeHasNoConnection = true
	return nil
}

func (f *feature) enableQuota() error {
	f.service.opts.QuotaEnabled = true
	return nil
}

func (f *feature) getService() *service {
	testControllerHasNoConnection = false
	testNodeHasNoConnection = false
	svc := new(service)
	var opts Opts

	opts.AccessZone = "System"
	opts.Path = "/ifs/data/csi-isilon"
	opts.SkipCertificateValidation = true
	opts.isiAuthType = 0
	opts.Verbose = 1
	opts.KubeConfigPath = "mock/k8s/admin.conf"

	newConfig := IsilonClusterConfig{}
	newConfig.ClusterName = clusterName1
	newConfig.Endpoint = "127.0.0.1"
	newConfig.EndpointPort = "8080"
	newConfig.EndpointURL = "http://127.0.0.1"
	newConfig.User = "blah"
	newConfig.Password = "blah"
	newConfig.SkipCertificateValidation = &opts.SkipCertificateValidation
	newConfig.IsiPath = "/ifs/data/csi-isilon"
	boolTrue := true
	newConfig.IsDefault = &boolTrue

	if os.Getenv("CSI_ISILON_ENDPOINT") != "" {
		newConfig.EndpointURL = os.Getenv("CSI_ISILON_ENDPOINT")
	}
	if os.Getenv("CSI_ISILON_USERID") != "" {
		newConfig.User = os.Getenv("CSI_ISILON_USERID")
	}
	if os.Getenv("CSI_ISILON_PASSWORD") != "" {
		newConfig.Password = os.Getenv("CSI_ISILON_PASSWORD")
	}
	if os.Getenv("CSI_ISILON_PATH") != "" {
		newConfig.IsiPath = os.Getenv("CSI_ISILON_PATH")
	}
	if os.Getenv("CSI_ISILON_ZONE") != "" {
		opts.AccessZone = os.Getenv("CSI_ISILON_ZONE")
	}

	svc.opts = opts
	svc.mode = "controller"
	server := grpc.NewServer()
	svc.RegisterAdditionalServers(server)
	f.service = svc
	f.service.nodeID, _ = os.Hostname()
	f.service.nodeIP = "127.0.0.1"
	f.service.defaultIsiClusterName = clusterName1
	f.service.isiClusters = new(sync.Map)
	f.service.isiClusters.Store(newConfig.ClusterName, &newConfig)

	return svc
}

func (f *feature) iSetEmptyPassword() error {
	cluster, _ := f.service.isiClusters.Load(clusterName1)
	cluster.(*IsilonClusterConfig).Password = ""
	f.service.isiClusters.Store(clusterName1, cluster)
	return nil
}

func (f *feature) checkGoRoutines(tag string) {
	goroutines := runtime.NumGoroutine()
	fmt.Printf("goroutines %s new %d old groutines %d\n", tag, goroutines, f.nGoRoutines)
	f.nGoRoutines = goroutines
}

func FeatureContext(s *godog.Suite) {
	f := &feature{}
	s.Step(`^a Isilon service$`, f.aIsilonService)
	s.Step(`^a Isilon service with params "([^"]*)" "([^"]*)"$`, f.aIsilonServiceWithParams)
	s.Step(`^a Isilon service with custom topology "([^"]*)" "([^"]*)"$`, f.aIsilonServiceWithParamsForCustomTopology)
	s.Step(`^a Isilon service with custom topology and no label "([^"]*)" "([^"]*)"$`, f.aIsilonServiceWithParamsForCustomTopologyNoLabel)
	s.Step(`^a Isilon service with IsiAuthType as session based$`, f.aIsilonservicewithIsiAuthTypeassessionbased)
	s.Step(`^I render Isilon service unreachable$`, f.renderOneFSAPIUnreachable)
	s.Step(`^I enable quota$`, f.enableQuota)
	s.Step(`^I call GetPluginInfo$`, f.iCallGetPluginInfo)
	s.Step(`^a valid GetPlugInfoResponse is returned$`, f.aValidGetPlugInfoResponseIsReturned)
	s.Step(`^I call GetPluginCapabilities$`, f.iCallGetPluginCapabilities)
	s.Step(`^a valid GetPluginCapabilitiesResponse is returned$`, f.aValidGetPluginCapabilitiesResponseIsReturned)
	s.Step(`^I call Probe$`, f.iCallProbe)
	s.Step(`^I call autoProbe$`, f.iCallAutoProbe)
	s.Step(`^a valid ProbeResponse is returned$`, f.aValidProbeResponseIsReturned)
	s.Step(`^an invalid ProbeResponse is returned$`, f.anInvalidProbeResponseIsReturned)
	s.Step(`^I set empty password for Isilon service$`, f.iSetEmptyPassword)
	s.Step(`^I call CreateVolume "([^"]*)"$`, f.iCallCreateVolume)
	s.Step(`^I call CreateVolume with persistent metadata "([^"]*)"$`, f.iCallCreateVolumeWithPersistentMetadata)
	s.Step(`^I call CreateVolume with params "([^"]*)" (-?\d+) "([^"]*)" "([^"]*)" "([^"]*)" "([^"]*)"$`, f.iCallCreateVolumeWithParams)
	s.Step(`^I call DeleteVolume "([^"]*)"$`, f.iCallDeleteVolume)
	s.Step(`^a valid CreateVolumeResponse is returned$`, f.aValidCreateVolumeResponseIsReturned)
	s.Step(`^a valid DeleteVolumeResponse is returned$`, f.aValidDeleteVolumeResponseIsReturned)
	s.Step(`^I induce error "([^"]*)"$`, f.iInduceError)
	s.Step(`^the error contains "([^"]*)"$`, f.theErrorContains)
	s.Step(`^I call ControllerGetCapabilities "([^"]*)"$`, f.iCallControllerGetCapabilities)
	s.Step(`^a valid ControllerGetCapabilitiesResponse is returned$`, f.aValidControllerGetCapabilitiesResponseIsReturned)
	s.Step(`^I call ValidateVolumeCapabilities with voltype "([^"]*)" access "([^"]*)"$`, f.iCallValidateVolumeCapabilitiesWithVoltypeAccess)
	s.Step(`^I call GetCapacity$`, f.iCallGetCapacity)
	s.Step(`^I call GetCapacity with params "([^"]*)"$`, f.iCallGetCapacityWithParams)
	s.Step(`^a valid GetCapacityResponse is returned$`, f.aValidGetCapacityResponseIsReturned)
	s.Step(`^I call GetCapacity with Invalid access mode$`, f.iCallGetCapacityWithInvalidAccessMode)
	s.Step(`^I call NodeGetInfo$`, f.iCallNodeGetInfo)
	s.Step(`^a valid NodeGetInfoResponse is returned$`, f.aValidNodeGetInfoResponseIsReturned)
	s.Step(`^I call set attribute MaxVolumesPerNode "([^"]*)"$`, f.iCallSetAttributeMaxVolumesPerNode)
	s.Step(`^a valid NodeGetInfoResponse is returned with volume limit "([^"]*)"$`, f.aValidNodeGetInfoResponseIsReturnedWithVolumeLimit)
	s.Step(`^I call NodeGetInfo with invalid volume limit "([^"]*)"$`, f.iCallNodeGetInfoWithInvalidVolumeLimit)
	s.Step(`^I call apply node label "([^"]*)"$`, f.iCallApplyNodeLabel)
	s.Step(`^I call remove node labels$`, f.iCallRemoveNodeLabels)
	s.Step(`^I call NodeGetCapabilities "([^"]*)"$`, f.iCallNodeGetCapabilities)
	s.Step(`^a valid NodeGetCapabilitiesResponse is returned$`, f.aValidNodeGetCapabilitiesResponseIsReturned)
	s.Step(`^I have a Node "([^"]*)" with AccessZone$`, f.iHaveANodeWithAccessZone)
	s.Step(`^I call ControllerPublishVolume with "([^"]*)" to "([^"]*)"$`, f.iCallControllerPublishVolumeWithTo)
	s.Step(`^a valid ControllerPublishVolumeResponse is returned$`, f.aValidControllerPublishVolumeResponseIsReturned)
	s.Step(`^a controller published volume$`, f.aControllerPublishedVolume)
	s.Step(`^a capability with voltype "([^"]*)" access "([^"]*)"$`, f.aCapabilityWithVoltypeAccess)
	s.Step(`^I call NodePublishVolume$`, f.iCallNodePublishVolume)
	s.Step(`^I call EphemeralNodePublishVolume$`, f.iCallEphemeralNodePublishVolume)
	s.Step(`^get Node Publish Volume Request$`, f.getNodePublishVolumeRequest)
	s.Step(`^get Node Publish Volume Request with Volume Name "([^"]*)"$`, f.getNodePublishVolumeRequestwithVolumeName)
	s.Step(`^get Node Publish Volume Request with Volume Name "([^"]*)" and path "([^"]*)"$`, f.getNodePublishVolumeRequestwithVolumeNameandPath)
	s.Step(`^I change the target path$`, f.iChangeTheTargetPath)
	s.Step(`^I mark request read only$`, f.iMarkRequestReadOnly)
	s.Step(`^I call NodeStageVolume with name "([^"]*)" and access type "([^"]*)"$`, f.iCallNodeStageVolume)
	s.Step(`^I call ControllerPublishVolume with name "([^"]*)" and access type "([^"]*)" to "([^"]*)"$`, f.iCallControllerPublishVolume)
	s.Step(`^a valid NodeStageVolumeResponse is returned$`, f.aValidNodeStageVolumeResponseIsReturned)
	s.Step(`^I call NodeUnstageVolume with name "([^"]*)"$`, f.iCallNodeUnstageVolume)
	s.Step(`^I call ControllerUnpublishVolume with name "([^"]*)" and access type "([^"]*)" to "([^"]*)"$`, f.iCallControllerUnPublishVolume)
	s.Step(`^a valid NodeUnstageVolumeResponse is returned$`, f.aValidNodeUnstageVolumeResponseIsReturned)
	s.Step(`^a valid ControllerUnpublishVolumeResponse is returned$`, f.aValidControllerUnpublishVolumeResponseIsReturned)
	s.Step(`^I call ListVolumes with max entries (-?\d+) starting token "([^"]*)"$`, f.iCallListVolumesWithMaxEntriesStartingToken)
	s.Step(`^a valid ListVolumesResponse is returned$`, f.aValidListVolumesResponseIsReturned)
	s.Step(`^I call NodeUnpublishVolume$`, f.iCallNodeUnpublishVolume)
	s.Step(`^I call EphemeralNodeUnpublishVolume$`, f.iCallEphemeralNodeUnpublishVolume)
	s.Step(`^a valid NodeUnpublishVolumeResponse is returned$`, f.aValidNodeUnpublishVolumeResponseIsReturned)
	s.Step(`^I call CreateSnapshot "([^"]*)" "([^"]*)" "([^"]*)"$`, f.iCallCreateSnapshot)
	s.Step(`^a valid CreateSnapshotResponse is returned$`, f.aValidCreateSnapshotResponseIsReturned)
	s.Step(`^I call DeleteSnapshot "([^"]*)"$`, f.iCallDeleteSnapshot)
	s.Step(`^I call CreateVolumeFromSnapshot "([^"]*)" "([^"]*)"$`, f.iCallCreateVolumeFromSnapshot)
	s.Step(`^I call CreateVolumeFromVolume "([^"]*)" "([^"]*)"$`, f.iCallCreateVolumeFromVolume)
	s.Step(`^I call initialize real isilon service$`, f.iCallInitializeRealIsilonService)
	s.Step(`^I call logStatistics (\d+) times$`, f.iCallLogStatisticsTimes)
	s.Step(`^I call BeforeServe$`, f.iCallBeforeServe)
	s.Step(`^I call CreateQuota in isiService with negative sizeInBytes$`, f.ICallCreateQuotaInIsiServiceWithNegativeSizeInBytes)
	s.Step(`^I call get export related functions in isiService$`, f.iCallGetExportRelatedFunctionsInIsiService)
	s.Step(`^I call unimplemented functions$`, f.iCallUnimplementedFunctions)
	s.Step(`^I call init Service object$`, f.iCallInitServiceObject)
	s.Step(`^I call ControllerExpandVolume "([^"]*)" "([^"]*)"$`, f.iCallControllerExpandVolume)
	s.Step(`^a valid ControllerExpandVolumeResponse is returned$`, f.aValidControllerExpandVolumeResponseIsReturned)
	s.Step(`^I call set allowed networks "([^"]*)"$`, f.iCallSetAllowedNetworks)
	s.Step(`^I call set allowed networks with multiple networks "([^"]*)" "([^"]*)"$`, f.iCallSetAllowedNetworkswithmultiplenetworks)
	s.Step(`^I call NodeGetInfo with invalid networks$`, f.iCallNodeGetInfowithinvalidnetworks)
	s.Step(`^I set RootClientEnabled to "([^"]*)"$`, f.iSetRootClientEnabledTo)
	s.Step(`^I call ControllerGetVolume with name "([^"]*)"$`, f.iCallControllerGetVolume)
	s.Step(`^a valid ControllerGetVolumeResponse is returned$`, f.aValidControllerGetVolumeResponseIsReturned)
	s.Step(`^I call NodeGetVolumeStats with name "([^"]*)"$`, f.iCallNodeGetVolumeStats)
	s.Step(`^a NodeGetVolumeResponse is returned$`, f.aNodeGetVolumeResponseIsReturned)
	s.Step(`^I call iCallNodeGetInfoWithNoFQDN`, f.iCallNodeGetInfoWithNoFQDN)
	s.Step(`^a valid NodeGetInfoResponse is returned$`, f.aValidNodeGetInfoResponseIsReturned)
	s.Step(`^I call CreateRemoteVolume`, f.iCallCreateRemoteVolume)
	s.Step(`^a valid CreateRemoteVolumeResponse is returned$`, f.aValidCreateRemoteVolumeResponseIsReturned)
	s.Step(`I call CreateStorageProtectionGroup`, f.iCallCreateStorageProtectionGroup)
	s.Step(`^a valid CreateStorageProtectionGroupResponse is returned$`, f.aValidCreateStorageProtectionGroupResponseIsReturned)
	s.Step(`^I call StorageProtectionGroupDelete "([^"]*)" and "([^"]*)" and "([^"]*)"$`, f.iCallStorageProtectionGroupDelete)
	s.Step(`^a valid DeleteStorageProtectionGroupResponse is returned$`, f.aValidDeleteStorageProtectionGroupResponseIsReturned)
	//s.Step(`^I call DeleteStorageProtectionGroupWithParams"$`, f.iCallWithParamsDeleteStorageProtectionGroup)
	s.Step(`^I call WithParamsCreateRemoteVolume "([^"]*)" "([^"]*)"$`, f.iCallCreateRemoteVolumeWithParams)
	s.Step(`^I call WithParamsCreateStorageProtectionGroup "([^"]*)" "([^"]*)"$`, f.iCallCreateStorageProtectionGroupWithParams)
	s.Step(`I call GetStorageProtectionGroupStatus`, f.iCallGetStorageProtectionGroupStatus)
	s.Step(`^a valid GetStorageProtectionGroupStatusResponse is returned$`, f.aValidGetStorageProtectionGroupStatusResponseIsReturned)
	s.Step(`^I call WithParamsGetStorageProtectionGroupStatus "([^"]*)" "([^"]*)" "([^"]*)" "([^"]*)" "([^"]*)" "([^"]*)"$`, f.iCallGetStorageProtectionGroupStatusWithParams)
	s.Step(`I call ExecuteAction to "([^"]*)" to "([^"]*)" to "([^"]*)" to "([^"]*)" to "([^"]*)" to "([^"]*)"$`, f.iCallExecuteAction)
	s.Step(`^a valid ExecuteActionResponse is returned$`, f.aValidExecuteActionResponseIsReturned)
	s.Step(`I call SuspendExecuteAction`, f.iCallExecuteActionSuspend)
	s.Step(`I call ReprotectExecuteAction`, f.iCallExecuteActionReprotect)
	s.Step(`I call SyncExecuteAction`, f.iCallExecuteActionSync)
	s.Step(`^a valid ExecuteActionResponse is returned$`, f.aValidExecuteActionResponseIsReturned)
	s.Step(`I call FailoverExecuteAction`, f.iCallExecuteActionSyncFailover)
	s.Step(`I call FailoverUnplannedExecuteAction`, f.iCallExecuteActionSyncFailoverUnplanned)
	s.Step(`I call BadExecuteAction`, f.iCallExecuteActionBad)
	s.Step(`^I call BadCreateRemoteVolume`, f.iCallCreateRemoteVolumeBad)
	s.Step(`^I call BadCreateStorageProtectionGroup`, f.iCallCreateStorageProtectionGroupBad)
	//s.Step(`I call ExecuteActionWithParams "([^"]*)" "([^"]*)" "([^"]*)" "([^"]*)" "([^"]*)" "([^"]*)"$`, f.iCallExecuteActionWithParams)
	s.Step(`^I call GetReplicationCapabilities`, f.iCallGetReplicationCapabilities)
	s.Step(`^a valid GetReplicationCapabilitiesResponse is returned$`, f.aValidGetReplicationCapabilitiesResponseIsReturned)
	s.Step(`^I call ValidateConnectivity$`, f.iCallValidateVolumeHostConnectivity)
	s.Step(`^the ValidateConnectivity response message contains "([^"]*)"$`, f.theValidateConnectivityResponseMessageContains)
	s.Step(`^I call ProbeController$`, f.iCallProbeController)
	s.Step(`^I call DynamicLogChange "([^"]*)"$`, f.iCallDynamicLogChange)
	s.Step(`^a valid DynamicLogChange occurs "([^"]*)" "([^"]*)"$`, f.aValidDynamicLogChangeOccurs)
	s.Step(`^I set noProbeOnStart to "([^"]*)"$`, f.iSetNoProbeOnStart)
	s.Step(`^I call GetSnapshotNameFromIsiPath with "([^"]*)"$`, f.iCallGetSnapshotNameFromIsiPathWith)
	s.Step(`^I call GetSnapshotIsiPathComponents`, f.iCallGetSnapshotIsiPathComponents)
	s.Step(`^I call GetSubDirectoryCount`, f.iCallGetSubDirectoryCount)
	s.Step(`^I call DeleteSnapshot`, f.iCallDeleteSnapshotIsiService)
	s.Step(`^I call CreateVolumeRequest$`, f.iCallCreateVolumeReplicationEnabled)
	s.Step(`^I call CreateVolumeFromSnapshotMultiReader "([^"]*)" "([^"]*)"$`, f.iCallCreateVolumeFromSnapshotMultiReader)
	s.Step(`^a valid DeleteSnapshotResponse is returned$`, f.aValidDeleteSnapshotResponseIsReturned)
	s.Step(`^I set mode to "([^"]*)"$`, f.iSetModeTo)
	s.Step(`^I call startAPIService`, f.iCallStartAPIService)
	s.Step(`^I set podmon enable to "([^"]*)"$`, f.iSetPodmonEnable)
	s.Step(`^I set API port to "([^"]*)"$`, f.iSetAPIPort)
	s.Step(`^I set polling freq to "([^"]*)"$`, f.iSetPollingFeqTo)
	s.Step(`^I call ControllerPublishVolume on Snapshot with name "([^"]*)" and access type "([^"]*)" to "([^"]*)" and path "([^"]*)"$`, f.iCallControllerPublishVolumeOnSnapshot)
	s.Step(`^I call QueryArrayStatus "([^"]*)"$`, f.iCallQueryArrayStatus)
	s.Step(`^get Node Unpublish Volume Request for RO Snapshot "([^"]*)" and path "([^"]*)"$`, f.getNodeUnpublishVolumeRequestForROSnapshot)
}

// GetPluginInfo
func (f *feature) iCallGetPluginInfo() error {
	req := new(csi.GetPluginInfoRequest)
	f.getPluginInfoResponse, f.err = f.service.GetPluginInfo(context.Background(), req)
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *feature) iCallStartAPIService() error {
	ctx, cancel := context2.WithTimeout(context.Background(), time.Duration(time.Second*2))
	defer cancel()
	f.service.startAPIService(ctx)
	return nil
}

func (f *feature) iSetPodmonEnable(value string) error {
	os.Setenv(constants.EnvPodmonEnabled, value)
	return nil
}
func (f *feature) iSetModeTo(value string) error {
	os.Setenv(gocsi.EnvVarMode, value)
	return nil
}
func (f *feature) iSetAPIPort(value string) error {
	os.Setenv(constants.EnvPodmonAPIPORT, value)
	return nil
}

func (f *feature) iSetPollingFeqTo(value string) error {
	os.Setenv(constants.EnvPodmonArrayConnectivityPollRate, value)
	return nil
}

func (f *feature) aValidGetPlugInfoResponseIsReturned() error {
	rep := f.getPluginInfoResponse
	url := rep.GetManifest()["url"]
	if rep.GetName() == "" || rep.GetVendorVersion() == "" || url == "" {
		return errors.New("Expected GetPluginInfo to return name and version")
	}
	log.Printf("Name %s Version %s URL %s", rep.GetName(), rep.GetVendorVersion(), url)
	return nil
}

func (f *feature) iCallGetPluginCapabilities() error {
	req := new(csi.GetPluginCapabilitiesRequest)
	f.getPluginCapabilitiesResponse, f.err = f.service.GetPluginCapabilities(context.Background(), req)
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *feature) aValidGetPluginCapabilitiesResponseIsReturned() error {
	rep := f.getPluginCapabilitiesResponse
	capabilities := rep.GetCapabilities()
	var foundController bool
	for _, capability := range capabilities {
		if capability.GetService().GetType() == csi.PluginCapability_Service_CONTROLLER_SERVICE {
			foundController = true
		}
	}
	if !foundController {
		return errors.New("Expected PluginCapabilitiesResponse to contain CONTROLLER_SERVICE")
	}
	return nil
}

func (f *feature) iCallProbe() error {
	req := new(csi.ProbeRequest)
	f.checkGoRoutines("before probe")
	f.probeResponse, f.err = f.service.Probe(context.Background(), req)
	f.checkGoRoutines("after probe")
	return nil
}

func (f *feature) iCallAutoProbe() error {
	f.checkGoRoutines("before auto probe")
	f.err = f.service.autoProbe(context.Background(), f.service.getIsilonClusterConfig(clusterName1))
	f.checkGoRoutines("after auto probe")
	return nil
}

func (f *feature) aValidProbeResponseIsReturned() error {
	if f.probeResponse.GetReady().GetValue() != true {
		return errors.New("Probe returned 'Ready': false")
	}
	return nil
}

func (f *feature) anInvalidProbeResponseIsReturned() error {
	if f.probeResponse.GetReady().GetValue() != false {
		return errors.New("Probe returned 'Ready': true")
	}
	return nil
}

func getTypicalCreateVolumeRequest() *csi.CreateVolumeRequest {
	req := new(csi.CreateVolumeRequest)
	req.Name = "volume1"
	capacityRange := new(csi.CapacityRange)
	capacityRange.RequiredBytes = 8 * 1024 * 1024 * 1024
	req.CapacityRange = capacityRange
	mount := new(csi.VolumeCapability_MountVolume)
	capability := new(csi.VolumeCapability)
	accessType := new(csi.VolumeCapability_Mount)
	accessType.Mount = mount
	capability.AccessType = accessType
	accessMode := new(csi.VolumeCapability_AccessMode)
	accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER
	capability.AccessMode = accessMode
	capabilities := make([]*csi.VolumeCapability, 0)
	capabilities = append(capabilities, capability)
	parameters := make(map[string]string)
	parameters[AccessZoneParam] = "System"
	parameters[IsiPathParam] = "/ifs/data/csi-isilon"
	req.Parameters = parameters
	req.VolumeCapabilities = capabilities
	return req
}

func getCreateVolumeRequestWithMetaData() *csi.CreateVolumeRequest {
	req := new(csi.CreateVolumeRequest)
	req.Name = "volume1"
	capacityRange := new(csi.CapacityRange)
	capacityRange.RequiredBytes = 8 * 1024 * 1024 * 1024
	req.CapacityRange = capacityRange
	mount := new(csi.VolumeCapability_MountVolume)
	capability := new(csi.VolumeCapability)
	accessType := new(csi.VolumeCapability_Mount)
	accessType.Mount = mount
	capability.AccessType = accessType
	accessMode := new(csi.VolumeCapability_AccessMode)
	accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER
	capability.AccessMode = accessMode
	capabilities := make([]*csi.VolumeCapability, 0)
	capabilities = append(capabilities, capability)
	parameters := make(map[string]string)
	parameters[AccessZoneParam] = "System"
	parameters[IsiPathParam] = "/ifs/data/csi-isilon"
	parameters[csiPersistentVolumeName] = "pv-name"
	parameters[csiPersistentVolumeClaimName] = "pv-claimname"
	parameters[csiPersistentVolumeClaimNamespace] = "pv-namespace"
	req.Parameters = parameters
	req.VolumeCapabilities = capabilities
	return req
}

func getCreateVolumeRequestWithParams(rangeInGiB int64, accessZone, isiPath, AzServiceIP, clusterName string) *csi.CreateVolumeRequest {
	req := new(csi.CreateVolumeRequest)
	req.Name = "volume1"
	capacityRange := new(csi.CapacityRange)
	capacityRange.RequiredBytes = rangeInGiB * 1024 * 1024 * 1024
	req.CapacityRange = capacityRange
	mount := new(csi.VolumeCapability_MountVolume)
	capability := new(csi.VolumeCapability)
	accessType := new(csi.VolumeCapability_Mount)
	accessType.Mount = mount
	capability.AccessType = accessType
	accessMode := new(csi.VolumeCapability_AccessMode)
	accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER
	capability.AccessMode = accessMode
	capabilities := make([]*csi.VolumeCapability, 0)
	capabilities = append(capabilities, capability)
	parameters := make(map[string]string)
	if accessZone != "none" {
		parameters[AccessZoneParam] = accessZone
	}
	if isiPath != "none" {
		parameters[IsiPathParam] = isiPath
	}
	if AzServiceIP != "none" {
		parameters[AzServiceIPParam] = AzServiceIP
	}
	if clusterName != "none" {
		parameters[ClusterNameParam] = clusterName
	}
	parameters[csiPersistentVolumeName] = "pv-name"
	parameters[csiPersistentVolumeClaimName] = "pv-claimname"
	parameters[csiPersistentVolumeClaimNamespace] = "pv-namespace"
	req.Parameters = parameters
	req.VolumeCapabilities = capabilities
	return req
}

func getTypicalDeleteVolumeRequest() *csi.DeleteVolumeRequest {
	req := new(csi.DeleteVolumeRequest)
	req.VolumeId = "volume1"
	return req
}

func getTypicalNodeStageVolumeRequest(accessType string) *csi.NodeStageVolumeRequest {
	req := new(csi.NodeStageVolumeRequest)
	volCtx := make(map[string]string)
	req.VolumeContext = volCtx
	req.VolumeId = "volume2"

	capability := new(csi.VolumeCapability)

	if !inducedErrors.omitAccessMode {
		capability.AccessMode = getAccessMode(accessType)
	}

	req.VolumeCapability = capability

	return req
}

func getTypicalNodeUnstageVolumeRequest(volID string) *csi.NodeUnstageVolumeRequest {
	req := new(csi.NodeUnstageVolumeRequest)
	req.VolumeId = volID
	return req
}

func getAccessMode(accessType string) *csi.VolumeCapability_AccessMode {

	accessMode := new(csi.VolumeCapability_AccessMode)
	switch accessType {
	case "single-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER
	case "multiple-reader":
		accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY
	case "multiple-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER
	case "single-reader":
		accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY
	case "unknown":
		accessMode.Mode = csi.VolumeCapability_AccessMode_UNKNOWN
	}

	return accessMode
}

func (f *feature) iCallCreateVolume(name string) error {
	req := getTypicalCreateVolumeRequest()
	if f.rootClientEnabled != "" {
		req.Parameters[RootClientEnabledParam] = f.rootClientEnabled
	}
	f.createVolumeRequest = req
	req.Name = name
	f.createVolumeResponse, f.err = f.service.CreateVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateVolume call failed: %s\n", f.err.Error())
	}
	if f.createVolumeResponse != nil {
		log.Printf("vol id %s\n", f.createVolumeResponse.GetVolume().VolumeId)
		stepHandlersErrors.ExportNotFoundError = false
		stepHandlersErrors.VolumeNotExistError = false
	}
	return nil
}

func (f *feature) iCallCreateVolumeWithPersistentMetadata(name string) error {
	req := getCreateVolumeRequestWithMetaData()
	f.createVolumeRequest = req
	req.Name = name
	f.createVolumeResponse, f.err = f.service.CreateVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateVolume call failed: %s\n", f.err.Error())
	}
	if f.createVolumeResponse != nil {
		log.Printf("vol id %s\n", f.createVolumeResponse.GetVolume().VolumeId)
		stepHandlersErrors.ExportNotFoundError = false
		stepHandlersErrors.VolumeNotExistError = false
	}
	return nil
}

func (f *feature) iCallCreateVolumeWithParams(name string, rangeInGiB int, accessZone, isiPath, AzServiceIP, clusterName string) error {
	req := getCreateVolumeRequestWithParams(int64(rangeInGiB), accessZone, isiPath, AzServiceIP, clusterName)
	f.createVolumeRequest = req
	req.Name = name
	stepHandlersErrors.ExportNotFoundError = true
	stepHandlersErrors.VolumeNotExistError = true
	f.createVolumeResponse, f.err = f.service.CreateVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateVolume call failed: %s\n", f.err.Error())
	}
	if f.createVolumeResponse != nil {
		log.Printf("vol id %s\n", f.createVolumeResponse.GetVolume().VolumeId)
		stepHandlersErrors.ExportNotFoundError = false
		stepHandlersErrors.VolumeNotExistError = false
	}
	return nil
}

func (f *feature) iCallDeleteVolume(name string) error {
	if f.deleteVolumeRequest == nil {
		req := getTypicalDeleteVolumeRequest()
		f.deleteVolumeRequest = req
	}
	req := f.deleteVolumeRequest
	req.VolumeId = name

	ctx, log, _ := GetRunIDLog(context.Background())

	f.deleteVolumeResponse, f.err = f.service.DeleteVolume(ctx, req)
	if f.err != nil {
		log.Printf("DeleteVolume call failed: '%v'\n", f.err)
	}

	return nil
}

func (f *feature) aValidCreateVolumeResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	f.volumeIDList = append(f.volumeIDList, f.createVolumeResponse.Volume.VolumeId)
	fmt.Printf("volume '%s'\n",
		f.createVolumeResponse.Volume.VolumeContext["Name"])
	return nil
}

func (f *feature) aValidDeleteVolumeResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *feature) iInduceError(errtype string) error {

	log.Printf("set induce error %s\n", errtype)
	switch errtype {
	case "InstancesError":
		stepHandlersErrors.InstancesError = true
	case "VolInstanceError":
		stepHandlersErrors.VolInstanceError = true
	case "StatsError":
		stepHandlersErrors.StatsError = true
	case "NoNodeID":
		inducedErrors.noNodeID = true
	case "OmitVolumeCapability":
		inducedErrors.omitVolumeCapability = true
	case "noIsiService":
		inducedErrors.noIsiService = true
	case "autoProbeNotEnabled":
		inducedErrors.autoProbeNotEnabled = true
	case "autoProbeFailed":
		updatedClusterConfig, _ := f.service.isiClusters.Load(clusterName1)
		updatedClusterConfig.(*IsilonClusterConfig).isiSvc = nil
		f.service.isiClusters.Store(clusterName1, updatedClusterConfig)
		f.service.opts.AutoProbe = false
	case "GOFSMockDevMountsError":
		gofsutil.GOFSMock.InduceDevMountsError = true
	case "GOFSMockMountError":
		gofsutil.GOFSMock.InduceMountError = true
	case "GOFSMockGetMountsError":
		gofsutil.GOFSMock.InduceGetMountsError = true
	case "GOFSMockUnmountError":
		gofsutil.GOFSMock.InduceUnmountError = true
	case "GOFSMockGetDiskFormatError":
		gofsutil.GOFSMock.InduceGetDiskFormatError = true
	case "GOFSMockGetDiskFormatType":
		gofsutil.GOFSMock.InduceGetDiskFormatType = "unknown-fs"
	case "GOFSMockFormatError":
		gofsutil.GOFSMock.InduceFormatError = true
	case "GOFSWWNToDevicePathError":
		gofsutil.GOFSMock.InduceWWNToDevicePathError = true
	case "GOFSRmoveBlockDeviceError":
		gofsutil.GOFSMock.InduceRemoveBlockDeviceError = true
	case "NodePublishNoTargetPath":
		f.nodePublishVolumeRequest.TargetPath = ""
	case "NodeUnpublishNoTargetPath":
		f.nodeUnpublishVolumeRequest.TargetPath = ""
	case "NodePublishNoVolumeCapability":
		f.nodePublishVolumeRequest.VolumeCapability = nil
	case "NodePublishNoAccessMode":
		f.nodePublishVolumeRequest.VolumeCapability.AccessMode = nil
	case "NodePublishNoAccessType":
		f.nodePublishVolumeRequest.VolumeCapability.AccessType = nil
	case "NodePublishFileTargetNotDir":
		f.nodePublishVolumeRequest.TargetPath = datafile
	case "BadVolumeIdentifier":
		inducedErrors.badVolumeIdentifier = true
	case "TargetNotCreatedForNodePublish":
		err := os.Remove(datafile)
		if err != nil {
			return nil
		}
		//cmd := exec.Command("rm", "-rf", datadir)
		//_, err = cmd.CombinedOutput()
		err = os.RemoveAll(datadir)
		if err != nil {
			return err
		}
	case "OmitAccessMode":
		inducedErrors.omitAccessMode = true
	case "TargetNotCreatedForNodeUnpublish":
		err := os.RemoveAll(datadir)
		if err != nil {
			return nil
		}
	case "GetSnapshotError":
		stepHandlersErrors.GetSnapshotError = true
	case "DeleteSnapshotError":
		stepHandlersErrors.DeleteSnapshotError = true
	case "CreateQuotaError":
		stepHandlersErrors.CreateQuotaError = true
	case "CreateExportError":
		stepHandlersErrors.CreateExportError = true
	case "UpdateQuotaError":
		stepHandlersErrors.UpdateQuotaError = true
	case "GetExportInternalError":
		stepHandlersErrors.GetExportInternalError = true
	case "VolumeNotExistError":
		stepHandlersErrors.VolumeNotExistError = true
	case "ExportNotFoundError":
		stepHandlersErrors.ExportNotFoundError = true
	case "VolumeExists":
		stepHandlersErrors.VolumeNotExistError = false
	case "ExportExists":
		stepHandlersErrors.ExportNotFoundError = false
	case "ControllerHasNoConnectionError":
		testControllerHasNoConnection = true
	case "NodeHasNoConnectionError":
		testNodeHasNoConnection = true
	case "GetExportByIDNotFoundError":
		stepHandlersErrors.GetExportByIDNotFoundError = true
	case "UnexportError":
		stepHandlersErrors.UnexportError = true
	case "CreateSnapshotError":
		stepHandlersErrors.CreateSnapshotError = true
	case "DeleteQuotaError":
		stepHandlersErrors.DeleteQuotaError = true
	case "QuotaNotFoundError":
		stepHandlersErrors.QuotaNotFoundError = true
	case "DeleteVolumeError":
		stepHandlersErrors.DeleteVolumeError = true
	case "GetPolicyInternalError":
		stepHandlersErrors.GetPolicyInternalError = true
	case "GetJobsInternalError":
		stepHandlersErrors.GetJobsInternalError = true
	case "GetTargetPolicyInternalError":
		stepHandlersErrors.GetTargetPolicyInternalError = true
	case "GetTargetPolicyNotFound":
		stepHandlersErrors.GetTargetPolicyNotFound = true
	case "GetPolicyNotFoundError":
		stepHandlersErrors.GetPolicyNotFoundError = true
	case "DeletePolicyError":
		stepHandlersErrors.DeletePolicyError = true
	case "DeletePolicyInternalError":
		stepHandlersErrors.DeletePolicyInternalError = true
	case "DeletePolicyNotAPIError":
		stepHandlersErrors.DeletePolicyNotAPIError = true
	case "FailedStatus":
		stepHandlersErrors.FailedStatus = true
	case "UnknownStatus":
		stepHandlersErrors.UnknownStatus = true
	case "UpdatePolicyError":
		stepHandlersErrors.UpdatePolicyError = true
	case "Reprotect":
		stepHandlersErrors.Reprotect = true
	case "ReprotectTP":
		stepHandlersErrors.ReprotectTP = true
	case "GetPolicyError":
		stepHandlersErrors.GetPolicyError = true
	case "Failover":
		stepHandlersErrors.Failover = true
	case "FailoverTP":
		stepHandlersErrors.FailoverTP = true
	case "Jobs":
		stepHandlersErrors.Jobs = true
	case "GetSpgErrors":
		stepHandlersErrors.GetSpgErrors = true
	case "GetSpgTPErrors":
		stepHandlersErrors.GetSpgTPErrors = true
	case "GetExportPolicyError":
		stepHandlersErrors.GetExportPolicyError = true
	case "no-nodeId":
		stepHandlersErrors.PodmonVolumeStatisticsError = true
		stepHandlersErrors.PodmonNoNodeIDError = true
	case "no-volume-no-nodeId":
		stepHandlersErrors.PodmonVolumeStatisticsError = true
		stepHandlersErrors.PodmonNoVolumeNoNodeIDError = true
	case "volumePathNotFound":
		inducedErrors.volumePathNotFound = true
	case "ModifyLastAttempt":
		stepHandlersErrors.ModifyLastAttempt = true
	case "none":

	default:
		return fmt.Errorf("Don't know how to induce error %q", errtype)
	}
	return nil
}

func (f *feature) theErrorContains(arg1 string) error {
	// If arg1 is none, we expect no error, any error received is unexpected
	clearErrors()
	if arg1 == "none" {
		if f.err == nil {
			return nil
		}
		return fmt.Errorf("Unexpected error: %s", f.err)
	}
	// We expected an error...
	if f.err == nil {
		return fmt.Errorf("Expected error to contain %s but no error", arg1)
	}
	// Allow for multiple possible matches, separated by @@. This was necessary
	// because Windows and Linux sometimes return different error strings for
	// gofsutil operations. Note @@ was used instead of || because the Gherkin
	// parser is not smart enough to ignore vertical braces within a quoted string,
	// so if || is used it thinks the row's cell count is wrong.
	possibleMatches := strings.Split(arg1, "@@")
	for _, possibleMatch := range possibleMatches {
		if strings.Contains(f.err.Error(), possibleMatch) {
			return nil
		}
	}
	return fmt.Errorf("Expected error to contain %s but it was %s", arg1, f.err.Error())
}

func (f *feature) iCallControllerGetCapabilities(isHealthMonitorEnabled string) error {
	if isHealthMonitorEnabled == "true" {
		f.service.opts.IsHealthMonitorEnabled = true
	}
	req := new(csi.ControllerGetCapabilitiesRequest)
	f.controllerGetCapabilitiesResponse, f.err = f.service.ControllerGetCapabilities(context.Background(), req)
	if f.err != nil {
		log.Printf("ControllerGetCapabilities call failed: %s\n", f.err.Error())
		return f.err
	}
	return nil
}

func (f *feature) aValidControllerGetCapabilitiesResponseIsReturned() error {
	rep := f.controllerGetCapabilitiesResponse
	if rep != nil {
		if rep.Capabilities == nil {
			return errors.New("no capabilities returned in ControllerGetCapabilitiesResponse")
		}
		count := 0
		for _, cap := range rep.Capabilities {
			rpcType := cap.GetRpc().Type
			switch rpcType {
			case csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME:
				count = count + 1
			case csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME:
				count = count + 1
			case csi.ControllerServiceCapability_RPC_LIST_VOLUMES:
				count = count + 1
			case csi.ControllerServiceCapability_RPC_GET_CAPACITY:
				count = count + 1
			case csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT:
				count = count + 1
			case csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS:
				count = count + 1
			case csi.ControllerServiceCapability_RPC_CLONE_VOLUME:
				count = count + 1
			case csi.ControllerServiceCapability_RPC_EXPAND_VOLUME:
				count = count + 1
			case csi.ControllerServiceCapability_RPC_VOLUME_CONDITION:
				count = count + 1
			case csi.ControllerServiceCapability_RPC_GET_VOLUME:
				count = count + 1
			case csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER:
				count = count + 1
			default:
				return fmt.Errorf("received unexpected capability: %v", rpcType)
			}
		}

		if f.service.opts.IsHealthMonitorEnabled && count != 9 {
			// Set default value
			f.service.opts.IsHealthMonitorEnabled = false
			return errors.New("Did not retrieve all the expected capabilities")
		} else if !f.service.opts.IsHealthMonitorEnabled && count != 7 {
			return errors.New("Did not retrieve all the expected capabilities")
		}

		// Set default value
		f.service.opts.IsHealthMonitorEnabled = false
		return nil
	}
	return errors.New("expected ControllerGetCapabilitiesResponse but didn't get one")
}

func (f *feature) iCallValidateVolumeCapabilitiesWithVoltypeAccess(voltype, access string) error {
	req := new(csi.ValidateVolumeCapabilitiesRequest)
	if inducedErrors.invalidVolumeID || f.createVolumeResponse == nil {
		req.VolumeId = "000-000"
	} else {
		req.VolumeId = f.createVolumeResponse.GetVolume().VolumeId
	}
	// Construct the volume capabilities
	capability := new(csi.VolumeCapability)
	switch voltype {
	case "block":
		block := new(csi.VolumeCapability_BlockVolume)
		accessType := new(csi.VolumeCapability_Block)
		accessType.Block = block
		capability.AccessType = accessType
	case "mount":
		mount := new(csi.VolumeCapability_MountVolume)
		accessType := new(csi.VolumeCapability_Mount)
		accessType.Mount = mount
		capability.AccessType = accessType
	}
	accessMode := new(csi.VolumeCapability_AccessMode)
	switch access {
	case "single-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER
	case "single-reader":
		accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY
	case "multi-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER
	case "multi-reader":
		accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY
	case "multi-node-single-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER
	case "single-node-single-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER
	case "single-node-multiple-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER
	}
	capability.AccessMode = accessMode
	capabilities := make([]*csi.VolumeCapability, 0)
	capabilities = append(capabilities, capability)
	req.VolumeCapabilities = capabilities
	log.Printf("Calling ValidateVolumeCapabilities")
	ctx, _, _ := GetRunIDLog(context.Background())
	f.validateVolumeCapabilitiesResponse, f.err = f.service.ValidateVolumeCapabilities(ctx, req)
	if f.err != nil {
		return nil
	}
	if f.validateVolumeCapabilitiesResponse.Message != "" {
		f.err = errors.New(f.validateVolumeCapabilitiesResponse.Message)
	} else {
		// Validate we get a Confirmed structure with VolumeCapabilities
		if f.validateVolumeCapabilitiesResponse.Confirmed == nil {
			return errors.New("Expected ValidateVolumeCapabilities to have a Confirmed structure but it did not")
		}
		confirmed := f.validateVolumeCapabilitiesResponse.Confirmed
		if len(confirmed.VolumeCapabilities) <= 0 {
			return errors.New("Expected ValidateVolumeCapabilities to return the confirmed VolumeCapabilities but it did not")
		}
	}
	return nil
}

func clearErrors() {
	stepHandlersErrors.ExportNotFoundError = true
	stepHandlersErrors.VolumeNotExistError = true
	stepHandlersErrors.InstancesError = false
	stepHandlersErrors.VolInstanceError = false
	stepHandlersErrors.FindVolumeIDError = false
	stepHandlersErrors.GetVolByIDError = false
	stepHandlersErrors.GetStoragePoolsError = false
	stepHandlersErrors.GetStatisticsError = false
	stepHandlersErrors.CreateSnapshotError = false
	stepHandlersErrors.RemoveVolumeError = false
	stepHandlersErrors.StatsError = false
	stepHandlersErrors.StartingTokenInvalidError = false
	stepHandlersErrors.GetSnapshotError = false
	stepHandlersErrors.DeleteSnapshotError = false
	stepHandlersErrors.ExportNotFoundError = false
	stepHandlersErrors.VolumeNotExistError = false
	stepHandlersErrors.CreateQuotaError = false
	stepHandlersErrors.UpdateQuotaError = false
	stepHandlersErrors.CreateExportError = false
	stepHandlersErrors.GetExportInternalError = false
	stepHandlersErrors.GetExportByIDNotFoundError = false
	stepHandlersErrors.UnexportError = false
	stepHandlersErrors.DeleteQuotaError = false
	stepHandlersErrors.QuotaNotFoundError = false
	stepHandlersErrors.DeleteVolumeError = false
	inducedErrors.noIsiService = false
	inducedErrors.autoProbeNotEnabled = false
	inducedErrors.volumePathNotFound = false
	stepHandlersErrors.GetJobsInternalError = false
	stepHandlersErrors.GetPolicyInternalError = false
	stepHandlersErrors.GetTargetPolicyInternalError = false
	stepHandlersErrors.GetTargetPolicyNotFound = false
	stepHandlersErrors.GetPolicyNotFoundError = false
	stepHandlersErrors.count = 0
	stepHandlersErrors.counter = 0
	stepHandlersErrors.reprotectCount = 0
	stepHandlersErrors.reprotectTPCount = 0
	stepHandlersErrors.failoverTPCount = 0
	stepHandlersErrors.failoverCount = 0
	stepHandlersErrors.jobCount = 0
	stepHandlersErrors.getSpgCount = 0
	stepHandlersErrors.getSpgTPCount = 0
	stepHandlersErrors.getExportCount = 0
	stepHandlersErrors.getPolicyTPCount = 0
	stepHandlersErrors.getPolicyInternalErrorTPCount = 0
	stepHandlersErrors.getPolicyNotFoundTPCount = 0
	stepHandlersErrors.DeletePolicyError = false
	stepHandlersErrors.DeletePolicyInternalError = false
	stepHandlersErrors.DeletePolicyNotAPIError = false
	stepHandlersErrors.FailedStatus = false
	stepHandlersErrors.UnknownStatus = false
	stepHandlersErrors.UpdatePolicyError = false
	stepHandlersErrors.Reprotect = false
	stepHandlersErrors.ReprotectTP = false
	stepHandlersErrors.Failover = false
	stepHandlersErrors.FailoverTP = false
	stepHandlersErrors.Jobs = false
	stepHandlersErrors.GetPolicyError = false
	stepHandlersErrors.GetSpgErrors = false
	stepHandlersErrors.GetSpgTPErrors = false
	stepHandlersErrors.GetExportPolicyError = false
	stepHandlersErrors.ModifyLastAttempt = false
}

func getTypicalCapacityRequest(valid bool) *csi.GetCapacityRequest {
	req := new(csi.GetCapacityRequest)
	// Construct the volume capabilities
	capability := new(csi.VolumeCapability)
	// Set FS type to mount volume
	mount := new(csi.VolumeCapability_MountVolume)
	accessType := new(csi.VolumeCapability_Mount)
	accessType.Mount = mount
	capability.AccessType = accessType
	// A single mode writer
	accessMode := new(csi.VolumeCapability_AccessMode)
	if valid {
		accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER
	} else {
		accessMode.Mode = csi.VolumeCapability_AccessMode_UNKNOWN
	}
	capability.AccessMode = accessMode
	capabilities := make([]*csi.VolumeCapability, 0)
	capabilities = append(capabilities, capability)
	req.VolumeCapabilities = capabilities
	return req
}

func (f *feature) iCallGetCapacity() error {
	header := metadata.New(map[string]string{"csi.requestid": "1"})
	ctx, _, _ := GetRunIDLog(context.Background())
	ctx = metadata.NewIncomingContext(ctx, header)
	req := getTypicalCapacityRequest(true)
	f.getCapacityResponse, f.err = f.service.GetCapacity(ctx, req)
	if f.err != nil {
		log.Printf("GetCapacity call failed: %s\n", f.err.Error())
		return nil
	}
	return nil
}

func (f *feature) iCallGetCapacityWithParams(clusterName string) error {
	header := metadata.New(map[string]string{"csi.requestid": "1"})
	ctx := metadata.NewIncomingContext(context.Background(), header)
	req := getTypicalCapacityRequest(true)
	params := make(map[string]string)
	params[ClusterNameParam] = clusterName
	req.Parameters = params

	f.getCapacityResponse, f.err = f.service.GetCapacity(ctx, req)
	if f.err != nil {
		log.Printf("GetCapacity call failed: %s\n", f.err.Error())
		return nil
	}
	return nil
}

func (f *feature) iCallGetCapacityWithInvalidAccessMode() error {
	header := metadata.New(map[string]string{"csi.requestid": "1"})
	ctx := metadata.NewIncomingContext(context.Background(), header)
	req := getTypicalCapacityRequest(false)
	f.getCapacityResponse, f.err = f.service.GetCapacity(ctx, req)
	if f.err != nil {
		log.Printf("GetCapacity call failed: %s\n", f.err.Error())
		return nil
	}
	return nil
}

func (f *feature) aValidGetCapacityResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	if f.getCapacityResponse == nil {
		return errors.New("Received null response to GetCapacity")
	}
	if f.getCapacityResponse.AvailableCapacity <= 0 {
		return errors.New("Expected AvailableCapacity to be positive")
	}
	fmt.Printf("Available capacity: %d\n", f.getCapacityResponse.AvailableCapacity)

	return nil
}

func (f *feature) iCallNodeGetInfo() error {
	MockK8sAPI()
	req := new(csi.NodeGetInfoRequest)
	f.nodeGetInfoResponse, f.err = f.service.NodeGetInfo(context.Background(), req)
	if f.err != nil {
		log.Printf("NodeGetInfo call failed: %s\n", f.err.Error())
		return f.err
	}
	return nil
}

func (f *feature) iCallSetAttributeMaxVolumesPerNode(volumeLimit int64) error {
	f.service.opts.MaxVolumesPerNode = volumeLimit
	return nil
}

func (f *feature) iCallNodeGetInfoWithInvalidVolumeLimit(volumeLimit int64) error {
	MockK8sAPI()
	req := new(csi.NodeGetInfoRequest)
	f.service.opts.MaxVolumesPerNode = volumeLimit
	f.nodeGetInfoResponse, f.err = f.service.NodeGetInfo(context.Background(), req)
	if f.err != nil {
		log.Printf("NodeGetInfo call failed: %s\n", f.err.Error())
	}
	return nil
}

func (f *feature) iCallNodeGetCapabilities(isHealthMonitorEnabled string) error {
	req := new(csi.NodeGetCapabilitiesRequest)
	if isHealthMonitorEnabled == "true" {
		f.service.opts.IsHealthMonitorEnabled = true
	}
	f.nodeGetCapabilitiesResponse, f.err = f.service.NodeGetCapabilities(context.Background(), req)
	if f.err != nil {
		log.Printf("NodeGetCapabilities call failed: %s\n", f.err.Error())
		return f.err
	}
	return nil
}

func (f *feature) aValidNodeGetInfoResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	fmt.Printf("The node ID is %s\n", f.nodeGetInfoResponse.NodeId)
	fmt.Printf("Default volume limit is %v\n", f.nodeGetInfoResponse.MaxVolumesPerNode)
	if f.nodeGetInfoResponse.MaxVolumesPerNode != 0 {
		return fmt.Errorf("default volume limit is not set to 0")
	}

	return nil
}

func (f *feature) aValidNodeGetInfoResponseIsReturnedWithVolumeLimit(volumeLimit int64) error {
	if f.err != nil {
		return f.err
	}
	fmt.Printf("The node ID is %s\n", f.nodeGetInfoResponse.NodeId)
	fmt.Printf("Default volume limit is %v\n", f.nodeGetInfoResponse.MaxVolumesPerNode)
	if f.nodeGetInfoResponse.MaxVolumesPerNode != volumeLimit {
		return fmt.Errorf("default volume limit is not set to %v", volumeLimit)
	}

	return nil
}

func (f *feature) aValidNodeGetCapabilitiesResponseIsReturned() error {
	rep := f.nodeGetCapabilitiesResponse
	if rep != nil {
		if rep.Capabilities == nil {
			return errors.New("No capabilities returned in NodeGetCapabilitiesResponse")
		}
		count := 0
		for _, cap := range rep.Capabilities {
			rpcType := cap.GetRpc().Type
			switch rpcType {
			case csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME:
				count = count + 1
			case csi.NodeServiceCapability_RPC_GET_VOLUME_STATS:
				count = count + 1
			case csi.NodeServiceCapability_RPC_VOLUME_CONDITION:
				count = count + 1
			case csi.NodeServiceCapability_RPC_EXPAND_VOLUME:
				count = count + 1
			case csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER:
				count = count + 1
			default:
				return fmt.Errorf("Received unexpected capability: %v", rpcType)
			}
		}
		if f.service.opts.IsHealthMonitorEnabled && count != 4 {
			// Set default value
			f.service.opts.IsHealthMonitorEnabled = false
			return errors.New("Did not retrieve all the expected capabilities")
		} else if !f.service.opts.IsHealthMonitorEnabled && count != 2 {
			return errors.New("Did not retrieve all the expected capabilities")
		}
		// Set default value
		f.service.opts.IsHealthMonitorEnabled = false
		return nil
	}
	return errors.New("Expected NodeGetCapabilitiesResponse but didn't get one")
}

func (f *feature) iHaveANodeWithAccessZone(nodeID string) error {
	f.accessZone = "CSI-" + nodeID
	return nil
}

func (f *feature) iCallControllerPublishVolumeWithTo(accessMode, nodeID string) error {
	header := metadata.New(map[string]string{"csi.requestid": "1"})
	ctx := metadata.NewIncomingContext(context.Background(), header)
	req := f.publishVolumeRequest
	if f.publishVolumeRequest == nil {
		req = f.getControllerPublishVolumeRequest(accessMode, nodeID)
		f.publishVolumeRequest = req
	}
	log.Printf("Calling controllerPublishVolume")
	f.publishVolumeResponse, f.err = f.service.ControllerPublishVolume(ctx, req)
	if f.err != nil {
		log.Printf("PublishVolume call failed: %s\n", f.err.Error())
	}
	f.publishVolumeRequest = nil
	return nil
}

func (f *feature) aValidControllerPublishVolumeResponseIsReturned() error {
	if f.err != nil {
		return errors.New("PublishVolume returned error: " + f.err.Error())
	}
	if f.publishVolumeResponse == nil {
		return errors.New("No PublishVolumeResponse returned")
	}
	for key, value := range f.publishVolumeResponse.PublishContext {
		fmt.Printf("PublishContext %s: %s", key, value)
	}
	return nil
}

func (f *feature) aValidControllerUnpublishVolumeResponseIsReturned() error {
	if f.err != nil {
		return errors.New("UnpublishVolume returned error: " + f.err.Error())
	}
	if f.unpublishVolumeResponse == nil {
		return errors.New("No UnpublishVolumeResponse returned")
	}
	return nil
}

func (f *feature) aValidNodeStageVolumeResponseIsReturned() error {
	if f.err != nil {
		return errors.New("NodeStageVolume returned error: " + f.err.Error())
	}
	if f.nodeStageVolumeResponse == nil {
		return errors.New("no NodeStageVolumeResponse is returned")
	}

	return nil
}

func (f *feature) aValidNodeUnstageVolumeResponseIsReturned() error {
	if f.err != nil {
		return errors.New("NodeUnstageVolume returned error: " + f.err.Error())
	}
	if f.nodeUnstageVolumeResponse == nil {
		return errors.New("no NodeUnstageVolumeResponse is returned")
	}
	return nil
}

func (f *feature) iCallNodeUnpublishVolume() error {
	req := f.nodeUnpublishVolumeRequest
	if req == nil {
		_ = f.getNodeUnpublishVolumeRequest()
		req = f.nodeUnpublishVolumeRequest
	}
	if inducedErrors.badVolumeIdentifier {
		req.VolumeId = "bad volume identifier"
	}
	fmt.Printf("Calling NodeUnPublishVolume\n")

	f.nodeUnpublishVolumeResponse, f.err = f.service.NodeUnpublishVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("NodePublishVolume call failed: %s\n", f.err.Error())
		if strings.Contains(f.err.Error(), "Target Path is required") {
			// Rollback for the future calls
			f.nodeUnpublishVolumeRequest.TargetPath = datadir
		}
	}
	if f.nodeUnpublishVolumeResponse != nil {
		err := os.RemoveAll(req.TargetPath)
		if err != nil {
			return nil
		}
		log.Printf("vol id %s\n", f.nodeUnpublishVolumeRequest.VolumeId)
	}
	return nil
}

func (f *feature) iCallEphemeralNodeUnpublishVolume() error {
	req := f.nodeUnpublishVolumeRequest
	if req == nil {
		_ = f.getNodeUnpublishVolumeRequest()
		req = f.nodeUnpublishVolumeRequest
	}
	if inducedErrors.badVolumeIdentifier {
		req.VolumeId = "bad volume identifier"
	}
	fmt.Printf("Calling NodePublishVolume\n")

	f.nodeUnpublishVolumeResponse, f.err = f.service.NodeUnpublishVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("NodePublishVolume call failed: %s\n", f.err.Error())
		if strings.Contains(f.err.Error(), "Target Path is required") {
			// Rollback for the future calls
			f.nodeUnpublishVolumeRequest.TargetPath = datadir
		}
	}
	if f.nodeUnpublishVolumeResponse != nil {
		err := os.RemoveAll(req.TargetPath)
		if err != nil {
			return nil
		}
		log.Printf("vol id %s\n", f.nodeUnpublishVolumeRequest.VolumeId)
	}
	return nil
}

func (f *feature) aValidNodeUnpublishVolumeResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *feature) getControllerPublishVolumeRequest(accessType, nodeID string) *csi.ControllerPublishVolumeRequest {
	capability := new(csi.VolumeCapability)

	mountVolume := new(csi.VolumeCapability_MountVolume)
	mountVolume.MountFlags = make([]string, 0)
	mount := new(csi.VolumeCapability_Mount)
	mount.Mount = mountVolume
	capability.AccessType = mount

	if !inducedErrors.omitAccessMode {
		capability.AccessMode = getAccessMode(accessType)
	}
	fmt.Printf("capability.AccessType %v\n", capability.AccessType)
	fmt.Printf("capability.AccessMode %v\n", capability.AccessMode)
	req := new(csi.ControllerPublishVolumeRequest)
	if !inducedErrors.noVolumeID {
		if inducedErrors.invalidVolumeID || f.createVolumeResponse == nil {
			req.VolumeId = "000-000"
		} else {
			req.VolumeId = "volume1=_=_=19=_=_=System"
		}
	}
	if !inducedErrors.noNodeID {
		req.NodeId = nodeID
	}
	req.Readonly = false
	if !inducedErrors.omitVolumeCapability {
		req.VolumeCapability = capability
	}
	// add in the context
	attributes := map[string]string{}
	attributes[AccessZoneParam] = f.accessZone
	if f.rootClientEnabled != "" {
		attributes[RootClientEnabledParam] = f.rootClientEnabled
	}
	req.VolumeContext = attributes
	return req
}

func (f *feature) getControllerUnPublishVolumeRequest(accessType, nodeID string) *csi.ControllerUnpublishVolumeRequest {
	capability := new(csi.VolumeCapability)

	mountVolume := new(csi.VolumeCapability_MountVolume)
	mountVolume.MountFlags = make([]string, 0)
	mount := new(csi.VolumeCapability_Mount)
	mount.Mount = mountVolume
	capability.AccessType = mount

	if !inducedErrors.omitAccessMode {
		capability.AccessMode = getAccessMode(accessType)
	}
	fmt.Printf("capability.AccessType %v\n", capability.AccessType)
	fmt.Printf("capability.AccessMode %v\n", capability.AccessMode)
	req := new(csi.ControllerUnpublishVolumeRequest)
	if !inducedErrors.noVolumeID {
		if inducedErrors.invalidVolumeID || f.createVolumeResponse == nil {
			req.VolumeId = "000-000"
		} else {
			req.VolumeId = "volume1=_=_=19=_=_=System"
		}
	}
	if !inducedErrors.noNodeID {
		req.NodeId = nodeID
	}
	// add in the context
	attributes := map[string]string{}
	attributes[AccessZoneParam] = f.accessZone
	return req
}

func (f *feature) aControllerPublishedVolume() error {
	var err error
	// Make the target directory if required
	_, err = os.Stat(datadir)
	if err != nil {
		err = os.MkdirAll(datadir, 0777)
		if err != nil {
			fmt.Printf("Couldn't make datadir: %s\n", datadir)
		}
	}

	// Make the target file if required
	_, err = os.Stat(datafile)
	if err != nil {
		file, err := os.Create(datafile)
		if err != nil {
			fmt.Printf("Couldn't make datafile: %s\n", datafile)
		} else {
			file.Close()
		}
	}

	// Empty WindowsMounts in gofsutil
	gofsutil.GOFSMockMounts = gofsutil.GOFSMockMounts[:0]
	return nil
}

func (f *feature) aCapabilityWithVoltypeAccess(voltype, access string) error {
	// Construct the volume capabilities
	capability := new(csi.VolumeCapability)
	switch voltype {
	case "block":
		blockVolume := new(csi.VolumeCapability_BlockVolume)
		block := new(csi.VolumeCapability_Block)
		block.Block = blockVolume
		capability.AccessType = block
	case "mount":
		mountVolume := new(csi.VolumeCapability_MountVolume)
		mountVolume.MountFlags = make([]string, 0)
		mount := new(csi.VolumeCapability_Mount)
		mount.Mount = mountVolume
		capability.AccessType = mount
	}
	accessMode := new(csi.VolumeCapability_AccessMode)
	accessMode.Mode = csi.VolumeCapability_AccessMode_UNKNOWN
	fmt.Printf("Access mode '%s'", access)
	switch access {
	case "single-reader":
		accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY
	case "single-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER
	case "single-node-single-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER
	case "single-node-multiple-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER
	case "multiple-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER
	case "multiple-reader":
		accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY
	case "multiple-node-single-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER
	}
	capability.AccessMode = accessMode
	f.capabilities = make([]*csi.VolumeCapability, 0)
	f.capabilities = append(f.capabilities, capability)
	f.capability = capability
	f.nodePublishVolumeRequest = nil
	return nil
}

func (f *feature) iCallNodePublishVolume() error {
	header := metadata.New(map[string]string{"csi.requestid": "1"})
	ctx := metadata.NewIncomingContext(context.Background(), header)
	req := f.nodePublishVolumeRequest
	if req == nil {
		_ = f.getNodePublishVolumeRequest()
		req = f.nodePublishVolumeRequest
	}
	if inducedErrors.badVolumeIdentifier {
		req.VolumeId = "bad volume identifier"
	}
	fmt.Printf("Calling NodePublishVolume\n")
	_, err := f.service.NodePublishVolume(ctx, req)
	if err != nil {
		fmt.Printf("NodePublishVolume failed: %s\n", err.Error())
		if f.err == nil {
			f.err = err
		}
	} else {
		fmt.Printf("NodePublishVolume completed successfully\n")
	}
	return nil
}

func (f *feature) iCallEphemeralNodePublishVolume() error {
	header := metadata.New(map[string]string{"csi.requestid": "1"})
	ctx := metadata.NewIncomingContext(context.Background(), header)
	req := f.nodePublishVolumeRequest
	if req == nil {
		_ = f.getNodePublishVolumeRequest()
		req = f.nodePublishVolumeRequest
	}
	f.nodePublishVolumeRequest.VolumeContext["csi.storage.k8s.io/ephemeral"] = "true"
	if inducedErrors.badVolumeIdentifier {
		req.VolumeId = "bad volume identifier"
	}
	fmt.Printf("Calling NodePublishVolume\n")
	_, err := f.service.NodePublishVolume(ctx, req)
	if err != nil {
		fmt.Printf("NodePublishVolume failed: %s\n", err.Error())
		if f.err == nil {
			f.err = err
		}
	} else {
		fmt.Printf("NodePublishVolume completed successfully\n")
	}
	return nil
}

func (f *feature) getNodePublishVolumeRequest() error {
	req := new(csi.NodePublishVolumeRequest)
	req.VolumeId = Volume1
	req.Readonly = false
	req.VolumeCapability = f.capability
	mount := f.capability.GetMount()
	if mount != nil {
		req.TargetPath = datadir
	}
	attributes := map[string]string{
		"Name":       req.VolumeId,
		"AccessZone": "",
		"Path":       f.service.opts.Path + "/" + req.VolumeId,
	}
	req.VolumeContext = attributes

	f.nodePublishVolumeRequest = req
	return nil
}

func (f *feature) getNodePublishVolumeRequestwithVolumeNameandPath(volName string, path string) error {
	req := new(csi.NodePublishVolumeRequest)
	req.VolumeId = volName
	req.Readonly = true
	req.VolumeCapability = f.capability
	mount := f.capability.GetMount()
	if mount != nil {
		req.TargetPath = datadir
	}
	attributes := map[string]string{
		"Name":       req.VolumeId,
		"AccessZone": "",
		"Path":       path,
	}
	req.VolumeContext = attributes

	f.nodePublishVolumeRequest = req
	return nil
}

func (f *feature) getNodePublishVolumeRequestwithVolumeName(volName string) error {
	req := new(csi.NodePublishVolumeRequest)
	req.VolumeId, _, _, _, _ = utils.ParseNormalizedVolumeID(context.Background(), volName)

	req.Readonly = false
	req.VolumeCapability = f.capability
	mount := f.capability.GetMount()
	if mount != nil {
		req.TargetPath = datadir
	}
	attributes := map[string]string{
		"Name":       req.VolumeId,
		"AccessZone": "",
		"Path":       f.service.opts.Path + "/" + req.VolumeId,
	}
	req.VolumeContext = attributes

	f.nodePublishVolumeRequest = req
	return nil
}

func (f *feature) getNodeUnpublishVolumeRequest() error {
	req := new(csi.NodeUnpublishVolumeRequest)
	req.VolumeId = Volume1
	req.TargetPath = datadir

	f.nodeUnpublishVolumeRequest = req
	return nil
}

func (f *feature) getNodeUnpublishVolumeRequestForROSnapshot(volName string, path string) error {
	req := new(csi.NodeUnpublishVolumeRequest)
	req.VolumeId = volName
	req.TargetPath = path

	f.nodeUnpublishVolumeRequest = req
	return nil
}

func (f *feature) iChangeTheTargetPath() error {
	// Make the target directory if required
	_, err := os.Stat(datadir2)
	if err != nil {
		err = os.MkdirAll(datadir2, 0777)
		if err != nil {
			fmt.Printf("Couldn't make datadir: %s\n", datadir2)
		}
	}

	// Make the target file if required
	_, err = os.Stat(datafile2)
	if err != nil {
		file, err := os.Create(datafile2)
		if err != nil {
			fmt.Printf("Couldn't make datafile: %s\n", datafile2)
		} else {
			file.Close()
		}
	}
	req := f.nodePublishVolumeRequest
	block := f.capability.GetBlock()
	if block != nil {
		req.TargetPath = datafile2
	}
	mount := f.capability.GetMount()
	if mount != nil {
		req.TargetPath = datadir2
	}
	return nil
}

func (f *feature) iMarkRequestReadOnly() error {
	f.nodePublishVolumeRequest.Readonly = true
	return nil
}

func (f *feature) iCallControllerPublishVolume(volID string, accessMode string, nodeID string) error {

	header := metadata.New(map[string]string{"csi.requestid": "1"})
	ctx := metadata.NewIncomingContext(context.Background(), header)
	req := f.publishVolumeRequest
	if f.publishVolumeRequest == nil {
		req = f.getControllerPublishVolumeRequest(accessMode, nodeID)
		f.publishVolumeRequest = req
	}

	// a customized volume ID can be specified to overwrite the default one
	if volID != "" {
		req.VolumeId = volID
	}

	log.Printf("Calling controllerPublishVolume")
	f.publishVolumeResponse, f.err = f.service.ControllerPublishVolume(ctx, req)
	if f.err != nil {
		log.Printf("PublishVolume call failed: %s\n", f.err.Error())
	}
	f.publishVolumeRequest = nil
	return nil
}
func (f *feature) iCallControllerGetVolume(volID string) error {

	header := metadata.New(map[string]string{"csi.requestid": "1"})
	ctx := metadata.NewIncomingContext(context.Background(), header)

	req := new(csi.ControllerGetVolumeRequest)
	req.VolumeId = volID

	abnormal := false
	message := ""
	f.controllerGetVolumeRequest = req
	fmt.Printf("Calling controllerGetVolume")
	f.controllerGetVolumeResponse, f.err = f.service.ControllerGetVolume(ctx, req)
	if f.err != nil {
		log.Printf("Controller GetVolume call failed: %s\n", f.err.Error())
	}
	if f.controllerGetVolumeResponse != nil {
		//check message and abnormal state returned in NodeGetVolumeStatsResponse.VolumeCondition
		if f.controllerGetVolumeResponse.Status.VolumeCondition.Abnormal == abnormal && strings.Contains(f.controllerGetVolumeResponse.Status.VolumeCondition.Message, message) {
			fmt.Printf("controllerGetVolumeResponse Response VolumeCondition check passed\n")
		} else {
			fmt.Printf("Expected controllerGetVolumeResponse.Abnormal to be %v, and message to contain: %s, but instead, abnormal was: %v and message was: %s", abnormal, message, f.controllerGetVolumeResponse.Status.VolumeCondition.Abnormal, f.controllerGetVolumeResponse.Status.VolumeCondition.Message)
		}
	}

	f.controllerGetVolumeRequest = nil
	return nil
}
func (f *feature) aValidControllerGetVolumeResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	fmt.Printf("The volume ID is %v\n", f.controllerGetVolumeResponse.Volume)
	fmt.Printf("The volume condition is '%s'\n", f.controllerGetVolumeResponse.Status)

	return nil
}

func (f *feature) iCallNodeGetVolumeStats(volID string) error {

	header := metadata.New(map[string]string{"csi.requestid": "1"})
	ctx := metadata.NewIncomingContext(context.Background(), header)

	req := new(csi.NodeGetVolumeStatsRequest)
	req.VolumeId = volID

	if inducedErrors.volumePathNotFound == true {
		req.VolumePath = ""
	} else {
		req.VolumePath = datadir
	}

	f.nodeGetVolumeStatsRequest = req
	fmt.Printf("Calling NodeGetVolumeStats")

	//assume no errors induced, so response should be okay, these values will change below if errors were induced
	abnormal := false
	message := ""

	f.nodeGetVolumeStatsResponse, f.err = f.service.NodeGetVolumeStats(ctx, req)
	if f.err != nil {
		log.Printf("Node GetVolumeStats call failed: %s\n", f.err.Error())
	}
	if f.nodeGetVolumeStatsResponse != nil {
		//check message and abnormal state returned in NodeGetVolumeStatsResponse.VolumeCondition
		if f.nodeGetVolumeStatsResponse.VolumeCondition.Abnormal == abnormal && strings.Contains(f.nodeGetVolumeStatsResponse.VolumeCondition.Message, message) {
			fmt.Printf("NodeGetVolumeStats Response VolumeCondition check passed\n")
		} else {
			fmt.Printf("Expected nodeGetVolumeStatsResponse.Abnormal to be %v, and message to contain: %s, but instead, abnormal was: %v and message was: %s", abnormal, message, f.nodeGetVolumeStatsResponse.VolumeCondition.Abnormal, f.nodeGetVolumeStatsResponse.VolumeCondition.Message)
		}
	}

	return nil
}

func (f *feature) aNodeGetVolumeResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	fmt.Printf("The volume condition is %v\n", f.nodeGetVolumeStatsResponse)

	return nil
}

func (f *feature) iCallControllerUnPublishVolume(volID string, accessMode string, nodeID string) error {
	req := f.getControllerUnPublishVolumeRequest(accessMode, nodeID)
	f.unpublishVolumeRequest = req

	// a customized volume ID can be specified to overwrite the default one
	req.VolumeId = volID
	f.unpublishVolumeResponse, f.err = f.service.ControllerUnpublishVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("ControllerUnPublishVolume call failed: %s\n", f.err.Error())
	}

	if f.unpublishVolumeResponse != nil {
		log.Printf("a unpublishVolumeResponse has been returned\n")
	}
	return nil
}

func (f *feature) iCallNodeStageVolume(volID string, accessType string) error {
	req := getTypicalNodeStageVolumeRequest(accessType)
	f.nodeStageVolumeRequest = req

	// a customized volume ID can be specified to overwrite the default one
	if volID != "" {
		req.VolumeId = volID
	}

	f.nodeStageVolumeResponse, f.err = f.service.NodeStageVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("NodeStageVolume call failed: %s\n", f.err.Error())
	}

	if f.nodeStageVolumeResponse != nil {
		log.Printf("a NodeStageVolumeResponse has been returned\n")
	}

	return nil
}

func (f *feature) iCallNodeUnstageVolume(volID string) error {
	req := getTypicalNodeUnstageVolumeRequest(volID)
	f.nodeUnstageVolumeRequest = req
	f.nodeUnstageVolumeResponse, f.err = f.service.NodeUnstageVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("NodeUnstageVolume call failed: %s\n", f.err.Error())
	}

	if f.nodeStageVolumeResponse != nil {
		log.Printf("a NodeUnstageVolumeResponse has been returned\n")
	}
	return nil
}

func (f *feature) iCallListVolumesWithMaxEntriesStartingToken(arg1 int, arg2 string) error {
	req := new(csi.ListVolumesRequest)
	//  The starting token is not valid
	if arg2 == "invalid" {
		stepHandlersErrors.StartingTokenInvalidError = true
	}
	req.MaxEntries = int32(arg1)
	req.StartingToken = arg2
	f.listVolumesResponse, f.err = f.service.ListVolumes(context.Background(), req)
	if f.err != nil {
		log.Printf("ListVolumes call failed: %s\n", f.err.Error())
		return nil
	}
	return nil
}

func (f *feature) aValidListVolumesResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	fmt.Printf("The volumes are %v\n", f.listVolumesResponse.Entries)
	fmt.Printf("The next token is '%s'\n", f.listVolumesResponse.NextToken)
	return nil
}

func (f *feature) iCallDeleteSnapshot(snapshotID string) error {
	req := new(csi.DeleteSnapshotRequest)
	req.SnapshotId = snapshotID
	f.deleteSnapshotRequest = req
	_, err := f.service.DeleteSnapshot(context.Background(), f.deleteSnapshotRequest)
	if err != nil {
		log.Printf("DeleteSnapshot call failed: %s\n", err.Error())
		f.err = err
		return nil
	}
	fmt.Printf("Delete snapshot successfully\n")
	return nil
}

func getCreateSnapshotRequest(srcVolumeID, name, isiPath string) *csi.CreateSnapshotRequest {
	req := new(csi.CreateSnapshotRequest)
	req.SourceVolumeId = srcVolumeID
	req.Name = name
	parameters := make(map[string]string)
	if isiPath != "none" {
		parameters[IsiPathParam] = isiPath
	}
	req.Parameters = parameters
	return req
}

func (f *feature) iCallCreateSnapshot(srcVolumeID, name, isiPath string) error {
	f.createSnapshotRequest = getCreateSnapshotRequest(srcVolumeID, name, isiPath)
	req := f.createSnapshotRequest

	f.createSnapshotResponse, f.err = f.service.CreateSnapshot(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateSnapshot call failed: %s\n", f.err.Error())
	}
	if f.createSnapshotResponse != nil {
		log.Printf("snapshot id %s\n", f.createSnapshotResponse.GetSnapshot().SnapshotId)
	}
	return nil
}

func (f *feature) aValidCreateSnapshotResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	f.snapshotIDList = append(f.snapshotIDList, f.createSnapshotResponse.Snapshot.SnapshotId)
	fmt.Printf("created snapshot id %s: source volume id %s, sizeInBytes %d, creation time %s\n",
		f.createSnapshotResponse.Snapshot.SnapshotId,
		f.createSnapshotResponse.Snapshot.SourceVolumeId,
		f.createSnapshotResponse.Snapshot.SizeBytes,
		f.createSnapshotResponse.Snapshot.CreationTime)
	return nil
}

func getControllerExpandVolumeRequest(volumeID string, requiredBytes int64) *csi.ControllerExpandVolumeRequest {
	return &csi.ControllerExpandVolumeRequest{
		VolumeId: volumeID,
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: requiredBytes,
			LimitBytes:    requiredBytes,
		},
	}
}

func (f *feature) iCallControllerExpandVolume(volumeID string, requiredBytes int64) error {
	log.Printf("###")
	f.controllerExpandVolumeRequest = getControllerExpandVolumeRequest(volumeID, requiredBytes)
	req := f.controllerExpandVolumeRequest

	ctx, log, _ := GetRunIDLog(context.Background())
	f.controllerExpandVolumeResponse, f.err = f.service.ControllerExpandVolume(ctx, req)
	if f.err != nil {
		log.Printf("ControllerExpandVolume call failed: %s\n", f.err.Error())
	}
	if f.controllerExpandVolumeResponse != nil {
		log.Printf("Volume capacity %d\n", f.controllerExpandVolumeResponse.CapacityBytes)
	}
	return nil
}

func (f *feature) aValidControllerExpandVolumeResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	if f.controllerExpandVolumeRequest.GetCapacityRange().GetRequiredBytes() <= f.controllerExpandVolumeResponse.CapacityBytes {
		fmt.Printf("Volume expansion succeeded\n")
		return nil
	}

	return fmt.Errorf("Volume expansion failed")
}

func (f *feature) setVolumeContent(isSnapshotType bool, identity string) *csi.CreateVolumeRequest {
	req := f.createVolumeRequest
	if isSnapshotType {

		req.VolumeContentSource = &csi.VolumeContentSource{
			Type: &csi.VolumeContentSource_Snapshot{
				Snapshot: &csi.VolumeContentSource_SnapshotSource{
					SnapshotId: identity,
				},
			},
		}
	} else {
		req.VolumeContentSource = &csi.VolumeContentSource{
			Type: &csi.VolumeContentSource_Volume{
				Volume: &csi.VolumeContentSource_VolumeSource{
					VolumeId: identity,
				},
			},
		}
	}

	return req
}

func (f *feature) iCallCreateVolumeFromSnapshot(srcSnapshotID, name string) error {
	req := getTypicalCreateVolumeRequest()
	f.createVolumeRequest = req
	req.Name = name
	req = f.setVolumeContent(true, srcSnapshotID)
	f.createVolumeResponse, f.err = f.service.CreateVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateVolume call failed: '%s'\n", f.err.Error())
	}
	if f.createVolumeResponse != nil {
		log.Printf("volume name '%s' created\n", name)
	}
	return nil
}

func (f *feature) iCallCreateVolumeFromVolume(srcVolumeName, name string) error {
	req := getTypicalCreateVolumeRequest()
	f.createVolumeRequest = req
	req.Name = name
	req = f.setVolumeContent(false, srcVolumeName)
	f.createVolumeResponse, f.err = f.service.CreateVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateVolume call failed: '%s'\n", f.err.Error())
	}
	if f.createVolumeResponse != nil {
		log.Printf("volume name '%s' created\n", name)
	}
	return nil
}

func (f *feature) iCallInitializeRealIsilonService() error {
	f.service.initializeServiceOpts(context.Background())
	return nil
}

func (f *feature) aIsilonServiceWithParams(user, mode string) error {
	f.checkGoRoutines("start aIsilonService")

	f.err = nil
	f.getPluginInfoResponse = nil
	f.volumeIDList = f.volumeIDList[:0]
	f.snapshotIDList = f.snapshotIDList[:0]

	// configure gofsutil; we use a mock interface
	gofsutil.UseMockFS()
	gofsutil.GOFSMock.InduceBindMountError = false
	gofsutil.GOFSMock.InduceMountError = false
	gofsutil.GOFSMock.InduceGetMountsError = false
	gofsutil.GOFSMock.InduceDevMountsError = false
	gofsutil.GOFSMock.InduceUnmountError = false
	gofsutil.GOFSMock.InduceFormatError = false
	gofsutil.GOFSMock.InduceGetDiskFormatError = false
	gofsutil.GOFSMock.InduceGetDiskFormatType = ""
	gofsutil.GOFSMockMounts = gofsutil.GOFSMockMounts[:0]

	// set induced errors
	inducedErrors.badVolumeIdentifier = false
	inducedErrors.invalidVolumeID = false
	inducedErrors.noVolumeID = false
	inducedErrors.differentVolumeID = false
	inducedErrors.noNodeName = false
	inducedErrors.noNodeID = false
	inducedErrors.omitVolumeCapability = false
	inducedErrors.omitAccessMode = false

	// initialize volume and export existence status
	stepHandlersErrors.ExportNotFoundError = true
	stepHandlersErrors.VolumeNotExistError = true

	// Get the httptest mock handler. Only set
	// a new server if there isn't one already.
	handler := getHandler()
	// Get or reuse the cached service
	f.getServiceWithParams(user, mode)
	clusterConfig := f.service.getIsilonClusterConfig(clusterName1)
	if handler != nil && os.Getenv("CSI_ISILON_ENDPOINT") == "" {
		if f.server == nil {
			f.server = httptest.NewServer(handler)
		}
		log.Printf("server url: %s\n", f.server.URL)
		clusterConfig.EndpointURL = f.server.URL
	} else {
		f.server = nil
	}
	isiSvc, _ := f.service.GetIsiService(context.Background(), clusterConfig, logLevel)
	updatedClusterConfig, _ := f.service.isiClusters.Load(clusterName1)
	updatedClusterConfig.(*IsilonClusterConfig).isiSvc = isiSvc
	f.service.isiClusters.Store(clusterName1, updatedClusterConfig)
	f.checkGoRoutines("end aIsilonService")
	f.service.logServiceStats()
	if inducedErrors.noIsiService || inducedErrors.autoProbeNotEnabled {
		updatedClusterConfig, _ := f.service.isiClusters.Load(clusterName1)
		updatedClusterConfig.(*IsilonClusterConfig).isiSvc = nil
		f.service.isiClusters.Store(clusterName1, updatedClusterConfig)
	}
	return nil
}

func (f *feature) aIsilonservicewithIsiAuthTypeassessionbased() error {
	f.checkGoRoutines("start aIsilonService")

	f.err = nil
	f.getPluginInfoResponse = nil
	f.volumeIDList = f.volumeIDList[:0]
	f.snapshotIDList = f.snapshotIDList[:0]

	// configure gofsutil; we use a mock interface
	gofsutil.UseMockFS()
	gofsutil.GOFSMock.InduceBindMountError = false
	gofsutil.GOFSMock.InduceMountError = false
	gofsutil.GOFSMock.InduceGetMountsError = false
	gofsutil.GOFSMock.InduceDevMountsError = false
	gofsutil.GOFSMock.InduceUnmountError = false
	gofsutil.GOFSMock.InduceFormatError = false
	gofsutil.GOFSMock.InduceGetDiskFormatError = false
	gofsutil.GOFSMock.InduceGetDiskFormatType = ""
	gofsutil.GOFSMockMounts = gofsutil.GOFSMockMounts[:0]

	// set induced errors
	inducedErrors.badVolumeIdentifier = false
	inducedErrors.invalidVolumeID = false
	inducedErrors.noVolumeID = false
	inducedErrors.differentVolumeID = false
	inducedErrors.noNodeName = false
	inducedErrors.noNodeID = false
	inducedErrors.omitVolumeCapability = false
	inducedErrors.omitAccessMode = false

	// initialize volume and export existence status
	stepHandlersErrors.ExportNotFoundError = true
	stepHandlersErrors.VolumeNotExistError = true

	// Get the httptest mock handler. Only set
	// a new server if there isn't one already.
	handler := getHandler()
	// Get or reuse the cached service
	f.getServiceWithsessionauth()
	clusterConfig := f.service.getIsilonClusterConfig(clusterName1)
	if handler != nil && os.Getenv("CSI_ISILON_ENDPOINT") == "" {
		if f.server == nil {
			f.server = httptest.NewServer(handler)
		}
		log.Printf("server url: %s\n", f.server.URL)
		clusterConfig.EndpointURL = f.server.URL
	} else {
		f.server = nil
	}
	isiSvc, _ := f.service.GetIsiService(context.Background(), clusterConfig, logLevel)
	updatedClusterConfig, _ := f.service.isiClusters.Load(clusterName1)
	updatedClusterConfig.(*IsilonClusterConfig).isiSvc = isiSvc
	f.service.isiClusters.Store(clusterName1, updatedClusterConfig)
	f.checkGoRoutines("end aIsilonService")
	f.service.logServiceStats()
	if inducedErrors.noIsiService || inducedErrors.autoProbeNotEnabled {
		updatedClusterConfig, _ := f.service.isiClusters.Load(clusterName1)
		updatedClusterConfig.(*IsilonClusterConfig).isiSvc = nil
		f.service.isiClusters.Store(clusterName1, updatedClusterConfig)
	}
	return nil
}

func (f *feature) aIsilonServiceWithParamsForCustomTopology(user, mode string) error {
	f.checkGoRoutines("start aIsilonService")

	f.err = nil
	f.getPluginInfoResponse = nil
	f.volumeIDList = f.volumeIDList[:0]
	f.snapshotIDList = f.snapshotIDList[:0]

	// configure gofsutil; we use a mock interface
	gofsutil.UseMockFS()
	gofsutil.GOFSMock.InduceBindMountError = false
	gofsutil.GOFSMock.InduceMountError = false
	gofsutil.GOFSMock.InduceGetMountsError = false
	gofsutil.GOFSMock.InduceDevMountsError = false
	gofsutil.GOFSMock.InduceUnmountError = false
	gofsutil.GOFSMock.InduceFormatError = false
	gofsutil.GOFSMock.InduceGetDiskFormatError = false
	gofsutil.GOFSMock.InduceGetDiskFormatType = ""
	gofsutil.GOFSMockMounts = gofsutil.GOFSMockMounts[:0]

	// set induced errors
	inducedErrors.badVolumeIdentifier = false
	inducedErrors.invalidVolumeID = false
	inducedErrors.noVolumeID = false
	inducedErrors.differentVolumeID = false
	inducedErrors.noNodeName = false
	inducedErrors.noNodeID = false
	inducedErrors.omitVolumeCapability = false
	inducedErrors.omitAccessMode = false

	// initialize volume and export existence status
	stepHandlersErrors.ExportNotFoundError = true
	stepHandlersErrors.VolumeNotExistError = true

	// Get the httptest mock handler. Only set
	// a new server if there isn't one already.
	handler := getHandler()
	// Get or reuse the cached service
	f.getServiceWithParamsForCustomTopology(user, mode, true)
	clusterConfig := f.service.getIsilonClusterConfig(clusterName1)
	if handler != nil && os.Getenv("CSI_ISILON_ENDPOINT") == "" {
		if f.server == nil {
			f.server = httptest.NewServer(handler)
		}
		log.Printf("server url: %s\n", f.server.URL)
		clusterConfig.EndpointURL = f.server.URL
		urlList := strings.Split(f.server.URL, ":")
		log.Printf("urlList: %v", urlList)
		clusterConfig.EndpointPort = urlList[2]
	} else {
		f.server = nil
	}
	isiSvc, err := f.service.GetIsiService(context.Background(), clusterConfig, logLevel)
	f.err = err
	updatedClusterConfig, _ := f.service.isiClusters.Load(clusterName1)
	updatedClusterConfig.(*IsilonClusterConfig).isiSvc = isiSvc
	f.service.isiClusters.Store(clusterName1, updatedClusterConfig)
	f.checkGoRoutines("end aIsilonService")
	f.service.logServiceStats()
	if inducedErrors.noIsiService || inducedErrors.autoProbeNotEnabled {
		updatedClusterConfig, _ := f.service.isiClusters.Load(clusterName1)
		updatedClusterConfig.(*IsilonClusterConfig).isiSvc = nil
		f.service.isiClusters.Store(clusterName1, updatedClusterConfig)
	}
	return nil
}

func (f *feature) aIsilonServiceWithParamsForCustomTopologyNoLabel(user, mode string) error {
	f.checkGoRoutines("start aIsilonService")

	f.err = nil
	f.getPluginInfoResponse = nil
	f.volumeIDList = f.volumeIDList[:0]
	f.snapshotIDList = f.snapshotIDList[:0]

	// configure gofsutil; we use a mock interface
	gofsutil.UseMockFS()
	gofsutil.GOFSMock.InduceBindMountError = false
	gofsutil.GOFSMock.InduceMountError = false
	gofsutil.GOFSMock.InduceGetMountsError = false
	gofsutil.GOFSMock.InduceDevMountsError = false
	gofsutil.GOFSMock.InduceUnmountError = false
	gofsutil.GOFSMock.InduceFormatError = false
	gofsutil.GOFSMock.InduceGetDiskFormatError = false
	gofsutil.GOFSMock.InduceGetDiskFormatType = ""
	gofsutil.GOFSMockMounts = gofsutil.GOFSMockMounts[:0]

	// set induced errors
	inducedErrors.badVolumeIdentifier = false
	inducedErrors.invalidVolumeID = false
	inducedErrors.noVolumeID = false
	inducedErrors.differentVolumeID = false
	inducedErrors.noNodeName = false
	inducedErrors.noNodeID = false
	inducedErrors.omitVolumeCapability = false
	inducedErrors.omitAccessMode = false

	// initialize volume and export existence status
	stepHandlersErrors.ExportNotFoundError = true
	stepHandlersErrors.VolumeNotExistError = true

	// Get the httptest mock handler. Only set
	// a new server if there isn't one already.
	handler := getHandler()
	// Get or reuse the cached service
	f.getServiceWithParamsForCustomTopology(user, mode, false)
	clusterConfig := f.service.getIsilonClusterConfig(clusterName1)
	if handler != nil && os.Getenv("CSI_ISILON_ENDPOINT") == "" {
		if f.server == nil {
			f.server = httptest.NewServer(handler)
		}
		log.Printf("server url: %s\n", f.server.URL)
		clusterConfig.EndpointURL = f.server.URL
		urlList := strings.Split(f.server.URL, ":")
		log.Printf("urlList: %v", urlList)
		clusterConfig.EndpointPort = urlList[2]
	} else {
		f.server = nil
	}
	isiSvc, _ := f.service.GetIsiService(context.Background(), clusterConfig, logLevel)
	updatedClusterConfig, _ := f.service.isiClusters.Load(clusterName1)
	updatedClusterConfig.(*IsilonClusterConfig).isiSvc = isiSvc
	f.service.isiClusters.Store(clusterName1, updatedClusterConfig)
	f.checkGoRoutines("end aIsilonService")
	f.service.logServiceStats()
	if inducedErrors.noIsiService || inducedErrors.autoProbeNotEnabled {
		updatedClusterConfig, _ := f.service.isiClusters.Load(clusterName1)
		updatedClusterConfig.(*IsilonClusterConfig).isiSvc = nil
		f.service.isiClusters.Store(clusterName1, updatedClusterConfig)
	}
	return nil
}

func removeNodeLabels(host string) (result bool) {
	mockStr := fmt.Sprintf("mocked call to remove labels on %s ", host)
	k8s.DeleteK8sValuesFile()
	fmt.Printf(mockStr)
	return true
}

func applyNodeLabel(host, label string) (result bool) {
	//don't need to run actual kubernetes commands for UTs
	//expect kubernetes commands to work
	mockStr := fmt.Sprintf("mocked call apply lable %s to %s", label, host)
	k8s.WriteK8sValueToFile(k8s.K8sLabel, label)
	fmt.Printf(mockStr)

	return true
}

func (f *feature) iCallApplyNodeLabel(nodeLabel string) error {
	host, _ := os.Hostname()
	if !applyNodeLabel(host, nodeLabel) {
		return fmt.Errorf("failed to create node lable '%s'", nodeLabel)
	}
	return nil
}

func (f *feature) iCallRemoveNodeLabels() error {
	host, _ := os.Hostname()
	if !removeNodeLabels(host) {
		return fmt.Errorf("failed to remove node lables")
	}
	return nil
}

func (f *feature) getServiceWithParamsForCustomTopology(user, mode string, applyLabel bool) *service {
	testControllerHasNoConnection = false
	testNodeHasNoConnection = false
	svc := new(service)
	var opts Opts

	opts.AccessZone = "System"
	opts.Path = "/ifs/data/csi-isilon"
	opts.SkipCertificateValidation = true
	opts.isiAuthType = 0
	opts.Verbose = 1
	opts.CustomTopologyEnabled = true
	pwd, _ := os.Getwd()
	pwd = "--" + pwd + "--"
	opts.KubeConfigPath = "mock/k8s/admin.conf"
	newConfig := IsilonClusterConfig{}
	newConfig.ClusterName = clusterName1
	newConfig.Endpoint = "127.0.0.1"
	newConfig.EndpointPort = "8080"
	newConfig.EndpointURL = "http://127.0.0.1"
	newConfig.User = user
	newConfig.Password = "blah"
	newConfig.SkipCertificateValidation = &opts.SkipCertificateValidation
	newConfig.IsiPath = "/ifs/data/csi-isilon"
	boolTrue := true
	newConfig.IsDefault = &boolTrue
	host, _ := os.Hostname()
	result := removeNodeLabels(host)
	if !result {
		log.Fatal("Setting custom topology failed")
	}

	if applyLabel {
		label := "csi-isilon.dellemc.com/127.0.0.1=csi-isilon.dellemc.com"
		result = applyNodeLabel(host, label)
		if !result {
			log.Fatalf("Applying '%s' label on node failed", label)
		}
	}

	if inducedErrors.autoProbeNotEnabled {
		opts.AutoProbe = false
	} else {
		opts.AutoProbe = true
	}
	svc.opts = opts
	svc.mode = mode
	f.service = svc
	f.service.nodeID = host
	// TODO - IP has to be updated before release
	f.service.nodeIP = "127.0.0.1"
	f.service.defaultIsiClusterName = clusterName1
	f.service.isiClusters = new(sync.Map)
	f.service.isiClusters.Store(newConfig.ClusterName, &newConfig)
	return svc
}

func (f *feature) getServiceWithParams(user, mode string) *service {
	testControllerHasNoConnection = false
	testNodeHasNoConnection = false
	svc := new(service)
	var opts Opts
	opts.AccessZone = "System"
	opts.Path = "/ifs/data/csi-isilon"
	opts.SkipCertificateValidation = true
	opts.isiAuthType = 0
	opts.Verbose = 1

	newConfig := IsilonClusterConfig{}
	newConfig.ClusterName = clusterName1
	newConfig.Endpoint = "127.0.0.1"
	newConfig.EndpointPort = "8080"
	newConfig.EndpointURL = "http://127.0.0.1"
	newConfig.User = user
	newConfig.Password = "blah"
	newConfig.SkipCertificateValidation = &opts.SkipCertificateValidation
	newConfig.IsiPath = "/ifs/data/csi-isilon"
	boolTrue := true
	newConfig.IsDefault = &boolTrue

	if inducedErrors.autoProbeNotEnabled {
		opts.AutoProbe = false
	} else {
		opts.AutoProbe = true
	}
	svc.opts = opts
	svc.mode = mode
	f.service = svc
	f.service.nodeID, _ = os.Hostname()
	f.service.nodeIP = "127.0.0.1"
	f.service.defaultIsiClusterName = clusterName1
	f.service.isiClusters = new(sync.Map)
	f.service.isiClusters.Store(newConfig.ClusterName, &newConfig)
	return svc
}

func (f *feature) getServiceWithsessionauth() *service {
	testControllerHasNoConnection = false
	testNodeHasNoConnection = false
	svc := new(service)
	var opts Opts
	opts.AccessZone = "System"
	opts.Path = "/ifs/data/csi-isilon"
	opts.SkipCertificateValidation = true
	opts.isiAuthType = 1
	opts.Verbose = 1

	newConfig := IsilonClusterConfig{}
	newConfig.ClusterName = clusterName1
	newConfig.Endpoint = "127.0.0.1"
	newConfig.EndpointPort = "8080"
	newConfig.EndpointURL = "http://127.0.0.1"
	newConfig.User = "blah"
	newConfig.Password = "blah"
	newConfig.SkipCertificateValidation = &opts.SkipCertificateValidation
	newConfig.IsiPath = "/ifs/data/csi-isilon"
	boolTrue := false
	newConfig.IsDefault = &boolTrue

	if inducedErrors.autoProbeNotEnabled {
		opts.AutoProbe = false
	} else {
		opts.AutoProbe = true
	}
	svc.opts = opts
	svc.mode = "controller"
	f.service = svc
	f.service.nodeID, _ = os.Hostname()
	f.service.nodeIP = "127.0.0.1"
	f.service.defaultIsiClusterName = clusterName1
	f.service.isiClusters = new(sync.Map)
	f.service.isiClusters.Store(newConfig.ClusterName, &newConfig)
	return svc
}

func (f *feature) iCallLogStatisticsTimes(times int) error {
	for i := 0; i < times; i++ {
		f.service.logStatistics()
	}
	return nil
}

func (f *feature) iCallBeforeServe() error {
	sp := new(gocsi.StoragePlugin)
	var lis net.Listener
	f.err = f.service.BeforeServe(context.Background(), sp, lis)
	return nil
}

func (f *feature) ICallCreateQuotaInIsiServiceWithNegativeSizeInBytes() error {
	clusterConfig := f.service.getIsilonClusterConfig(clusterName1)
	ctx, _, _ := GetRunIDLog(context.Background())
	_, f.err = clusterConfig.isiSvc.CreateQuota(ctx, f.service.opts.Path, "volume1", -1, true)
	return nil
}

func (f *feature) iCallGetExportRelatedFunctionsInIsiService() error {
	clusterConfig := f.service.getIsilonClusterConfig(clusterName1)
	ctx, _, _ := GetRunIDLog(context.Background())
	_, f.err = clusterConfig.isiSvc.GetExports(ctx)
	_, f.err = clusterConfig.isiSvc.GetExportByIDWithZone(ctx, 557, "System")
	f.err = clusterConfig.isiSvc.DeleteQuotaByExportIDWithZone(ctx, "volume1", 557, "System")
	_, _, f.err = clusterConfig.isiSvc.GetExportsWithLimit(ctx, "2")
	return nil
}

func (f *feature) iCallUnimplementedFunctions() error {
	_, f.err = f.service.ListSnapshots(context.Background(), new(csi.ListSnapshotsRequest))
	_, f.err = f.service.NodeUnstageVolume(context.Background(), new(csi.NodeUnstageVolumeRequest))
	_, f.err = f.service.NodeStageVolume(context.Background(), new(csi.NodeStageVolumeRequest))
	_, f.err = f.service.ListVolumes(context.Background(), new(csi.ListVolumesRequest))
	_, f.err = f.service.NodeExpandVolume(context.Background(), new(csi.NodeExpandVolumeRequest))
	return nil
}

func (f *feature) iCallInitServiceObject() error {
	service := New()
	if service == nil {
		f.err = errors.New("failed to initialize Service object")
	} else {
		f.err = nil
	}
	return nil
}

func (f *feature) iCallSetAllowedNetworks(envIP1 string) error {
	var envIP = []string{envIP1}
	f.service.opts.allowedNetworks = envIP
	return nil
}

func (f *feature) iCallSetAllowedNetworkswithmultiplenetworks(envIP1 string, envIP2 string) error {
	var envIP = []string{envIP1, envIP2}
	f.service.opts.allowedNetworks = envIP
	return nil
}

func (f *feature) iCallNodeGetInfowithinvalidnetworks() error {
	MockK8sAPI()
	req := new(csi.NodeGetInfoRequest)
	f.nodeGetInfoResponse, f.err = f.service.NodeGetInfo(context.Background(), req)
	if f.err != nil {
		log.Printf("NodeGetInfo call failed: %s\n", f.err.Error())
		return nil
	}
	return nil
}
func (f *feature) iSetRootClientEnabledTo(val string) error {
	f.rootClientEnabled = val
	return nil
}

func getCreateRemoteVolumeRequest(s *service) *csiext.CreateRemoteVolumeRequest {
	req := new(csiext.CreateRemoteVolumeRequest)
	req.VolumeHandle = "volume1=_=_=19=_=_=System"
	parameters := make(map[string]string)
	parameters[constants.EnvReplicationPrefix+"/"+KeyReplicationRemoteSystem] = ""
	parameters[s.WithRP(KeyReplicationRemoteSystem)] = "cluster1"
	req.Parameters = parameters
	return req
}

func (f *feature) iCallCreateRemoteVolume() error {
	req := getCreateRemoteVolumeRequest(f.service)
	f.createRemoteVolumeRequest = req
	f.createRemoteVolumeResponse, f.err = f.service.CreateRemoteVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateRemoteVolume call failed: %s\n", f.err.Error())
	}
	if f.createRemoteVolumeResponse != nil {
		stepHandlersErrors.ExportNotFoundError = false
		stepHandlersErrors.VolumeNotExistError = false
	}
	return nil
}

func getCreateRemoteVolumeRequestWithParams(s *service, volhand string, keyreplremsys string) *csiext.CreateRemoteVolumeRequest {
	req := new(csiext.CreateRemoteVolumeRequest)
	req.VolumeHandle = volhand
	parameters := make(map[string]string)
	parameters[s.WithRP(keyreplremsys)] = "cluster1"
	req.Parameters = parameters
	return req
}

func (f *feature) iCallCreateRemoteVolumeWithParams(volhand string, keyreplremsys string) error {
	req := getCreateRemoteVolumeRequestWithParams(f.service, volhand, keyreplremsys)
	f.createRemoteVolumeRequest = req
	f.createRemoteVolumeResponse, f.err = f.service.CreateRemoteVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateRemoteVolume call failed: %s\n", f.err.Error())
	}
	if f.createRemoteVolumeResponse != nil {
		stepHandlersErrors.ExportNotFoundError = false
		stepHandlersErrors.VolumeNotExistError = false
	}
	return nil
}

func (f *feature) aValidCreateRemoteVolumeResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	f.volumeIDList = append(f.volumeIDList, f.createRemoteVolumeResponse.RemoteVolume.VolumeId)
	fmt.Printf("volume '%s'\n",
		f.createRemoteVolumeResponse.RemoteVolume.VolumeContext["Name"])
	return nil
}

func getCreateStorageProtectionGroupRequest(s *service) *csiext.CreateStorageProtectionGroupRequest {
	req := new(csiext.CreateStorageProtectionGroupRequest)
	req.VolumeHandle = "volume1=_=_=19=_=_=System"
	parameters := make(map[string]string)
	parameters[s.WithRP(KeyReplicationRemoteSystem)] = "cluster1"
	req.Parameters = parameters
	return req
}

func (f *feature) iCallCreateStorageProtectionGroup() error {
	req := getCreateStorageProtectionGroupRequest(f.service)
	f.createStorageProtectionGroupRequest = req
	f.createStorageProtectionGroupResponse, f.err = f.service.CreateStorageProtectionGroup(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateStorageProtectionGroup call failed: %s\n", f.err.Error())
	}
	return nil
}

func getCreateStorageProtectionGroupRequestWithParams(s *service, volhand string, keyreplremsys string) *csiext.CreateStorageProtectionGroupRequest {
	req := new(csiext.CreateStorageProtectionGroupRequest)
	req.VolumeHandle = volhand
	parameters := make(map[string]string)
	parameters[keyreplremsys] = "cluster1"
	req.Parameters = parameters
	return req
}

func (f *feature) iCallCreateStorageProtectionGroupWithParams(volhand string, keyreplremsys string) error {
	req := getCreateStorageProtectionGroupRequestWithParams(f.service, volhand, keyreplremsys)
	f.createStorageProtectionGroupRequest = req
	f.createStorageProtectionGroupResponse, f.err = f.service.CreateStorageProtectionGroup(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateStorageProtectionGroup call failed: %s\n", f.err.Error())
	}
	return nil
}

func (f *feature) aValidCreateStorageProtectionGroupResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	return nil
}

func deleteStorageProtectionGroupRequest(s *service, volume, systemName, clustername string) *csiext.DeleteStorageProtectionGroupRequest {

	req := new(csiext.DeleteStorageProtectionGroupRequest)

	//req.ProtectionGroupId = "cluster1" + "::" + "/ifs/data/csi-isilon" + volume
	req.ProtectionGroupId = volume
	req.ProtectionGroupAttributes = map[string]string{
		s.opts.replicationContextPrefix + systemName: clustername,
	}
	return req
}

func (f *feature) iCallStorageProtectionGroupDelete(volume, systemName, clustername string) error {
	req := deleteStorageProtectionGroupRequest(f.service, volume, systemName, clustername)
	f.deleteStorageProtectionGroupRequest = req
	f.deleteStorageProtectionGroupResponse, f.err = f.service.DeleteStorageProtectionGroup(context.Background(), req)
	if f.err != nil {
		log.Printf("DeleteStorageProtectionGroup call failed: %s\n", f.err.Error())
	}
	return nil
}

func (f *feature) aValidDeleteStorageProtectionGroupResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *feature) iCallNodeGetInfoWithNoFQDN() error {
	req := new(csi.NodeGetInfoRequest)
	f.service.nodeIP = "192.0.2.0"
	f.nodeGetInfoResponse, f.err = f.service.NodeGetInfo(context.Background(), req)
	if f.err != nil {
		log.Printf("NodeGetInfo call failed: %s\n", f.err.Error())
		return f.err
	}
	return nil
}
func getStorageProtectionGroupStatusRequest(s *service) *csiext.GetStorageProtectionGroupStatusRequest {
	req := new(csiext.GetStorageProtectionGroupStatusRequest)
	req.ProtectionGroupId = ""
	req.ProtectionGroupAttributes = map[string]string{
		s.opts.replicationContextPrefix + "systemName":       "cluster1",
		s.opts.replicationContextPrefix + "remoteSystemName": "cluster1",
		s.opts.replicationContextPrefix + "VolumeGroupName":  "csi-prov-test-19743d82-192-168-111-25-Five_Minutes",
	}
	return req
}

func (f *feature) iCallGetStorageProtectionGroupStatus() error {
	req := getStorageProtectionGroupStatusRequest(f.service)
	f.getStorageProtectionGroupStatusRequest = req
	f.getStorageProtectionGroupStatusResponse, f.err = f.service.GetStorageProtectionGroupStatus(context.Background(), req)
	if f.err != nil {
		log.Printf("GetStorageProtectionGroupStatus call failed: %s\n", f.err.Error())
	}
	return nil
}

func (f *feature) aValidGetStorageProtectionGroupStatusResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *feature) iCallGetReplicationCapabilities() error {
	req := new(csiext.GetReplicationCapabilityRequest)
	f.getReplicationCapabilityResponse, f.err = f.service.GetReplicationCapabilities(context.Background(), req)
	if f.err != nil {
		log.Printf("GetReplicationCapabilities call failed: %s\n", f.err.Error())
		return f.err
	}
	return nil
}

func getStorageProtectionGroupStatusRequestWithParams(s *service, id, localSystemName, remoteSystemName, vgname, clustername1, clustername2 string) *csiext.GetStorageProtectionGroupStatusRequest {
	req := new(csiext.GetStorageProtectionGroupStatusRequest)
	req.ProtectionGroupId = id
	req.ProtectionGroupAttributes = map[string]string{
		s.opts.replicationContextPrefix + localSystemName:  clustername1,
		s.opts.replicationContextPrefix + remoteSystemName: clustername2,
		s.opts.replicationContextPrefix + vgname:           "csi-prov-test-19743d82-192-168-111-25-Five_Minutes",
	}
	return req
}

func (f *feature) iCallGetStorageProtectionGroupStatusWithParams(id, localSystemName, remoteSystemName, vgname, clustername1, clustername2 string) error {
	req := getStorageProtectionGroupStatusRequestWithParams(f.service, id, localSystemName, remoteSystemName, vgname, clustername1, clustername2)
	f.getStorageProtectionGroupStatusRequest = req
	f.getStorageProtectionGroupStatusResponse, f.err = f.service.GetStorageProtectionGroupStatus(context.Background(), req)
	if f.err != nil {
		log.Printf("GetStorageProtectionGroupStatus call failed: %s\n", f.err.Error())
	}
	return nil
}

func executeActionRequest(s *service, systemName, clusterNameOne, clusterNameTwo, remoteSystemName, vgname, ppname string) *csiext.ExecuteActionRequest {
	action := &csiext.Action{
		ActionTypes: csiext.ActionTypes_RESUME,
	}
	params := map[string]string{
		s.opts.replicationContextPrefix + systemName:       clusterNameOne,
		s.opts.replicationContextPrefix + remoteSystemName: clusterNameTwo,
		s.opts.replicationContextPrefix + vgname:           ppname,
	}
	req := &csiext.ExecuteActionRequest{
		ActionId:                        "",
		ProtectionGroupId:               "",
		ActionTypes:                     &csiext.ExecuteActionRequest_Action{Action: action},
		ProtectionGroupAttributes:       params,
		RemoteProtectionGroupId:         "",
		RemoteProtectionGroupAttributes: nil,
	}

	return req
}

func (f *feature) iCallExecuteAction(systemName, clusterNameOne, clusterNameTwo, remoteSystemName, vgname, ppname string) error {
	req := executeActionRequest(f.service, systemName, clusterNameOne, clusterNameTwo, remoteSystemName, vgname, ppname)
	f.executeActionRequest = req
	f.executeActionResponse, f.err = f.service.ExecuteAction(context.Background(), req)
	if f.err != nil {
		log.Printf("ExecuteAction call failed: %s\n", f.err.Error())
	}
	return nil
}

func (f *feature) aValidExecuteActionResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	return nil
}

func executeActionRequestSuspend(s *service) *csiext.ExecuteActionRequest {
	action := &csiext.Action{
		ActionTypes: csiext.ActionTypes_SUSPEND,
	}
	params := map[string]string{
		s.opts.replicationContextPrefix + "systemName":       "cluster1",
		s.opts.replicationContextPrefix + "remoteSystemName": "cluster1",
		s.opts.replicationContextPrefix + "VolumeGroupName":  "csi-prov-test-19743d82-192-168-111-25-Five_Minutes",
	}
	req := &csiext.ExecuteActionRequest{
		ActionId:                        "",
		ProtectionGroupId:               "",
		ActionTypes:                     &csiext.ExecuteActionRequest_Action{Action: action},
		ProtectionGroupAttributes:       params,
		RemoteProtectionGroupId:         "",
		RemoteProtectionGroupAttributes: nil,
	}

	return req
}

func (f *feature) iCallExecuteActionSuspend() error {
	req := executeActionRequestSuspend(f.service)
	f.executeActionRequest = req
	f.executeActionResponse, f.err = f.service.ExecuteAction(context.Background(), req)
	if f.err != nil {
		log.Printf("ExecuteAction call failed: %s\n", f.err.Error())
	}
	return nil
}

func executeActionRequestReprotect(s *service) *csiext.ExecuteActionRequest {
	action := &csiext.Action{
		ActionTypes: csiext.ActionTypes_REPROTECT_LOCAL,
	}
	params := map[string]string{
		s.opts.replicationContextPrefix + "systemName":       "cluster1",
		s.opts.replicationContextPrefix + "remoteSystemName": "cluster1",
		s.opts.replicationContextPrefix + "VolumeGroupName":  "csi-prov-test-19743d82-192-168-111-25-Five_Minutes",
	}
	req := &csiext.ExecuteActionRequest{
		ActionId:                        "",
		ProtectionGroupId:               "",
		ActionTypes:                     &csiext.ExecuteActionRequest_Action{Action: action},
		ProtectionGroupAttributes:       params,
		RemoteProtectionGroupId:         "",
		RemoteProtectionGroupAttributes: nil,
	}

	return req
}

func (f *feature) iCallExecuteActionReprotect() error {
	req := executeActionRequestReprotect(f.service)
	f.executeActionRequest = req
	f.executeActionResponse, f.err = f.service.ExecuteAction(context.Background(), req)
	if f.err != nil {
		log.Printf("ExecuteAction call failed: %s\n", f.err.Error())
	}
	return nil
}

func executeActionRequestSync(s *service) *csiext.ExecuteActionRequest {
	action := &csiext.Action{
		ActionTypes: csiext.ActionTypes_SYNC,
	}
	params := map[string]string{
		s.opts.replicationContextPrefix + "systemName":       "cluster1",
		s.opts.replicationContextPrefix + "remoteSystemName": "cluster1",
		s.opts.replicationContextPrefix + "VolumeGroupName":  "csi-prov-test-19743d82-192-168-111-25-Five_Minutes",
	}
	req := &csiext.ExecuteActionRequest{
		ActionId:                        "",
		ProtectionGroupId:               "",
		ActionTypes:                     &csiext.ExecuteActionRequest_Action{Action: action},
		ProtectionGroupAttributes:       params,
		RemoteProtectionGroupId:         "",
		RemoteProtectionGroupAttributes: nil,
	}

	return req
}

func (f *feature) iCallExecuteActionSync() error {
	req := executeActionRequestSync(f.service)
	f.executeActionRequest = req
	f.executeActionResponse, f.err = f.service.ExecuteAction(context.Background(), req)
	if f.err != nil {
		log.Printf("ExecuteAction call failed: %s\n", f.err.Error())
	}
	return nil
}

func executeActionRequestFailover(s *service) *csiext.ExecuteActionRequest {
	action := &csiext.Action{
		ActionTypes: csiext.ActionTypes_FAILOVER_REMOTE,
	}
	params := map[string]string{
		s.opts.replicationContextPrefix + "systemName":       "cluster1",
		s.opts.replicationContextPrefix + "remoteSystemName": "cluster1",
		s.opts.replicationContextPrefix + "VolumeGroupName":  "csi-prov-test-19743d82-192-168-111-25-Five_Minutes",
	}
	req := &csiext.ExecuteActionRequest{
		ActionId:                        "",
		ProtectionGroupId:               "",
		ActionTypes:                     &csiext.ExecuteActionRequest_Action{Action: action},
		ProtectionGroupAttributes:       params,
		RemoteProtectionGroupId:         "",
		RemoteProtectionGroupAttributes: nil,
	}

	return req
}

func (f *feature) iCallExecuteActionSyncFailoverUnplanned() error {
	req := executeActionRequestFailoverUnplanned(f.service)
	f.executeActionRequest = req
	f.executeActionResponse, f.err = f.service.ExecuteAction(context.Background(), req)
	if f.err != nil {
		log.Printf("ExecuteAction call failed: %s\n", f.err.Error())
	}
	return nil
}

func executeActionRequestFailoverUnplanned(s *service) *csiext.ExecuteActionRequest {
	action := &csiext.Action{
		ActionTypes: csiext.ActionTypes_UNPLANNED_FAILOVER_LOCAL,
	}
	params := map[string]string{
		s.opts.replicationContextPrefix + "systemName":       "cluster1",
		s.opts.replicationContextPrefix + "remoteSystemName": "cluster1",
		s.opts.replicationContextPrefix + "VolumeGroupName":  "csi-prov-test-19743d82-192-168-111-25-Five_Minutes",
	}
	req := &csiext.ExecuteActionRequest{
		ActionId:                        "",
		ProtectionGroupId:               "",
		ActionTypes:                     &csiext.ExecuteActionRequest_Action{Action: action},
		ProtectionGroupAttributes:       params,
		RemoteProtectionGroupId:         "",
		RemoteProtectionGroupAttributes: nil,
	}

	return req
}

func (f *feature) iCallExecuteActionSyncFailover() error {
	req := executeActionRequestFailover(f.service)
	f.executeActionRequest = req
	f.executeActionResponse, f.err = f.service.ExecuteAction(context.Background(), req)
	if f.err != nil {
		log.Printf("ExecuteAction call failed: %s\n", f.err.Error())
	}
	return nil
}

func executeActionRequestWithParams(s *service, systemName, clusterNameOne, clusterNameTwo, remoteSystemName, vgname, ppname string) *csiext.ExecuteActionRequest {
	action := &csiext.Action{
		ActionTypes: csiext.ActionTypes_RESUME,
	}
	params := map[string]string{
		//s.opts.replicationContextPrefix + systemName: clusterNameOne,
		//s.opts.replicationContextPrefix + remoteSystemName: clusterNameTwo,
		//s.opts.replicationContextPrefix + vgname:  ppname,

	}
	req := &csiext.ExecuteActionRequest{
		ActionId:                        "",
		ProtectionGroupId:               "",
		ActionTypes:                     &csiext.ExecuteActionRequest_Action{Action: action},
		ProtectionGroupAttributes:       params,
		RemoteProtectionGroupId:         "",
		RemoteProtectionGroupAttributes: nil,
	}

	return req
}

func (f *feature) iCallExecuteActionBad() error {
	req := executeActionRequestBad(f.service)
	f.executeActionRequest = req
	f.executeActionResponse, f.err = f.service.ExecuteAction(context.Background(), req)
	if f.err != nil {
		log.Printf("ExecuteAction call failed: %s\n", f.err.Error())
	}
	return nil
}

func executeActionRequestBad(s *service) *csiext.ExecuteActionRequest {
	action := &csiext.Action{
		ActionTypes: csiext.ActionTypes_UNKNOWN_ACTION,
	}
	params := map[string]string{
		s.opts.replicationContextPrefix + "systemName":       "cluster1",
		s.opts.replicationContextPrefix + "remoteSystemName": "cluster1",
		s.opts.replicationContextPrefix + "VolumeGroupName":  "csi-prov-test-19743d82-192-168-111-25-Five_Minutes",
	}
	req := &csiext.ExecuteActionRequest{
		ActionId:                        "",
		ProtectionGroupId:               "",
		ActionTypes:                     &csiext.ExecuteActionRequest_Action{Action: action},
		ProtectionGroupAttributes:       params,
		RemoteProtectionGroupId:         "",
		RemoteProtectionGroupAttributes: nil,
	}

	return req
}

func (f *feature) iCallExecuteActionWithParams(s *service, systemName, clusterNameOne, clusterNameTwo, remoteSystemName, vgname, ppname string) error {
	req := executeActionRequestWithParams(f.service, systemName, clusterNameOne, clusterNameTwo, remoteSystemName, vgname, ppname)
	f.executeActionRequest = req
	f.executeActionResponse, f.err = f.service.ExecuteAction(context.Background(), req)
	if f.err != nil {
		log.Printf("ExecuteAction call failed: %s\n", f.err.Error())
	}
	return nil
}

func getCreateRemoteVolumeRequestBad(s *service) *csiext.CreateRemoteVolumeRequest {
	req := new(csiext.CreateRemoteVolumeRequest)
	req.VolumeHandle = "volume1=_=_=19=_=_=System"
	parameters := make(map[string]string)
	parameters[constants.EnvReplicationPrefix+"/"+KeyReplicationRemoteSystem] = ""
	parameters[s.WithRP(KeyReplicationRemoteSystem)] = "cluster1"
	parameters[s.WithRP(KeyReplicationRemoteSystem)] = "cluster2"
	req.Parameters = parameters
	return req
}

func (f *feature) iCallCreateRemoteVolumeBad() error {
	req := getCreateRemoteVolumeRequestBad(f.service)
	f.createRemoteVolumeRequest = req
	f.createRemoteVolumeResponse, f.err = f.service.CreateRemoteVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateRemoteVolume call failed: %s\n", f.err.Error())
	}
	if f.createRemoteVolumeResponse != nil {
		stepHandlersErrors.ExportNotFoundError = false
		stepHandlersErrors.VolumeNotExistError = false
	}
	return nil
}

func getCreateStorageProtectionGroupRequestBad(s *service) *csiext.CreateStorageProtectionGroupRequest {
	req := new(csiext.CreateStorageProtectionGroupRequest)
	req.VolumeHandle = "volume1=_=_=19=_=_=System"
	parameters := make(map[string]string)
	parameters[constants.EnvReplicationPrefix+"/"+KeyReplicationRemoteSystem] = ""
	parameters[s.WithRP(KeyReplicationRemoteSystem)] = "cluster1"
	parameters[s.WithRP(KeyReplicationRemoteSystem)] = "cluster2"
	req.Parameters = parameters
	return req
}

func (f *feature) iCallCreateStorageProtectionGroupBad() error {
	req := getCreateStorageProtectionGroupRequestBad(f.service)
	f.createStorageProtectionGroupRequest = req
	f.createStorageProtectionGroupResponse, f.err = f.service.CreateStorageProtectionGroup(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateStorageProtectionGroup call failed: %s\n", f.err.Error())
	}
	return nil
}

func (f *feature) aValidGetReplicationCapabilitiesResponseIsReturned() error {
	rep := f.getReplicationCapabilityResponse
	if rep != nil {
		if rep.Capabilities == nil {
			return errors.New("no capabilities returned in GetReplicationCapabilitiesResponse")
		}
		count := 0
		for _, cap := range rep.Capabilities {
			rpcType := cap.GetRpc().Type
			switch rpcType {
			case csiext.ReplicationCapability_RPC_CREATE_REMOTE_VOLUME:
				count = count + 1
			case csiext.ReplicationCapability_RPC_CREATE_PROTECTION_GROUP:
				count = count + 1
			case csiext.ReplicationCapability_RPC_DELETE_PROTECTION_GROUP:
				count = count + 1
			case csiext.ReplicationCapability_RPC_REPLICATION_ACTION_EXECUTION:
				count = count + 1
			case csiext.ReplicationCapability_RPC_MONITOR_PROTECTION_GROUP:
				count = count + 1
			default:
				return fmt.Errorf("received unexpected capability: %v", rpcType)
			}
		}

		if rep.Actions == nil {
			return errors.New("no actions returned in GetReplicationCapabilitiesResponse")
		}
		for _, action := range rep.Actions {
			actType := action.GetType()
			switch actType {
			case csiext.ActionTypes_FAILOVER_REMOTE:
				count = count + 1
			case csiext.ActionTypes_UNPLANNED_FAILOVER_LOCAL:
				count = count + 1
			case csiext.ActionTypes_REPROTECT_LOCAL:
				count = count + 1
			case csiext.ActionTypes_SUSPEND:
				count = count + 1
			case csiext.ActionTypes_RESUME:
				count = count + 1
			case csiext.ActionTypes_SYNC:
				count = count + 1
			default:
				return fmt.Errorf("received unexpected actiontype: %v", actType)

			}
		}

	}

	return nil
}

func (f *feature) iCallValidateVolumeHostConnectivity() error {

	header := metadata.New(map[string]string{"csi.requestid": "1"})
	ctx := metadata.NewIncomingContext(context.Background(), header)

	csiNodeID, err := f.service.getPowerScaleNodeID(ctx)
	if err != nil {
		f.err = errors.New(err.Error())
		return nil
	}
	log.Printf("Node id is: %v", csiNodeID)

	volIDs := make([]string, 0)

	if stepHandlersErrors.PodmonNoVolumeNoNodeIDError == true {
		csiNodeID = ""
	} else if stepHandlersErrors.PodmonNoNodeIDError == true {
		csiNodeID = ""
		volid := f.createVolumeResponse.GetVolume().VolumeId
		volIDs = volIDs[:0]
		volIDs = append(volIDs, volid)
	} else if stepHandlersErrors.PodmonControllerProbeError == true {
		f.service.mode = "controller"
	} else if stepHandlersErrors.PodmonNodeProbeError == true {
		f.service.mode = "node"
	} else if stepHandlersErrors.PodmonVolumeError == true {
		volid := "9999"
		volIDs = append(volIDs, volid)
	} else {
		volid := f.createVolumeResponse.GetVolume().VolumeId
		volIDs = volIDs[:0]
		volIDs = append(volIDs, volid)
	}

	req := &podmon.ValidateVolumeHostConnectivityRequest{
		NodeId:    csiNodeID,
		VolumeIds: volIDs,
	}

	connect, err := f.service.ValidateVolumeHostConnectivity(ctx, req)
	if err != nil {
		f.err = errors.New(err.Error())
		return nil
	}
	f.validateVolumeHostConnectivityResp = connect
	if len(connect.Messages) > 0 {
		for i, msg := range connect.Messages {
			fmt.Printf("messages %d: %s\n", i, msg)
			if stepHandlersErrors.PodmonVolumeStatisticsError == true ||
				stepHandlersErrors.PodmonVolumeError == true {
				if strings.Contains(msg, "volume") {
					fmt.Printf("found %d: %s\n", i, msg)
					f.err = errors.New(connect.Messages[i])
					return nil
				}
			}
		}
		fmt.Printf("DEBUG connect Messages %s\n", connect.Messages[0])
		if stepHandlersErrors.PodmonVolumeStatisticsError == true {
			f.err = errors.New(connect.Messages[0])
			return nil
		}
	}

	if connect.IosInProgress {
		return nil
	}
	err = fmt.Errorf("Unexpected error IO to volume: %t", connect.IosInProgress)
	return nil
}

func (f *feature) theValidateConnectivityResponseMessageContains(expected string) error {
	resp := f.validateVolumeHostConnectivityResp
	if resp != nil {
		for _, m := range resp.Messages {
			if strings.Contains(m, expected) {
				return nil
			}
		}
	}
	return fmt.Errorf("Expected %s message in ValidateVolumeHostConnectivityResp but it wasn't there", expected)
}

func (f *feature) iCallProbeController() error {
	header := metadata.New(map[string]string{"csi.requestid": "1"})
	ctx := metadata.NewIncomingContext(context.Background(), header)
	req := &commonext.ProbeControllerRequest{}
	connect, err := f.service.ProbeController(ctx, req)
	if err != nil {
		f.err = errors.New(err.Error())
		return nil
	}
	fmt.Printf("response is %v", connect)
	return nil
}

func (f *feature) iCallDynamicLogChange(file string) error {
	log.Printf("level before change: %s", utils.GetCurrentLogLevel())
	DriverConfigParamsFile = "mock/loglevel/" + file
	log.Printf("wait for config change %s", DriverConfigParamsFile)
	f.iCallBeforeServe()
	time.Sleep(10 * time.Second)
	return nil
}

func (f *feature) aValidDynamicLogChangeOccurs(file, expectedLevel string) error {
	log.Printf("level after change: %s", utils.GetCurrentLogLevel())
	if utils.GetCurrentLogLevel().String() != expectedLevel {
		err := fmt.Errorf("level was expected to be %s, but was %s instead", expectedLevel, utils.GetCurrentLogLevel().String())
		return err
	}
	log.Printf("Reverting log changes made")
	DriverConfigParamsFile = "mock/loglevel/logConfig.yaml"
	f.iCallBeforeServe()
	time.Sleep(10 * time.Second)
	return nil
}

func (f *feature) iSetNoProbeOnStart(value string) error {
	os.Setenv(constants.EnvNoProbeOnStart, value)
	return nil
}

func (f *feature) iCallGetSnapshotNameFromIsiPathWith(isiPath string) error {
	clusterConfig := f.service.getIsilonClusterConfig(clusterName1)
	ctx, _, _ := GetRunIDLog(context.Background())
	_, f.err = clusterConfig.isiSvc.GetSnapshotNameFromIsiPath(ctx, isiPath)
	if f.err != nil {
		log.Printf("inside iCallGetSnapshotNameFromIsiPath error %s\n", f.err.Error())
	}
	return nil
}
func (f *feature) iCallGetSnapshotIsiPathComponents() error {
	clusterConfig := f.service.getIsilonClusterConfig(clusterName1)
	_, _, _ = clusterConfig.isiSvc.GetSnapshotIsiPathComponents("/ifs/.snapshot/data/csiislon")
	_ = clusterConfig.isiSvc.GetSnapshotTrackingDirName("data")
	return nil
}

func (f *feature) iCallGetSubDirectoryCount() error {
	clusterConfig := f.service.getIsilonClusterConfig(clusterName1)
	ctx, _, _ := GetRunIDLog(context.Background())
	_, _ = clusterConfig.isiSvc.GetSubDirectoryCount(ctx, "/ifs/data/csi-isilon", "csi-isilon")
	return nil
}

func (f *feature) iCallDeleteSnapshotIsiService() error {
	clusterConfig := f.service.getIsilonClusterConfig(clusterName1)
	ctx, _, _ := GetRunIDLog(context.Background())
	f.err = clusterConfig.isiSvc.DeleteSnapshot(ctx, 64, "")
	if f.err != nil {
		log.Printf("inside iCallDeleteSnapshotIsiService error %s\n", f.err.Error())
	}
	return nil
}

func (f *feature) iCallCreateVolumeReplicationEnabled() error {
	req := getCreatevolumeReplicationEnabled(f.service)
	f.createVolumeRequestTest = req
	f.createVolumeResponseTest, f.err = f.service.CreateVolume(context.Background(), req)
	if f.err != nil {
		stepHandlersErrors.ExportNotFoundError = false
		stepHandlersErrors.VolumeNotExistError = false
	}
	return nil
}

func getCreatevolumeReplicationEnabled(s *service) *csi.CreateVolumeRequest {
	req := new(csi.CreateVolumeRequest)
	req.Name = "volume1"
	capacityRange := new(csi.CapacityRange)
	capacityRange.RequiredBytes = 8 * 1024 * 1024 * 1024
	req.CapacityRange = capacityRange
	mount := new(csi.VolumeCapability_MountVolume)
	capability := new(csi.VolumeCapability)
	accessType := new(csi.VolumeCapability_Mount)
	accessType.Mount = mount
	capability.AccessType = accessType
	accessMode := new(csi.VolumeCapability_AccessMode)
	accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY
	capability.AccessMode = accessMode
	capabilities := make([]*csi.VolumeCapability, 0)
	capabilities = append(capabilities, capability)
	parameters := make(map[string]string)
	parameters[AccessZoneParam] = "System"
	parameters[IsiPathParam] = "/ifs/data/csi-isilon"
	parameters[s.WithRP(KeyReplicationEnabled)] = "true"
	parameters[s.WithRP(KeyReplicationVGPrefix)] = "volumeGroupPrefix"
	parameters[s.WithRP(KeyReplicationRemoteAccessZone)] = "remoteAccessZone"
	parameters[s.WithRP(KeyReplicationRemoteAzServiceIP)] = "remoteAzServiceIP"
	parameters[s.WithRP(KeyReplicationRemoteRootClientEnabled)] = "remoteRootClientEnabled"
	parameters[s.WithRP(KeyReplicationRPO)] = "Five_Minutes"
	parameters[s.WithRP(KeyReplicationRemoteSystem)] = "cluster1"
	parameters[req.VolumeContentSource.String()] = "contentsource"
	req.Parameters = parameters
	return req
}

func (f *feature) aValidCreateVolumeRespIsReturned() error {
	if f.err != nil {
		stepHandlersErrors.ExportNotFoundError = false
		stepHandlersErrors.VolumeNotExistError = false
	}
	return nil
}

func getTypicalCreateROVolumeFromSnapshotRequest() *csi.CreateVolumeRequest {
	req := new(csi.CreateVolumeRequest)
	req.Name = "volume1"
	capacityRange := new(csi.CapacityRange)
	capacityRange.RequiredBytes = 8 * 1024 * 1024 * 1024
	req.CapacityRange = capacityRange
	mount := new(csi.VolumeCapability_MountVolume)
	capability := new(csi.VolumeCapability)
	accessType := new(csi.VolumeCapability_Mount)
	accessType.Mount = mount
	capability.AccessType = accessType
	accessMode := new(csi.VolumeCapability_AccessMode)
	accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY
	capability.AccessMode = accessMode
	capabilities := make([]*csi.VolumeCapability, 0)
	capabilities = append(capabilities, capability)
	parameters := make(map[string]string)
	parameters[AccessZoneParam] = "System"
	parameters[IsiPathParam] = "/ifs/data/csi-isilon"
	req.Parameters = parameters
	req.VolumeCapabilities = capabilities
	return req
}

func (f *feature) iCallCreateROVolumeFromSnapshot(name string) error {
	req := getTypicalCreateROVolumeFromSnapshotRequest()
	if f.rootClientEnabled != "" {
		req.Parameters[RootClientEnabledParam] = f.rootClientEnabled
	}
	f.createVolumeRequest = req
	req.Name = name
	f.createVolumeResponse, f.err = f.service.CreateVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateVolume call failed: %s\n", f.err.Error())
	}
	if f.createVolumeResponse != nil {
		log.Printf("volume name '%s' created\n", name)
	}
	return nil
}

func (f *feature) iCallCreateVolumeFromSnapshotMultiReader(srcSnapshotID, name string) error {
	req := getTypicalCreateROVolumeFromSnapshotRequest()
	f.createVolumeRequest = req
	req.Name = name
	req = f.setVolumeContent(true, srcSnapshotID)
	log.Printf("called iCallCreateVolumeFromSnapshotMultiReader")
	f.createVolumeResponse, f.err = f.service.CreateVolume(context.Background(), req)
	if f.err != nil {
		log.Printf("CreateVolume call failed: '%s'\n", f.err.Error())
	}
	if f.createVolumeResponse != nil {
		log.Printf("volume name '%s' created\n", name)
	}
	return nil
}

func (f *feature) iCallDeleteVolumeFromSnapshot(id string) error {
	if f.deleteVolumeRequest == nil {
		req := getTypicalDeleteVolumeRequest()
		f.deleteVolumeRequest = req
	}
	req := f.deleteVolumeRequest
	req.VolumeId = id

	ctx, log, _ := GetRunIDLog(context.Background())

	f.deleteVolumeResponse, f.err = f.service.DeleteVolume(ctx, req)
	if f.err != nil {
		log.Printf("DeleteVolume call failed: '%v'\n", f.err)
	}
	return nil
}

func (f *feature) aValidDeleteSnapshotResponseIsReturned() error {
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *feature) iCallControllerPublishVolumeOnSnapshot(volID, accessMode, nodeID, path string) error {
	log.Printf("iCallControllerPublishVolume called with %s and %s", accessMode, nodeID)
	header := metadata.New(map[string]string{"csi.requestid": "1"})
	ctx := metadata.NewIncomingContext(context.Background(), header)
	req := f.publishVolumeRequest
	if f.publishVolumeRequest == nil {
		req = f.getControllerPublishVolumeRequestOnSnapshot(accessMode, nodeID, path)
		f.publishVolumeRequest = req
	}

	// a customized volume ID can be specified to overwrite the default one
	if volID != "" {
		req.VolumeId = volID
	}

	log.Printf("Calling controllerPublishVolume with request %v", req)
	f.publishVolumeResponse, f.err = f.service.ControllerPublishVolume(ctx, req)
	if f.err != nil {
		log.Printf("PublishVolume call failed: %s\n", f.err.Error())
	}
	f.publishVolumeRequest = nil
	return nil
}

func (f *feature) getControllerPublishVolumeRequestOnSnapshot(accessType, nodeID, path string) *csi.ControllerPublishVolumeRequest {
	capability := new(csi.VolumeCapability)

	mountVolume := new(csi.VolumeCapability_MountVolume)
	mountVolume.MountFlags = make([]string, 0)
	mount := new(csi.VolumeCapability_Mount)
	mount.Mount = mountVolume
	capability.AccessType = mount

	if !inducedErrors.omitAccessMode {
		capability.AccessMode = getAccessMode(accessType)
	}
	fmt.Printf("capability.AccessType %v\n", capability.AccessType)
	fmt.Printf("capability.AccessMode %v\n", capability.AccessMode)
	req := new(csi.ControllerPublishVolumeRequest)
	if !inducedErrors.noVolumeID {
		if inducedErrors.invalidVolumeID || f.createVolumeResponse == nil {
			req.VolumeId = "000-000"
		} else {
			req.VolumeId = "volume1=_=_=19=_=_=System"
		}
	}
	if !inducedErrors.noNodeID {
		req.NodeId = nodeID
	}
	req.Readonly = false
	if !inducedErrors.omitVolumeCapability {
		req.VolumeCapability = capability
	}
	// add in the context
	attributes := map[string]string{}
	attributes[AccessZoneParam] = f.accessZone
	if f.rootClientEnabled != "" {
		attributes[RootClientEnabledParam] = f.rootClientEnabled
	}
	attributes[ExportPathParam] = path
	//ExportPathParam
	req.VolumeContext = attributes
	return req
}

func (f *feature) iCallQueryArrayStatus(apiPort string) error {

	// Calling http mock server
	MockK8sAPI()
	ctx := context2.Background()
	url := "http://" + "127.0.0.1:" + apiPort + arrayStatus + "/" + "cluster1"
	_, err := f.service.queryArrayStatus(ctx, url)
	if err != nil {
		log.Printf("queryArrayStatus failed: %s", err)
	}
	return nil
}
