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
	"sync"
	"time"

	fPath "path"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/csi-isilon/common/constants"
	"github.com/dell/csi-isilon/common/utils"
	isi "github.com/dell/goisilon"
	isiApi "github.com/dell/goisilon/api"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RPOEnum represents valid rpo values
type RPOEnum string

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
	IsiVolumePathPermissionsParam = "IsiVolumePathPermissions"
	AzServiceIPParam              = "AzServiceIP"
	RootClientEnabledParam        = "RootClientEnabled"
	RootClientEnabledParamDefault = "false"
	DeleteSnapshotMarker          = "DELETE_SNAPSHOT"
	IgnoreDotAndDotDotSubDirs     = 2
	ClusterNameParam              = "ClusterName"
	// KeyReplicationEnabled represents key for replication enabled
	KeyReplicationEnabled = "isReplicationEnabled"

	// These are available when enabling --extra-create-metadata for the external-provisioner.
	csiPersistentVolumeName           = "csi.storage.k8s.io/pv/name"
	csiPersistentVolumeClaimName      = "csi.storage.k8s.io/pvc/name"
	csiPersistentVolumeClaimNamespace = "csi.storage.k8s.io/pvc/namespace"
	// These map to the above fields in the form of HTTP header names.
	headerPersistentVolumeName           = "x-csi-pv-name"
	headerPersistentVolumeClaimName      = "x-csi-pv-claimname"
	headerPersistentVolumeClaimNamespace = "x-csi-pv-namespace"
	// KeyReplicationVGPrefix represents key for replication vg prefix
	KeyReplicationVGPrefix = "volumeGroupPrefix"
	// KeyReplicationRemoteSystem represents key for replication remote system
	KeyReplicationRemoteSystem = "remoteSystem"
	// KeyReplicationIgnoreNamespaces represents key for replication ignore namespaces
	KeyReplicationIgnoreNamespaces = "ignoreNamespaces"
	// KeyCSIPVCNamespace represents key for csi pvc namespace
	KeyCSIPVCNamespace = "csi.storage.k8s.io/pvc/namespace"
	// KeyReplicationRPO represents key for replication RPO
	KeyReplicationRPO         = "rpo"
	RpoFiveMinutes    RPOEnum = "Five_Minutes"
	RpoFifteenMinutes RPOEnum = "Fifteen_Minutes"
	RpoThirtyMinutes  RPOEnum = "Thirty_Minutes"
	RpoOneHour        RPOEnum = "One_Hour"
	RpoSixHours       RPOEnum = "Six_Hours"
	RpoTwelveHours    RPOEnum = "Twelve_Hours"
	RpoOneDay         RPOEnum = "One_Day"
)

// clusterToNodeIDMap is a map[clusterName][]*nodeIDToClientMap
var clusterToNodeIDMap = new(sync.Map)

//type nodeIDElementsMap map[string]string
type nodeIDToClientMap map[string]string

// IsValid - checks valid RPO
func (rpo RPOEnum) IsValid() error {
	switch rpo {
	case RpoFiveMinutes, RpoFifteenMinutes, RpoThirtyMinutes, RpoOneHour, RpoSixHours, RpoTwelveHours, RpoOneDay:
		return nil
	}
	return errors.New("invalid rpo type")
}

