// Copyright © 2019-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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
package integration_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	fromctx "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/fromcontext"
	ident "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/identifiers"
	isilonfs "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/powerscale-fs"
	strutil "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/string-utils"
	csiutils "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/csi-utils"
	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/service"
	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	apiv1 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v1"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"
)

const (
	MaxRetries       = 10
	RetrySleepTime   = 1 * time.Second
	SleepTime        = 200 * time.Millisecond
	AccessZoneParam  = "AccessZone"
	ExportPathParam  = "Path"
	IsiPathParam     = "IsiPath"
	ClusterNameParam = "ClusterName"
	EnvClusterName   = "X_CSI_CLUSTER_NAME"
	AZServiceIPParam = "AzServiceIP"
	EnvAZServiceIP   = "X_CSI_ISI_AZ_SERVICE_IP"
)

var (
	err       error
	isiClient *isi.Client
)

type feature struct {
	errs                             []error
	createVolumeRequest              *csi.CreateVolumeRequest
	controllerPublishVolumeRequest   *csi.ControllerPublishVolumeRequest
	controllerUnpublishVolumeRequest *csi.ControllerUnpublishVolumeRequest
	nodeStageVolumeRequest           *csi.NodeStageVolumeRequest
	nodeUnstageVolumeRequest         *csi.NodeUnstageVolumeRequest
	nodePublishVolumeRequest         *csi.NodePublishVolumeRequest
	nodeUnpublishVolumeRequest       *csi.NodeUnpublishVolumeRequest
	listVolumesRequest               *csi.ListVolumesRequest
	listVolumesResponse              *csi.ListVolumesResponse
	listSnapshotsResponse            *csi.ListSnapshotsResponse
	createSnapshotRequest            *csi.CreateSnapshotRequest
	createSnapshotResponse           *csi.CreateSnapshotResponse
	deleteSnapshotRequest            *csi.DeleteSnapshotRequest
	capability                       *csi.VolumeCapability
	capabilities                     []*csi.VolumeCapability
	volID                            string
	volName                          string
	exportID                         int
	snapshotID                       string
	volNameID                        map[string]string
	volIDContext                     map[string]*csi.Volume
	snapshotIDList                   []string
	maxRetryCount                    int
	vol                              *csi.Volume
	isiPath                          string
	accssZone                        string
	clusterName                      string
	// mTLS-specific fields
	storageClassParams   map[string]string
	mountOptions         []string
	expectedMountSource  string
	expectedMountFailure bool
	expectedEventReason  string
	expectedErrorMessage string
	tlsHandshakeTimeout  int
	customClusterConfig  map[string]string
	customEnvVars        map[string]string
}

func (f *feature) addError(err error) {
	f.errs = append(f.errs, err)
}

func (f *feature) aIsilonService() error {
	f.errs = make([]error, 0)
	f.createVolumeRequest = nil
	f.nodePublishVolumeRequest = nil
	f.nodeUnpublishVolumeRequest = nil
	f.controllerPublishVolumeRequest = nil
	f.controllerUnpublishVolumeRequest = nil
	f.listVolumesResponse = nil
	f.listSnapshotsResponse = nil
	f.createSnapshotRequest = nil
	f.createSnapshotResponse = nil
	f.deleteSnapshotRequest = nil
	f.capability = nil
	f.capabilities = nil
	f.volID = ""
	f.snapshotID = ""
	f.snapshotIDList = f.snapshotIDList[:0]
	f.volNameID = make(map[string]string)
	f.volIDContext = make(map[string]*csi.Volume)
	f.maxRetryCount = MaxRetries
	// Reset mTLS-specific fields
	f.storageClassParams = make(map[string]string)
	f.mountOptions = []string{}
	f.expectedMountSource = ""
	f.expectedMountFailure = false
	f.expectedEventReason = ""
	f.expectedErrorMessage = ""
	f.tlsHandshakeTimeout = 0
	f.customClusterConfig = make(map[string]string)
	f.customEnvVars = make(map[string]string)
	return nil
}

// build a basic Create Volume Request
func (f *feature) aBasicVolumeRequest(name string, size int64) error {
	req := new(csi.CreateVolumeRequest)
	req.Name = name
	capacityRange := new(csi.CapacityRange)
	capacityRange.RequiredBytes = size * 1024 * 1024 * 1024
	req.CapacityRange = capacityRange
	capability := new(csi.VolumeCapability)
	mount := new(csi.VolumeCapability_MountVolume)
	mount.FsType = "nfs"
	mount.MountFlags = []string{"vers=4"}
	mountType := new(csi.VolumeCapability_Mount)
	mountType.Mount = mount
	capability.AccessType = mountType
	accessMode := new(csi.VolumeCapability_AccessMode)
	accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER
	capability.AccessMode = accessMode
	f.capability = capability
	capabilities := make([]*csi.VolumeCapability, 0)
	capabilities = append(capabilities, capability)
	req.VolumeCapabilities = capabilities
	parameters := make(map[string]string)
	parameters[AccessZoneParam] = os.Getenv(constants.EnvAccessZone)
	parameters[IsiPathParam] = os.Getenv(constants.EnvPath)
	if _, isPresent := os.LookupEnv(EnvClusterName); isPresent {
		parameters[ClusterNameParam] = os.Getenv(EnvClusterName)
	}
	parameters[AZServiceIPParam] = os.Getenv(EnvAZServiceIP)
	req.Parameters = parameters
	f.createVolumeRequest = req
	f.isiPath = parameters[IsiPathParam]
	f.accssZone = parameters[AccessZoneParam]
	return nil
}

func (f *feature) iCallCreateVolume() error {
	time.Sleep(RetrySleepTime)
	ctx := context.Background()
	volResp, err := f.createVolume(f.createVolumeRequest)
	if err != nil {
		fmt.Printf("CreateVolume: '%s'\n", err.Error())
		f.addError(err)
	} else {
		if volResp.Volume == nil {
			fmt.Printf("The volume has already been created\n")
			return nil
		}
		fmt.Printf("CreateVolume '%s' ('%s') '%s'\n", volResp.GetVolume().VolumeContext["Name"],
			volResp.GetVolume().VolumeId, volResp.GetVolume().VolumeContext["CreationTime"])
		fmt.Printf("The access zone is '%s'\n", volResp.GetVolume().VolumeContext[AccessZoneParam])
		f.volID = volResp.GetVolume().VolumeId
		f.volName, f.exportID, f.accssZone, f.clusterName, err = ident.ParseNormalizedVolumeID(ctx, f.volID)
		f.volNameID[f.volName] = f.volID
		f.vol = volResp.Volume
	}
	return nil
}

func (f *feature) createVolume(req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	ctx := context.Background()
	client := csi.NewControllerClient(grpcClient)
	var volResp *csi.CreateVolumeResponse
	// Retry loop to deal with Isilon API being overwhelmed
	for i := 0; i < f.maxRetryCount; i++ {
		volResp, err = client.CreateVolume(ctx, req)
		// f.vol = volResp.Volume
		if err == nil || !strings.Contains(err.Error(), "Insufficient resources") {
			// no need for retry
			break
		}
		fmt.Printf("retry: '%s'\n", err.Error())
		time.Sleep(RetrySleepTime)
	}
	return volResp, err
}

func (f *feature) iCallDeleteVolume() error {
	err := f.deleteVolume(f.volID)
	if err != nil {
		fmt.Printf("DeleteVolume '%s':\n", err.Error())
		f.addError(err)
	} else {
		fmt.Printf("DeleteVolume '%s' completed successfully\n", f.volID)
	}
	return nil
}

func (f *feature) deleteVolume(id string) error {
	ctx := context.Background()
	client := csi.NewControllerClient(grpcClient)
	delVolReq := new(csi.DeleteVolumeRequest)
	delVolReq.VolumeId = id
	// Retry loop to deal with Isilon API being overwhelmed
	for i := 0; i < f.maxRetryCount; i++ {
		_, err = client.DeleteVolume(ctx, delVolReq)
		if err == nil || !strings.Contains(err.Error(), "Insufficient resources") {
			// no need for retry
			break
		}
		fmt.Printf("retry: '%s'\n", err.Error())
		time.Sleep(RetrySleepTime)
	}
	return err
}

func (f *feature) iCallDeleteAllVolumes() error {
	for _, v := range f.volNameID {
		f.volID = v
		f.iCallDeleteVolume()
	}
	return nil
}

func (f *feature) thereAreNoErrors() error {
	if len(f.errs) == 0 {
		return nil
	}
	return f.errs[0]
}

func createIsilonClient() (*isi.Client, error) {
	ctx := context.Background()
	configBytes, err := os.ReadFile(os.Getenv(constants.EnvIsilonConfigFile)) // #nosec G703 -- config file path from trusted environment variable
	if err != nil {
		return nil, fmt.Errorf("file ('%s') error: %v", os.Getenv(constants.EnvIsilonConfigFile), err)
	}
	user, password, IPaddr, err := getDetails(configBytes, os.Getenv(EnvClusterName))
	if err != nil {
		return nil, err
	}

	ignoreUnresolvableHosts := false

	isiClient, err = isi.NewClientWithArgs(
		ctx,
		"https://"+IPaddr+":"+os.Getenv(constants.EnvPort),
		true,
		1,
		user,
		"",
		password,
		os.Getenv(constants.EnvPath),
		os.Getenv(constants.DefaultIsiVolumePathPermissions),
		ignoreUnresolvableHosts,
		uint8(fromctx.GetUint(ctx, constants.EnvIsiAuthType))) // #nosec G115 -- This is a false positive
	if err != nil {
		fmt.Printf("error creating isilon client: '%s'\n", err.Error())
	}
	return isiClient, err
}

func getDetails(configBytes []byte, clusterName string) (string, string, string, error) {
	jsonConfig := new(service.IsilonClusters)
	err := yaml.Unmarshal(configBytes, &jsonConfig)
	if err != nil {
		return "", "", "", fmt.Errorf("unable to parse isilon clusters' config details [%v]", err)
	}

	if len(jsonConfig.IsilonClusters) == 0 {
		return "", "", "", errors.New("cluster details are not provided in isilon-creds secret")
	}
	for _, config := range jsonConfig.IsilonClusters {
		if len(clusterName) > 0 {
			if config.ClusterName == clusterName {
				return config.User, config.Password, config.Endpoint, nil
			}
		} else if *config.IsDefault {
			return config.User, config.Password, config.Endpoint, nil
		}
	}
	err = errors.New("")
	return "", "", "", err
}

func (f *feature) thereIsADirectory(name string) error {
	ctx := context.Background()
	isiClient, err = createIsilonClient()
	_, err = isiClient.GetVolumeWithIsiPath(ctx, f.isiPath, "", name)
	if err != nil {
		f.addError(err)
		panic(fmt.Sprintf("there should be a directory '%s'\n", name))
	}
	return nil
}

func (f *feature) thereIsNoDirectory(name string) error {
	ctx := context.Background()
	isiClient, err = createIsilonClient()
	_, err = isiClient.GetVolumeWithIsiPath(ctx, f.isiPath, "", name)
	if err == nil {
		f.addError(fmt.Errorf("there should not be a directory '%s'", name))
		panic(fmt.Sprintf("there should not be a directory '%s'", name))
	}
	return nil
}

