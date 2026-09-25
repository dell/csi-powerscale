// Copyright © 2019-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/k8sutils"
	id "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/identifiers"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	isilonfs "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/powerscale-fs"
	csiutils "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/csi-utils"
	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	getIsVolumeExistentFunc = func(isiConfig *IsilonClusterConfig) func(context.Context, string, string, string) bool {
		return isiConfig.isiSvc.IsVolumeExistent
	}
	// setVolumeGroupOwnershipFunc applies fsGroup ownership to a directory-backed
	// volume via the OneFS management API (ACL). It is a package-level var to allow
	// test mocking without an interface.
	setVolumeGroupOwnershipFunc = func(isiConfig *IsilonClusterConfig) func(context.Context, string, string, int, bool) (*SetVolumeGroupOwnershipResult, error) {
		return isiConfig.isiSvc.SetVolumeGroupOwnershipByPath
	}
	getIsVolumeMounted = isVolumeMounted
	getOsReadDir       = os.ReadDir
	getOsOpenRoot      = os.OpenRoot
	applyFSPermsFunc   = applyFSGroupPermissions
	jsonMarshalFunc    = json.Marshal
	// Test-only variables for k8s client mocking
	k8sListVolumeAttachmentsFunc = func(ctx context.Context, k8sclient kubernetes.Interface, opts metav1.ListOptions) (*storagev1.VolumeAttachmentList, error) {
		return k8sclient.StorageV1().VolumeAttachments().List(ctx, opts)
	}
	k8sGetPersistentVolumeFunc = func(ctx context.Context, k8sclient kubernetes.Interface, name string, opts metav1.GetOptions) (*corev1.PersistentVolume, error) {
		return k8sclient.CoreV1().PersistentVolumes().Get(ctx, name, opts)
	}
	getCreateVolumeFunc = func(s *service) func(context.Context, *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
		return s.CreateVolume
	}
	getControllerPublishVolume = func(s *service) func(context.Context, *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
		return s.ControllerPublishVolume
	}
	getUtilsGetFQDNByIP = id.GetFQDNByIP
	getK8sutilsGetStats = k8sutils.GetStats
	getNodeLabelsFunc   = func(s *service) func() (map[string]string, error) {
		return s.GetNodeLabels
	}
	getPatchNodeLabelsFunc = func(s *service) func(map[string]string, []string) error {
		return s.PatchNodeLabels
	}
	getInterfaceAddrsFunc = func() func() ([]net.Addr, error) {
		return net.InterfaceAddrs
	}
)

func (s *service) NodeExpandVolume(
	context.Context,
	*csi.NodeExpandVolumeRequest,
) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// withResolvedMTLSVolumeContext returns a copy of volumeContext with the effective
// mTLS values, leaving the original request context unchanged.
func withResolvedMTLSVolumeContext(volumeContext map[string]string, fqdn, transportSecurity string) map[string]string {
	resolvedContext := make(map[string]string, len(volumeContext)+2)
	for key, value := range volumeContext {
		resolvedContext[key] = value
	}
	if fqdn != "" {
		resolvedContext[constants.SmartConnectZoneFQDNParam] = fqdn
	}
	if transportSecurity != "" {
		resolvedContext[constants.NFSTransportSecurityParam] = transportSecurity
	}
	return resolvedContext
}

// validateMTLSPrerequisites performs comprehensive mTLS validation including TLS capability,
// mount target validation, and mount options validation. Returns an error if validation fails.
// This helper consolidates validation logic shared between NodeStageVolume and NodePublishVolume.
func (s *service) validateMTLSPrerequisites(
	ctx context.Context,
	runID string,
	nfsTransportSecurity string,
	mountTarget string,
	resolvedMountTarget string,
	accessZone string,
	mountOptions []string,
	volumeContext map[string]string,
) error {
	if !IsMTLSEnabled(nfsTransportSecurity) {
		return nil
	}

	// Layer 2 fail-fast check: verify kernel TLS and tlshd daemon
	tlsCapability, err := ValidateTLSCapabilityForMount(ctx, nfsTransportSecurity)
	if err != nil {
		// Log but continue - non-critical validation warning
		csmlog.WithContext(ctx).Warnf("TLS capability validation warning: %v", err)
	}
	if !tlsCapability.TLSCapable {
		errMsg := GetTLSCapabilityErrorMessage(tlsCapability)
		reason := GetTLSCapabilityEventReason(tlsCapability)

		LogMTLSMountOperation(ctx, MTLSLogFields{
			MountTarget:     mountTarget,
			MountTargetFQDN: resolvedMountTarget,
			TLSEnabled:      true,
			AccessZone:      accessZone,
			MountOptions:    strings.Join(mountOptions, ","),
			Outcome:         "failed",
			ErrorCode:       "FailedPrecondition",
			ErrorMessage:    errMsg,
			NodeID:          s.nodeID,
		})

		pvcNamespace := volumeContext[csiPersistentVolumeClaimNamespace]
		pvcName := volumeContext[csiPersistentVolumeClaimName]
		if pvcNamespace != "" && pvcName != "" {
			_ = EmitMTLSEvent(ctx, s.k8sclient, pvcNamespace, pvcName, reason, errMsg)
		}

		return status.Error(codes.FailedPrecondition, GetMessageWithReqID(runID, "%s", errMsg))
	}

	validationResult := ValidateMTLSMountTarget(mountTarget, nfsTransportSecurity)
	if !validationResult.Valid {
		// Log structured mTLS operation failure
		LogMTLSMountOperation(ctx, MTLSLogFields{
			MountTarget:     mountTarget,
			MountTargetFQDN: resolvedMountTarget,
			TLSEnabled:      true,
			AccessZone:      accessZone,
			MountOptions:    strings.Join(mountOptions, ","),
			Outcome:         "failed",
			ErrorCode:       validationResult.ErrorCode,
			ErrorMessage:    validationResult.ErrorMessage,
			NodeID:          s.nodeID,
		})

		// Emit Kubernetes event for the PVC
		pvcNamespace := volumeContext[csiPersistentVolumeClaimNamespace]
		pvcName := volumeContext[csiPersistentVolumeClaimName]
		if pvcNamespace != "" && pvcName != "" {
			_ = EmitMTLSEvent(ctx, s.k8sclient, pvcNamespace, pvcName, validationResult.EventReason, validationResult.ErrorMessage)
		}

		return status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "%s", validationResult.ErrorMessage))
	}

	// Validate mount options for mTLS conflicts
	if warning, err := ValidateMountOptionsForMTLS(mountOptions, nfsTransportSecurity); err != nil {
		LogMTLSMountOperation(ctx, MTLSLogFields{
			MountTarget:     mountTarget,
			MountTargetFQDN: resolvedMountTarget,
			TLSEnabled:      true,
			AccessZone:      accessZone,
			MountOptions:    strings.Join(mountOptions, ","),
			Outcome:         "failed",
			ErrorCode:       "InvalidArgument",
			ErrorMessage:    err.Error(),
			NodeID:          s.nodeID,
		})
		return status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "%s", err.Error()))
	} else if warning != "" {
		csmlog.WithContext(ctx).Warnf("mTLS mount warning: %s", warning)
	}

	// Log successful mTLS validation
	csmlog.WithContext(ctx).Infof("mTLS validation passed: mount_target=%s, transport_security=%s", mountTarget, nfsTransportSecurity)
	return nil
}