// ToInt - converts to seconds
func (rpo RPOEnum) ToInt() (int, error) {
	switch rpo {
	case RpoFiveMinutes:
		return 300, nil
	case RpoFifteenMinutes:
		return 900, nil
	case RpoThirtyMinutes:
		return 1800, nil
	case RpoOneHour:
		return 3600, nil
	case RpoSixHours:
		return 21600, nil
	case RpoTwelveHours:
		return 43200, nil
	case RpoOneDay:
		return 86400, nil
	default:
		return -1, errors.New("invalid rpo type")
	}
}

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
		accessZone                        string
		isiPath                           string
		volumePathPermissions             string
		path                              string
		azServiceIP                       string
		rootClientEnabled                 string
		quotaID                           string
		exportID                          int
		foundVol                          bool
		export                            isi.Export
		contentSource                     *csi.VolumeContentSource
		sourceSnapshotID                  string
		sourceVolumeID                    string
		snapshotIsiPath                   string
		isROVolumeFromSnapshot            bool
		snapshotTrackingDir               string
		snapshotTrackingDirEntryForVolume string
		clusterName                       string
		isReplication                     bool
		VolumeGroupDir                    string
		snapshotSourceVolumeIsiPath       string
	)

	params := req.GetParameters()

	if _, ok := params[ClusterNameParam]; ok {
		if params[ClusterNameParam] == "" {
			clusterName = s.defaultIsiClusterName
		} else {
			clusterName = params[ClusterNameParam]
		}
	}

	// Fetch log handler
	ctx, log, runID := GetRunIDLog(ctx)

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	// auto probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	// validate request
	sizeInBytes, err := s.ValidateCreateVolumeRequest(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

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
			isiPath = isiConfig.IsiPath
		} else {
			isiPath = params[IsiPathParam]
		}
	} else {
		// use the default isiPath if not set in the storage class
		isiPath = isiConfig.IsiPath
	}

	if _, ok := params[IsiVolumePathPermissionsParam]; ok {
		if params[IsiVolumePathPermissionsParam] == "" {
			volumePathPermissions = isiConfig.IsiVolumePathPermissions
		} else {
			volumePathPermissions = params[IsiVolumePathPermissionsParam]
		}
	} else {
		// use the default volumePathPermissions if not set in the storage class
		volumePathPermissions = isiConfig.IsiVolumePathPermissions
	}

	if repl, ok := params[s.WithRP(KeyReplicationEnabled)]; ok {
		if boolRepl, err := strconv.ParseBool(repl); err == nil {
			isReplication = boolRepl
		} else {
			log.Info("Unable to parse replication flag from SC")
		}
	} else {
		log.Debug("Replication flag unset")
	}

	// When custom topology is enabled it takes precedence over the current default behavior
	// Set azServiceIP to updated endpoint when custom topology is enabled
	if s.opts.CustomTopologyEnabled {
		azServiceIP = isiConfig.Endpoint
	} else if _, ok := params[AzServiceIPParam]; ok {
		azServiceIP = params[AzServiceIPParam]
		if azServiceIP == "" {
			// use the endpoint if empty in the storage class
			azServiceIP = isiConfig.Endpoint
		}
	} else {
		// use the endpoint if not set in the storage class
		azServiceIP = isiConfig.Endpoint
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

	//CSI specific metada for authorization
	var headerMetadata = addMetaData(params)

	// check volume content source in the request
	isROVolumeFromSnapshot = false
	if contentSource = req.GetVolumeContentSource(); contentSource != nil {

		// Fetch source snapshot ID  or volume ID from content source
		if snapshot := contentSource.GetSnapshot(); snapshot != nil {
			normalizedSnapshotID := snapshot.GetSnapshotId()

			// parse the input snapshot id and fetch it's components
			var snapshotSrcClusterName string
			sourceSnapshotID, snapshotSrcClusterName, err = utils.ParseNormalizedSnapshotID(ctx, normalizedSnapshotID)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, "failed to parse snapshot ID '%s', error : '%v'", normalizedSnapshotID, err))
			}

			if snapshotSrcClusterName != "" && snapshotSrcClusterName != clusterName {
				return nil, status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, "source snapshot's cluster name '%s' and new volume's cluster name '%s' doesn't match", snapshotSrcClusterName, clusterName))
			}

			log.Infof("Creating volume from snapshot ID: '%s'", sourceSnapshotID)

			// Get snapshot path
			if snapshotSourceVolumeIsiPath, err = isiConfig.isiSvc.GetSnapshotSourceVolumeIsiPath(ctx, sourceSnapshotID); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			log.Infof("Snapshot source volume isiPath is '%s'", snapshotSourceVolumeIsiPath)

			if snapshotIsiPath, err = isiConfig.isiSvc.GetSnapshotIsiPath(ctx, snapshotSourceVolumeIsiPath, sourceSnapshotID); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			log.Debugf("The Isilon directory path of snapshot is= '%s'", snapshotIsiPath)

			vcs := req.GetVolumeCapabilities()
			if len(vcs) == 0 {
				return nil, status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, "volume capabilty is required"))
			}

			for _, vc := range vcs {
				if vc == nil {
					return nil, status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, "volume capabilty is required"))
				}

				am := vc.GetAccessMode()
				if am == nil {
					return nil, status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, "access mode is required"))
				}

				if am.Mode == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
					isROVolumeFromSnapshot = true
					break
				}
			}
		} else if volume := contentSource.GetVolume(); volume != nil {
			sourceVolumeID = volume.GetVolumeId()
			log.Infof("Creating volume from existing volume ID: '%s'", sourceVolumeID)
		}
	}
	if isReplication {
		log.Info("Preparing volume replication")

		vgPrefix, ok := params[s.WithRP(KeyReplicationVGPrefix)]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "replication enabled but no volume group prefix specified in storage class")
		}

		rpo, ok := params[s.WithRP(KeyReplicationRPO)]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "replication enabled but no RPO specified in storage class")
		}

		rpoEnum := RPOEnum(rpo)
		if err := rpoEnum.IsValid(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid rpo value")
		}

		rpoint, err := rpoEnum.ToInt()
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "unable to parse rpo seconds")
		}

		remoteSystemName, ok := params[s.WithRP(KeyReplicationRemoteSystem)]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "replication enabled but no remote system specified in storage class")
		}

		remoteIsiConfig, err := s.getIsilonConfig(ctx, &remoteSystemName)
		if err != nil {
			log.Error("Failed to get Isilon config with error ", err.Error())
			return nil, status.Errorf(codes.InvalidArgument, "can't find cluster with name %s in driver config", remoteSystemName)
		}
		remoteSystemEndpoint := remoteIsiConfig.Endpoint

		namespace := ""
		if ignoreNS, ok := params[s.WithRP(KeyReplicationIgnoreNamespaces)]; ok && ignoreNS == "false" {
			pvcNS, ok := params[KeyCSIPVCNamespace]
			if ok {
				namespace = pvcNS + "-"
			}
		}

		vgName := vgPrefix + "-" + namespace + remoteSystemEndpoint + "-" + rpo
		if len(vgName) > 128 {
			vgName = vgName[:128]
		}
		VolumeGroupDir = vgName
		var vg isi.Volume
		vg, err = isiConfig.isiSvc.client.GetVolumeWithIsiPath(ctx, isiPath, "", vgName)
		if err != nil {
			if apiErr, ok := err.(*isiApi.JSONError); ok && apiErr.StatusCode == 404 {
				vg, err = isiConfig.isiSvc.client.CreateVolumeWithIsipath(ctx, isiPath, vgName, "0777")
			}
			if err != nil {
				return nil, err
			}
		}

		ppName := strings.ReplaceAll(vg.Name, ".", "-")
		_, err = isiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
		if err != nil {
			if apiErr, ok := err.(*isiApi.JSONError); ok && apiErr.StatusCode == 404 {
				err := isiConfig.isiSvc.client.CreatePolicy(ctx, ppName, rpoint, isiPath+"/"+vgName, isiPath+"/"+vgName, remoteSystemEndpoint)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "can't create protection policy %s", err.Error())
				}
			} else {
				return nil, status.Errorf(codes.Internal, "can't ensure protection policy exists %s", err.Error())
			}
		}

		isiPath = isiPath + "/" + VolumeGroupDir
	}

	foundVol = false
	if isROVolumeFromSnapshot {
		if isReplication {
			return nil, errors.New("Unable to create replication volume from snapshot")
		}
		path = snapshotIsiPath
		accessZone = constants.DefaultAccessZone
		snapshotSrc, err := isiConfig.isiSvc.GetSnapshot(ctx, sourceSnapshotID)
		if err != nil {
			return nil, fmt.Errorf("failed to get snapshot id '%s', error '%v'", sourceSnapshotID, err)
		}
		snapshotName := snapshotSrc.Name

		// Populate names for snapshot's tracking dir, snapshot tracking dir entry for this volume
		snapshotTrackingDir = isiConfig.isiSvc.GetSnapshotTrackingDirName(snapshotName)
		snapshotTrackingDirEntryForVolume = fPath.Join(snapshotTrackingDir, req.GetName())

		// Check if entry for this volume is present in snapshot tracking dir
		if isiConfig.isiSvc.IsVolumeExistent(ctx, isiPath, "", snapshotTrackingDirEntryForVolume) {
			log.Debugf("the path '%s' has already existed", path)
			foundVol = true
		} else {
			// Allow creation of only one active volume from a snapshot at any point in time
			totalSubDirectories, _ := isiConfig.isiSvc.GetSubDirectoryCount(ctx, isiPath, snapshotTrackingDir)
			if totalSubDirectories > 2 {
				return nil, fmt.Errorf("another RO volume from this snapshot is already present")
			}
		}
	} else {
		path = utils.GetPathForVolume(isiPath, req.GetName())
		// to ensure idempotency, check if the volume still exists.
		// k8s might have made the same CreateVolume call in quick succession and the volume was already created in the first run
		if isiConfig.isiSvc.IsVolumeExistent(ctx, isiPath, "", req.GetName()) {
			log.Debugf("the path '%s' has already existed", path)
			foundVol = true
		}
	}

	if !foundVol && isROVolumeFromSnapshot {
		// Create an entry for this volume in snapshot tracking dir
		if err = isiConfig.isiSvc.CreateVolume(ctx, isiPath, snapshotTrackingDir, volumePathPermissions); err != nil {
			return nil, err
		}
		if err = isiConfig.isiSvc.CreateVolume(ctx, isiPath, snapshotTrackingDirEntryForVolume, volumePathPermissions); err != nil {
			return nil, err
		}
	}

	if export, err = isiConfig.isiSvc.GetExportWithPathAndZone(ctx, path, accessZone); err != nil || export == nil {

		var errMsg string
		if err == nil {
			if foundVol {
				return nil, status.Error(codes.Internal, utils.GetMessageWithRunID(runID, "the export may not be ready yet and the path is '"+path+"'"))
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
		log.Debugf("id of the corresponding nfs export of existing volume '%s' has been resolved to '%d'", req.GetName(), exportID)
		if exportID != 0 {
			if foundVol || isROVolumeFromSnapshot {
				return s.getCreateVolumeResponse(ctx, exportID, req.GetName(), path, export.Zone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName), nil
			}
			// in case the export exists but no related volume (directory)
			if err = isiConfig.isiSvc.UnexportByIDWithZone(ctx, exportID, accessZone); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			exportID = 0
		}
	}

	// create volume (directory) with ACL 0777
	if !isROVolumeFromSnapshot {
		if len(headerMetadata) == 0 {
			if err = isiConfig.isiSvc.CreateVolume(ctx, isiPath, req.GetName(), volumePathPermissions); err != nil {
				return nil, err
			}
		} else {
			log.Debugf("create volume with header metadata '%s' has been resolved to '%v'", req.GetName(), headerMetadata)
			if err = isiConfig.isiSvc.CreateVolumeWithMetaData(ctx, isiPath, req.GetName(), volumePathPermissions, headerMetadata); err != nil {
				return nil, err
			}
		}
	}

	// if volume content source is not null and new volume request is not for RO volume from snapshot,
	// copy content from the datasource
	if contentSource != nil && !isROVolumeFromSnapshot {
		err = s.createVolumeFromSource(ctx, isiConfig, isiPath, contentSource, req, sizeInBytes)
		if err != nil {
			// Clear volume since the volume creation is not successful
			if err := isiConfig.isiSvc.DeleteVolume(ctx, isiPath, req.GetName()); err != nil {
				log.Infof("Delete volume in CreateVolume returned error '%s'", err)
			}
			return nil, err
		}
	}

	if !foundVol && !isROVolumeFromSnapshot {
		// create quota
		if quotaID, err = isiConfig.isiSvc.CreateQuota(ctx, path, req.GetName(), sizeInBytes, s.opts.QuotaEnabled); err != nil {
			log.Errorf("error creating quota ('%s', '%d' bytes), abort, also roll back by deleting the newly created volume: '%v'", req.GetName(), sizeInBytes, err)
			//roll back, delete the newly created volume
			if err = isiConfig.isiSvc.DeleteVolume(ctx, isiPath, req.GetName()); err != nil {
				return nil, fmt.Errorf("rollback (deleting volume '%s') failed with error : '%v'", req.GetName(), err)
			}
			return nil, fmt.Errorf("error creating quota ('%s', '%d' bytes), abort, also succesfully rolled back by deleting the newly created volume", req.GetName(), sizeInBytes)
		}
	}

	// export volume in the given access zone, also add normalized quota id to the description field, in DeleteVolume,
	// the quota ID will be used for the quota to be directly deleted by ID
	if isROVolumeFromSnapshot {
		if exportID, err = isiConfig.isiSvc.ExportVolumeWithZone(ctx, path, "", accessZone, ""); err == nil && exportID != 0 {
			// get the export and retry if not found to ensure the export has been created
			for i := 0; i < MaxRetries; i++ {
				if export, _ := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone); export != nil {
					// Add dummy localhost entry for pvc security
					if !isiConfig.isiSvc.IsHostAlreadyAdded(ctx, exportID, accessZone, utils.DummyHostNodeID) {
						err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, utils.DummyHostNodeID, isiConfig.isiSvc.AddExportClientByIDWithZone)
						if err != nil {
							log.Debugf("Error while adding dummy localhost entry to export '%d'", exportID)
						}
					}
					// return the response
					return s.getCreateVolumeResponse(ctx, exportID, req.GetName(), path, accessZone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName), nil
				}
				time.Sleep(RetrySleepTime)
				log.Printf("Begin to retry '%d' time(s), for export id '%d' and path '%s'\n", i+1, exportID, path)
			}
		} else {
			return nil, err
		}
	} else {

		if exportID, err = isiConfig.isiSvc.ExportVolumeWithZone(ctx, isiPath, req.GetName(), accessZone, utils.GetQuotaIDWithCSITag(quotaID)); err == nil && exportID != 0 {
			// get the export and retry if not found to ensure the export has been created
			for i := 0; i < MaxRetries; i++ {
				if export, _ := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone); export != nil {
					// Add dummy localhost entry for pvc security
					if !isiConfig.isiSvc.IsHostAlreadyAdded(ctx, exportID, accessZone, utils.DummyHostNodeID) {
						err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, utils.DummyHostNodeID, isiConfig.isiSvc.AddExportClientByIDWithZone)
						if err != nil {
							log.Debugf("Error while adding dummy localhost entry to export '%d'", exportID)
						}
					}
					// return the response
					return s.getCreateVolumeResponse(ctx, exportID, req.GetName(), path, accessZone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName), nil
				}
				time.Sleep(RetrySleepTime)
				log.Printf("Begin to retry '%d' time(s), for export id '%d' and path '%s'\n", i+1, exportID, path)
			}
		} else {
			// clear quota and delete volume since the export cannot be created
			if error := isiConfig.isiSvc.ClearQuotaByID(ctx, quotaID); error != nil {
				log.Infof("Clear Quota returned error '%s'", error)
			}
			if error := isiConfig.isiSvc.DeleteVolume(ctx, isiPath, req.GetName()); error != nil {
				log.Infof("Delete volume in CreateVolume returned error '%s'", error)
			}
			return nil, err
		}
	}
	return nil, status.Error(codes.Internal, utils.GetMessageWithRunID(runID, "the export id '"+strconv.Itoa(exportID)+"' and path '"+path+"' may not be ready yet after retrying"))
}