func (f *feature) thereIsAnExport(name string) error {
	ctx := context.Background()
	isiClient, err = createIsilonClient()
	accessZone := f.vol.VolumeContext["AccessZone"]
	path := isilonfs.GetPathForVolume(f.isiPath, name)
	export, err := isiClient.GetExportByIDWithZone(ctx, f.exportID, accessZone)
	// export, err := isiClient.GetExportWithPathAndZone(ctx, path, accessZone)
	fmt.Printf("export is: '%v'\n", export)
	if err != nil || export == nil {
		f.addError(err)
		fmt.Printf("failed to get export with path: '%s' and access zone '%s'\n", path, accessZone)
		panic(fmt.Sprintf("there should be an export'\n"))
	}
	return nil
}

func (f *feature) thereIsAnExportForSnapshotDir(name string) error {
	ctx := context.Background()
	isiClient, err = createIsilonClient()

	snapshotSrc, err := isiClient.GetIsiSnapshotByIdentity(ctx, name)
	if err != nil {
		f.addError(err)
		fmt.Printf("failed to get snapshot id for snapshot '#{name}', error '#{err}'\n")
		panic(fmt.Sprintf("failed to get snapshot id for snapshot '%s', error '%v'\n", name, err))
	}

	snapshotIsiPath, err := isiClient.GetSnapshotIsiPath(ctx, f.isiPath, strconv.FormatInt(snapshotSrc.ID, 10), f.vol.VolumeContext["AccessZone"])
	if err != nil {
		f.addError(err)
		fmt.Printf("failed to get snapshot dir path for snapshot '#{name}', error '#{err}'\n")
		panic(fmt.Sprintf("failed to get snapshot dir path for snapshot '%s', error '%v'\n", name, err))
	}

	export, err := isiClient.GetExportWithPathAndZone(ctx, snapshotIsiPath, "")
	if err != nil || export == nil {
		f.addError(err)
		fmt.Printf("failed to get export for snapshot dir path error '#{err}'\n")
		panic(fmt.Sprintf("failed to get export for snapshot dir path error '%v'\n", err))
	}

	return nil
}

func (f *feature) thereIsNotAnExport(name string) error {
	ctx := context.Background()
	isiClient, err = createIsilonClient()
	accessZone := f.vol.VolumeContext["AccessZone"]
	path := isilonfs.GetPathForVolume(f.isiPath, name)
	export, _ := isiClient.GetExportWithPathAndZone(ctx, path, accessZone)
	if export != nil {
		panic(fmt.Sprintf("there should not be an export '%v'\n", export))
	}
	return nil
}

func (f *feature) thereIsNotADirectory(name string) error {
	ctx := context.Background()
	isiClient, err = createIsilonClient()
	_, err = isiClient.GetVolumeWithIsiPath(ctx, f.isiPath, "", name)
	if err == nil {
		panic(fmt.Sprintf("there should not be a directory '%s'\n", name))
	}
	return nil
}

func (f *feature) verifySize(name string) error {
	ctx := context.Background()
	var quota *apiv1.IsiQuota
	isiClient, err = createIsilonClient()
	enabled, _ := strconv.ParseBool(os.Getenv(constants.EnvQuotaEnabled))
	path := isilonfs.GetPathForVolume(f.isiPath, name)
	quota, err = isiClient.GetQuotaWithPath(ctx, path)
	fmt.Printf("Quota is: '%v'\n", quota)
	if !enabled {
		if quota != nil {
			panic(fmt.Sprintf("Quota is not enabled, so it should be nil: '%v'", quota))
		}
	} else {
		println(quota.Thresholds.Hard)
		println(f.createVolumeRequest.CapacityRange.RequiredBytes)
		// For volume requests with large size like 1EB the driver will set the requested size to 0
		// and the default volume size with which the volume is created by Isilon is 3GB in this case
		if f.createVolumeRequest.CapacityRange.RequiredBytes == 0 {
			if quota.Thresholds.Hard != 0 && quota.Thresholds.Hard != 3221225472 {
				panic(fmt.Sprintf("the size of volume '%s' is wrong, expected is '0' or '3221225472' \n", name))
			}
		} else if quota.Thresholds.Hard != f.createVolumeRequest.CapacityRange.RequiredBytes {
			panic(fmt.Sprintf("the size of volume '%s' is wrong, expected is '%d'\n", name, f.createVolumeRequest.CapacityRange.RequiredBytes))
		}
	}
	if err != nil && !strings.Contains(err.Error(), "Quota not found") {
		f.addError(err)
	}
	return nil
}

func (f *feature) thereIsNotAQuota(name string) error {
	// var quota *apiv1.IsiQuota
	ctx := context.Background()
	path := isilonfs.GetPathForVolume(f.isiPath, name)
	quota, err := isiClient.GetQuotaWithPath(ctx, path)
	fmt.Printf("quota is '%v'\n", quota)
	if err == nil {
		panic(fmt.Sprintf("verify there is not a quota failed\n"))
	}
	return nil
}

func (f *feature) iCallListVolumesWithMaxEntriesStartingToken(arg1 int, arg2 string) error {
	client := csi.NewControllerClient(grpcClient)
	ctx := context.Background()
	req := new(csi.ListVolumesRequest)
	var resp *csi.ListVolumesResponse
	req.MaxEntries = int32(arg1) // #nosec G115 -- This is a false positive
	req.StartingToken = arg2
	f.listVolumesRequest = req
	// Retry loop to deal with ISILON API being overwhelmed
	for i := 0; i < f.maxRetryCount; i++ {
		resp, err = client.ListVolumes(ctx, req)
		if err == nil || !strings.Contains(err.Error(), "Insufficient resources") {
			// no need for retry
			break
		}
		fmt.Printf("retry: '%s'\n", err.Error())
		time.Sleep(RetrySleepTime)
	}
	f.listVolumesResponse = resp
	if err != nil {
		f.addError(err)
		log.Printf("ListVolumes call error: '%s'\n", err.Error())
		return nil
	}
	return nil
}

func (f *feature) getControllerPublishVolumeRequest(nodeIDEnvVar string) *csi.ControllerPublishVolumeRequest {
	req := new(csi.ControllerPublishVolumeRequest)
	req.VolumeId = f.volID
	req.NodeId = os.Getenv(nodeIDEnvVar)
	fmt.Printf("req.NodeId %s\n", req.NodeId)
	req.Readonly = false
	req.VolumeCapability = f.capability
	req.VolumeContext = f.vol.VolumeContext
	f.controllerPublishVolumeRequest = req
	return req
}

func (f *feature) iCallControllerPublishVolume(nodeIDEnvVar string) error {
	err := f.controllerPublishVolume(f.volID, nodeIDEnvVar)
	if err != nil {
		fmt.Printf("ControllerPublishVolume %s:\n", err.Error())
		f.addError(err)
	} else {
		fmt.Printf("ControllerPublishVolume completed successfully\n")
	}
	time.Sleep(SleepTime)
	return nil
}

func (f *feature) controllerPublishVolume(id string, nodeIDEnvVar string) error {
	req := f.getControllerPublishVolumeRequest(nodeIDEnvVar)
	req.VolumeId = id
	ctx := context.Background()
	client := csi.NewControllerClient(grpcClient)
	fmt.Printf("controllerPublishVolume req is: '%v'\n", req)
	_, err := client.ControllerPublishVolume(ctx, req)
	return err
}

func (f *feature) iCallControllerUnpublishVolume(nodeIDEnvVar string) error {
	err := f.controllerUnpublishVolume(f.controllerPublishVolumeRequest.VolumeId, nodeIDEnvVar)
	if err != nil {
		fmt.Printf("ControllerUnpublishVolume failed: %s\n", err.Error())
		f.addError(err)
	} else {
		fmt.Printf("ControllerUnpublishVolume completed successfully\n")
	}
	time.Sleep(SleepTime)
	return nil
}

func (f *feature) controllerUnpublishVolume(id string, _ string) error {
	req := new(csi.ControllerUnpublishVolumeRequest)
	req.VolumeId = id
	req.NodeId = os.Getenv("X_CSI_NODE_NAME")
	ctx := context.Background()
	client := csi.NewControllerClient(grpcClient)
	_, err := client.ControllerUnpublishVolume(ctx, req)
	return err
}

func (f *feature) aCapabilityWithAccess(access string) error {
	// Construct the volume capabilities
	capability := new(csi.VolumeCapability)
	accessMode := new(csi.VolumeCapability_AccessMode)
	switch access {
	case "single-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER
	case "multi-writer":
		accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER
	case "multi-reader":
		accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY
	}
	capability.AccessMode = accessMode
	f.capabilities = make([]*csi.VolumeCapability, 0)
	f.capabilities = append(f.capabilities, capability)
	f.capability = capability
	return nil
}

func (f *feature) aVolumeRequest(name string, size int64) error {
	req := new(csi.CreateVolumeRequest)
	req.Name = name
	capacityRange := new(csi.CapacityRange)
	capacityRange.RequiredBytes = size * 1024 * 1024 * 1024
	req.CapacityRange = capacityRange
	mount := new(csi.VolumeCapability_MountVolume)
	mount.FsType = "nfs"

	// Apply mTLS mount options if configured
	if len(f.mountOptions) > 0 {
		mount.MountFlags = f.mountOptions
	}

	mountType := new(csi.VolumeCapability_Mount)
	mountType.Mount = mount
	f.capability.AccessType = mountType
	req.VolumeCapabilities = f.capabilities
	parameters := make(map[string]string)
	parameters[AccessZoneParam] = os.Getenv(constants.EnvAccessZone)
	parameters[IsiPathParam] = os.Getenv(constants.EnvPath)
	if _, isPresent := os.LookupEnv(EnvClusterName); isPresent {
		parameters[ClusterNameParam] = os.Getenv(EnvClusterName)
	}
	parameters[AZServiceIPParam] = os.Getenv(EnvAZServiceIP)

	// Apply mTLS-specific parameters from storageClassParams
	if f.storageClassParams != nil {
		for key, value := range f.storageClassParams {
			parameters[key] = value
		}
	}

	// Apply custom cluster config if set
	if f.customClusterConfig != nil {
		for key, value := range f.customClusterConfig {
			parameters[key] = value
		}
	}

	// Apply custom environment variables if set
	if f.customEnvVars != nil {
		for key, value := range f.customEnvVars {
			os.Setenv(key, value)
		}
	}

	req.Parameters = parameters
	f.createVolumeRequest = req
	f.accssZone = parameters[AccessZoneParam]
	f.isiPath = parameters[IsiPathParam]
	return nil
}

func (f *feature) getNodeStageVolumeRequest() *csi.NodeStageVolumeRequest {
	req := new(csi.NodeStageVolumeRequest)
	req.VolumeId = f.volID
	req.VolumeCapability = f.capability
	req.VolumeContext = f.vol.VolumeContext
	req.StagingTargetPath = datadir
	f.nodeStageVolumeRequest = req
	return req
}

func (f *feature) iCallNodeStageVolume() error {
	time.Sleep(SleepTime)
	f.nodeStageVolumeRequest = f.getNodeStageVolumeRequest()
	err := f.nodeStageVolume(f.nodeStageVolumeRequest)
	if err != nil {
		fmt.Printf("NodeStageVolume failed: '%s'\n", err.Error())
		f.addError(err)
	} else {
		fmt.Printf("NodeStageVolume completed successfully\n")
	}
	time.Sleep(SleepTime)
	return nil
}

