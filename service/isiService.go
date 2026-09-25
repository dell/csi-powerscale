// Copyright © 2019-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package service

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
	apiv1 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v1"
	apiv2 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v2"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	id "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/identifiers"
	isilonfs "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/powerscale-fs"
	strutil "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/string-utils"

	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	"github.com/Ecosystems/container-storage-modules/src/gopowerscale/api"
)

type isiService struct {
	endpoint string
	client   *isi.Client
}

// resumeableContainerChildList is used for paginated directory listing with resume tokens
type resumeableContainerChildList struct {
	Children []*apiv2.ContainerChild `json:"children"`
	Resume   string                  `json:"resume,omitempty"`
}

// CreateWritableSnapshot creates a writable snapshot from a source snapshot.
// Parameters:
//   - isiPath: Base IsiPath for volume lookup after creation
//   - destinationPath: Full destination path for the writable snapshot
//   - sourceSnapshot: Source snapshot ID
//   - volumeName: Volume name for post-creation lookup
//   - accessZone: (reserved) Access zone parameter - reserved for future multi-zone writable snapshot support
//
// Note: The accessZone parameter is currently unused but reserved for future functionality.
// It is maintained for:
// 1. Function-variable signature compatibility with the indirection pattern in controller.go (createWritableSnapshotFunc)
// 2. Future support for multi-access-zone writable snapshots when OneFS API adds zone-scoped writable snapshot operations
// 3. Consistency with other isiService methods that accept accessZone (CopySnapshot, GetSnapshotSize, etc.)
//
// The underlying gopowerscale client (v1.22.1+) only requires destinationPath and sourceSnapshot for the
// POST /platform/14/snapshot/writable API call. If OneFS adds zone-aware writable snapshot APIs in future
// releases, this parameter will be used without requiring a breaking signature change.
func (svc *isiService) CreateWritableSnapshot(ctx context.Context, isiPath string, destinationPath string, sourceSnapshot string, volumeName string, _ string) (isi.Volume, error) {
	csmlog.WithContext(ctx).Debugf("begin to create writable snapshot from source snapshot: '%s'", sourceSnapshot)

	ws, err := svc.client.CreateWritableSnapshot(ctx, destinationPath, sourceSnapshot)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("create writable snapshot failed, '%s'", err.Error())
		return nil, err
	}

	csmlog.WithContext(ctx).Debugf("writable snapshot created successfully, DstPath: '%s', State: '%s'", ws.DstPath, ws.State)

	// Return a Volume-compatible result using the writable snapshot's destination path
	vol, err := svc.client.GetVolumeWithIsiPath(ctx, isiPath, "", volumeName)
	if err != nil {
		csmlog.WithContext(ctx).Debugf("writable snapshot volume lookup failed: '%v'", err)
		return nil, err
	}

	return vol, nil
}

func (svc *isiService) GetWritableSnapshot(ctx context.Context, dstPath string) (isi.WritableSnapshot, error) {
	csmlog.WithContext(ctx).Debugf("begin to get writable snapshot at path: '%s'", dstPath)

	ws, err := svc.client.GetWritableSnapshot(ctx, dstPath)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("get writable snapshot failed, '%s'", err.Error())
		return nil, err
	}

	return ws, nil
}

func (svc *isiService) DeleteWritableSnapshot(ctx context.Context, dstPath string) error {
	csmlog.WithContext(ctx).Debugf("begin to delete writable snapshot at path: '%s'", dstPath)

	if err := svc.client.DeleteWritableSnapshot(ctx, dstPath); err != nil {
		csmlog.WithContext(ctx).Errorf("delete writable snapshot failed, '%s'", err.Error())
		return err
	}

	return nil
}

func (svc *isiService) CopySnapshot(ctx context.Context, isiPath, snapshotSourceVolumeIsiPath string, srcSnapshotID int64, dstVolumeName string, accessZone string) (isi.Volume, error) {
	csmlog.WithContext(ctx).Debugf("begin to copy snapshot '%d'", srcSnapshotID)

	var volumeNew isi.Volume
	var err error
	if volumeNew, err = svc.client.CopySnapshotWithIsiPath(ctx, isiPath, snapshotSourceVolumeIsiPath, srcSnapshotID, "", dstVolumeName, accessZone); err != nil {
		csmlog.WithContext(ctx).Errorf("copy snapshot failed, '%s'", err.Error())
		return nil, err
	}

	return volumeNew, nil
}

func (svc *isiService) CopyVolume(ctx context.Context, isiPath, srcVolumeName, dstVolumeName string) (isi.Volume, error) {
	csmlog.WithContext(ctx).Debugf("begin to copy volume '%s'", srcVolumeName)

	var volumeNew isi.Volume
	var err error
	if volumeNew, err = svc.client.CopyVolumeWithIsiPath(ctx, isiPath, srcVolumeName, dstVolumeName); err != nil {
		csmlog.WithContext(ctx).Errorf("copy volume failed, '%s'", err.Error())
		return nil, err
	}

	return volumeNew, nil
}

func (svc *isiService) CreateSnapshot(ctx context.Context, path, snapshotName string) (isi.Snapshot, error) {
	csmlog.WithContext(ctx).Debugf("begin to create snapshot '%s'", snapshotName)

	var snapshot isi.Snapshot
	var err error
	if snapshot, err = svc.client.CreateSnapshotWithPath(ctx, path, snapshotName); err != nil {
		csmlog.WithContext(ctx).Errorf("create snapshot failed, '%s'", err.Error())
		return nil, err
	}

	return snapshot, nil
}

func (svc *isiService) GetLicenseByID(ctx context.Context, licenseID string) (isi.License, error) {
	csmlog.WithContext(ctx).Debugf("begin to get license by id '%s'", licenseID)

	license, err := svc.client.GetLicenseByID(ctx, licenseID)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get license by id '%s': %s", licenseID, err.Error())
		return nil, err
	}

	return license, nil
}

func (svc *isiService) CreateVolume(ctx context.Context, isiPath, volName, isiVolumePathPermissions string) error {
	csmlog.WithContext(ctx).Debugf("begin to create volume '%s'", volName)

	if _, err := svc.client.CreateVolumeWithIsipath(ctx, isiPath, volName, isiVolumePathPermissions); err != nil {
		csmlog.WithContext(ctx).Errorf("create volume failed, '%s'", err.Error())
		return err
	}
	return nil
}

func (svc *isiService) CreateVolumeWithMetaData(ctx context.Context, isiPath, volName, isiVolumePathPermissions string, metadata map[string]string) error {
	csmlog.WithContext(ctx).Debugf("begin to create volume '%s'", volName)
	csmlog.WithContext(ctx).Debugf("header metadata '%v'", metadata)

	if _, err := svc.client.CreateVolumeWithIsipathMetaData(ctx, isiPath, volName, isiVolumePathPermissions, metadata); err != nil {
		csmlog.WithContext(ctx).Errorf("create volume failed, '%s'", err.Error())
		return err
	}
	return nil
}

// SetVolumeGroupOwnershipByPath sets the group ownership and permission mode of a
// directory-backed volume subdirectory via the OneFS management API (ACL).
//
// The directory is located at <isiPath>/<name>. The group is set to the supplied
// GID and the mode to 2770 (setgid + group rwx, no world access), which allows a
// pod running with the matching fsGroup to read/write while preserving isolation
// between directories that use distinct fsGroups.
//
// SECURITY NOTE: For RWX (ReadWriteMany) volumes, all pods sharing the same PVC
// should use identical fsGroup values to avoid access control conflicts. Different
// fsGroups will cause the last-mounted pod to overwrite directory ownership,
// potentially breaking access for other pods and creating security vulnerabilities.
//
// This call is issued on the management plane (authenticated as the OneFS service
// account) and is therefore NOT subject to NFS root_squash — it succeeds even when
// RootClientEnabled is "false". It must never be replaced by a node-side chown over
// the NFS mount, which would require disabling root_squash.

// SetVolumeGroupOwnershipResult indicates whether the directory group was changed
// and can be used to decide whether recursive ownership updates are needed.
type SetVolumeGroupOwnershipResult struct {
	// Changed is true when the directory ACL was actually updated.
	Changed bool
	// ExistingGID is the group that was already configured, if any.
	ExistingGID string
}

