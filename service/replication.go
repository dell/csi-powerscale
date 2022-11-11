package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dell/csi-isilon/common/utils"
	csiext "github.com/dell/dell-csi-extensions/replication"
	isi "github.com/dell/goisilon"
	isiApi "github.com/dell/goisilon/api"
	v11 "github.com/dell/goisilon/api/v11"
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
	pathToStrip := ""
	storageClassIsi, ok := req.Parameters["IsiPath"]
	if !ok {
		pathToStrip = isiConfig.IsiPath
	} else {
		pathToStrip = storageClassIsi
	}
	ppName := strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(isiPath, pathToStrip), "/", ""), ".", "-")

	err = isiConfig.isiSvc.client.SyncPolicy(ctx, ppName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to sync data %s", err.Error())
	}
	volumeSize := isiConfig.isiSvc.GetVolumeSize(ctx, isiPath, volName)
	log.Info("Volume size got: ", volumeSize)

	remoteAccessZone, ok := req.Parameters[s.WithRP(KeyReplicationRemoteAccessZone)]
	if !ok {
		remoteAccessZone = accessZone
	}

	// Check if export exists
	remoteExport, err := remoteIsiConfig.isiSvc.GetExportWithPathAndZone(ctx, exportPath, remoteAccessZone)
	if err != nil {
		log.Info("Remote export error")
		return nil, status.Error(codes.NotFound, err.Error())
	}

	var remoteExportID int

	// If export does not exist we need to create it
	if remoteExport == nil {
		// Check if quota already exists
		log.Info("Remote export doesn't exist, create it")
		var quotaID string
		quota, err := remoteIsiConfig.isiSvc.client.GetQuotaWithPath(ctx, exportPath)
		log.Info("Get quota", quota)
		if err != nil {
			log.Info("Remote quota doesn't exist, create it")
			quotaID, err = remoteIsiConfig.isiSvc.CreateQuota(ctx, exportPath, volName, volumeSize, s.opts.QuotaEnabled)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "can't create volume quota %s", err.Error())
			}
		} else {
			quotaID = quota.Id
		}
		if remoteExportID, err = remoteIsiConfig.isiSvc.ExportVolumeWithZone(ctx, isiPath, volName, remoteAccessZone, utils.GetQuotaIDWithCSITag(quotaID)); err == nil && remoteExportID != 0 {
			// get the export and retry if not found to ensure the export has been created
			for i := 0; i < MaxRetries; i++ {
				if export, _ := remoteIsiConfig.isiSvc.GetExportByIDWithZone(ctx, remoteExportID, remoteAccessZone); export != nil {
					// Add dummy localhost entry for pvc security
					if !remoteIsiConfig.isiSvc.IsHostAlreadyAdded(ctx, remoteExportID, remoteAccessZone, utils.DummyHostNodeID) {
						err = remoteIsiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, remoteClusterName, remoteExportID, remoteAccessZone, utils.DummyHostNodeID, remoteIsiConfig.isiSvc.AddExportClientByIDWithZone)
						if err != nil {
							log.Debugf("Error while adding dummy localhost entry to export '%d'", remoteExportID)
						}
					}
				}
				time.Sleep(RetrySleepTime)
				log.Printf("Begin to retry '%d' time(s), for export id '%d' and path '%s'\n", i+1, remoteExportID, exportPath)
			}
		} else {
			return nil, status.Errorf(codes.Internal, "failed to create export: %s", err.Error())
		}
	} else {
		remoteExportID = remoteExport.ID
	}

	remoteVolume := getRemoteCSIVolume(ctx, remoteExportID, volName, remoteAccessZone, volumeSize, remoteClusterName)
	remoteAzServiceIP, ok := req.Parameters[s.WithRP(KeyReplicationRemoteAzServiceIP)]
	if !ok {
		remoteAzServiceIP = remoteIsiConfig.Endpoint
	}
	remoteRootClientEnabled, ok := req.Parameters[s.WithRP(KeyReplicationRemoteRootClientEnabled)]
	if !ok {
		remoteRootClientEnabled = RootClientEnabledParamDefault
	}
	volumeContext := map[string]string{
		"Path":              exportPath,
		"AccessZone":        remoteAccessZone,
		"ID":                strconv.Itoa(remoteExportID),
		"Name":              volName,
		"ClusterName":       remoteClusterName,
		"AzServiceIP":       remoteAzServiceIP,
		"RootClientEnabled": remoteRootClientEnabled,
	}

	log.Println(volumeContext)
	remoteVolume.VolumeContext = volumeContext

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
		return nil, status.Errorf(codes.Internal, "Error: Unable to get Volume Group '%s'", isiPath)
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "Error: Unable to get Volume Group '%s'", isiPath)
	}
	childs, err := isiConfig.isiSvc.client.QueryVolumeChildren(ctx, strings.TrimPrefix(isiPath, isiConfig.IsiPath))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error: Unable to get VG's childs at '%s'", isiPath)
	}
	for key := range childs {
		log.Info("Child Path: ", key)
		_, err := isiConfig.isiSvc.GetExportWithPathAndZone(ctx, key, "")
		if err == nil {
			return nil, status.Errorf(codes.Internal, "VG '%s' is not empty", isiPath)
		}
	}

	ppName := strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(isiPath, isiConfig.IsiPath), "/", ""), ".", "-")
	err = isiConfig.isiSvc.client.SyncPolicy(ctx, ppName)
	if err != nil {
		log.Error("Failed to sync before deletion ", err.Error())
	}

	log.Info("Breaking association on SRC site")
	err = isiConfig.isiSvc.client.BreakAssociation(ctx, ppName)
	e, ok := err.(*isiApi.JSONError)
	if err != nil {
		if (ok && e.StatusCode != 404) || !strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.Internal, "can't break association on source site %s", err.Error())
		}
	}

	err = isiConfig.isiSvc.DeleteVolume(ctx, isiPath, "")
	if err != nil {
		return nil, err
	}

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
			log.Errorf("Unknown error while deleting PP %s", ppName)
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
	case csiext.ActionTypes_FAILOVER_REMOTE.String(): // FAILOVER_LOCAL is not supported as of now. Need to handle failover steps in the mirrored perspective.
		actionFunc = failover
	case csiext.ActionTypes_UNPLANNED_FAILOVER_LOCAL.String():
		actionFunc = failoverUnplanned
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

	statusResp, err := s.GetStorageProtectionGroupStatus(ctx, &csiext.GetStorageProtectionGroupStatusRequest{
		ProtectionGroupId:         protectionGroupID,
		ProtectionGroupAttributes: localParams,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't get storage protection group status: %s", err.Error())
	}

	resp := &csiext.ExecuteActionResponse{
		Success: true,
		ActionTypes: &csiext.ExecuteActionResponse_Action{
			Action: req.GetAction(),
		},
		Status: statusResp.Status,
	}
	return resp, nil
}