func (s *service) NodeStageVolume(
	ctx context.Context,
	req *csi.NodeStageVolumeRequest) (
	*csi.NodeStageVolumeResponse, error,
) {
	fields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", fields["csi.requestid"])

	s.logStatistics()

	// Validate request
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeID is required")
	}
	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "StagingTargetPath is required")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "VolumeCapability is required")
	}
	mntVol := req.GetVolumeCapability().GetMount()
	if mntVol == nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid access type: mount capability required")
	}

	volumeContext := req.GetVolumeContext()
	if volumeContext == nil {
		return nil, status.Error(codes.InvalidArgument, "VolumeContext is required")
	}

	stagingPath := req.GetStagingTargetPath()

	// Detect provisioning mode and extract volume name for mount detection
	volName, _, _, _, provisioningMode, _ := id.ParseVolumeIDWithMode(ctx, req.GetVolumeId())
	if volName == "" {
		volName = req.GetVolumeId() // Fallback for non-normalized volume IDs
	}
	isDirectoryBacked := (provisioningMode == id.ProvisioningModeDirectory) ||
		(volumeContext["ProvisioningMode"] == "directory")

	// Validate the VolumeContext fields required to build the mount source before any
	// cluster lookup, so a malformed VolumeContext reports the specific missing field
	// instead of being masked by an unrelated cluster-config error.
	sharedExportPath := volumeContext["SharedExportPath"]
	directoryPath := volumeContext["DirectoryPath"]
	exportPath := volumeContext["Path"]
	if isDirectoryBacked {
		if sharedExportPath == "" {
			return nil, status.Error(codes.FailedPrecondition,
				"SharedExportPath not found in VolumeContext for directory-backed volume")
		}
		if directoryPath == "" {
			return nil, status.Error(codes.FailedPrecondition,
				"DirectoryPath not found in VolumeContext for directory-backed volume")
		}
	} else if exportPath == "" {
		return nil, status.Error(codes.FailedPrecondition,
			"Path not found in VolumeContext for export-backed volume")
	}

	// Get cluster config early (needed for both mTLS resolution and URL construction)
	clusterName := volumeContext["ClusterName"]
	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get cluster config: %v", err)
	}

	// Resolve azServiceIP (handles custom topology and authorization)
	var azServiceIP string
	if s.opts.CustomTopologyEnabled {
		azServiceIP = isiConfig.Endpoint
	} else {
		azServiceIP = volumeContext[AzServiceIPParam]
	}

	if strings.Contains(azServiceIP, "localhost") {
		csmlog.WithContext(ctx).Debugf("Authorization is enabled, reading MountEndpoint: '%s'", isiConfig.MountEndpoint)
		azServiceIP = isiConfig.MountEndpoint
	}

	// Extract mTLS parameters from volume context
	smartConnectZoneFQDN := volumeContext[constants.SmartConnectZoneFQDNParam]

	// NFSTransportSecurity is StorageClass-only (no Secret or environment inheritance)
	nfsTransportSecurity := volumeContext[constants.NFSTransportSecurityParam]

	// Resolve the mount target FQDN using 3-layer precedence:
	// 1. StorageClass parameter SmartConnectZoneFQDN (highest)
	// 2. Cluster Secret field nfsMountFQDN
	// 3. Environment variable X_CSI_ISI_NFS_MOUNT_FQDN (lowest)
	// Note: Only FQDN has this fallback chain; transport security does not.
	envNFSMountFQDN := os.Getenv(constants.EnvNFSMountFQDN)
	resolvedMountTarget := ResolveMountFQDN(smartConnectZoneFQDN, isiConfig.NFSMountFQDN, envNFSMountFQDN)
	volumeContext = withResolvedMTLSVolumeContext(volumeContext, resolvedMountTarget, nfsTransportSecurity)

	// Determine the final mount target
	mountTarget := azServiceIP
	if resolvedMountTarget != "" {
		mountTarget = resolvedMountTarget
		csmlog.WithContext(ctx).Infof("NodeStageVolume: Using resolved FQDN for mount target: %s (FQDN precedence: SC=%s, Secret=%s, env=%s)",
			resolvedMountTarget, smartConnectZoneFQDN, isiConfig.NFSMountFQDN, envNFSMountFQDN)
	}

	accessZone := volumeContext["AccessZone"]

	// Validate mTLS mount target requirements
	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()
	if err := s.validateMTLSPrerequisites(ctx, runID, nfsTransportSecurity, mountTarget, resolvedMountTarget, accessZone, mountOptions, volumeContext); err != nil {
		return nil, err
	}

	var nfsExportURL string
	var mountPath string

	if isDirectoryBacked {
		// Directory-backed: mount subdirectory
		// Construct full mount path
		mountPath = isilonfs.GetPathForVolume(sharedExportPath, directoryPath)

		// Build NFS URL using resolved mount target (FQDN if mTLS, otherwise azServiceIP)
		nfsExportURL = isiConfig.isiSvc.GetNFSExportURLForPath(mountTarget, mountPath)

		logFields := csmlog.Fields{
			"ProvisioningMode": "directory",
			"SharedExportPath": sharedExportPath,
			"DirectoryPath":    directoryPath,
			"MountPath":        mountPath,
			"StagingPath":      stagingPath,
			"NFSExportURL":     nfsExportURL,
		}
		if IsTLSEnabled(nfsTransportSecurity) {
			logFields["MountTarget"] = mountTarget
			logFields["NFSTransportSecurity"] = nfsTransportSecurity
		}
		csmlog.WithContext(ctx).WithFields(logFields).Info("NodeStageVolume: mounting directory-backed volume")
	} else {
		// Export-backed: mount export root
		// Build NFS URL using resolved mount target (FQDN if mTLS, otherwise azServiceIP)
		nfsExportURL = isiConfig.isiSvc.GetNFSExportURLForPath(mountTarget, exportPath)

		logFields := csmlog.Fields{
			"ProvisioningMode": "export",
			"Path":             exportPath,
			"StagingPath":      stagingPath,
			"NFSExportURL":     nfsExportURL,
		}
		if IsTLSEnabled(nfsTransportSecurity) {
			logFields["MountTarget"] = mountTarget
			logFields["NFSTransportSecurity"] = nfsTransportSecurity
		}
		csmlog.WithContext(ctx).WithFields(logFields).Info("NodeStageVolume: mounting export-backed volume")
	}

	// Perform network mount to staging path
	// Apply TLS handshake timeout for mTLS mounts
	mountCtx, mountCancel := CreateMountTimeoutContext(ctx, nfsTransportSecurity)
	defer mountCancel()

	if err := publishVolume(mountCtx, &csi.NodePublishVolumeRequest{
		VolumeId:         req.GetVolumeId(),
		TargetPath:       stagingPath,
		VolumeCapability: req.GetVolumeCapability(),
		VolumeContext:    volumeContext,
	}, nfsExportURL); err != nil {
		// Classify TLS errors and emit Kubernetes events for mTLS mount failures
		if IsMTLSEnabled(nfsTransportSecurity) {
			if IsTLSError(err) {
				classification := ParseTLSError(ctx, err)

				LogMTLSMountOperation(ctx, MTLSLogFields{
					MountTarget:     mountTarget,
					MountTargetFQDN: resolvedMountTarget,
					TLSEnabled:      true,
					AccessZone:      accessZone,
					MountOptions:    strings.Join(mntVol.GetMountFlags(), ","),
					Outcome:         "failed",
					ErrorCode:       classification.ErrorCode,
					ErrorMessage:    classification.Message,
					NodeID:          s.nodeID,
				})

				pvcNamespace := volumeContext[csiPersistentVolumeClaimNamespace]
				pvcName := volumeContext[csiPersistentVolumeClaimName]
				if pvcNamespace != "" && pvcName != "" {
					_ = EmitMTLSEvent(ctx, s.k8sclient, pvcNamespace, pvcName, classification.Reason, classification.Message)
				}

				return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "mTLS mount failed at staging path: %s", classification.Message))
			}

			// Check if mount timed out during mTLS handshake
			if IsTimeoutError(err) {
				timeoutMsg := fmt.Sprintf("TLS handshake timed out after %d seconds for mount target '%s'. "+
					"Verify network connectivity and TLS daemon (tlshd) are operational.",
					GetTLSHandshakeTimeout(ctx), mountTarget)

				LogMTLSMountOperation(ctx, MTLSLogFields{
					MountTarget:     mountTarget,
					MountTargetFQDN: resolvedMountTarget,
					TLSEnabled:      true,
					AccessZone:      accessZone,
					MountOptions:    strings.Join(mntVol.GetMountFlags(), ","),
					Outcome:         "failed",
					ErrorCode:       "DeadlineExceeded",
					ErrorMessage:    timeoutMsg,
					NodeID:          s.nodeID,
				})

				pvcNamespace := volumeContext[csiPersistentVolumeClaimNamespace]
				pvcName := volumeContext[csiPersistentVolumeClaimName]
				if pvcNamespace != "" && pvcName != "" {
					_ = EmitMTLSEvent(ctx, s.k8sclient, pvcNamespace, pvcName, constants.TLSEventReasonHandshakeTimeout, timeoutMsg)
				}

				return nil, status.Error(codes.DeadlineExceeded, GetMessageWithReqID(runID, "%s", timeoutMsg))
			}
		}

		return nil, status.Errorf(codes.Internal, "failed to mount volume at staging path: %v", err)
	}

	// Apply fsGroup ownership if present.
	//
	// fsGroup is delivered by the Container Orchestrator via the CSI
	// VolumeMountGroup field (requires CSIDriver.fsGroupPolicy=File and the
	// VOLUME_MOUNT_GROUP node capability).
	//
	// Directory-backed volumes apply ownership via the OneFS management API (ACL).
	// This is required because the shared NFS export used by directory-backed
	// volumes keeps RootClientEnabled "false" for security, which prevents
	// node-side chown over the NFS mount (root is squashed to nobody).
	// The management API is authenticated as the service account and is therefore
	// not subject to NFS root_squash.
	//
	// Export-backed volumes are handled via node-side recursive chown, but only when
	// RootClientEnabled is "true". With RootClientEnabled "false", the node root
	// is squashed to nobody and cannot change ownership, so fsGroup is skipped
	// (this is the same behavior as before VOLUME_MOUNT_GROUP was advertised).
	if fsGroup := req.GetVolumeCapability().GetMount().GetVolumeMountGroup(); fsGroup != "" {
		gid, err := strconv.Atoi(fsGroup)
		if err != nil {
			// Unmount on error using volName as filterStr
			_ = unpublishVolume(ctx, &csi.NodeUnpublishVolumeRequest{
				VolumeId:   req.GetVolumeId(),
				TargetPath: stagingPath,
			}, volName)
			return nil, status.Errorf(codes.InvalidArgument,
				"invalid fsGroup value '%s': %v", fsGroup, err)
		}

		if isDirectoryBacked {
			// Use cluster config and paths already fetched earlier in NodeStageVolume
			// Check if this volume was created from snapshot/clone
			isClone := volumeContext["NeedsRecursiveOwnership"] == "true"

			// Set group=<fsGroup> and mode 2770 via the OneFS management API.
			result, err := setVolumeGroupOwnershipFunc(isiConfig)(ctx, sharedExportPath, directoryPath, gid, isClone)
			if err != nil {
				// Unmount on error using volName as filterStr
				_ = unpublishVolume(ctx, &csi.NodeUnpublishVolumeRequest{
					VolumeId:   req.GetVolumeId(),
					TargetPath: stagingPath,
				}, volName)
				return nil, status.Errorf(codes.Internal,
					"failed to apply fsGroup ownership: %v", err)
			}

			csmlog.WithContext(ctx).WithFields(csmlog.Fields{
				"sharedExportPath": sharedExportPath,
				"directoryPath":    directoryPath,
				"fsGroup":          fsGroup,
				"mode":             "2770",
				"ownershipChanged": result.Changed,
				"existingGID":      result.ExistingGID,
			}).Info("Applied fsGroup ownership to directory-backed volume via management API")

			// Check if this volume was created from snapshot/clone and needs recursive ownership fix
			needsRecursiveOwnership := volumeContext["NeedsRecursiveOwnership"] == "true"
			if needsRecursiveOwnership && result.Changed {
				// Only run recursive ownership if:
				// 1. Volume was created from snapshot/clone (NeedsRecursiveOwnership=true)
				// 2. Directory ownership was actually changed (result.Changed=true)
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"sharedExportPath": sharedExportPath,
					"directoryPath":    directoryPath,
					"fsGroup":          fsGroup,
					"reason":           "volume_created_from_source_and_ownership_changed",
				}).Info("Applying recursive fsGroup ownership fix for volume created from snapshot/clone")

				if err := isiConfig.isiSvc.SetVolumeGroupOwnershipRecursive(ctx, sharedExportPath, directoryPath, gid, s.opts.ChownWorkers); err != nil {
					// Log error but don't fail the mount - directory-level ownership is already applied
					// This ensures pods can still function even if recursive fix fails
					csmlog.WithContext(ctx).WithFields(csmlog.Fields{
						"sharedExportPath": sharedExportPath,
						"directoryPath":    directoryPath,
						"fsGroup":          fsGroup,
						"error":            err,
					}).Error("Failed to apply recursive fsGroup ownership - directory-level ownership applied successfully. " +
						"Some existing files may have incorrect ownership. Manual intervention may be required.")
				} else {
					csmlog.WithContext(ctx).WithFields(csmlog.Fields{
						"sharedExportPath": sharedExportPath,
						"directoryPath":    directoryPath,
						"fsGroup":          fsGroup,
					}).Info("Successfully applied recursive fsGroup ownership to all files in volume")
				}
			} else if needsRecursiveOwnership && !result.Changed {
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"sharedExportPath": sharedExportPath,
					"directoryPath":    directoryPath,
					"fsGroup":          fsGroup,
					"existingGID":      result.ExistingGID,
					"optimization":     "skipped_recursive_scan",
				}).Info("Volume created from snapshot/clone but directory ownership unchanged - skipping expensive recursive scan")
			}
		} else {
			// Export-backed: apply fsGroup on the mount point when RootClientEnabled is "true".
			// A timeout protects the CSI driver from hanging on large directory trees.
			// A hidden state file inside the export records the relative paths already
			// chowned so that kubelet retries can resume, even within a directory.
			rootClientEnabled := volumeContext["RootClientEnabled"]
			fsType := ""
			if m := req.GetVolumeCapability().GetMount(); m != nil {
				fsType = m.GetFsType()
			}
			accessMode := csi.VolumeCapability_AccessMode_UNKNOWN
			if a := req.GetVolumeCapability().GetAccessMode(); a != nil {
				accessMode = a.GetMode()
			}
			if err := s.applyFSGroupToExportBackedVolume(ctx, stagingPath, fsGroup, rootClientEnabled, fsType, accessMode); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return nil, status.Errorf(codes.DeadlineExceeded, "recursive chown for fsGroup timed out after %ds", s.opts.ChownTimeoutSeconds)
				}
				return nil, status.Errorf(codes.Internal, "failed to apply fsGroup ownership to export-backed volume: %v", err)
			}

		}
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

