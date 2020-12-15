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
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/csi-isilon/common/constants"
	"github.com/dell/csi-isilon/common/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io/ioutil"
	"os"
	"time"
)

func (s *service) NodeExpandVolume(
	context.Context,
	*csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {

	return nil, status.Error(codes.Unimplemented, "")
}

func (s *service) NodeStageVolume(
	ctx context.Context,
	req *csi.NodeStageVolumeRequest) (
	*csi.NodeStageVolumeResponse, error) {

	// TODO - Need to have logic for staging path of export
	s.logStatistics()

	return &csi.NodeStageVolumeResponse{}, nil
}

func (s *service) NodeUnstageVolume(
	ctx context.Context,
	req *csi.NodeUnstageVolumeRequest) (
	*csi.NodeUnstageVolumeResponse, error) {

	// TODO - Need to have logic for staging path of export
	s.logStatistics()

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (s *service) NodePublishVolume(
	ctx context.Context,
	req *csi.NodePublishVolumeRequest) (
	*csi.NodePublishVolumeResponse, error) {
	// Probe the node if required and make sure startup called
	if err := s.autoProbe(ctx); err != nil {
		log.Error("nodeProbe failed with error :" + err.Error())
		return nil, err
	}

	volumeContext := req.GetVolumeContext()
	if volumeContext == nil {
		return nil, status.Error(codes.InvalidArgument, "VolumeContext is nil, skip NodePublishVolume")
	}

	utils.LogMap("VolumeContext", volumeContext)

	isEphemeralVolume := volumeContext["csi.storage.k8s.io/ephemeral"] == "true"
	if isEphemeralVolume {
		return s.ephemeralNodePublish(ctx, req)
	}
	path := volumeContext["Path"]

	if path == "" {
		return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf("no entry keyed by 'Path' found in VolumeContext of volume id : '%s', name '%s', skip NodePublishVolume", req.GetVolumeId(), volumeContext["name"]))
	}
	volName := volumeContext["Name"]
	if volName == "" {
		return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf("no entry keyed by 'Name' found in VolumeContext of volume id : '%s', name '%s', skip NodePublishVolume", req.GetVolumeId(), volumeContext["name"]))
	}

	isROVolumeFromSnapshot := s.isiSvc.isROVolumeFromSnapshot(path)
	if isROVolumeFromSnapshot {
		log.Info("Volume source is snapshot")
		if export, err := s.isiSvc.GetExportWithPathAndZone(path, ""); err != nil || export == nil {
			return nil, status.Errorf(codes.Internal, "error retrieving export for '%s'", path)
		}
	} else {
		// Parse the target path and empty volume name to get the volume
		isiPath := utils.GetIsiPathFromExportPath(path)
		if _, err := s.getVolByName(isiPath, volName); err != nil {
			log.Errorf("Error in getting '%s' Volume '%v'", volName, err)
			return nil, err
		}
	}

	// When custom topology is enabled it takes precedence over the current default behavior
	// Set azServiceIP to updated endpoint when custom topology is enabled
	var azServiceIP string
	if s.opts.CustomTopologyEnabled {
		azServiceIP = s.opts.Endpoint
	} else {
		azServiceIP = volumeContext[AzServiceIPParam]
	}

	f := log.Fields{
		"ID":          req.VolumeId,
		"Name":        volumeContext["Name"],
		"TargetPath":  req.GetTargetPath(),
		"AzServiceIP": azServiceIP,
	}
	log.WithFields(f).Info("Calling publishVolume")
	if err := publishVolume(req, s.isiSvc.GetNFSExportURLForPath(azServiceIP, path), s.opts.NfsV3); err != nil {
		return nil, err
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *service) NodeUnpublishVolume(
	ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest) (
	*csi.NodeUnpublishVolumeResponse, error) {

	log.Debug("executing NodeUnpublishVolume")
	volID := req.GetVolumeId()
	if volID == "" {
		return nil, status.Error(codes.FailedPrecondition, "no VolumeID found in request")
	}
	log.Infof("The volume ID fetched from NodeUnPublish req is %s", volID)

	volName, exportID, accessZone, _ := utils.ParseNormalizedVolumeID(req.GetVolumeId())
	if volName == "" {
		volName = volID
	}

	ephemeralVolName := fmt.Sprintf("ephemeral-%s", volID)
	filePath := req.TargetPath + "/" + ephemeralVolName
	var isEphemeralVolume bool
	var data []byte
	lockFile := filePath + "/id"

	if _, err := os.Stat(lockFile); err == nil {
		isEphemeralVolume = true
		data, err = ioutil.ReadFile(lockFile)
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
		export, err := s.isiSvc.GetExportByIDWithZone(exportID, accessZone)
		if err != nil {
			return nil, err
		}
		exportPath := (*export.Paths)[0]
		isROVolumeFromSnapshot := s.isiSvc.isROVolumeFromSnapshot(exportPath)
		// If it is a RO volume from snapshot
		if isROVolumeFromSnapshot {
			volName = exportPath
		}
	}

	if err := unpublishVolume(req, volName); err != nil {
		return nil, err
	}

	if isEphemeralVolume {
		req.VolumeId = string(data)
		err := s.ephemeralNodeUnpublish(ctx, req)
		if err != nil {
			return nil, err
		}
	}
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *service) nodeProbe(ctx context.Context) error {

	if err := s.validateOptsParameters(); err != nil {
		return fmt.Errorf("node probe failed : '%v'", err)
	}

	if s.isiSvc == nil {

		return errors.New("s.isiSvc (type isiService) is nil, probe failed")

	}

	if err := s.isiSvc.TestConnection(); err != nil {
		return fmt.Errorf("node probe failed : '%v'", err)
	}

	log.Debug("node probe succeeded")

	return nil
}

func (s *service) NodeGetCapabilities(
	ctx context.Context,
	req *csi.NodeGetCapabilitiesRequest) (
	*csi.NodeGetCapabilitiesResponse, error) {

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
			/*{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
					},
				},
			},{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},*/
		},
	}, nil
}

