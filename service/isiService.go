package service

/*
 Copyright (c) 2019-2023 Dell Inc, or its subsidiaries.

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
	"path"
	"strconv"
	"strings"
	"sync"

	apiv1 "github.com/dell/goisilon/api/v1"

	utils "github.com/dell/csi-isilon/v2/common/utils"
	isi "github.com/dell/goisilon"
	"github.com/dell/goisilon/api"
)

type isiService struct {
	endpoint string
	client   *isi.Client
}

func (svc *isiService) CopySnapshot(ctx context.Context, isiPath, snapshotSourceVolumeIsiPath string, srcSnapshotID int64, dstVolumeName string, accessZone string) (isi.Volume, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin to copy snapshot '%d'", srcSnapshotID)

	var volumeNew isi.Volume
	var err error
	if volumeNew, err = svc.client.CopySnapshotWithIsiPath(ctx, isiPath, snapshotSourceVolumeIsiPath, srcSnapshotID, "", dstVolumeName, accessZone); err != nil {
		log.Errorf("copy snapshot failed, '%s'", err.Error())
		return nil, err
	}

	return volumeNew, nil
}

func (svc *isiService) CopyVolume(ctx context.Context, isiPath, srcVolumeName, dstVolumeName string) (isi.Volume, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin to copy volume '%s'", srcVolumeName)

	var volumeNew isi.Volume
	var err error
	if volumeNew, err = svc.client.CopyVolumeWithIsiPath(ctx, isiPath, srcVolumeName, dstVolumeName); err != nil {
		log.Errorf("copy volume failed, '%s'", err.Error())
		return nil, err
	}

	return volumeNew, nil
}

func (svc *isiService) CreateSnapshot(ctx context.Context, path, snapshotName string) (isi.Snapshot, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin to create snapshot '%s'", snapshotName)

	var snapshot isi.Snapshot
	var err error
	if snapshot, err = svc.client.CreateSnapshotWithPath(ctx, path, snapshotName); err != nil {
		log.Errorf("create snapshot failed, '%s'", err.Error())
		return nil, err
	}

	return snapshot, nil
}

func (svc *isiService) CreateWriteableSnapshot(ctx context.Context, isipath, sourceSnapshotID, snapshotName string) (isi.WriteableSnapshot, error) {
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("CreateWriteableSnapshot: isiPath: ", isipath) // Remove before PR submission.
	log.Debugf("begin to create writeable snapshot '%s' from source snapshot '%s'", snapshotName, sourceSnapshotID)

	snapshot, err := svc.client.CreateWriteableSnapshot(ctx, sourceSnapshotID, snapshotName)
	if err != nil {
		log.Errorf("create writeable snapshot failed, '%s'", err.Error())
		return nil, err
	}

	return snapshot, nil
}

func (svc *isiService) CreateVolume(ctx context.Context, isiPath, volName, isiVolumePathPermissions string) error {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin to create volume '%s'", volName)

	if _, err := svc.client.CreateVolumeWithIsipath(ctx, isiPath, volName, isiVolumePathPermissions); err != nil {
		log.Errorf("create volume failed, '%s'", err.Error())
		return err
	}
	return nil
}

func (svc *isiService) CreateVolumeWithMetaData(ctx context.Context, isiPath, volName, isiVolumePathPermissions string, metadata map[string]string) error {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin to create volume '%s'", volName)
	log.Debugf("header metadata '%v'", metadata)

	if _, err := svc.client.CreateVolumeWithIsipathMetaData(ctx, isiPath, volName, isiVolumePathPermissions, metadata); err != nil {
		log.Errorf("create volume failed, '%s'", err.Error())
		return err
	}
	return nil
}

func (svc *isiService) GetExports(ctx context.Context) (isi.ExportList, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debug("begin getting exports for Isilon")

	var exports isi.ExportList
	var err error
	if exports, err = svc.client.GetExports(ctx); err != nil {
		log.Error("failed to get exports")
		return nil, err
	}

	return exports, nil
}

func (svc *isiService) GetExportByIDWithZone(ctx context.Context, exportID int, accessZone string) (isi.Export, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin getting export by id '%d' with access zone '%s' for Isilon", exportID, accessZone)

	var export isi.Export
	var err error
	if export, err = svc.client.GetExportByIDWithZone(ctx, exportID, accessZone); err != nil {
		log.Error("failed to get export by id with access zone")
		return nil, err
	}

	return export, nil
}

func (svc *isiService) ExportVolumeWithZone(ctx context.Context, isiPath, volName, accessZone, description string) (int, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin to export volume '%s' with access zone '%s' in Isilon path '%s'", volName, accessZone, isiPath)

	var exportID int
	var err error

	path := utils.GetPathForVolume(isiPath, volName)
	if exportID, err = svc.client.ExportVolumeWithZoneAndPath(ctx, path, accessZone, description); err != nil {
		log.Errorf("Export volume failed, volume '%s', access zone '%s' , id %d error '%s'", volName, accessZone, exportID, err.Error())
		return -1, err
	}

	log.Infof("Exported volume '%s' successfully, id '%d'", volName, exportID)
	return exportID, nil
}

func (svc *isiService) CreateQuota(ctx context.Context, path, volName, softLimit, advisoryLimit, softGracePrd string, sizeInBytes int64, quotaEnabled bool) (string, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)
	log.Debugf("begin to create quota for '%s', size '%d', quota enabled: '%t'", volName, sizeInBytes, quotaEnabled)
	var softi, advisoryi int64
	var err error
	var softlimitInt, advisoryLimitInt, softGracePrdInt int64
	softGracePrdInt, err = strconv.ParseInt(softGracePrd, 10, 64)
	if err != nil {
		log.Debugf("Invalid softGracePrd value. Setting it to default.")
		softGracePrdInt = 0
	}
	// converting soft limit from %ge to value
	if softLimit != "" {
		softi, err = strconv.ParseInt(softLimit, 10, 64)
		if err != nil {
			log.Debugf("Invalid softLimit value. Setting it to default.")
			softlimitInt = 0
		} else {
			softlimitInt = (softi * sizeInBytes) / 100
		}
	}
	if advisoryLimit != "" {
		advisoryi, err = strconv.ParseInt(advisoryLimit, 10, 64)
		if err != nil {
			log.Debugf("Invalid advisoryLimit value. Setting it to default.")
			advisoryLimitInt = 0

		} else {
			advisoryLimitInt = (advisoryi * sizeInBytes) / 100
		}
	}

	// if quotas are enabled, we need to set a quota on the volume
	if quotaEnabled {
		// need to set the quota based on the requested pv size
		// if a size isn't requested, skip creating the quota
		if sizeInBytes <= 0 {
			log.Debugf("SmartQuotas is enabled, but storage size is not requested, skip creating quotas for volume '%s'", volName)
			return "", nil
		}
		// Check if soft and advisory < 100
		if (softlimitInt >= sizeInBytes) || (advisoryLimitInt >= sizeInBytes) {
			log.Warnf("Soft and advisory thresholds must be smaller than the hard threshold. Setting it to default for Volume '%s'", volName)
			softlimitInt, advisoryLimitInt, softGracePrdInt = 0, 0, 0
		}
		// Check if Soft Grace period is set along with soft limit
		if (softlimitInt != 0) && (softGracePrdInt == 0) {
			log.Warnf("Soft Grace Period must be configured along with Soft threshold, Setting it to default for Volume '%s'", volName)
			softlimitInt, softGracePrdInt = 0, 0
		}
		var isQuotaActivated bool
		var checkLicErr error

		isQuotaActivated, checkLicErr = svc.client.IsQuotaLicenseActivated(ctx)

		if checkLicErr != nil {
			log.Errorf("failed to check SmartQuotas license info: '%v'", checkLicErr)
		}

		if (!isQuotaActivated) && (checkLicErr == nil) {
			log.Debugf("SmartQuotas is not activated, cannot add capacity limit '%d' bytes via quota, skip creating quota", sizeInBytes)
			return "", nil
		}

		// create quota with container set to true
		var quotaID string
		var err error
		if quotaID, err = svc.client.CreateQuotaWithPath(ctx, path, true, sizeInBytes, softlimitInt, advisoryLimitInt, softGracePrdInt); err != nil {
			if (isQuotaActivated) && (checkLicErr == nil) {
				return "", fmt.Errorf("SmartQuotas is activated, but creating quota failed with error: '%v'", err)
			}

			// if checkLicErr != nil, then it's uncertain whether creating quota failed because SmartQuotas license is not activated, or it failed with some other reason
			return "", fmt.Errorf("creating quota failed with error, it might or might not be because SmartQuotas license has not been activated: '%v'", err)
		}

		log.Infof("quota set to: %d on directory: '%s'", sizeInBytes, volName)

		return quotaID, nil
	}

	log.Debugf("quota is disabled, skip creating quota for '%s'", volName)

	return "", nil
}

func (svc *isiService) DeleteQuotaByExportIDWithZone(ctx context.Context, volName string, exportID int, accessZone string) error {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin to delete quota for volume name : '%s', export ID : '%d'", volName, exportID)

	var export isi.Export
	var err error
	var quotaID string

	if export, err = svc.client.GetExportByIDWithZone(ctx, exportID, accessZone); err != nil {
		return fmt.Errorf("failed to get export '%s':'%d' with access zone '%s', skip DeleteQuotaByID. error : '%s'", volName, exportID, accessZone, err.Error())
	}

	log.Debugf("export (id : '%d') corresponding to path '%s' found, description field is '%s'", export.ID, volName, export.Description)

	if quotaID, err = utils.GetQuotaIDFromDescription(ctx, export); err != nil {
		return err
	}

	if quotaID == "" {
		log.Debugf("No quota set on the volume, skip deleting quota")
		return nil
	}

	log.Debugf("deleting quota with id '%s' for path '%s'", quotaID, volName)

	if err = svc.client.ClearQuotaByID(ctx, quotaID); err != nil {
		return err
	}

	return nil
}

func (svc *isiService) GetVolumeQuota(ctx context.Context, volName string, exportID int, accessZone string) (isi.Quota, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin to get quota for volume name : '%s', export ID : '%d'", volName, exportID)

	var export isi.Export
	var err error
	var quotaID string

	if export, err = svc.client.GetExportByIDWithZone(ctx, exportID, accessZone); err != nil {
		return nil, fmt.Errorf("failed to get export '%s':'%d' with access zone '%s', error: '%s'", volName, exportID, accessZone, err.Error())
	}

	log.Debugf("export (id : '%d') corresponding to path '%s' found, description field is '%s'", export.ID, volName, export.Description)

	if quotaID, err = utils.GetQuotaIDFromDescription(ctx, export); err != nil {
		return nil, err
	}

	if quotaID == "" {
		log.Debugf("No quota set on the volume")
		return nil, fmt.Errorf("failed to get quota: No quota set on the volume '%s'", volName)
	}

	log.Debugf("get quota by id '%s'", quotaID)
	return svc.client.GetQuotaByID(ctx, quotaID)
}

func (svc *isiService) UpdateQuotaSize(ctx context.Context, quotaID string, updatedSize, updatedSoftLimit, updatedAdvisoryLimit, softGrace int64) error {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("updating quota by id '%s' with size '%d'", quotaID, updatedSize)

	if err := svc.client.UpdateQuotaSizeByID(ctx, quotaID, updatedSize, updatedSoftLimit, updatedAdvisoryLimit, softGrace); err != nil {
		return fmt.Errorf("failed to update quota '%s' with size '%d', error: '%s'", quotaID, updatedSize, err.Error())
	}

	return nil
}

func (svc *isiService) UnexportByIDWithZone(ctx context.Context, exportID int, accessZone string) error {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin to unexport NFS export with ID '%d' in access zone '%s'", exportID, accessZone)

	if err := svc.client.UnexportByIDWithZone(ctx, exportID, accessZone); err != nil {
		return fmt.Errorf("failed to unexport volume directory '%d' in access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}

	return nil
}

func (svc *isiService) GetExportsWithParams(ctx context.Context, params api.OrderedValues) (isi.Exports, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin to get exports with params..")
	var exports isi.Exports
	var err error

	if exports, err = svc.client.GetExportsWithParams(ctx, params); err != nil {
		return nil, fmt.Errorf("failed to get exports with params")
	}
	return exports, nil
}

func (svc *isiService) DeleteVolume(ctx context.Context, isiPath, volName string) error {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin to delete volume directory '%s'", volName)

	if err := svc.client.DeleteVolumeWithIsiPath(ctx, isiPath, volName); err != nil {
		return fmt.Errorf("failed to delete volume directory '%v' : '%v'", volName, err)
	}

	return nil
}

func (svc *isiService) ClearQuotaByID(ctx context.Context, quotaID string) error {
	if quotaID != "" {
		if err := svc.client.ClearQuotaByID(ctx, quotaID); err != nil {
			return fmt.Errorf("failed to clear quota for '%s' : '%v'", quotaID, err)
		}
	}
	return nil
}

func (svc *isiService) TestConnection(ctx context.Context) error {
	// Fetch log handler
	ctx, log, _ := GetRunIDLog(ctx)

	log.Debugf("test connection client, user name : '%s'", svc.client.API.User())
	if _, err := svc.client.GetClusterConfig(ctx); err != nil {
		log.Errorf("error encountered, test connection failed : '%v'", err)
		return err
	}

	log.Debug("test connection succeeded")

	return nil
}

func (svc *isiService) GetNFSExportURLForPath(ip string, dirPath string) string {
	return fmt.Sprintf("%s:%s", ip, dirPath)
}

func (svc *isiService) GetVolume(ctx context.Context, isiPath, volID, volName string) (isi.Volume, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin getting volume with id '%s' and name '%s' for Isilon", volID, volName)

	var vol isi.Volume
	var err error
	if vol, err = svc.client.GetVolumeWithIsiPath(ctx, isiPath, volID, volName); err != nil {
		log.Errorf("failed to get volume '%s'", err)
		return nil, err
	}

	return vol, nil
}

func (svc *isiService) GetVolumeSize(ctx context.Context, isiPath, name string) int64 {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin getting volume size with name '%s' for Isilon", name)

	size, err := svc.client.GetVolumeSize(ctx, isiPath, name)
	if err != nil {
		log.Errorf("failed to get volume size '%s'", err.Error())
		return 0
	}

	return size
}

func (svc *isiService) GetStatistics(ctx context.Context, keys []string) (isi.Stats, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	var stat isi.Stats
	var err error
	if stat, err = svc.client.GetStatistics(ctx, keys); err != nil {
		log.Errorf("failed to get array statistics '%s'", err)
		return nil, err
	}
	return stat, nil
}

func (svc *isiService) IsIOInProgress(ctx context.Context) (isi.Clients, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	var clients isi.Clients
	var err error
	if clients, err = svc.client.IsIOInProgress(ctx); err != nil {
		log.Errorf("failed to get array Clients '%s'", err)
		return nil, err
	}
	return clients, nil
}

func (svc *isiService) IsVolumeExistent(ctx context.Context, isiPath, volID, name string) bool {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("check if volume (id: '%s', name: '%s') already exists", volID, name)
	isExistent := svc.client.IsVolumeExistentWithIsiPath(ctx, isiPath, volID, name)
	log.Debugf("volume (id: '%s', name: '%s') already exists: '%v'", volID, name, isExistent)

	return isExistent
}

func (svc *isiService) OtherClientsAlreadyAdded(ctx context.Context, exportID int, accessZone string, nodeID string) bool {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	export, _ := svc.GetExportByIDWithZone(ctx, exportID, accessZone)

	if export == nil {
		log.Debugf("failed to get export by id '%d' with access zone '%s', return true for otherClientsAlreadyAdded as a safer return value", exportID, accessZone)
		return true
	}

	clientName, clientFQDN, clientIP, err := utils.ParseNodeID(ctx, nodeID)
	if err != nil {
		log.Debugf("failed to parse node ID '%s', return true for otherClientsAlreadyAdded as a safer return value", nodeID)
		return true
	}

	clientFieldsNotEmpty := len(*export.Clients) > 0 || len(*export.ReadOnlyClients) > 0 || len(*export.ReadWriteClients) > 0 || len(*export.RootClients) > 0

	clientFieldLength := len(*export.Clients)

	isNodeInClientFields := utils.IsStringInSlices(clientName, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)

	isNodeFQDNInClientFields := utils.IsStringInSlices(clientFQDN, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)

	if clientIP != "" {
		isNodeInClientFields = isNodeInClientFields || utils.IsStringInSlices(clientIP, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
	}

	clientName, clientFQDN, clientIP, err = utils.ParseNodeID(ctx, utils.DummyHostNodeID)
	if err != nil {
		log.Debugf("failed to parse node ID '%s', return true for otherClientsAlreadyAdded as a safer return value", nodeID)
		return true
	}

	// Additional check for dummy localhost entry
	isLocalHostInClientFields := utils.IsStringInSlices(clientName, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
	if !isLocalHostInClientFields {
		isLocalHostInClientFields = utils.IsStringInSlices(clientFQDN, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
		if !isLocalHostInClientFields {
			isLocalHostInClientFields = utils.IsStringInSlices(clientIP, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
		}
	}

	if clientFieldLength == 1 && isLocalHostInClientFields {
		clientFieldsNotEmpty = false
	}
	return clientFieldsNotEmpty && !isNodeInClientFields && !isNodeFQDNInClientFields
}

// updateClusterToNodeIDMap updates cluster to nodeID map from input clusterName, nodeID and clientToUse
func updateClusterToNodeIDMap(ctx context.Context, clusterToNodeIDMap *sync.Map, clusterName, nodeID, clientToUse string) error {
	log := utils.GetRunIDLogger(ctx)
	log.Debugf("updating ClusterToNodeIDMap map for cluster '%s', for nodeID '%s' with clientToUse '%s'", clusterName, nodeID, clientToUse)

	var nodeIDToClientMaps []*nodeIDToClientMap

	if m, found := clusterToNodeIDMap.Load(clusterName); found {
		log.Debugf("entry for cluster '%s' found in cluster to nodeID map", clusterName)
		var ok bool
		if nodeIDToClientMaps, ok = m.([]*nodeIDToClientMap); !ok {
			return fmt.Errorf("failed to extract nodeIDToClientMap for cluster '%s'", clusterName)
		}

		for i, nodeIDMap := range nodeIDToClientMaps {
			if client, ok := (*nodeIDMap)[nodeID]; ok {
				// no need to update the map for this nodeID, as current client and the client to be updated are same
				if client == clientToUse {
					return nil
				}

				// update the for the current nodeID with the new client
				nodeIDToClientMaps[i] = &nodeIDToClientMap{nodeID: clientToUse}
				clusterToNodeIDMap.Store(clusterName, nodeIDToClientMaps)
				return nil
			}
		}
		// make a new entry in the map for this nodeID
		clusterToNodeIDMap.Store(clusterName, append(nodeIDToClientMaps, &nodeIDToClientMap{nodeID: clientToUse}))
		return nil
	}

	// make a new entry for the input cluster and nodeID in the map
	clusterToNodeIDMap.Store(clusterName, []*nodeIDToClientMap{{nodeID: clientToUse}})

	return nil
}

// getClientToUseForNodeID returns client to use for an input nodeID from the cluster to nodeID map, if present.
// Otherwise returns an error
func getClientToUseForNodeID(ctx context.Context, clusterToNodeIDMap *sync.Map, clusterName, nodeID string) (string, error) {
	log := utils.GetRunIDLogger(ctx)

	var nodeIDToClientMaps []*nodeIDToClientMap

	if m, found := clusterToNodeIDMap.Load(clusterName); found {
		log.Debugf("entry for cluster '%s' found in cluster to nodeID map", clusterName)
		var ok bool
		if nodeIDToClientMaps, ok = m.([]*nodeIDToClientMap); !ok {
			return "", fmt.Errorf("failed to extract nodeIDToClientMap for cluster '%s'", clusterName)
		}

		for _, nodeIDMap := range nodeIDToClientMaps {
			if client, ok := (*nodeIDMap)[nodeID]; ok {
				log.Debugf("node id to client mapping found for nodeID '%s' client '%s'", nodeID, client)
				return client, nil
			}
		}
		return "", fmt.Errorf("node id to client map not found for nodeID '%s' in in cluster to nodeID map", nodeID)
	}
	return "", fmt.Errorf("entry for cluster '%s' not found in cluster to nodeID map", clusterName)
}

func (svc *isiService) AddExportClientNetworkIdentifierByIDWithZone(ctx context.Context, clusterName string, exportID int, accessZone, nodeID string, ignoreUnresolvableHosts bool, addClientFunc func(ctx context.Context, exportID int, accessZone, clientIP string, ignoreUnresolvableHosts bool) error) error {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)
	var clientToUse string

	// try adding by client FQDN first as it is preferred over IP for its stableness.
	// OneFS API will return error if it cannot resolve the client FQDN ,
	// in that case, fall back to adding by IP

	_, clientFQDN, clientIP, err := utils.ParseNodeID(ctx, nodeID)
	if err != nil {
		return err
	}

	log.Debugf("ignoreUnresolvableHosts set to '%v' for cluster '%s'", ignoreUnresolvableHosts, clusterName)
	if ignoreUnresolvableHosts {
		if err = addClientFunc(ctx, exportID, accessZone, clientIP, true); err != nil {
			log.Errorf("failed to add client '%s' to export id '%d': '%v'", clientIP, exportID, err)
			return fmt.Errorf("failed to add client '%s' to the export id '%d'", clientIP, exportID)
		}
		return nil
	}

	currentClient, err := getClientToUseForNodeID(ctx, clusterToNodeIDMap, clusterName, nodeID)
	if err != nil {
		log.Debug(err.Error())
		clientToUse = clientFQDN
	} else {
		clientToUse = currentClient
	}

	log.Debugf("AddExportClientNetworkIdentifierByID adding '%s' as client to export id '%d'", clientToUse, exportID)
	if err = addClientFunc(ctx, exportID, accessZone, clientToUse, false); err == nil {
		if err := updateClusterToNodeIDMap(ctx, clusterToNodeIDMap, clusterName, nodeID, clientToUse); err != nil {
			// not returning with error as export is already updated with client
			log.Warnf("failed to update cluster to nodeID map: '%s'", err)
		}

		return nil
	}
	log.Warnf("failed to add client '%s' to export id '%d': '%v'", clientToUse, exportID, err)

	// try updating export with other client
	otherClientToUse := clientFQDN
	if clientToUse == clientFQDN {
		otherClientToUse = clientIP
	}
	log.Debugf("AddExportClientNetworkIdentifierByID trying to add '%s' as client to export id '%d'", otherClientToUse, exportID)
	if err = addClientFunc(ctx, exportID, accessZone, otherClientToUse, false); err == nil {
		if err := updateClusterToNodeIDMap(ctx, clusterToNodeIDMap, clusterName, nodeID, otherClientToUse); err != nil {
			// not returning with error as export is already updated with client
			log.Warnf("failed to update cluster to nodeID map '%s'", err)
		}
		return nil
	}
	log.Warnf("failed to add client '%s' to export id '%d': '%v'", otherClientToUse, exportID, err)

	return fmt.Errorf("failed to add clients '%s' or '%s' to export id '%d'", clientToUse, otherClientToUse, exportID)
}

func (svc *isiService) AddExportClientByIDWithZone(ctx context.Context, exportID int, accessZone, clientIP string, ignoreUnresolvableHosts bool) error {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("AddExportClientByID client '%s'", clientIP)
	if err := svc.client.AddExportClientsByIDWithZone(ctx, exportID, accessZone, []string{clientIP}, ignoreUnresolvableHosts); err != nil {
		return fmt.Errorf("failed to add client to export id '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}
	return nil
}

func (svc *isiService) AddExportRootClientByIDWithZone(ctx context.Context, exportID int, accessZone, clientIP string, ignoreUnresolvableHosts bool) error {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("AddExportRootClientByID client '%s'", clientIP)
	if err := svc.client.AddExportRootClientsByIDWithZone(ctx, exportID, accessZone, []string{clientIP}, ignoreUnresolvableHosts); err != nil {
		return fmt.Errorf("failed to add client to export id '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}
	return nil
}

func (svc *isiService) AddExportReadOnlyClientByIDWithZone(ctx context.Context, exportID int, accessZone, clientIP string, ignoreUnresolvableHosts bool) error {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("AddExportReadOnlyClientByID client '%s'", clientIP)
	if err := svc.client.AddExportReadOnlyClientsByIDWithZone(ctx, exportID, accessZone, []string{clientIP}, ignoreUnresolvableHosts); err != nil {
		return fmt.Errorf("failed to add read only client to export id '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}
	return nil
}

func (svc *isiService) RemoveExportClientByIDWithZone(ctx context.Context, exportID int, accessZone, nodeID string, ignoreUnresolvableHosts bool) error {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	// it could either be IP or FQDN that has been added to the export's client fields, should consider both during the removal
	clientName, clientFQDN, clientIP, err := utils.ParseNodeID(ctx, nodeID)
	if err != nil {
		return err
	}

	log.Debugf("RemoveExportClientByIDWithZone client Name '%s', client FQDN '%s' client IP '%s'", clientName, clientFQDN, clientIP)

	clientsToRemove := []string{clientIP, clientName, clientFQDN}

	log.Debugf("RemoveExportClientByName client '%v'", clientsToRemove)

	if err := svc.client.RemoveExportClientsByIDWithZone(ctx, exportID, accessZone, clientsToRemove, ignoreUnresolvableHosts); err != nil {
		// Return success if export doesn't exist
		if notFoundErr, ok := err.(*api.JSONError); ok {
			if notFoundErr.StatusCode == 404 {
				log.Debugf("Export id '%v' does not exist", exportID)
				return nil
			}
		}
		return fmt.Errorf("failed to remove clients from export '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}

	return nil
}

func (svc *isiService) GetExportsWithLimit(ctx context.Context, limit string) (isi.ExportList, string, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debug("begin getting exports for Isilon")
	var exports isi.Exports
	var err error
	if exports, err = svc.client.GetExportsWithLimit(ctx, limit); err != nil {
		log.Error("failed to get exports")
		return nil, "", err
	}
	return exports.Exports, exports.Resume, nil
}

/*
	 func (svc *isiService) GetExportsWithResume(ctx context.Context, resume string) (isi.ExportList, string, error) {
		// Fetch log handler
		log := utils.GetRunIDLogger(ctx)

		log.Debug("begin getting exports for Isilon")
		var exports isi.Exports
		var err error
		if exports, err = svc.client.GetExportsWithResume(ctx, resume); err != nil {
			log.Error("failed to get exports: " + err.Error())
			return nil, "", err
		}
		return exports.Exports, exports.Resume, nil
	}
*/
func (svc *isiService) DeleteSnapshot(ctx context.Context, id int64, name string) error {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin to delete snapshot '%s'", name)
	if err := svc.client.RemoveSnapshot(ctx, id, name); err != nil {
		log.Errorf("delete snapshot failed, '%s'", err.Error())
		return err
	}
	return nil
}