// applyFSGroupPermissions applies the group ownership and kubelet-compatible permission
// bits to a single path within an os.Root. Files receive mode | 0660; directories receive
// mode | 0770 plus the setgid bit, matching kubelet's changeFilePermission behavior.
func applyFSGroupPermissions(root *os.Root, rel string, gid int, mode os.FileMode) error {
	if err := root.Lchown(rel, -1, gid); err != nil {
		return err
	}
	if mode&os.ModeSymlink != 0 {
		return nil
	}
	mask := os.FileMode(0o0660)
	if mode.IsDir() {
		mask |= os.FileMode(0o0110) | os.ModeSetgid
	}
	return root.Chmod(rel, mode|mask)
}

// applyFSGroupToExportBackedVolume performs a parallel, resumable recursive chown of the
// export-backed volume at the given path. It is called from both NodeStageVolume and
// NodePublishVolume so that fsGroup ownership is applied whether the volume is being
// staged for the first time or being published to a pod that reuses an already-staged
// volume.
func (s *service) applyFSGroupToExportBackedVolume(ctx context.Context, stagingPath, fsGroupStr, rootClientEnabled, fsType string, accessMode csi.VolumeCapability_AccessMode_Mode) error {
	if fsGroupStr == "" {
		return nil
	}

	// Validate fsGroup value early (fail fast) before any other logic
	gid, err := strconv.Atoi(fsGroupStr)
	if err != nil {
		return fmt.Errorf("invalid fsGroup value '%s': %w", fsGroupStr, err)
	}
	if gid <= 0 {
		return fmt.Errorf("invalid fsGroup value '%s': must be positive", fsGroupStr)
	}

	if rootClientEnabled != "true" {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"stagingPath":       stagingPath,
			"fsGroup":           fsGroupStr,
			"rootClientEnabled": rootClientEnabled,
		}).Debug("skipping fsGroup for export-backed volume: RootClientEnabled is not 'true', node cannot chown")
		return nil
	}
	if fsType == "" {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"stagingPath": stagingPath,
			"fsGroup":     fsGroupStr,
		}).Debug("skipping recursive fsGroup chown for export-backed volume: fsType not set")
		return nil
	}
	if accessMode != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER &&
		accessMode != csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"stagingPath": stagingPath,
			"fsGroup":     fsGroupStr,
			"accessMode":  accessMode,
		}).Debug("skipping recursive fsGroup chown for export-backed volume: access mode is not single-node")
		return nil
	}

	stateFileName := ".csi-powerscale-chown-state"
	if s.nodeID != "" {
		stateFileName = stateFileName + "." + s.nodeID
	}
	statePath := filepath.Join(stagingPath, stateFileName)
	stateMarker := fsGroupStr + ",m2"

	// Remove stale temp files left by a previous interrupted write.
	if matches, err := filepath.Glob(filepath.Join(stagingPath, stateFileName+".tmp.*")); err == nil {
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}

	// If the mount root already has the desired group, setgid, and 0770 permissions,
	// and no resumable state file exists, a previous chown completed and we can skip
	// the expensive walk.
	if info, err := os.Stat(stagingPath); err == nil {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			if int(stat.Gid) == gid && info.Mode()&os.ModeSetgid != 0 && info.Mode().Perm()&0o0770 == 0o0770 {
				if _, err := os.Stat(statePath); err != nil && os.IsNotExist(err) {
					return nil
				}
			}
		}
	}

	completed := make(map[string]bool)
	if data, err := os.ReadFile(statePath); err == nil && len(data) > 0 {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) >= 2 {
			if ts, err := time.Parse(time.RFC3339, lines[0]); err == nil {
				if time.Since(ts) <= 30*time.Minute {
					if lines[1] == stateMarker {
						for _, line := range lines[2:] {
							if line != "" {
								completed[line] = true
							}
						}
					}
				}
			}
		}
	}

	chownCtx, cancel := context.WithTimeout(ctx, time.Duration(s.opts.ChownTimeoutSeconds)*time.Second)
	defer cancel()

	if !s.opts.EnableDriverFSGroupChown {
		return s.applySequentialFSGroupChown(chownCtx, stagingPath, gid, stateFileName, fsGroupStr)
	}

	// Open a root-scoped handle to prevent symlink TOCTOU attacks (G122).
	root, err := getOsOpenRoot(stagingPath)
	if err != nil {
		return fmt.Errorf("failed to open root for chown: %w", err)
	}
	defer root.Close()

	// chownWorkers is the number of parallel os.Chown workers. More workers reduce wall-clock time while keeping NodeStageVolume under the gRPC timeout.
	chownWorkers := s.opts.ChownWorkers
	// writeBatch controls how often the resumable state file is written. A larger batch reduces I/O overhead while preserving enough progress to resume quickly.
	writeBatch := s.opts.ChownWriteBatch

	// chownJob is the work item sent from the walker to each fsGroup worker.
	type chownJob struct {
		rel   string
		isDir bool
		mode  os.FileMode
	}

	// jobs feeds paths to the worker pool; recordCh feeds completed file paths to the state writer.
	jobs := make(chan chownJob, 100)
	recordCh := make(chan string, writeBatch)
	errCh := make(chan error, 1)

	var chownMu sync.RWMutex
	var wg sync.WaitGroup
	var writerWg sync.WaitGroup

	// writeState atomically persists the set of already-chowned files to the in-mount state file.
	// It is used to resume a timed-out or interrupted chown on the next NodeStageVolume retry.
	writeState := func() error {
		chownMu.Lock()
		keys := make([]string, 0, len(completed))
		for k := range completed {
			keys = append(keys, k)
		}
		chownMu.Unlock()

		sort.Strings(keys)

		var sb strings.Builder
		sb.WriteString(time.Now().UTC().Format(time.RFC3339))
		sb.WriteString("\n")
		sb.WriteString(stateMarker)
		sb.WriteString("\n")
		for _, k := range keys {
			sb.WriteString(k)
			sb.WriteString("\n")
		}

		tmp, err := os.CreateTemp(stagingPath, stateFileName+".tmp.*")
		if err != nil {
			return err
		}
		_, err = tmp.WriteString(sb.String())
		if err1 := tmp.Close(); err1 != nil && err == nil {
			err = err1
		}
		if err != nil {
			os.Remove(tmp.Name())
			return err
		}
		if err := os.Rename(tmp.Name(), statePath); err != nil {
			os.Remove(tmp.Name())
			return err
		}
		return nil
	}

	// The writer goroutine batches completed file paths and writes the resumable state file.
	writerWg.Add(1)
	go func() {
		defer writerWg.Done()
		pendingWrites := 0
		for rel := range recordCh {
			chownMu.Lock()
			completed[rel] = true
			pendingWrites++
			if pendingWrites >= writeBatch {
				pendingWrites = 0
				chownMu.Unlock()
				_ = writeState()
			} else {
				chownMu.Unlock()
			}
		}
		chownMu.Lock()
		if pendingWrites > 0 {
			pendingWrites = 0
			chownMu.Unlock()
			_ = writeState()
		} else {
			chownMu.Unlock()
		}
	}()

	// Start a pool of goroutines to apply fsGroup permissions in parallel.
	for i := 0; i < chownWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if chownCtx.Err() != nil {
					return
				}
				if err := applyFSPermsFunc(root, job.rel, gid, job.mode); err != nil {
					cancel()
					select {
					case errCh <- err:
					default:
					}
					return
				}
				if !job.isDir {
					select {
					case recordCh <- job.rel:
					case <-chownCtx.Done():
					}
				}
			}
		}()
	}

	// The walker enumerates the directory tree and feeds each file/directory to a worker.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(jobs)

		walkErr := filepath.WalkDir(stagingPath, func(path string, d fs.DirEntry, err error) error {
			if chownCtx.Err() != nil {
				return chownCtx.Err()
			}
			if err != nil {
				return nil
			}

			rel, _ := filepath.Rel(stagingPath, path)

			// Apply fsGroup permissions to the root mount point directly; it is not recorded because the walker always re-enters it on resume.
			if rel == "." {
				info, err := root.Lstat(".")
				if err != nil {
					return err
				}
				return applyFSPermsFunc(root, ".", gid, os.FileMode(info.Mode()))
			}

			// Skip the resumable chown state files and any stale temp files so we do not chown ourselves.
			if strings.HasPrefix(d.Name(), ".csi-powerscale-chown-state") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}

			// Skip files already processed in a previous attempt. Directories are always re-entered so their contents can be checked.
			chownMu.RLock()
			done := completed[rel]
			chownMu.RUnlock()
			if done {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return err
			}
			select {
			case jobs <- chownJob{rel: rel, isDir: d.IsDir(), mode: os.FileMode(info.Mode())}:
			case <-chownCtx.Done():
				return chownCtx.Err()
			}
			return nil
		})

		if walkErr != nil && walkErr != context.Canceled && walkErr != context.DeadlineExceeded {
			cancel()
			select {
			case errCh <- walkErr:
			default:
			}
		}
	}()

	// Wait for the tree walk and all workers to finish, then flush any remaining completed files to state.
	wg.Wait()
	close(recordCh)
	writerWg.Wait()

	// Capture the first error reported by the walker or any worker.
	var chownErr error
	select {
	case chownErr = <-errCh:
	default:
	}

	// Return a real chown error, or report the timeout so kubelet retries and resumes from the persisted state.
	if chownErr != nil {
		return fmt.Errorf("recursive chown for fsGroup failed: %w", chownErr)
	}
	if chownCtx.Err() == context.DeadlineExceeded {
		return context.DeadlineExceeded
	}
	if chownCtx.Err() != nil {
		return fmt.Errorf("recursive chown for fsGroup canceled: %w", chownCtx.Err())
	}

	_ = os.Remove(statePath)
	if matches, err := filepath.Glob(filepath.Join(stagingPath, stateFileName+".tmp.*")); err == nil {
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}
	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		"stagingPath": stagingPath,
		"fsGroup":     fsGroupStr,
	}).Info("Applied fsGroup ownership to export-backed volume via recursive chown")
	return nil
}