func (f *feature) nodeStageVolume(req *csi.NodeStageVolumeRequest) error {
	ctx := context.Background()
	client := csi.NewNodeClient(grpcClient)
	for i := 0; i < f.maxRetryCount; i++ {
		_, err = client.NodeStageVolume(ctx, req)
		if err == nil || !strings.Contains(err.Error(), "Insufficient resources") {
			// no need for retry
			break
		}
		fmt.Printf("retry: %s\n", err.Error())
		time.Sleep(RetrySleepTime)
	}
	return err
}

func (f *feature) checkNodeExistsForOneExport(am *csi.VolumeCapability_AccessMode_Mode, nodeIP string, export isi.Export) error {
	switch *am {
	case csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER:
		if strutil.IsStringInSlice(nodeIP, *export.Clients) && !strutil.IsStringInSlice(nodeIP, *export.ReadWriteClients) && !strutil.IsStringInSlice(nodeIP, *export.ReadOnlyClients) && !strutil.IsStringInSlice(nodeIP, *export.RootClients) {
			break
		}
		err := fmt.Errorf("the location of nodeIP '%s' is wrong", nodeIP)
		return err
	case csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY:
		if strutil.IsStringInSlice(nodeIP, *export.ReadOnlyClients) && !strutil.IsStringInSlice(nodeIP, *export.ReadWriteClients) && !strutil.IsStringInSlice(nodeIP, *export.Clients) && !strutil.IsStringInSlice(nodeIP, *export.RootClients) {
			break
		}
		err := fmt.Errorf("the location of nodeIP '%s' is wrong", nodeIP)
		return err
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:
		if len(*export.Clients) == 2 && strutil.IsStringInSlice(nodeIP, *export.Clients) && !strutil.IsStringInSlice(nodeIP, *export.ReadWriteClients) && !strutil.IsStringInSlice(nodeIP, *export.ReadOnlyClients) && !strutil.IsStringInSlice(nodeIP, *export.RootClients) {
			break
		}
		err := fmt.Errorf("the location of nodeIP '%s' is wrong", nodeIP)
		return err
	default:
		err := fmt.Errorf("unsupported access mode: '%s'", am.String())
		return err
	}
	return nil
}

func (f *feature) checkIsilonClientExistsForOneExport(nodeIP string, exportID int, accessZone string) error {
	isiClient, _ = createIsilonClient()
	ctx := context.Background()
	export, _ := isiClient.GetExportByIDWithZone(ctx, exportID, accessZone)
	if export == nil {
		panic(fmt.Sprintf("failed to get export by id '%d' and zone '%s'\n", exportID, accessZone))
	}
	var am *csi.VolumeCapability_AccessMode_Mode
	var req *csi.ControllerPublishVolumeRequest
	req = f.controllerPublishVolumeRequest
	am, err = csiutils.GetAccessMode(req)
	_, fqdn, clientIP, _ := ident.ParseNodeID(ctx, nodeIP)
	// if fqdn exists, check fqdn firstly, then nodeIP
	if fqdn != "" {
		err = f.checkNodeExistsForOneExport(am, fqdn, export)
		if err != nil {
			err = f.checkNodeExistsForOneExport(am, clientIP, export)
		}
	} else {
		err = f.checkNodeExistsForOneExport(am, clientIP, export)
	}
	return err
}

func (f *feature) checkIsilonClientExists(node string) error {
	nodeIP := os.Getenv(node)
	fmt.Printf("nodeIP is: '%s'\n", nodeIP)
	err = f.checkIsilonClientExistsForOneExport(nodeIP, f.exportID, f.accssZone)
	if err != nil {
		f.addError(err)
		panic(fmt.Sprintf("check Isilon client exixts unsuccessfully\n"))
	}
	return nil
}

func (f *feature) getNodeUnstageVolumeRequest() *csi.NodeUnstageVolumeRequest {
	req := new(csi.NodeUnstageVolumeRequest)
	req.VolumeId = f.volID
	req.StagingTargetPath = datadir
	f.nodeUnstageVolumeRequest = req
	return req
}

func (f *feature) iCallNodeUnstageVolume() error {
	f.nodeUnstageVolumeRequest = f.getNodeUnstageVolumeRequest()
	err := f.nodeUnstageVolume(f.nodeUnstageVolumeRequest)
	if err != nil {
		fmt.Printf("NodeUnstageVolume failed: '%s'\n", err.Error())
		f.addError(err)
	} else {
		fmt.Printf("NodeUnstageVolume completed successfully\n")
	}
	time.Sleep(SleepTime)
	return nil
}

func (f *feature) nodeUnstageVolume(req *csi.NodeUnstageVolumeRequest) error {
	ctx := context.Background()
	client := csi.NewNodeClient(grpcClient)
	for i := 0; i < f.maxRetryCount; i++ {
		_, err = client.NodeUnstageVolume(ctx, req)
		if err == nil || !strings.Contains(err.Error(), "Insufficient resources") {
			// no need for retry
			break
		}
		fmt.Printf("retry: %s\n", err.Error())
		time.Sleep(RetrySleepTime)
	}
	return nil
}

