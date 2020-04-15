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
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/csi-isilon/common/constants"
	"github.com/dell/csi-isilon/common/utils"
	isi "github.com/dell/goisilon"
	isiApi "github.com/dell/goisilon/api"
	log "github.com/sirupsen/logrus"
)

// constants
const (
	errUnknownAccessType          = "unknown access type is not Mount"
	errUnknownAccessMode          = "unknown or unsupported access mode"
	errNoSingleNodeReader         = "Single node only reader access mode is not supported"
	errNoMultiNodeSingleWriter    = "Multi node single writer access mode is not supported"
	MaxRetries                    = 10
	RetrySleepTime                = 1000 * time.Millisecond
	AccessZoneParam               = "AccessZone"
	ExportPathParam               = "Path"
	IsiPathParam                  = "IsiPath"
	AzServiceIPParam              = "AzServiceIP"
	RootClientEnabledParam        = "RootClientEnabled"
	RootClientEnabledParamDefault = "false"
)

// validateVolSize uses the CapacityRange range params to determine what size
// volume to create. Returned size is in bytes
func validateVolSize(cr *csi.CapacityRange) (int64, error) {

	minSize := cr.GetRequiredBytes()

	if minSize < 0 {
		return 0, status.Errorf(
			codes.OutOfRange,
			"bad capacity: volume size bytes '%d' must not be negative", minSize)
	}

	if minSize == 0 {
		minSize = constants.DefaultVolumeSizeInBytes
	}

	return minSize, nil
}