// applySequentialFSGroupChown performs a simple sequential recursive chown of the export-backed volume.
// It is used as a fallback when X_CSI_ISILON_ENABLE_DRIVER_FSGROUP_CHOWN is false.
func (s *service) applySequentialFSGroupChown(ctx context.Context, stagingPath string, gid int, stateFileName, fsGroupStr string) error {
	statePath := filepath.Join(stagingPath, stateFileName)

	// Clean up any stale parallel-chown state/temp files left from a previous run.
	_ = os.Remove(statePath)
	if matches, err := filepath.Glob(filepath.Join(stagingPath, stateFileName+".tmp.*")); err == nil {
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}

	// Open a root-scoped handle to prevent symlink TOCTOU attacks (G122).
	root, err := getOsOpenRoot(stagingPath)
	if err != nil {
		return fmt.Errorf("failed to open root for chown: %w", err)
	}
	defer root.Close()

	chownErr := filepath.WalkDir(stagingPath, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}
		// Skip the resumable chown state files and any stale temp files so we do not chown ourselves.
		if strings.HasPrefix(d.Name(), ".csi-powerscale-chown-state") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(stagingPath, path)
		info, err := d.Info()
		if err != nil {
			return err
		}
		return applyFSPermsFunc(root, rel, gid, os.FileMode(info.Mode()))
	})

	if chownErr != nil {
		if chownErr == context.DeadlineExceeded {
			return context.DeadlineExceeded
		}
		return fmt.Errorf("recursive chown for fsGroup failed: %w", chownErr)
	}

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		"stagingPath": stagingPath,
		"fsGroup":     fsGroupStr,
	}).Info("Applied fsGroup ownership to export-backed volume via sequential recursive chown")
	return nil
}