func (s *service) createVolumeFromSnapshot(ctx context.Context, isiConfig *IsilonClusterConfig,
	isiPath, normalizedSnapshotID, dstVolumeName string, sizeInBytes int64) error {
	var snapshotSrc isi.Snapshot
	var err error

	// parse the input snapshot id and fetch it's components
	srcSnapshotID, _, err := utils.ParseNormalizedSnapshotID(ctx, normalizedSnapshotID)
	if err != nil {
		return err
	}

	if snapshotSrc, err = isiConfig.isiSvc.GetSnapshot(ctx, srcSnapshotID); err != nil {
		return fmt.Errorf("failed to get snapshot id '%s', error '%v'", srcSnapshotID, err)
	}

	// check source snapshot size
	snapshotSourceVolumeIsiPath := path.Dir(snapshotSrc.Path)
	size := isiConfig.isiSvc.GetSnapshotSize(ctx, snapshotSourceVolumeIsiPath, snapshotSrc.Name)
	if size > sizeInBytes {
		return fmt.Errorf("specified size '%d' is smaller than source snapshot size '%d'", sizeInBytes, size)
	}

	if _, err = isiConfig.isiSvc.CopySnapshot(ctx, isiPath, snapshotSourceVolumeIsiPath, snapshotSrc.Id, dstVolumeName); err != nil {
		return fmt.Errorf("failed to copy snapshot id '%s', error '%s'", srcSnapshotID, err.Error())
	}

	return nil
}

func (s *service) createVolumeFromVolume(ctx context.Context, isiConfig *IsilonClusterConfig, isiPath, srcVolumeName, dstVolumeName string, sizeInBytes int64) error {
	var err error
	if isiConfig.isiSvc.IsVolumeExistent(ctx, isiPath, "", srcVolumeName) {
		// check source volume size
		size := isiConfig.isiSvc.GetVolumeSize(ctx, isiPath, srcVolumeName)
		if size > sizeInBytes {
			return fmt.Errorf("specified size '%d' is smaller than source volume size '%d'", sizeInBytes, size)
		}

		if _, err = isiConfig.isiSvc.CopyVolume(ctx, isiPath, srcVolumeName, dstVolumeName); err != nil {
			return fmt.Errorf("failed to copy volume name '%s', error '%v'", srcVolumeName, err)
		}
	} else {
		return fmt.Errorf("failed to get volume name '%s', error '%v'", srcVolumeName, err)
	}

	return nil
}