// NodeGetInfo RPC call returns NodeId and AccessibleTopology as part of NodeGetInfoResponse
func (s *service) NodeGetInfo(
	ctx context.Context,
	req *csi.NodeGetInfoRequest) (
	*csi.NodeGetInfoResponse, error) {

	nodeID, err := s.getPowerScaleNodeID(ctx)
	if (err) != nil {
		return nil, err
	}

	if s.opts.CustomTopologyEnabled {
		return &csi.NodeGetInfoResponse{NodeId: nodeID}, nil
	}

	// As NodeGetInfo is invoked only once during driver registration, we validate
	// connectivity with backend PowerScale Array upto MaxIsiConnRetries, before adding topology keys
	var connErr error
	for i := 0; i < constants.MaxIsiConnRetries; i++ {
		connErr = s.isiSvc.TestConnection()
		if connErr == nil {
			break
		}
		time.Sleep(RetrySleepTime)
	}

	if connErr != nil {
		return &csi.NodeGetInfoResponse{NodeId: nodeID}, nil
	}

	// Create the topology keys
	// <provisionerName>.dellemc.com/<powerscaleIP>: <provisionerName>
	topology := map[string]string{}
	topology[constants.PluginName+"/"+s.opts.Endpoint] = constants.PluginName

	// Create NodeGetInfoResponse including nodeID and AccessibleTopology information
	return &csi.NodeGetInfoResponse{
		NodeId: nodeID,
		AccessibleTopology: &csi.Topology{
			Segments: topology,
		},
	}, nil
}

func (s *service) NodeGetVolumeStats(
	ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (s *service) ephemeralNodePublish(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	log.Info("Received request to node publish Ephemeral Volume..")

	volID := req.GetVolumeId()
	volName := fmt.Sprintf("ephemeral-%s", volID)
	createEphemeralVolResp, err := s.CreateVolume(ctx, &csi.CreateVolumeRequest{
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

	controllerPublishEphemeralVolResp, err := s.ControllerPublishVolume(ctx, &csi.ControllerPublishVolumeRequest{
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

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Infof("path %s does not exists", filePath)
		err = os.MkdirAll(filePath, 0750)
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

	f, err := os.Create(filePath + "/id")
	if err != nil {
		log.Error("Create id file in target path for ephemeral vol failed with error :" + err.Error())
		if rollbackError := s.ephemeralNodeUnpublish(ctx, nodeUnpublishRequest); rollbackError != nil {
			log.Error("Rollback failed with error :" + err.Error())
			return nil, err
		}
		return nil, err
	}
	log.Infof("Created file in target path %s", filePath+"/id")

	defer f.Close()
	_, err2 := f.WriteString(createEphemeralVolResp.Volume.VolumeId)
	if err2 != nil {
		log.Error("Writing to id file in target path for ephemeral vol failed with error :" + err.Error())
		if rollbackError := s.ephemeralNodeUnpublish(ctx, nodeUnpublishRequest); rollbackError != nil {
			log.Error("Rollback failed with error :" + err.Error())
			return nil, err
		}
		return nil, err
	}
	log.Infof("Ephemeral Node Publish was successful...")

	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *service) ephemeralNodeUnpublish(
	ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest) error {
	log.Infof("Request received for Ephemeral NodeUnpublish..")
	volumeID := req.GetVolumeId()
	log.Infof("The volID is %s", volumeID)
	if volumeID == "" {
		return status.Error(codes.InvalidArgument, "volume ID is required")
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
	volName, _, _, err := utils.ParseNormalizedVolumeID(req.GetVolumeId())
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

	nodeIP, err := s.GetCSINodeIP(ctx)
	if (err) != nil {
		return "", err
	}

	nodeFQDN, err := utils.GetFQDNByIP(nodeIP)
	if (err) != nil {
		return "", err
	}

	nodeID, err := s.GetCSINodeID(ctx)
	if (err) != nil {
		return "", err
	}

	nodeID = nodeID + utils.NodeIDSeparator + nodeFQDN + utils.NodeIDSeparator + nodeIP

	return nodeID, nil
}
