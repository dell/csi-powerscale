package service

import (
	"context"
	"fmt"
	"github.com/dell/csi-isilon/common/utils"
	csiext "github.com/dell/dell-csi-extensions/replication"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

func (s *service) CreateRemoteVolume(ctx context.Context,
	req *csiext.CreateRemoteVolumeRequest) (*csiext.CreateRemoteVolumeResponse, error) {
	ctx, log, _ := GetRunIDLog(ctx)

	volID := req.GetVolumeHandle()
	if volID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	volName, exportID, accessZone, clusterName, err := utils.ParseNormalizedVolumeID(ctx, volID)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	log.Info("volume name", volName)
	log.Info("export ID", exportID)

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	remoteClusterName, ok := req.Parameters[s.WithRP(KeyReplicationRemoteSystem)]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "replication enabled but no remote system specified in storage class")
	}

	remoteIsiConfig, err := s.getIsilonConfig(ctx, &remoteClusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, status.Errorf(codes.InvalidArgument, "can't find cluster with name %s in driver config", remoteClusterName)
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	// auto probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	if err := s.autoProbe(ctx, remoteIsiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	export, err := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	if len(*export.Paths) == 0 {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("can't find paths for export with id %d", exportID))
	}
	exportPath := (*export.Paths)[0]

	isiPath := utils.GetIsiPathFromExportPath(exportPath)

	volumeSize := isiConfig.isiSvc.GetVolumeSize(ctx, isiPath, volName)

	// Check if export exists
	remoteExport, err := remoteIsiConfig.isiSvc.GetExportWithPathAndZone(ctx, exportPath, accessZone)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	var remoteExportID int

	// If export does not exist we need to create it
	if remoteExport == nil {
		// Check if quota already exists
		var quotaID string
		quota, err := remoteIsiConfig.isiSvc.client.GetQuotaWithPath(ctx, exportPath)
		if quota == nil {
			quotaID, err = remoteIsiConfig.isiSvc.CreateQuota(ctx, exportPath, volName, volumeSize, s.opts.QuotaEnabled)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "can't create volume quota %s", err.Error())
			}
		} else {
			quotaID = quota.Id
		}

		if remoteExportID, err = remoteIsiConfig.isiSvc.ExportVolumeWithZone(ctx, isiPath, volName, accessZone, utils.GetQuotaIDWithCSITag(quotaID)); err == nil && remoteExportID != 0 {
			// get the export and retry if not found to ensure the export has been created
			for i := 0; i < MaxRetries; i++ {
				if export, _ := remoteIsiConfig.isiSvc.GetExportByIDWithZone(ctx, remoteExportID, accessZone); export != nil {
					// Add dummy localhost entry for pvc security
					if !remoteIsiConfig.isiSvc.IsHostAlreadyAdded(ctx, remoteExportID, accessZone, utils.DummyHostNodeID) {
						err = remoteIsiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, remoteExportID, accessZone, utils.DummyHostNodeID, remoteIsiConfig.isiSvc.AddExportClientByIDWithZone)
						if err != nil {
							log.Debugf("Error while adding dummy localhost entry to export '%d'", remoteExportID)
						}
					}
				}
				time.Sleep(RetrySleepTime)
				log.Printf("Begin to retry '%d' time(s), for export id '%d' and path '%s'\n", i+1, remoteExportID, exportPath)
			}
		}
	} else {
		remoteExportID = remoteExport.ID
	}

	remoteVolume := getRemoteCSIVolume(ctx, remoteExportID, volName, accessZone, volumeSize, remoteClusterName)

	// TODO: figure out what remote parameters we would need if any
	// remoteParams := map[string]string{
	// 	"remoteSystem": localSystem.Name,
	// 	s.replicationContextPrefix + "managementAddress": remoteSystem.ManagementAddress,
	// }
	// remoteVolume.VolumeContext = remoteParams

	return &csiext.CreateRemoteVolumeResponse{
		RemoteVolume: remoteVolume,
	}, nil
}