func (s *service) createVolumeFromSource(
	ctx context.Context,
	isiConfig *IsilonClusterConfig,
	isiPath string,
	contentSource *csi.VolumeContentSource,
	req *csi.CreateVolumeRequest,
	sizeInBytes int64) error {
	if contentSnapshot := contentSource.GetSnapshot(); contentSnapshot != nil {
		// create volume from source snapshot
		if err := s.createVolumeFromSnapshot(ctx, isiConfig, isiPath, contentSnapshot.GetSnapshotId(), req.GetName(), sizeInBytes); err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	}

	if contentVolume := contentSource.GetVolume(); contentVolume != nil {
		// create volume from source volume
		srcVolumeName, _, _, _, err := utils.ParseNormalizedVolumeID(ctx, contentVolume.GetVolumeId())
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		if err := s.createVolumeFromVolume(ctx, isiConfig, isiPath, srcVolumeName, req.GetName(), sizeInBytes); err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	}
	return nil
}

func (s *service) getCreateVolumeResponse(ctx context.Context, exportID int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName string) *csi.CreateVolumeResponse {
	return &csi.CreateVolumeResponse{
		Volume: s.getCSIVolume(ctx, exportID, volName, path, accessZone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName),
	}
}

func (s *service) getCSIVolume(ctx context.Context, exportID int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName string) *csi.Volume {
	// Make the additional volume attributes
	attributes := map[string]string{
		"ID":                strconv.Itoa(exportID),
		"Name":              volName,
		"Path":              path,
		"AccessZone":        accessZone,
		"AzServiceIP":       azServiceIP,
		"RootClientEnabled": rootClientEnabled,
		"ClusterName":       clusterName,
	}

	// Fetch log handler
	ctx, log, _ := GetRunIDLog(ctx)

	log.Debugf("Attributes '%v'", attributes)

	// Set content source as part of create volume response if volume is created from snapshot or existing volume
	// ContentSource is an optional field as part of CSI spec, but provisioner side car version 1.4.0
	// mandates it
	var contentSource *csi.VolumeContentSource
	if sourceSnapshotID != "" {
		contentSource = &csi.VolumeContentSource{
			Type: &csi.VolumeContentSource_Snapshot{
				Snapshot: &csi.VolumeContentSource_SnapshotSource{
					SnapshotId: sourceSnapshotID,
				},
			},
		}
	} else if sourceVolumeID != "" {
		contentSource = &csi.VolumeContentSource{
			Type: &csi.VolumeContentSource_Volume{
				Volume: &csi.VolumeContentSource_VolumeSource{
					VolumeId: sourceVolumeID,
				},
			},
		}
	}

	vi := &csi.Volume{
		VolumeId:      utils.GetNormalizedVolumeID(ctx, volName, exportID, accessZone, clusterName),
		CapacityBytes: sizeInBytes,
		VolumeContext: attributes,
		ContentSource: contentSource,
	}
	return vi
}

