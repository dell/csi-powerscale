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
	"context"
	"fmt"
	"net"

	utils "github.com/dell/csi-isilon/common/utils"
	isi "github.com/dell/goisilon"
	"github.com/dell/goisilon/api"
	log "github.com/sirupsen/logrus"
)

type isiService struct {
	endpoint string
	client   *isi.Client
}

func (svc *isiService) CopySnapshot(isiPath string, srcSnapshotID int64, dstVolumeName string) (isi.Volume, error) {
	log.Debugf("begin to copy snapshot '%d'", srcSnapshotID)

	var volumeNew isi.Volume
	var err error
	if volumeNew, err = svc.client.CopySnapshotWithIsiPath(context.Background(), isiPath, srcSnapshotID, "", dstVolumeName); err != nil {
		log.Errorf("copy snapshot failed, '%s'", err.Error())
		return nil, err
	}

	return volumeNew, nil
}

func (svc *isiService) CopyVolume(isiPath, srcVolumeName, dstVolumeName string) (isi.Volume, error) {
	log.Debugf("begin to copy volume '%s'", srcVolumeName)

	var volumeNew isi.Volume
	var err error
	if volumeNew, err = svc.client.CopyVolumeWithIsiPath(context.Background(), isiPath, srcVolumeName, dstVolumeName); err != nil {
		log.Errorf("copy volume failed, '%s'", err.Error())
		return nil, err
	}

	return volumeNew, nil
}

func (svc *isiService) CreateSnapshot(path, snapshotName string) (isi.Snapshot, error) {
	log.Debugf("begin to create snapshot '%s'", snapshotName)

	var snapshot isi.Snapshot
	var err error
	if snapshot, err = svc.client.CreateSnapshotWithPath(context.Background(), path, snapshotName); err != nil {
		log.Errorf("create snapshot failed, '%s'", err.Error())
		return nil, err
	}

	return snapshot, nil
}

func (svc *isiService) CreateVolume(isiPath, volName string) error {
	log.Debugf("begin to create volume '%s'", volName)

	if _, err := svc.client.CreateVolumeWithIsipath(context.Background(), isiPath, volName); err != nil {
		log.Errorf("create volume failed, '%s'", err.Error())
		return err
	}

	return nil
}

func (svc *isiService) GetExports() (isi.ExportList, error) {
	log.Debug("begin getting exports for Isilon")

	var exports isi.ExportList
	var err error
	if exports, err = svc.client.GetExports(context.Background()); err != nil {
		log.Error("failed to get exports")
		return nil, err
	}

	return exports, nil
}

func (svc *isiService) GetExportByIDWithZone(exportID int, accessZone string) (isi.Export, error) {
	log.Debugf("begin getting export by id '%d' with access zone '%s' for Isilon", exportID, accessZone)

	var export isi.Export
	var err error
	if export, err = svc.client.GetExportByIDWithZone(context.Background(), exportID, accessZone); err != nil {
		log.Error("failed to get export by id with access zone")
		return nil, err
	}

	return export, nil
}

func (svc *isiService) ExportVolumeWithZone(isiPath, volName, accessZone, description string) (int, error) {
	log.Debugf("begin to export volume '%s' with access zone '%s' in Isilon path '%s'", volName, accessZone, isiPath)

	var exportID int
	var err error

	path := utils.GetPathForVolume(isiPath, volName)
	if exportID, err = svc.client.ExportVolumeWithZoneAndPath(context.Background(), path, accessZone, description); err != nil {
		log.Errorf("Export volume failed, volume '%s', access zone '%s' , id %d error '%s'", volName, accessZone, exportID, err.Error())
		return -1, err
	}

	log.Infof("Exported volume '%s' successfully, id '%d'", volName, exportID)
	return exportID, nil
}