func (svc *isiService) SetVolumeGroupOwnershipByPath(ctx context.Context, isiPath, name string, gid int, isClone bool) (*SetVolumeGroupOwnershipResult, error) {
	csmlog.WithContext(ctx).Debugf("setting group ownership gid=%d mode=2770 on '%s/%s'", gid, isiPath, name)

	result := &SetVolumeGroupOwnershipResult{Changed: false}

	// Check for existing ACL to prevent fsGroup conflicts (security enforcement).
	// Use the volume's real path (isiPath/name) instead of the client's default volumesPath.
	var existingACL apiv2.ACL
	if err := svc.client.API.Get(
		ctx,
		apiv1.GetRealNamespacePathWithIsiPath(isiPath),
		name,
		api.OrderedValues{{[]byte("acl")}},
		nil,
		&existingACL,
	); err == nil && existingACL.Group != nil && existingACL.Group.ID != nil {
		existingGID := existingACL.Group.ID.ID
		result.ExistingGID = existingGID
		if existingGID != strconv.Itoa(gid) && existingGID != "0" {
			// Allow ownership change for cloned volumes (separate volume, not shared RWX mount)
			if isClone {
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"volumePath":   fmt.Sprintf("%s/%s", isiPath, name),
					"existingGID":  existingGID,
					"requestedGID": gid,
					"reason":       "clone_volume_ownership_change",
				}).Debug("Allowing fsGroup change for cloned volume - this is a separate volume, not a shared RWX mount")
				result.Changed = true
			} else {
				// Block conflicting fsGroup to protect existing pods (fail-secure)
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"volumePath":   fmt.Sprintf("%s/%s", isiPath, name),
					"existingGID":  existingGID,
					"requestedGID": gid,
					"action":       "mount_blocked",
				}).Error("fsGroup conflict detected - mount blocked to prevent security vulnerability. " +
					"All pods using the same RWX volume must use identical fsGroup values. " +
					"Change pod's fsGroup to match existing value or use a different PVC.")

				return nil, fmt.Errorf("fsGroup conflict: volume already configured for fsGroup %s, requested fsGroup %d. "+
					"For ReadWriteMany volumes, all pods must use the same fsGroup. "+
					"Either change the pod's fsGroup to %s or use a separate PVC",
					existingGID, gid, existingGID)
			}
		}

		if existingGID == strconv.Itoa(gid) {
			// Group already matches, no need to update
			csmlog.WithContext(ctx).WithFields(csmlog.Fields{
				"volumePath": fmt.Sprintf("%s/%s", isiPath, name),
				"fsGroup":    gid,
			}).Debug("Volume directory already has target fsGroup, skipping directory ownership update")
			return result, nil
		}

		// Existing group is 0 (root/inherited); treat as unset and allow ownership update.
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"volumePath":   fmt.Sprintf("%s/%s", isiPath, name),
			"existingGID":  existingGID,
			"requestedGID": gid,
		}).Debug("Existing directory group is root (0), treating as unset and applying requested fsGroup")
	}

	mode := apiv2.FileMode(0o2770)
	acl := &apiv2.ACL{
		Action:        &apiv2.PActionTypeReplace,
		Authoritative: &apiv2.PAuthoritativeTypeMode,
		Group: &apiv2.Persona{
			ID: &apiv2.PersonaID{
				ID:   strconv.Itoa(gid),
				Type: apiv2.PersonaIDTypeGID,
			},
		},
		Mode: &mode,
	}

	if err := svc.client.API.Put(
		ctx,
		apiv1.GetRealNamespacePathWithIsiPath(isiPath),
		name,
		api.OrderedValues{{[]byte("acl")}},
		nil,
		acl,
		nil,
	); err != nil {
		return nil, fmt.Errorf("failed to set group ownership (gid=%d) on '%s/%s': %w", gid, isiPath, name, err)
	}

	result.Changed = true
	return result, nil
}

// SetVolumeGroupOwnershipRecursive applies fsGroup ownership to all files and directories
// within a directory-backed volume. This is needed for volumes created from snapshots or
// clones, where copied files retain the source ownership and must be updated to match
// the target pod's fsGroup.
//
// Uses the OneFS management API to recursively enumerate and update ACLs for all paths.
// Performance: O(number of files/directories). Optimized with pagination limit=1000 and
// a fixed worker pool for bounded concurrency.
// Typical performance: ~1-2 seconds for 100 files, ~1-2 minutes for 10,000 files.
//
// This function should only be called when necessary (e.g., first mount of a cloned volume
// with a different fsGroup than the source). The caller should check if the directory
// ownership changed before calling this to avoid redundant expensive scans.
//
// Parameters:
//   - workerCount: Number of parallel workers for ownership updates (configurable via X_CSI_ISILON_CHOWN_WORKERS, default 8)
func (svc *isiService) SetVolumeGroupOwnershipRecursive(ctx context.Context, isiPath, name string, gid int, workerCount int) error {
	volumePath := fmt.Sprintf("%s/%s", isiPath, name)
	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		"volumePath": volumePath,
		"gid":        gid,
		"operation":  "recursive_ownership_fix",
	}).Info("Starting recursive group ownership update for volume created from snapshot/clone")

	parts := strings.SplitN(volumePath, "/", 3)
	if len(parts) < 3 {
		return fmt.Errorf("invalid volume path format: %s", volumePath)
	}
	baseIsiPath := parts[1]
	relativePath := parts[2]
	namespacePath := path.Join("namespace", baseIsiPath)

	var allChildren []*apiv2.ContainerChild
	detailFields := []string{
		"type",
		"container_path",
		"size",
		"mode",
		"owner",
		"group",
		"name",
	}
	qs := api.OrderedValues{
		{[]byte("query")},
		{[]byte("limit"), []byte("1000")}, // Optimized from 2 to 1000 for better performance
		{[]byte("max-depth"), []byte("-1")},
	}
	detailParams := [][]byte{[]byte("detail")}
	for _, f := range detailFields {
		detailParams = append(detailParams, []byte(f))
	}
	qs = append(qs, detailParams)

	for {
		var resp resumeableContainerChildList
		err := svc.client.API.Get(
			ctx,
			namespacePath,
			relativePath,
			qs,
			nil,
			&resp,
		)
		if err != nil {
			return fmt.Errorf("failed to query volume children for recursive ownership: %w", err)
		}

		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"namespacePath": namespacePath,
			"relativePath":  relativePath,
			"childrenCount": len(resp.Children),
			"hasMore":       resp.Resume != "",
		}).Debug("Queried volume children batch")

		allChildren = append(allChildren, resp.Children...)

		if resp.Resume == "" {
			break
		}
		qs.Set([]byte("resume"), []byte(resp.Resume))
	}

	children := make(map[string]*apiv2.ContainerChild)
	for _, c := range allChildren {
		if c.Path != nil && c.Name != nil {
			children[path.Join(*c.Path, *c.Name)] = c
		}
	}

	if len(children) == 0 {
		csmlog.WithContext(ctx).Debug("No children found, recursive ownership update not needed")
		return nil
	}

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		"childCount":  len(children),
		"gid":         gid,
		"workerCount": workerCount,
	}).Info("Applying recursive group ownership to volume children")

	// Use a fixed worker pool to bound goroutine creation
	// workerCount is passed from the caller and matches the ChownWorkers setting (default 8)
	// used for export-backed volumes, configurable via X_CSI_ISILON_CHOWN_WORKERS
	type workItem struct {
		path        string
		isDirectory bool
	}
	workChan := make(chan workItem, len(children))
	errorChan := make(chan error, len(children))

	// Start exactly workerCount goroutines (not one per file)
	for i := 0; i < workerCount; i++ {
		go func() {
			for item := range workChan {
				if err := svc.setPathGroupOwnership(ctx, item.path, gid, item.isDirectory); err != nil {
					csmlog.WithContext(ctx).WithFields(csmlog.Fields{
						"childPath":   item.path,
						"gid":         gid,
						"isDirectory": item.isDirectory,
						"error":       err,
					}).Warn("Failed to set group ownership on child path")
					errorChan <- fmt.Errorf("failed to set group ownership on %s: %w", item.path, err)
				} else {
					errorChan <- nil
				}
			}
		}()
	}

	// Dispatch work to the worker pool
	for childPath, child := range children {
		// Determine if this is a directory based on the ContainerChild.Type field
		// from the OneFS API. Type values: "container" = directory, "object"/"file" = regular file
		isDirectory := child.Type != nil && *child.Type == "container"
		workChan <- workItem{childPath, isDirectory}
	}
	close(workChan)

	// Collect results
	var errors []error
	for range children {
		if err := <-errorChan; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"errorCount": len(errors),
			"childCount": len(children),
		}).Error("Some recursive ownership updates failed")
		return fmt.Errorf("recursive ownership failed for %d/%d files: %v", len(errors), len(children), errors[0])
	}

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		"childCount": len(children),
		"gid":        gid,
	}).Info("Successfully completed recursive group ownership update")

	return nil
}