func (s *service) DeleteVolume(
	ctx context.Context,
	req *csi.DeleteVolumeRequest) (
	*csi.DeleteVolumeResponse, error) {
	// TODO more checks need to be done, e.g. if access mode is VolumeCapability_AccessMode_MULTI_NODE_XXX, then other nodes might still be using this volume, thus the delete should be skipped
	// Fetch log handler
	ctx, log, _ := GetRunIDLog(ctx)

	// validate request
	if err := s.ValidateDeleteVolumeRequest(ctx, req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// parse the input volume id and fetch it's components
	volName, exportID, accessZone, clusterName, err := utils.ParseNormalizedVolumeID(ctx, req.GetVolumeId())
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	// probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		log.Error("Failed to probe with error: " + err.Error())
		return nil, err
	}
	s.logStatistics()
	quotaEnabled := s.opts.QuotaEnabled

	export, err := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone)
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

	isROVolumeFromSnapshot := isiConfig.isiSvc.isROVolumeFromSnapshot(exportPath)
	// If it is a RO volume and dataSource is snapshot
	if isROVolumeFromSnapshot {
		if err := s.processSnapshotTrackingDirectoryDuringDeleteVolume(ctx, volName, export, isiConfig); err != nil {
			return nil, err
		}
		return &csi.DeleteVolumeResponse{}, nil
	}

	isiPath := utils.GetIsiPathFromExportPath(exportPath)
	// to ensure idempotency, check if the volume and export still exists.
	// k8s might have made the same DeleteVolume call in quick succession and the volume was already deleted in the first run
	log.Debugf("controller begins to delete volume, name '%s', quotaEnabled '%t'", volName, quotaEnabled)
	if err := isiConfig.isiSvc.DeleteQuotaByExportIDWithZone(ctx, volName, exportID, accessZone); err != nil {
		jsonError, ok := err.(*isiApi.JSONError)
		if ok {
			if jsonError.StatusCode != 404 {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	if !isiConfig.isiSvc.IsVolumeExistent(ctx, isiPath, "", volName) {
		log.Debugf("volume '%s' not found, skip calling delete directory.", volName)
	} else {
		// Before deleting the Volume, we would like to check if there are any
		// NFS exports which still exist on the Volume. These exports could
		// have been created out-of-band outside of CSI Driver.
		path := utils.GetPathForVolume(isiPath, volName)
		params := isiApi.OrderedValues{
			{[]byte("path"), []byte(path)},
			{[]byte("zone"), []byte(accessZone)},
		}
		exports, err := isiConfig.isiSvc.GetExportsWithParams(ctx, params)
		if err != nil {
			jsonError, ok := err.(*isiApi.JSONError)
			if ok {
				if jsonError.StatusCode != 404 {
					return nil, err
				}
			}
			return nil, err
		}

		if exports != nil && exports.Total == 1 && exports.Exports[0].ID == exportID {
			log.Infof("controller begins to unexport id '%d', target path '%s', access zone '%s'", exportID, volName, accessZone)
			if err := isiConfig.isiSvc.UnexportByIDWithZone(ctx, exportID, accessZone); err != nil {
				return nil, err
			}
		} else if exports != nil && exports.Total > 1 {
			return nil, fmt.Errorf("exports found for volume %s in AccessZone %s. It is not safe to delete the volume", volName, accessZone)
		}

		if err := isiConfig.isiSvc.DeleteVolume(ctx, isiPath, volName); err != nil {
			return nil, err
		}
	}
	return &csi.DeleteVolumeResponse{}, nil
}

func (s *service) processSnapshotTrackingDirectoryDuringDeleteVolume(
	ctx context.Context,
	volName string,
	export isi.Export,
	isiConfig *IsilonClusterConfig) error {
	exportPath := (*export.Paths)[0]

	// Fetch log handler
	ctx, log, _ := GetRunIDLog(ctx)

	// Get snapshot name
	snapshotName, err := isiConfig.isiSvc.GetSnapshotNameFromIsiPath(ctx, exportPath)
	if err != nil {
		return err
	}
	log.Debugf("snapshot name associated with volume '%s' is '%s'", volName, snapshotName)

	// Populate names for snapshot's tracking dir, snapshot tracking dir entry for this volume
	// and snapshot delete marker
	snapshotTrackingDir := isiConfig.isiSvc.GetSnapshotTrackingDirName(snapshotName)
	snapshotTrackingDirEntryForVolume := path.Join(snapshotTrackingDir, volName)
	snapshotTrackingDirDeleteMarker := path.Join(snapshotTrackingDir, DeleteSnapshotMarker)

	// Delete the snapshot tracking directory entry for this volume
	isiPath, _, _ := isiConfig.isiSvc.GetSnapshotIsiPathComponents(exportPath)
	log.Debugf("Delete the snapshot tracking directory entry '%s' for volume '%s'", snapshotTrackingDirEntryForVolume, volName)
	if isiConfig.isiSvc.IsVolumeExistent(ctx, isiPath, "", snapshotTrackingDirEntryForVolume) {
		if err := isiConfig.isiSvc.DeleteVolume(ctx, isiPath, snapshotTrackingDirEntryForVolume); err != nil {
			return err
		}
	}

	// Get subdirectories count of snapshot tracking dir.
	// Every directory will have two subdirectory entries . and ..
	totalSubDirectories, err := isiConfig.isiSvc.GetSubDirectoryCount(ctx, isiPath, snapshotTrackingDir)
	if err != nil {
		log.Errorf("failed to get subdirectories count of snapshot tracking dir '%s'", snapshotTrackingDir)
		return nil
	}

	// Delete snapshot tracking directory, if required (i.e., if there is a
	// snapshot delete marker as a result of snapshot deletion on k8s side)
	if isiConfig.isiSvc.IsVolumeExistent(ctx, isiPath, "", snapshotTrackingDirDeleteMarker) {
		// There are no more volumes present which were created using this snapshot
		// This indicates that there are only three subdirectories ., .. and snapshot delete marker.
		if totalSubDirectories == 3 {
			err = isiConfig.isiSvc.UnexportByIDWithZone(ctx, export.ID, "")
			if err != nil {
				log.Errorf("failed to delete snapshot directory export with id '%v'", export.ID)
				return nil
			}
			// Delete snapshot tracking directory
			if err := isiConfig.isiSvc.DeleteVolume(ctx, isiPath, snapshotTrackingDir); err != nil {
				log.Errorf("error while deleting snapshot tracking directory '%s'", path.Join(isiPath, snapshotName))
				return nil
			}
			// Delete snapshot
			err = isiConfig.isiSvc.client.RemoveSnapshot(context.Background(), -1, snapshotName)
			if err != nil {
				log.Errorf("error deleting snapshot: '%s'", err.Error())
				return nil
			}
		}
	}

	if totalSubDirectories == 2 {
		// Delete snapshot tracking directory
		if err := isiConfig.isiSvc.DeleteVolume(ctx, isiPath, snapshotTrackingDir); err != nil {
			log.Errorf("error while deleting snapshot tracking directory '%s'", path.Join(isiPath, snapshotName))
			return nil
		}
	}

	return nil
}

func (s *service) ControllerExpandVolume(
	ctx context.Context,
	req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	// Fetch log handler
	ctx, log, _ := GetRunIDLog(ctx)

	volName, exportID, accessZone, clusterName, err := utils.ParseNormalizedVolumeID(ctx, req.GetVolumeId())
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	// auto probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	requiredBytes := req.GetCapacityRange().GetRequiredBytes()

	// when Quota is disabled, always return success
	// Otherwise, update the quota size as requested
	if s.opts.QuotaEnabled {
		quota, err := isiConfig.isiSvc.GetVolumeQuota(ctx, volName, exportID, accessZone)
		if err != nil {
			return nil, status.Error(codes.NotFound, err.Error())
		}

		quotaSize := quota.Thresholds.Hard
		if requiredBytes <= quotaSize {
			// volume capacity is larger than or equal to the target capacity, return OK
			return &csi.ControllerExpandVolumeResponse{CapacityBytes: quotaSize, NodeExpansionRequired: false}, nil
		}

		if err = isiConfig.isiSvc.UpdateQuotaSize(ctx, quota.Id, requiredBytes); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &csi.ControllerExpandVolumeResponse{CapacityBytes: requiredBytes, NodeExpansionRequired: false}, nil
}

func (s *service) getAddClientFunc(rootClientEnabled bool, isiConfig *IsilonClusterConfig) (addClientFunc func(ctx context.Context, exportID int, accessZone, clientIP string) error) {
	if rootClientEnabled {
		return isiConfig.isiSvc.AddExportRootClientByIDWithZone
	}

	return isiConfig.isiSvc.AddExportClientByIDWithZone
}

/*
 * ControllerPublishVolume : Checks all params and validity
 */
func (s *service) ControllerPublishVolume(
	ctx context.Context,
	req *csi.ControllerPublishVolumeRequest) (
	*csi.ControllerPublishVolumeResponse, error) {
	var (
		accessZone             string
		exportPath             string
		isiPath                string
		isROVolumeFromSnapshot bool
	)

	// Fetch log handler
	ctx, log, runID := GetRunIDLog(ctx)

	volumeContext := req.GetVolumeContext()
	if volumeContext != nil {
		log.Printf("VolumeContext:")
		for key, value := range volumeContext {
			log.Printf("    [%s]=%s", key, value)
		}
	}

	volID := req.GetVolumeId()
	if volID == "" {
		return nil, status.Error(codes.InvalidArgument,
			utils.GetMessageWithRunID(runID, "volume ID is required"))
	}

	volName, exportID, accessZone, clusterName, err := utils.ParseNormalizedVolumeID(ctx, volID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, "failed to parse volume ID '%s', error : '%v'", volID, err))
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	if err := s.autoProbe(ctx, isiConfig); err != nil {
		log.Error("Failed to probe with error: " + err.Error())
		return nil, err
	}

	if exportID == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid export ID")
	}

	if exportPath = volumeContext[ExportPathParam]; exportPath == "" {
		exportPath = utils.GetPathForVolume(isiConfig.IsiPath, volName)
	}

	isROVolumeFromSnapshot = isiConfig.isiSvc.isROVolumeFromSnapshot(exportPath)
	if isROVolumeFromSnapshot {
		log.Info("Volume source is snapshot")
		accessZone = constants.DefaultAccessZone
		if export, err := isiConfig.isiSvc.GetExportWithPathAndZone(ctx, exportPath, accessZone); err != nil || export == nil {
			return nil, status.Errorf(codes.Internal, utils.GetMessageWithRunID(runID, "error retrieving export for '%s'", exportPath))
		}
	} else {
		isiPath = utils.GetIsiPathFromExportPath(exportPath)
		vol, err := isiConfig.isiSvc.GetVolume(ctx, isiPath, "", volName)
		if err != nil || vol.Name == "" {
			return nil, status.Errorf(codes.Internal,
				utils.GetMessageWithRunID(runID, "failure checking volume status before controller publish: '%s'",
					err.Error()))
		}
	}

	nodeID := req.GetNodeId()
	if nodeID == "" {
		return nil, status.Error(codes.InvalidArgument,
			utils.GetMessageWithRunID(runID, "node ID is required"))
	}

	vc := req.GetVolumeCapability()
	if vc == nil {
		return nil, status.Error(codes.InvalidArgument,
			utils.GetMessageWithRunID(runID, "volume capability is required"))
	}

	am := vc.GetAccessMode()
	if am == nil {
		return nil, status.Error(codes.InvalidArgument,
			utils.GetMessageWithRunID(runID, "access mode is required"))
	}

	if am.Mode == csi.VolumeCapability_AccessMode_UNKNOWN {
		return nil, status.Error(codes.InvalidArgument,
			utils.GetMessageWithRunID(runID, errUnknownAccessMode))
	}

	vcs := []*csi.VolumeCapability{req.GetVolumeCapability()}
	if !checkValidAccessTypes(vcs) {
		return nil, status.Error(codes.InvalidArgument,
			utils.GetMessageWithRunID(runID, errUnknownAccessType))
	}

	rootClientEnabled := false
	rootClientEnabledStr := volumeContext[RootClientEnabledParam]
	val, err := strconv.ParseBool(rootClientEnabledStr)
	if err == nil {
		rootClientEnabled = val
	}

	addClientFunc := s.getAddClientFunc(rootClientEnabled, isiConfig)

	switch am.Mode {
	case csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER:
		if isROVolumeFromSnapshot {
			err = fmt.Errorf("unsupported access mode: '%s'", am.String())
			break
		}

		if !isiConfig.isiSvc.IsHostAlreadyAdded(ctx, exportID, accessZone, utils.DummyHostNodeID) {
			err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, utils.DummyHostNodeID, isiConfig.isiSvc.AddExportClientByIDWithZone)
		}

		err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, nodeID, addClientFunc)
		if err == nil && rootClientEnabled {
			err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, nodeID, isiConfig.isiSvc.AddExportClientByIDWithZone)
		}
	case csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY:
		err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, nodeID, isiConfig.isiSvc.AddExportReadOnlyClientByIDWithZone)
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER:
		if isROVolumeFromSnapshot {
			err = fmt.Errorf("unsupported access mode: '%s'", am.String())
			break
		}
		if isiConfig.isiSvc.OtherClientsAlreadyAdded(ctx, exportID, accessZone, nodeID) {
			return nil, status.Errorf(codes.FailedPrecondition, utils.GetMessageWithRunID(runID,
				"export '%d' in access zone '%s' already has other clients added to it, and the access mode is "+
					"%s, thus the request fails", exportID, accessZone, am.Mode))
		}

		if !isiConfig.isiSvc.IsHostAlreadyAdded(ctx, exportID, accessZone, utils.DummyHostNodeID) {
			err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, utils.DummyHostNodeID, isiConfig.isiSvc.AddExportClientByIDWithZone)
		}
		err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, nodeID, addClientFunc)
		if err == nil && rootClientEnabled {
			err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, nodeID, isiConfig.isiSvc.AddExportClientByIDWithZone)
		}
	default:
		return nil, status.Errorf(codes.InvalidArgument, utils.GetMessageWithRunID(runID, "unsupported access mode: '%s'", am.String()))
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, utils.GetMessageWithRunID(runID,
			"internal error occurred when attempting to add client ip '%s' to export '%d', error : '%v'", nodeID, exportID, err))
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

	// Fetch log handler
	ctx, log, runID := GetRunIDLog(ctx)

	// parse the input volume id and fetch it's components
	volID := req.GetVolumeId()
	volName, _, _, clusterName, err := utils.ParseNormalizedVolumeID(ctx, volID)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	if err := s.autoProbe(ctx, isiConfig); err != nil {
		log.Error("Failed to probe with error: " + err.Error())
		return nil, err
	}

	volumeContext := req.GetVolumeContext()
	if exportPath = volumeContext[ExportPathParam]; exportPath == "" {
		exportPath = utils.GetPathForVolume(s.opts.Path, volName)
	}
	isiPath = utils.GetIsiPathFromExportPath(exportPath)

	vol, err := s.getVolByName(ctx, isiPath, volName, isiConfig)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			utils.GetMessageWithRunID(runID, "failure checking volume status for capabilities: '%s'",
				err.Error()))
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
	// TODO The below implementation(commented code) doesn't work for multi-cluster.
	// Add multi-cluster support by considering both MaxEntries and StartingToken(if specified) attributes.
	/*
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
				//<TODO> update with input cluster config
				clusterConfig := IsilonClusterConfig{}
				volume := s.getCSIVolume(export.ID, volName, path, export.Zone, 0, clusterConfig.Endpoint, "false", "", "", "")
				entries[i] = &csi.ListVolumesResponse_Entry{
					Volume: volume,
				}
				i++
			}
		}
		resp.Entries = entries
		return resp, nil
	*/

	return nil, status.Error(codes.Unimplemented, "")
}

