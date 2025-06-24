package service

/*
 Copyright (c) 2019-2025 Dell Inc, or its subsidiaries.

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
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/dell/csi-isilon/v2/common/k8sutils"
	"github.com/dell/csi-isilon/v2/common/utils"
	csiutils "github.com/dell/csi-isilon/v2/csi-utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	getIsVolumeExistentFunc = func(isiConfig *IsilonClusterConfig) func(context.Context, string, string, string) bool {
		return isiConfig.isiSvc.IsVolumeExistent
	}
	getIsVolumeMounted  = isVolumeMounted
	getOsReadDir        = os.ReadDir
	getCreateVolumeFunc = func(s *service) func(context.Context, *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
		return s.CreateVolume
	}
	getControllerPublishVolume = func(s *service) func(context.Context, *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
		return s.ControllerPublishVolume
	}
	getUtilsGetFQDNByIP = utils.GetFQDNByIP
	getK8sutilsGetStats = k8sutils.GetStats
)

func (s *service) NodeExpandVolume(
	context.Context,
	*csi.NodeExpandVolumeRequest,
) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (s *service) NodeStageVolume(
	_ context.Context,
	_ *csi.NodeStageVolumeRequest) (
	*csi.NodeStageVolumeResponse, error,
) {
	// TODO - Need to have logic for staging path of export
	s.logStatistics()

	return &csi.NodeStageVolumeResponse{}, nil
}

func (s *service) NodeUnstageVolume(
	_ context.Context,
	_ *csi.NodeUnstageVolumeRequest) (
	*csi.NodeUnstageVolumeResponse, error,
) {
	// TODO - Need to have logic for staging path of export
	s.logStatistics()

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (s *service) NodePublishVolume(
	ctx context.Context,
	req *csi.NodePublishVolumeRequest) (
	*csi.NodePublishVolumeResponse, error,
) {
	// Fetch log handler
	ctx, log, runID := GetRunIDLog(ctx)
	// set noProbeOnStart to false so subsequent calls can lead to probe
	noProbeOnStart = false

	volumeContext := req.GetVolumeContext()
	if volumeContext == nil {
		return nil, status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, "VolumeContext is nil, skip NodePublishVolume"))
	}
	utils.LogMap(ctx, "VolumeContext", volumeContext)

	isEphemeralVolume := volumeContext["csi.storage.k8s.io/ephemeral"] == "true"
	var clusterName string
	var err error
	if isEphemeralVolume {
		clusterName = volumeContext["ClusterName"]
	} else {
		// parse the input volume id and fetch it's components
		_, _, _, clusterName, _ = utils.ParseNormalizedVolumeID(ctx, req.GetVolumeId())
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		return nil, err
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	// Probe the node if required and make sure startup called
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		log.Error("nodeProbe failed with error :" + err.Error())
		return nil, err
	}

	if isEphemeralVolume {
		return s.ephemeralNodePublish(ctx, req)
	}
	path := volumeContext["Path"]

	if path == "" {
		return nil, status.Error(codes.FailedPrecondition, utils.GetMessageWithRunID(runID, "no entry keyed by 'Path' found in VolumeContext of volume id : '%s', name '%s', skip NodePublishVolume", req.GetVolumeId(), volumeContext["name"]))
	}
	volName := volumeContext["Name"]
	if volName == "" {
		return nil, status.Error(codes.FailedPrecondition, utils.GetMessageWithRunID(runID, "no entry keyed by 'Name' found in VolumeContext of volume id : '%s', name '%s', skip NodePublishVolume", req.GetVolumeId(), volumeContext["name"]))
	}
	accessZone := volumeContext["AccessZone"]
	isROVolumeFromSnapshot := isiConfig.isiSvc.isROVolumeFromSnapshot(path, accessZone)
	if isROVolumeFromSnapshot {
		log.Info("Volume source is snapshot")
		if export, err := isiConfig.isiSvc.GetExportWithPathAndZone(ctx, path, accessZone); err != nil || export == nil {
			return nil, status.Error(codes.Internal, utils.GetMessageWithRunID(runID, "error retrieving export for %s", path))
		}
	} else {
		// Parse the target path and empty volume name to get the volume
		isiPath := utils.GetIsiPathFromExportPath(path)

		if _, err := s.getVolByName(ctx, isiPath, volName, isiConfig); err != nil {
			log.Errorf("Error in getting '%s' Volume '%v'", volName, err)
			return nil, err
		}
	}

	// When custom topology is enabled it takes precedence over the current default behavior
	// Set azServiceIP to updated endpoint when custom topology is enabled
	var azServiceIP string
	if s.opts.CustomTopologyEnabled {
		azServiceIP = isiConfig.Endpoint
	} else {
		azServiceIP = volumeContext[AzServiceIPParam]
	}

	if strings.Contains(azServiceIP, "localhost") {
		azServiceIP = isiConfig.MountEndpoint
	}

	f := map[string]interface{}{
		"ID":          req.VolumeId,
		"Name":        volumeContext["Name"],
		"TargetPath":  req.GetTargetPath(),
		"AzServiceIP": azServiceIP,
	}
	// TODO: Replace logrus with log
	logrus.WithFields(f).Info("Calling publishVolume")
	if err := publishVolume(ctx, req, isiConfig.isiSvc.GetNFSExportURLForPath(azServiceIP, path)); err != nil {
		return nil, err
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *service) NodeUnpublishVolume(
	ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest) (
	*csi.NodeUnpublishVolumeResponse, error,
) {
	// Fetch log handler
	ctx, log, runID := GetRunIDLog(ctx)

	log.Debug("executing NodeUnpublishVolume")
	// set noProbeOnStart to false so subsequent calls can lead to probe
	noProbeOnStart = false
	volID := req.GetVolumeId()
	if volID == "" {
		return nil, status.Error(codes.FailedPrecondition, utils.GetMessageWithRunID(runID, "no VolumeID found in request"))
	}
	log.Infof("The volume ID fetched from NodeUnPublish req is %s", volID)

	volName, exportID, accessZone, clusterName, _ := utils.ParseNormalizedVolumeID(ctx, req.GetVolumeId())
	if volName == "" {
		volName = volID
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	// Probe the node if required
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		log.Error("nodeProbe failed with error :" + err.Error())
		return nil, err
	}

	ephemeralVolName := fmt.Sprintf("ephemeral-%s", volID)
	filePath := req.TargetPath + "/" + ephemeralVolName
	var isEphemeralVolume bool
	var data []byte
	lockFile := filePath + "/id"

	if _, err := os.Stat(lockFile); err == nil {
		isEphemeralVolume = true
		data, err = readFileFunc(filepath.Clean(lockFile))
		if err != nil {
			return nil, errors.New("unable to get volume id for ephemeral volume")
		}
	}

	var isExportIDEmpty bool
	if exportID == 0 && accessZone == "" {
		isExportIDEmpty = true
	}

	log.Infof("Ephemeral volume check: %t", isEphemeralVolume)

	// Check if it is a RO volume from snapshot
	// We need not execute this logic for ephemeral volumes.
	if !isExportIDEmpty {
		export, err := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone)
		if err != nil {
			// Export doesn't exist - this is OK during unpublish
			// Log it but don't fail the operation
			log.Infof("Export ID %d not found during unpublish (may already be cleaned up): %v", exportID, err)
			// Continue with unpublish using just the volume name
		} else if export != nil && export.Paths != nil && len(*export.Paths) > 0 {
			exportPath := (*export.Paths)[0]
			isROVolumeFromSnapshot := isiConfig.isiSvc.isROVolumeFromSnapshot(exportPath, accessZone)
			// If it is a RO volume from snapshot
			if isROVolumeFromSnapshot {
				volName = exportPath
			}
		}
	}

	if err := unpublishVolume(ctx, req, volName); err != nil {
		log.Error("Error while calling Unbuplish Volume", err.Error())
		return nil, err
	}

	if isEphemeralVolume {
		req.VolumeId = string(data)
		err := s.ephemeralNodeUnpublish(ctx, req)
		if err != nil {
			log.Error("Error while calling Ephemeral Node Unpublish", err.Error())
			return nil, err
		}
	}
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *service) nodeProbe(ctx context.Context, isiConfig *IsilonClusterConfig) error {
	// Fetch log handler
	ctx, log, _ := GetRunIDLog(ctx)

	if err := s.validateOptsParameters(isiConfig); err != nil {
		return fmt.Errorf("node probe failed : '%v'", err)
	}

	if isiConfig.isiSvc == nil {
		logLevel := utils.GetCurrentLogLevel()
		var err error
		isiConfig.isiSvc, err = s.GetIsiService(ctx, isiConfig, logLevel)
		if isiConfig.isiSvc == nil {
			return errors.New("clusterConfig.isiSvc (type isiService) is nil, probe failed")
		}
		if err != nil {
			return err
		}
	}

	if err := isiConfig.isiSvc.TestConnection(ctx); err != nil {
		return fmt.Errorf("node probe failed : '%v'", err)
	}

	ctx, log = setClusterContext(ctx, isiConfig.ClusterName)
	log.Debug("node probe succeeded")

	return nil
}

func (s *service) NodeGetCapabilities(
	_ context.Context,
	_ *csi.NodeGetCapabilitiesRequest) (
	*csi.NodeGetCapabilitiesResponse, error,
) {
	capabilities := []*csi.NodeServiceCapability{
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
				},
			},
		},
	}

	healthMonitorCapabilities := []*csi.NodeServiceCapability{
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
				},
			},
		}, {
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_VOLUME_CONDITION,
				},
			},
		},
	}

	if s.opts.IsHealthMonitorEnabled {
		capabilities = append(capabilities, healthMonitorCapabilities...)
	}

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: capabilities,
	}, nil
}

// NodeGetInfo RPC call returns NodeId and AccessibleTopology as part of NodeGetInfoResponse
func (s *service) NodeGetInfo(
	ctx context.Context,
	_ *csi.NodeGetInfoRequest) (
	*csi.NodeGetInfoResponse, error,
) {
	// Fetch log handler
	ctx, log, _ := GetRunIDLog(ctx)

	nodeID, err := s.getPowerScaleNodeID(ctx)
	log.Infof("Node ID of worker node is '%s'", nodeID)
	if (err) != nil {
		log.Error("Failed to create Node ID with error", err.Error())
		return nil, err
	}
	if noProbeOnStart {
		log.Debugf("noProbeOnStart is set to true, skip probe")
		return &csi.NodeGetInfoResponse{NodeId: nodeID}, nil
	}
	// If Custom Topology is enabled we do not add node labels to the worker node
	if s.opts.CustomTopologyEnabled {
		return &csi.NodeGetInfoResponse{NodeId: nodeID}, nil
	}

	// If Custom Topology is not enabled, proceed with adding node labels for all
	// PowerScale clusters part of secret.yaml
	isiClusters := s.getIsilonClusters()
	topology := make(map[string]string)

	for cluster := range isiClusters {
		// Validate if we have valid clusterConfig
		if isiClusters[cluster].isiSvc == nil {
			continue
		}

		// As NodeGetInfo is invoked only once during driver registration, we validate
		// connectivity with backend PowerScale Array upto MaxIsiConnRetries, before adding topology keys
		var connErr error
		for i := 0; i < constants.MaxIsiConnRetries; i++ {
			connErr = isiClusters[cluster].isiSvc.TestConnection(ctx)
			if connErr == nil {
				break
			}
			time.Sleep(RetrySleepTime)
		}

		if connErr != nil {
			continue
		}

		// Create the topology keys
		// <provisionerName>.dellemc.com/<powerscaleIP>: <provisionerName>
		topology[constants.PluginName+"/"+isiClusters[cluster].Endpoint] = constants.PluginName
	}

	// Check for node label 'max-isilon-volumes-per-node'. If present set 'MaxVolumesPerNode' to this value.
	// If node label is not present, set 'MaxVolumesPerNode' to default value i.e., 0
	var maxIsilonVolumesPerNode int64
	labels, err := s.GetNodeLabels()
	if err != nil {
		log.Error("failed to get Node Labels with error", err.Error())
		return nil, err
	}

	if val, ok := labels["max-isilon-volumes-per-node"]; ok {
		maxIsilonVolumesPerNode, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value '%s' specified for 'max-isilon-volumes-per-node' node label", val)
		}
		log.Infof("node label 'max-isilon-volumes-per-node' is available and is set to value '%v'", maxIsilonVolumesPerNode)
	} else {
		// As per the csi spec the plugin MUST NOT set negative values to
		// 'MaxVolumesPerNode' in the NodeGetInfoResponse response
		if s.opts.MaxVolumesPerNode < 0 {
			return nil, fmt.Errorf("maxIsilonVolumesPerNode MUST NOT be set to negative value")
		}
		maxIsilonVolumesPerNode = s.opts.MaxVolumesPerNode
		log.Infof("node label 'max-isilon-volumes-per-node' is not available. Using default volume limit '%v'", maxIsilonVolumesPerNode)
	}

	// Create NodeGetInfoResponse including nodeID and AccessibleTopology information
	return &csi.NodeGetInfoResponse{
		NodeId: nodeID,
		AccessibleTopology: &csi.Topology{
			Segments: topology,
		},
		MaxVolumesPerNode: maxIsilonVolumesPerNode,
	}, nil
}

func (s *service) NodeGetVolumeStats(
	ctx context.Context, req *csi.NodeGetVolumeStatsRequest,
) (*csi.NodeGetVolumeStatsResponse, error) {
	// Fetch log handler
	ctx, log, runID := GetRunIDLog(ctx)
	volID := req.GetVolumeId()
	if volID == "" {
		return nil, status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, "no VolumeID found in request"))
	}
	volPath := req.GetVolumePath()
	if volPath == "" {
		return nil, status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, "no Volume Path found in request"))
	}

	volName, _, _, clusterName, _ := utils.ParseNormalizedVolumeID(ctx, volID)
	if volName == "" {
		volName = volID
	}

	// Check if given volume exists
	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		return nil, err
	}

	isiPath := isiConfig.IsiPath

	isiPathFromParams, err := s.validateIsiPath(ctx, volName)
	if err != nil {
		log.Error("Failed get isiPath", err.Error())
	}

	if isiPathFromParams != isiPath && isiPathFromParams != "" {
		log.Debug("overriding isiPath with value from StorageClass", isiPathFromParams)
		isiPath = isiPathFromParams
	}

	// Probe the node if required and make sure startup called
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		log.Error("nodeProbe failed with error :" + err.Error())
		return nil, err
	}

	isVolumeExistentFunc := getIsVolumeExistentFunc(isiConfig)
	isVolumeExistent := isVolumeExistentFunc(ctx, volName, volPath, "")

	if !isVolumeExistent {
		return nil, status.Error(codes.NotFound, utils.GetMessageWithRunID(runID, "volume %v does not exist at path %v", volName, volPath))
	}

	// check whether the original volume is mounted
	isMounted, err := getIsVolumeMounted(ctx, volName, volPath)
	if !isMounted {
		return nil, status.Error(codes.NotFound, utils.GetMessageWithRunID(runID, "no volume is mounted at path: %s", volPath))
	}

	// check whether volume path is accessible
	_, err = getOsReadDir(volPath)
	if err != nil {
		return nil, status.Error(codes.NotFound, utils.GetMessageWithRunID(runID, "volume path is not accessible: %s", err))
	}

	// Get Volume stats metrics
	availableBytes, totalBytes, usedBytes, totalInodes, freeInodes, usedInodes, err := getK8sutilsGetStats(ctx, volPath)
	if err != nil {
		return &csi.NodeGetVolumeStatsResponse{
			Usage: []*csi.VolumeUsage{
				{
					Unit:      csi.VolumeUsage_UNKNOWN,
					Available: availableBytes,
					Total:     totalBytes,
					Used:      usedBytes,
				},
			},
			VolumeCondition: &csi.VolumeCondition{
				Abnormal: true,
				Message:  fmt.Sprintf("failed to get volume stats metrics: %s", err),
			},
		}, nil
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Unit:      csi.VolumeUsage_BYTES,
				Available: availableBytes,
				Total:     totalBytes,
				Used:      usedBytes,
			},
			{
				Unit:      csi.VolumeUsage_INODES,
				Available: freeInodes,
				Total:     totalInodes,
				Used:      usedInodes,
			},
		},
		VolumeCondition: &csi.VolumeCondition{
			Abnormal: false,
			Message:  "",
		},
	}, nil
}

func (s *service) ephemeralNodePublish(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// Fetch log handler
	ctx, log, _ := GetRunIDLog(ctx)

	log.Info("Received request to node publish Ephemeral Volume..")

	volID := req.GetVolumeId()
	volName := fmt.Sprintf("ephemeral-%s", volID)
	createVolumeFunc := getCreateVolumeFunc(s)
	createEphemeralVolResp, err := createVolumeFunc(ctx, &csi.CreateVolumeRequest{
		Name:               volName,
		VolumeCapabilities: []*csi.VolumeCapability{req.VolumeCapability},
		Parameters:         req.VolumeContext,
		Secrets:            req.Secrets,
	})
	if err != nil {
		log.Error("Create ephemeral volume failed with error :" + err.Error())
		return nil, err
	}
	filePath := req.TargetPath + "/" + volName
	log.Infof("Ephemeral Volume %s creation was successful %s", volID, createEphemeralVolResp)

	// Build nodeUnPublish object for rollbacks
	nodeUnpublishRequest := &csi.NodeUnpublishVolumeRequest{
		VolumeId:   createEphemeralVolResp.Volume.VolumeId,
		TargetPath: req.TargetPath,
	}

	nodeID, err := s.getPowerScaleNodeID(ctx)
	if (err) != nil {
		return nil, err
	}

	controllerPublishEphemeralVolResp, err := getControllerPublishVolume(s)(ctx, &csi.ControllerPublishVolumeRequest{
		VolumeId:         createEphemeralVolResp.Volume.VolumeId,
		NodeId:           nodeID,
		VolumeCapability: req.VolumeCapability,
		Readonly:         req.Readonly,
		Secrets:          req.Secrets,
		VolumeContext:    createEphemeralVolResp.Volume.VolumeContext,
	})
	if err != nil {
		log.Error("Need to rollback because ControllerPublish ephemeral volume failed with error :" + err.Error())
		if rollbackError := s.ephemeralNodeUnpublish(ctx, nodeUnpublishRequest); rollbackError != nil {
			log.Error("Rollback failed with error :" + err.Error())
			return nil, err
		}
		return nil, err
	}
	log.Infof("Ephemeral ControllerPublish for volume %s was successful %v", volID, controllerPublishEphemeralVolResp)

	delete(createEphemeralVolResp.Volume.VolumeContext, "csi.storage.k8s.io/ephemeral")
	_, err = s.NodePublishVolume(ctx, &csi.NodePublishVolumeRequest{
		VolumeId:         createEphemeralVolResp.Volume.VolumeId,
		PublishContext:   controllerPublishEphemeralVolResp.PublishContext,
		TargetPath:       req.TargetPath,
		VolumeCapability: req.VolumeCapability,
		Readonly:         req.Readonly,
		Secrets:          req.Secrets,
		VolumeContext:    createEphemeralVolResp.Volume.VolumeContext,
	})
	if err != nil {
		log.Error("Need to rollback because NodePublish ephemeral volume failed with error :" + err.Error())
		if rollbackError := s.ephemeralNodeUnpublish(ctx, nodeUnpublishRequest); rollbackError != nil {
			log.Error("Rollback failed with error :" + err.Error())
			return nil, err
		}
		return nil, err
	}
	log.Infof("NodePublish step for volume %s was successful", volID)

	if _, err := statFileFunc(filePath); os.IsNotExist(err) {
		log.Infof("path %s does not exists", filePath)
		err = mkDirAllFunc(filePath, 0o750)
		if err != nil {
			log.Error("Create directory in target path for ephemeral vol failed with error :" + err.Error())
			if rollbackError := s.ephemeralNodeUnpublish(ctx, nodeUnpublishRequest); rollbackError != nil {
				log.Error("Rollback failed with error :" + err.Error())
				return nil, err
			}
			return nil, err
		}
	}
	log.Infof("Created dir in target path %s", filePath)

	f, err := createFileFunc(filepath.Clean(filePath) + "/id")
	if err != nil {
		log.Error("Create id file in target path for ephemeral vol failed with error :" + err.Error())
		if rollbackError := s.ephemeralNodeUnpublish(ctx, nodeUnpublishRequest); rollbackError != nil {
			log.Error("Rollback failed with error :" + err.Error())
			return nil, err
		}
		return nil, err
	}
	log.Infof("Created file in target path %s", filePath+"/id")

	defer func() {
		if err := closeFileFunc(f); err != nil {
			log.Errorf("Error closing file: %s \n", err)
		}
	}()
	_, err2 := writeStringFunc(f, createEphemeralVolResp.Volume.VolumeId)
	if err2 != nil {
		log.Error("Writing to id file in target path for ephemeral vol failed with error :" + err2.Error())
		if rollbackError := s.ephemeralNodeUnpublish(ctx, nodeUnpublishRequest); rollbackError != nil {
			log.Error("Rollback failed with error :" + rollbackError.Error())
		}
		return nil, err2
	}
	log.Infof("Ephemeral Node Publish was successful...")

	return &csi.NodePublishVolumeResponse{}, nil
}

var mkDirAllFunc = func(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

var createFileFunc = func(path string) (*os.File, error) {
	cleanedPath := filepath.Clean(path)
	return os.Create(cleanedPath)
}

var writeStringFunc = func(f *os.File, output string) (int, error) {
	return f.WriteString(output)
}

var closeFileFunc = func(f *os.File) error {
	return f.Close()
}

var statFileFunc = func(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

var readFileFunc = func(path string) ([]byte, error) {
	cleanedPath := filepath.Clean(path)
	return os.ReadFile(cleanedPath)
}

func (s *service) ephemeralNodeUnpublish(
	ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest,
) error {
	return ephemeralNodeUnpublishFunc(s, ctx, req)
}

var ephemeralNodeUnpublishFunc = func(s *service, ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest,
) error {
	// Fetch log handler
	ctx, log, runID := GetRunIDLog(ctx)

	log.Infof("Request received for Ephemeral NodeUnpublish..")
	volumeID := req.GetVolumeId()
	log.Infof("The volID is %s", volumeID)
	if volumeID == "" {
		return status.Error(codes.InvalidArgument, utils.GetMessageWithRunID(runID, "volume ID is required"))
	}

	nodeID, nodeIDErr := s.getPowerScaleNodeID(ctx)
	if (nodeIDErr) != nil {
		return nodeIDErr
	}

	_, err := s.ControllerUnpublishVolume(ctx, &csi.ControllerUnpublishVolumeRequest{
		VolumeId: volumeID,
		NodeId:   nodeID,
	})
	if err != nil {
		log.Error("ControllerUnPublish ephemeral volume failed with error :" + err.Error())
		return err
	}
	log.Infof("Controller UnPublish for Ephemeral inline volume %s sucessful..", volumeID)

	// Before deleting the volume on PowerScale,
	// Cleaning up the directories we created.
	volName, _, _, _, err := utils.ParseNormalizedVolumeID(ctx, req.GetVolumeId())
	if err != nil {
		return err
	}
	tmpPath := req.TargetPath + "/" + volName
	log.Infof("Going to clean up the temporary directory on path %s", tmpPath)
	err = os.RemoveAll(tmpPath)
	if err != nil {
		return errors.New("failed to cleanup lock files")
	}

	_, err = s.DeleteVolume(ctx, &csi.DeleteVolumeRequest{
		VolumeId: volumeID,
	})
	if err != nil {
		log.Error("Delete ephemeral volume failed with error :" + err.Error())
		return err
	}
	log.Infof("Delete volume for Ephemeral inline volume %s successful..", volumeID)

	return nil
}

func (s *service) getPowerScaleNodeID(ctx context.Context) (string, error) {
	var nodeIP string
	var err error

	// Fetch log handler
	ctx, log, _ := GetRunIDLog(ctx)

	// When valid list of allowedNetworks is being given as part of values.yaml, we need
	// to fetch first IP from matching network
	if len(s.opts.allowedNetworks) > 0 {
		log.Debugf("Fetching IP address of custom network for NFS I/O traffic")
		nodeIP, err = csiutils.GetNFSClientIP(s.opts.allowedNetworks)
		if err != nil {
			log.Error("Failed to find IP address corresponding to the allowed network with error", err.Error())
			return "", err
		}
	} else {
		nodeIP, err = s.GetCSINodeIP(ctx)
		if (err) != nil {
			return "", err
		}
	}

	nodeFQDN, err := getUtilsGetFQDNByIP(ctx, nodeIP)
	if (err) != nil {
		nodeFQDN = nodeIP
		log.Warnf("Setting nodeFQDN to %s as failed to resolve IP to FQDN due to %v", nodeIP, err)
	}

	nodeID, err := s.GetCSINodeID(ctx)
	if (err) != nil {
		return "", err
	}

	nodeID = nodeID + utils.NodeIDSeparator + nodeFQDN + utils.NodeIDSeparator + nodeIP

	return nodeID, nil
}
