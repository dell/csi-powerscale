package service

/*
 Copyright (c) 2019-2022 Dell Inc, or its subsidiaries.

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

// constants for ease of understanding
const (
	PolicySchedulingManual    = ""
	PolicySchedulingAutomatic = "when-source-modified"
	WritesEnabled             = "writes_enabled"
	WritesDisabled            = "writes_disabled"
	ResyncPolicyCreated       = "resync_policy_created"
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
						err = remoteIsiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, remoteClusterName, remoteExportID, remoteAccessZone, utils.DummyHostNodeID, *remoteIsiConfig.IgnoreUnresolvableHosts, remoteIsiConfig.isiSvc.AddExportClientByIDWithZone)
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

	vgName, ok := localParams[s.opts.replicationContextPrefix+"VolumeGroupName"]
	if !ok {
		log.Error("Can't get protection policy name from PG params")
		return nil, status.Errorf(codes.InvalidArgument, "can't find `VolumeGroupName` parameter from PG params")
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
	// TODO: This does not support storageclass IsiPath being anything other than a subdirectory of array secret's IsiPath.
	// Need to enhance IsiPath priority logic and querying logic.
	childs, err := isiConfig.isiSvc.client.QueryVolumeChildren(ctx, strings.TrimPrefix(isiPath, isiConfig.IsiPath))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error: Unable to get VG's childs at '%s'", isiPath)
	}
	for key := range childs {
		log.Info("Child Path: ", key)
		exports, _ := isiConfig.isiSvc.GetExportWithPathAndZone(ctx, key, "")
		if exports != nil {
			return nil, status.Errorf(codes.Internal, "VG '%s' is not empty", isiPath)
		}
	}

	ppName := strings.ReplaceAll(vgName, ".", "-")
	log.Info("!!!!!!!!!!!")
	log.Info("ppName: " + ppName)
	log.Info("!!!!!!!!!!!")

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
	case csiext.ActionTypes_FAILOVER_REMOTE.String(): // FAILOVER_LOCAL is not supported. Need to handle failover steps in the mirrored perspective.
		actionFunc = failover
	case csiext.ActionTypes_UNPLANNED_FAILOVER_LOCAL.String(): // UNPLANNED_FAILOVER_REMOTE is not supported.
		actionFunc = failoverUnplanned
	case csiext.ActionTypes_FAILBACK_LOCAL.String(): // FAILBACK_REMOTE is not supported.
		actionFunc = failbackDiscardLocal
	case csiext.ActionTypes_ACTION_FAILBACK_DISCARD_CHANGES_LOCAL.String(): // ACTION_FAILBACK_DISCARD_CHANGES_REMOTE is not supported.
		actionFunc = failbackDiscardRemote
	case csiext.ActionTypes_REPROTECT_LOCAL.String(): // REPROTECT_REMOTE is not supported.
		actionFunc = reprotect
	case csiext.ActionTypes_SYNC.String():
		actionFunc = synchronize
	case csiext.ActionTypes_SUSPEND.String():
		actionFunc = suspend
	case csiext.ActionTypes_RESUME.String():
		actionFunc = resume
	default:
		return nil, status.Errorf(codes.Unknown, "The requested action does not match with supported actions")
	}

	if err := actionFunc(ctx, isiConfig, remoteIsiConfig, vgName, log.WithFields(fields)); err != nil {
		return nil, status.Errorf(codes.Unknown, err.Error()) // Error while executing action, shouldn't be retried.
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

	log.Info("Getting storage protection group status")
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
		log.Warn("Can't find local replication policy on local cluster, unexpected error ", err.Error())
	}

	// obtain target policy for local cluster
	localTP, err := isiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
	if err != nil {
		log.Warn("Can't find target replication policy on local cluster, unexpected error ", err.Error())
	}

	// obtain local policy for remote cluster
	remoteP, err := remoteIsiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
	if err != nil {
		log.Warn("Can't find local replication policy on remote cluster, unexpected error ", err.Error())
	}

	// obtain target policy for remote cluster
	remoteTP, err := remoteIsiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
	if err != nil {
		log.Warn("Can't find target replication policy on remote cluster, unexpected error ", err.Error())
	}

	// Check if any of the policy jobs are currently running
	var isSyncInProgress, isSyncCheckFailed bool
	localJob, err := isiConfig.isiSvc.client.GetJobsByPolicyName(ctx, ppName)
	if err != nil {
		if apiErr, ok := err.(*isiApi.JSONError); ok && apiErr.StatusCode != 404 {
			log.Warn("Unexpected error while querying active jobs for local policy ", err.Error())
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
			log.Warn("Unexpected error while querying active jobs for remote policy ", err.Error())
			isSyncCheckFailed = true
		}
	}
	for _, i := range remoteJob {
		if i.Action == v11.SYNC {
			isSyncInProgress = true
		}
	}

	linkState := getGroupLinkState(localP, localTP, remoteP, remoteTP, isSyncInProgress)
	log.Infof("The current state for group (%s) is (%s).", groupID, linkState.String())

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
	if localP != nil { // Policy can exist only on the source side
		source = true
		log.Info("Current side is source")
	}

	resp := &csiext.GetStorageProtectionGroupStatusResponse{
		Status: &csiext.StorageProtectionGroupStatus{
			State:    linkState,
			IsSource: source,
		},
	}
	log.Info("Get storage protection group status completed")
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

	log.Info("Failover action completed")
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

	log.Info("Unplanned failover action completed")
	return nil
}

func reprotect(ctx context.Context, localIsiConfig *IsilonClusterConfig, remoteIsiConfig *IsilonClusterConfig, vgName string, log *logrus.Entry) error {
	log.Info("Running reprotect action")
	ppName := strings.ReplaceAll(vgName, ".", "-")

	// Ensure local array's target policy exists and is write enabled (original target)
	localTP, err := localIsiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "reprotect: can't find target policy on the local site, perform reprotect on another side. %s", err.Error())
	}
	if localTP.FailoverFailbackState != WritesEnabled {
		return status.Errorf(codes.InvalidArgument, "reprotect: unable to perform reprotect with writes disabled, should perform failover first.")
	}

	// Get remote policy
	remotePolicy, err := remoteIsiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "reprotect: can't find remote replication policy, unexpected error %s", err.Error())
	}

	// Delete the remote policy
	log.Info("Deleting SyncIQ policy on the remote")
	err = remoteIsiConfig.isiSvc.client.DeletePolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "reprotect: delete policy on remote site failed %s", err.Error())
	}

	// Create a new local policy based on previous remote policy's parameters
	log.Info("Creating new local SyncIQ policy")
	err = localIsiConfig.isiSvc.client.CreatePolicy(ctx, ppName, remotePolicy.JobDelay,
		remotePolicy.TargetPath, remotePolicy.SourcePath, remoteIsiConfig.Endpoint, localIsiConfig.ReplicationCertificateID, true)
	if err != nil {
		return status.Errorf(codes.Internal, "reprotect: create protection policy on the local site failed %s", err.Error())
	}
	err = localIsiConfig.isiSvc.client.WaitForPolicyLastJobState(ctx, ppName, isi.FINISHED)
	if err != nil {
		return status.Errorf(codes.Internal, "reprotect: policy job couldn't reach FINISHED state %s", err.Error())
	}

	log.Info("Reprotect action completed")
	return nil
}

func failbackDiscardLocal(ctx context.Context, localIsiConfig *IsilonClusterConfig, remoteIsiConfig *IsilonClusterConfig, vgName string, log *logrus.Entry) error {
	log.Info("Running failback action - discard local")
	ppName := strings.ReplaceAll(vgName, ".", "-")
	ppNameMirror := ppName + "_mirror"

	log.Info("Obtaining RPO value from policy name")
	rpoInt := getRpoInt(vgName)
	if rpoInt == -1 {
		return status.Errorf(codes.InvalidArgument, "unable to parse RPO seconds")
	}

	// If source policy is not disabled (unplanned failover), disable it
	log.Info("Ensuring SRC policy is disabled")
	err := localIsiConfig.isiSvc.client.DisablePolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard local): can't disable local policy %s", err.Error())
	}

	// Edit the source policy to manual.
	log.Info("Setting SRC policy to manual")
	err = localIsiConfig.isiSvc.client.ModifyPolicy(ctx, ppName, PolicySchedulingManual, 0)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard local): can't set local policy to manual %s", err.Error())
	}

	// Enable the source policy
	log.Info("Enabling SRC policy")
	err = localIsiConfig.isiSvc.client.EnablePolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard local): can't enable local policy %s", err.Error())
	}

	// Run Resync-prep on source (also disables source policy)
	log.Info("Running resync-prep on SRC policy")
	err = localIsiConfig.isiSvc.client.ResyncPrep(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard local): can't run resync-prep on local policy %s", err.Error())
	}
	err = remoteIsiConfig.isiSvc.client.WaitForTargetPolicyCondition(ctx, ppName, ResyncPolicyCreated)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard local): error waiting for condition on the remote target policy. %s", err.Error())
	}
	err = remoteIsiConfig.isiSvc.client.WaitForPolicyEnabledFieldCondition(ctx, ppNameMirror, true)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard local): TGT mirror policy couldn't reach enabled condition %s", err.Error())
	}

	// Run Sync-Job on target policy (_mirror)
	log.Info("Running sync job on TGT mirror policy")
	err = remoteIsiConfig.isiSvc.client.SyncPolicy(ctx, ppNameMirror)
	if err != nil {
		return status.Errorf(codes.Internal, "policy sync failed %s", err.Error())
	}

	// Allow write on source
	log.Info("Allowing write on SRC")
	err = localIsiConfig.isiSvc.client.AllowWrites(ctx, ppNameMirror)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard local): allow writes on local site failed %s", err.Error())
	}

	// Run resync-prep on target (also disables target policy)
	log.Info("Running resync-prep on TGT mirror policy")
	err = remoteIsiConfig.isiSvc.client.ResyncPrep(ctx, ppNameMirror)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard local): can't run resync-prep on remote mirror policy %s", err.Error())
	}
	err = localIsiConfig.isiSvc.client.WaitForTargetPolicyCondition(ctx, ppNameMirror, ResyncPolicyCreated)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard local): error waiting for condition on the local target policy. %s", err.Error())
	}

	// Delete the target mirror policy as recommended
	log.Info("Deleting TGT mirror policy")
	err = remoteIsiConfig.isiSvc.client.DeletePolicy(ctx, ppNameMirror)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard local): delete mirror policy on target site failed %s", err.Error())
	}

	// Edit source policy to automatic
	log.Info("Setting SRC policy to automatic")
	err = localIsiConfig.isiSvc.client.ModifyPolicy(ctx, ppName, PolicySchedulingAutomatic, rpoInt)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard local): can't set local policy to automatic %s", err.Error())
	}

	log.Info("Failback action - discard local completed")
	return nil
}

func failbackDiscardRemote(ctx context.Context, localIsiConfig *IsilonClusterConfig, remoteIsiConfig *IsilonClusterConfig, vgName string, log *logrus.Entry) error {
	log.Info("Running failback action - discard remote")
	ppName := strings.ReplaceAll(vgName, ".", "-")

	log.Info("Obtaining RPO value from policy name")
	rpoInt := getRpoInt(vgName)
	if rpoInt == -1 {
		return status.Errorf(codes.InvalidArgument, "unable to parse RPO seconds")
	}

	// If source policy is not disabled (unplanned failover), disable it
	log.Info("Ensuring SRC policy is disabled")
	err := localIsiConfig.isiSvc.client.DisablePolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard remote): can't disable local policy %s", err.Error())
	}

	// Edit the source policy to manual.
	log.Info("Setting SRC policy to manual")
	err = localIsiConfig.isiSvc.client.ModifyPolicy(ctx, ppName, PolicySchedulingManual, 0)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard remote): can't set local policy to manual %s", err.Error())
	}

	// disallow writes on target
	log.Info("Disabling writes on TGT site")
	err = remoteIsiConfig.isiSvc.client.DisallowWrites(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard remote): disallow writes on target site failed %s", err.Error())
	}

	// set source policy to automatic
	log.Info("Setting SRC policy to automatic")
	err = localIsiConfig.isiSvc.client.ModifyPolicy(ctx, ppName, PolicySchedulingAutomatic, rpoInt)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard remote): can't set local policy to automatic %s", err.Error())
	}

	// enable source policy
	log.Info("Enabling SRC policy")
	err = localIsiConfig.isiSvc.client.EnablePolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "failback (discard remote): can't enable local policy %s", err.Error())
	}

	log.Info("Failback action - discard remote completed")
	return nil
}

func synchronize(ctx context.Context, localIsiConfig *IsilonClusterConfig, remoteIsiConfig *IsilonClusterConfig, vgName string, log *logrus.Entry) error {
	log.Info("Running sync action")
	// get all running
	// if running - wait for it and succeed
	// if no running - start new - wait for it and succeed
	ppName := strings.ReplaceAll(vgName, ".", "-")
	err := localIsiConfig.isiSvc.client.SyncPolicy(ctx, ppName)
	if err != nil {
		return status.Errorf(codes.Internal, "policy sync failed %s", err.Error())
	}

	log.Info("Sync action completed")
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

	log.Info("Suspend action completed")
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

	log.Info("Resume action completed")
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

/*
  - Identify the status of the protection group from local and remote policies.
    Synchronized:
  - (Source side) If the local policy is enabled, remote policy is NIL, local TP is NIL and remote TP is write disabled
  - (Target side) If the local policy is NIL, remote policy is enabled, local TP is write disabled and remote TP is NIL
    Suspended:
  - (Source side) If the local policy is disabled, remote policy is NIL, local TP is NIL and remote is write disabled
  - (Target side) If the local policy is NIL, remote policy is disabled, local TP is write disabled and remote TP is NIL
    Failover:
    1. Planned failover
  - (Source side) If the local policy is disabled, remote policy is NIL, local TP is NIL and remote TP is write enabled
  - (Target side) If the local policy is NIL, remote policy is disabled, local TP is write enabled and remote TP is NIL
    2. Unplanned failover (source down)
  - (Source side) source is down. remote policy is NIL and remote TP is write enabled
  - (Target side) source is down. local policy is NIL and local TP is write enabled
    3. Unplanned failover but source is up now
  - (Source side) If the local policy is enabled, remote policy is NIL, local TP is NIL and remote TP is write enabled
  - (Target side) If the local policy is NIL, remote policy is enabled, local TP is write enabled and remote TP is NIL
*/
func getGroupLinkState(localP isi.Policy, localTP isi.TargetPolicy, remoteP isi.Policy, remoteTP isi.TargetPolicy, isSyncInProgress bool) csiext.StorageProtectionGroupStatus_State {
	var state csiext.StorageProtectionGroupStatus_State
	if (localP != nil && localP.Enabled && remoteP == nil && localTP == nil && remoteTP != nil && remoteTP.FailoverFailbackState == WritesDisabled) || // Synchronized state - source side
		(localP == nil && remoteP != nil && remoteP.Enabled && localTP != nil && localTP.FailoverFailbackState == WritesDisabled && remoteTP == nil) { // target side
		state = csiext.StorageProtectionGroupStatus_SYNCHRONIZED
	} else if (localP != nil && !localP.Enabled && remoteP == nil && localTP == nil && remoteTP != nil && remoteTP.FailoverFailbackState == WritesDisabled) || // Suspended state - source side
		(localP == nil && remoteP != nil && !remoteP.Enabled && localTP != nil && localTP.FailoverFailbackState == WritesDisabled && remoteTP == nil) { // target side
		state = csiext.StorageProtectionGroupStatus_SUSPENDED
	} else if (localP != nil && !localP.Enabled && remoteP == nil && localTP == nil && remoteTP != nil && remoteTP.FailoverFailbackState == WritesEnabled) || // planned failover - source side
		(localP == nil && remoteP != nil && !remoteP.Enabled && localTP != nil && localTP.FailoverFailbackState == WritesEnabled && remoteTP == nil) { // target side
		state = csiext.StorageProtectionGroupStatus_FAILEDOVER
	} else if localP == nil && remoteP == nil && localTP == nil && remoteTP != nil && remoteTP.FailoverFailbackState == WritesEnabled { // unplanned failover & source down - source side
		state = csiext.StorageProtectionGroupStatus_UNKNOWN // report UNKNOWN and maintain isSource when source is down on failedover
	} else if localP == nil && remoteP == nil && localTP != nil && localTP.FailoverFailbackState == WritesEnabled && remoteTP == nil { // unplanned failover & source down - target side
		state = csiext.StorageProtectionGroupStatus_FAILEDOVER
	} else if (localP != nil && localP.Enabled && remoteP == nil && localTP == nil && remoteTP != nil && remoteTP.FailoverFailbackState == WritesEnabled) || // unplanned failover & source up now - source side
		(localP == nil && remoteP != nil && remoteP.Enabled && localTP != nil && localTP.FailoverFailbackState == WritesEnabled && remoteTP == nil) { // target side
		state = csiext.StorageProtectionGroupStatus_FAILEDOVER
	} else if isSyncInProgress { // sync-in-progress state
		state = csiext.StorageProtectionGroupStatus_SYNC_IN_PROGRESS
	} else if (remoteTP != nil && remoteTP.LastJobState == "failed") || (localTP != nil && localTP.LastJobState == "failed") { // invalid state, sync job failed
		state = csiext.StorageProtectionGroupStatus_INVALID
	} else { // unknown state
		state = csiext.StorageProtectionGroupStatus_UNKNOWN
	}

	return state
}

func getRpoInt(vgName string) int {
	s := strings.Split(vgName, "-") // split by "_" and get last part -- it would be RPO
	rpo := s[len(s)-1]

	rpoEnum := RPOEnum(rpo)
	if err := rpoEnum.IsValid(); err != nil {
		return -1
	}

	rpoInt, err := rpoEnum.ToInt()
	if err != nil {
		return -1
	}

	return rpoInt
}