func (s *service) ListSnapshots(context.Context,
	*csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (s *service) ControllerUnpublishVolume(
	ctx context.Context,
	req *csi.ControllerUnpublishVolumeRequest) (
	*csi.ControllerUnpublishVolumeResponse, error) {

	// Fetch log handler
	ctx, log, runID := GetRunIDLog(ctx)

	if req.VolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, utils.GetMessageWithRunID(runID, "ControllerUnpublishVolumeRequest.VolumeId is empty"))
	}

	_, exportID, accessZone, clusterName, err := utils.ParseNormalizedVolumeID(ctx, req.VolumeId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, "failed to parse volume ID '%s', error : '%s'", req.VolumeId, err.Error()))
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	// auto probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, utils.GetMessageWithRunID(runID, err.Error()))
	}

	nodeID := req.GetNodeId()
	if nodeID == "" {
		return nil, status.Error(codes.InvalidArgument,
			utils.GetMessageWithRunID(runID, "node ID is required"))
	}

	if err := isiConfig.isiSvc.RemoveExportClientByIDWithZone(ctx, exportID, accessZone, nodeID); err != nil {
		return nil, status.Errorf(codes.Internal, utils.GetMessageWithRunID(runID, "error encountered when"+
			" trying to remove client '%s' from export '%d' with access zone '%s' on cluster '%s'", nodeID, exportID, accessZone, clusterName))
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (s *service) GetCapacity(
	ctx context.Context,
	req *csi.GetCapacityRequest) (
	*csi.GetCapacityResponse, error) {

	var clusterName string
	params := req.GetParameters()

	// Fetch log handler
	ctx, log, runID := GetRunIDLog(ctx)

	if _, ok := params[ClusterNameParam]; ok {
		if params[ClusterNameParam] == "" {
			clusterName = s.defaultIsiClusterName
		} else {
			clusterName = params[ClusterNameParam]
		}
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	if err := s.autoProbe(ctx, isiConfig); err != nil {
		log.Error("Failed to probe with error: " + err.Error())
		return nil, err
	}

	// Optionally validate the volume capability
	vcs := req.GetVolumeCapabilities()
	if vcs != nil {
		supported, reason := validateVolumeCaps(vcs, nil)
		if !supported {
			log.Errorf("GetVolumeCapabilities failed with error: '%s'", reason)
			return nil, status.Errorf(codes.InvalidArgument, utils.GetMessageWithRunID(runID, reason))
		}
	}

	//pass the key(s) to rest api
	keyArray := []string{"ifs.bytes.avail"}

	stat, err := isiConfig.isiSvc.GetStatistics(ctx, keyArray)
	if err != nil || len(stat.StatsList) < 1 {
		return nil, status.Errorf(codes.Internal, utils.GetMessageWithRunID(runID, "Could not retrieve capacity. Error '%s'", err.Error()))
	}
	if stat.StatsList[0].Error != "" {
		return nil, status.Errorf(codes.Internal, utils.GetMessageWithRunID("Could not retrieve capacity. Data returned error '%s'", stat.StatsList[0].Error))
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

	capabilities := []*csi.ControllerServiceCapability{
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
				},
			},
		},
		/*{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
				},
			},
		},*/
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
				},
			},
		},
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
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
				},
			},
		},
	}

	healthMonitorCapabilities := []*csi.ControllerServiceCapability{
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_VOLUME_CONDITION,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_GET_VOLUME,
				},
			},
		},
	}

	if s.opts.IsHealthMonitorEnabled {
		capabilities = append(capabilities, healthMonitorCapabilities...)
	}

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: capabilities,
	}, nil
}

