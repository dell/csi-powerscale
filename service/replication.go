package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dell/csi-isilon/common/utils"
	csiext "github.com/dell/dell-csi-extensions/replication"
	isi "github.com/dell/goisilon"
	isiApi "github.com/dell/goisilon/api"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// DeleteStorageProtectionGroup deletes storage protection group
func (s *service) DeleteStorageProtectionGroup(ctx context.Context,
	req *csiext.DeleteStorageProtectionGroupRequest) (*csiext.DeleteStorageProtectionGroupResponse, error) {

	ctx, log, _ := GetRunIDLog(ctx)
	localParams := req.GetProtectionGroupAttributes()
	groupID := req.GetProtectionGroupId()
	isiPath := utils.GetIsiPathFromPgID(groupID)
	log.Infof("IsiPath: %s", isiPath)
	clusterName, ok := localParams[s.opts.replicationContextPrefix+"systemName"]
	if !ok {
		log.Error("Can't get systemName from PG params")
		return nil, status.Errorf(codes.InvalidArgument, "Error: Can't get systemName from PG params")
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	fields := map[string]interface{}{
		"ProtectedStorageGroup": groupID,
	}

	log.WithFields(fields).Info("Deleting storage protection group")

	_, err = isiConfig.isiSvc.GetVolume(ctx, isiPath, "", "")
	if e, ok := err.(*isiApi.JSONError); ok {
		if e.StatusCode == 404 {
			return &csiext.DeleteStorageProtectionGroupResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "Error: Unable to get Volume Group")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "Error: Unable to get Volume Group")
	}
	childs, err := isiConfig.isiSvc.client.QueryVolumeChildren(ctx, strings.TrimPrefix(isiPath, isiConfig.IsiPath))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error: Unable to get VG's childs")
	}
	for key := range childs {
		log.Info("Child Path: ", key)
		export, err := isiConfig.isiSvc.GetExportWithPathAndZone(ctx, key, "")
		if err == nil {
			log.Error("Contains paths: ", export.Paths)
			return nil, status.Errorf(codes.Internal, "VG is not empty")
		}
	}
	err = isiConfig.isiSvc.DeleteVolume(ctx, isiPath, "")
	if err != nil {
		return nil, err
	}
	ppName := strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(isiPath, isiConfig.IsiPath), "/", ""), ".", "-")
	log.Info(ppName, "ppname")
	err = isiConfig.isiSvc.client.DeletePolicy(ctx, ppName)
	if err != nil {
		if e, ok := err.(*isiApi.JSONError); ok {
			if e.StatusCode == 404 {
				log.Info("No PP Found")
			} else {
				log.Errorf("Failed to delete PP %s.", ppName)
			}
		} else {
			log.Error("Unknown error while deleting PP")
		}
	}
	log.Info("PP cleared out")
	return &csiext.DeleteStorageProtectionGroupResponse{}, nil
}