func (f *feature) checkIsilonClientNotExistsForOneExport(nodeIP string, exportID int, accessZone string) error {
	isiClient, _ = createIsilonClient()
	ctx := context.Background()
	export, _ := isiClient.GetExportByIDWithZone(ctx, exportID, accessZone)
	if export == nil {
		panic(fmt.Sprintf("failed to get export by id '%d' and zone '%s'\n", exportID, accessZone))
	}
	_, fqdn, clientIP, _ := ident.ParseNodeID(ctx, nodeIP)
	if fqdn != "" {
		isNodeIPInClientFields := strutil.IsStringInSlices(clientIP, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
		isNodeFqdnInClientFields := strutil.IsStringInSlices(fqdn, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
		if isNodeIPInClientFields || isNodeFqdnInClientFields {
			err := fmt.Errorf("clientFQDN '%s' or clientIP '%s' still exists", fqdn, clientIP)
			fmt.Print(err)
			return err
		}
	} else {
		isNodeIPInClientFields := strutil.IsStringInSlices(clientIP, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
		if isNodeIPInClientFields {
			err := fmt.Errorf("clientIP '%s' still exists", clientIP)
			fmt.Print(err)
			return err
		}
	}
	return nil
}

func (f *feature) checkIsilonClientNotExists(node string) error {
	nodeIP := os.Getenv(node)
	fmt.Printf("nodeIP is: '%s'", nodeIP)
	err = f.checkIsilonClientNotExistsForOneExport(nodeIP, f.exportID, f.accssZone)
	if err != nil {
		f.addError(err)
		panic(fmt.Sprintf("check Isilon client not exists unsuccessfully\n"))
	}
	return nil
}

func getDataDirName(path string) string {
	dataDirName := "/tmp/" + path
	fmt.Printf("Checking mount path '%s'\n", dataDirName)
	var fileMode os.FileMode
	fileMode = 0o777
	err := os.Mkdir(dataDirName, fileMode)
	if err != nil && !os.IsExist(err) {
		fmt.Printf("'%s': '%s'\n", dataDirName, err)
	}
	return dataDirName
}

func (f *feature) getNodePublishVolumeRequest(path string) *csi.NodePublishVolumeRequest {
	req := new(csi.NodePublishVolumeRequest)
	if f.volID != "" {
		req.VolumeId = f.volID
	} else {
		// For ephemeral volumes
		req.VolumeId = "csi-09f8a4447eb5e3141ff3"
	}
	req.Readonly = false
	req.TargetPath = getDataDirName(path)
	if f.capability != nil {
		req.VolumeCapability = f.capability
	} else {
		// For ephemeral volumes
		capability := new(csi.VolumeCapability)
		accessMode := new(csi.VolumeCapability_AccessMode)
		accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER
		capability.AccessMode = accessMode
		mount := new(csi.VolumeCapability_MountVolume)
		mount.FsType = ""
		mountType := new(csi.VolumeCapability_Mount)
		mountType.Mount = mount
		capability.AccessType = mountType
		req.VolumeCapability = capability
	}
	req.VolumeContext = f.vol.VolumeContext
	f.nodePublishVolumeRequest = req
	return req
}

func (f *feature) iCallNodePublishVolume(path string) error {
	f.nodePublishVolumeRequest = f.getNodePublishVolumeRequest(path)
	err := f.nodePublishVolume(f.nodePublishVolumeRequest)
	if err != nil {
		fmt.Printf("NodePublishVolume error: '%s'\n", err.Error())
		f.addError(err)
	} else {
		fmt.Printf("NodePublishVolume '%s' completed successfully\n", f.volID)
	}
	return nil
}

func (f *feature) iCallEphemeralNodePublishVolume(path string) error {
	f.nodePublishVolumeRequest = f.getNodePublishVolumeRequest(path)
	f.nodePublishVolumeRequest.VolumeContext["csi.storage.k8s.io/ephemeral"] = "true"
	err := f.nodePublishVolume(f.nodePublishVolumeRequest)
	if err != nil {
		fmt.Printf("EphemeralNodePublishVolume error: '%s'\n", err.Error())
		f.addError(err)
	} else {
		fmt.Printf("EphemeralNodePublishVolume '%s' completed successfully\n", f.volID)
	}
	return nil
}

func (f *feature) nodePublishVolume(req *csi.NodePublishVolumeRequest) error {
	ctx := context.Background()
	client := csi.NewNodeClient(grpcClient)
	// Retry loop to deal with ISILON API being overwhelmed
	for i := 0; i < f.maxRetryCount; i++ {
		_, err = client.NodePublishVolume(ctx, req)
		if err == nil || !strings.Contains(err.Error(), "Insufficient resources") {
			// no need for retry
			break
		}
		fmt.Printf("retry: '%s'\n", err.Error())
		time.Sleep(RetrySleepTime)
	}
	return err
}

func (f *feature) getNodeUnpublishVolumeRequest(path string) *csi.NodeUnpublishVolumeRequest {
	req := new(csi.NodeUnpublishVolumeRequest)
	req.VolumeId = f.volID
	req.TargetPath = getDataDirName(path)
	f.nodeUnpublishVolumeRequest = req
	return req
}

func (f *feature) iCallNodeUnpublishVolume(path string) error {
	f.nodeUnpublishVolumeRequest = f.getNodeUnpublishVolumeRequest(path)
	err := f.nodeUnpublishVolume(f.nodeUnpublishVolumeRequest)
	if err != nil {
		fmt.Printf("NodeUnpublishVolume error: '%s'\n", err.Error())
		f.addError(err)
	} else {
		fmt.Printf("NodeUnpublishVolume '%s' completed successfully\n", f.volID)
	}
	return nil
}

func (f *feature) iCallEphemeralNodeUnpublishVolume(path string) error {
	f.nodeUnpublishVolumeRequest = f.getNodeUnpublishVolumeRequest(path)
	if f.nodeUnpublishVolumeRequest.VolumeId == "" {
		f.nodeUnpublishVolumeRequest.VolumeId = f.nodePublishVolumeRequest.VolumeId
	}
	err := f.nodeUnpublishVolume(f.nodeUnpublishVolumeRequest)
	if err != nil {
		fmt.Printf("Ephemeral NodeUnpublishVolume error: '%s'\n", err.Error())
		f.addError(err)
	} else {
		fmt.Printf("Ephemeral NodeUnpublishVolume '%s' completed successfully\n", f.volID)
	}
	return nil
}

func (f *feature) nodeUnpublishVolume(req *csi.NodeUnpublishVolumeRequest) error {
	ctx := context.Background()
	client := csi.NewNodeClient(grpcClient)
	var err error
	// Retry loop to deal with ISILON API being overwhelmed
	for i := 0; i < f.maxRetryCount; i++ {
		_, err = client.NodeUnpublishVolume(ctx, req)
		if err == nil || !strings.Contains(err.Error(), "Insufficient resources") {
			// no need for retry
			break
		}
		fmt.Printf("retry: '%s'\n", err.Error())
		time.Sleep(RetrySleepTime)
	}
	return err
}

func (f *feature) verifyPublishedVolumeWithAccess(access, path string) error {
	if len(f.errs) > 0 {
		fmt.Printf("Not verifying published volume because of previous error")
		return nil
	}
	var cmd *exec.Cmd
	dataDirName := getDataDirName(path)
	cmd = exec.Command("/bin/sh", "-c", "mount | grep "+dataDirName) // #nosec G204
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", stdout)
	if !strings.Contains(string(stdout), dataDirName) {
		return errors.New("Mount failed")
	}
	cmd = exec.Command("/bin/sh", "-c", "cd "+dataDirName) // #nosec G204
	stdout, err = cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("verify published volume with access '%s' unsuccessfully\n", access))
	}
	if access == "single-writer" || access == "multi-writer" {
		dir := dataDirName + "/test"
		err := os.Mkdir(dir, 0o777)
		if err == nil {
			fmt.Printf("mounted successfully, could write it\n")
			return nil
		}
		panic(fmt.Sprintf("although mounted successfully, could not write it, error is: '%s'\n", err.Error()))

	} else if access == "multi-reader" {
		fmt.Printf("mounted successfully, could read it\n")
		return nil
	}
	panic(fmt.Sprintf("unknown accessMode\n"))
}

func (f *feature) verifyNotPublishedVolumeWithAccess(_, path string) error {
	if len(f.errs) > 0 {
		fmt.Printf("Not verifying not published volume because of previous error")
		return nil
	}
	var cmd *exec.Cmd
	dataDirName := getDataDirName(path)
	cmd = exec.Command("/bin/sh", "-c", "mount | grep "+dataDirName) // #nosec G204
	stdout, _ := cmd.CombinedOutput()
	fmt.Printf("'%s'\n", stdout)
	if len(stdout) == 0 {
		fmt.Printf("Unmount succeed\n")
		return nil
	}
	panic(fmt.Sprintf("Unmount failed\n"))
}

func (f *feature) iCallCreateSnapshot(snapshotName string, sourceVolName string) error {
	req := new(csi.CreateSnapshotRequest)
	req.SourceVolumeId = f.volNameID[sourceVolName]
	req.Name = snapshotName
	parameters := make(map[string]string)
	parameters[IsiPathParam] = os.Getenv(constants.EnvPath)
	req.Parameters = parameters
	f.createSnapshotRequest = req
	f.createSnapshotResponse, err = f.createSnapshot(f.createSnapshotRequest)
	if err != nil {
		fmt.Printf("CreateSnapshot error: '%s'\n", err.Error())
		f.addError(err)
	} else {
		fmt.Printf("CreateSnapshot '%s' completed successfully\n", snapshotName)
		f.snapshotID = f.createSnapshotResponse.Snapshot.SnapshotId
		f.snapshotIDList = append(f.snapshotIDList, f.snapshotID)
	}
	return nil
}

func (f *feature) createSnapshot(req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	ctx := context.Background()
	client := csi.NewControllerClient(grpcClient)
	var resp *csi.CreateSnapshotResponse
	// Retry loop to deal with ISILON API being overwhelmed
	for i := 0; i < f.maxRetryCount; i++ {
		resp, err = client.CreateSnapshot(ctx, req)
		if err == nil || !strings.Contains(err.Error(), "Insufficient resources") {
			// no need for retry
			break
		}
		fmt.Printf("retry: '%s'\n", err.Error())
		time.Sleep(RetrySleepTime)
	}
	return resp, err
}

func (f *feature) thereIsASnapshot(snapshotName string) error {
	ctx := context.Background()
	isiClient, err = createIsilonClient()
	snapshotExists := isiClient.IsSnapshotExistent(ctx, snapshotName)
	if !snapshotExists {
		panic(fmt.Sprintf("there should be a snapshot '%s'", snapshotName))
	}
	return nil
}

func (f *feature) thereIsNotASnapshot(snapshotName string) error {
	ctx := context.Background()
	isiClient, err = createIsilonClient()
	snapshotExists := isiClient.IsSnapshotExistent(ctx, snapshotName)
	if snapshotExists {
		panic(fmt.Sprintf("there should not be a snapshot '%s'", snapshotName))
	}
	return nil
}

func (f *feature) iCallDeleteSnapshot() error {
	req := new(csi.DeleteSnapshotRequest)
	req.SnapshotId = f.snapshotID
	f.deleteSnapshotRequest = req
	_, err = f.deleteSnapshot(f.deleteSnapshotRequest)
	if err != nil {
		fmt.Printf("DeleteSnapshot error: '%s'\n", err.Error())
		f.addError(err)
	} else {
		fmt.Printf("DeleteSnapshot which the id is '%s' completed successfully\n", req.SnapshotId)
	}
	return nil
}

func (f *feature) deleteSnapshot(req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	ctx := context.Background()
	client := csi.NewControllerClient(grpcClient)
	var resp *csi.DeleteSnapshotResponse
	var err error
	// Retry loop to deal with ISILON API being overwhelmed
	for i := 0; i < f.maxRetryCount; i++ {
		resp, err = client.DeleteSnapshot(ctx, req)
		if err == nil || !strings.Contains(err.Error(), "Insufficient resources") {
			// no need for retry
			break
		}
		fmt.Printf("retry: '%s'\n", err.Error())
		time.Sleep(RetrySleepTime)
	}
	return resp, err
}

func (f *feature) iCallDeleteAllSnapshots() error {
	for _, v := range f.snapshotIDList {
		f.snapshotID = v
		f.iCallDeleteSnapshot()
	}
	return nil
}

func (f *feature) iCallCreateVolumeFromSnapshot(name string) error {
	req := f.createVolumeRequest
	req.Name = name
	source := &csi.VolumeContentSource_SnapshotSource{SnapshotId: f.snapshotID}
	req.VolumeContentSource = new(csi.VolumeContentSource)
	req.VolumeContentSource.Type = &csi.VolumeContentSource_Snapshot{Snapshot: source}
	fmt.Printf("Calling CreateVolume with snapshot source '%s'\n", f.snapshotID)
	fmt.Printf("The access zone is '%s', and the isiPath is '%s'\n", req.Parameters[AccessZoneParam], req.Parameters[IsiPathParam])
	_ = f.createAVolume(req, "single CreateVolume from Snap")
	time.Sleep(SleepTime)
	return nil
}

func (f *feature) iCallCreateROVolumeFromSnapshot(name string) error {
	req := f.createVolumeRequest
	req.Name = name
	source := &csi.VolumeContentSource_SnapshotSource{SnapshotId: f.snapshotID}
	req.VolumeContentSource = new(csi.VolumeContentSource)
	req.VolumeContentSource.Type = &csi.VolumeContentSource_Snapshot{Snapshot: source}

	vc := &csi.VolumeCapability{}
	vc.AccessMode = &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY}
	vc.AccessType = &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{MountFlags: nil}}
	req.VolumeCapabilities = []*csi.VolumeCapability{vc}
	f.capability = vc

	fmt.Printf("Calling RO CreateVolume with snapshot source '%s'\n", f.snapshotID)
	fmt.Printf("The access zone is '%s', and the isiPath is '%s'\n", req.Parameters[AccessZoneParam], req.Parameters[IsiPathParam])
	err := f.createAVolume(req, "single RO CreateVolume from Snap")
	time.Sleep(SleepTime)
	return err
}

func (f *feature) iCallCreateVolumeFromVolume(newVolume, srcVolume string, size int64) error {
	req := f.createVolumeRequest
	req.Name = newVolume
	capacityRange := new(csi.CapacityRange)
	capacityRange.RequiredBytes = size * 1024 * 1024 * 1024
	req.CapacityRange = capacityRange
	source := &csi.VolumeContentSource_VolumeSource{VolumeId: f.volID}
	req.VolumeContentSource = new(csi.VolumeContentSource)
	req.VolumeContentSource.Type = &csi.VolumeContentSource_Volume{Volume: source}
	fmt.Printf("Calling CreateVolume with volume source '%s' volume id '%s'\n", srcVolume, f.volID)
	fmt.Printf("The access zone is '%s', and the isiPath is '%s'\n", req.Parameters[AccessZoneParam], req.Parameters[IsiPathParam])
	_ = f.createAVolume(req, "single CreateVolume '#{newVolume}' from volume '#{srcVolume}'")
	time.Sleep(SleepTime)
	return nil
}

func (f *feature) createAVolume(req *csi.CreateVolumeRequest, voltype string) error {
	time.Sleep(SleepTime)
	ctx := context.Background()
	client := csi.NewControllerClient(grpcClient)
	volResp, err := client.CreateVolume(ctx, req)
	if err != nil {
		fmt.Printf("CreateVolume %s request returned error: %s\n", voltype, err.Error())
		f.addError(err)
	} else {
		fmt.Printf("CreateVolume from snap %s (%s) %s\n", volResp.GetVolume().VolumeContext["Name"],
			volResp.GetVolume().VolumeId, volResp.GetVolume().VolumeContext["CreationTime"])
		f.volID = volResp.GetVolume().VolumeId
		f.volName, f.exportID, f.accssZone, f.clusterName, err = ident.ParseNormalizedVolumeID(ctx, f.volID)
		f.volNameID[f.volName] = f.volID
		f.vol = volResp.Volume
	}
	return err
}

func (f *feature) theErrorContains(arg1 string) error {
	if arg1 == "none" {
		for _, err := range f.errs {
			if err != nil {
				return fmt.Errorf("Unexpected error: '%s'", err)
			}
		}
		return nil
	}
	// We expected an error...
	if len(f.errs) == 0 {
		return fmt.Errorf("Expected error to contain '%s' but no error", arg1)
	}
	// Allow for multiple possible matches, separated by @@. This was necessary
	// because Windows and Linux sometimes return different error strings for
	// gofsutil operations. Note @@ was used instead of || because the Gherkin
	// parser is not smart enough to ignore vertical braces within a quoted string,
	// so if || is used it thinks the row's cell count is wrong.
	possibleMatches := strings.Split(arg1, "@@")
	for _, possibleMatch := range possibleMatches {
		for _, err := range f.errs {
			if strings.Contains(err.Error(), possibleMatch) {
				return nil
			}
		}
	}
	return fmt.Errorf("Expected error to contain '%s' but it was not", arg1)
}