func (svc *isiService) GetSnapshot(ctx context.Context, identity string) (isi.Snapshot, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin getting snapshot with id|name '%s' for Isilon", identity)
	var snapshot isi.Snapshot
	var err error
	if snapshot, err = svc.client.GetIsiSnapshotByIdentity(ctx, identity); err != nil {
		log.Errorf("failed to get snapshot '%s'", err.Error())
		return nil, err
	}

	return snapshot, nil
}

func (svc *isiService) GetSnapshotSize(ctx context.Context, isiPath, name string, accessZone string) int64 {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin getting snapshot size with name '%s' for Isilon", name)
	size, err := svc.client.GetSnapshotFolderSize(ctx, isiPath, name, accessZone)
	if err != nil {
		log.Errorf("failed to get snapshot size '%s'", err.Error())
		return 0
	}

	return size
}

func (svc *isiService) GetExportWithPathAndZone(ctx context.Context, path, accessZone string) (isi.Export, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	log.Debugf("begin getting export with target path '%s' and access zone '%s' for Isilon", path, accessZone)
	var export isi.Export
	var err error
	if export, err = svc.client.GetExportWithPathAndZone(ctx, path, accessZone); err != nil {
		log.Error("failed to get export with target path '" + path + "' and access zone '" + accessZone + "': '" + err.Error() + "'")
		return nil, err
	}

	return export, nil
}