func (s *service) controllerProbe(ctx context.Context, clusterConfig *IsilonClusterConfig) error {
	// Fetch log handler
	ctx, log, _ := GetRunIDLog(ctx)

	if err := s.validateOptsParameters(clusterConfig); err != nil {
		return fmt.Errorf("controller probe failed : '%v'", err)
	}

	if clusterConfig.isiSvc == nil {
		logLevel := utils.GetCurrentLogLevel()
		var err error
		clusterConfig.isiSvc, err = s.GetIsiService(ctx, clusterConfig, logLevel)
		if clusterConfig.isiSvc == nil {
			return errors.New("clusterConfig.isiSvc (type isiService) is nil, probe failed")
		}
		if err != nil {
			return err
		}
	}

	if err := clusterConfig.isiSvc.TestConnection(ctx); err != nil {
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

	// Fetch log handler
	ctx, log, runID := GetRunIDLog(ctx)

	log.Infof("CreateSnapshot started")
	// parse the input volume id and fetch it's components
	_, _, _, clusterName, err := utils.ParseNormalizedVolumeID(ctx, req.GetSourceVolumeId())
	if err != nil {
		return nil, status.Error(codes.NotFound, utils.GetMessageWithRunID(runID, err.Error()))
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	// auto probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, utils.GetMessageWithRunID(runID, err.Error()))
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
			isiPath = isiConfig.IsiPath
		} else {
			isiPath = params[IsiPathParam]
		}
	} else {
		// use the default isiPath if not set in the storage class
		isiPath = isiConfig.IsiPath
	}
	srcVolumeID, snapshotName, err := s.validateCreateSnapshotRequest(ctx, req, isiPath, isiConfig)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, err.Error()))
	}

	log.Infof("snapshot name is '%s' and source volume ID is '%s' ", snapshotName, srcVolumeID)
	// check if snapshot already exists
	var snapshotByName isi.Snapshot
	log.Infof("check for existence of snapshot '%s'", snapshotName)
	if snapshotByName, err = isiConfig.isiSvc.GetSnapshot(ctx, snapshotName); snapshotByName != nil {
		if path.Base(snapshotByName.Path) == srcVolumeID {
			// return the existent snapshot
			return s.getCreateSnapshotResponse(ctx, strconv.FormatInt(snapshotByName.Id, 10), req.GetSourceVolumeId(), snapshotByName.Created, isiConfig.isiSvc.GetSnapshotSize(ctx, isiPath, snapshotName), clusterName), nil
		}
		// return already exists error
		return nil, status.Error(codes.AlreadyExists,
			utils.GetMessageWithRunID(runID, "a snapshot with name '%s' already exists but is "+
				"incompatible with the specified source volume id '%s'", snapshotName, req.GetSourceVolumeId()))
	}

	// create new snapshot for source direcory
	path := utils.GetPathForVolume(isiPath, srcVolumeID)
	if snapshotNew, err = isiConfig.isiSvc.CreateSnapshot(ctx, path, snapshotName); err != nil {
		return nil, status.Error(codes.Internal, utils.GetMessageWithRunID(runID, err.Error()))
	}
	_, _ = isiConfig.isiSvc.GetSnapshot(ctx, snapshotName)

	log.Infof("snapshot creation is successful")
	// return the response
	return s.getCreateSnapshotResponse(ctx, strconv.FormatInt(snapshotNew.Id, 10), req.GetSourceVolumeId(), snapshotNew.Created, isiConfig.isiSvc.GetSnapshotSize(ctx, isiPath, snapshotName), clusterName), nil
}

// validateCreateSnapshotRequest validate the input params in CreateSnapshotRequest
func (s *service) validateCreateSnapshotRequest(
	ctx context.Context,
	req *csi.CreateSnapshotRequest, isiPath string, isiConfig *IsilonClusterConfig) (string, string, error) {

	// Fetch log handler
	ctx, log, runID := GetRunIDLog(ctx)

	srcVolumeID, _, _, clusterName, err := utils.ParseNormalizedVolumeID(ctx, req.GetSourceVolumeId())
	if err != nil {
		return "", "", status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, err.Error()))
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	if !isiConfig.isiSvc.IsVolumeExistent(ctx, isiPath, "", srcVolumeID) {
		return "", "", status.Error(codes.InvalidArgument,
			utils.GetMessageWithRunID(runID, "source volume id is invalid"))
	}

	snapshotName := req.GetName()
	if snapshotName == "" {
		return "", "", status.Error(codes.InvalidArgument,
			utils.GetMessageWithRunID(runID, "name cannot be empty"))
	}

	return srcVolumeID, snapshotName, nil
}