func (s *service) ExecuteAction(ctx context.Context, req *csiext.ExecuteActionRequest) (*csiext.ExecuteActionResponse, error) {
	ctx, log, _ := GetRunIDLog(ctx)

	var reqID string
	localParams := req.GetProtectionGroupAttributes()
	protectionGroupID := req.GetProtectionGroupId()
	action := req.GetAction().GetActionTypes().String()

	// Local
	clusterName, ok := localParams[s.opts.replicationContextPrefix+"systemName"]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "can't find `systemName` parameter in replication group")
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, status.Errorf(codes.InvalidArgument, "can't find cluster with name %s in driver config: %s", clusterName, err.Error())
	}

	// Remote
	remoteClusterName, ok := localParams[s.opts.replicationContextPrefix+"remoteSystemName"]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "can't find `remoteSystemName` parameter in replication group")
	}

	remoteIsiConfig, err := s.getIsilonConfig(ctx, &remoteClusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, status.Errorf(codes.InvalidArgument, "can't find cluster with name %s in driver config: %s", remoteClusterName, err.Error())
	}

	// auto probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	if err := s.autoProbe(ctx, remoteIsiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	vgName, ok := localParams[s.opts.replicationContextPrefix+"VolumeGroupName"]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "can't find `VolumeGroupName` parameter in replication group")
	}

	// log all parameters used in ExecuteAction call
	fields := map[string]interface{}{
		"RequestID":             reqID,
		"ClusterName":           clusterName,
		"RemoteClusterName":     remoteClusterName,
		"ProtectedStorageGroup": protectionGroupID,
		"Action":                action,
	}

	log.WithFields(fields).Info("Executing ExecuteAction with following fields")

	var actionFunc func(context.Context, *IsilonClusterConfig, *IsilonClusterConfig, string, *logrus.Entry) error

	switch action {
	case csiext.ActionTypes_FAILOVER_REMOTE.String():
		actionFunc = failover
	case csiext.ActionTypes_UNPLANNED_FAILOVER_LOCAL.String():
		actionFunc = failoverUnplanned
	case csiext.ActionTypes_REPROTECT_LOCAL.String():
		actionFunc = reprotect
	case csiext.ActionTypes_SYNC.String():
		actionFunc = syncAction
	case csiext.ActionTypes_SUSPEND.String():
		actionFunc = suspend
	case csiext.ActionTypes_RESUME.String():
		actionFunc = resume
	default:
		return nil, status.Errorf(codes.Unknown, "The requested action does not match with supported actions")
	}

	if err := actionFunc(ctx, isiConfig, remoteIsiConfig, vgName, log.WithFields(fields)); err != nil {
		return nil, err
	}

	// TODO: uncomment when GetSPGStatus call will be implemented
	// statusResp, err := s.GetStorageProtectionGroupStatus(ctx, &csiext.GetStorageProtectionGroupStatusRequest{
	// 	ProtectionGroupId:         protectionGroupID,
	// 	ProtectionGroupAttributes: localParams,
	// })
	// if err != nil {
	// 	return nil, status.Errorf(codes.Internal, "can't get storage protection group status: %s", err.Error())
	// }

	resp := &csiext.ExecuteActionResponse{
		Success: true,
		ActionTypes: &csiext.ExecuteActionResponse_Action{
			Action: req.GetAction(),
		},
		// Status: statusResp.Status,
	}
	return resp, nil
}

func (s *service) GetStorageProtectionGroupStatus(ctx context.Context, request *csiext.GetStorageProtectionGroupStatusRequest) (*csiext.GetStorageProtectionGroupStatusResponse, error) {
	panic("implement me")
}

func failover(ctx context.Context, localIsiConfig *IsilonClusterConfig, remoteIsiConfig *IsilonClusterConfig, vgName string, log *logrus.Entry) error {
	log.Info("Running failover action")

	ppName := strings.ReplaceAll(vgName, ".", "-")

	log.Info("Running sync on SRC policy")
	err := localIsiConfig.isiSvc.client.SyncPolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "failover: encountered error when trying to sync policy %s", err.Error())
	}

	log.Info("Ensuring that mirror policy exists on target site")
	// Get local policy to get necessary info
	localPolicy, err := localIsiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "failover: can't find local replication policy, unexpected error %s", err.Error())
	}

	_, err = remoteIsiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
	if err != nil {
		if apiErr, ok := err.(*isiApi.JSONError); ok && apiErr.StatusCode == 404 {
			err := remoteIsiConfig.isiSvc.client.CreatePolicy(ctx, ppName, localPolicy.JobDelay,
				localPolicy.SourcePath, localPolicy.TargetPath, localIsiConfig.Endpoint, false)
			if err != nil {
				return status.Errorf(codes.Internal, "failover: can't create protection policy %s", err.Error())
			}
			err = remoteIsiConfig.isiSvc.client.WaitForPolicyLastJobState(ctx, ppName, isi.UNKNOWN) // UNKNOWN because we created disabled policy
			if err != nil {
				return status.Errorf(codes.Internal, "failover: remote policy job couldn't reach UNKNOWN state %s", err.Error())
			}
		} else {
			return status.Errorf(codes.Internal, "failover: can't ensure protection policy exists %s", err.Error())
		}
	}

	log.Info("Enabling writes on TGT site")

	err = remoteIsiConfig.isiSvc.client.AllowWrites(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "failover: can't allow writes on target site %s", err.Error())
	}

	log.Info("Disabling policy on SRC site")

	err = localIsiConfig.isiSvc.client.DisablePolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "failover: can't disable local policy %s", err.Error())
	}

	err = localIsiConfig.isiSvc.client.WaitForPolicyEnabledFieldCondition(ctx, ppName, false)
	if err != nil {
		return status.Errorf(codes.Internal, "failover: policy couldn't reach disabled condition %s", err.Error())
	}

	log.Info("Disabling writes on SRC site, if we have target policy created here")

	// Disable writes on local (if we can)
	tp, err := localIsiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
	if err != nil {
		if e, ok := err.(*isiApi.JSONError); ok {
			if e.StatusCode != 404 {
				return status.Errorf(codes.Internal, "failover: couldn't get target policy %s", err.Error())
			}
		}
	}

	if tp != nil {
		err := localIsiConfig.isiSvc.client.DisallowWrites(ctx, ppName)
		if err != nil {
			return status.Errorf(codes.Internal, "failover: can't disallow writes on local site %s", err.Error())
		}
	}

	return nil
}