func (svc *isiService) GetSnapshotIsiPath(ctx context.Context, isiPath string, sourceSnapshotID string, accessZone string) (string, error) {
	return svc.client.GetSnapshotIsiPath(ctx, isiPath, sourceSnapshotID, accessZone)
}

func (svc *isiService) GetZoneByName(ctx context.Context, accessZone string) (*apiv1.IsiZone, error) {
	zone, err := svc.client.GetZoneByName(ctx, accessZone)
	return zone, err
}

func (svc *isiService) isROVolumeFromSnapshot(exportPath, accessZone string) bool {
	isROVolFromSnapshot := false
	if accessZone == "System" {
		if strings.Index(exportPath, "/ifs/.snapshot") == 0 {
			isROVolFromSnapshot = true
		}
	} else {
		if strings.Index(exportPath, "/.snapshot") != -1 {
			isROVolFromSnapshot = true
		}
	}
	return isROVolFromSnapshot
}

func (svc *isiService) GetSnapshotNameFromIsiPath(ctx context.Context, snapshotIsiPath, accessZone, zonePath string) (string, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)
	var snapShotName string
	if !svc.isROVolumeFromSnapshot(snapshotIsiPath, accessZone) {
		log.Debugf("invalid snapshot isilon path- '%s'", snapshotIsiPath)
		return "", fmt.Errorf("invalid snapshot isilon path")
	}
	// Snapshot isi path format /<ifs>/.snapshot/<snapshot_name>/<volume_path_without_ifs_prefix>
	// Non System Access Zone /<ifs>/<csi_zone_base_path>/.snapshot/<snapshot_name>/<volume_path_without_ifs_prefix>
	pathWithoutZonePath := strings.Trim(snapshotIsiPath, zonePath)
	directories := strings.Split(pathWithoutZonePath, "/")
	// If there is no snapshot name in snapshot isi path or if it is empty
	if len(directories) < 2 || directories[2] == "" {
		log.Debugf("invalid snapshot isilon path- '%s'", snapshotIsiPath)
		return "", fmt.Errorf("invalid snapshot isilon path")
	}
	snapShotName = directories[2]
	return snapShotName, nil
}

