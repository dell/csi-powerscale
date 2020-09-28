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
	"github.com/dell/csi-isilon/common/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	path := volumeContext["Path"]
	if path == "" {
		return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf("no entry keyed by 'Path' found in VolumeContext of volume id : '%s', name '%s', skip NodePublishVolume", req.GetVolumeId(), volumeContext["name"]))
	}
	volName := volumeContext["Name"]
	if volName == "" {
		return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf("no entry keyed by 'Name' found in VolumeContext of volume id : '%s', name '%s', skip NodePublishVolume", req.GetVolumeId(), volumeContext["name"]))
	}

	// Parse the target path and empty volume name to get the volume
	isiPath := utils.GetIsiPathFromExportPath(path)
	if _, err := s.getVolByName(isiPath, volName); err != nil {
		log.Errorf("Error in getting '%s' Volume '%v'", volName, err)
		return nil, err
	}
	azServiceIP := volumeContext[AzServiceIPParam]
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

	volName, _, _, _ := utils.ParseNormalizedVolumeID(req.GetVolumeId())
	if volName == "" {
		volName = volID
	}
	if err := unpublishVolume(req, volName); err != nil {
		return nil, err
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

// Minimal version of NodeGetInfo. Returns NodeId
// MaxVolumesPerNode (optional) is left as 0 which means unlimited, and AccessibleTopology is left nil.
func (s *service) NodeGetInfo(
	ctx context.Context,
	req *csi.NodeGetInfoRequest) (
	*csi.NodeGetInfoResponse, error) {

	nodeID, err := s.GetCSINodeID(ctx)
	if (err) != nil {
		return nil, err
	}

	return &csi.NodeGetInfoResponse{NodeId: nodeID}, nil
}

func (s *service) NodeGetVolumeStats(
	ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