func (s *service) CreateVolume(
	ctx context.Context,
	req *csi.CreateVolumeRequest) (
	*csi.CreateVolumeResponse, error) {
	var (
		accessZone        string
		isiPath           string
		path              string
		azServiceIP       string
		rootClientEnabled string
		quotaID           string
		exportID          int
		foundVol          bool
		export            isi.Export
	)
	// auto probe
	if err := s.autoProbe(ctx); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	// validate request
	sizeInBytes, err := s.ValidateCreateVolumeRequest(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	params := req.GetParameters()
	if _, ok := params[AccessZoneParam]; ok {
		if params[AccessZoneParam] == "" {
			accessZone = s.opts.AccessZone
		} else {
			accessZone = params[AccessZoneParam]
		}
	} else {
		// use the default access zone if not set in the storage class
		accessZone = s.opts.AccessZone
	}
	if _, ok := params[IsiPathParam]; ok {
		if params[IsiPathParam] == "" {
			isiPath = s.opts.Path
		} else {
			isiPath = params[IsiPathParam]
		}
	} else {
		// use the default isiPath if not setu in the storage class
		isiPath = s.opts.Path
	}
	if _, ok := params[AzServiceIPParam]; ok {
		azServiceIP = params[AzServiceIPParam]
		if azServiceIP == "" {
			// use the endpoint if empty in the storage class
			azServiceIP = s.opts.Endpoint
		}
	} else {
		// use the endpoint if not set in the storage class
		azServiceIP = s.opts.Endpoint
	}

	if val, ok := params[RootClientEnabledParam]; ok {
		_, err := strconv.ParseBool(val)
		// use the default if the boolean literal from the storage class is malformed
		if err != nil {
			log.WithField(RootClientEnabledParam, val).Debugf(
				"invalid boolean value for '%s', defaulting to 'false'", RootClientEnabledParam)

			rootClientEnabled = RootClientEnabledParamDefault
		}
		rootClientEnabled = val
	} else {
		// use the default if not set in the storage class
		rootClientEnabled = RootClientEnabledParamDefault
	}

	foundVol = false
	path = utils.GetPathForVolume(isiPath, req.GetName())
	// to ensure idempotency, check if the volume still exists.
	// k8s might have made the same CreateVolume call in quick succession and the volume was already created in the first run
	if s.isiSvc.IsVolumeExistent(isiPath, "", req.GetName()) {
		log.Debugf("the path '%s' has already existed", path)
		foundVol = true
	}

	if export, err = s.isiSvc.GetExportWithPathAndZone(path, accessZone); err != nil || export == nil {
		var errMsg string
		if err == nil {
			if foundVol {
				return nil, status.Error(codes.Internal, "the export may not be ready yet and the path is '"+path+"'")
			}
		} else {
			// internal error
			return nil, err
		}
		log.Errorf("error retrieving export ID for '%s', set it to 0. error : '%s'.\n", req.GetName(), errMsg)
		log.Errorf("request parameters: the path is '%s', and the access zone is '%s'.", path, accessZone)
		exportID = 0
	} else {
		exportID = export.ID
		log.Debugf("id of the corresonding nfs export of existing volume '%s' has been resolved to '%d'", req.GetName(), exportID)
		if exportID != 0 {
			if foundVol {
				return s.getCreateVolumeResponse(exportID, req.GetName(), path, export.Zone, sizeInBytes, azServiceIP, rootClientEnabled), nil
			}
			// in case the export exists but no related volume (directory)
			if err = s.isiSvc.UnexportByIDWithZone(exportID, accessZone); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			exportID = 0
		}
	}
	// create volume (directory) with ACL 0777
	if err = s.isiSvc.CreateVolume(isiPath, req.GetName()); err != nil {
		return nil, err
	}
	// check volume content source in the request
	// if not null, copy content from the datasource
	if contentSource := req.GetVolumeContentSource(); contentSource != nil {
		err = s.createVolumeFromSource(isiPath, contentSource, req, sizeInBytes)
		if err != nil {
			return nil, err
		}
	}

	if !foundVol {
		// create quota
		if quotaID, err = s.isiSvc.CreateQuota(path, req.GetName(), sizeInBytes, s.opts.QuotaEnabled); err != nil {
			log.Errorf("error creating quota ('%s', '%d' bytes), abort, also roll back by deleting the newly created volume: '%v'", req.GetName(), sizeInBytes, err)
			//roll back, delete the newly created volume
			if err = s.isiSvc.DeleteVolume(isiPath, req.GetName()); err != nil {
				return nil, fmt.Errorf("rollback (deleting volume '%s') failed with error : '%v'", req.GetName(), err)
			}
			return nil, fmt.Errorf("error creating quota ('%s', '%d' bytes), abort, also succesfully rolled back by deleting the newly created volume", req.GetName(), sizeInBytes)
		}
	}

	// export volume in the given access zone, also add normalized quota id to the description field, in DeleteVolume,
	// the quota ID will be used for the quota to be directly deleted by ID
	if exportID, err = s.isiSvc.ExportVolumeWithZone(isiPath, req.GetName(), accessZone, utils.GetQuotaIDWithCSITag(quotaID)); err == nil && exportID != 0 {
		// get the export and retry if not found to ensure the export has been created
		for i := 0; i < MaxRetries; i++ {
			if export, _ := s.isiSvc.GetExportByIDWithZone(exportID, accessZone); export != nil {
				// return the response
				return s.getCreateVolumeResponse(exportID, req.GetName(), path, accessZone, sizeInBytes, azServiceIP, rootClientEnabled), nil
			}
			time.Sleep(RetrySleepTime)
			log.Printf("Begin to retry '%d' time(s), for export id '%d' and path '%s'\n", i+1, exportID, path)
		}
	} else {
		// clear quota and delete volume since the export cannot be created
		if error := s.isiSvc.ClearQuotaByID(quotaID); error != nil {
			log.Infof("Clear Quota returned error '%s'", error)
		}
		if error := s.isiSvc.DeleteVolume(isiPath, req.GetName()); error != nil {
			log.Infof("Delete volume in CreateVolume returned error '%s'", error)
		}
		return nil, err
	}
	return nil, status.Error(codes.Internal, "the export id '"+strconv.Itoa(exportID)+"' and path '"+path+"' may not be ready yet after retrying")
}

func (s *service) createVolumeFromSnapshot(isiPath, srcSnapshotID, dstVolumeName string, sizeInBytes int64) error {
	var snapshotSrc isi.Snapshot
	var err error
	if snapshotSrc, err = s.isiSvc.GetSnapshot(srcSnapshotID); err != nil {
		return fmt.Errorf("failed to get snapshot id '%s', error '%v'", srcSnapshotID, err)
	}

	// check source snapshot size
	size := s.isiSvc.GetSnapshotSize(isiPath, snapshotSrc.Name)
	if size > sizeInBytes {
		return fmt.Errorf("Specified size '%d' is smaller than source snapshot size '%d'", sizeInBytes, size)
	}

	if _, err = s.isiSvc.CopySnapshot(isiPath, snapshotSrc.Id, dstVolumeName); err != nil {
		return fmt.Errorf("failed to copy snapshot id '%s', error '%s'", srcSnapshotID, err.Error())
	}

	return nil
}

func (s *service) createVolumeFromVolume(isiPath, srcVolumeName, dstVolumeName string, sizeInBytes int64) error {
	var err error
	if s.isiSvc.IsVolumeExistent(isiPath, "", srcVolumeName) {
		// check source volume size
		size := s.isiSvc.GetVolumeSize(isiPath, srcVolumeName)
		if size > sizeInBytes {
			return fmt.Errorf("Specified size '%d' is smaller than source volume size '%d'", sizeInBytes, size)
		}

		if _, err = s.isiSvc.CopyVolume(isiPath, srcVolumeName, dstVolumeName); err != nil {
			return fmt.Errorf("failed to copy volume name '%s', error '%v'", srcVolumeName, err)
		}
	} else {
		return fmt.Errorf("failed to get volume name '%s', error '%v'", srcVolumeName, err)
	}

	return nil
}

func (s *service) createVolumeFromSource(
	isiPath string,
	contentSource *csi.VolumeContentSource,
	req *csi.CreateVolumeRequest,
	sizeInBytes int64) error {
	if contentSnapshot := contentSource.GetSnapshot(); contentSnapshot != nil {
		// create volume from source snapshot
		if err := s.createVolumeFromSnapshot(isiPath, contentSnapshot.GetSnapshotId(), req.GetName(), sizeInBytes); err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	}

	if contentVolume := contentSource.GetVolume(); contentVolume != nil {
		// create volume from source volume
		srcVolumeName, _, _, err := utils.ParseNormalizedVolumeID(contentVolume.GetVolumeId())
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		if err := s.createVolumeFromVolume(isiPath, srcVolumeName, req.GetName(), sizeInBytes); err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	}
	return nil
}

func (s *service) getCreateVolumeResponse(exportID int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled string) *csi.CreateVolumeResponse {
	return &csi.CreateVolumeResponse{
		Volume: s.getCSIVolume(exportID, volName, path, accessZone, sizeInBytes, azServiceIP, rootClientEnabled),
	}
}

func (s *service) getCSIVolume(exportID int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled string) *csi.Volume {
	// Make the additional volume attributes
	attributes := map[string]string{
		"ID":                strconv.Itoa(exportID),
		"Name":              volName,
		"Path":              path,
		"AccessZone":        accessZone,
		"AzServiceIP":       azServiceIP,
		"RootClientEnabled": rootClientEnabled,
	}
	log.Debugf("Attributes '%v'", attributes)
	vi := &csi.Volume{
		VolumeId:      utils.GetNormalizedVolumeID(volName, exportID, accessZone),
		CapacityBytes: sizeInBytes,
		VolumeContext: attributes,
	}
	return vi
}

func (s *service) DeleteVolume(
	ctx context.Context,
	req *csi.DeleteVolumeRequest) (
	*csi.DeleteVolumeResponse, error) {

	// TODO more checks need to be done, e.g. if access mode is VolumeCapability_AccessMode_MULTI_NODE_XXX, then other nodes might still be using this volume, thus the delete should be skipped

	// probe
	if err := s.autoProbe(ctx); err != nil {
		return nil, err
	}
	s.logStatistics()
	// validate request
	if err := s.ValidateDeleteVolumeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	volName, exportID, accessZone, _ := utils.ParseNormalizedVolumeID(req.GetVolumeId())
	quotaEnabled := s.opts.QuotaEnabled

	export, err := s.isiSvc.GetExportByIDWithZone(exportID, accessZone)
	if err != nil {
		if jsonError, ok := err.(*isiApi.JSONError); ok {
			if jsonError.StatusCode == 404 {
				// export not found means the volume doesn't exist
				return &csi.DeleteVolumeResponse{}, nil
			}
			return nil, err
		}
		return nil, err
	} else if export == nil {
		// in case it occurs the case that export is nil and error is also nil
		return &csi.DeleteVolumeResponse{}, nil
	}

	exportPath := (*export.Paths)[0]
	isiPath := utils.GetIsiPathFromExportPath(exportPath)
	// to ensure idempotency, check if the volume and export still exists.
	// k8s might have made the same DeleteVolume call in quick succession and the volume was already deleted in the first run
	log.Debugf("controller begins to delete volume, name '%s', quotaEnabled '%t'", volName, quotaEnabled)
	if err := s.isiSvc.DeleteQuotaByExportIDWithZone(volName, exportID, accessZone); err != nil {
		jsonError, ok := err.(*isiApi.JSONError)
		if ok {
			if jsonError.StatusCode != 404 {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	if !s.isiSvc.IsVolumeExistent(isiPath, "", volName) {
		log.Debugf("volume '%s' not found, skip calling delete directory.", volName)
	} else {
		if err := s.isiSvc.DeleteVolume(isiPath, volName); err != nil {
			return nil, err
		}
	}

	log.Infof("controller begins to unexport id '%d', target path '%s', access zone '%s'", exportID, volName, accessZone)
	if err := s.isiSvc.UnexportByIDWithZone(exportID, accessZone); err != nil {
		return nil, err
	}
	return &csi.DeleteVolumeResponse{}, nil
}

func (s *service) ControllerExpandVolume(
	context.Context,
	*csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {

	return nil, status.Error(codes.Unimplemented, "")
}

/*
 * ControllerPublishVolume : Checks all params and validity
 */
func (s *service) ControllerPublishVolume(
	ctx context.Context,
	req *csi.ControllerPublishVolumeRequest) (
	*csi.ControllerPublishVolumeResponse, error) {
	var (
		accessZone string
		exportPath string
		isiPath    string
	)

	volumeContext := req.GetVolumeContext()
	if volumeContext != nil {
		log.Printf("VolumeContext:")
		for key, value := range volumeContext {
			log.Printf("    [%s]=%s", key, value)
		}
	}

	if err := s.autoProbe(ctx); err != nil {
		return nil, err
	}

	volID := req.GetVolumeId()
	if volID == "" {
		return nil, status.Error(codes.InvalidArgument,
			"volume ID is required")
	}

	volName, _, _, err := utils.ParseNormalizedVolumeID(volID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to parse volume ID '%s', error : '%v'", volID, err))
	}
	if accessZone = volumeContext[AccessZoneParam]; accessZone == "" {
		accessZone = s.opts.AccessZone
	}
	if exportPath = volumeContext[ExportPathParam]; exportPath == "" {
		exportPath = utils.GetPathForVolume(s.opts.Path, volName)
	}
	isiPath = utils.GetIsiPathFromExportPath(exportPath)
	vol, err := s.isiSvc.GetVolume(isiPath, "", volName)
	if err != nil || vol.Name == "" {
		return nil, status.Errorf(codes.Internal,
			"failure checking volume status before controller publish: '%s'",
			err.Error())
	}

	nodeID := req.GetNodeId()
	if nodeID == "" {
		return nil, status.Error(codes.InvalidArgument,
			"node ID is required")
	}

	vc := req.GetVolumeCapability()
	if vc == nil {
		return nil, status.Error(codes.InvalidArgument,
			"volume capability is required")
	}

	am := vc.GetAccessMode()
	if am == nil {
		return nil, status.Error(codes.InvalidArgument,
			"access mode is required")
	}

	if am.Mode == csi.VolumeCapability_AccessMode_UNKNOWN {
		return nil, status.Error(codes.InvalidArgument,
			errUnknownAccessMode)
	}

	vcs := []*csi.VolumeCapability{req.GetVolumeCapability()}
	if !checkValidAccessTypes(vcs) {
		return nil, status.Error(codes.InvalidArgument,
			errUnknownAccessType)
	}
	return &csi.ControllerPublishVolumeResponse{}, nil
}

func (s *service) ValidateVolumeCapabilities(
	ctx context.Context,
	req *csi.ValidateVolumeCapabilitiesRequest) (
	*csi.ValidateVolumeCapabilitiesResponse, error) {
	var (
		exportPath string
		isiPath    string
	)
	if err := s.autoProbe(ctx); err != nil {
		return nil, err
	}

	volID := req.GetVolumeId()
	volName, _, _, err := utils.ParseNormalizedVolumeID(volID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to parse volume ID '%s', error : '%v'", volID, err))
	}

	volumeContext := req.GetVolumeContext()
	if exportPath = volumeContext[ExportPathParam]; exportPath == "" {
		exportPath = utils.GetPathForVolume(s.opts.Path, volName)
	}
	isiPath = utils.GetIsiPathFromExportPath(exportPath)

	vol, err := s.getVolByName(isiPath, volName)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"failure checking volume status for capabilities: '%s'",
			err.Error())
	}

	vcs := req.GetVolumeCapabilities()
	supported, reason := validateVolumeCaps(vcs, vol)

	resp := &csi.ValidateVolumeCapabilitiesResponse{}
	if supported {
		// The optional fields volume_context and parameters are not passed.
		confirmed := &csi.ValidateVolumeCapabilitiesResponse_Confirmed{}
		confirmed.VolumeCapabilities = vcs
		resp.Confirmed = confirmed
	} else {
		resp.Message = reason
	}

	return resp, nil
}

func (s *service) ListVolumes(ctx context.Context,
	req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	var (
		exports isi.ExportList
		resume  string
		err     error
	)
	resp := new(csi.ListVolumesResponse)
	if req.MaxEntries == 0 && req.StartingToken == "" {
		// The value of max_entries is zero and no starting token in the request means no restriction
		exports, err = s.isiSvc.GetExports()
	} else {
		maxEntries := strconv.Itoa(int(req.MaxEntries))
		if req.StartingToken == "" {
			// Get the first page if there's no starting token
			if req.MaxEntries < 0 {
				return nil, status.Error(codes.InvalidArgument, "Invalid max entries")
			}
			exports, resume, err = s.isiSvc.GetExportsWithLimit(maxEntries)
			if err != nil {
				return nil, status.Error(codes.Internal, "Cannot get exports with limit")
			}
		} else {
			// Continue to get exports based on the previous call
			exports, resume, err = s.isiSvc.GetExportsWithResume(req.StartingToken)
			if err != nil {
				// The starting token is not valid, return the gRPC aborted code to indicate
				return nil, status.Error(codes.Aborted, "The starting token is not valid")
			}
		}
		resp.NextToken = resume
	}

	// Count the number of entries
	num := 0
	for _, export := range exports {
		paths := export.Paths
		for range *paths {
			num++
		}
	}
	// Convert exports to entries
	entries := make([]*csi.ListVolumesResponse_Entry, num)
	i := 0
	for _, export := range exports {
		paths := export.Paths
		for _, path := range *paths {
			// TODO get the capacity range, not able to get now
			volName := utils.GetVolumeNameFromExportPath(path)
			// Not able to get "rootClientEnabled", it's read from the volume's storage class
			// and added to "volumeContext" in CreateVolume, and read in NodeStageVolume.
			// The value is not relevant here so just pass default value "false" here.
			volume := s.getCSIVolume(export.ID, volName, path, export.Zone, 0, s.opts.Endpoint, "false")
			entries[i] = &csi.ListVolumesResponse_Entry{
				Volume: volume,
			}
			i++
		}
	}
	resp.Entries = entries
	return resp, nil
}

func (s *service) ListSnapshots(context.Context,
	*csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (s *service) ControllerUnpublishVolume(
	ctx context.Context,
	req *csi.ControllerUnpublishVolumeRequest) (
	*csi.ControllerUnpublishVolumeResponse, error) {

	return nil, status.Error(codes.Unimplemented, "")
}

func (s *service) GetCapacity(
	ctx context.Context,
	req *csi.GetCapacityRequest) (
	*csi.GetCapacityResponse, error) {

	if err := s.autoProbe(ctx); err != nil {
		log.Error("Failed to probe with error: " + err.Error())
		return nil, err
	}

	// Optionally validate the volume capability
	vcs := req.GetVolumeCapabilities()
	if vcs != nil {
		supported, reason := validateVolumeCaps(vcs, nil)
		if !supported {
			log.Errorf("GetVolumeCapabilities failed with error: '%s'", reason)
			return nil, status.Errorf(codes.InvalidArgument, reason)
		}
	}

	//pass the key(s) to rest api
	keyArray := []string{"ifs.bytes.avail"}

	stat, err := s.isiSvc.GetStatistics(keyArray)
	if err != nil || len(stat.StatsList) < 1 {
		return nil, status.Errorf(codes.Internal, "Could not retrieve capacity. Error '%s'", err.Error())
	}
	if stat.StatsList[0].Error != "" {
		return nil, status.Errorf(codes.Internal, "Could not retrieve capacity. Data returned error '%s'", stat.StatsList[0].Error)
	}
	remainingCapInBytes := stat.StatsList[0].Value

	return &csi.GetCapacityResponse{
		AvailableCapacity: int64(remainingCapInBytes),
	}, nil
}

func (s *service) ControllerGetCapabilities(
	ctx context.Context,
	req *csi.ControllerGetCapabilitiesRequest) (
	*csi.ControllerGetCapabilitiesResponse, error) {

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
					},
				},
			},
			/*{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
					},
				},
			},*/
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_GET_CAPACITY,
					},
				},
			},
			/*{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
					},
				},
			},*/
		},
	}, nil
}