func (svc *isiService) GetSnapshotIsiPathComponents(snapshotIsiPath, zonePath string) (string, string, string) {
	// Returns snapshot isi path components- isiPath, snapshotName, srcVolName
	var isiPath string
	var snapshotName string
	// Snapshot isi path format /<ifs>/.snapshot/<snapshot_name>/<volume_path_without_ifs_prefix>
	// Non System Access Zone /<ifs>/<csi_zone_base_path>/.snapshot/<snapshot_name>/<volume_path_without_ifs_prefix>
	dirs := strings.Split(snapshotIsiPath, "/")
	srcVolName := dirs[len(dirs)-1]
	// in case of non system access zone
	if dirs[2] != ".snapshot" {
		//.snapshot/snapshot_name/<volume path>
		pathWithoutZonePath := strings.Split(snapshotIsiPath, zonePath)
		directories := strings.Split(pathWithoutZonePath[1], "/")
		snapshotName = directories[2]
		// isi path is different than zone path
		if len(directories) > 3 {
			// volume path without volume name and ifs prefix
			remainIsiPath := strings.Join(directories[3:len(directories)-1], "/")
			isiPath = path.Join("/", zonePath, remainIsiPath)
		} else {
			isiPath = zonePath
		}
	} else {
		snapshotName = dirs[3]
		isiPath = path.Join("/", dirs[1], strings.Join(dirs[4:len(dirs)-1], "/"))
	}
	return isiPath, snapshotName, srcVolName
}