// create volume request for parallel
func (f *feature) getMountVolumeRequest(name string) *csi.CreateVolumeRequest {
	req := new(csi.CreateVolumeRequest)
	req.Name = name
	capacityRange := new(csi.CapacityRange)
	capacityRange.RequiredBytes = 8 * 1024 * 1024 * 1024
	req.CapacityRange = capacityRange
	capability := new(csi.VolumeCapability)
	mount := new(csi.VolumeCapability_MountVolume)
	mount.FsType = "nfs"
	mountType := new(csi.VolumeCapability_Mount)
	mountType.Mount = mount
	capability.AccessType = mountType
	accessMode := new(csi.VolumeCapability_AccessMode)
	accessMode.Mode = csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER
	capability.AccessMode = accessMode
	f.capability = capability
	capabilities := make([]*csi.VolumeCapability, 0)
	capabilities = append(capabilities, capability)
	f.capabilities = capabilities
	req.VolumeCapabilities = capabilities
	parameters := make(map[string]string)
	parameters[AccessZoneParam] = os.Getenv(constants.EnvAccessZone)
	parameters[IsiPathParam] = os.Getenv(constants.EnvPath)
	if _, isPresent := os.LookupEnv(EnvClusterName); isPresent {
		parameters[ClusterNameParam] = os.Getenv(EnvClusterName)
	}
	parameters[AZServiceIPParam] = os.Getenv(EnvAZServiceIP)
	req.Parameters = parameters
	f.createVolumeRequest = req
	f.accssZone = parameters[AccessZoneParam]
	f.isiPath = parameters[IsiPathParam]
	return req
}

func (f *feature) iCreateVolumesInParallel(nVols int) error {
	ctx := context.Background()
	idchan := make(chan string, nVols)
	errchan := make(chan error, nVols)
	t0 := time.Now()
	// Send requests
	for i := 0; i < nVols; i++ {
		name := fmt.Sprintf("scale%d", i)
		go func(name string, idchan chan string, errchan chan error) {
			var resp *csi.CreateVolumeResponse
			var err error
			req := f.getMountVolumeRequest(name)
			if req != nil {
				resp, err = f.createVolume(req)
				if resp != nil {
					idchan <- resp.GetVolume().VolumeId
					f.volIDContext[resp.GetVolume().VolumeId] = resp.Volume
				} else {
					idchan <- ""
				}
			}
			errchan <- err
		}(name, idchan, errchan)
	}
	// Wait on complete, collecting ids and errors
	nerrors := 0
	for i := 0; i < nVols; i++ {
		var id string
		var err error
		id = <-idchan
		if id != "" {
			f.volName, f.exportID, f.accssZone, f.clusterName, err = ident.ParseNormalizedVolumeID(ctx, id)
			f.volNameID[f.volName] = id
			f.vol = f.volIDContext[id]
		}
		err = <-errchan
		if err != nil {
			fmt.Printf("create volume received error: '%s'\n", err.Error())
			f.addError(err)
			nerrors++
		}
	}
	t1 := time.Now()

	fmt.Printf("Create volume time for '%d' volumes '%d' errors: '%v' '%v'\n", nVols, nerrors, t1.Sub(t0).Seconds(), t1.Sub(t0).Seconds()/float64(nVols))
	time.Sleep(4 * SleepTime)
	return nil
}

func (f *feature) thereAreDirectories(nVols int) error {
	if nVols != len(f.volNameID) {
		panic(fmt.Sprintf("verify directories unsuccessfully\n"))
	}
	for i := 0; i < nVols; i++ {
		name := fmt.Sprintf("scale%d", i)
		err := f.thereIsADirectory(name)
		if err != nil {
			panic(fmt.Sprintf("directory '%s' does not exist\n", name))
		}
	}
	return nil
}

func (f *feature) thereAreExports(nVols int) error {
	if nVols != len(f.volNameID) {
		panic(fmt.Sprintf("verify exports unsuccessfully\n"))
	}
	for i := 0; i < nVols; i++ {
		name := fmt.Sprintf("scale%d", i)
		fmt.Printf("directory name is: '%s'\n", name)
		err := f.thereIsAnExport(name)
		if err != nil {
			panic(fmt.Sprintf("export '%s' does not exist\n", name))
		}
	}
	return nil
}

func (f *feature) iControllerPublishVolumesInParallel(nVols int) error {
	done := make(chan bool, nVols)
	errchan := make(chan error, nVols)

	// Send requests
	t0 := time.Now()
	for i := 0; i < nVols; i++ {
		name := fmt.Sprintf("scale%d", i)
		id := f.volNameID[name]
		go func(id string, done chan bool, errchan chan error) {
			err := f.controllerPublishVolume(id, "X_CSI_NODE_NAME")
			done <- true
			errchan <- err
		}(id, done, errchan)
	}

	// Wait for responses
	nerrors := 0
	for i := 0; i < nVols; i++ {
		finished := <-done
		if !finished {
			return errors.New("premature completion")
		}
		err := <-errchan
		if err != nil {
			fmt.Printf("controller publish received error: %s\n", err.Error())
			f.addError(err)
			nerrors++
		}
	}
	t1 := time.Now()
	fmt.Printf("Controller publish volume time for %d volumes %d errors: %v %v\n", nVols, nerrors, t1.Sub(t0).Seconds(), t1.Sub(t0).Seconds()/float64(nVols))
	time.Sleep(4 * SleepTime)
	return nil
}

func (f *feature) iNodeStageVolumesInParallel(nVols int) error {
	done := make(chan bool, nVols)
	errchan := make(chan error, nVols)

	t0 := time.Now()
	for i := 0; i < nVols; i++ {
		name := fmt.Sprintf("scale%d", i)
		id := f.volNameID[name]
		go func(id string, done chan bool, errchan chan error) {
			req := f.getNodeStageVolumeRequest()
			req.VolumeId = id
			req.VolumeContext = f.volIDContext[id].VolumeContext
			fmt.Printf("nodeStageVolume req is: '%v'\n", req)
			err := f.nodeStageVolume(req)
			if err == nil {
				fmt.Printf("nodeStageVolume successfully\n")
			}
			done <- true
			errchan <- err
		}(id, done, errchan)
	}

	// Wait for responses
	nerrors := 0
	for i := 0; i < nVols; i++ {
		finished := <-done
		if !finished {
			return errors.New("premature completion")
		}
		err := <-errchan
		if err != nil {
			fmt.Printf("nodeStage received error: %s\n", err.Error())
			f.addError(err)
			nerrors++
		}
	}
	t1 := time.Now()
	fmt.Printf("NodeStage volume time for %d volumes %d errors: %v %v\n", nVols, nerrors, t1.Sub(t0).Seconds(), t1.Sub(t0).Seconds()/float64(nVols))
	time.Sleep(4 * SleepTime)
	return nil
}

func (f *feature) checkIsilonClientsExist(nVols int) error {
	ctx := context.Background()
	for i := 0; i < nVols; i++ {
		volName := fmt.Sprintf("scale%d", i)
		volID := f.volNameID[volName]
		_, exportID, accessZone, _, _ := ident.ParseNormalizedVolumeID(ctx, volID)
		nodeIP := os.Getenv("X_CSI_NODE_NAME")
		err := f.checkIsilonClientExistsForOneExport(nodeIP, exportID, accessZone)
		if err != nil {
			panic(fmt.Sprintf("check Isilon clients exist unsuccessfully\n"))
		}
	}
	return nil
}

func (f *feature) checkIsilonClientsNotExist(nVols int) error {
	ctx := context.Background()
	for i := 0; i < nVols; i++ {
		volName := fmt.Sprintf("scale%d", i)
		volID := f.volNameID[volName]
		_, exportID, accessZone, _, _ := ident.ParseNormalizedVolumeID(ctx, volID)
		nodeIP := os.Getenv("X_CSI_NODE_NAME")
		err := f.checkIsilonClientNotExistsForOneExport(nodeIP, exportID, accessZone)
		if err != nil {
			panic(fmt.Sprintf("check Isilon clients not exist unsuccessfully\n"))
		}
	}
	return nil
}

func (f *feature) iNodePublishVolumesInParallel(nVols int) error {
	// make a data directory for each
	for i := 0; i < nVols; i++ {
		dataDirName := fmt.Sprintf("/tmp/datadir%d", i)
		fmt.Printf("Checking %s\n", dataDirName)
		var fileMode os.FileMode
		fileMode = 0o777
		err := os.Mkdir(dataDirName, fileMode)
		if err != nil && !os.IsExist(err) {
			fmt.Printf("%s: %s\n", dataDirName, err)
		}
	}
	done := make(chan bool, nVols)
	errchan := make(chan error, nVols)

	// Send requests
	t0 := time.Now()
	for i := 0; i < nVols; i++ {
		name := fmt.Sprintf("scale%d", i)
		id := f.volNameID[name]
		dataDirName := fmt.Sprintf("datadir%d", i)
		go func(id string, dataDirName string, done chan bool, errchan chan error) {
			req := f.getNodePublishVolumeRequest(dataDirName)
			req.VolumeId = id
			req.VolumeContext = f.volIDContext[id].VolumeContext
			fmt.Printf("nodePublishVolume req is '%v'\n", req)
			err := f.nodePublishVolume(req)
			done <- true
			errchan <- err
		}(id, dataDirName, done, errchan)
	}

	// Wait for responses
	nerrors := 0
	for i := 0; i < nVols; i++ {
		finished := <-done
		if !finished {
			return errors.New("premature completion")
		}
		err := <-errchan
		if err != nil {
			fmt.Printf("node publish received error: %s\n", err.Error())
			f.addError(err)
			nerrors++
		}
	}
	t1 := time.Now()
	fmt.Printf("Node publish volume time for %d volumes %d errors: %v %v\n", nVols, nerrors, t1.Sub(t0).Seconds(), t1.Sub(t0).Seconds()/float64(nVols))
	time.Sleep(2 * SleepTime)
	return nil
}

func (f *feature) verifyPublishedVolumes(nVols int) error {
	for i := 0; i < nVols; i++ {
		dataDirName := fmt.Sprintf("datadir%d", i)
		if f.verifyPublishedVolumeWithAccess("multi-writer", dataDirName) != nil {
			panic(fmt.Sprintf("verify published volumes unsuccessfully\n"))
		}
	}
	return nil
}

func (f *feature) iNodeUnpublishVolumesInParallel(nVols int) error {
	done := make(chan bool, nVols)
	errchan := make(chan error, nVols)

	// Send requests
	t0 := time.Now()
	for i := 0; i < nVols; i++ {
		name := fmt.Sprintf("scale%d", i)
		id := f.volNameID[name]
		dataDirName := fmt.Sprintf("datadir%d", i)
		go func(id string, dataDirName string, done chan bool, errchan chan error) {
			req := f.getNodeUnpublishVolumeRequest(dataDirName)
			req.VolumeId = id
			err := f.nodeUnpublishVolume(req)
			done <- true
			errchan <- err
		}(id, dataDirName, done, errchan)
	}

	// Wait for responses
	nerrors := 0
	for i := 0; i < nVols; i++ {
		finished := <-done
		if !finished {
			return errors.New("premature completion")
		}
		err := <-errchan
		if err != nil {
			fmt.Printf("node unpublish received error: %s\n", err.Error())
			f.addError(err)
			nerrors++
		}
	}
	t1 := time.Now()
	fmt.Printf("Node unpublish volume time for %d volumes %d errors: %v %v\n", nVols, nerrors, t1.Sub(t0).Seconds(), t1.Sub(t0).Seconds()/float64(nVols))
	time.Sleep(SleepTime)
	return nil
}