// setPathGroupOwnership is a helper that updates the group ownership on a single path.
// The path must be an absolute OneFS path starting with /ifs/.
// The isDirectory parameter determines the permission mode:
//   - Directories: 2770 (rwxrwx--- + setgid)
//   - Files: 0660 (rw-rw----)
//
// This matches the node-side applyFSGroupPermissions behavior and kubelet's documented
// fsGroup behavior.
func (svc *isiService) setPathGroupOwnership(ctx context.Context, fullPath string, gid int, isDirectory bool) error {
	parts := strings.SplitN(fullPath, "/", 3)
	if len(parts) < 3 {
		return fmt.Errorf("invalid path for group ownership update: %s", fullPath)
	}
	namespacePath := path.Join("namespace", parts[1])
	relativePath := parts[2]

	// Set appropriate mode based on entry type:
	// - Directories: 2770 (rwxrwx--- + setgid) - allows traversal and setgid for group inheritance
	// - Files: 0660 (rw-rw----) - read/write for owner and group, no execute
	var mode apiv2.FileMode
	if isDirectory {
		mode = apiv2.FileMode(0o2770)
	} else {
		mode = apiv2.FileMode(0o0660)
	}

	acl := &apiv2.ACL{
		Action:        &apiv2.PActionTypeReplace,
		Authoritative: &apiv2.PAuthoritativeTypeMode,
		Group: &apiv2.Persona{
			ID: &apiv2.PersonaID{
				ID:   strconv.Itoa(gid),
				Type: apiv2.PersonaIDTypeGID,
			},
		},
		Mode: &mode,
	}

	if err := svc.client.API.Put(
		ctx,
		namespacePath,
		relativePath,
		api.OrderedValues{{[]byte("acl")}},
		nil,
		acl,
		nil,
	); err != nil {
		return fmt.Errorf("failed to set group ownership on path %s: %w", fullPath, err)
	}

	return nil
}

func (svc *isiService) GetExports(ctx context.Context) (isi.ExportList, error) {
	csmlog.WithContext(ctx).Debug("begin getting exports for Isilon")

	var exports isi.ExportList
	var err error
	if exports, err = svc.client.GetExports(ctx); err != nil {
		csmlog.WithContext(ctx).Error("failed to get exports")
		return nil, err
	}

	return exports, nil
}

func (svc *isiService) GetExportByIDWithZone(ctx context.Context, exportID int, accessZone string) (isi.Export, error) {
	csmlog.WithContext(ctx).Debugf("begin getting export by id '%d' with access zone '%s' for Isilon", exportID, accessZone)

	var export isi.Export
	var err error
	if export, err = svc.client.GetExportByIDWithZone(ctx, exportID, accessZone); err != nil {
		csmlog.WithContext(ctx).Error("failed to get export by id with access zone")
		return nil, err
	}

	return export, nil
}

func (svc *isiService) GetExportsCountAttachedToNode(ctx context.Context, nodeip string) (int64, error) {
	csmlog.WithContext(ctx).Debugf("begin getting export count for nodeip '%s' for Isilon", nodeip)
	var count int64
	var err error
	if count, err = svc.client.GetExportsCountAttachedToNode(ctx, nodeip); err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get export count for node ip %s err %s", nodeip, err.Error())
		return 0, err
	}
	return count, nil
}

// GetExportsCountAttachedToNodeIPs returns the count of exports that have any
// of the provided node IPs in their client fields. Used in multi-NIC mode for
// accurate MaxVolumesPerNode checks across all NFS interfaces.
func (svc *isiService) GetExportsCountAttachedToNodeIPs(ctx context.Context, nodeIPs []string, accessZone string) (int64, error) {
	csmlog.WithContext(ctx).Debugf("begin getting export count for %d node IPs in zone '%s'", len(nodeIPs), accessZone)
	count, err := svc.client.GetExportsCountAttachedToNodeIPs(ctx, nodeIPs, accessZone)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get export count for node IPs %v err %s", nodeIPs, err.Error())
		return 0, err
	}
	return int64(count), nil
}

func (svc *isiService) ExportVolumeWithZone(ctx context.Context, isiPath, volName, accessZone, description string) (int, error) {
	return svc.ExportVolumeWithZoneAndXprtsec(ctx, isiPath, volName, accessZone, description, "")
}

// ExportVolumeWithZoneAndXprtsec exports a volume with transport security policy (xprtsec).
// The xprtsec parameter controls NFS over TLS enforcement at the PowerScale export level.
//
// Parameters:
//   - isiPath: Base path on PowerScale (e.g., "/ifs/data")
//   - volName: Volume name to export
//   - accessZone: PowerScale access zone (e.g., "System")
//   - description: Export description (e.g., "k8s_pvc_<uuid>")
//   - xprtsec: Transport security policy (empty string uses cluster default)
//
// Valid xprtsec values:
//   - "" (empty): Use cluster default (backward compatible, usually "none:tls:mtls")
//   - "mtls": mTLS-only export (defense-in-depth security)
//   - "tls": TLS-only export (no plaintext, no mTLS)
//   - "none": Plaintext-only export
//   - "tls:mtls": TLS or mTLS (no plaintext)
//
// Requires OneFS 9.16.0+ (PAPI v27) for xprtsec support.
// On older OneFS versions, xprtsec is ignored and cluster default is used.
func (svc *isiService) ExportVolumeWithZoneAndXprtsec(ctx context.Context, isiPath, volName, accessZone, description, xprtsec string) (int, error) {
	csmlog.WithContext(ctx).Debugf("begin to export volume '%s' with access zone '%s' in Isilon path '%s', xprtsec '%s'", volName, accessZone, isiPath, xprtsec)

	var exportID int
	var err error

	path := isilonfs.GetPathForVolume(isiPath, volName)

	// Validate cluster TLS mode if xprtsec is specified
	// SECURITY CRITICAL: When TLS/mTLS is explicitly requested, we MUST fail if the array
	// does not support it. This prevents plaintext fallback when encrypted transport was requested.
	if xprtsec != "" {
		if err := ValidateClusterTLSMode(ctx, svc.client, xprtsec); err != nil {
			csmlog.WithContext(ctx).Errorf("Cluster TLS mode validation failed for xprtsec '%s': %v. "+
				"Export creation aborted to prevent plaintext fallback when TLS/mTLS was explicitly requested.", xprtsec, err)
			return -1, fmt.Errorf("cluster TLS mode validation failed: %w", err)
		}
	}

	// Create export with xprtsec (OneFS 9.16.0+) or without (backward compatible)
	if exportID, err = svc.client.ExportVolumeWithZoneAndPathAndXprtsec(ctx, path, accessZone, description, xprtsec); err != nil {
		csmlog.WithContext(ctx).Errorf("Export volume failed, volume '%s', access zone '%s', xprtsec '%s', id %d error '%s'", volName, accessZone, xprtsec, exportID, err.Error())
		return -1, err
	}

	if xprtsec != "" {
		csmlog.WithContext(ctx).Infof("Exported volume '%s' successfully with xprtsec '%s', id '%d'", volName, xprtsec, exportID)
	} else {
		csmlog.WithContext(ctx).Infof("Exported volume '%s' successfully (cluster default xprtsec), id '%d'", volName, exportID)
	}

	return exportID, nil
}