func (svc *isiService) GetSnapshotTrackingDirName(snapshotName string) string {
	return "." + "csi-" + snapshotName + "-tracking-dir"
}

func (svc *isiService) GetWriteableSnapshotTrackingDirName(snapshotName string) string {
	return "." + "csi-" + snapshotName + "-rw-tracking-dir"
}

func (svc *isiService) GetSubDirectoryCount(ctx context.Context, isiPath, directory string) (int64, error) {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	var totalSubDirectories int64
	if svc.IsVolumeExistent(ctx, isiPath, "", directory) {
		// Check if there are any entries for volumes present in snapshot tracking dir
		dirDetails, err := svc.GetVolume(ctx, isiPath, "", directory)
		if err != nil {
			return 0, err
		}
		log.Debugf("directory details for directory '%s' are '%s'", directory, dirDetails)

		// Get nlinks(i.e., subdirectories present) for snapshotTrackingDir
		for _, attr := range dirDetails.AttributeMap {
			if attr.Name == "nlink" {
				f, ok := attr.Value.(float64)
				if !ok {
					return 0, fmt.Errorf("failed to get total subdirectory count")
				}
				totalSubDirectories = int64(f)
				break
			}
		}
		// Every directory will have two subdirectory entries . and ..
		log.Debugf("total number of subdirectories present under directory '%s' is '%v'",
			directory, totalSubDirectories)
		return totalSubDirectories, nil
	}

	return 0, fmt.Errorf("failed to get subdirectory count for directory '%s'", directory)
}