func (s *service) NodeUnstageVolume(
	ctx context.Context,
	req *csi.NodeUnstageVolumeRequest) (
	*csi.NodeUnstageVolumeResponse, error,
) {
	s.logStatistics()

	// Validate request
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeID is required")
	}
	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "StagingTargetPath is required")
	}

	stagingPath := req.GetStagingTargetPath()

	// Parse volume ID to extract volume name for mount detection.
	// The volume name (not the full volumeID) is what appears in the NFS mount device path.
	volName, _, _, _, _ := id.ParseNormalizedVolumeID(ctx, req.GetVolumeId())
	if volName == "" {
		volName = req.GetVolumeId() // Fallback for non-normalized volume IDs
	}

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		"volumeId":    req.GetVolumeId(),
		"volName":     volName,
		"stagingPath": stagingPath,
	}).Info("NodeUnstageVolume: unmounting staging path")

	// Unmount staging path using volName as filterStr (not full volumeID).
	// This matches NodeUnpublishVolume behavior and ensures isVolumeMounted
	// can correctly detect the mount by matching against the NFS device path.
	if err := unpublishVolume(ctx, &csi.NodeUnpublishVolumeRequest{
		VolumeId:   req.GetVolumeId(),
		TargetPath: stagingPath,
	}, volName); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount staging path: %v", err)
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (s *service) NodePublishVolume(
	ctx context.Context,
	req *csi.NodePublishVolumeRequest) (
	*csi.NodePublishVolumeResponse, error,
) {
	fields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", fields["csi.requestid"])
	// set noProbeOnStart to false so subsequent calls can lead to probe
	noProbeOnStart.Store(false)

	volumeContext := req.GetVolumeContext()
	if volumeContext == nil {
		return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "VolumeContext is nil, skip NodePublishVolume"))
	}
	LogMap(ctx, "VolumeContext", volumeContext)

	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "VolumeCapability is required"))
	}
	mntVol := req.GetVolumeCapability().GetMount()
	if mntVol == nil {
		return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "Invalid access type: mount capability required"))
	}

	isEphemeralVolume := volumeContext["csi.storage.k8s.io/ephemeral"] == "true"
	var clusterName string
	var err error
	if isEphemeralVolume {
		// Do not honor user-supplied ClusterName for ephemeral volumes.
		// Force to empty string so getIsilonConfig uses the default cluster.
		// ClusterName in volumeAttributes is user-controlled and untrusted.
		clusterName = ""
		if volumeContext["ClusterName"] != "" {
			csmlog.WithContext(ctx).Infof("Ignoring requested ClusterName '%s' for ephemeral volume; using default cluster",
				volumeContext["ClusterName"])
		}
	} else {
		// parse the input volume id and fetch it's components
		_, _, _, clusterName, _ = id.ParseNormalizedVolumeID(ctx, req.GetVolumeId())
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		return nil, err
	}

	f := csmlog.Fields{
		csmlog.FieldOperation: "NodePublishVolume",
		csmlog.FieldVolumeID:  req.GetVolumeId(),
		csmlog.FieldArrayID:   clusterName,
	}
	csmlog.WithContext(ctx).WithFields(f).Info("NodePublishVolume called")
	csmlog.WithContext(ctx).Debugf("Cluster Name: %v", clusterName)

	// Probe the node if required and make sure startup called
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		csmlog.WithContext(ctx).Error("nodeProbe failed with error :" + err.Error())
		return nil, err
	}

	if isEphemeralVolume {
		return s.ephemeralNodePublish(ctx, req)
	}

	// Detect directory-backed mode: first try parsing from volume ID (self-contained),
	// then fall back to VolumeContext for backward compatibility
	_, _, _, _, provisioningMode, parseErr := id.ParseVolumeIDWithMode(ctx, req.GetVolumeId())
	isDirectoryBacked := (provisioningMode == id.ProvisioningModeDirectory)
	if parseErr == nil && provisioningMode == "" {
		// Volume ID parsed successfully but has no mode token; check VolumeContext as fallback
		if volumeContext["ProvisioningMode"] == "directory" {
			isDirectoryBacked = true
			provisioningMode = "directory"
			csmlog.WithContext(ctx).Debugf("Directory-backed mode detected via VolumeContext fallback for volume %s", req.GetVolumeId())
		}
	} else if parseErr != nil {
		csmlog.WithContext(ctx).Debugf("Failed to parse volume ID with mode: %v, using VolumeContext fallback", parseErr)
		provisioningMode = volumeContext["ProvisioningMode"]
		isDirectoryBacked = (provisioningMode == "directory")
	}

	var path string
	var volName string

	if isDirectoryBacked {
		// Directory-backed: construct mount path from shared export + directory
		sharedExportPath := volumeContext["SharedExportPath"]
		directoryPath := volumeContext["DirectoryPath"]

		if sharedExportPath == "" {
			return nil, status.Error(codes.FailedPrecondition, GetMessageWithReqID(runID, "no entry keyed by 'SharedExportPath' found in VolumeContext for directory-backed volume '%s'", req.GetVolumeId()))
		}
		if directoryPath == "" {
			return nil, status.Error(codes.FailedPrecondition, GetMessageWithReqID(runID, "no entry keyed by 'DirectoryPath' found in VolumeContext for directory-backed volume '%s'", req.GetVolumeId()))
		}

		// Mount path is SharedExportPath/DirectoryPath
		path = isilonfs.GetPathForVolume(sharedExportPath, directoryPath)
		volName = directoryPath

		logFields := csmlog.Fields{
			"ProvisioningMode": "directory",
			"SharedExportPath": sharedExportPath,
			"DirectoryPath":    directoryPath,
			"ConstructedPath":  path,
		}
		if fsGroup := volumeContext["FsGroup"]; fsGroup != "" {
			logFields["FsGroup"] = fsGroup
		}
		csmlog.WithContext(ctx).WithFields(logFields).Info("Directory-backed volume detected in NodePublishVolume")
	} else {
		// Export-backed: use original logic
		path = volumeContext["Path"]
		if path == "" {
			return nil, status.Error(codes.FailedPrecondition, GetMessageWithReqID(runID, "no entry keyed by 'Path' found in VolumeContext of volume id : '%s', name '%s', skip NodePublishVolume", req.GetVolumeId(), volumeContext["name"]))
		}
		volName = volumeContext["Name"]
		if volName == "" {
			return nil, status.Error(codes.FailedPrecondition, GetMessageWithReqID(runID, "no entry keyed by 'Name' found in VolumeContext of volume id : '%s', name '%s', skip NodePublishVolume", req.GetVolumeId(), volumeContext["name"]))
		}
	}

	accessZone := volumeContext["AccessZone"]
	isROVolumeFromSnapshot := isiConfig.isiSvc.isROVolumeFromSnapshot(path, accessZone)
	if isROVolumeFromSnapshot {
		csmlog.WithContext(ctx).Info("Volume source is snapshot")
		if export, err := isiConfig.isiSvc.GetExportWithPathAndZone(ctx, path, accessZone); err != nil || export == nil {
			return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "error retrieving export for %s", path))
		}
	} else {
		// Parse the target path and empty volume name to get the volume
		isiPath := isilonfs.GetIsiPathFromExportPath(path)

		if _, err := s.getVolByName(ctx, isiPath, volName, isiConfig); err != nil {
			csmlog.WithContext(ctx).Errorf("Error in getting '%s' Volume '%v'", volName, err)
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
		csmlog.WithContext(ctx).Debugf("Authorization is enabled, reading MountEndpoint: '%s'", isiConfig.MountEndpoint)

		azServiceIP = isiConfig.MountEndpoint
	}

	// Extract mTLS parameters from volume context
	smartConnectZoneFQDN := volumeContext[constants.SmartConnectZoneFQDNParam]

	// NFSTransportSecurity is StorageClass-only (no Secret or environment inheritance)
	nfsTransportSecurity := volumeContext[constants.NFSTransportSecurityParam]

	// Resolve the mount target FQDN using 3-layer precedence:
	// 1. StorageClass parameter SmartConnectZoneFQDN (highest)
	// 2. Cluster Secret field nfsMountFQDN
	// 3. Environment variable X_CSI_ISI_NFS_MOUNT_FQDN (lowest)
	// Note: Only FQDN has this fallback chain; transport security does not.
	envNFSMountFQDN := os.Getenv(constants.EnvNFSMountFQDN)
	resolvedMountTarget := ResolveMountFQDN(smartConnectZoneFQDN, isiConfig.NFSMountFQDN, envNFSMountFQDN)
	volumeContext = withResolvedMTLSVolumeContext(volumeContext, resolvedMountTarget, nfsTransportSecurity)

	// Determine the final mount target
	mountTarget := azServiceIP
	if resolvedMountTarget != "" {
		mountTarget = resolvedMountTarget
		csmlog.WithContext(ctx).Infof("NodePublishVolume: Using resolved FQDN for mount target: %s (FQDN precedence: SC=%s, Secret=%s, env=%s)",
			resolvedMountTarget, smartConnectZoneFQDN, isiConfig.NFSMountFQDN, envNFSMountFQDN)
	}

	// Validate mTLS mount target requirements
	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()
	if err := s.validateMTLSPrerequisites(ctx, runID, nfsTransportSecurity, mountTarget, resolvedMountTarget, accessZone, mountOptions, volumeContext); err != nil {
		return nil, err
	}

	publishFields := csmlog.Fields{
		"ID":          req.VolumeId,
		"Name":        volumeContext["Name"],
		"TargetPath":  req.GetTargetPath(),
		"AzServiceIP": azServiceIP,
	}
	if IsTLSEnabled(nfsTransportSecurity) {
		publishFields["MountTarget"] = mountTarget
		publishFields["NFSTransportSecurity"] = nfsTransportSecurity
	}

	// Check if volume is already staged (STAGE_UNSTAGE capability enabled)
	stagingPath := req.GetStagingTargetPath()
	if stagingPath != "" {
		// Volume is staged - perform bind-mount from staging path to target path
		publishFields["StagingPath"] = stagingPath
		publishFields["MountType"] = "bind"

		// Extract mount options and readonly flag (mirrors publishVolume logic)
		// Note: VolumeCapability and Mount already validated at function entry (lines 989-994)

		// Apply fsGroup ownership to an already-staged export-backed volume when a pod
		// with a volume_mount_group reuses the staged global mount. NodeStageVolume may not be called again.
		if !isDirectoryBacked {
			fsType := mntVol.GetFsType()
			fsGroup := mntVol.GetVolumeMountGroup()
			rootClientEnabled := volumeContext["RootClientEnabled"]
			accessMode := csi.VolumeCapability_AccessMode_UNKNOWN
			if a := req.GetVolumeCapability().GetAccessMode(); a != nil {
				accessMode = a.GetMode()
			}
			if err := s.applyFSGroupToExportBackedVolume(ctx, stagingPath, fsGroup, rootClientEnabled, fsType, accessMode); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return nil, status.Errorf(codes.DeadlineExceeded, "recursive chown for fsGroup timed out after %ds", s.opts.ChownTimeoutSeconds)
				}
				return nil, status.Errorf(codes.Internal, "failed to apply fsGroup ownership to export-backed volume: %v", err)
			}
		}

		// Start with user-provided mount flags
		mntOptions := mntVol.GetMountFlags()

		// Add readonly flag if requested
		roFlag := req.GetReadonly()
		if roFlag {
			mntOptions = append(mntOptions, "ro")
			publishFields["ReadOnly"] = true
		} else {
			mntOptions = append(mntOptions, "rw")
		}

		// Always include "bind" for bind-mounts
		mntOptions = append(mntOptions, "bind")

		publishFields["MountOptions"] = mntOptions
		csmlog.WithContext(ctx).WithFields(publishFields).Info("NodePublishVolume: performing bind-mount from staging path")

		// Create target directory
		targetPath := req.GetTargetPath()
		if _, err := mkdir(ctx, targetPath); err != nil {
			return nil, status.Errorf(codes.FailedPrecondition, "could not create target path '%s': %v", targetPath, err)
		}

		// Perform bind-mount with correct options
		mountFunc := getMountFunc()
		if err := mountFunc(ctx, stagingPath, targetPath, "", mntOptions...); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to bind-mount from staging path: %v", err)
		}

		csmlog.WithContext(ctx).WithFields(publishFields).Info("NodePublishVolume: bind-mount completed successfully")
	} else {
		// No staging path - perform direct network mount (backward compatibility)
		publishFields["MountType"] = "direct"
		csmlog.WithContext(ctx).WithFields(publishFields).Info("NodePublishVolume: performing direct NFS mount")

		// Apply TLS handshake timeout for mTLS mounts
		mountCtx, mountCancel := CreateMountTimeoutContext(ctx, nfsTransportSecurity)
		defer mountCancel()

		// Update the request's VolumeContext to use the resolved context.
		// This ensures publishVolume sees the effective mTLS settings.
		// We assign the map reference directly to avoid copying the protobuf
		// struct which contains a mutex.
		req.VolumeContext = volumeContext
		if err := publishVolume(mountCtx, req, isiConfig.isiSvc.GetNFSExportURLForPath(mountTarget, path)); err != nil {
			// Classify TLS errors and emit Kubernetes events for mTLS mount failures
			if IsMTLSEnabled(nfsTransportSecurity) && IsTLSError(err) {
				classification := ParseTLSError(ctx, err)

				LogMTLSMountOperation(ctx, MTLSLogFields{
					MountTarget:     mountTarget,
					MountTargetFQDN: resolvedMountTarget,
					TLSEnabled:      true,
					AccessZone:      accessZone,
					MountOptions:    strings.Join(mntVol.GetMountFlags(), ","),
					Outcome:         "failed",
					ErrorCode:       classification.ErrorCode,
					ErrorMessage:    classification.Message,
					NodeID:          s.nodeID,
				})

				pvcNamespace := volumeContext[csiPersistentVolumeClaimNamespace]
				pvcName := volumeContext[csiPersistentVolumeClaimName]
				if pvcNamespace != "" && pvcName != "" {
					_ = EmitMTLSEvent(ctx, s.k8sclient, pvcNamespace, pvcName, classification.Reason, classification.Message)
				}

				return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "mTLS mount failed: %s", classification.Message))
			}

			// Check if mount timed out during mTLS handshake
			if IsMTLSEnabled(nfsTransportSecurity) && IsTimeoutError(err) {
				timeoutMsg := fmt.Sprintf("TLS handshake timed out after %d seconds for mount target '%s'. "+
					"Verify network connectivity and TLS daemon (tlshd) are operational.",
					GetTLSHandshakeTimeout(ctx), mountTarget)

				LogMTLSMountOperation(ctx, MTLSLogFields{
					MountTarget:     mountTarget,
					MountTargetFQDN: resolvedMountTarget,
					TLSEnabled:      true,
					AccessZone:      accessZone,
					MountOptions:    strings.Join(mntVol.GetMountFlags(), ","),
					Outcome:         "failed",
					ErrorCode:       "DeadlineExceeded",
					ErrorMessage:    timeoutMsg,
					NodeID:          s.nodeID,
				})

				pvcNamespace := volumeContext[csiPersistentVolumeClaimNamespace]
				pvcName := volumeContext[csiPersistentVolumeClaimName]
				if pvcNamespace != "" && pvcName != "" {
					_ = EmitMTLSEvent(ctx, s.k8sclient, pvcNamespace, pvcName, constants.TLSEventReasonHandshakeTimeout, timeoutMsg)
				}

				return nil, status.Error(codes.DeadlineExceeded, GetMessageWithReqID(runID, "%s", timeoutMsg))
			}

			return nil, err
		}
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func sanitizeEphemeralCreateVolumeParams(params map[string]string) map[string]string {
	safeParams := make(map[string]string, len(params))
	for k, v := range params {
		switch k {
		case IsiPathParam, AccessZoneParam, IsiVolumePathPermissionsParam,
			ClusterNameParam, AzServiceIPParam, RootClientEnabledParam:
			continue
		default:
			safeParams[k] = v
		}
	}
	return safeParams
}