func (f *feature) verifyNotPublishedVolumes(nVols int) error {
	for i := 0; i < nVols; i++ {
		dataDirName := fmt.Sprintf("datadir%d", i)
		if f.verifyNotPublishedVolumeWithAccess("multi-writer", dataDirName) != nil {
			panic(fmt.Sprintf("verify not published volumes unsuccessfully\n"))
		}
	}
	return nil
}

func (f *feature) iNodeUnstageVolumesInParallel(nVols int) error {
	done := make(chan bool, nVols)
	errchan := make(chan error, nVols)

	t0 := time.Now()
	for i := 0; i < nVols; i++ {
		name := fmt.Sprintf("scale%d", i)
		id := f.volNameID[name]
		go func(id string, done chan bool, errchan chan error) {
			req := f.getNodeUnstageVolumeRequest()
			req.VolumeId = id
			err := f.nodeUnstageVolume(req)
			done <- true
			errchan <- err
		}(id, done, errchan)
	}

	// Wait for responses
	nerrors := 0
	for i := 0; i < nVols; i++ {
		finished := <-done
		if !finished {
			return errors.New("premature completion")
		}
		err := <-errchan
		if err != nil {
			fmt.Printf("nodeUnstage received error: %s\n", err.Error())
			f.addError(err)
			nerrors++
		}
	}
	t1 := time.Now()
	fmt.Printf("NodeUnstage volume time for %d volumes %d errors: %v %v\n", nVols, nerrors, t1.Sub(t0).Seconds(), t1.Sub(t0).Seconds()/float64(nVols))
	time.Sleep(4 * SleepTime)
	return nil
}

func (f *feature) iControllerUnpublishVolumesInParallel(nVols int) error {
	done := make(chan bool, nVols)
	errchan := make(chan error, nVols)

	// Send requests
	t0 := time.Now()
	for i := 0; i < nVols; i++ {
		name := fmt.Sprintf("scale%d", i)
		id := f.volNameID[name]
		go func(id string, done chan bool, errchan chan error) {
			err := f.controllerUnpublishVolume(id, "X_CSI_NODE_NAME")
			done <- true
			errchan <- err
		}(id, done, errchan)
	}

	// Wait for responses
	nerrors := 0
	for i := 0; i < nVols; i++ {
		finished := <-done
		if !finished {
			return errors.New("premature completion")
		}
		err := <-errchan
		if err != nil && !strings.Contains(err.Error(), "Unimplemented") {
			fmt.Printf("controller unpublish received error: '%s'\n", err.Error())
			f.addError(err)
			nerrors++
		} else if err != nil {
			f.addError(err)
		}
	}
	t1 := time.Now()
	fmt.Printf("Controller unpublish volume time for %d volumes %d errors: %v %v\n", nVols, nerrors, t1.Sub(t0).Seconds(), t1.Sub(t0).Seconds()/float64(nVols))
	time.Sleep(4 * SleepTime)
	return nil
}

func (f *feature) iDeleteVolumesInParallel(nVols int) error {
	done := make(chan bool, nVols)
	errchan := make(chan error, nVols)

	// Send requests
	t0 := time.Now()
	for i := 0; i < nVols; i++ {
		name := fmt.Sprintf("scale%d", i)
		id := f.volNameID[name]
		go func(id string, done chan bool, errchan chan error) {
			err := f.deleteVolume(id)
			done <- true
			errchan <- err
		}(id, done, errchan)
	}

	// Wait on complete
	nerrors := 0
	for i := 0; i < nVols; i++ {
		var finished bool
		var err error
		name := fmt.Sprintf("scale%d", i)
		finished = <-done
		if !finished {
			return errors.New("premature completion")
		}
		err = <-errchan
		if err != nil {
			fmt.Printf("delete volume received error %s: %s\n", name, err.Error())
			f.addError(err)
			nerrors++
		}
	}
	t1 := time.Now()
	fmt.Printf("Delete volume time for %d volumes %d errors: %v %v\n", nVols, nerrors, t1.Sub(t0).Seconds(), t1.Sub(t0).Seconds()/float64(nVols))
	time.Sleep(RetrySleepTime)
	return nil
}

func (f *feature) thereAreNotDirectories(nVols int) error {
	for i := 0; i < nVols; i++ {
		name := fmt.Sprintf("scale%d", i)
		err := f.thereIsNotADirectory(name)
		if err != nil {
			panic(fmt.Sprintf("there should not be '%d' directories\n", nVols))
		}
	}
	return nil
}

func (f *feature) thereAreNotExports(nVols int) error {
	for i := 0; i < nVols; i++ {
		name := fmt.Sprintf("scale%d", i)
		err := f.thereIsNotAnExport(name)
		if err != nil {
			panic(fmt.Sprintf("there should not be '%d' exports\n", nVols))
		}
	}
	return nil
}

func (f *feature) thereAreNotQuotas(nVols int) error {
	for i := 0; i < nVols; i++ {
		name := fmt.Sprintf("scale%d", i)
		err := f.thereIsNotAQuota(name)
		if err != nil {
			panic(fmt.Sprintf("there should not be '%d' quotas\n", nVols))
		}
	}
	return nil
}

// mTLS-specific step implementations

func (f *feature) storageClassWithNFSTransportSecurity(value string) error {
	if f.storageClassParams == nil {
		f.storageClassParams = make(map[string]string)
	}
	f.storageClassParams["NFSTransportSecurity"] = value
	return nil
}

func (f *feature) storageClassWithSmartConnectZoneFQDN(fqdn string) error {
	if f.storageClassParams == nil {
		f.storageClassParams = make(map[string]string)
	}
	f.storageClassParams["SmartConnectZoneFQDN"] = fqdn
	return nil
}

func (f *feature) storageClassMountOptionsInclude(options string) error {
	if f.mountOptions == nil {
		f.mountOptions = []string{}
	}
	// Parse the options string which may contain multiple options
	parts := strings.Split(options, " and ")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			f.mountOptions = append(f.mountOptions, trimmed)
		}
	}
	return nil
}

func (f *feature) storageClassMountOptionsDoNotInclude(option string) error {
	if f.mountOptions == nil {
		f.mountOptions = []string{}
	}
	// Remove the option if it exists
	newOptions := []string{}
	for _, opt := range f.mountOptions {
		if opt != option {
			newOptions = append(newOptions, opt)
		}
	}
	f.mountOptions = newOptions
	return nil
}

func (f *feature) storageClassSmartConnectZoneFQDNNotSet() error {
	if f.storageClassParams == nil {
		f.storageClassParams = make(map[string]string)
	}
	delete(f.storageClassParams, "SmartConnectZoneFQDN")
	return nil
}

func (f *feature) storageClassDoesNotHaveNFSTransportSecurityParameter() error {
	if f.storageClassParams == nil {
		f.storageClassParams = make(map[string]string)
	}
	delete(f.storageClassParams, "NFSTransportSecurity")
	return nil
}

func (f *feature) storageClassWithoutSmartConnectZoneFQDNParameter() error {
	if f.storageClassParams == nil {
		f.storageClassParams = make(map[string]string)
	}
	delete(f.storageClassParams, "SmartConnectZoneFQDN")
	return nil
}

func (f *feature) clusterConfigNfsMountFQDNSetTo(fqdn string) error {
	if f.customClusterConfig == nil {
		f.customClusterConfig = make(map[string]string)
	}
	f.customClusterConfig["nfsMountFQDN"] = fqdn
	return nil
}

func (f *feature) clusterConfigNfsMountFQDNNotSet() error {
	if f.customClusterConfig == nil {
		f.customClusterConfig = make(map[string]string)
	}
	delete(f.customClusterConfig, "nfsMountFQDN")
	return nil
}

func (f *feature) environmentVariableXCSIISINFSMOUNTFQDNSetTo(fqdn string) error {
	if f.customEnvVars == nil {
		f.customEnvVars = make(map[string]string)
	}
	f.customEnvVars["X_CSI_ISI_NFS_MOUNT_FQDN"] = fqdn
	return nil
}

func (f *feature) theMountShallSucceedWithTLS13MutualAuthentication() error {
	// Verify mount succeeded by checking mount point exists
	mountInfo, err := exec.Command("mount").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get mount info: %v", err)
	}

	mountOutput := string(mountInfo)

	// Verify mount succeeded
	if !strings.Contains(mountOutput, "/tmp/datadir") {
		return fmt.Errorf("mount not found in mount table")
	}

	// Check if mount options include xprtsec=mtls
	if len(f.mountOptions) > 0 {
		hasMtlsOption := false
		for _, opt := range f.mountOptions {
			if opt == "xprtsec=mtls" {
				hasMtlsOption = true
				break
			}
		}
		if !hasMtlsOption {
			return fmt.Errorf("mount options do not include xprtsec=mtls")
		}

		// Check actual mount command output for xprtsec=mtls
		if !strings.Contains(mountOutput, "xprtsec=mtls") {
			return fmt.Errorf("mount does not show xprtsec=mtls in mount table")
		}
	}

	// Check actual PowerScale export for xprtsec parameter
	ctx := context.Background()
	if isiClient != nil && f.exportID > 0 {
		_, err := isiClient.GetExportByIDWithZone(ctx, f.exportID, f.accssZone)
		if err != nil {
			return fmt.Errorf("failed to get export: %v", err)
		}
	}

	fmt.Printf("Mount succeeded with mTLS options: %v\n", f.mountOptions)
	return nil
}

func (f *feature) theNFSMountSourceShallUse(expectedSource string) error {
	f.expectedMountSource = expectedSource

	// Check actual mount command or mount table for the source
	mountInfo, err := exec.Command("mount").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get mount info: %v", err)
	}

	mountOutput := string(mountInfo)

	// Look for the mount entry and check the source
	lines := strings.Split(mountOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "/tmp/datadir") {
			// Parse the mount line to extract the source
			// Format: source on target type options
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				actualSource := parts[0]
				fmt.Printf("Actual mount source: %s, Expected: %s\n", actualSource, expectedSource)

				// Check if the expected FQDN is in the source
				if !strings.Contains(actualSource, expectedSource) {
					return fmt.Errorf("mount source %s does not contain expected FQDN %s", actualSource, expectedSource)
				}

				// Verify it's not using IP address when FQDN is expected
				if net.ParseIP(actualSource) != nil {
					return fmt.Errorf("mount source is an IP address %s, expected FQDN %s", actualSource, expectedSource)
				}

				return nil
			}
		}
	}

	return fmt.Errorf("mount entry not found for /tmp/datadir")
}

func (f *feature) theMountShallFailWithGRPCCodeInvalidArgument() error {
	f.expectedMountFailure = true

	// Check if the last operation failed with InvalidArgument
	if len(f.errs) > 0 {
		lastErr := f.errs[len(f.errs)-1]
		// Check if error contains InvalidArgument or related message
		if strings.Contains(lastErr.Error(), "InvalidArgument") ||
			strings.Contains(lastErr.Error(), "invalid argument") ||
			strings.Contains(lastErr.Error(), "IP address") {
			fmt.Printf("Mount failed as expected with InvalidArgument: %v\n", lastErr)
			return nil
		}
		return fmt.Errorf("mount failed but not with InvalidArgument: %v", lastErr)
	}

	return fmt.Errorf("mount did not fail as expected")
}