func (svc *isiService) IsHostAlreadyAdded(ctx context.Context, exportID int, accessZone string, nodeID string) bool {
	// Fetch log handler
	log := utils.GetRunIDLogger(ctx)

	export, _ := svc.GetExportByIDWithZone(ctx, exportID, accessZone)

	if export == nil {
		log.Debugf("failed to get export by id '%d' with access zone '%s', return true for LocalhostAlreadyAdded as a safer return value", exportID, accessZone)
		return true
	}

	clientName, clientFQDN, clientIP, err := utils.ParseNodeID(ctx, nodeID)
	if err != nil {
		log.Debugf("failed to parse node ID '%s', return true for LocalhostAlreadyAdded as a safer return value", nodeID)
		return true
	}

	clientFieldsNotEmpty := len(*export.Clients) > 0 || len(*export.ReadOnlyClients) > 0 || len(*export.ReadWriteClients) > 0 || len(*export.RootClients) > 0

	isNodeInClientFields := utils.IsStringInSlices(clientName, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)

	isNodeFQDNInClientFields := utils.IsStringInSlices(clientFQDN, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)

	if clientIP != "" {
		isNodeInClientFields = isNodeInClientFields || utils.IsStringInSlices(clientIP, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
	}

	return clientFieldsNotEmpty && isNodeInClientFields || isNodeFQDNInClientFields
}

func (svc *isiService) GetSnapshotSourceVolumeIsiPath(ctx context.Context, snapshotID string) (string, error) {
	snapshot, err := svc.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return "", fmt.Errorf("failed to get snapshot id '%s', error '%v'", snapshotID, err)
	}

	return path.Dir(snapshot.Path), nil
}