func (s *service) controllerProbe(ctx context.Context) error {

	if err := s.validateOptsParameters(); err != nil {
		return fmt.Errorf("controller probe failed : '%v'", err)
	}

	if s.isiSvc == nil {

		return errors.New("s.isiSvc (type isiService) is nil, probe failed")

	}

	if err := s.isiSvc.TestConnection(); err != nil {
		return fmt.Errorf("controller probe failed : '%v'", err)
	}

	log.Debug("controller probe succeeded")

	return nil
}

// CreateSnapshot creates a snapshot.
// If Parameters["VolumeIDList"] has a comma separated list of additional volumes, they will be
// snapshotted in a consistency group with the primary volume in CreateSnapshotRequest.SourceVolumeId.
func (s *service) CreateSnapshot(
	ctx context.Context,
	req *csi.CreateSnapshotRequest) (
	*csi.CreateSnapshotResponse, error) {

	// auto probe
	if err := s.autoProbe(ctx); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	// validate request and get details of the request
	// srcVolumeID: source volume name
	// snapshotName: name of the snapshot that need to be created
	var (
		snapshotNew isi.Snapshot
		params      map[string]string
		isiPath     string
	)
	params = req.GetParameters()
	if _, ok := params[IsiPathParam]; ok {
		if params[IsiPathParam] == "" {
			isiPath = s.opts.Path
		} else {
			isiPath = params[IsiPathParam]
		}
	} else {
		// use the default isiPath if not set in the storage class
		isiPath = s.opts.Path
	}
	srcVolumeID, snapshotName, err := s.validateCreateSnapshotRequest(req, isiPath)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// check if snapshot already exists
	var snapshotByName isi.Snapshot
	if snapshotByName, err = s.isiSvc.GetSnapshot(snapshotName); snapshotByName != nil {
		if path.Base(snapshotByName.Path) == srcVolumeID {
			// return the existent snapshot
			return s.getCreateSnapshotResponse(strconv.FormatInt(snapshotByName.Id, 10), req.GetSourceVolumeId(), snapshotByName.Created, s.isiSvc.GetSnapshotSize(isiPath, snapshotName)), nil
		}
		// return already exists error
		return nil, status.Error(codes.AlreadyExists,
			fmt.Sprintf("a snapshot with name '%s' already exists but is incompatible with the specified source volume id '%s'", snapshotName, req.GetSourceVolumeId()))
	}

	// create new snapshot for source direcory
	path := utils.GetPathForVolume(isiPath, srcVolumeID)
	if snapshotNew, err = s.isiSvc.CreateSnapshot(path, snapshotName); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// return the response
	return s.getCreateSnapshotResponse(strconv.FormatInt(snapshotNew.Id, 10), req.GetSourceVolumeId(), snapshotNew.Created, s.isiSvc.GetSnapshotSize(isiPath, snapshotName)), nil
}