func (s *service) CreateStorageProtectionGroup(ctx context.Context,
	req *csiext.CreateStorageProtectionGroupRequest) (*csiext.CreateStorageProtectionGroupResponse, error) {
	ctx, log, _ := GetRunIDLog(ctx)

	volID := req.GetVolumeHandle()
	if volID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	volName, exportID, accessZone, clusterName, err := utils.ParseNormalizedVolumeID(ctx, volID)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	log.Info("volume name", volName)
	log.Info("export ID", exportID)

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	remoteClusterName, ok := req.Parameters[s.WithRP(KeyReplicationRemoteSystem)]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "replication enabled but no remote system specified in storage class")
	}

	remoteIsiConfig, err := s.getIsilonConfig(ctx, &remoteClusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, status.Errorf(codes.InvalidArgument, "can't find cluster with name %s in driver config", remoteClusterName)
	}

	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	// auto probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	if err := s.autoProbe(ctx, remoteIsiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	export, err := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	if len(*export.Paths) == 0 {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("can't find paths for export with id %d", exportID))
	}

	exportPath := (*export.Paths)[0]

	isiPath := utils.GetIsiPathFromExportPath(exportPath)

	vgName := utils.GetVolumeNameFromExportPath(isiPath)

	localParams := map[string]string{
		s.opts.replicationContextPrefix + "systemName":              clusterName,
		s.opts.replicationContextPrefix + "remoteSystemName":        remoteClusterName,
		s.opts.replicationContextPrefix + "managementAddress":       isiConfig.Endpoint,
		s.opts.replicationContextPrefix + "remoteManagementAddress": remoteIsiConfig.Endpoint,
		s.opts.replicationContextPrefix + "VolumeGroupName":         vgName,
	}
	remoteParams := map[string]string{
		s.opts.replicationContextPrefix + "systemName":              remoteClusterName,
		s.opts.replicationContextPrefix + "remoteSystemName":        clusterName,
		s.opts.replicationContextPrefix + "managementAddress":       remoteIsiConfig.Endpoint,
		s.opts.replicationContextPrefix + "remoteManagementAddress": isiConfig.Endpoint,
		s.opts.replicationContextPrefix + "VolumeGroupName":         vgName,
	}

	return &csiext.CreateStorageProtectionGroupResponse{
		LocalProtectionGroupId:          fmt.Sprintf("%s::%s", clusterName, isiPath),
		RemoteProtectionGroupId:         fmt.Sprintf("%s::%s", remoteClusterName, isiPath),
		LocalProtectionGroupAttributes:  localParams,
		RemoteProtectionGroupAttributes: remoteParams,
	}, nil
}

func (s *service) DeleteStorageProtectionGroup(ctx context.Context, request *csiext.DeleteStorageProtectionGroupRequest) (*csiext.DeleteStorageProtectionGroupResponse, error) {
	ctx, log, _ := GetRunIDLog(ctx)

	log.Info("not implemented yet")

	return &csiext.DeleteStorageProtectionGroupResponse{}, nil
}

func (s *service) ExecuteAction(ctx context.Context, request *csiext.ExecuteActionRequest) (*csiext.ExecuteActionResponse, error) {
	panic("implement me")
}

func (s *service) GetStorageProtectionGroupStatus(ctx context.Context, request *csiext.GetStorageProtectionGroupStatusRequest) (*csiext.GetStorageProtectionGroupStatusResponse, error) {
	panic("implement me")
}

func getRemoteCSIVolume(ctx context.Context, exportID int, volName, accessZone string, sizeInBytes int64, clusterName string) *csiext.Volume {
	volume := &csiext.Volume{
		VolumeId:      utils.GetNormalizedVolumeID(ctx, volName, exportID, accessZone, clusterName),
		CapacityBytes: sizeInBytes,
		VolumeContext: nil, // TODO: add values to volume context if needed
	}
	return volume
}