func (svc *isiService) CreateQuota(ctx context.Context, path, volName, softLimit, advisoryLimit, softGracePrd string, sizeInBytes int64, quotaEnabled bool) (string, error) {
	csmlog.WithContext(ctx).Debugf("begin to create quota for '%s', size '%d', quota enabled: '%t'", volName, sizeInBytes, quotaEnabled)
	var softi, advisoryi int64
	var err error
	var softlimitInt, advisoryLimitInt, softGracePrdInt int64
	softGracePrdInt, err = strconv.ParseInt(softGracePrd, 10, 64)
	if err != nil {
		csmlog.WithContext(ctx).Debugf("Invalid softGracePrd value. Setting it to default.")
		softGracePrdInt = 0
	}
	// converting soft limit from %ge to value
	if softLimit != "" {
		softi, err = strconv.ParseInt(softLimit, 10, 64)
		if err != nil {
			csmlog.WithContext(ctx).Debugf("Invalid softLimit value. Setting it to default.")
			softlimitInt = 0
		} else {
			softlimitInt = (softi * sizeInBytes) / 100
		}
	}
	if advisoryLimit != "" {
		advisoryi, err = strconv.ParseInt(advisoryLimit, 10, 64)
		if err != nil {
			csmlog.WithContext(ctx).Debugf("Invalid advisoryLimit value. Setting it to default.")
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
			csmlog.WithContext(ctx).Debugf("SmartQuotas is enabled, but storage size is not requested, skip creating quotas for volume '%s'", volName)
			return "", nil
		}
		// Check if soft and advisory < 100
		if (softlimitInt >= sizeInBytes) || (advisoryLimitInt >= sizeInBytes) {
			csmlog.WithContext(ctx).Warnf("Soft and advisory thresholds must be smaller than the hard threshold. Setting it to default for Volume '%s'", volName)
			softlimitInt, advisoryLimitInt, softGracePrdInt = 0, 0, 0
		}
		// Check if Soft Grace period is set along with soft limit
		if (softlimitInt != 0) && (softGracePrdInt == 0) {
			csmlog.WithContext(ctx).Warnf("Soft Grace Period must be configured along with Soft threshold, Setting it to default for Volume '%s'", volName)
			softlimitInt, softGracePrdInt = 0, 0
		}

		isQuotaActivated, checkLicErr := svc.client.IsQuotaLicenseActivated(ctx)
		if checkLicErr != nil {
			csmlog.WithContext(ctx).Errorf("failed to check SmartQuotas license info: '%v'", checkLicErr)
		}

		if (!isQuotaActivated) && (checkLicErr == nil) {
			csmlog.WithContext(ctx).Debugf("SmartQuotas is not activated, cannot add capacity limit '%d' bytes via quota, skip creating quota", sizeInBytes)
			return "", nil
		}

		// create quota with container set to true
		var quotaID string
		var err error
		if quotaID, err = svc.client.CreateQuotaWithPath(ctx, path, true, sizeInBytes, softlimitInt, advisoryLimitInt, softGracePrdInt); err != nil {
			if isQuotaActivated && (checkLicErr == nil) {
				return "", fmt.Errorf("SmartQuotas is activated, but creating quota failed with error: '%v'", err)
			}

			// if checkLicErr != nil, then it's uncertain whether creating quota failed because SmartQuotas license is not activated, or it failed with some other reason
			return "", fmt.Errorf("creating quota failed with error, it might or might not be because SmartQuotas license has not been activated: '%v'", err)
		}

		csmlog.WithContext(ctx).Infof("quota set to: %d on directory: '%s'", sizeInBytes, volName)

		return quotaID, nil
	}

	csmlog.WithContext(ctx).Debugf("quota is disabled, skip creating quota for '%s'", volName)

	return "", nil
}

func (svc *isiService) DeleteQuotaByExportIDWithZone(ctx context.Context, volName string, exportID int, accessZone string) error {
	csmlog.WithContext(ctx).Debugf("begin to delete quota for volume name : '%s', export ID : '%d'", volName, exportID)

	var export isi.Export
	var err error
	var quotaID string

	if export, err = svc.client.GetExportByIDWithZone(ctx, exportID, accessZone); err != nil {
		return fmt.Errorf("failed to get export '%s':'%d' with access zone '%s', skip DeleteQuotaByID. error : '%s'", volName, exportID, accessZone, err.Error())
	}

	if export != nil {
		csmlog.WithContext(ctx).Debugf("export (id : '%d') corresponding to path '%s' found, description field is '%s'", export.ID, volName, export.Description)

		quotaID, _ = isilonfs.GetQuotaIDFromDescription(ctx, export)

		if quotaID == "" {
			csmlog.WithContext(ctx).Debugf("No quota set on the volume, skip deleting quota")
			return nil
		}

		csmlog.WithContext(ctx).Debugf("deleting quota with id '%s' for path '%s'", quotaID, volName)

		if err = svc.client.ClearQuotaByID(ctx, quotaID); err != nil {
			return err
		}

	}

	return nil
}

func (svc *isiService) GetVolumeQuota(ctx context.Context, volName string, exportID int, accessZone string) (isi.Quota, error) {
	csmlog.WithContext(ctx).Debugf("begin to get quota for volume name : '%s', export ID : '%d'", volName, exportID)

	var export isi.Export
	var err error
	var quotaID string

	if export, err = svc.client.GetExportByIDWithZone(ctx, exportID, accessZone); err != nil {
		return nil, fmt.Errorf("failed to get export '%s':'%d' with access zone '%s', error: '%s'", volName, exportID, accessZone, err.Error())
	}

	if export != nil {
		csmlog.WithContext(ctx).Debugf("export (id : '%d') corresponding to path '%s' found, description field is '%s'", export.ID, volName, export.Description)

		quotaID, err = isilonfs.GetQuotaIDFromDescription(ctx, export)

		if quotaID == "" {
			csmlog.WithContext(ctx).Debugf("No quota set on the volume")
			return nil, fmt.Errorf("failed to get quota: No quota set on the volume '%s'", volName)
		}

		csmlog.WithContext(ctx).Debugf("get quota by id '%s'", quotaID)
		return svc.client.GetQuotaByID(ctx, quotaID)

	}

	return nil, fmt.Errorf("failed to get quota for volume '%s'", volName)
}

// GetQuotaByPath retrieves quota information by directory path (for directory-backed volumes)
func (svc *isiService) GetQuotaByPath(ctx context.Context, volumePath string) (isi.Quota, error) {
	csmlog.WithContext(ctx).Debugf("begin to get quota for path: '%s'", volumePath)

	quota, err := svc.client.GetQuotaWithPath(ctx, volumePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get quota for path '%s': %w", volumePath, err)
	}

	return quota, nil
}

func (svc *isiService) UpdateQuotaSize(ctx context.Context, quotaID string, hardLimitSize int64,
	softLimitSize, advisoryLimitSize, softGracePrd int64,
) error {
	csmlog.WithContext(ctx).Debugf("begin updating quota '%s' with hard limit '%d'", quotaID, hardLimitSize)

	if err := svc.client.UpdateQuotaSizeByID(ctx, quotaID, hardLimitSize, softLimitSize, advisoryLimitSize, softGracePrd); err != nil {
		csmlog.WithContext(ctx).Errorf("update quota hard limit failed for '%s': '%s'", quotaID, err.Error())
		return err
	}

	return nil
}

// ModifyQuota modifies specific quota parameters using PATCH (CSI 1.12 ControllerModifyVolume)
// params is a map of parameter names to values (e.g., {"AdvisoryLimit": 1073741824})
// Only the specified parameters are modified; others remain unchanged
func (svc *isiService) ModifyQuota(ctx context.Context, quotaID string, params map[string]interface{}) error {
	csmlog.WithContext(ctx).Debugf("modifying quota by id '%s' with params: %v", quotaID, params)

	if err := svc.client.ModifyQuotaByID(ctx, quotaID, params); err != nil {
		return fmt.Errorf("failed to modify quota '%s', error: '%s'", quotaID, err.Error())
	}

	return nil
}