// validateCreateSnapshotRequest validate the input params in CreateSnapshotRequest
func (s *service) validateCreateSnapshotRequest(
	req *csi.CreateSnapshotRequest, isiPath string) (string, string, error) {
	srcVolumeID, _, _, err := utils.ParseNormalizedVolumeID(req.GetSourceVolumeId())
	if err != nil {
		return "", "", status.Error(codes.InvalidArgument, err.Error())
	}

	if !s.isiSvc.IsVolumeExistent(isiPath, "", srcVolumeID) {
		return "", "", status.Error(codes.InvalidArgument,
			"source volume id is invalid")
	}

	snapshotName := req.GetName()
	if snapshotName == "" {
		return "", "", status.Error(codes.InvalidArgument,
			"name cannot be empty")
	}

	// Validate requested name is not too long, if supplied. If so, truncate to 31 characters.
	if len(snapshotName) > 31 {
		snapshotName = strings.Replace(snapshotName, "snapshot-", "sn-", 1)
		snapshotName = snapshotName[0:31]
		log.Debugf("Requested name '%s' longer than 31 character max, truncated to '%s'\n", req.Name, snapshotName)
		req.Name = snapshotName
	}

	return srcVolumeID, snapshotName, nil
}

func (s *service) getCreateSnapshotResponse(snapshotID string, sourceVolumeID string, creationTime, sizeInBytes int64) *csi.CreateSnapshotResponse {
	return &csi.CreateSnapshotResponse{
		Snapshot: s.getCSISnapshot(snapshotID, sourceVolumeID, creationTime, sizeInBytes),
	}
}