func (s *service) NodeUnpublishVolume(
	ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest) (
	*csi.NodeUnpublishVolumeResponse, error,
) {
	fields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", fields["csi.requestid"])

	csmlog.WithContext(ctx).Debug("executing NodeUnpublishVolume")
	// set noProbeOnStart to false so subsequent calls can lead to probe
	noProbeOnStart.Store(false)
	volID := req.GetVolumeId()
	if volID == "" {
		return nil, status.Error(codes.FailedPrecondition, GetMessageWithReqID(runID, "no VolumeID found in request"))
	}
	csmlog.WithContext(ctx).Infof("The volume ID fetched from NodeUnPublish req is %s", volID)

	volName, exportID, accessZone, clusterName, _ := id.ParseNormalizedVolumeID(ctx, req.GetVolumeId())
	if volName == "" {
		volName = volID
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error %v", err.Error())
		return nil, err
	}

	f := csmlog.Fields{
		csmlog.FieldOperation: "NodeUnpublishVolume",
		csmlog.FieldVolumeID:  volID,
		csmlog.FieldArrayID:   clusterName,
	}
	csmlog.WithContext(ctx).WithFields(f).Info("NodeUnpublishVolume called")
	csmlog.WithContext(ctx).Debugf("Cluster Name: %v", clusterName)

	// Probe the node if required
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		csmlog.WithContext(ctx).Error("nodeProbe failed with error :" + err.Error())
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

	csmlog.WithContext(ctx).Infof("Ephemeral volume check: %t", isEphemeralVolume)

	// Check if it is a RO volume from snapshot
	// We need not execute this logic for ephemeral volumes.
	if !isExportIDEmpty {
		export, err := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone)
		if err != nil {
			// Export doesn't exist - this is OK during unpublish
			// Log it but don't fail the operation
			csmlog.WithContext(ctx).Infof("Export ID %d not found during unpublish (may already be cleaned up): %v", exportID, err)
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
		csmlog.WithContext(ctx).Errorf("Error while calling Unbuplish Volume %v", err.Error())
		return nil, err
	}

	if isEphemeralVolume {
		req.VolumeId = string(data)
		err := s.ephemeralNodeUnpublish(ctx, req)
		if err != nil {
			csmlog.WithContext(ctx).Errorf("Error while calling Ephemeral Node Unpublish  %v", err.Error())
			return nil, err
		}
	}
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *service) nodeProbe(ctx context.Context, isiConfig *IsilonClusterConfig) error {
	if err := s.validateOptsParameters(isiConfig); err != nil {
		return fmt.Errorf("node probe failed : '%v'", err)
	}

	if isiConfig.isiSvc == nil {
		var err error
		isiConfig.isiSvc, err = s.GetIsiService(ctx, isiConfig, csmlog.GetLevel())
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

	f := csmlog.Fields{
		csmlog.FieldOperation: "nodeProbe",
		csmlog.FieldArrayID:   isiConfig.ClusterName,
	}
	csmlog.WithContext(ctx).WithFields(f).Info("node probe succeeded")

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
					Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
				},
			},
		},
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
				},
			},
		},
		{
			// Advertise VOLUME_MOUNT_GROUP so the CO delegates fsGroup
			// application to the driver (via VolumeMountGroup) instead of
			// performing its own chown over the NFS mount. Directory-backed
			// volumes apply it via the OneFS management API; export-backed
			// volumes are unaffected (no-op).
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_VOLUME_MOUNT_GROUP,
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
	nodeID, err := s.getPowerScaleNodeID(ctx)
	csmlog.WithContext(ctx).Infof("Node ID of worker node is '%s'", nodeID)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to create Node ID with error %v", err.Error())
		return nil, err
	}

	// Non-fatal mTLS readiness diagnostic. This logs whether the node has the
	// kernel TLS module and tlshd daemon required for mTLS mounts. It never blocks
	// registration: mTLS is opt-in and a node without these dependencies remains
	// fully usable for plain NFS.
	_ = LogTLSReadiness(ctx)
	if noProbeOnStart.Load() {
		csmlog.WithContext(ctx).Debugf("noProbeOnStart is set to true, skip probe")
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

	// NOTE: TLS capability topology label (csi-isilon.dellemc.com/tls-capable) is NOT
	// auto-applied by the driver. Customers who require mTLS-aware scheduling should
	// manually label nodes (kubectl label node <name> csi-isilon.dellemc.com/tls-capable=true)
	// and use StorageClass allowedTopologies for node selection. The mount-time TLS
	// capability validation in NodePublishVolume remains as the fail-safe check.

	// Check for node label 'max-isilon-volumes-per-node'. If present set 'MaxVolumesPerNode' to this value.
	// If node label is not present, set 'MaxVolumesPerNode' to default value i.e., 0
	var maxIsilonVolumesPerNode int64
	labels, err := s.GetNodeLabels()
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get Node Labels with error %v", err.Error())
		return nil, err
	}

	if val, ok := labels["max-isilon-volumes-per-node"]; ok {
		maxIsilonVolumesPerNode, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value '%s' specified for 'max-isilon-volumes-per-node' node label", val)
		}
		csmlog.WithContext(ctx).Infof("node label 'max-isilon-volumes-per-node' is available and is set to value '%v'", maxIsilonVolumesPerNode)
	} else {
		// As per the csi spec the plugin MUST NOT set negative values to
		// 'MaxVolumesPerNode' in the NodeGetInfoResponse response
		if s.opts.MaxVolumesPerNode < 0 {
			return nil, fmt.Errorf("maxIsilonVolumesPerNode MUST NOT be set to negative value")
		}
		maxIsilonVolumesPerNode = s.opts.MaxVolumesPerNode
		csmlog.WithContext(ctx).Infof("node label 'max-isilon-volumes-per-node' is not available. Using default volume limit '%v'", maxIsilonVolumesPerNode)
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
	fields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", fields["csi.requestid"])

	volID := req.GetVolumeId()
	if volID == "" {
		return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "no VolumeID found in request"))
	}
	volPath := req.GetVolumePath()
	if volPath == "" {
		return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "no Volume Path found in request"))
	}

	volName, exportID, accessZone, clusterName, _ := id.ParseNormalizedVolumeID(ctx, volID)
	if volName == "" {
		volName = volID
	}

	// Check if given volume exists
	// Create copy of isiconfig so we don't modify the original
	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		return nil, err
	}

	isiConfigCopy := &IsilonClusterConfig{
		ClusterName:               isiConfig.ClusterName,
		Endpoint:                  isiConfig.Endpoint,
		EndpointPort:              isiConfig.EndpointPort,
		MountEndpoint:             isiConfig.MountEndpoint,
		EndpointURL:               isiConfig.EndpointURL,
		accessZone:                isiConfig.accessZone,
		User:                      isiConfig.User,
		Password:                  isiConfig.Password,
		SkipCertificateValidation: isiConfig.SkipCertificateValidation,
		IsiPath:                   isiConfig.IsiPath,
		IsiVolumePathPermissions:  isiConfig.IsiVolumePathPermissions,
		IsDefault:                 isiConfig.IsDefault,
		ReplicationCertificateID:  isiConfig.ReplicationCertificateID,
		IgnoreUnresolvableHosts:   isiConfig.IgnoreUnresolvableHosts,
		isiSvc:                    isiConfig.isiSvc,
	}

	// save the isiPath var, if we cannot find another isiPath tied to the vol, we will use this one
	isiPath := isiConfigCopy.IsiPath

	// first we check pv, sc for isiPath
	// if we cannot find it there, we check the export
	isiPathFromParams, err := s.validateIsiPath(ctx, volName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get isiPath %v", err.Error())
		// if not in pv or sc, calculate it from the export
		exportPath, err := getExportPathFromExportID(ctx, isiConfig, exportID, accessZone)
		if err != nil {
			csmlog.WithContext(ctx).Debugf("Failed to get export path: %s, using default: %s", err.Error(), isiPath)
		} else {
			isiPathFromParams = isilonfs.GetIsiPathFromExportPath(exportPath)
		}
	}
	if isiPathFromParams != "" {
		csmlog.WithContext(ctx).Debugf("Found IsiPath from PV/SC/Export: %v ", isiPathFromParams)
		isiPath = isiPathFromParams
		// set service to utilize new path
		isiConfigCopy.IsiPath = isiPath
		isiConfigCopy.isiSvc, err = s.GetIsiService(ctx, isiConfigCopy, csmlog.GetLevel())
		if err != nil {
			csmlog.WithContext(ctx).Errorf("NodeGetVolumeStats: Failed to get isiService %v ", err.Error())
			return nil, err
		}
	}

	// Probe the node if required and make sure startup called
	if err := s.autoProbe(ctx, isiConfigCopy); err != nil {
		csmlog.WithContext(ctx).Error("nodeProbe failed with error :" + err.Error())
		return nil, err
	}

	isiVol, err := isiConfigCopy.isiSvc.GetVolume(ctx, "", volName)
	if err != nil || isiVol == nil {
		return nil, status.Error(codes.NotFound, GetMessageWithReqID(runID, "volume %v does not exist at path %v", volName, volPath))
	}

	// check whether the original volume is mounted
	isMounted, _ := getIsVolumeMounted(ctx, volName, volPath)
	if !isMounted {
		return nil, status.Error(codes.NotFound, GetMessageWithReqID(runID, "no volume is mounted at path: %s", volPath))
	}

	// check whether volume path is accessible
	_, err = getOsReadDir(volPath)
	if err != nil {
		return nil, status.Error(codes.NotFound, GetMessageWithReqID(runID, "volume path is not accessible: %s", err))
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
				Message:  fmt.Sprintf("failed to get volume stats metrics : %s", err),
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
	csmlog.WithContext(ctx).Info("Received request to node publish Ephemeral Volume..")

	volID := req.GetVolumeId()
	volName := fmt.Sprintf("ephemeral-%s", volID)
	createVolumeFunc := getCreateVolumeFunc(s)
	createEphemeralVolResp, err := createVolumeFunc(ctx, &csi.CreateVolumeRequest{
		Name:               volName,
		VolumeCapabilities: []*csi.VolumeCapability{req.VolumeCapability},
		Parameters:         sanitizeEphemeralCreateVolumeParams(req.VolumeContext),
		Secrets:            req.Secrets,
	})
	if err != nil {
		csmlog.WithContext(ctx).Error("Create ephemeral volume failed with error :" + err.Error())
		return nil, err
	}
	filePath := req.TargetPath + "/" + volName
	csmlog.WithContext(ctx).Infof("Ephemeral Volume %s creation was successful %s", volID, createEphemeralVolResp)

	// Build nodeUnPublish object for rollbacks
	nodeUnpublishRequest := &csi.NodeUnpublishVolumeRequest{
		VolumeId:   createEphemeralVolResp.Volume.VolumeId,
		TargetPath: req.TargetPath,
	}

	nodeID, err := s.getPowerScaleNodeID(ctx)
	if err != nil {
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
		csmlog.WithContext(ctx).Error("Need to rollback because ControllerPublish ephemeral volume failed with error :" + err.Error())
		if rollbackError := s.ephemeralNodeUnpublish(ctx, nodeUnpublishRequest); rollbackError != nil {
			csmlog.WithContext(ctx).Error("Rollback failed with error :" + err.Error())
			return nil, err
		}
		return nil, err
	}
	csmlog.WithContext(ctx).Infof("Ephemeral ControllerPublish for volume %s was successful %v", volID, controllerPublishEphemeralVolResp)

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
		csmlog.WithContext(ctx).Error("Need to rollback because NodePublish ephemeral volume failed with error :" + err.Error())
		if rollbackError := s.ephemeralNodeUnpublish(ctx, nodeUnpublishRequest); rollbackError != nil {
			csmlog.WithContext(ctx).Error("Rollback failed with error :" + err.Error())
			return nil, err
		}
		return nil, err
	}
	csmlog.WithContext(ctx).Infof("NodePublish step for volume %s was successful", volID)

	if _, err := statFileFunc(filePath); os.IsNotExist(err) {
		csmlog.WithContext(ctx).Infof("path %s does not exists", filePath)
		err = mkDirAllFunc(filePath, 0o750)
		if err != nil {
			csmlog.WithContext(ctx).Error("Create directory in target path for ephemeral vol failed with error :" + err.Error())
			if rollbackError := s.ephemeralNodeUnpublish(ctx, nodeUnpublishRequest); rollbackError != nil {
				csmlog.WithContext(ctx).Error("Rollback failed with error :" + err.Error())
				return nil, err
			}
			return nil, err
		}
	}
	csmlog.WithContext(ctx).Infof("Created dir in target path %s", filePath)

	f, err := createFileFunc(filepath.Clean(filePath) + "/id")
	if err != nil {
		csmlog.WithContext(ctx).Error("Create id file in target path for ephemeral vol failed with error :" + err.Error())
		if rollbackError := s.ephemeralNodeUnpublish(ctx, nodeUnpublishRequest); rollbackError != nil {
			csmlog.WithContext(ctx).Error("Rollback failed with error :" + err.Error())
			return nil, err
		}
		return nil, err
	}
	csmlog.WithContext(ctx).Infof("Created file in target path %s", filePath+"/id")

	defer func() {
		if err := closeFileFunc(f); err != nil {
			csmlog.WithContext(ctx).Errorf("Error closing file: %s \n", err)
		}
	}()
	_, err2 := writeStringFunc(f, createEphemeralVolResp.Volume.VolumeId)
	if err2 != nil {
		csmlog.WithContext(ctx).Error("Writing to id file in target path for ephemeral vol failed with error :" + err2.Error())
		if rollbackError := s.ephemeralNodeUnpublish(ctx, nodeUnpublishRequest); rollbackError != nil {
			csmlog.WithContext(ctx).Error("Rollback failed with error :" + rollbackError.Error())
		}
		return nil, err2
	}
	csmlog.WithContext(ctx).Infof("Ephemeral Node Publish was successful...")

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
	fields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", fields["csi.requestid"])

	csmlog.WithContext(ctx).Infof("Request received for Ephemeral NodeUnpublish..")
	volumeID := req.GetVolumeId()
	csmlog.WithContext(ctx).Infof("The volID is %s", volumeID)
	if volumeID == "" {
		return status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "volume ID is required"))
	}

	nodeID, nodeIDErr := s.getPowerScaleNodeID(ctx)
	if nodeIDErr != nil {
		return nodeIDErr
	}

	_, err := s.ControllerUnpublishVolume(ctx, &csi.ControllerUnpublishVolumeRequest{
		VolumeId: volumeID,
		NodeId:   nodeID,
	})
	if err != nil {
		csmlog.WithContext(ctx).Error("ControllerUnPublish ephemeral volume failed with error :" + err.Error())
		return err
	}
	csmlog.WithContext(ctx).Infof("Controller UnPublish for Ephemeral inline volume %s sucessful..", volumeID)

	// Before deleting the volume on PowerScale,
	// Cleaning up the directories we created.
	volName, _, _, _, err := id.ParseNormalizedVolumeID(ctx, req.GetVolumeId())
	if err != nil {
		return err
	}
	tmpPath := req.TargetPath + "/" + volName
	csmlog.WithContext(ctx).Infof("Going to clean up the temporary directory on path %s", tmpPath)
	err = os.RemoveAll(tmpPath)
	if err != nil {
		return errors.New("failed to cleanup lock files")
	}

	_, err = s.DeleteVolume(ctx, &csi.DeleteVolumeRequest{
		VolumeId: volumeID,
	})
	if err != nil {
		csmlog.WithContext(ctx).Error("Delete ephemeral volume failed with error :" + err.Error())
		return err
	}
	csmlog.WithContext(ctx).Infof("Delete volume for Ephemeral inline volume %s successful..", volumeID)

	return nil
}