func (svc *isiService) UnexportByIDWithZone(ctx context.Context, exportID int, accessZone string) error {
	csmlog.WithContext(ctx).Debugf("begin to unexport NFS export with ID '%d' in access zone '%s'", exportID, accessZone)

	if err := svc.client.UnexportByIDWithZone(ctx, exportID, accessZone); err != nil {
		return fmt.Errorf("failed to unexport volume directory '%d' in access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}

	return nil
}

func (svc *isiService) GetExportsWithParams(ctx context.Context, params api.OrderedValues) (isi.Exports, error) {
	csmlog.WithContext(ctx).Debugf("begin to get exports with params..")
	var exports isi.Exports
	var err error

	if exports, err = svc.client.GetExportsWithParams(ctx, params); err != nil {
		return nil, fmt.Errorf("failed to get exports with params")
	}
	return exports, nil
}

func (svc *isiService) DeleteVolume(ctx context.Context, isiPath, volName string) error {
	csmlog.WithContext(ctx).Debugf("begin to delete volume directory '%s'", volName)

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
	csmlog.WithContext(ctx).Debugf("test connection client, user name : '%s'", svc.client.API.User())
	if _, err := svc.client.GetClusterConfig(ctx); err != nil {
		csmlog.WithContext(ctx).Errorf("error encountered, test connection failed : '%v'", err)
		return err
	}

	csmlog.WithContext(ctx).Debug("test connection succeeded")

	return nil
}

func (svc *isiService) GetNFSExportURLForPath(ip string, dirPath string) string {
	return fmt.Sprintf("%s:%s", ip, dirPath)
}

func (svc *isiService) GetVolumeWithIsiPath(ctx context.Context, isiPath, volID, volName string) (isi.Volume, error) {
	csmlog.WithContext(ctx).Debugf("begin getting volume with id '%s' and name '%s' for Isilon", volID, volName)

	var vol isi.Volume
	var err error
	if vol, err = svc.client.GetVolumeWithIsiPath(ctx, isiPath, volID, volName); err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get volume '%s'", err)
		return nil, err
	}

	return vol, nil
}

func (svc *isiService) GetVolume(ctx context.Context, volID, volName string) (isi.Volume, error) {
	csmlog.WithContext(ctx).Debugf("begin getting volume with name '%s' for Isilon", volName)

	var vol isi.Volume
	var err error
	if vol, err = svc.client.GetVolume(ctx, volID, volName); err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get volume '%s'", err)
		return nil, err
	}

	return vol, nil
}

func (svc *isiService) GetVolumeMetaData(ctx context.Context, fullPath string) (map[string]string, error) {
	cleanPath := path.Clean(fullPath)
	if cleanPath == "." || cleanPath == "/" {
		return nil, fmt.Errorf("invalid volume path '%s'", fullPath)
	}

	isiPath := path.Dir(cleanPath)
	volName := path.Base(cleanPath)

	vol, err := svc.GetVolumeWithIsiPath(ctx, isiPath, "", volName)
	if err != nil {
		return nil, err
	}

	metadata := make(map[string]string, len(vol.AttributeMap))
	for _, attr := range vol.AttributeMap {
		metadata[attr.Name] = fmt.Sprintf("%v", attr.Value)
	}

	return metadata, nil
}

func (svc *isiService) GetVolumeSize(ctx context.Context, isiPath, name string) int64 {
	csmlog.WithContext(ctx).Debugf("begin getting volume size with name '%s' for Isilon", name)

	size, err := svc.client.GetVolumeSize(ctx, isiPath, name)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get volume size '%s'", err.Error())
		return 0
	}

	return size
}

func (svc *isiService) GetStatistics(ctx context.Context, keys []string) (isi.Stats, error) {
	var stat isi.Stats
	var err error
	if stat, err = svc.client.GetStatistics(ctx, keys); err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get array statistics '%s'", err)
		return nil, err
	}
	return stat, nil
}

func (svc *isiService) IsIOInProgress(ctx context.Context) (isi.Clients, error) {
	var clients isi.Clients
	var err error
	if clients, err = svc.client.IsIOInProgress(ctx); err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get array Clients '%s'", err)
		return nil, err
	}
	return clients, nil
}

func (svc *isiService) IsVolumeExistent(ctx context.Context, isiPath, volID, name string) bool {
	csmlog.WithContext(ctx).Debugf("check if volume (id :'%s', name '%s') already exists", volID, name)

	isExistent := svc.client.IsVolumeExistentWithIsiPath(ctx, isiPath, volID, name)

	csmlog.WithContext(ctx).Debugf("volume (id :'%s', name '%s') already exists : '%v'", volID, name, isExistent)

	return isExistent
}