func failoverUnplanned(ctx context.Context, localIsiConfig *IsilonClusterConfig, remoteIsiConfig *IsilonClusterConfig, vgName string, log *logrus.Entry) error {
	log.Info("Running unplanned failover action")
	// With unplanned failover -- do minimum requests, we will ensure mirrored policy is created in further reprotect call
	// We can't use remote config because we need to assume it's down

	ppName := strings.ReplaceAll(vgName, ".", "-")

	log.Info("Breaking association on TGT site")
	err := localIsiConfig.isiSvc.client.BreakAssociation(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "unplanned failover: can't break association on target site %s", err.Error())
	}

	return nil
}

func reprotect(ctx context.Context, localIsiConfig *IsilonClusterConfig, remoteIsiConfig *IsilonClusterConfig, vgName string, log *logrus.Entry) error {
	log.Info("Running reprotect action")
	// this assumes we run reprotect_local action hence we use localIsiConfig

	ppName := strings.ReplaceAll(vgName, ".", "-")

	remotePolicy, err := remoteIsiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
	if err != nil {
		if apiErr, ok := err.(*isiApi.JSONError); ok && apiErr.StatusCode != 404 {
			return status.Errorf(codes.Internal, "reprotect: can't get policy %s by name %s", ppName, err.Error())
		}
	}

	if remotePolicy != nil && remotePolicy.Enabled {
		// If remote policy is enabled we assume we got here after unplanned failover call
		log.Info("Protection Policy is still enabled on TGT site, disabling it")
		err = remoteIsiConfig.isiSvc.client.DisablePolicy(ctx, ppName)
		if err != nil {
			return status.Errorf(codes.Internal, "reprotect: can't disable the policy on TGT %s", err.Error())
		}

		err = remoteIsiConfig.isiSvc.client.WaitForPolicyEnabledFieldCondition(ctx, ppName, false)
		if err != nil {
			return status.Errorf(codes.Internal, "reprotect: policy couldn't reach enabled condition on TGT %s", err.Error())
		}

		log.Info("Resetting the policy")
		err = remoteIsiConfig.isiSvc.client.ResetPolicy(ctx, ppName)
		if err != nil {
			return status.Errorf(codes.Internal, "reprotect: policy couldn't reach enabled condition on TGT %s", err.Error())
		}
	}

	var jobDelay int
	var sourcePath, targetPath string

	if remotePolicy != nil {
		log.Info("Remote policy is NOT empty, taking replication parameters")
		jobDelay = remotePolicy.JobDelay
		sourcePath = remotePolicy.SourcePath
		targetPath = remotePolicy.TargetPath
	} else {
		log.Info("Remote policy is empty, figuring out replication parameters")
		s := strings.Split(vgName, "-") // split by "_" and get last part -- it would be RPO
		rpo := s[len(s)-1]

		rpoEnum := RPOEnum(rpo)
		if err := rpoEnum.IsValid(); err != nil {
			return status.Errorf(codes.InvalidArgument, "invalid rpo value")
		}

		rpoint, err := rpoEnum.ToInt()
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "unable to parse rpo seconds")
		}

		jobDelay = rpoint
		sourcePath = localIsiConfig.IsiPath + "/" + vgName
		targetPath = remoteIsiConfig.IsiPath + "/" + vgName
	}

	log.Info("Ensuring that policy exists on local site")
	_, err = localIsiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
	if err != nil {
		if apiErr, ok := err.(*isiApi.JSONError); ok && apiErr.StatusCode == 404 {
			err := localIsiConfig.isiSvc.client.CreatePolicy(ctx, ppName, jobDelay,
				sourcePath, targetPath, remoteIsiConfig.Endpoint, true)
			if err != nil {
				return status.Errorf(codes.Internal, "reprotect: can't create protection policy %s", err.Error())
			}
			err = localIsiConfig.isiSvc.client.WaitForPolicyLastJobState(ctx, ppName, isi.FINISHED)
			if err != nil {
				return status.Errorf(codes.Internal, "reprotect: policy job couldn't reach FINISHED state %s", err.Error())
			}
		} else {
			return status.Errorf(codes.Internal, "reprotect: can't ensure protection policy exists %s", err.Error())
		}
	}

	log.Info("Enabling policy")
	err = localIsiConfig.isiSvc.client.EnablePolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "reprotect: can't enable policy %s", err.Error())
	}

	err = localIsiConfig.isiSvc.client.WaitForPolicyEnabledFieldCondition(ctx, ppName, true)
	if err != nil {
		return status.Errorf(codes.Internal, "reprotect: policy couldn't reach enabled condition %s", err.Error())
	}

	log.Info("Disable writes on remote")
	tp, err := remoteIsiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
	if err != nil {
		if e, ok := err.(*isiApi.JSONError); ok {
			if e.StatusCode != 404 {
				return status.Errorf(codes.Internal, "reprotect: couldn't get target policy %s", err.Error())
			}
		}
	}

	if tp != nil {
		err := remoteIsiConfig.isiSvc.client.DisallowWrites(ctx, ppName)
		if err != nil {
			return status.Errorf(codes.Internal, "reprotect: can't disallow writes on remote site %s", err.Error())
		}
	}

	return nil
}