func (s *service) getPowerScaleNodeID(ctx context.Context) (string, error) {
	var nodeIP string
	var err error

	// When valid list of allowedNetworks is being given as part of values.yaml, we need
	// to fetch first IP from matching network
	if len(s.opts.allowedNetworks) > 0 && s.opts.allowedNetworksMode == constants.AllowedNetworksModeDefault {
		// Single mode: prefer the management IP (X_CSI_NODE_IP) if it matches an allowed network.
		// This avoids picking a secondary interface IP that the node may not use as the NFS source
		// address, which would cause mount access denied errors on multi-homed nodes.
		nodeIP, err = s.GetCSINodeIP()
		if err != nil {
			csmlog.WithContext(ctx).Debugf("Management IP not available, falling back to allowed network IP: %v", err)
		} else {
			nodeIPMatches := false
			for _, cidr := range s.opts.allowedNetworks {
				if csiutils.IPInCIDR(nodeIP, cidr) {
					nodeIPMatches = true
					break
				}
			}
			if !nodeIPMatches {
				nodeIP = ""
			}
		}
		if nodeIP == "" {
			csmlog.WithContext(ctx).Debugf("Fetching IP address of custom network for NFS I/O traffic")
			nodeIP, err = csiutils.GetNFSClientIP(s.opts.allowedNetworks)
			if err != nil {
				csmlog.WithContext(ctx).Errorf("Failed to find IP address corresponding to the allowed network with error %v", err.Error())
				return "", err
			}
		}
	} else {
		// Multi mode or no allowedNetworks: use management IP (X_CSI_NODE_IP) for stable node ID
		nodeIP, err = s.GetCSINodeIP()
		if err != nil {
			return "", err
		}
	}

	nodeFQDN, err := getUtilsGetFQDNByIP(ctx, nodeIP)
	if err != nil {
		nodeFQDN = nodeIP
		csmlog.WithContext(ctx).Warnf("Setting nodeFQDN to %s as failed to resolve IP to FQDN due to %v", nodeIP, err)
	}

	nodeID, err := s.GetCSINodeID()
	if err != nil {
		return "", err
	}

	nodeID = nodeID + id.NodeIDSeparator + nodeFQDN + id.NodeIDSeparator + nodeIP

	return nodeID, nil
}