func (s *service) getCreateSnapshotResponse(ctx context.Context, snapshotID string, sourceVolumeID string, creationTime, sizeInBytes int64, clusterName string) *csi.CreateSnapshotResponse {
	snapID := utils.GetNormalizedSnapshotID(ctx, snapshotID, clusterName)
	return &csi.CreateSnapshotResponse{
		Snapshot: s.getCSISnapshot(snapID, sourceVolumeID, creationTime, sizeInBytes),
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

	// Fetch log handler
	ctx, log, runID := GetRunIDLog(ctx)

	log.Infof("DeleteSnapshot started")
	if req.GetSnapshotId() == "" {
		return nil, status.Errorf(codes.FailedPrecondition, utils.GetMessageWithRunID(runID, "snapshot id to be deleted is required"))
	}

	// parse the input snapshot id and fetch it's components
	snapshotID, clusterName, err := utils.ParseNormalizedSnapshotID(ctx, req.GetSnapshotId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to parse snapshot ID '%s', error : '%v'", req.GetSnapshotId(), err))
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	if err := s.autoProbe(ctx, isiConfig); err != nil {
		log.Error("Failed to probe with error: " + err.Error())
		return nil, err
	}

	id, err := strconv.ParseInt(snapshotID, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.Internal, utils.GetMessageWithRunID(runID, "cannot convert snapshot to integer: '%s'", err.Error()))
	}
	snapshot, err := isiConfig.isiSvc.GetSnapshot(ctx, snapshotID)
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
				return nil, status.Errorf(codes.Internal, utils.GetMessageWithRunID(runID, "cannot check the existence of the snapshot: '%s'", err.Error()))
			}
		} else {
			if jsonError.StatusCode == 404 {
				return &csi.DeleteSnapshotResponse{}, nil
			}
			return nil, status.Errorf(codes.Internal, utils.GetMessageWithRunID(runID, "cannot check the existence of the snapshot: '%s'", err.Error()))
		}
	}

	// Get snapshot path
	snapshotIsiPath, err := isiConfig.isiSvc.GetSnapshotIsiPath(ctx, isiConfig.IsiPath, snapshotID)
	if err != nil {
		return nil, status.Error(codes.Internal, utils.GetMessageWithRunID(runID, err.Error()))
	}
	log.Debugf("The Isilon directory path of snapshot is= %v", snapshotIsiPath)

	export, err := isiConfig.isiSvc.GetExportWithPathAndZone(ctx, snapshotIsiPath, "")
	if err != nil {
		// internal error
		return nil, err
	}

	deleteSnapshot := true
	// Check if there are any RO volumes created from this snapshot
	// Note: This is true only for RO volumes from snapshots
	if export != nil {
		if err := s.processSnapshotTrackingDirectoryDuringDeleteSnapshot(ctx, export, snapshotIsiPath, &deleteSnapshot, isiConfig); err != nil {
			log.Error("Failed to get RO volume from snapshot ", err.Error())
			return nil, err
		}
	}

	if deleteSnapshot {
		err = isiConfig.isiSvc.DeleteSnapshot(ctx, id, "")
		if err != nil {
			return nil, status.Errorf(codes.Internal, utils.GetMessageWithRunID(runID, "error deleting snapshot: '%s'", err.Error()))
		}
	}
	log.Infof("Snapshot with id '%s' deleted", snapshotID)
	return &csi.DeleteSnapshotResponse{}, nil
}

func (s *service) processSnapshotTrackingDirectoryDuringDeleteSnapshot(
	ctx context.Context,
	export isi.Export,
	snapshotIsiPath string,
	deleteSnapshot *bool,
	isiConfig *IsilonClusterConfig) error {

	// Fetch log handler
	ctx, log, _ := GetRunIDLog(ctx)

	// Populate names for snapshot's tracking dir and snapshot delete marker
	isiPath, snapshotName, _ := isiConfig.isiSvc.GetSnapshotIsiPathComponents(snapshotIsiPath)
	snapshotTrackingDir := isiConfig.isiSvc.GetSnapshotTrackingDirName(snapshotName)
	snapshotTrackingDirDeleteMarker := path.Join(snapshotTrackingDir, DeleteSnapshotMarker)

	// Check if the snapshot tracking dir is present (this indicates
	// there were some RO volumes created from this snapshot)
	// Get subdirectories count of snapshot tracking dir.
	// Every directory will have two subdirectory entries . and ..
	totalSubDirectories, _ := isiConfig.isiSvc.GetSubDirectoryCount(ctx, isiPath, snapshotTrackingDir)

	// There are no more volumes present which were created using this snapshot
	// Every directory will have two subdirectories . and ..
	if totalSubDirectories == IgnoreDotAndDotDotSubDirs || totalSubDirectories == 0 {
		if err := isiConfig.isiSvc.UnexportByIDWithZone(ctx, export.ID, ""); err != nil {
			return err
		}

		// Delete snapshot tracking directory
		if err := isiConfig.isiSvc.DeleteVolume(ctx, isiPath, snapshotTrackingDir); err != nil {
			log.Errorf("error while deleting snapshot tracking directory '%s'", path.Join(isiPath, snapshotTrackingDir))
		}
	} else {
		*deleteSnapshot = false
		// Set a marker in snapshot tracking dir to delete snapshot, once
		// all the volumes created from this snapshot were deleted
		log.Debugf("set DeleteSnapshotMarker marker in snapshot tracking dir")
		if err := isiConfig.isiSvc.CreateVolume(ctx, isiPath, snapshotTrackingDirDeleteMarker, isiConfig.IsiVolumePathPermissions); err != nil {
			return err
		}
	}

	return nil
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
		case csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER:
			break
		case csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER:
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

func addMetaData(params map[string]string) map[string]string {
	// CSI specific metadata header for authorization
	var headerMetadata = make(map[string]string)
	if _, ok := params[csiPersistentVolumeName]; ok {
		headerMetadata[headerPersistentVolumeName] = params[csiPersistentVolumeName]
	}

	if _, ok := params[csiPersistentVolumeClaimName]; ok {
		headerMetadata[headerPersistentVolumeClaimName] = params[csiPersistentVolumeClaimName]
	}

	if _, ok := params[csiPersistentVolumeClaimNamespace]; ok {
		headerMetadata[headerPersistentVolumeClaimNamespace] = params[csiPersistentVolumeClaimNamespace]
	}
	return headerMetadata
}
func (s *service) ControllerGetVolume(ctx context.Context,
	req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {

	// Fetch log handler
	ctx, log, runID := GetRunIDLog(ctx)

	abnormal := false
	message := ""
	var volume isi.Volume

	volID := req.GetVolumeId()
	if volID == "" {
		return nil, status.Error(codes.FailedPrecondition, utils.GetMessageWithRunID(runID, "no VolumeID found in request"))
	}

	volName, exportID, accessZone, clusterName, err := utils.ParseNormalizedVolumeID(ctx, volID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, err.Error()))
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	isiPath := isiConfig.IsiPath

	if err := s.autoProbe(ctx, isiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, utils.GetMessageWithRunID(runID, err.Error()))
	}

	//check if volume exists
	if !isiConfig.isiSvc.IsVolumeExistent(ctx, isiPath, volName, "") {
		abnormal = true
		message = fmt.Sprintf("volume does not exists at this path %v", isiPath)
	}

	//Fetch volume details
	if !abnormal {
		volume, err = isiConfig.isiSvc.GetVolume(ctx, isiPath, "", volName)
		if err != nil {
			abnormal = true
			message = fmt.Sprintf("error in getting '%s' volume '%v'", volName, err)
		}
	}

	if abnormal {
		return &csi.ControllerGetVolumeResponse{
			Volume: nil,
			Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: abnormal,
					Message:  message,
				},
			},
		}, nil
	}

	//Fetch export clients list
	exports, err := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone)
	if err != nil {
		return &csi.ControllerGetVolumeResponse{
			Volume: &csi.Volume{
				VolumeId: volume.Name,
			},
			Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
				PublishedNodeIds: nil,
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: true,
					Message:  fmt.Sprintf("unable to fetch export list"),
				},
			},
		}, nil
	}

	//remove localhost from the clients
	exportList := removeString(*exports.Clients, "localhost")
	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			VolumeId: volume.Name,
		},
		Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
			PublishedNodeIds: exportList,
			VolumeCondition: &csi.VolumeCondition{
				Abnormal: abnormal,
				Message:  "Volume is healthy",
			},
		},
	}, nil
}

func removeString(exportList []string, strToRemove string) []string {
	for index, export := range exportList {
		if export == strToRemove {
			return append(exportList[:index], exportList[index+1:]...)
		}
	}
	return exportList
}