func (svc *isiService) OtherClientsAlreadyAdded(ctx context.Context, exportID int, accessZone string, nodeID string) bool {
	export, _ := svc.GetExportByIDWithZone(ctx, exportID, accessZone)

	if export == nil {
		csmlog.WithContext(ctx).Debugf("failed to get export by id '%d' with access zone '%s', return true for otherClientsAlreadyAdded as a safer return value", exportID, accessZone)
		return true
	}

	clientName, clientFQDN, clientIP, err := id.ParseNodeID(ctx, nodeID)
	if err != nil {
		csmlog.WithContext(ctx).Debugf("failed to parse node ID '%s', return true for otherClientsAlreadyAdded as a safer return value", nodeID)
		return true
	}

	clientFieldsNotEmpty := len(*export.Clients) > 0 || len(*export.ReadOnlyClients) > 0 || len(*export.ReadWriteClients) > 0 || len(*export.RootClients) > 0

	clientFieldLength := len(*export.Clients)

	isNodeInClientFields := strutil.IsStringInSlices(clientName, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)

	isNodeFQDNInClientFields := strutil.IsStringInSlices(clientFQDN, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)

	if clientIP != "" {
		isNodeInClientFields = isNodeInClientFields || strutil.IsStringInSlices(clientIP, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
	}

	clientName, clientFQDN, clientIP, err = id.ParseNodeID(ctx, id.DummyHostNodeID)
	if err != nil {
		csmlog.WithContext(ctx).Debugf("failed to parse node ID '%s', return true for otherClientsAlreadyAdded as a safer return value", nodeID)
		return true
	}

	// Additional check for dummy localhost entry
	isLocalHostInClientFields := strutil.IsStringInSlices(clientName, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
	if !isLocalHostInClientFields {
		isLocalHostInClientFields = strutil.IsStringInSlices(clientFQDN, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
		if !isLocalHostInClientFields {
			isLocalHostInClientFields = strutil.IsStringInSlices(clientIP, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
		}
	}

	if clientFieldLength == 1 && isLocalHostInClientFields {
		clientFieldsNotEmpty = false
	}
	return clientFieldsNotEmpty && !isNodeInClientFields && !isNodeFQDNInClientFields
}

// updateClusterToNodeIDMap updates cluster to nodeID map from input clusterName, nodeID and clientToUse
func updateClusterToNodeIDMap(ctx context.Context, clusterToNodeIDMap *sync.Map, clusterName, nodeID, clientToUse string) error {
	csmlog.WithContext(ctx).Debugf("updating ClusterToNodeIDMap map for cluster '%s', for nodeID '%s' with clientToUse '%s'", clusterName, nodeID, clientToUse)

	var nodeIDToClientMaps []*nodeIDToClientMap

	if m, found := clusterToNodeIDMap.Load(clusterName); found {
		csmlog.WithContext(ctx).Debugf("entry for cluster '%s' found in cluster to nodeID map", clusterName)
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
	var nodeIDToClientMaps []*nodeIDToClientMap

	if m, found := clusterToNodeIDMap.Load(clusterName); found {
		csmlog.WithContext(ctx).Debugf("entry for cluster '%s' found in cluster to nodeID map", clusterName)
		var ok bool
		if nodeIDToClientMaps, ok = m.([]*nodeIDToClientMap); !ok {
			return "", fmt.Errorf("failed to extract nodeIDToClientMap for cluster '%s'", clusterName)
		}

		for _, nodeIDMap := range nodeIDToClientMaps {
			if client, ok := (*nodeIDMap)[nodeID]; ok {
				csmlog.WithContext(ctx).Debugf("node id to client mapping found for nodeID '%s' client '%s'", nodeID, client)
				return client, nil
			}
		}
		return "", fmt.Errorf("node id to client map not found for nodeID '%s' in in cluster to nodeID map", nodeID)
	}
	return "", fmt.Errorf("entry for cluster '%s' not found in cluster to nodeID map", clusterName)
}

func (svc *isiService) AddExportClientNetworkIdentifierByIDWithZone(ctx context.Context, clusterName string, exportID int, accessZone, nodeID string, ignoreUnresolvableHosts bool, addClientFunc func(ctx context.Context, exportID int, accessZone, clientIP string, ignoreUnresolvableHosts bool) error) error {
	var clientToUse string

	// try adding by client FQDN first as it is preferred over IP for its stableness.
	// OneFS API will return error if it cannot resolve the client FQDN ,
	// in that case, fall back to adding by IP

	_, clientFQDN, clientIP, err := id.ParseNodeID(ctx, nodeID)
	if err != nil {
		return err
	}

	csmlog.WithContext(ctx).Debugf("ignoreUnresolvableHosts set to '%v' for cluster '%s'", ignoreUnresolvableHosts, clusterName)
	if ignoreUnresolvableHosts {
		if err = addClientFunc(ctx, exportID, accessZone, clientIP, true); err != nil {
			csmlog.WithContext(ctx).Errorf("failed to add client '%s' to export id '%d': '%v'", clientIP, exportID, err)
			return fmt.Errorf("failed to add client '%s' to the export id '%d'", clientIP, exportID)
		}
		return nil
	}

	currentClient, err := getClientToUseForNodeID(ctx, clusterToNodeIDMap, clusterName, nodeID)
	if err != nil {
		csmlog.WithContext(ctx).Debug(err.Error())
		clientToUse = clientFQDN
	} else {
		clientToUse = currentClient
	}

	csmlog.WithContext(ctx).Debugf("AddExportClientNetworkIdentifierByID adding '%s' as client to export id '%d'", clientToUse, exportID)
	if err = addClientFunc(ctx, exportID, accessZone, clientToUse, false); err == nil {
		if err := updateClusterToNodeIDMap(ctx, clusterToNodeIDMap, clusterName, nodeID, clientToUse); err != nil {
			// not returning with error as export is already updated with client
			csmlog.WithContext(ctx).Warnf("failed to update cluster to nodeID map: '%v'", err)
		}

		return nil
	}
	csmlog.WithContext(ctx).Warnf("failed to add client '%s' to export id '%d': '%v'", clientToUse, exportID, err)

	// try updating export with other client
	otherClientToUse := clientFQDN
	if clientToUse == clientFQDN {
		otherClientToUse = clientIP
	}
	csmlog.WithContext(ctx).Debugf("AddExportClientNetworkIdentifierByID trying to add '%s' as client to export id '%d'", otherClientToUse, exportID)
	if err = addClientFunc(ctx, exportID, accessZone, otherClientToUse, false); err == nil {
		if err := updateClusterToNodeIDMap(ctx, clusterToNodeIDMap, clusterName, nodeID, otherClientToUse); err != nil {
			// not returning with error as export is already updated with client
			csmlog.WithContext(ctx).Warnf("failed to update cluster to nodeID map '%s'", err)
		}
		return nil
	}
	csmlog.WithContext(ctx).Warnf("failed to add client '%s' to export id '%d': '%v'", otherClientToUse, exportID, err)

	return fmt.Errorf("failed to add clients '%s' or '%s' to export id '%d'", clientToUse, otherClientToUse, exportID)
}

type (
	addClientFunc  func(ctx context.Context, exportID int, accessZone, clientIP string, ignoreUnresolvableHosts bool) error
	addClientsFunc func(ctx context.Context, exportID int, accessZone string, clientIPs []string, ignoreUnresolvableHosts bool) error
)

// AddExportClientByIPWithZone adds client IPs to an export with a given ID and access zone.
// When mode is "multi", all IPs are added in a single batch call; if the batch fails, it falls
// back to per-IP addition. When mode is "single" (default), it stops at the first successful IP (failover).
func (svc *isiService) AddExportClientByIPWithZone(ctx context.Context, clusterName string, exportID int, accessZone, nodeID string, clientIPs []string, addClientFunc addClientFunc, addClientsFunc addClientsFunc, mode string) error {
	startTime := time.Now()
	operationID := fmt.Sprintf("export-%d-node-%s", exportID, nodeID)
	defer func() {
		duration := time.Since(startTime)
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"operation":    "AddExportClientByIPWithZone",
			"operation_id": operationID,
			"mode":         mode,
			"export_id":    exportID,
			"access_zone":  accessZone,
			"node_id":      nodeID,
			"duration_ms":  duration.Milliseconds(),
		}).Debugf("IP addition operation completed in %dms", duration.Milliseconds())
	}()

	var err error

	if mode == constants.AllowedNetworksModeMulti {
		if addClientsFunc != nil && len(clientIPs) > 0 {
			csmlog.WithContext(ctx).Debugf("AddExportClientByIPWithZone (multi) batch adding %d clients to export id '%d'", len(clientIPs), exportID)
			if err = addClientsFunc(ctx, exportID, accessZone, clientIPs, false); err == nil {
				for _, clientIP := range clientIPs {
					if mapErr := updateClusterToNodeIDMap(ctx, clusterToNodeIDMap, clusterName, nodeID, clientIP); mapErr != nil {
						csmlog.GetLogger().WithContext(ctx).WithFields(csmlog.Fields{
							"operation":    "AddExportClientByIPWithZone",
							"operation_id": operationID,
							"mode":         mode,
							"client_ip":    clientIP,
							"success":      true,
							"map_update":   false,
						}).Warnf("failed to update cluster to nodeID map: '%v'", mapErr)
					}
				}
				duration := time.Since(startTime)
				csmlog.GetLogger().WithContext(ctx).WithFields(csmlog.Fields{
					"operation":    "AddExportClientByIPWithZone",
					"operation_id": operationID,
					"mode":         mode,
					"total_ips":    len(clientIPs),
					"added_count":  len(clientIPs),
					"duration_ms":  duration.Milliseconds(),
					"success_rate": 100.0,
					"success":      true,
				}).Infof("Multi-NIC IP batch addition completed: %d clients in %dms", len(clientIPs), duration.Milliseconds())
				return nil
			}
			csmlog.GetLogger().WithContext(ctx).WithFields(csmlog.Fields{
				"operation":    "AddExportClientByIPWithZone",
				"operation_id": operationID,
				"mode":         mode,
				"client_ips":   clientIPs,
				"export_id":    exportID,
				"success":      false,
			}).Warnf("failed to batch add clients '%v' to export id '%d': '%v' (falling back to per-IP)", clientIPs, exportID, err)
		}

		if addClientFunc == nil {
			return fmt.Errorf("addClientFunc is nil for multi-IP fallback")
		}

		var addedCount, failedCount int
		for _, clientIP := range clientIPs {
			csmlog.WithContext(ctx).WithFields(csmlog.Fields{
				"operation":    "AddExportClientByIPWithZone",
				"operation_id": operationID,
				"mode":         mode,
				"client_ip":    clientIP,
				"export_id":    exportID,
			}).Debugf("AddExportClientByIPWithZone (multi) adding '%s' as client to export id '%d'", clientIP, exportID)

			if err = addClientFunc(ctx, exportID, accessZone, clientIP, false); err != nil {
				failedCount++
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"operation":    "AddExportClientByIPWithZone",
					"operation_id": operationID,
					"mode":         mode,
					"client_ip":    clientIP,
					"export_id":    exportID,
					"success":      false,
				}).Warnf("failed to add client '%s' to export id '%d': '%v' (continuing)", clientIP, exportID, err)
			} else {
				if mapErr := updateClusterToNodeIDMap(ctx, clusterToNodeIDMap, clusterName, nodeID, clientIP); mapErr != nil {
					csmlog.WithContext(ctx).WithFields(csmlog.Fields{
						"operation":    "AddExportClientByIPWithZone",
						"operation_id": operationID,
						"mode":         mode,
						"client_ip":    clientIP,
						"success":      true,
						"map_update":   false,
					}).Warnf("failed to update cluster to nodeID map: '%v'", mapErr)
				}
				addedCount++
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"operation":    "AddExportClientByIPWithZone",
					"operation_id": operationID,
					"mode":         mode,
					"client_ip":    clientIP,
					"export_id":    exportID,
					"success":      true,
				}).Debugf("successfully added client '%s' to export id '%d'", clientIP, exportID)
			}
		}

		duration := time.Since(startTime)
		successRate := 0.0
		if len(clientIPs) > 0 {
			successRate = float64(addedCount) / float64(len(clientIPs)) * 100
		}

		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"operation":    "AddExportClientByIPWithZone",
			"operation_id": operationID,
			"mode":         mode,
			"total_ips":    len(clientIPs),
			"added_count":  addedCount,
			"failed_count": failedCount,
			"duration_ms":  duration.Milliseconds(),
			"success_rate": successRate,
			"success":      addedCount > 0,
		}).Infof("Multi-NIC IP addition completed: %d/%d succeeded in %dms (%.1f%% success rate)",
			addedCount, len(clientIPs), duration.Milliseconds(), successRate)

		if addedCount == 0 {
			csmlog.WithContext(ctx).WithFields(csmlog.Fields{
				"operation":    "AddExportClientByIPWithZone",
				"operation_id": operationID,
				"mode":         mode,
				"total_ips":    len(clientIPs),
				"added_count":  addedCount,
				"failed_count": failedCount,
				"success":      false,
			}).Errorf("failed to add any of clients '%v' to export id '%d'", clientIPs, exportID)
			return fmt.Errorf("failed to add any of clients '%v' to export id '%d'", clientIPs, exportID)
		}
		return nil
	}

	// Single mode: existing first-success failover behavior
	for _, clientIP := range clientIPs {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"operation":    "AddExportClientByIPWithZone",
			"operation_id": operationID,
			"mode":         mode,
			"client_ip":    clientIP,
			"export_id":    exportID,
		}).Debugf("AddExportClientByIPWithZone adding '%s' as client to export id '%d'", clientIP, exportID)

		if err = addClientFunc(ctx, exportID, accessZone, clientIP, false); err == nil {
			if err = updateClusterToNodeIDMap(ctx, clusterToNodeIDMap, clusterName, nodeID, clientIP); err != nil {
				// not returning with error as export is already updated with client
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"operation":    "AddExportClientByIPWithZone",
					"operation_id": operationID,
					"mode":         mode,
					"client_ip":    clientIP,
					"success":      true,
					"map_update":   false,
				}).Warnf("failed to update cluster to nodeID map: '%v'", err)
			}

			csmlog.WithContext(ctx).WithFields(csmlog.Fields{
				"operation":    "AddExportClientByIPWithZone",
				"operation_id": operationID,
				"mode":         mode,
				"client_ip":    clientIP,
				"export_id":    exportID,
				"success":      true,
			}).Debugf("successfully added client '%s' to export id '%d' in single mode", clientIP, exportID)
			return nil
		}
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"operation":    "AddExportClientByIPWithZone",
			"operation_id": operationID,
			"mode":         mode,
			"client_ip":    clientIP,
			"export_id":    exportID,
			"success":      false,
		}).Warnf("failed to add client '%s' to export id '%d': '%v'", clientIP, exportID, err)
	}

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		"operation":    "AddExportClientByIPWithZone",
		"operation_id": operationID,
		"mode":         mode,
		"total_ips":    len(clientIPs),
		"success":      false,
	}).Errorf("failed to add clients '%v' to export id '%d'", clientIPs, exportID)
	return fmt.Errorf("failed to add clients '%v' to export id '%d'", clientIPs, exportID)
}