func (s *service) ReconcileNodeAzLabels(ctx context.Context) error {
	addrs, err := getInterfaceAddrsFunc()()
	if err != nil {
		csmlog.WithContext(ctx).Errorf("could not get network interface addresses: '%v'", err.Error())
		return err
	}

	labelsToAdd := make(map[string]string)
	for _, addr := range addrs {
		switch v := addr.(type) {
		case *net.IPNet:
			if v.IP.To4() != nil && !v.IP.IsLoopback() {
				ip, cnet, err := net.ParseCIDR(addr.String())
				if err != nil {
					csmlog.WithContext(ctx).Errorf("encountered error while parsing IP address %v", addr)
				} else {
					sanitizedNet := strings.ReplaceAll(cnet.String(), "/", "-")
					key := fmt.Sprintf("%s/az-%s-%s", constants.PluginName, sanitizedNet, ip.String())
					labelsToAdd[key] = "true"
					csmlog.WithContext(ctx).Debugf("discovered label %s -> %s", key, labelsToAdd[key])
				}
			}
		}
	}

	labels, err := getNodeLabelsFunc(s)()
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get node labels %v", err.Error())
	}

	labelsToRemove := make([]string, 0)
	for k := range labels {
		if strings.HasPrefix(k, constants.PluginName+"/az-") {
			if _, ok := labelsToAdd[k]; !ok {
				labelsToRemove = append(labelsToRemove, k)
			}
		}
	}

	if nodeLabelsNeedPatching(labels, labelsToAdd, labelsToRemove) {
		err = getPatchNodeLabelsFunc(s)(labelsToAdd, labelsToRemove)
		if err != nil {
			csmlog.WithContext(ctx).Errorf("failed to patch node labels %v", err.Error())
			return err
		}
		csmlog.WithContext(ctx).Debugf("reconciled node network labels, added: %v, removed: %v", labelsToAdd, labelsToRemove)
	}

	return nil
}

func nodeLabelsNeedPatching(labels, labelsToAdd map[string]string, labelsToRemove []string) bool {
	for k, v := range labelsToAdd {
		if labels[k] != v {
			return true
		}
	}

	for _, k := range labelsToRemove {
		if _, ok := labels[k]; ok {
			return true
		}
	}
	return false
}