func (svc *isiService) CreateQuota(path, volName string, sizeInBytes int64, quotaEnabled bool) (string, error) {
	log.Debugf("begin to create quota for '%s', size '%d', quota enabled: '%t'", volName, sizeInBytes, quotaEnabled)

	// if quotas are enabled, we need to set a quota on the volume
	if quotaEnabled {
		// need to set the quota based on the requested pv size
		// if a size isn't requested, skip creating the quota
		if sizeInBytes <= 0 {
			log.Debugf("SmartQuotas is enabled, but storage size is not requested, skip creating quotas for volume '%s'", volName)
			return "", nil
		}

		var isQuotaActivated bool
		var checkLicErr error

		isQuotaActivated, checkLicErr = svc.client.IsQuotaLicenseActivated(context.Background())

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
		if quotaID, err = svc.client.CreateQuotaWithPath(context.Background(), path, true, sizeInBytes); err != nil {
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

func (svc *isiService) DeleteQuotaByExportIDWithZone(volName string, exportID int, accessZone string) error {
	log.Debugf("begin to delete quota for volume name : '%s', export ID : '%d'", volName, exportID)

	var export isi.Export
	var err error
	var quotaID string

	if export, err = svc.client.GetExportByIDWithZone(context.Background(), exportID, accessZone); err != nil {
		return fmt.Errorf("failed to get export '%s':'%d' with access zone '%s', skip DeleteQuotaByID. error : '%s'", volName, exportID, accessZone, err.Error())
	}

	log.Debugf("export (id : '%d') corresponding to path '%s' found, description field is '%s'", export.ID, volName, export.Description)

	if quotaID, err = utils.GetQuotaIDFromDescription(export); err != nil {
		return err
	}

	if quotaID == "" {
		log.Debugf("No quota set on the volume, skip deleting quota")
		return nil
	}

	log.Debugf("deleting quota with id '%s' for path '%s'", quotaID, volName)

	if err = svc.client.ClearQuotaByID(context.Background(), quotaID); err != nil {
		return err
	}

	return nil
}

func (svc *isiService) GetVolumeQuota(volName string, exportID int, accessZone string) (isi.Quota, error) {
	log.Debugf("begin to get quota for volume name : '%s', export ID : '%d'", volName, exportID)

	var export isi.Export
	var err error
	var quotaID string

	if export, err = svc.client.GetExportByIDWithZone(context.Background(), exportID, accessZone); err != nil {
		return nil, fmt.Errorf("failed to get export '%s':'%d' with access zone '%s', error: '%s'", volName, exportID, accessZone, err.Error())
	}

	log.Debugf("export (id : '%d') corresponding to path '%s' found, description field is '%s'", export.ID, volName, export.Description)

	if quotaID, err = utils.GetQuotaIDFromDescription(export); err != nil {
		return nil, err
	}

	if quotaID == "" {
		log.Debugf("No quota set on the volume")
		return nil, fmt.Errorf("failed to get quota: No quota set on the volume '%s'", volName)
	}

	log.Debugf("get quota by id '%s'", quotaID)
	return svc.client.GetQuotaByID(context.Background(), quotaID)
}

func (svc *isiService) UpdateQuotaSize(quotaID string, updatedSize int64) error {

	log.Debugf("updating quota by id '%s' with size '%d'", quotaID, updatedSize)

	if err := svc.client.UpdateQuotaSizeByID(context.Background(), quotaID, updatedSize); err != nil {
		return fmt.Errorf("failed to update quota '%s' with size '%d', error: '%s'", quotaID, updatedSize, err.Error())
	}

	return nil
}

func (svc *isiService) UnexportByIDWithZone(exportID int, accessZone string) error {
	log.Debugf("begin to unexport NFS export with ID '%d' in access zone '%s'", exportID, accessZone)

	if err := svc.client.UnexportByIDWithZone(context.Background(), exportID, accessZone); err != nil {
		return fmt.Errorf("failed to unexport volume directory '%d' in access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}

	return nil
}

func (svc *isiService) GetExportsWithParams(params api.OrderedValues) (isi.Exports, error) {
	log.Debugf("begin to get exports with params..")
	var exports isi.Exports
	var err error

	if exports, err = svc.client.GetExportsWithParams(context.Background(), params); err != nil {
		return nil, fmt.Errorf("failed to get exports with params")
	}
	return exports, nil
}

func (svc *isiService) DeleteVolume(isiPath, volName string) error {
	log.Debugf("begin to delete volume directory '%s'", volName)

	if err := svc.client.DeleteVolumeWithIsiPath(context.Background(), isiPath, volName); err != nil {
		return fmt.Errorf("failed to delete volume directory '%v' : '%v'", volName, err)
	}

	return nil
}

func (svc *isiService) ClearQuotaByID(quotaID string) error {
	if quotaID != "" {
		if err := svc.client.ClearQuotaByID(context.Background(), quotaID); err != nil {
			return fmt.Errorf("failed to clear quota for '%s' : '%v'", quotaID, err)
		}
	}
	return nil
}

func (svc *isiService) TestConnection() error {
	log.Debugf("test connection client, user name : '%s'", svc.client.API.User())
	if _, err := svc.client.GetClusterConfig(context.Background()); err != nil {
		log.Errorf("error encountered, test connection failed : '%v'", err)
		return err
	}

	log.Debug("test connection succeeded")

	return nil
}

func (svc *isiService) GetNFSExportURLForPath(ip string, dirPath string) string {
	return fmt.Sprintf("%s:%s", ip, dirPath)
}

func (svc *isiService) GetVolume(isiPath, volID, volName string) (isi.Volume, error) {
	log.Debugf("begin getting volume with id '%s' and name '%s' for Isilon", volID, volName)

	var vol isi.Volume
	var err error
	if vol, err = svc.client.GetVolumeWithIsiPath(context.Background(), isiPath, volID, volName); err != nil {
		log.Errorf("failed to get volume '%s'", err)
		return nil, err
	}

	return vol, nil
}

func (svc *isiService) GetVolumeSize(isiPath, name string) int64 {
	log.Debugf("begin getting volume size with name '%s' for Isilon", name)

	size, err := svc.client.GetVolumeSize(context.Background(), isiPath, name)
	if err != nil {
		log.Errorf("failed to get volume size '%s'", err.Error())
		return 0
	}

	return size
}

func (svc *isiService) GetStatistics(keys []string) (isi.Stats, error) {
	var stat isi.Stats
	var err error
	if stat, err = svc.client.GetStatistics(context.Background(), keys); err != nil {
		log.Errorf("failed to get array statistics '%s'", err)
		return nil, err
	}
	fmt.Printf("Available capacity: %+v\n", stat.StatsList[0])
	return stat, nil
}

func (svc *isiService) IsVolumeExistent(isiPath, volID, name string) bool {
	log.Debugf("check if volume (id :'%s', name '%s') already exists", volID, name)

	isExistent := svc.client.IsVolumeExistentWithIsiPath(context.Background(), isiPath, volID, name)

	log.Debugf("volume (id :'%s', name '%s') already exists : '%v'", volID, name, isExistent)

	return isExistent
}

func (svc *isiService) OtherClientsAlreadyAdded(exportID int, accessZone string, clientIP string) bool {
	export, _ := svc.GetExportByIDWithZone(exportID, accessZone)

	if export == nil {
		log.Debugf("failed to get export by id '%d' with access zone '%s', return true for otherClientsAlreadyAdded as a safer return value", exportID, accessZone)
		return true
	}

	clientFieldsNotEmpty := len(*export.Clients) > 0 || len(*export.ReadOnlyClients) > 0 || len(*export.ReadWriteClients) > 0 || len(*export.RootClients) > 0

	isNodeInClientFields := utils.IsStringInSlices(clientIP, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)

	fqdn, _ := utils.GetFQDNByIP(clientIP)

	if fqdn != "" {
		isNodeInClientFields = isNodeInClientFields || utils.IsStringInSlices(fqdn, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
	}

	return clientFieldsNotEmpty && !isNodeInClientFields
}

func (svc *isiService) AddExportClientNetworkIdentifierByIDWithZone(exportID int, accessZone, clientIP string, addClientFunc func(exportID int, accessZone, clientIP string) error) error {
	log.Debugf("AddExportClientNetworkIdentifierByID client ip '%s'", clientIP)

	var fqdn string
	if net.ParseIP(clientIP) != nil {
		fqdn, _ = utils.GetFQDNByIP(clientIP)
		log.Debugf("ip '%s' is resolved to '%s'", clientIP, fqdn)
	} else {
		fqdn = clientIP
		log.Debugf("fqdn is '%s'", fqdn)
	}

	if fqdn != "" {

		// try adding by FQDN first as FQDN is preferred over IP for its stableness. OneFS API will return error if it cannot resolve the FQDN,
		// in that case, fall back to adding by IP
		var err error
		if err = addClientFunc(exportID, accessZone, fqdn); err == nil {

			//adding by FQDN is successful, no need to trying adding by IP
			return nil
		}

		log.Errorf("failed to add client fqdn '%s' to export id '%d' : '%v', try using client ip '%s'", fqdn, exportID, err, clientIP)
	}

	if err := addClientFunc(exportID, accessZone, clientIP); err != nil {
		return fmt.Errorf("failed to add client ip '%s' to export id '%d' : '%v'", clientIP, exportID, err)
	}

	return nil
}

func (svc *isiService) AddExportClientByIDWithZone(exportID int, accessZone, clientIP string) error {
	log.Debugf("AddExportClientByID client '%s'", clientIP)
	if err := svc.client.AddExportClientsByIDWithZone(context.Background(), exportID, accessZone, []string{clientIP}); err != nil {
		return fmt.Errorf("failed to add client to export id '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}
	return nil
}

func (svc *isiService) AddExportRootClientByIDWithZone(exportID int, accessZone, clientIP string) error {
	log.Debugf("AddExportRootClientByID client '%s'", clientIP)
	if err := svc.client.AddExportRootClientsByIDWithZone(context.Background(), exportID, accessZone, []string{clientIP}); err != nil {
		return fmt.Errorf("failed to add client to export id '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}
	return nil
}

func (svc *isiService) AddExportReadOnlyClientByIDWithZone(exportID int, accessZone, clientIP string) error {
	log.Debugf("AddExportReadOnlyClientByID client '%s'", clientIP)
	if err := svc.client.AddExportReadOnlyClientsByIDWithZone(context.Background(), exportID, accessZone, []string{clientIP}); err != nil {
		return fmt.Errorf("failed to add read only client to export id '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}
	return nil
}

func (svc *isiService) RemoveExportClientByIDWithZone(exportID int, accessZone, clientIP string) error {
	// it could either be IP or FQDN that has been added to the export's client fields, should consider both during the removal
	fqdn, _ := utils.GetFQDNByIP(clientIP)
	clientsToRemove := []string{clientIP, fqdn}

	log.Debugf("RemoveExportClientByName client IP '%v'", clientsToRemove)

	if err := svc.client.RemoveExportClientsByIDWithZone(context.Background(), exportID, accessZone, clientsToRemove); err != nil {
		//Return success if export doesn't exist
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

func (svc *isiService) GetExportsWithLimit(limit string) (isi.ExportList, string, error) {
	log.Debug("begin getting exports for Isilon")
	var exports isi.Exports
	var err error
	if exports, err = svc.client.GetExportsWithLimit(context.Background(), limit); err != nil {
		log.Error("failed to get exports")
		return nil, "", err
	}
	return exports.Exports, exports.Resume, nil
}

func (svc *isiService) GetExportsWithResume(resume string) (isi.ExportList, string, error) {
	log.Debug("begin getting exports for Isilon")
	var exports isi.Exports
	var err error
	if exports, err = svc.client.GetExportsWithResume(context.Background(), resume); err != nil {
		log.Error("failed to get exports: " + err.Error())
		return nil, "", err
	}
	return exports.Exports, exports.Resume, nil
}

func (svc *isiService) DeleteSnapshot(id int64, name string) error {
	log.Debugf("begin to delete snapshot '%s'", name)
	if err := svc.client.RemoveSnapshot(context.Background(), id, name); err != nil {
		log.Errorf("delete snapshot failed, '%s'", err.Error())
		return err
	}
	return nil
}

func (svc *isiService) GetSnapshot(idendity string) (isi.Snapshot, error) {
	log.Debugf("begin getting snapshot with id|name '%s' for Isilon", idendity)
	var snapshot isi.Snapshot
	var err error
	if snapshot, err = svc.client.GetIsiSnapshotByIdentity(context.Background(), idendity); err != nil {
		log.Errorf("failed to get snapshot '%s'", err.Error())
		return nil, err
	}

	return snapshot, nil
}

func (svc *isiService) GetSnapshotSize(isiPath, name string) int64 {
	log.Debugf("begin getting snapshot size with name '%s' for Isilon", name)
	size, err := svc.client.GetSnapshotFolderSize(context.Background(), isiPath, name)
	if err != nil {
		log.Errorf("failed to get snapshot size '%s'", err.Error())
		return 0
	}

	return size
}

func (svc *isiService) GetExportWithPathAndZone(path, accessZone string) (isi.Export, error) {
	log.Debugf("begin getting export with target path '%s' and access zone '%s' for Isilon", path, accessZone)
	var export isi.Export
	var err error
	if export, err = svc.client.GetExportWithPathAndZone(context.Background(), path, accessZone); err != nil {
		log.Error("failed to get export with target path '" + path + "' and access zone '" + accessZone + "': '" + err.Error() + "'")
		return nil, err
	}

	return export, nil
}