func syncAction(ctx context.Context, localIsiConfig *IsilonClusterConfig, remoteIsiConfig *IsilonClusterConfig, vgName string, log *logrus.Entry) error {
	log.Info("Running sync action")

	ppName := strings.ReplaceAll(vgName, ".", "-")

	err := localIsiConfig.isiSvc.client.SyncPolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "sync: encountered error when trying to sync policy %s", err.Error())
	}

	return nil
}

func suspend(ctx context.Context, localIsiConfig *IsilonClusterConfig, remoteIsiConfig *IsilonClusterConfig, vgName string, log *logrus.Entry) error {
	log.Info("Running suspend action")

	ppName := strings.ReplaceAll(vgName, ".", "-")

	log.Info("Disabling policy on SRC site")

	err := localIsiConfig.isiSvc.client.DisablePolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "suspend: can't disable local policy %s", err.Error())
	}

	err = localIsiConfig.isiSvc.client.WaitForPolicyEnabledFieldCondition(ctx, ppName, false)
	if err != nil {
		return status.Errorf(codes.Internal, "suspend: policy couldn't reach disabled condition %s", err.Error())
	}

	return nil
}

func resume(ctx context.Context, localIsiConfig *IsilonClusterConfig, remoteIsiConfig *IsilonClusterConfig, vgName string, log *logrus.Entry) error {
	log.Info("Running resume action")

	ppName := strings.ReplaceAll(vgName, ".", "-")

	log.Info("Disabling policy on SRC site")

	err := localIsiConfig.isiSvc.client.EnablePolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "suspend: can't disable local policy %s", err.Error())
	}

	err = localIsiConfig.isiSvc.client.WaitForPolicyEnabledFieldCondition(ctx, ppName, true)
	if err != nil {
		return status.Errorf(codes.Internal, "suspend: policy couldn't reach disabled condition %s", err.Error())
	}

	return nil
}

func getRemoteCSIVolume(ctx context.Context, exportID int, volName, accessZone string, sizeInBytes int64, clusterName string) *csiext.Volume {
	volume := &csiext.Volume{
		VolumeId:      utils.GetNormalizedVolumeID(ctx, volName, exportID, accessZone, clusterName),
		CapacityBytes: sizeInBytes,
		VolumeContext: nil, // TODO: add values to volume context if needed
	}
	return volume
}