func (svc *isiService) AddExportClientByIDWithZone(ctx context.Context, exportID int, accessZone, clientIP string, ignoreUnresolvableHosts bool) error {
	csmlog.WithContext(ctx).Debugf("AddExportClientByID client '%s'", clientIP)
	if err := svc.client.AddExportClientsByIDWithZone(ctx, exportID, accessZone, []string{clientIP}, ignoreUnresolvableHosts); err != nil {
		return fmt.Errorf("failed to add client to export id '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}
	return nil
}

func (svc *isiService) AddExportRootClientByIDWithZone(ctx context.Context, exportID int, accessZone, clientIP string, ignoreUnresolvableHosts bool) error {
	csmlog.WithContext(ctx).Debugf("AddExportRootClientByID client '%s'", clientIP)
	if err := svc.client.AddExportRootClientsByIDWithZone(ctx, exportID, accessZone, []string{clientIP}, ignoreUnresolvableHosts); err != nil {
		return fmt.Errorf("failed to add client to export id '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}
	return nil
}

func (svc *isiService) AddExportReadOnlyClientByIDWithZone(ctx context.Context, exportID int, accessZone, clientIP string, ignoreUnresolvableHosts bool) error {
	csmlog.WithContext(ctx).Debugf("AddExportReadOnlyClientByID client '%s'", clientIP)
	if err := svc.client.AddExportReadOnlyClientsByIDWithZone(ctx, exportID, accessZone, []string{clientIP}, ignoreUnresolvableHosts); err != nil {
		return fmt.Errorf("failed to add read only client to export id '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}
	return nil
}

func (svc *isiService) AddExportClientsByIDWithZone(ctx context.Context, exportID int, accessZone string, clientIPs []string, ignoreUnresolvableHosts bool) error {
	log := csmlog.WithContext(ctx)

	log.Debugf("AddExportClientsByID clients '%v'", clientIPs)
	if err := svc.client.AddExportClientsByIDWithZone(ctx, exportID, accessZone, clientIPs, ignoreUnresolvableHosts); err != nil {
		return fmt.Errorf("failed to add clients to export id '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}
	return nil
}

func (svc *isiService) AddExportRootClientsByIDWithZone(ctx context.Context, exportID int, accessZone string, clientIPs []string, ignoreUnresolvableHosts bool) error {
	log := csmlog.WithContext(ctx)

	log.Debugf("AddExportRootClientsByID clients '%v'", clientIPs)
	if err := svc.client.AddExportRootClientsByIDWithZone(ctx, exportID, accessZone, clientIPs, ignoreUnresolvableHosts); err != nil {
		return fmt.Errorf("failed to add clients to export id '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}
	return nil
}

func (svc *isiService) AddExportReadOnlyClientsByIDWithZone(ctx context.Context, exportID int, accessZone string, clientIPs []string, ignoreUnresolvableHosts bool) error {
	log := csmlog.WithContext(ctx)

	log.Debugf("AddExportReadOnlyClientsByID clients '%v'", clientIPs)
	if err := svc.client.AddExportReadOnlyClientsByIDWithZone(ctx, exportID, accessZone, clientIPs, ignoreUnresolvableHosts); err != nil {
		return fmt.Errorf("failed to add read only clients to export id '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}
	return nil
}

func (svc *isiService) RemoveExportClientByIDWithZone(ctx context.Context, exportID int, accessZone, nodeID string, ignoreUnresolvableHosts bool) error {
	// it could either be IP or FQDN that has been added to the export's client fields, should consider both during the removal
	clientName, clientFQDN, clientIP, err := id.ParseNodeID(ctx, nodeID)
	if err != nil {
		return err
	}

	csmlog.WithContext(ctx).Debugf("RemoveExportClientByIDWithZone client Name '%s', client FQDN '%s' client IP '%s'", clientName, clientFQDN, clientIP)

	clientsToRemove := []string{clientIP, clientName, clientFQDN}

	csmlog.WithContext(ctx).Debugf("RemoveExportClientByName client '%v'", clientsToRemove)

	if err := svc.client.RemoveExportClientsByIDWithZone(ctx, exportID, accessZone, clientsToRemove, ignoreUnresolvableHosts); err != nil {
		if notFoundErr, ok := err.(*api.JSONError); ok {
			if notFoundErr.StatusCode == 404 {
				csmlog.WithContext(ctx).Debugf("Export id '%d' does not exist", exportID)
				return nil
			}
		}
		return fmt.Errorf("failed to remove clients from export '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}

	return nil
}

func (svc *isiService) RemoveExportClientByIPsWithZone(ctx context.Context, exportID int, accessZone string, clientIPs []string, ignoreUnresolvableHosts bool) error {
	if err := svc.client.RemoveExportClientsByIDWithZone(ctx, exportID, accessZone, clientIPs, ignoreUnresolvableHosts); err != nil {
		if notFoundErr, ok := err.(*api.JSONError); ok {
			if notFoundErr.StatusCode == 404 {
				csmlog.WithContext(ctx).Debugf("Export id '%d' does not exist", exportID)
				return nil
			}
		}
		return fmt.Errorf("failed to remove clients from export '%d' with access zone '%s' : '%s'", exportID, accessZone, err.Error())
	}

	return nil
}

func (svc *isiService) GetExportsWithLimit(ctx context.Context, limit string) (isi.ExportList, string, error) {
	csmlog.WithContext(ctx).Debug("begin getting exports for Isilon")
	var exports isi.Exports
	var err error
	if exports, err = svc.client.GetExportsWithLimit(ctx, limit); err != nil {
		csmlog.WithContext(ctx).Error("failed to get exports")
		return nil, "", err
	}
	return exports.Exports, exports.Resume, nil
}

func (svc *isiService) GetExportsWithResume(ctx context.Context, resume string) (isi.ExportList, string, error) {
	csmlog.WithContext(ctx).Debug("begin getting exports for Isilon")
	var exports isi.Exports
	var err error
	if exports, err = svc.client.GetExportsWithResume(ctx, resume); err != nil {
		csmlog.WithContext(ctx).Error("failed to get exports: " + err.Error())
		return nil, "", err
	}
	return exports.Exports, exports.Resume, nil
}

// GetFilesystems lists filesystems using the namespace API
func (svc *isiService) GetFilesystems(ctx context.Context, containerPath string) ([]*apiv2.ContainerChild, error) {
	csmlog.WithContext(ctx).Debugf("begin getting filesystems for path %s", containerPath)
	filter := "type=container,detail=name|container_path|type|owner|group|size"
	filesystems, _, err := svc.client.ListVolumes(ctx, containerPath, filter, 0, "")
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get filesystems: %s", err.Error())
		return nil, err
	}
	return filesystems, nil
}

// GetFilesystemsWithLimit lists filesystems with pagination using the namespace API
func (svc *isiService) GetFilesystemsWithLimit(ctx context.Context, containerPath string, limit string) ([]*apiv2.ContainerChild, string, error) {
	csmlog.WithContext(ctx).Debugf("begin getting filesystems for path %s with limit %s", containerPath, limit)
	filter := "type=container,detail=name|container_path|type|owner|group|size"
	maxEntries, err := strconv.Atoi(limit)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("invalid limit value %s: %s", limit, err.Error())
		return nil, "", err
	}
	filesystems, resume, err := svc.client.ListVolumes(ctx, containerPath, filter, maxEntries, "")
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get filesystems with limit: %s", err.Error())
		return nil, "", err
	}
	return filesystems, resume, nil
}

// GetFilesystemsWithResume lists filesystems with pagination resume token using the namespace API
func (svc *isiService) GetFilesystemsWithResume(ctx context.Context, containerPath string, maxEntries int, resume string) ([]*apiv2.ContainerChild, string, error) {
	csmlog.WithContext(ctx).Debugf("begin getting filesystems for path %s with limit %d and resume token %s", containerPath, maxEntries, resume)
	filter := "type=container,detail=name|container_path|type|owner|group|size"
	filesystems, newResume, err := svc.client.ListVolumes(ctx, containerPath, filter, maxEntries, resume)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get filesystems with resume: %s", err.Error())
		return nil, "", err
	}
	return filesystems, newResume, nil
}

func (svc *isiService) DeleteSnapshot(ctx context.Context, id int64, name string) error {
	csmlog.WithContext(ctx).Debugf("begin to delete snapshot '%s'", name)
	if err := svc.client.RemoveSnapshot(ctx, id, name); err != nil {
		csmlog.WithContext(ctx).Errorf("delete snapshot failed, '%s'", err.Error())
		return err
	}
	return nil
}

func (svc *isiService) GetSnapshot(ctx context.Context, identity string) (isi.Snapshot, error) {
	csmlog.WithContext(ctx).Debugf("begin getting snapshot with id|name '%s' for Isilon", identity)
	var snapshot isi.Snapshot
	var err error
	if snapshot, err = svc.client.GetIsiSnapshotByIdentity(ctx, identity); err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get snapshot '%s'", err.Error())
		return nil, err
	}

	return snapshot, nil
}

func (svc *isiService) GetSnapshots(ctx context.Context) (isi.SnapshotList, error) {
	csmlog.WithContext(ctx).Debugf("begin getting all the snapshot  for Isilon")
	var snapshotList isi.SnapshotList
	var err error
	if snapshotList, err = svc.client.GetSnapshots(ctx); err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get snapshots '%s'", err.Error())
		return nil, err
	}
	return snapshotList, nil
}

func (svc *isiService) GetSnapshotSize(ctx context.Context, isiPath, name string, accessZone string) int64 {
	csmlog.WithContext(ctx).Debugf("begin getting snapshot size with name '%s' for Isilon", name)
	size, err := svc.client.GetSnapshotFolderSize(ctx, isiPath, name, accessZone)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get snapshot size '%s'", err.Error())
		return 0
	}

	return size
}

func (svc *isiService) GetExportWithPathAndZone(ctx context.Context, path, accessZone string) (isi.Export, error) {
	csmlog.WithContext(ctx).Debugf("begin getting export with target path '%s' and access zone '%s' for Isilon", path, accessZone)
	var export isi.Export
	var err error
	if export, err = svc.client.GetExportWithPathAndZone(ctx, path, accessZone); err != nil {
		csmlog.WithContext(ctx).Error("failed to get export with target path '" + path + "' and access zone '" + accessZone + "': '" + err.Error() + "'")
		return nil, err
	}

	return export, nil
}

func (svc *isiService) GetExportWithPath(ctx context.Context, path string) (isi.Export, error) {
	csmlog.WithContext(ctx).Debugf("begin getting export with target path '%s' for Isilon", path)
	var export isi.Export
	var err error
	if export, err = svc.client.GetExportWithPath(ctx, path); err != nil {
		csmlog.WithContext(ctx).Error("failed to get export with target path '" + path + "' : '" + err.Error() + "'")
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
		if strings.Contains(exportPath, "/.snapshot") {
			isROVolFromSnapshot = true
		}
	}
	return isROVolFromSnapshot
}

func (svc *isiService) GetSnapshotNameFromIsiPath(ctx context.Context, snapshotIsiPath, accessZone, zonePath string) (string, error) {
	var snapShotName string
	if !svc.isROVolumeFromSnapshot(snapshotIsiPath, accessZone) {
		csmlog.WithContext(ctx).Debugf("invalid snapshot isilon path- '%s'", snapshotIsiPath)
		return "", fmt.Errorf("invalid snapshot isilon path")
	}
	// Snapshot isi path format /<ifs>/.snapshot/<snapshot_name>/<volume_path_without_ifs_prefix>
	// Non System Access Zone /<ifs>/<csi_zone_base_path>/.snapshot/<snapshot_name>/<volume_path_without_ifs_prefix>
	pathWithoutZonePath := strings.Trim(snapshotIsiPath, zonePath)
	directories := strings.Split(pathWithoutZonePath, "/")
	// If there is no snapshot name in snapshot isi path or if it is empty
	if len(directories) < 2 || directories[2] == "" {
		csmlog.WithContext(ctx).Debugf("invalid snapshot isilon path- '%s'", snapshotIsiPath)
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

func (svc *isiService) GetSubDirectoryCount(ctx context.Context, isiPath, directory string) (int64, error) {
	var totalSubDirectories int64
	if svc.IsVolumeExistent(ctx, isiPath, "", directory) {
		// Check if there are any entries for volumes present in snapshot tracking dir
		dirDetails, err := svc.GetVolumeWithIsiPath(ctx, isiPath, "", directory)
		if err != nil {
			return 0, err
		}
		csmlog.WithContext(ctx).Debugf("directory details for directory '%s' are '%s'", directory, dirDetails)

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
		csmlog.WithContext(ctx).Debugf("total number of subdirectories present under directory '%s' is '%v'",
			directory, totalSubDirectories)
		return totalSubDirectories, nil
	}

	return 0, fmt.Errorf("failed to get subdirectory count for directory '%s'", directory)
}

func (svc *isiService) IsHostAlreadyAdded(ctx context.Context, exportID int, accessZone string, nodeID string) bool {
	export, _ := svc.GetExportByIDWithZone(ctx, exportID, accessZone)

	if export == nil {
		csmlog.WithContext(ctx).Debugf("failed to get export by id '%d' with access zone '%s', return true for LocalhostAlreadyAdded as a safer return value", exportID, accessZone)
		return true
	}

	clientName, clientFQDN, clientIP, err := id.ParseNodeID(ctx, nodeID)
	if err != nil {
		csmlog.WithContext(ctx).Debugf("failed to parse node ID '%s', return true for LocalhostAlreadyAdded as a safer return value", nodeID)
		return true
	}

	clientFieldsNotEmpty := len(*export.Clients) > 0 || len(*export.ReadOnlyClients) > 0 || len(*export.ReadWriteClients) > 0 || len(*export.RootClients) > 0

	isNodeInClientFields := strutil.IsStringInSlices(clientName, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)

	isNodeFQDNInClientFields := strutil.IsStringInSlices(clientFQDN, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)

	if clientIP != "" {
		isNodeInClientFields = isNodeInClientFields || strutil.IsStringInSlices(clientIP, *export.Clients, *export.ReadOnlyClients, *export.ReadWriteClients, *export.RootClients)
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