func (s *service) GetStorageProtectionGroupStatus(ctx context.Context, req *csiext.GetStorageProtectionGroupStatusRequest) (*csiext.GetStorageProtectionGroupStatusResponse, error) {
	ctx, log, _ := GetRunIDLog(ctx)
	localParams := req.GetProtectionGroupAttributes()
	groupID := req.GetProtectionGroupId()
	isiPath := utils.GetIsiPathFromPgID(groupID)
	log.Infof("IsiPath: %s", isiPath)
	clusterName, ok := localParams[s.opts.replicationContextPrefix+"systemName"]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "Error: can't find `systemName` in replication group")
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, status.Errorf(codes.InvalidArgument, "can't find cluster with name %s in driver config: %s", clusterName, err.Error())
	}

	remoteClusterName, ok := localParams[s.opts.replicationContextPrefix+"remoteSystemName"]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "can't find `remoteSystemName` parameter in replication group")
	}

	remoteIsiConfig, err := s.getIsilonConfig(ctx, &remoteClusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, status.Errorf(codes.InvalidArgument, "can't find cluster with name %s in driver config: %s", remoteClusterName, err.Error())
	}

	vgName, ok := localParams[s.opts.replicationContextPrefix+"VolumeGroupName"]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "can't find `VolumeGroupName` parameter in replication group")
	}

	ppName := strings.ReplaceAll(vgName, ".", "-")

	// obtain local policy for local cluster
	localP, err := isiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
	if err != nil {
		log.Error("Can't find local replication policy on local cluster, unexpected error ", err.Error())
	}

	// obtain target policy for local cluster
	localTP, err := isiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
	if err != nil {
		log.Error("Can't find target replication policy on local cluster, unexpected error ", err.Error())
	}

	// obtain local policy for remote cluster
	remoteP, err := remoteIsiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
	if err != nil {
		log.Error("Can't find local replication policy on remote cluster, unexpected error ", err.Error())
	}

	// obtain target policy for remote cluster
	remoteTP, err := remoteIsiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
	if err != nil {
		log.Error("Can't find target replication policy on remote cluster, unexpected error ", err.Error())
	}

	// Check if any of the policy jobs are currently running
	var isSyncInProgress, isSyncCheckFailed bool
	localJob, err := isiConfig.isiSvc.client.GetJobsByPolicyName(ctx, ppName)
	if err != nil {
		if apiErr, ok := err.(*isiApi.JSONError); ok && apiErr.StatusCode != 404 {
			log.Error("Unexpected error while querying active jobs for local policy ", err.Error())
			isSyncCheckFailed = true
		}
	}
	for _, i := range localJob {
		if i.Action == v11.SYNC {
			isSyncInProgress = true
		}
	}

	remoteJob, err := remoteIsiConfig.isiSvc.client.GetJobsByPolicyName(ctx, ppName)
	if err != nil {
		if apiErr, ok := err.(*isiApi.JSONError); ok && apiErr.StatusCode != 404 {
			log.Error("Unexpected error while querying active jobs for remote policy ", err.Error())
			isSyncCheckFailed = true
		}
	}
	for _, i := range remoteJob {
		if i.Action == v11.SYNC {
			isSyncInProgress = true
		}
	}

	linkState := getGroupLinkState(localP, localTP, remoteP, remoteTP, isSyncInProgress)

	if linkState == csiext.StorageProtectionGroupStatus_UNKNOWN || isSyncCheckFailed {
		errMsg := "unexpected error while getting link state"
		if isSyncCheckFailed {
			errMsg = "unexpected error while querying active jobs for local or remote policy"
		}
		log.Error(errMsg)
		resp := &csiext.GetStorageProtectionGroupStatusResponse{
			Status: &csiext.StorageProtectionGroupStatus{
				State: csiext.StorageProtectionGroupStatus_UNKNOWN,
				// do not update isSource
			},
		}
		return resp, status.Errorf(codes.Internal, errMsg)
	}

	log.Info("Trying to get replication direction")
	source := false
	if linkState != csiext.StorageProtectionGroupStatus_FAILEDOVER && // no side can be source when in failed over state
		(localP.Enabled || // when synchronized
			(!remoteP.Enabled && localTP.FailoverFailbackState == "writes_enabled" && remoteTP.FailoverFailbackState == "writes_disabled")) { // when suspended (source side)
		source = true
		log.Info("Current side is source")
	}

	log.Infof("The current state for group (%s) is (%s).", groupID, linkState.String())
	resp := &csiext.GetStorageProtectionGroupStatusResponse{
		Status: &csiext.StorageProtectionGroupStatus{
			State:    linkState,
			IsSource: source,
		},
	}
	return resp, nil
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
				localPolicy.SourcePath, localPolicy.TargetPath, localIsiConfig.Endpoint, localIsiConfig.ReplicationCertificateID, false)
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

	log.Info("Disabling policy on SRC site")

	err = localIsiConfig.isiSvc.client.DisablePolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "failover: can't disable local policy %s", err.Error())
	}

	err = localIsiConfig.isiSvc.client.WaitForPolicyEnabledFieldCondition(ctx, ppName, false)
	if err != nil {
		return status.Errorf(codes.Internal, "failover: policy couldn't reach disabled condition %s", err.Error())
	}

	log.Info("Enabling writes on TGT site")

	err = remoteIsiConfig.isiSvc.client.AllowWrites(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "failover: can't allow writes on target site %s", err.Error())
	}

	return nil
}