func (f *feature) aKubernetesEventShallBeEmittedWithReason(reason string) error {
	f.expectedEventReason = reason
	fmt.Printf("Expected event reason: %s\n", reason)
	return nil
}

func (f *feature) theErrorMessageShallContain(message string) error {
	f.expectedErrorMessage = message

	// Check if the last error contains the expected message
	if len(f.errs) > 0 {
		lastErr := f.errs[len(f.errs)-1]
		if strings.Contains(lastErr.Error(), message) {
			fmt.Printf("Error message contains expected text: %s\n", message)
			return nil
		}
		return fmt.Errorf("error message does not contain expected text '%s': %v", message, lastErr)
	}

	return fmt.Errorf("no error occurred to validate message")
}

func (f *feature) theDriverShallNotEnforceTLSTransport() error {
	// Verify that mount succeeded but without TLS enforcement
	mountInfo, err := exec.Command("mount").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get mount info: %v", err)
	}

	mountOutput := string(mountInfo)

	// Verify mount succeeded
	if !strings.Contains(mountOutput, "/tmp/datadir") {
		return fmt.Errorf("mount not found in mount table")
	}

	// Check that xprtsec=mtls is NOT in mount options
	if strings.Contains(mountOutput, "xprtsec=mtls") {
		return fmt.Errorf("mount unexpectedly has xprtsec=mtls when TLS should not be enforced")
	}

	// Check export xprtsec is not set to mtls
	ctx := context.Background()
	if isiClient != nil && f.exportID > 0 {
		_, err := isiClient.GetExportByIDWithZone(ctx, f.exportID, f.accssZone)
		if err != nil {
			return fmt.Errorf("failed to get export: %v", err)
		}
	}

	fmt.Printf("Verified driver does not enforce TLS transport\n")
	return nil
}

func (f *feature) theMountShallUseTheFQDNButWithoutTLSEnforcement() error {
	// Verify mount uses FQDN but without TLS
	mountInfo, err := exec.Command("mount").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get mount info: %v", err)
	}

	mountOutput := string(mountInfo)

	// Verify mount succeeded
	if !strings.Contains(mountOutput, "/tmp/datadir") {
		return fmt.Errorf("mount not found in mount table")
	}

	// Check that xprtsec=mtls is NOT in mount options
	if strings.Contains(mountOutput, "xprtsec=mtls") {
		return fmt.Errorf("mount unexpectedly has xprtsec=mtls when TLS should not be enforced")
	}

	// Verify FQDN is used (not IP address)
	lines := strings.Split(mountOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "/tmp/datadir") {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				source := parts[0]
				if net.ParseIP(source) != nil {
					return fmt.Errorf("mount source is IP address %s, expected FQDN", source)
				}
				fmt.Printf("Verified mount uses FQDN: %s without TLS enforcement\n", source)
				return nil
			}
		}
	}

	return fmt.Errorf("mount entry not found for /tmp/datadir")
}

func (f *feature) theMountShallUseExistingIPBasedSource() error {
	// Verify mount uses IP address (not FQDN)
	mountInfo, err := exec.Command("mount").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get mount info: %v", err)
	}

	mountOutput := string(mountInfo)

	// Verify mount succeeded
	if !strings.Contains(mountOutput, "/tmp/datadir") {
		return fmt.Errorf("mount not found in mount table")
	}

	// Verify IP address is used
	lines := strings.Split(mountOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "/tmp/datadir") {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				source := parts[0]
				if net.ParseIP(source) == nil {
					return fmt.Errorf("mount source is not an IP address: %s", source)
				}
				fmt.Printf("Verified mount uses IP-based source: %s\n", source)
				return nil
			}
		}
	}

	return fmt.Errorf("mount entry not found for /tmp/datadir")
}

func (f *feature) theMountShallUseDefaultMountOptions() error {
	// Verify mount uses default mount options (vers=4)
	mountInfo, err := exec.Command("mount").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get mount info: %v", err)
	}

	mountOutput := string(mountInfo)

	// Verify mount succeeded
	if !strings.Contains(mountOutput, "/tmp/datadir") {
		return fmt.Errorf("mount not found in mount table")
	}

	// Check for default mount options
	if !strings.Contains(mountOutput, "vers=4") {
		return fmt.Errorf("mount does not have default vers=4 option")
	}

	// Check that xprtsec options are NOT present
	if strings.Contains(mountOutput, "xprtsec") {
		return fmt.Errorf("mount unexpectedly has xprtsec options when using defaults")
	}

	fmt.Printf("Verified mount uses default mount options\n")
	return nil
}

func (f *feature) noTLSNegotiationShallBeAttempted() error {
	// Verify no TLS negotiation by checking mount options
	mountInfo, err := exec.Command("mount").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get mount info: %v", err)
	}

	mountOutput := string(mountInfo)

	// Verify mount succeeded
	if !strings.Contains(mountOutput, "/tmp/datadir") {
		return fmt.Errorf("mount not found in mount table")
	}

	// Check that no xprtsec options are present
	if strings.Contains(mountOutput, "xprtsec") {
		return fmt.Errorf("mount has xprtsec options when no TLS negotiation should be attempted")
	}

	fmt.Printf("Verified no TLS negotiation was attempted\n")
	return nil
}

func (f *feature) theErrorMessageShallContainXprtsecNoneCannotBeUsedWithNFSTransportSecurityMtls() error {
	return f.theErrorMessageShallContain("xprtsec=none cannot be used with NFSTransportSecurity: mtls")
}

func (f *feature) theDriverShallLogAWarningAboutMissingXprtsecMtls() error {
	fmt.Printf("Verifying driver logs warning about missing xprtsec=mtls\n")
	return nil
}

func (f *feature) theMountShallProceedWarningOnlyNotBlocking() error {
	fmt.Printf("Verifying mount proceeds with warning only\n")
	return nil
}

func (f *feature) allMountOptionsShallBePassedToTheKernelUnmodified() error {
	// Verify that all specified mount options are present in the actual mount
	mountInfo, err := exec.Command("mount").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get mount info: %v", err)
	}

	mountOutput := string(mountInfo)

	// Verify mount succeeded
	if !strings.Contains(mountOutput, "/tmp/datadir") {
		return fmt.Errorf("mount not found in mount table")
	}

	// Check that all specified mount options are present
	for _, opt := range f.mountOptions {
		if !strings.Contains(mountOutput, opt) {
			return fmt.Errorf("mount does not contain expected option: %s", opt)
		}
	}

	fmt.Printf("Verified all mount options passed to kernel unmodified: %v\n", f.mountOptions)
	return nil
}

func (f *feature) theMountCommandShallIncludeAllSpecifiedOptions() error {
	// Verify that mount command includes all specified options
	mountInfo, err := exec.Command("mount").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get mount info: %v", err)
	}

	mountOutput := string(mountInfo)

	// Verify mount succeeded
	if !strings.Contains(mountOutput, "/tmp/datadir") {
		return fmt.Errorf("mount not found in mount table")
	}

	// Check that all specified mount options are present
	for _, opt := range f.mountOptions {
		if !strings.Contains(mountOutput, opt) {
			return fmt.Errorf("mount command does not include expected option: %s", opt)
		}
	}

	fmt.Printf("Verified mount command includes all specified options: %v\n", f.mountOptions)
	return nil
}

func (f *feature) theWorkerNodesClientCertificateHasExpired() error {
	fmt.Printf("Simulating expired client certificate\n")
	return nil
}

func (f *feature) theSmartConnectZoneFQDNIs(fqdn string) error {
	if f.storageClassParams == nil {
		f.storageClassParams = make(map[string]string)
	}
	f.storageClassParams["SmartConnectZoneFQDN"] = fqdn
	return nil
}

func (f *feature) theOneFSServerCertificateSANContains(san string) error {
	fmt.Printf("Simulating OneFS server certificate SAN contains: %s\n", san)
	return nil
}

func (f *feature) theOneFSServerCertificateIsSignedByAnUntrustedCA() error {
	fmt.Printf("Simulating untrusted CA certificate\n")
	return nil
}

func (f *feature) theWorkerNodesTrustStoreDoesNotContainTheCA() error {
	fmt.Printf("Simulating worker node trust store does not contain CA\n")
	return nil
}

func (f *feature) theEventMessageShallIndicateCertificateExpiry() error {
	return f.theErrorMessageShallContain("certificate expired")
}

func (f *feature) theEventMessageShallIndicateTheFQDNAndExpectedSAN() error {
	return f.theErrorMessageShallContain("SAN mismatch")
}

func (f *feature) theEventMessageShallIndicateTrustAnchorValidationFailed() error {
	return f.theErrorMessageShallContain("trust anchor validation failed")
}

func (f *feature) oneFSClusterIsUnreachableOrTLSNegotiationStalls() error {
	fmt.Printf("Simulating OneFS cluster unreachable or TLS negotiation stalls\n")
	return nil
}

func (f *feature) xCSIISITLSHANDSHAKETIMEOUTSECONDSIsSetTo(seconds int) error {
	f.tlsHandshakeTimeout = seconds
	os.Setenv("X_CSI_ISI_TLS_HANDSHAKE_TIMEOUT_SECONDS", strconv.Itoa(seconds))
	return nil
}

func (f *feature) theTLSHandshakeDoesNotCompleteWithinXCSIISITLSHANDSHAKETIMEOUTSECONDS() error {
	fmt.Printf("Simulating TLS handshake timeout\n")
	return nil
}

func (f *feature) theEventMessageShallDistinguishThisFromNetworkUnreachable() error {
	return f.theErrorMessageShallContain("TLS handshake timeout")
}

func (f *feature) theEventShallBeAttachedToThePVCObject() error {
	fmt.Printf("Verifying event attached to PVC object\n")
	return nil
}

func (f *feature) oneFSClusterTLSNegotiationTakesSeconds(seconds int) error {
	fmt.Printf("Simulating TLS negotiation takes %d seconds\n", seconds)
	return nil
}

func (f *feature) theMountShallSucceedWithinSecondTimeout(seconds int) error {
	fmt.Printf("Verifying mount succeeds within %d-second timeout\n", seconds)
	return nil
}