func (s *service) getCSISnapshot(snapshotID string, sourceVolumeID string, creationTime, sizeInBytes int64) *csi.Snapshot {
	ts := &timestamp.Timestamp{
		Seconds: creationTime,
	}

	vi := &csi.Snapshot{
		SizeBytes:      sizeInBytes,
		SnapshotId:     snapshotID,
		SourceVolumeId: sourceVolumeID,
		CreationTime:   ts,
		ReadyToUse:     true,
	}

	return vi
}

func (s *service) DeleteSnapshot(
	ctx context.Context,
	req *csi.DeleteSnapshotRequest) (
	*csi.DeleteSnapshotResponse, error) {
	if err := s.autoProbe(ctx); err != nil {
		return nil, err
	}

	if req.GetSnapshotId() == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "snapshot id to be deleted is required")
	}
	id, err := strconv.ParseInt(req.GetSnapshotId(), 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot convert snapshot to integer: '%s'", err.Error())
	}
	snapshot, err := s.isiSvc.GetSnapshot(req.SnapshotId)
	// Idempotency check
	if err != nil {
		jsonError, ok := err.(*isiApi.JSONError)
		if !ok {
			log.Error("type casting from error to JSONError failed, attempting to determine the error by parsing the error msg instead of the status code")
			// Check the error message if failed to convert the error to JSONError
			if snapshot == nil && strings.Contains(err.Error(), "not found") {
				return &csi.DeleteSnapshotResponse{}, nil
			}
			// Internal server error if the error is not about "not found"
			if err != nil {
				return nil, status.Errorf(codes.Internal, "cannot check the existence of the snapshot: '%s'", err.Error())
			}
		} else {
			if jsonError.StatusCode == 404 {
				return &csi.DeleteSnapshotResponse{}, nil
			}
			return nil, status.Errorf(codes.Internal, "cannot check the existence of the snapshot: '%s'", err.Error())
		}
	}

	err = s.isiSvc.DeleteSnapshot(id, "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error deleteing snapshot: '%s'", err.Error())
	}
	return &csi.DeleteSnapshotResponse{}, nil
}

//Validate volume capabilities
func validateVolumeCaps(
	vcs []*csi.VolumeCapability,
	vol isi.Volume) (bool, string) {

	var (
		supported = true
		reason    string
	)
	// Check that all access types are valid
	if !checkValidAccessTypes(vcs) {
		return false, errUnknownAccessType
	}

	for _, vc := range vcs {
		am := vc.GetAccessMode()
		if am == nil {
			continue
		}
		switch am.Mode {
		case csi.VolumeCapability_AccessMode_UNKNOWN:
			supported = false
			reason = errUnknownAccessMode
		case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:
			break
		case csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY:
			supported = false
			reason = errNoSingleNodeReader
		case csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY:
			break
		case csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER:
			supported = false
			reason = errNoMultiNodeSingleWriter
		case csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER:
			break
		default:
			// This is to guard against new access modes not understood
			supported = false
			reason = errUnknownAccessMode
		}
	}

	return supported, reason
}

func checkValidAccessTypes(vcs []*csi.VolumeCapability) bool {
	for _, vc := range vcs {
		if vc == nil {
			continue
		}
		atmount := vc.GetMount()
		if atmount != nil {
			continue
		}
		// Unknown access type, we should reject it.
		return false
	}
	return true
}