func failoverUnplanned(ctx context.Context, localIsiConfig *IsilonClusterConfig, remoteIsiConfig *IsilonClusterConfig, vgName string, log *logrus.Entry) error {
	log.Info("Running unplanned failover action")
	// With unplanned failover -- do minimum requests, we will ensure mirrored policy is created in further reprotect call
	// We can't use remote config (source site) because we need to assume it's down

	log.Info("Enabling writes on TGT site")
	ppName := strings.ReplaceAll(vgName, ".", "-")
	err := localIsiConfig.isiSvc.client.AllowWrites(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "unplanned failover: allow writes on target site failed %s", err.Error())
	}

	return nil
}

func syncAction(ctx context.Context, localIsiConfig *IsilonClusterConfig, remoteIsiConfig *IsilonClusterConfig, vgName string, log *logrus.Entry) error {
	log.Info("Running sync action")
	// get all running
	// if running - wait for it and succeed
	// if no running - start new - wait for it and succeed
	ppName := strings.ReplaceAll(vgName, ".", "-")
	err := localIsiConfig.isiSvc.client.SyncPolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "policy sync failed %s", err.Error())
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

	log.Info("Enabling policy on SRC site")

	err := localIsiConfig.isiSvc.client.EnablePolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "resume: can't enable local policy %s", err.Error())
	}

	err = localIsiConfig.isiSvc.client.WaitForPolicyEnabledFieldCondition(ctx, ppName, true)
	if err != nil {
		return status.Errorf(codes.Internal, "resume: policy couldn't reach enabled condition %s", err.Error())
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

/** Identify the status of the protection group from local and remote policies.
	Synchronized:
		- (Source side) If the local policy is enabled, remote policy is disabled, local is write enabled and remote is write disabled
 		- (Target side) If the local policy is disabled, remote policy is enabled, local is write disabled and remote is write enabled
	Suspended:
		- (Source side) If the local policy is disabled, remote policy is disabled, local is write enabled and remote is write disabled
 		- (Target side) If the local policy is disabled, remote policy is disabled, local is write disabled and remote is write enabled
	Failover:
		1. Planned failover
		 - (both sides) local policy is disabled, remote policy is disabled, local is write enabled and remote is write enabled
		2. Unplanned failover (source down)
		 - (Source side) source is down. remote policy is still disabled and remote is write enabled
		 - (Target side) source is down. local policy is still disabled and local is write enabled
		3. Unplanned failover but source is up now
		 - (Source side) If the local policy is enabled, remote policy is disabled, local is write enabled and remote is write enabled
 		 - (Target side) If the local policy is disabled, remote policy is enabled, local is write enabled and remote is write enabled
*/
func getGroupLinkState(localP isi.Policy, localTP isi.TargetPolicy, remoteP isi.Policy, remoteTP isi.TargetPolicy, isSyncInProgress bool) csiext.StorageProtectionGroupStatus_State {
	var state csiext.StorageProtectionGroupStatus_State
	if isSyncInProgress { // sync-in-progress state
		state = csiext.StorageProtectionGroupStatus_SYNC_IN_PROGRESS
	} else if (localP == nil && remoteP != nil && !remoteP.Enabled && localTP == nil && remoteTP != nil && remoteTP.FailoverFailbackState == "writes_enabled") || // unplanned failover & source down - source side
		(localP != nil && !localP.Enabled && remoteP == nil && localTP != nil && localTP.FailoverFailbackState == "writes_enabled" && remoteTP == nil) { // target side
		state = csiext.StorageProtectionGroupStatus_FAILEDOVER
	} else if localP == nil || remoteP == nil || localTP == nil || remoteTP == nil { // both arrays should be up - unexpected case
		state = csiext.StorageProtectionGroupStatus_UNKNOWN
	} else if (localP.Enabled && !remoteP.Enabled && localTP.FailoverFailbackState == "writes_enabled" && remoteTP.FailoverFailbackState == "writes_enabled") || // unplanned failover & source up now - source side
		(!localP.Enabled && remoteP.Enabled && localTP.FailoverFailbackState == "writes_enabled" && remoteTP.FailoverFailbackState == "writes_enabled") { // target side
		state = csiext.StorageProtectionGroupStatus_FAILEDOVER
	} else if !localP.Enabled && !remoteP.Enabled && localTP.FailoverFailbackState == "writes_enabled" && remoteTP.FailoverFailbackState == "writes_enabled" { // planned failover - source OR target side
		state = csiext.StorageProtectionGroupStatus_FAILEDOVER
	} else if (localP.Enabled && !remoteP.Enabled && localTP.FailoverFailbackState == "writes_enabled" && remoteTP.FailoverFailbackState == "writes_disabled") || // Synchronized state - source side
		(!localP.Enabled && remoteP.Enabled && localTP.FailoverFailbackState == "writes_disabled" && remoteTP.FailoverFailbackState == "writes_enabled") { // target side
		state = csiext.StorageProtectionGroupStatus_SYNCHRONIZED
	} else if (!localP.Enabled && !remoteP.Enabled && localTP.FailoverFailbackState == "writes_enabled" && remoteTP.FailoverFailbackState == "writes_disabled") || // Suspended state - source side
		(!localP.Enabled && !remoteP.Enabled && localTP.FailoverFailbackState == "writes_disabled" && remoteTP.FailoverFailbackState == "writes_enabled") { // target side
		state = csiext.StorageProtectionGroupStatus_SUSPENDED
	} else if localTP.LastJobState == "failed" || remoteTP.LastJobState == "failed" { // invalid state, sync job failed
		state = csiext.StorageProtectionGroupStatus_INVALID
	} else { // unknown state
		state = csiext.StorageProtectionGroupStatus_UNKNOWN
	}

	return state
}