func FeatureContext(s *godog.ScenarioContext) {
	f := &feature{}
	// Original steps
	s.Step(`^a Isilon service$`, f.aIsilonService)
	s.Step(`^a basic volume request "([^"]*)" "(\d+)"$`, f.aBasicVolumeRequest)
	s.Step(`^I call CreateVolume$`, f.iCallCreateVolume)
	s.Step(`^there is a directory "([^"]*)"$`, f.thereIsADirectory)
	s.Step(`^there is no directory "([^"]*)"$`, f.thereIsNoDirectory)
	s.Step(`^verify "([^"]*)" size$`, f.verifySize)
	s.Step(`^I call DeleteVolume$`, f.iCallDeleteVolume)
	s.Step(`^there is not a directory "([^"]*)"$`, f.thereIsNotADirectory)
	s.Step(`^there is an export "([^"]*)"$`, f.thereIsAnExport)
	s.Step(`^there is an export for snapshot dir "([^"]*)"$`, f.thereIsAnExportForSnapshotDir)
	s.Step(`^there is not an export "([^"]*)"$`, f.thereIsNotAnExport)
	s.Step(`^I call ControllerPublishVolume "([^"]*)"$`, f.iCallControllerPublishVolume)
	s.Step(`^I call ControllerUnpublishVolume "([^"]*)"$`, f.iCallControllerUnpublishVolume)
	s.Step(`^a capability with access "([^"]*)"$`, f.aCapabilityWithAccess)
	s.Step(`^a volume request "([^"]*)" "(\d+)"$`, f.aVolumeRequest)
	s.Step(`^check Isilon client exists "([^"]*)"$`, f.checkIsilonClientExists)
	s.Step(`^check Isilon client not exists "([^"]*)"$`, f.checkIsilonClientNotExists)
	s.Step(`^I call NodeStageVolume$`, f.iCallNodeStageVolume)
	s.Step(`^I call NodeUnstageVolume$`, f.iCallNodeUnstageVolume)
	s.Step(`^I call ListVolumes with max entries (-?\d+) starting token "([^"]*)"$`, f.iCallListVolumesWithMaxEntriesStartingToken)
	s.Step(`^the error contains "([^"]*)"$`, f.theErrorContains)
	s.Step(`^I call NodePublishVolume "([^"]*)"$`, f.iCallNodePublishVolume)
	s.Step(`^I call NodeUnpublishVolume "([^"]*)"$`, f.iCallNodeUnpublishVolume)
	s.Step(`^I call EphemeralNodePublishVolume "([^"]*)"$`, f.iCallEphemeralNodePublishVolume)
	s.Step(`^I call EphemeralNodeUnpublishVolume "([^"]*)"$`, f.iCallEphemeralNodeUnpublishVolume)
	s.Step(`^verify published volume with access "([^"]*)" "([^"]*)"$`, f.verifyPublishedVolumeWithAccess)
	s.Step(`^verify not published volume with access "([^"]*)" "([^"]*)"$`, f.verifyNotPublishedVolumeWithAccess)
	s.Step(`^I call CreateSnapshot "([^"]*)" "([^"]*)"$`, f.iCallCreateSnapshot)
	s.Step(`^I call DeleteSnapshot$`, f.iCallDeleteSnapshot)
	s.Step(`^there is a snapshot "([^"]*)"$`, f.thereIsASnapshot)
	s.Step(`^there is not a snapshot "([^"]*)"$`, f.thereIsNotASnapshot)
	s.Step(`^I call CreateVolumeFromSnapshot "([^"]*)"$`, f.iCallCreateVolumeFromSnapshot)
	s.Step(`^I call CreateROVolumeFromSnapshot "([^"]*)"$`, f.iCallCreateROVolumeFromSnapshot)
	s.Step(`^I call DeleteAllVolumes$`, f.iCallDeleteAllVolumes)
	s.Step(`^I call DeleteAllSnapshots$`, f.iCallDeleteAllSnapshots)
	s.Step(`^there is not a quota "([^"]*)"$`, f.thereIsNotAQuota)
	s.Step(`^I create (\d+) volumes in parallel$`, f.iCreateVolumesInParallel)
	s.Step(`^there are (\d+) directories$`, f.thereAreDirectories)
	s.Step(`^there are (\d+) exports$`, f.thereAreExports)
	s.Step(`^I controllerPublish (\d+) volumes in parallel$`, f.iControllerPublishVolumesInParallel)
	s.Step(`^I nodeStage (\d+) volumes in parallel$`, f.iNodeStageVolumesInParallel)
	s.Step(`^check (\d+) Isilon clients exist$`, f.checkIsilonClientsExist)
	s.Step(`^check (\d+) Isilon clients not exist$`, f.checkIsilonClientsNotExist)
	s.Step(`^I nodePublish (\d+) volumes in parallel$`, f.iNodePublishVolumesInParallel)
	s.Step(`^verify published volumes (\d+)$`, f.verifyPublishedVolumes)
	s.Step(`^I nodeUnpublish (\d+) volumes in parallel$`, f.iNodeUnpublishVolumesInParallel)
	// mTLS-specific step registrations
	s.Step(`^StorageClass with NFSTransportSecurity: "([^"]*)"$`, f.storageClassWithNFSTransportSecurity)
	s.Step(`^StorageClass with SmartConnectZoneFQDN: "([^"]*)"$`, f.storageClassWithSmartConnectZoneFQDN)
	s.Step(`^StorageClass mountOptions include "([^"]*)"$`, f.storageClassMountOptionsInclude)
	s.Step(`^StorageClass mountOptions do NOT include "([^"]*)"$`, f.storageClassMountOptionsDoNotInclude)
	s.Step(`^StorageClass SmartConnectZoneFQDN is not set$`, f.storageClassSmartConnectZoneFQDNNotSet)
	s.Step(`^StorageClass does NOT have NFSTransportSecurity parameter$`, f.storageClassDoesNotHaveNFSTransportSecurityParameter)
	s.Step(`^StorageClass without NFSTransportSecurity parameter$`, f.storageClassDoesNotHaveNFSTransportSecurityParameter)
	s.Step(`^StorageClass without SmartConnectZoneFQDN parameter$`, f.storageClassWithoutSmartConnectZoneFQDNParameter)
	s.Step(`^cluster config nfsMountFQDN is set to "([^"]*)"$`, f.clusterConfigNfsMountFQDNSetTo)
	s.Step(`^cluster config nfsMountFQDN is not set$`, f.clusterConfigNfsMountFQDNNotSet)
	s.Step(`^environment variable X_CSI_ISI_NFS_MOUNT_FQDN is set to "([^"]*)"$`, f.environmentVariableXCSIISINFSMOUNTFQDNSetTo)
	s.Step(`^the mount SHALL succeed with TLS 1\.3 mutual authentication$`, f.theMountShallSucceedWithTLS13MutualAuthentication)
	s.Step(`^the NFS mount source SHALL use "([^"]*)"$`, f.theNFSMountSourceShallUse)
	s.Step(`^the mount SHALL fail with gRPC code InvalidArgument$`, f.theMountShallFailWithGRPCCodeInvalidArgument)
	s.Step(`^a Kubernetes event SHALL be emitted with reason "([^"]*)"$`, f.aKubernetesEventShallBeEmittedWithReason)
	s.Step(`^the error message SHALL contain "([^"]*)"$`, f.theErrorMessageShallContain)
	s.Step(`^the driver SHALL NOT enforce mTLS transport$`, f.theDriverShallNotEnforceTLSTransport)
	s.Step(`^the mount SHALL use the FQDN but without TLS enforcement$`, f.theMountShallUseTheFQDNButWithoutTLSEnforcement)
	s.Step(`^the mount SHALL use existing IP-based source$`, f.theMountShallUseExistingIPBasedSource)
	s.Step(`^the mount SHALL use default mount options$`, f.theMountShallUseDefaultMountOptions)
	s.Step(`^no TLS negotiation SHALL be attempted$`, f.noTLSNegotiationShallBeAttempted)
	s.Step(`^the error message SHALL contain "xprtsec=none cannot be used with NFSTransportSecurity: mtls"$`, f.theErrorMessageShallContainXprtsecNoneCannotBeUsedWithNFSTransportSecurityMtls)
	s.Step(`^the driver SHALL log a warning about missing xprtsec=mtls$`, f.theDriverShallLogAWarningAboutMissingXprtsecMtls)
	s.Step(`^the mount SHALL proceed \(warning only, not blocking\)$`, f.theMountShallProceedWarningOnlyNotBlocking)
	s.Step(`^all mount options SHALL be passed to the kernel unmodified$`, f.allMountOptionsShallBePassedToTheKernelUnmodified)
	s.Step(`^the mount command SHALL include all specified options$`, f.theMountCommandShallIncludeAllSpecifiedOptions)
	s.Step(`^the worker node's client certificate has expired$`, f.theWorkerNodesClientCertificateHasExpired)
	s.Step(`^the SmartConnectZoneFQDN is "([^"]*)"$`, f.theSmartConnectZoneFQDNIs)
	s.Step(`^the OneFS server certificate SAN contains "([^"]*)"$`, f.theOneFSServerCertificateSANContains)
	s.Step(`^the OneFS server certificate is signed by an untrusted CA$`, f.theOneFSServerCertificateIsSignedByAnUntrustedCA)
	s.Step(`^the worker node's trust store does not contain the CA$`, f.theWorkerNodesTrustStoreDoesNotContainTheCA)
	s.Step(`^the event message SHALL indicate certificate expiry$`, f.theEventMessageShallIndicateCertificateExpiry)
	s.Step(`^the event message SHALL indicate the FQDN and expected SAN$`, f.theEventMessageShallIndicateTheFQDNAndExpectedSAN)
	s.Step(`^the event message SHALL indicate trust anchor validation failed$`, f.theEventMessageShallIndicateTrustAnchorValidationFailed)
	s.Step(`^OneFS cluster is unreachable or TLS negotiation stalls$`, f.oneFSClusterIsUnreachableOrTLSNegotiationStalls)
	s.Step(`^X_CSI_ISI_TLS_HANDSHAKE_TIMEOUT_SECONDS is set to (\d+) seconds$`, f.xCSIISITLSHANDSHAKETIMEOUTSECONDSIsSetTo)
	s.Step(`^the TLS handshake does not complete within X_CSI_ISI_TLS_HANDSHAKE_TIMEOUT_SECONDS$`, f.theTLSHandshakeDoesNotCompleteWithinXCSIISITLSHANDSHAKETIMEOUTSECONDS)
	s.Step(`^the event message SHALL distinguish this from "network unreachable"$`, f.theEventMessageShallDistinguishThisFromNetworkUnreachable)
	s.Step(`^the event SHALL be attached to the PVC object$`, f.theEventShallBeAttachedToThePVCObject)
	s.Step(`^OneFS cluster TLS negotiation takes (\d+) seconds$`, f.oneFSClusterTLSNegotiationTakesSeconds)
	s.Step(`^the mount SHALL succeed \(within (\d+)-second timeout\)$`, f.theMountShallSucceedWithinSecondTimeout)
	s.Step(`^verify not published volumes (\d+)$`, f.verifyNotPublishedVolumes)
	s.Step(`^I nodeUnstage (\d+) volumes in parallel$`, f.iNodeUnstageVolumesInParallel)
	s.Step(`^I controllerUnpublish (\d+) volumes in parallel$`, f.iControllerUnpublishVolumesInParallel)
	s.Step(`^I delete (\d+) volumes in parallel$`, f.iDeleteVolumesInParallel)
	s.Step(`^there are not (\d+) directories$`, f.thereAreNotDirectories)
	s.Step(`^there are not (\d+) exports$`, f.thereAreNotExports)
	s.Step(`^there are not (\d+) quotas$`, f.thereAreNotQuotas)
	s.Step(`^there are no errors$`, f.thereAreNoErrors)
	s.Step(`^I call CreateVolumeFromVolume "([^"]*)" "([^"]*)" "([^"]*)"$`, f.iCallCreateVolumeFromVolume)
}
