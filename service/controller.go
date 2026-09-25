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
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	fPath "path"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	id "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/identifiers"
	strutil "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/string-utils"

	isilonfs "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/powerscale-fs"
	csiutils "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/csi-utils"
	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	isiApi "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api"
	v1 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v1"
	v2 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v2"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// RPOEnum represents valid rpo values
type RPOEnum string

// constants
const (
	errUnknownAccessType             = "unknown access type is not Mount"
	errUnknownAccessMode             = "unknown or unsupported access mode"
	errNoSingleNodeReader            = "Single node only reader access mode is not supported"
	errNoMultiNodeSingleWriter       = "Multi node single writer access mode is not supported"
	MaxRetries                       = 10
	RetrySleepTime                   = 1000 * time.Millisecond
	AccessZoneParam                  = "AccessZone"
	ExportPathParam                  = "Path"
	IsiPathParam                     = "IsiPath"
	IsiVolumePathPermissionsParam    = "IsiVolumePathPermissions"
	AzServiceIPParam                 = "AzServiceIP"
	AzNetwork                        = "AzNetwork"
	RootClientEnabledParam           = "RootClientEnabled"
	RootClientEnabledParamDefault    = "false"
	SnapshotIQLicenseID              = "SNAPSHOTIQ"
	DeleteSnapshotMarker             = "DELETE_SNAPSHOT"
	IgnoreDotAndDotDotSubDirs        = 2
	ClusterNameParam                 = "ClusterName"
	SoftLimitParam                   = "SoftLimit"
	SoftLimitParamDefault            = ""
	AdvisoryLimitParam               = "AdvisoryLimit"
	AdvisoryLimitParamDefault        = ""
	SoftGracePrdParam                = "SoftGracePrd"
	SoftGracePrdParamDefault         = ""
	WritableFromSnapshotParam        = "csi-isilon.dellemc.com/writable-from-snapshot"
	WritableFromSnapshotParamDefault = "false"

	// Parameters to set quota limit from pvc
	PVCSoftLimitParam     = "pvcSoftLimit"
	PVCAdvisoryLimitParam = "pvcAdvisoryLimit"
	PVCSoftGracePrdParam  = "pvcSoftGracePrd"
	// KeyCSIPVCName represents key for csi pvc name
	KeyCSIPVCName = "csi.storage.k8s.io/pvc/name"
	// KeyReplicationEnabled represents key for replication enabled
	KeyReplicationEnabled = "isReplicationEnabled"

	// These are available when enabling --extra-create-metadata for the external-provisioner.
	csiPersistentVolumeName           = "csi.storage.k8s.io/pv/name"
	csiPersistentVolumeClaimName      = "csi.storage.k8s.io/pvc/name"
	csiPersistentVolumeClaimNamespace = "csi.storage.k8s.io/pvc/namespace"
	// These map to the above fields in the form of HTTP header names.
	headerPersistentVolumeName           = "x-csi-pv-name"
	headerPersistentVolumeClaimName      = "x-csi-pv-claimname"
	headerPersistentVolumeClaimNamespace = "x-csi-pv-namespace"
	// KeyReplicationVGPrefix represents key for replication vg prefix
	KeyReplicationVGPrefix = "volumeGroupPrefix"
	// KeyReplicationRemoteSystem represents key for replication remote system
	KeyReplicationRemoteSystem = "remoteSystem"
	// KeyReplicationRemoteAccessZone represents key for replication remote access zone
	KeyReplicationRemoteAccessZone = "remoteAccessZone"
	// KeyReplicationRemoteAccessZoneNetwork represents key for replication remote access zone network
	KeyReplicationRemoteAccessZoneNetwork = "remoteAzNetwork"
	// KeyReplicationRemoteAzServiceIP represents key for replication remote AzServiceIP
	KeyReplicationRemoteAzServiceIP = "remoteAzServiceIP"
	// KeyReplicationRemoteRootClientEnabled represents key for replication remote root client enabled
	KeyReplicationRemoteRootClientEnabled = "remoteRootClientEnabled"
	// KeyReplicationIgnoreNamespaces represents key for replication ignore namespaces
	KeyReplicationIgnoreNamespaces = "ignoreNamespaces"
	// KeyCSIPVCNamespace represents key for csi pvc namespace
	KeyCSIPVCNamespace = "csi.storage.k8s.io/pvc/namespace"
	// KeyReplicationRPO represents key for replication RPO
	KeyReplicationRPO         = "rpo"
	RpoFiveMinutes    RPOEnum = "Five_Minutes"
	RpoFifteenMinutes RPOEnum = "Fifteen_Minutes"
	RpoThirtyMinutes  RPOEnum = "Thirty_Minutes"
	RpoOneHour        RPOEnum = "One_Hour"
	RpoSixHours       RPOEnum = "Six_Hours"
	RpoTwelveHours    RPOEnum = "Twelve_Hours"
	RpoOneDay         RPOEnum = "One_Day"
)

// clusterToNodeIDMap is a map[clusterName][]*nodeIDToClientMap
var clusterToNodeIDMap = new(sync.Map)

var (
	listVolumesWorkerCount = 8

	getGetExportWithPathAndZoneFunc = func(isiConfig *IsilonClusterConfig) func(context.Context, string, string) (isi.Export, error) {
		return isiConfig.isiSvc.GetExportWithPathAndZone
	}

	getNodeLabelsWithNameFunc = func(s *service) func(string) (map[string]string, error) {
		return s.GetNodeLabelsWithName
	}
)

// type nodeIDElementsMap map[string]string
type nodeIDToClientMap map[string]string

// IsValid - checks valid RPO
func (rpo RPOEnum) IsValid() error {
	switch rpo {
	case RpoFiveMinutes, RpoFifteenMinutes, RpoThirtyMinutes, RpoOneHour, RpoSixHours, RpoTwelveHours, RpoOneDay:
		return nil
	}
	return errors.New("invalid rpo type")
}

// ToInt - converts to seconds
func (rpo RPOEnum) ToInt() (int, error) {
	switch rpo {
	case RpoFiveMinutes:
		return 300, nil
	case RpoFifteenMinutes:
		return 900, nil
	case RpoThirtyMinutes:
		return 1800, nil
	case RpoOneHour:
		return 3600, nil
	case RpoSixHours:
		return 21600, nil
	case RpoTwelveHours:
		return 43200, nil
	case RpoOneDay:
		return 86400, nil
	default:
		return -1, errors.New("invalid rpo type")
	}
}

// validateVolSize uses the CapacityRange range params to determine what size
// volume to create. Returned size is in bytes
func validateVolSize(cr *csi.CapacityRange) (int64, error) {
	minSize := cr.GetRequiredBytes()

	if minSize < 0 {
		return 0, status.Errorf(
			codes.OutOfRange,
			"bad capacity: volume size bytes '%d' must not be negative", minSize,
		)
	}

	if minSize == 0 {
		minSize = constants.DefaultVolumeSizeInBytes
	}

	return minSize, nil
}

func readQuotaLimitParams(params map[string]string, mutableParams map[string]string) (softlimit, advisorylimit, softgraceprd string) {
	// Setting Soft Limit
	softLimit := SoftLimitParamDefault
	if _, ok := params[SoftLimitParam]; ok {
		if params[SoftLimitParam] != "" {
			softLimit = params[SoftLimitParam]
		}
	}
	// If value is passed in pvc than it should get precedence
	if _, ok := params[PVCSoftLimitParam]; ok {
		if params[PVCSoftLimitParam] != "" {
			softLimit = params[PVCSoftLimitParam]
		}
	}
	// If softLimit value is passed in mutable params then it should get precedence over PVC and SC
	if val, ok := mutableParams[SoftLimitParam]; ok && val != "" {
		softLimit = val
	}

	// Setting Advisory Limit
	advisoryLimit := AdvisoryLimitParamDefault
	if _, ok := params[AdvisoryLimitParam]; ok {
		if params[AdvisoryLimitParam] != "" {
			advisoryLimit = params[AdvisoryLimitParam]
		}
	}
	// If value is passed in pvc than it should get precedence
	if _, ok := params[PVCAdvisoryLimitParam]; ok {
		if params[PVCAdvisoryLimitParam] != "" {
			advisoryLimit = params[PVCAdvisoryLimitParam]
		}
	}
	// If advisoryLimit value is passed in mutable params then it should get precedence over PVC and SC
	if val, ok := mutableParams[AdvisoryLimitParam]; ok && val != "" {
		advisoryLimit = val
	}

	// Setting Soft Grace Period
	softGracePrd := SoftGracePrdParamDefault
	if _, ok := params[SoftGracePrdParam]; ok {
		if params[SoftGracePrdParam] != "" {
			softGracePrd = params[SoftGracePrdParam]
		}
	}
	// If value is passed in pvc than it should get precedence
	if _, ok := params[PVCSoftGracePrdParam]; ok {
		if params[PVCSoftGracePrdParam] != "" {
			softGracePrd = params[PVCSoftGracePrdParam]
		}
	}
	// If softGracePrd value is passed in mutable params then it should get precedence over PVC and SC
	if val, ok := mutableParams[SoftGracePrdParam]; ok && val != "" {
		softGracePrd = val
	}

	return softLimit, advisoryLimit, softGracePrd
}

// resolveCreateVolumeMTLSSettings resolves mTLS configuration for volume creation.
//
// SmartConnectZoneFQDN uses three-layer precedence:
//  1. StorageClass parameter SmartConnectZoneFQDN (highest)
//  2. Cluster Secret field nfsMountFQDN
//  3. Environment variable X_CSI_ISI_NFS_MOUNT_FQDN (lowest)
//
// NFSTransportSecurity is StorageClass-only with no fallback. It must be explicitly
// set in each StorageClass to enable mTLS. This design allows mixed-mode deployments
// where mTLS and non-mTLS StorageClasses coexist independently.
func resolveCreateVolumeMTLSSettings(params map[string]string, isiConfig *IsilonClusterConfig) (string, string) {
	scFQDN := params[constants.SmartConnectZoneFQDNParam]

	var clusterFQDN string
	if isiConfig != nil {
		clusterFQDN = isiConfig.NFSMountFQDN
	}

	// FQDN resolution: StorageClass > cluster Secret > environment
	smartConnectZoneFQDN := ResolveMountFQDN(
		scFQDN,
		clusterFQDN,
		os.Getenv(constants.EnvNFSMountFQDN),
	)

	// Transport security: StorageClass-only, no inheritance from Secret or environment
	return smartConnectZoneFQDN, params[constants.NFSTransportSecurityParam]
}

func (s *service) CreateVolume(
	ctx context.Context,
	req *csi.CreateVolumeRequest) (
	resp *csi.CreateVolumeResponse, err error,
) {
	var (
		accessZone                        string
		isiPath                           string
		volumePathPermissions             string
		path                              string
		azServiceIP                       string
		azNetwork                         string
		rootClientEnabled                 string
		quotaID                           string
		exportID                          int
		foundVol                          bool
		export                            isi.Export
		contentSource                     *csi.VolumeContentSource
		sourceSnapshotID                  string
		sourceVolumeID                    string
		snapshotIsiPath                   string
		isROVolumeFromSnapshot            bool
		snapshotTrackingDir               string
		snapshotTrackingDirEntryForVolume string
		clusterName                       string
		softLimit                         string
		advisoryLimit                     string
		softGracePrd                      string
		isReplication                     bool
		VolumeGroupDir                    string
		snapshotSourceVolumeIsiPath       string
	)

	params := req.GetParameters()
	// Read the mutable parameters
	mutableParams := req.GetMutableParameters()

	if len(mutableParams) > 0 {
		if err := validateMutableParamKeys(mutableParams); err != nil {
			return nil, err
		}
	}

	if _, ok := params[AzNetwork]; ok {
		azNetwork = params[AzNetwork]
	}

	if _, ok := params[ClusterNameParam]; ok {
		if params[ClusterNameParam] == "" {
			clusterName = s.defaultIsiClusterName
		} else {
			clusterName = params[ClusterNameParam]
		}
	}

	startTime := time.Now()
	defer func() {
		log := csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			csmlog.FieldComponent: "controller",
			csmlog.FieldOperation: "CreateVolume",
		}).TrackDuration(startTime)
		if err != nil {
			log.Debugf("CreateVolume Failed with error: %v", err)
		} else {
			log.Info("CreateVolume Successful")
		}
	}()
	logFields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", logFields["csi.requestid"])

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		csmlog.FieldComponent: "controller",
		csmlog.FieldOperation: "CreateVolume",
		csmlog.FieldProtocol:  "NFS",
		"volume_name":         req.GetName(),
	}).Info("CreateVolume called")

	// set noProbeOnStart to false so subsequent calls can lead to probe
	noProbeOnStart.Store(false)

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error: %v ", err.Error())
		return nil, err
	}
	logFields[clusterName] = clusterName

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		csmlog.FieldOperation: "CreateVolume",
		csmlog.FieldArrayID:   isiConfig.Endpoint,
	}).Debugf("Cluster Name: %v", clusterName)

	// auto probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	// validate request
	sizeInBytes, err := s.ValidateCreateVolumeRequest(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if _, ok := params[AccessZoneParam]; ok {
		if params[AccessZoneParam] == "" {
			accessZone = s.opts.AccessZone
		} else {
			accessZone = params[AccessZoneParam]
		}
	} else {
		// use the default access zone if not set in the storage class
		accessZone = s.opts.AccessZone
	}
	if _, ok := params[IsiPathParam]; ok {
		if params[IsiPathParam] == "" {
			isiPath = isiConfig.IsiPath
		} else {
			isiPath = params[IsiPathParam]
		}
	} else {
		// use the default isiPath if not set in the storage class
		isiPath = isiConfig.IsiPath
	}

	if _, ok := params[IsiVolumePathPermissionsParam]; ok {
		if params[IsiVolumePathPermissionsParam] == "" {
			volumePathPermissions = isiConfig.IsiVolumePathPermissions
		} else {
			volumePathPermissions = params[IsiVolumePathPermissionsParam]
		}
	} else {
		// use the default volumePathPermissions if not set in the storage class
		volumePathPermissions = isiConfig.IsiVolumePathPermissions
	}

	if repl, ok := params[s.WithRP(KeyReplicationEnabled)]; ok {
		if boolRepl, err := strconv.ParseBool(repl); err == nil {
			isReplication = boolRepl
		} else {
			csmlog.WithContext(ctx).Info("Unable to parse replication flag from SC")
		}
	} else {
		csmlog.WithContext(ctx).Debug("Replication flag unset")
	}

	// When custom topology is enabled it takes precedence over the current default behavior
	// Set azServiceIP to updated endpoint when custom topology is enabled

	if s.opts.CustomTopologyEnabled {
		azServiceIP = isiConfig.Endpoint
	} else if _, ok := params[AzServiceIPParam]; ok {
		azServiceIP = params[AzServiceIPParam]
		if azServiceIP == "" {
			// use the endpoint if empty in the storage class
			azServiceIP = isiConfig.Endpoint
		}
	} else {
		// use the endpoint if not set in the storage class
		azServiceIP = isiConfig.Endpoint
	}
	if strings.Contains(azServiceIP, "localhost") {
		csmlog.WithContext(ctx).Debugf("Authorization is enabled, reading MountEndpoint: '%s'", isiConfig.MountEndpoint)
		azServiceIP = isiConfig.MountEndpoint
	}

	if val, ok := params[RootClientEnabledParam]; ok {
		_, err := strconv.ParseBool(val)
		// use the default if the boolean literal from the storage class is malformed
		if err != nil {
			csmlog.WithContext(ctx).WithFields(csmlog.Fields{RootClientEnabledParam: val}).Debugf(
				"invalid boolean value for '%s', defaulting to 'false'", RootClientEnabledParam,
			)

			rootClientEnabled = RootClientEnabledParamDefault
		}
		rootClientEnabled = val
	} else {
		// use the default if not set in the storage class
		rootClientEnabled = RootClientEnabledParamDefault
	}

	var directoryBacked bool
	var sharedExportPath string
	var sharedExport isi.Export // Cached shared export for directory-backed volumes (avoids redundant API call, isi.Export is *apiv2.Export)
	if val, ok := params[constants.DirectoryBackedParam]; ok && val == "true" {
		directoryBacked = true
		csmlog.WithContext(ctx).Info("Directory-backed provisioning mode enabled")

		// SharedExportPath is required when DirectoryBacked is true
		if sharedPath, ok := params[constants.SharedExportPathParam]; ok && sharedPath != "" {
			sharedExportPath = sharedPath
		} else {
			return nil, status.Errorf(codes.InvalidArgument,
				"SharedExportPath is required when DirectoryBacked is enabled")
		}

		// Validate SharedExportPath to prevent path traversal attacks (Req-SEC-I-1, Req-SEC-I-4)
		if !strings.HasPrefix(sharedExportPath, "/") {
			return nil, status.Errorf(codes.InvalidArgument,
				"parameter '%s' must be an absolute path (start with '/'), got '%s'",
				constants.SharedExportPathParam, sharedExportPath)
		}
		if strings.Contains(sharedExportPath, "..") {
			return nil, status.Errorf(codes.InvalidArgument,
				"parameter '%s' contains path traversal sequences ('..'), got '%s'",
				constants.SharedExportPathParam, sharedExportPath)
		}

		getExportWithPathAndZoneFunc := getGetExportWithPathAndZoneFunc(isiConfig)
		sharedExport, err = getExportWithPathAndZoneFunc(ctx, sharedExportPath, accessZone)
		if err != nil {
			return nil, status.Errorf(codes.Internal,
				"failed to query shared export at path '%s' in access zone '%s': %v",
				sharedExportPath, accessZone, err)
		}
		if sharedExport == nil {
			// Emit Kubernetes event for SharedExportNotFound (FR-2.1, NFR-2, NFR-4)
			pvcName := params[KeyCSIPVCName]
			pvcNamespace := params[KeyCSIPVCNamespace]
			s.emitSharedExportNotFoundEvent(ctx, pvcName, pvcNamespace, sharedExportPath, accessZone)

			return nil, status.Errorf(codes.InvalidArgument,
				"shared export not found at path '%s' in access zone '%s'. Ensure the shared export is pre-created by an administrator.",
				sharedExportPath, accessZone)
		}
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"sharedExportPath": sharedExportPath,
			"accessZone":       accessZone,
			"exportID":         sharedExport.ID,
		}).Info("Shared export validated successfully for directory-backed provisioning")
	} else {
		directoryBacked = false
		csmlog.WithContext(ctx).Debug("Using default export-backed provisioning mode")
	}

	// Reading quota limit parameters
	softLimit, advisoryLimit, softGracePrd = readQuotaLimitParams(params, mutableParams)
	csmlog.WithContext(ctx).Infof("Limit parameters considered for quota creation SoftLimit: '%s' , AdvisoryLimit: '%s',SoftGracePrd: '%s'", softLimit, advisoryLimit, softGracePrd)

	// Resolve mTLS parameters: FQDN uses StorageClass > Secret > environment precedence;
	// transport security is StorageClass-only (no inheritance).
	smartConnectZoneFQDN, nfsTransportSecurity := resolveCreateVolumeMTLSSettings(params, isiConfig)
	if err := ValidateNFSTransportSecurity(nfsTransportSecurity); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if smartConnectZoneFQDN != "" || nfsTransportSecurity != "" {
		csmlog.WithContext(ctx).Infof("mTLS parameters: SmartConnectZoneFQDN='%s', NFSTransportSecurity='%s'", smartConnectZoneFQDN, nfsTransportSecurity)
	}

	// Fail-fast: when mTLS is requested, SmartConnectZoneFQDN must be a valid FQDN (not an IP).
	// Catching this at CreateVolume time gives the user an immediate, actionable error
	// instead of a deferred mount failure when a Pod is scheduled.
	if IsMTLSEnabled(nfsTransportSecurity) {
		if smartConnectZoneFQDN == "" {
			return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID,
				"NFSTransportSecurity is set to 'mtls' but no FQDN is configured. "+
					"mTLS requires an FQDN for certificate validation. "+
					"Configure the FQDN using one of: StorageClass parameter 'SmartConnectZoneFQDN', "+
					"cluster Secret field 'nfsMountFQDN', or environment variable 'X_CSI_ISI_NFS_MOUNT_FQDN'."))
		}
		if IsIPAddress(smartConnectZoneFQDN) {
			return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID,
				"SmartConnectZoneFQDN '%s' is an IP address, but mTLS requires a valid FQDN for certificate validation. "+
					"Configure a valid FQDN (e.g., 'powerscale.example.com') using one of: StorageClass parameter 'SmartConnectZoneFQDN', "+
					"cluster Secret field 'nfsMountFQDN', or environment variable 'X_CSI_ISI_NFS_MOUNT_FQDN'.", smartConnectZoneFQDN))
		}
	}

	// Parse writable-from-snapshot parameter
	writableFromSnapshot := false
	if val, ok := params[WritableFromSnapshotParam]; ok && strings.EqualFold(val, "true") {
		writableFromSnapshot = true
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"volume_source_type": "writable-snapshot",
		}).Info("Writable snapshot provisioning enabled via StorageClass parameter")
	}

	// CSI specific metada for authorization
	headerMetadata := addMetaData(params)

	// check volume content source in the request
	isROVolumeFromSnapshot = false
	// check volume content source in the request
	if contentSource = req.GetVolumeContentSource(); contentSource != nil {
		// Fetch source snapshot ID  or volume ID from content source
		if snapshot := contentSource.GetSnapshot(); snapshot != nil {
			normalizedSnapshotID := snapshot.GetSnapshotId()
			// parse the input snapshot id and fetch it's components
			var snapshotSrcClusterName string
			sourceSnapshotID, snapshotSrcClusterName, _, err = id.ParseNormalizedSnapshotID(ctx, normalizedSnapshotID)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "failed to parse snapshot ID '%s', error : '%v'", normalizedSnapshotID, err))
			}

			if snapshotSrcClusterName != "" && snapshotSrcClusterName != clusterName {
				return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "source snapshot's cluster name '%s' and new volume's cluster name '%s' doesn't match", snapshotSrcClusterName, clusterName))
			}

			csmlog.WithContext(ctx).Infof("Creating volume from snapshot ID: '%s'", sourceSnapshotID)

			// Get snapshot path
			if snapshotSourceVolumeIsiPath, err = isiConfig.isiSvc.GetSnapshotSourceVolumeIsiPath(ctx, sourceSnapshotID); err != nil {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			csmlog.WithContext(ctx).Infof("Snapshot source volume isiPath is '%s' accessZone '%s'", snapshotSourceVolumeIsiPath, accessZone)

			if snapshotIsiPath, err = isiConfig.isiSvc.GetSnapshotIsiPath(ctx, snapshotSourceVolumeIsiPath, sourceSnapshotID, accessZone); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			csmlog.WithContext(ctx).Debugf("The Isilon directory path of snapshot is= '%s'", snapshotIsiPath)

			vcs := req.GetVolumeCapabilities()
			if len(vcs) == 0 {
				return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "volume capabilty is required"))
			}

			for _, vc := range vcs {
				if vc == nil {
					return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "volume capabilty is required"))
				}

				am := vc.GetAccessMode()
				if am == nil {
					return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "access mode is required"))
				}

				if am.Mode == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
					isROVolumeFromSnapshot = true
					break
				}
			}
		} else if volume := contentSource.GetVolume(); volume != nil {
			sourceVolumeID = volume.GetVolumeId()
			csmlog.WithContext(ctx).Infof("Creating volume from existing volume ID: '%s'", sourceVolumeID)

			// Validate writable-from-snapshot early: PVC clone sources are not supported
			if writableFromSnapshot {
				return nil, status.Error(codes.InvalidArgument, "writable-from-snapshot parameter is not supported with PVC clone dataSource")
			}
		}
	} else if writableFromSnapshot {
		// Writable snapshots require a snapshot content source
		return nil, status.Error(codes.InvalidArgument, "writable-from-snapshot parameter requires volume content source snapshot")
	}

	if isReplication {
		csmlog.WithContext(ctx).Info("Preparing volume replication")

		vgPrefix, ok := params[s.WithRP(KeyReplicationVGPrefix)]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "replication enabled but no volume group prefix specified in storage class")
		}

		rpo, ok := params[s.WithRP(KeyReplicationRPO)]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "replication enabled but no RPO specified in storage class")
		}

		rpoEnum := RPOEnum(rpo)
		if err := rpoEnum.IsValid(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid rpo value")
		}

		rpoint, err := rpoEnum.ToInt()
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "unable to parse rpo seconds")
		}

		remoteSystemName, ok := params[s.WithRP(KeyReplicationRemoteSystem)]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "replication enabled but no remote system specified in storage class")
		}

		remoteIsiConfig, err := s.getIsilonConfig(ctx, &remoteSystemName)
		if err != nil {
			csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error %v ", err.Error())
			return nil, status.Errorf(codes.InvalidArgument, "can't find cluster with name %s in driver config", remoteSystemName)
		}

		remoteSystemEndpoint := remoteIsiConfig.Endpoint
		if strings.Contains(remoteSystemEndpoint, "localhost") {
			csmlog.WithContext(ctx).Debugf("Authorization is enabled, reading MountEndpoint: '%s'", remoteIsiConfig.MountEndpoint)
			remoteSystemEndpoint = remoteIsiConfig.MountEndpoint
		}
		namespace := ""
		if ignoreNS, ok := params[s.WithRP(KeyReplicationIgnoreNamespaces)]; ok && ignoreNS == "false" {
			pvcNS, ok := params[KeyCSIPVCNamespace]
			if ok {
				namespace = pvcNS + "-"
			}
		}

		vgName := vgPrefix + "-" + namespace + remoteSystemEndpoint + "-" + rpo
		if len(vgName) > 128 {
			vgName = vgName[:128]
		}
		VolumeGroupDir = vgName
		var vg isi.Volume
		vg, err = isiConfig.isiSvc.client.GetVolumeWithIsiPath(ctx, isiPath, "", vgName)
		if err != nil {
			if apiErr, ok := err.(*isiApi.JSONError); ok && apiErr.StatusCode == 404 {
				vg, err = isiConfig.isiSvc.client.CreateVolumeWithIsipath(ctx, isiPath, vgName, "0777")
			}
			if err != nil {
				return nil, err
			}
		}

		ppName := strings.ReplaceAll(vg.Name, ".", "-")
		_, err = isiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
		if err != nil {
			if apiErr, ok := err.(*isiApi.JSONError); ok && apiErr.StatusCode == 404 {
				err := isiConfig.isiSvc.client.CreatePolicy(ctx, ppName, rpoint, isiPath+"/"+vgName, isiPath+"/"+vgName, remoteSystemEndpoint, remoteIsiConfig.ReplicationCertificateID, true)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "can't create protection policy %s", err.Error())
				}
				err = isiConfig.isiSvc.client.WaitForPolicyLastJobState(ctx, ppName, isi.FINISHED)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "policy job couldn't reach FINISHED state %s", err.Error())
				}
			} else {
				return nil, status.Errorf(codes.Internal, "can't ensure protection policy exists %s", err.Error())
			}
		}

		isiPath = isiPath + "/" + VolumeGroupDir
	}

	foundVol = false
	needsQuota := false // Track if we need to create quota for an existing directory
	if isROVolumeFromSnapshot {
		csmlog.WithContext(ctx).Debugf("Processing read only volume from existing snapshot")
		if isReplication {
			return nil, errors.New("unable to create replication volume from snapshot")
		}
		path = snapshotIsiPath
		snapshotSrc, err := isiConfig.isiSvc.GetSnapshot(ctx, sourceSnapshotID)
		if err != nil {
			return nil, fmt.Errorf("failed to get snapshot id '%s', error '%v'", sourceSnapshotID, err)
		}
		snapshotName := snapshotSrc.Name

		// Populate names for snapshot's tracking dir, snapshot tracking dir entry for this volume
		snapshotTrackingDir = isiConfig.isiSvc.GetSnapshotTrackingDirName(snapshotName)
		snapshotTrackingDirEntryForVolume = fPath.Join(snapshotTrackingDir, req.GetName())

		// Check if entry for this volume is present in snapshot tracking dir
		if isiConfig.isiSvc.IsVolumeExistent(ctx, snapshotSourceVolumeIsiPath, "", snapshotTrackingDirEntryForVolume) {
			csmlog.WithContext(ctx).Debugf("the path '%s' has already existed", path)
			foundVol = true
		} else {
			// Allow creation of only one active volume from a snapshot at any point in time
			totalSubDirectories, _ := isiConfig.isiSvc.GetSubDirectoryCount(ctx, snapshotSourceVolumeIsiPath, snapshotTrackingDir)
			if totalSubDirectories > 2 {
				return nil, fmt.Errorf("another RO volume from this snapshot is already present")
			}
		}
	} else {
		if directoryBacked {
			// Directory-backed: volume is a subdirectory under the shared export
			path = fPath.Join(sharedExportPath, req.GetName())
			csmlog.WithContext(ctx).WithFields(csmlog.Fields{
				"volumeName":       req.GetName(),
				"sharedExportPath": sharedExportPath,
				"directoryPath":    path,
			}).Debug("Directory-backed volume path calculated")

			// Check if directory already exists under shared export (idempotency)
			isVolumeExistentFunc := getIsVolumeExistentFunc(isiConfig)
			isVolumeExistent := isVolumeExistentFunc(ctx, sharedExportPath, "", req.GetName())
			if isVolumeExistent {
				csmlog.WithContext(ctx).Debugf("directory-backed volume '%s' already exists at path '%s'", req.GetName(), path)
				foundVol = true
			}
		} else {
			// Export-backed: original behavior - volume gets its own export
			path = isilonfs.GetPathForVolume(isiPath, req.GetName())
			// to ensure idempotency, check if the volume still exists.
			// k8s might have made the same CreateVolume call in quick succession and the volume was already created in the first run
			isVolumeExistentFunc := getIsVolumeExistentFunc(isiConfig)
			isVolumeExistent := isVolumeExistentFunc(ctx, isiPath, "", req.GetName())
			if isVolumeExistent {
				csmlog.WithContext(ctx).Debugf("the path '%s' has already existed", path)
				foundVol = true
			}
		}
	}

	if isROVolumeFromSnapshot && !foundVol {
		csmlog.WithContext(ctx).Debugf("Creating read only volume from existing snapshot")
		// Create an entry for this volume in snapshot tracking dir
		if err = isiConfig.isiSvc.CreateVolume(ctx, snapshotSourceVolumeIsiPath, snapshotTrackingDir, volumePathPermissions); err != nil {
			return nil, err
		}
		if err = isiConfig.isiSvc.CreateVolume(ctx, snapshotSourceVolumeIsiPath, snapshotTrackingDirEntryForVolume, volumePathPermissions); err != nil {
			return nil, err
		}
	}

	// For directory-backed volumes, reuse the sharedExport already fetched during validation.
	// For export-backed volumes, query the per-volume export.
	if directoryBacked {
		// Reuse cached sharedExport from validation block (line 461) — no redundant API call
		export = sharedExport
		err = nil
	} else {
		getExportWithPathAndZoneFunc := getGetExportWithPathAndZoneFunc(isiConfig)
		export, err = getExportWithPathAndZoneFunc(ctx, path, accessZone)
	}

	if err != nil || export == nil {
		var errMsg string
		var queryPath string
		if directoryBacked {
			queryPath = sharedExportPath
		} else {
			queryPath = path
		}
		if err == nil {
			if foundVol {
				return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "the export may not be ready yet and the path is %s", queryPath))
			}
		} else {
			// internal error
			return nil, err
		}
		csmlog.WithContext(ctx).Errorf("error retrieving export ID for '%s', set it to 0. error : '%s'.\n", req.GetName(), errMsg)
		csmlog.WithContext(ctx).Errorf("request parameters: the path is '%s', and the access zone is '%s'.", queryPath, accessZone)
		exportID = 0
	} else {
		exportID = export.ID
		if directoryBacked {
			csmlog.WithContext(ctx).Debugf("shared export ID '%d' resolved for directory-backed volume '%s'", exportID, req.GetName())
		} else {
			csmlog.WithContext(ctx).Debugf("id of the corresponding nfs export of existing volume '%s' has been resolved to '%d'", req.GetName(), exportID)
		}

		if exportID != 0 {
			if foundVol || isROVolumeFromSnapshot {
				if directoryBacked {
					// Check if quota exists for the directory (query by path, not export description)
					quota, quotaErr := isiConfig.isiSvc.GetQuotaByPath(ctx, path)
					if quotaErr == nil && quota != nil {
						if sizeInBytes != quota.Thresholds.Hard {
							return nil, status.Errorf(codes.AlreadyExists,
								"volume '%s' exists with quota, but at different size (requested: %d bytes, existing: %d bytes)",
								req.GetName(), sizeInBytes, quota.Thresholds.Hard)
						}
						csmlog.WithContext(ctx).Debugf("directory-backed volume '%s' already exists with matching quota, returning existing volume (idempotent)", req.GetName())
						return s.getCreateVolumeResponse(ctx, exportID, req.GetName(), path, export.Zone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork, true, sharedExportPath, smartConnectZoneFQDN, nfsTransportSecurity), nil
					}

					subDirCount, err := isiConfig.isiSvc.GetSubDirectoryCount(ctx, sharedExportPath, req.GetName())
					if err != nil {
						return nil, status.Errorf(codes.Internal, "failed to check directory contents for '%s': %v", req.GetName(), err)
					}

					if subDirCount > IgnoreDotAndDotDotSubDirs {
						return nil, status.Errorf(codes.FailedPrecondition,
							"pre-existing data detected at path '%s': directory exists with %d entries but no quota (possible external data)",
							path, subDirCount)
					}

					csmlog.WithContext(ctx).Debugf("directory-backed volume '%s' exists but has no quota, will create quota (idempotent case 3)", req.GetName())
					// Directory exists, but needs quota. Don't reset foundVol to avoid unnecessary CreateVolume call.
					needsQuota = true
				} else {
					// Export-backed: original behavior
					if s.opts.QuotaEnabled {
						quota, err := isiConfig.isiSvc.GetVolumeQuota(ctx, req.GetName(), exportID, accessZone)
						if err != nil {
							return nil, status.Errorf(codes.NotFound, "can't find quota for volume '%s': %s", req.GetName(), err.Error())
						}

						if sizeInBytes != quota.Thresholds.Hard {
							return nil, status.Errorf(codes.AlreadyExists, "volume '%s' exists, but at different size than requested", req.GetName())
						}
					}
					return s.getCreateVolumeResponse(ctx, exportID, req.GetName(), path, export.Zone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork, false, "", smartConnectZoneFQDN, nfsTransportSecurity), nil
				}
			}
			if !directoryBacked {
				// in case the export exists but no related volume (directory)
				if err = isiConfig.isiSvc.UnexportByIDWithZone(ctx, exportID, accessZone); err != nil {
					return nil, status.Error(codes.Internal, err.Error())
				}
				exportID = 0
			}
		}
	}

	// create volume (directory) with ACL 0777
	// Only create the volume directory for non-RO, non-writable-snapshot volumes
	if !isROVolumeFromSnapshot && !foundVol && !writableFromSnapshot {
		var basePath string
		var permissions string

		if directoryBacked {
			basePath = sharedExportPath
			// Check if IsiVolumePathPermissions was explicitly set in StorageClass
			_, explicitlySet := params[IsiVolumePathPermissionsParam]
			if !explicitlySet {
				// Parameter not set in StorageClass → use directory-backed default
				permissions = constants.DefaultDirectoryBackedVolumePermissions
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"provisioningMode":   "directory",
					"basePath":           basePath,
					"directory":          req.GetName(),
					"permissionsApplied": permissions,
					"permissionSource":   "default (param not in StorageClass)",
					"defaultValue":       constants.DefaultDirectoryBackedVolumePermissions,
					"clusterConfigValue": isiConfig.IsiVolumePathPermissions,
				}).Info("Creating directory-backed volume with default permissions")
			} else {
				// Parameter explicitly set in StorageClass → use that value
				permissions = volumePathPermissions
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"provisioningMode":   "directory",
					"basePath":           basePath,
					"directory":          req.GetName(),
					"permissionsApplied": permissions,
					"permissionSource":   "StorageClass explicit override",
					"storageClassParam":  volumePathPermissions,
					"defaultValue":       constants.DefaultDirectoryBackedVolumePermissions,
				}).Info("Creating directory-backed volume with overridden permissions")
			}
		} else {
			basePath = isiPath
			// Check if IsiVolumePathPermissions was explicitly set in StorageClass
			_, explicitlySet := params[IsiVolumePathPermissionsParam]
			if !explicitlySet {
				// Parameter not set in StorageClass → use export-backed default
				permissions = constants.DefaultIsiVolumePathPermissions
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"provisioningMode":   "export",
					"basePath":           basePath,
					"volume":             req.GetName(),
					"permissionsApplied": permissions,
					"permissionSource":   "default (param not in StorageClass)",
					"defaultValue":       constants.DefaultIsiVolumePathPermissions,
				}).Info("Creating export-backed volume with default permissions")
			} else {
				// Parameter explicitly set in StorageClass → use that value
				permissions = volumePathPermissions
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"provisioningMode":   "export",
					"basePath":           basePath,
					"volume":             req.GetName(),
					"permissionsApplied": permissions,
					"permissionSource":   "StorageClass explicit override",
					"storageClassParam":  volumePathPermissions,
					"defaultValue":       constants.DefaultIsiVolumePathPermissions,
				}).Info("Creating export-backed volume with overridden permissions")
			}
		}

		csmlog.WithContext(ctx).Debugf("Creating new volume (not from snapshot) '%s'", req.GetName())

		if len(headerMetadata) == 0 {
			if err = isiConfig.isiSvc.CreateVolume(ctx, basePath, req.GetName(), permissions); err != nil {
				return nil, fmt.Errorf("failed to create volume directory: %w", err)
			}
			csmlog.WithContext(ctx).Debugf("created volume without header metadata '%s'", req.GetName())
		} else {
			if err = isiConfig.isiSvc.CreateVolumeWithMetaData(ctx, basePath, req.GetName(), permissions, headerMetadata); err != nil {
				return nil, fmt.Errorf("failed to create volume directory with metadata: %w", err)
			}
			csmlog.WithContext(ctx).Debugf("created volume with header metadata '%s' has been resolved to '%v'", req.GetName(), headerMetadata)
		}
	}

	// if volume content source is not null and new volume request is not for RO volume from snapshot,
	// copy content from the datasource
	if contentSource != nil && !isROVolumeFromSnapshot {
		csmlog.WithContext(ctx).Debugf("Creating volume from content source '%s'", req.GetName())
		// For directory-backed volumes, use sharedExportPath as the base path for snapshot restore
		// This ensures the snapshot data is copied to the correct destination
		volumeBasePath := isiPath
		if directoryBacked {
			volumeBasePath = sharedExportPath
		}
		err = s.createVolumeFromSource(ctx, isiConfig, volumeBasePath, contentSource, req, sizeInBytes, accessZone, writableFromSnapshot)
		if err != nil {
			// Clear volume since the volume creation is not successful
			// For writable snapshots, clean up the writable snapshot metadata first before deleting the volume directory.
			// If writable snapshot cleanup fails, we must not proceed with volume deletion to avoid orphaning
			// the writable snapshot metadata in OneFS (per Issue 1 from code review).
			if writableFromSnapshot {
				dstPath := isilonfs.GetPathForVolume(isiPath, req.GetName())
				if cleanupErr := deleteWritableSnapshotFunc(isiConfig)(ctx, dstPath); cleanupErr != nil {
					csmlog.WithContext(ctx).Errorf("cleanup: failed to delete writable snapshot at '%s': %v", dstPath, cleanupErr)
					// Return cleanup error to prevent orphaning writable snapshot metadata
					return nil, status.Errorf(codes.Internal, "failed to clean up writable snapshot during rollback at '%s': %v (original error: %v)", dstPath, cleanupErr, err)
				}
			}
			if err := isiConfig.isiSvc.DeleteVolume(ctx, isiPath, req.GetName()); err != nil {
				csmlog.WithContext(ctx).Infof("Delete volume in CreateVolume returned error '%s'", err)
			}
			return nil, err
		}
	}

	volumeName := req.GetName()
	if needsQuota || (!foundVol && !isROVolumeFromSnapshot && !writableFromSnapshot) {
		quotaEnabled := s.opts.QuotaEnabled || directoryBacked

		const maxQuotaRetries = 3
		var quotaErr error
		for attempt := 1; attempt <= maxQuotaRetries; attempt++ {
			quotaID, quotaErr = isiConfig.isiSvc.CreateQuota(ctx, path, volumeName, softLimit, advisoryLimit, softGracePrd, sizeInBytes, quotaEnabled)
			if quotaErr == nil {
				csmlog.WithContext(ctx).Debugf("quota created successfully for '%s' on attempt %d", volumeName, attempt)
				break
			}

			if attempt < maxQuotaRetries {
				backoffDuration := time.Duration(attempt) * RetrySleepTime
				csmlog.WithContext(ctx).Warnf("quota creation failed for '%s' (attempt %d/%d): %v, retrying in %v", volumeName, attempt, maxQuotaRetries, quotaErr, backoffDuration)
				// Context-aware sleep to respect cancellation during retries
				select {
				case <-ctx.Done():
					csmlog.WithContext(ctx).Warnf("quota creation cancelled for '%s' during retry backoff", volumeName)
					return nil, status.Errorf(codes.Canceled, "quota creation cancelled: %v", ctx.Err())
				case <-time.After(backoffDuration):
					// Continue to next retry attempt
				}
			}
		}

		if quotaErr != nil {
			csmlog.WithContext(ctx).Errorf("error creating quota ('%s', '%d' bytes) after %d attempts, abort and roll back: '%v'", req.GetName(), sizeInBytes, maxQuotaRetries, quotaErr)
			// roll back, delete the newly created volume
			var rollbackPath string
			if directoryBacked {
				rollbackPath = sharedExportPath
			} else {
				rollbackPath = isiPath
			}
			if deleteErr := isiConfig.isiSvc.DeleteVolume(ctx, rollbackPath, volumeName); deleteErr != nil {
				csmlog.WithContext(ctx).Errorf("rollback deletion of volume '%s' also failed: %v", req.GetName(), deleteErr)
				return nil, fmt.Errorf("quota creation failed after %d retries: %w, and rollback deletion also failed: %v", maxQuotaRetries, quotaErr, deleteErr)
			}
			csmlog.WithContext(ctx).Infof("successfully rolled back by deleting volume '%s'", req.GetName())
			return nil, fmt.Errorf("quota creation failed after %d retries: %w, volume deleted successfully", maxQuotaRetries, quotaErr)
		}
	} else if !foundVol && !isROVolumeFromSnapshot && writableFromSnapshot {
		// For writable snapshots, OneFS automatically creates the quota regardless of QuotaEnabled setting.
		// Query the auto-created quota ID so we can store it in the export description for cleanup during DeleteVolume.
		// OneFS creates quotas for writable snapshots automatically, so we must track the quota ID
		// even when QuotaEnabled=false to ensure proper cleanup.
		csmlog.WithContext(ctx).Infof("Querying auto-created quota for writable snapshot volume '%s' at path '%s'", volumeName, path)
		quota, err := isiConfig.isiSvc.client.GetQuotaWithPath(ctx, path)
		if err != nil {
			csmlog.WithContext(ctx).Errorf("Failed to retrieve auto-created quota for writable snapshot '%s': %v", volumeName, err)
			// Rollback: clean up the writable snapshot metadata first before deleting the volume directory.
			// If writable snapshot cleanup fails, we must not proceed with volume deletion to avoid orphaning
			// the writable snapshot metadata in OneFS (per Issue 1 from code review).
			dstPath := isilonfs.GetPathForVolume(isiPath, volumeName)
			if cleanupErr := deleteWritableSnapshotFunc(isiConfig)(ctx, dstPath); cleanupErr != nil {
				csmlog.WithContext(ctx).Errorf("cleanup: failed to delete writable snapshot at '%s': %v", dstPath, cleanupErr)
				// Return cleanup error to prevent orphaning writable snapshot metadata
				return nil, status.Errorf(codes.Internal, "failed to clean up writable snapshot during rollback at '%s': %v (original error: failed to retrieve auto-created quota: %v)", dstPath, cleanupErr, err)
			}
			if cleanupErr := isiConfig.isiSvc.DeleteVolume(ctx, isiPath, volumeName); cleanupErr != nil {
				csmlog.WithContext(ctx).Errorf("cleanup: failed to delete volume '%s': %v", volumeName, cleanupErr)
			}
			return nil, status.Errorf(codes.Internal, "failed to retrieve auto-created quota for writable snapshot '%s': %v", volumeName, err)
		}
		quotaID = quota.ID
		csmlog.WithContext(ctx).Infof("Retrieved auto-created quota ID '%s' for writable snapshot volume '%s'", quotaID, volumeName)
		if err = isiConfig.isiSvc.UpdateQuotaSize(ctx, quota.ID, sizeInBytes, 0, 0, 0); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// export volume in the given access zone, also add normalized quota id to the description field, in DeleteVolume,
	// the quota ID will be used for the quota to be directly deleted by ID
	if directoryBacked {
		// Directory-backed: shared export already exists, return response directly
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"volumeName": volumeName,
			"exportID":   exportID,
			"path":       path,
			"accessZone": accessZone,
		}).Info("Directory-backed volume created successfully, using shared export")
		return s.getCreateVolumeResponse(ctx, exportID, volumeName, path, accessZone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork, true, sharedExportPath, smartConnectZoneFQDN, nfsTransportSecurity), nil
	} else if isROVolumeFromSnapshot {
		// Resolve xprtsec from NFSTransportSecurity parameter
		xprtsec := ResolveXprtsec(nfsTransportSecurity)

		if exportID, err = isiConfig.isiSvc.ExportVolumeWithZoneAndXprtsec(ctx, path, "", accessZone, "", xprtsec); err == nil && exportID != 0 {
			// get the export and retry if not found to ensure the export has been created
			for i := 0; i < MaxRetries; i++ {
				if export, _ := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone); export != nil {
					// Add dummy localhost entry for pvc security
					if !isiConfig.isiSvc.IsHostAlreadyAdded(ctx, exportID, accessZone, id.DummyHostNodeID) {
						err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, id.DummyHostNodeID, *isiConfig.IgnoreUnresolvableHosts, isiConfig.isiSvc.AddExportClientByIDWithZone)
						if err != nil {
							csmlog.WithContext(ctx).Debugf("Error while adding dummy localhost entry to export '%d'", exportID)
						}
					}
					// For RO volumes from snapshots, preserve the original volume name (req.GetName())
					// to ensure it matches the snapshot tracking directory entry created earlier.
					// Using the source volume name from the export path would cause a mismatch
					// that prevents cleanup during DeleteVolume, leaving stale tracking entries
					// that block DeleteSnapshot from actually deleting the snapshot on the array.
					exportPath := path
					if export.Paths != nil {
						if len(*export.Paths) > 0 {
							exportPath = (*export.Paths)[0]
						}
					}
					csmlog.WithContext(ctx).Debugf("volume name '%s' and export path: %s", volumeName, exportPath)
					// return the response
					return s.getCreateVolumeResponse(ctx, exportID, volumeName, exportPath, accessZone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork, false, "", smartConnectZoneFQDN, nfsTransportSecurity), nil
				}
				select {
				case <-ctx.Done():
					return nil, status.Errorf(codes.Canceled, "context canceled during export creation retry")
				case <-time.After(RetrySleepTime):
				}
				csmlog.WithContext(ctx).Infof("Begin to retry '%d' time(s), for export id '%d' and path '%s'\n", i+1, exportID, path)
			}
		} else {
			return nil, err
		}
	} else {
		// Resolve xprtsec from NFSTransportSecurity parameter
		xprtsec := ResolveXprtsec(nfsTransportSecurity)

		if exportID, err = isiConfig.isiSvc.ExportVolumeWithZoneAndXprtsec(ctx, isiPath, volumeName, accessZone, isilonfs.GetQuotaIDWithCSITag(quotaID), xprtsec); err == nil && exportID != 0 {
			// get the export and retry if not found to ensure the export has been created
			for i := 0; i < MaxRetries; i++ {
				if export, _ := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone); export != nil {
					// Add dummy localhost entry for pvc security
					if !isiConfig.isiSvc.IsHostAlreadyAdded(ctx, exportID, accessZone, id.DummyHostNodeID) {
						err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, id.DummyHostNodeID, *isiConfig.IgnoreUnresolvableHosts, isiConfig.isiSvc.AddExportClientByIDWithZone)
						if err != nil {
							csmlog.WithContext(ctx).Debugf("Error while adding dummy localhost entry to export '%d'", exportID)
						}
					}
					// return the createVolume response with actual array volume name
					exportPath := path
					if export.Paths != nil {
						if len(*export.Paths) > 0 {
							exportPath = (*export.Paths)[0]
							pathToken := strings.Split(exportPath, "/")
							volumeName = pathToken[len(pathToken)-1]
							csmlog.WithContext(ctx).Debugf("volume name at array '%s' and export path: %s", volumeName, exportPath)
						}
					}

					return s.getCreateVolumeResponse(ctx, exportID, volumeName, exportPath, accessZone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork, false, "", smartConnectZoneFQDN, nfsTransportSecurity), nil
				}
				select {
				case <-ctx.Done():
					return nil, status.Errorf(codes.Canceled, "context canceled during export creation retry")
				case <-time.After(RetrySleepTime):
				}
				csmlog.WithContext(ctx).Infof("Begin to retry '%d' time(s), for export id '%d' and path '%s'\n", i+1, exportID, path)
			}
		} else {
			// clear quota and delete volume since the export cannot be created
			if err := isiConfig.isiSvc.ClearQuotaByID(ctx, quotaID); err != nil {
				csmlog.WithContext(ctx).Infof("Clear Quota returned error '%s'", err)
			}
			// For writable snapshots, clean up the writable snapshot metadata first before deleting the volume directory.
			// If writable snapshot cleanup fails, we must not proceed with volume deletion to avoid orphaning
			// the writable snapshot metadata in OneFS (per Issue 1 from code review).
			if writableFromSnapshot {
				dstPath := isilonfs.GetPathForVolume(isiPath, req.GetName())
				if cleanupErr := deleteWritableSnapshotFunc(isiConfig)(ctx, dstPath); cleanupErr != nil {
					csmlog.WithContext(ctx).Errorf("cleanup: failed to delete writable snapshot at '%s': %v", dstPath, cleanupErr)
					// Return cleanup error to prevent orphaning writable snapshot metadata
					return nil, status.Errorf(codes.Internal, "failed to clean up writable snapshot during rollback at '%s': %v (original error: export creation failed)", dstPath, cleanupErr)
				}
			}
			if err := isiConfig.isiSvc.DeleteVolume(ctx, isiPath, req.GetName()); err != nil {
				csmlog.WithContext(ctx).Infof("Delete volume in CreateVolume returned error '%s'", err)
			}
			return nil, err
		}
	}
	return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "the export id %d and path %s may not be ready yet after retrying", exportID, path))
}

// Define function types for external function calls
var getSnapshotFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, snapshotID string) (isi.Snapshot, error) {
	return isiConfig.isiSvc.GetSnapshot
}

var getSnapshotSizeFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, volumePath, snapshotName, accessZone string) int64 {
	return isiConfig.isiSvc.GetSnapshotSize
}

var copySnapshotFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, dstPath string, srcPath string, snapshotID int64, dstName string, accessZone string) (isi.Volume, error) {
	return isiConfig.isiSvc.CopySnapshot
}

var getSnapshotIQLicenseStatusFunc = func(ctx context.Context, isiConfig *IsilonClusterConfig) (string, error) {
	if isiConfig == nil || isiConfig.isiSvc == nil {
		return "", fmt.Errorf("isilon service is not initialized")
	}
	license, err := isiConfig.isiSvc.GetLicenseByID(ctx, SnapshotIQLicenseID)
	if err != nil {
		return "", err
	}
	if license == nil {
		return "", fmt.Errorf("no license found with ID %s", SnapshotIQLicenseID)
	}
	return license.Status, nil
}

func validateSnapshotIQLicense(ctx context.Context, isiConfig *IsilonClusterConfig) error {
	licenseStatus, err := getSnapshotIQLicenseStatusFunc(ctx, isiConfig)
	if err != nil {
		// Some test mocks and older environments do not implement platform/17 license API.
		// In those cases, keep existing behavior and let snapshot calls fail on the backend.
		if strings.Contains(strings.ToLower(err.Error()), "cannot unmarshal number into go value of type api.jsonerror") {
			csmlog.WithContext(ctx).Warnf("SnapshotIQ license pre-check skipped: %s", err.Error())
			return nil
		}
		return status.Errorf(codes.FailedPrecondition, "SnapshotIQ license is not activated or available: %s", err.Error())
	}
	csmlog.WithContext(ctx).Infof("SnapshotIQ license found with status '%s'", licenseStatus)
	if !strings.EqualFold(licenseStatus, "Licensed") && !strings.EqualFold(licenseStatus, "Evaluation") {
		return status.Errorf(codes.FailedPrecondition, "SnapshotIQ license status is '%s'", licenseStatus)
	}
	return nil
}

func (s *service) createVolumeFromSnapshot(ctx context.Context, isiConfig *IsilonClusterConfig,
	isiPath, normalizedSnapshotID, dstVolumeName string, sizeInBytes int64, accessZone string,
) error {
	getSnapshot := getSnapshotFunc(isiConfig)
	getSnapshotSize := getSnapshotSizeFunc(isiConfig)
	copySnapshot := copySnapshotFunc(isiConfig)

	var snapshotSrc isi.Snapshot
	var err error

	// parse the input snapshot id and fetch it's components
	srcSnapshotID, _, _, err := id.ParseNormalizedSnapshotID(ctx, normalizedSnapshotID)
	if err != nil {
		return err
	}

	if snapshotSrc, err = getSnapshot(ctx, srcSnapshotID); err != nil {
		return fmt.Errorf("failed to get snapshot id '%s', error '%v'", srcSnapshotID, err)
	}

	// check source snapshot size
	snapshotSourceVolumeIsiPath := fPath.Dir(snapshotSrc.Path)
	size := getSnapshotSize(ctx, snapshotSourceVolumeIsiPath, snapshotSrc.Name, accessZone)
	if size > sizeInBytes {
		return fmt.Errorf("specified size '%d' is smaller than source snapshot size '%d'", sizeInBytes, size)
	}
	if _, err = copySnapshot(ctx, isiPath, snapshotSourceVolumeIsiPath, snapshotSrc.ID, dstVolumeName, accessZone); err != nil {
		return status.Errorf(codes.Internal, "failed to copy snapshot id '%s', error '%s'", srcSnapshotID, err.Error())
	}

	return nil
}

// createWritableSnapshotFunc is a function-variable indirection for testability
var createWritableSnapshotFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, isiPath string, destinationPath string, sourceSnapshot string, volumeName string, accessZone string) (isi.Volume, error) {
	return isiConfig.isiSvc.CreateWritableSnapshot
}

// deleteWritableSnapshotFunc is a function-variable indirection for testability
var deleteWritableSnapshotFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, dstPath string) error {
	return isiConfig.isiSvc.DeleteWritableSnapshot
}

// getWritableSnapshotFunc is a function-variable indirection for testability
var getWritableSnapshotFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, dstPath string) (isi.WritableSnapshot, error) {
	return isiConfig.isiSvc.GetWritableSnapshot
}

// createVolumeFromWritableSnapshot creates a writable volume from a snapshot using the OneFS writable snapshot API.
func (s *service) createVolumeFromWritableSnapshot(ctx context.Context, isiConfig *IsilonClusterConfig,
	isiPath, normalizedSnapshotID, dstVolumeName string, sizeInBytes int64, accessZone string,
) error {
	getSnapshot := getSnapshotFunc(isiConfig)
	getSnapshotSize := getSnapshotSizeFunc(isiConfig)
	createWritableSnapshot := createWritableSnapshotFunc(isiConfig)
	getWritableSnapshot := getWritableSnapshotFunc(isiConfig)

	var snapshotSrc isi.Snapshot
	var err error

	// parse the input snapshot id and fetch it's components
	srcSnapshotID, _, _, err := id.ParseNormalizedSnapshotID(ctx, normalizedSnapshotID)
	if err != nil {
		return err
	}

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		"volume_source_type": "writable-snapshot",
		"source_snapshot_id": normalizedSnapshotID,
	}).Info("Provisioning writable snapshot volume")

	if snapshotSrc, err = getSnapshot(ctx, srcSnapshotID); err != nil {
		return status.Errorf(codes.NotFound, "failed to get snapshot id '%s', error '%v'", srcSnapshotID, err)
	}

	// check source snapshot size
	snapshotSourceVolumeIsiPath := path.Dir(snapshotSrc.Path)
	size := getSnapshotSize(ctx, snapshotSourceVolumeIsiPath, snapshotSrc.Name, accessZone)
	if size > sizeInBytes {
		return status.Errorf(codes.InvalidArgument, "specified size '%d' is smaller than source snapshot size '%d'", sizeInBytes, size)
	}

	// Build destination path for the writable snapshot
	destinationPath := isilonfs.GetPathForVolume(isiPath, dstVolumeName)

	// Idempotency check: verify if writable snapshot already exists at the destination path
	if existingWS, err := getWritableSnapshot(ctx, destinationPath); err == nil && existingWS != nil {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"volume_source_type": "writable-snapshot",
			"source_snapshot_id": normalizedSnapshotID,
			"volume_name":        dstVolumeName,
			"destination_path":   destinationPath,
		}).Info("Writable snapshot already exists at destination path, skipping creation (idempotent)")
		return nil
	}

	if _, err = createWritableSnapshot(ctx, isiPath, destinationPath, srcSnapshotID, dstVolumeName, accessZone); err != nil {
		return status.Errorf(codes.Internal, "failed to create writable snapshot from snapshot '%s', error '%v'", srcSnapshotID, err)
	}

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		"volume_source_type":   "writable-snapshot",
		"source_snapshot_id":   normalizedSnapshotID,
		"source_snapshot_name": snapshotSrc.Name,
		"volume_name":          dstVolumeName,
	}).Info("Writable snapshot volume provisioning successful")

	return nil
}

// createVolumeFromWritableSnapshotFunc is a function-variable indirection for testability
var createVolumeFromWritableSnapshotFunc = func(svc *service) func(ctx context.Context, isiConfig *IsilonClusterConfig, isiPath, snapshotID, volName string, sizeInBytes int64, accessZone string) error {
	return svc.createVolumeFromWritableSnapshot
}

var (
	getVolumeSizeFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, isiPath, srcVolumeName string) int64 {
		return isiConfig.isiSvc.GetVolumeSize
	}

	copyVolumeFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, isiPath, srcVolumeName, dstVolumeName string) (isi.Volume, error) {
		return isiConfig.isiSvc.CopyVolume
	}
)

func (s *service) createVolumeFromVolume(ctx context.Context, isiConfig *IsilonClusterConfig, isiPath, srcVolumeName, dstVolumeName string, sizeInBytes int64) error {
	isVolumeExistent := isVolumeExistentFunc(isiConfig)
	getVolumeSize := getVolumeSizeFunc(isiConfig)
	copyVolume := copyVolumeFunc(isiConfig)
	var err error
	if isVolumeExistent(ctx, isiPath, "", srcVolumeName) {
		// check source volume size
		size := getVolumeSize(ctx, isiPath, srcVolumeName)
		if size > sizeInBytes {
			return fmt.Errorf("specified size '%d' is smaller than source volume size '%d'", sizeInBytes, size)
		}

		if _, err = copyVolume(ctx, isiPath, srcVolumeName, dstVolumeName); err != nil {
			return fmt.Errorf("failed to copy volume name '%s', error '%v'", srcVolumeName, err)
		}
	} else {
		return fmt.Errorf("failed to get volume name '%s', error '%v'", srcVolumeName, err)
	}

	return nil
}

var (
	// Variables for the functions within the service struct
	getSnapshotSourceFunc = func(contentSource *csi.VolumeContentSource) *csi.VolumeContentSource_SnapshotSource {
		return contentSource.GetSnapshot()
	}

	getVolumeFunc = func(contentSource *csi.VolumeContentSource) *csi.VolumeContentSource_VolumeSource {
		return contentSource.GetVolume()
	}

	createVolumeFromSnapshotFunc = func(svc *service) func(ctx context.Context, isiConfig *IsilonClusterConfig, isiPath, snapshotID, volName string, sizeInBytes int64, accessZone string) error {
		return svc.createVolumeFromSnapshot
	}

	createVolumeFromVolumeFunc = func(svc *service) func(ctx context.Context, isiConfig *IsilonClusterConfig, isiPath, srcVolumeName, dstVolumeName string, sizeInBytes int64) error {
		return svc.createVolumeFromVolume
	}

	getUtilsParseNormalizedVolumeID = id.ParseNormalizedVolumeID
)

func (s *service) createVolumeFromSource(
	ctx context.Context,
	isiConfig *IsilonClusterConfig,
	isiPath string,
	contentSource *csi.VolumeContentSource,
	req *csi.CreateVolumeRequest,
	sizeInBytes int64, accessZone string,
	writableFromSnapshot bool,
) error {
	if contentSnapshot := getSnapshotSourceFunc(contentSource); contentSnapshot != nil {
		if writableFromSnapshot {
			// Writable snapshot path (PR‑203)
			if err := createVolumeFromWritableSnapshotFunc(s)(
				ctx, isiConfig, isiPath, contentSnapshot.GetSnapshotId(),
				req.GetName(), sizeInBytes, accessZone,
			); err != nil {
				// preserve gRPC status codes from createVolumeFromWritableSnapshot
				if _, ok := status.FromError(err); ok {
					return err
				}
				return status.Error(codes.Internal, err.Error())
			}
		} else {
			// Non‑writable snapshot path (your directory‑backed-aware behavior)

			// isiPath now contains the correct base path:
			// - For export-backed: /ifs/data (from driver config)
			// - For directory-backed: /ifs/k8s/shared (from destination StorageClass sharedExportPath)
			csmlog.WithContext(ctx).WithFields(csmlog.Fields{
				"snapshotID":      contentSnapshot.GetSnapshotId(),
				"destinationPath": isiPath,
				"volumeName":      req.GetName(),
			}).Debug("createVolumeFromSource: restoring volume from snapshot")

			if err := createVolumeFromSnapshotFunc(s)(
				ctx, isiConfig, isiPath, contentSnapshot.GetSnapshotId(),
				req.GetName(), sizeInBytes, accessZone,
			); err != nil {
				// Preserve gRPC status codes for consistency with writable snapshot path
				if _, ok := status.FromError(err); ok {
					return err
				}
				return status.Error(codes.Internal, err.Error())
			}
		}
	}

	if contentVolume := getVolumeFunc(contentSource); contentVolume != nil {

		// create volume from source volume
		srcVolumeID := contentVolume.GetVolumeId()
		srcVolumeName, srcExportID, srcAccessZone, _, err := getUtilsParseNormalizedVolumeID(ctx, srcVolumeID)
		if err != nil {
			if strings.Contains(err.Error(), "cannot be split into tokens") {
				return status.Error(codes.NotFound, "volume ID is invalid or not found")
			}
			return status.Error(codes.Internal, err.Error())
		}

		// Check if source volume is directory-backed by parsing volume ID mode
		srcBasePath := isiPath
		srcVolumeNameOrPath := srcVolumeName
		_, _, _, _, srcProvisioningMode, parseErr := id.ParseVolumeIDWithMode(ctx, srcVolumeID)
		if parseErr == nil && srcProvisioningMode == id.ProvisioningModeDirectory {
			// Source is directory-backed, get shared export path from export ID
			if srcExport, err := isiConfig.isiSvc.GetExportByIDWithZone(ctx, srcExportID, srcAccessZone); err == nil && srcExport != nil && srcExport.Paths != nil && len(*srcExport.Paths) > 0 {
				// For directory-backed volumes, use the shared export path as the base
				// and the volume name as the relative path within it
				srcSharedExportPath := path.Clean((*srcExport.Paths)[0])
				srcBasePath = srcSharedExportPath
				srcVolumeNameOrPath = srcVolumeName
				csmlog.WithContext(ctx).Debugf("Source volume %s is directory-backed, using basePath=%s and volumeName=%s", srcVolumeName, srcBasePath, srcVolumeNameOrPath)
			} else {
				csmlog.WithContext(ctx).Warnf("Failed to get export %d for directory-backed source volume %s, falling back to default path", srcExportID, srcVolumeName)
			}
		}

		if err := createVolumeFromVolumeFunc(s)(ctx, isiConfig, srcBasePath, srcVolumeNameOrPath, req.GetName(), sizeInBytes); err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	}
	return nil
}

// Define a variable for the getCSIVolume function
var getCSIVolumeFunc = func(svc *service) func(ctx context.Context, exportID int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork string, directoryBacked bool, sharedExportPath, smartConnectZoneFQDN, nfsTransportSecurity string) *csi.Volume {
	return svc.getCSIVolume
}

func (s *service) getCreateVolumeResponse(ctx context.Context, exportID int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork string, directoryBacked bool, sharedExportPath, smartConnectZoneFQDN, nfsTransportSecurity string) *csi.CreateVolumeResponse {
	return &csi.CreateVolumeResponse{
		Volume: getCSIVolumeFunc(s)(ctx, exportID, volName, path, accessZone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork, directoryBacked, sharedExportPath, smartConnectZoneFQDN, nfsTransportSecurity),
	}
}

func (s *service) getCSIVolume(ctx context.Context, exportID int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork string, directoryBacked bool, sharedExportPath, smartConnectZoneFQDN, nfsTransportSecurity string) *csi.Volume {
	attributes := map[string]string{
		"ID":                strconv.Itoa(exportID),
		"Name":              volName,
		"Path":              path,
		"AccessZone":        accessZone,
		"AzServiceIP":       azServiceIP,
		"AzNetwork":         azNetwork,
		"RootClientEnabled": rootClientEnabled,
		"ClusterName":       clusterName,
	}

	if directoryBacked {
		attributes["ProvisioningMode"] = "directory"
		attributes["DirectoryPath"] = volName
		attributes["SharedExportPath"] = sharedExportPath
		attributes["SharedExportID"] = strconv.Itoa(exportID)
		csmlog.WithContext(ctx).Debugf("Added directory-backed volume attributes: mode=directory, directoryPath=%s, sharedExportPath=%s", volName, sharedExportPath)
	} else {
		attributes["ProvisioningMode"] = "export"
	}

	// Add mTLS parameters to volume context if configured
	if smartConnectZoneFQDN != "" {
		attributes[constants.SmartConnectZoneFQDNParam] = smartConnectZoneFQDN
	}
	if nfsTransportSecurity != "" {
		attributes[constants.NFSTransportSecurityParam] = nfsTransportSecurity
	}

	csmlog.WithContext(ctx).Debugf("Attributes '%v'", attributes)

	// Set content source as part of create volume response if volume is created from snapshot or existing volume
	// ContentSource is an optional field as part of CSI spec, but provisioner side car version 1.4.0
	// mandates it
	var contentSource *csi.VolumeContentSource
	if sourceSnapshotID != "" {
		contentSource = &csi.VolumeContentSource{
			Type: &csi.VolumeContentSource_Snapshot{
				Snapshot: &csi.VolumeContentSource_SnapshotSource{
					SnapshotId: sourceSnapshotID,
				},
			},
		}
	} else if sourceVolumeID != "" {
		contentSource = &csi.VolumeContentSource{
			Type: &csi.VolumeContentSource_Volume{
				Volume: &csi.VolumeContentSource_VolumeSource{
					VolumeId: sourceVolumeID,
				},
			},
		}
	}

	// Mark volumes created from snapshots/clones as needing recursive ownership fix
	// This signals NodeStageVolume to apply fsGroup ownership recursively to handle
	// files copied from the source that may have different ownership
	if contentSource != nil && directoryBacked {
		attributes["NeedsRecursiveOwnership"] = "true"
		csmlog.WithContext(ctx).Debug("Volume created from snapshot/clone - marked for recursive ownership fix")
	}

	var volumeID string
	if directoryBacked {
		volumeID = id.GetDirectoryBackedVolumeID(ctx, volName, exportID, accessZone, clusterName)
	} else {
		volumeID = id.GetNormalizedVolumeID(ctx, volName, exportID, accessZone, clusterName)
	}

	vi := &csi.Volume{
		VolumeId:      volumeID,
		CapacityBytes: sizeInBytes,
		VolumeContext: attributes,
		ContentSource: contentSource,
	}
	return vi
}

func (s *service) DeleteVolume(
	ctx context.Context,
	req *csi.DeleteVolumeRequest) (
	resp *csi.DeleteVolumeResponse, err error,
) {
	// TODO more checks need to be done, e.g. if access mode is VolumeCapability_AccessMode_MULTI_NODE_XXX, then other nodes might still be using this volume, thus the delete should be skipped
	startTime := time.Now()
	defer func() {
		log := csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			csmlog.FieldComponent: "controller",
			csmlog.FieldOperation: "DeleteVolume",
		}).TrackDuration(startTime)
		if err != nil {
			log.Debugf("DeleteVolume Failed with error: %v", err)
		} else {
			log.Info("DeleteVolume Successful")
		}
	}()
	fields := csmlog.ExtractFieldsFromContext(ctx)

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		csmlog.FieldComponent: "controller",
		csmlog.FieldOperation: "DeleteVolume",
		csmlog.FieldProtocol:  "NFS",
		csmlog.FieldVolumeID:  req.GetVolumeId(),
	}).Info("DeleteVolume called")

	// set noProbeOnStart to false so subsequent calls can lead to probe
	noProbeOnStart.Store(false)

	// validate request
	if err := s.ValidateDeleteVolumeRequest(ctx, req); err != nil {
		csmlog.WithContext(ctx).Errorf("invalid volume id %v", err.Error())
		return &csi.DeleteVolumeResponse{}, nil
	}

	// parse the input volume id and fetch it's components
	volName, exportID, accessZone, clusterName, err := id.ParseNormalizedVolumeID(ctx, req.GetVolumeId())
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error %v", err.Error())
		return nil, err
	}

	fields[clusterName] = clusterName

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		csmlog.FieldOperation: "DeleteVolume",
		csmlog.FieldArrayID:   isiConfig.Endpoint,
	}).Debugf("Cluster Name: %v", clusterName)
	// probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		csmlog.WithContext(ctx).Error("Failed to probe with error: " + err.Error())
		return nil, err
	}
	s.logStatistics()
	quotaEnabled := s.opts.QuotaEnabled

	export, err := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone)
	if err != nil {
		if jsonError, ok := err.(*isiApi.JSONError); ok {
			if jsonError.StatusCode == 404 {
				// export not found means the volume doesn't exist
				return &csi.DeleteVolumeResponse{}, nil
			}
			return nil, err
		}
		return nil, err
	} else if export == nil {
		// in case it occurs the case that export is nil and error is also nil
		return &csi.DeleteVolumeResponse{}, nil
	}

	exportPath := (*export.Paths)[0]
	isiPath := isilonfs.GetIsiPathFromExportPath(exportPath)
	volumePath := isilonfs.GetPathForVolume(isiPath, volName)

	isROVolumeFromSnapshot := isiConfig.isiSvc.isROVolumeFromSnapshot(exportPath, accessZone)
	// If it is a RO volume and dataSource is snapshot
	if isROVolumeFromSnapshot {
		if err := s.processSnapshotTrackingDirectoryDuringDeleteVolume(ctx, volName, accessZone, export, isiConfig); err != nil {
			return nil, err
		}
		return &csi.DeleteVolumeResponse{}, nil
	}

	// Detect directory-backed mode: first try parsing from volume ID (self-contained),
	// then fall back to path-based detection for backward compatibility
	_, _, _, _, provisioningMode, parseErr := id.ParseVolumeIDWithMode(ctx, req.GetVolumeId())
	directoryBacked := (provisioningMode == id.ProvisioningModeDirectory)

	detectionMethod := "volumeID"
	if parseErr != nil || provisioningMode == "" {
		// Fallback: For directory-backed volumes, the export path is the shared export (parent directory)
		// For export-backed volumes, the export path contains the volume name as the last component
		expectedVolumePath := isilonfs.GetPathForVolume(isiPath, volName)
		directoryBacked = (exportPath != expectedVolumePath)
		detectionMethod = "pathComparison"
		if parseErr != nil {
			csmlog.WithContext(ctx).Debugf("Failed to parse volume ID with mode: %v, using path-based detection", parseErr)
		}
	}

	if directoryBacked {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"volumeName":       volName,
			"exportPath":       exportPath,
			"provisioningMode": provisioningMode,
			"detectionMethod":  detectionMethod,
		}).Info("Directory-backed volume detected in DeleteVolume")
	}

	// to ensure idempotency, check if the volume and export still exists.
	// k8s might have made the same DeleteVolume call in quick succession and the volume was already deleted in the first run
	csmlog.WithContext(ctx).Debugf("controller begins to delete volume, name '%s', quotaEnabled '%t', directoryBacked '%t'", volName, quotaEnabled, directoryBacked)

	// Delete quota: directory-backed uses path-based lookup, export-backed uses export description
	if directoryBacked {
		// For directory-backed volumes, the quota is on the subdirectory, not the shared export
		volumePath := isilonfs.GetPathForVolume(exportPath, volName)
		csmlog.WithContext(ctx).Debugf("attempting to delete quota for directory-backed volume at path '%s'", volumePath)

		quota, quotaErr := isiConfig.isiSvc.GetQuotaByPath(ctx, volumePath)
		if quotaErr == nil && quota != nil {
			csmlog.WithContext(ctx).Debugf("found quota ID '%s' for path '%s', deleting", quota.ID, volumePath)
			if err := isiConfig.isiSvc.ClearQuotaByID(ctx, quota.ID); err != nil {
				jsonError, ok := err.(*isiApi.JSONError)
				if !ok || jsonError.StatusCode != 404 {
					return nil, fmt.Errorf("failed to clear quota '%s' for directory-backed volume '%s': %v", quota.ID, volName, err)
				}
				csmlog.WithContext(ctx).Debugf("quota already deleted (404), continuing")
			}
		} else if quotaErr != nil {
			// Quota not found is acceptable (idempotent)
			csmlog.WithContext(ctx).Debugf("no quota found for path '%s' (may already be deleted): %v", volumePath, quotaErr)
		} else {
			csmlog.WithContext(ctx).Debugf("no quota set on directory-backed volume '%s', skip quota deletion", volName)
		}
	} else {
		// Export-backed: use original logic (quota ID from export description)
		if err := isiConfig.isiSvc.DeleteQuotaByExportIDWithZone(ctx, volName, exportID, accessZone); err != nil {
			jsonError, ok := err.(*isiApi.JSONError)
			if ok {
				if jsonError.StatusCode != 404 {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}

	if !directoryBacked {
		// Export-backed mode: delete the per-volume export
		// Before deleting the Volume, we would like to check if there are any
		// NFS exports which still exist on the Volume. These exports could
		// have been created out-of-band outside of CSI Driver.
		path := isilonfs.GetPathForVolume(isiPath, volName)
		params := isiApi.OrderedValues{
			{[]byte("path"), []byte(path)},
			{[]byte("zone"), []byte(accessZone)},
		}
		exports, err := isiConfig.isiSvc.GetExportsWithParams(ctx, params)
		if err != nil {
			jsonError, ok := err.(*isiApi.JSONError)
			if ok {
				if jsonError.StatusCode != 404 {
					return nil, err
				}
			}
			return nil, err
		}

		if exports != nil && exports.Total == 1 && exports.Exports[0].ID == exportID {
			csmlog.WithContext(ctx).Infof("controller begins to unexport id '%d', target path '%s', access zone '%s'", exportID, volName, accessZone)
			if err := isiConfig.isiSvc.UnexportByIDWithZone(ctx, exportID, accessZone); err != nil {
				return nil, err
			}
		} else if exports != nil && exports.Total > 1 {
			return nil, fmt.Errorf("exports found for volume %s in AccessZone %s. It is not safe to delete the volume", volName, accessZone)
		}
	} else {
		// Directory-backed mode: shared export is never deleted
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"exportID":   exportID,
			"exportPath": exportPath,
		}).Debug("Skipping export deletion for directory-backed volume (shared export protection)")
	}

	// Compute the full volume path for writable snapshot cleanup (always based on isiPath)
	volumePath = isilonfs.GetPathForVolume(isiPath, volName)

	// Writable snapshot cleanup (PR‑203) – leave this exactly as main has it
	getWritableSnapshot := getWritableSnapshotFunc(isiConfig)
	deleteWritableSnapshot := deleteWritableSnapshotFunc(isiConfig)
	if existingWS, getErr := getWritableSnapshot(ctx, volumePath); getErr == nil && existingWS != nil {
		if delErr := deleteWritableSnapshot(ctx, volumePath); delErr != nil {
			if !isIgnorableWritableSnapshotDeleteError(delErr) {
				return nil, delErr
			}
		}
	} else if getErr != nil && !isIgnorableWritableSnapshotLookupError(getErr) {
		csmlog.WithContext(ctx).Warnf("failed to determine writable snapshot state at '%s': %v", volumePath, getErr)
		if delErr := deleteWritableSnapshot(ctx, volumePath); delErr != nil && !isIgnorableWritableSnapshotDeleteError(delErr) {
			return nil, delErr
		}
	}

	// Now pick the correct base path for deleting the actual directory
	var volumeBasePath string
	if directoryBacked {
		// For directory-backed volumes, the directory lives under the shared export
		volumeBasePath = exportPath
	} else {
		// For export-backed volumes, the directory lives under isiPath
		volumeBasePath = isiPath
	}

	if !isiConfig.isiSvc.IsVolumeExistent(ctx, volumeBasePath, "", volName) {
		csmlog.WithContext(ctx).Debugf("volume '%s' not found under '%s', skip calling delete directory.", volName, volumeBasePath)
	} else {
		csmlog.WithContext(ctx).Debugf("deleting directory '%s' under base path '%s'", volName, volumeBasePath)
		if err := isiConfig.isiSvc.DeleteVolume(ctx, volumeBasePath, volName); err != nil {
			return nil, err
		}
	}
	return &csi.DeleteVolumeResponse{}, nil
}

var (
	getZoneByNameFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, zoneName string) (*v1.IsiZone, error) {
		return isiConfig.isiSvc.GetZoneByName
	}

	getSnapshotIsiPathComponentsFunc = func(isiConfig *IsilonClusterConfig) func(exportPath, zonePath string) (string, string, string) {
		return isiConfig.isiSvc.GetSnapshotIsiPathComponents
	}

	getSnapshotTrackingDirNameFunc = func(isiConfig *IsilonClusterConfig) func(snapshotName string) string {
		return isiConfig.isiSvc.GetSnapshotTrackingDirName
	}

	isVolumeExistentFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeID, volumeEntry string) bool {
		return isiConfig.isiSvc.IsVolumeExistent
	}

	deleteVolumeFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) error {
		return isiConfig.isiSvc.DeleteVolume
	}

	getSubDirectoryCountFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) (int64, error) {
		return isiConfig.isiSvc.GetSubDirectoryCount
	}

	unexportByIDWithZoneFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, exportID int, zoneName string) error {
		return isiConfig.isiSvc.UnexportByIDWithZone
	}

	removeSnapshotFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, snapID int64, snapName string) error {
		return isiConfig.isiSvc.client.RemoveSnapshot
	}
)

func isIgnorableWritableSnapshotLookupError(err error) bool {
	if err == nil {
		return false
	}
	if jsonError, ok := err.(*isiApi.JSONError); ok && jsonError.StatusCode == 404 {
		return true
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "not a member of writablesnapshot domain") ||
		strings.Contains(errMsg, "writable snapshot not found") ||
		strings.Contains(errMsg, "failed to open path")
}

func isIgnorableWritableSnapshotDeleteError(err error) bool {
	if err == nil {
		return false
	}
	if jsonError, ok := err.(*isiApi.JSONError); ok && jsonError.StatusCode == 404 {
		return true
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "not a member of writablesnapshot domain") ||
		strings.Contains(errMsg, "writable snapshot not found") ||
		strings.Contains(errMsg, "failed to open path")
}

func (s *service) processSnapshotTrackingDirectoryDuringDeleteVolume(
	ctx context.Context,
	volName string,
	accessZone string,
	export isi.Export,
	isiConfig *IsilonClusterConfig,
) error {
	exportPath := (*export.Paths)[0]

	// Get Zone Path
	zone, err := getZoneByNameFunc(isiConfig)(ctx, accessZone)
	if err != nil {
		return err
	}
	// Delete the snapshot tracking directory entry for this volume
	isiPath, snapshotName, _ := getSnapshotIsiPathComponentsFunc(isiConfig)(exportPath, zone.Path)
	csmlog.WithContext(ctx).Debugf("snapshot name associated with volume '%s' is '%s'", volName, snapshotName)

	// Populate names for snapshot's tracking dir, snapshot tracking dir entry for this volume
	// and snapshot delete marker
	snapshotTrackingDir := getSnapshotTrackingDirNameFunc(isiConfig)(snapshotName)
	snapshotTrackingDirEntryForVolume := fPath.Join(snapshotTrackingDir, volName)
	snapshotTrackingDirDeleteMarker := fPath.Join(snapshotTrackingDir, DeleteSnapshotMarker)

	csmlog.WithContext(ctx).Debugf("Delete the snapshot tracking directory entry '%s' for volume '%s'", snapshotTrackingDirEntryForVolume, volName)
	if isVolumeExistentFunc(isiConfig)(ctx, isiPath, "", snapshotTrackingDirEntryForVolume) {
		if err := deleteVolumeFunc(isiConfig)(ctx, isiPath, snapshotTrackingDirEntryForVolume); err != nil {
			return err
		}
	}

	// Get subdirectories count of snapshot tracking dir.
	// Every directory will have two subdirectory entries . and ..
	totalSubDirectories, err := getSubDirectoryCountFunc(isiConfig)(ctx, isiPath, snapshotTrackingDir)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get subdirectories count of snapshot tracking dir '%s'", snapshotTrackingDir)
		return nil
	}

	// Delete snapshot tracking directory, if required (i.e., if there is a
	// snapshot delete marker as a result of snapshot deletion on k8s side)
	if isVolumeExistentFunc(isiConfig)(ctx, isiPath, "", snapshotTrackingDirDeleteMarker) {
		// There are no more volumes present which were created using this snapshot
		// This indicates that there are only three subdirectories ., .. and snapshot delete marker.
		if totalSubDirectories == 3 {
			err = unexportByIDWithZoneFunc(isiConfig)(ctx, export.ID, accessZone)
			if err != nil {
				csmlog.WithContext(ctx).Errorf("failed to delete snapshot directory export with id '%v'", export.ID)
				return nil
			}
			// Delete snapshot tracking directory
			if err := deleteVolumeFunc(isiConfig)(ctx, isiPath, snapshotTrackingDir); err != nil {
				csmlog.WithContext(ctx).Errorf("error while deleting snapshot tracking directory '%s'", fPath.Join(isiPath, snapshotName))
				return nil
			}
			// Delete snapshot
			err = removeSnapshotFunc(isiConfig)(context.Background(), -1, snapshotName)
			if err != nil {
				csmlog.WithContext(ctx).Errorf("error deleting snapshot: '%s'", err.Error())
				return nil
			}
		}
	}

	if totalSubDirectories == 2 {
		// Delete snapshot tracking directory
		if err := deleteVolumeFunc(isiConfig)(ctx, isiPath, snapshotTrackingDir); err != nil {
			csmlog.WithContext(ctx).Errorf("error while deleting snapshot tracking directory '%s'", fPath.Join(isiPath, snapshotName))
			return nil
		}
	}

	return nil
}

func (s *service) ControllerExpandVolume(
	ctx context.Context,
	req *csi.ControllerExpandVolumeRequest,
) (resp *csi.ControllerExpandVolumeResponse, err error) {
	startTime := time.Now()
	defer func() {
		log := csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			csmlog.FieldComponent: "controller",
			csmlog.FieldOperation: "ControllerExpandVolume",
		}).TrackDuration(startTime)
		if err != nil {
			log.Debugf("ControllerExpandVolume Failed with error: %v", err)
		} else {
			log.Info("ControllerExpandVolume Successful")
		}
	}()
	fields := csmlog.ExtractFieldsFromContext(ctx)

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		csmlog.FieldComponent: "controller",
		csmlog.FieldOperation: "ControllerExpandVolume",
		csmlog.FieldProtocol:  "NFS",
		csmlog.FieldVolumeID:  req.GetVolumeId(),
	}).Info("ControllerExpandVolume called")

	volName, exportID, accessZone, clusterName, err := id.ParseNormalizedVolumeID(ctx, req.GetVolumeId())
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error %v", err.Error())
		return nil, err
	}

	fields[clusterName] = clusterName

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		csmlog.FieldOperation: "ControllerExpandVolume",
		csmlog.FieldArrayID:   isiConfig.Endpoint,
	}).Debugf("Cluster Name: %v", clusterName)

	// auto probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	requiredBytes := req.GetCapacityRange().GetRequiredBytes()

	// Detect directory-backed mode using volume ID provisioning mode
	isiPath := isiConfig.IsiPath
	_, _, _, _, provisioningMode, parseErr := id.ParseVolumeIDWithMode(ctx, req.GetVolumeId())
	directoryBacked := (provisioningMode == id.ProvisioningModeDirectory)

	// Get export to determine the actual export path for fallback detection
	export, err := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "export '%d' not found in access zone '%s': %v", exportID, accessZone, err)
	}

	exportPath := ""
	if export.Paths != nil && len(*export.Paths) > 0 {
		exportPath = (*export.Paths)[0]
	}

	detectionMethod := "volumeID"
	if parseErr != nil || provisioningMode == "" {
		// Fallback: For directory-backed volumes, the export path is the shared export (parent directory)
		// For export-backed volumes, the export path contains the volume name as the last component
		expectedVolumePath := isilonfs.GetPathForVolume(isiPath, volName)
		directoryBacked = (exportPath != expectedVolumePath)
		detectionMethod = "pathComparison"
		if parseErr != nil {
			csmlog.WithContext(ctx).Debugf("Failed to parse volume ID with mode: %v, using path-based detection", parseErr)
		}
	}

	if directoryBacked {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"volumeName":       volName,
			"exportPath":       exportPath,
			"provisioningMode": provisioningMode,
			"detectionMethod":  detectionMethod,
		}).Info("Directory-backed volume detected in ControllerExpandVolume")
	}

	// Get quota: directory-backed uses path-based lookup, export-backed uses export description
	var quota isi.Quota
	var quotaErr error

	if directoryBacked {
		// For directory-backed volumes, the quota is on the subdirectory, not the shared export
		volumePath := isilonfs.GetPathForVolume(exportPath, volName)
		csmlog.WithContext(ctx).Debugf("attempting to get quota for directory-backed volume at path '%s'", volumePath)
		quota, quotaErr = isiConfig.isiSvc.GetQuotaByPath(ctx, volumePath)
	} else {
		quota, quotaErr = isiConfig.isiSvc.GetVolumeQuota(ctx, volName, exportID, accessZone)
	}

	quotaExists := (quotaErr == nil && quota != nil)

	if s.opts.QuotaEnabled || quotaExists {
		if quotaErr != nil {
			return nil, status.Errorf(codes.NotFound, "quota not found for volume '%s': %v", volName, quotaErr)
		}

		quotaSizeHard := quota.Thresholds.Hard
		quotaSizeSoft := quota.Thresholds.Soft
		quotaSizeAdvisory := quota.Thresholds.Advisory
		quotaSoftGrace := quota.Thresholds.SoftGrace

		if requiredBytes <= quotaSizeHard {
			// volume capacity is larger than or equal to the target capacity, return OK
			return &csi.ControllerExpandVolumeResponse{CapacityBytes: quotaSizeHard, NodeExpansionRequired: false}, nil
		}

		if quotaSizeHard == 0 {
			return nil, status.Errorf(codes.Internal, "Hard limit is 0, cannot proceed with volume expansion")
		}

		updatedSoftLimit := quotaSizeSoft * (requiredBytes / quotaSizeHard)
		updatedAdvisoryLimit := quotaSizeAdvisory * (requiredBytes / quotaSizeHard)

		if err = isiConfig.isiSvc.UpdateQuotaSize(ctx, quota.ID, requiredBytes, updatedSoftLimit, updatedAdvisoryLimit, quotaSoftGrace); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update quota: %v", err)
		}
	}

	return &csi.ControllerExpandVolumeResponse{CapacityBytes: requiredBytes, NodeExpansionRequired: false}, nil
}

type exportClientAdders struct {
	addClient  addClientFunc
	addClients addClientsFunc
}

func (s *service) getExportClientAdders(rootClientEnabled, readOnly, isROVolumeFromSnapshot bool, isiConfig *IsilonClusterConfig) exportClientAdders {
	if readOnly {
		if rootClientEnabled && isROVolumeFromSnapshot {
			return exportClientAdders{
				addClient:  isiConfig.isiSvc.AddExportRootClientByIDWithZone,
				addClients: isiConfig.isiSvc.AddExportRootClientsByIDWithZone,
			}
		}
		return exportClientAdders{
			addClient:  isiConfig.isiSvc.AddExportReadOnlyClientByIDWithZone,
			addClients: isiConfig.isiSvc.AddExportReadOnlyClientsByIDWithZone,
		}
	}

	if rootClientEnabled {
		return exportClientAdders{
			addClient:  isiConfig.isiSvc.AddExportRootClientByIDWithZone,
			addClients: isiConfig.isiSvc.AddExportRootClientsByIDWithZone,
		}
	}

	return exportClientAdders{
		addClient:  isiConfig.isiSvc.AddExportClientByIDWithZone,
		addClients: isiConfig.isiSvc.AddExportClientsByIDWithZone,
	}
}

/*
 * ControllerPublishVolume : Checks all params and validity
 */
func (s *service) ControllerPublishVolume(
	ctx context.Context,
	req *csi.ControllerPublishVolumeRequest) (
	*csi.ControllerPublishVolumeResponse, error,
) {
	var (
		accessZone             string
		exportPath             string
		isiPath                string
		newExportIP            []string
		isROVolumeFromSnapshot bool
	)

	logFields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", logFields["csi.requestid"])

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		csmlog.FieldComponent: "controller",
		csmlog.FieldOperation: "ControllerPublishVolume",
		csmlog.FieldProtocol:  "NFS",
		csmlog.FieldVolumeID:  req.GetVolumeId(),
		csmlog.FieldNodeID:    req.GetNodeId(),
	}).Info("ControllerPublishVolume called")

	// set noProbeOnStart to false so subsequent calls can lead to probe
	noProbeOnStart.Store(false)

	volumeContext := req.GetVolumeContext()
	if volumeContext != nil {
		csmlog.WithContext(ctx).Infof("VolumeContext:")
		for key, value := range volumeContext {
			csmlog.WithContext(ctx).Infof("    [%s]=%s", key, value)
		}
		// Check volumeContext for AzNetwork and get the corresponding IP from node labels
		if azNet, ok := volumeContext["AzNetwork"]; ok && azNet != "" {
			var err error
			newExportIP, err = s.getIpsFromAZNetworkLabel(ctx, req.GetNodeId(), azNet)
			if err != nil {
				return nil, status.Error(codes.Internal, fmt.Sprintf("getting AZNetwork IPs: %v", err))
			}
			csmlog.WithContext(ctx).Debugf("AzNetwork %s matched a node label IP %s", azNet, newExportIP)

		} else {
			csmlog.WithContext(ctx).Debugf("AzNetwork not found in volumeContext, proceeding without it")
		}
	}

	// Multi-NIC: if mode=multi and allowedNetworks configured, resolve all NFS IPs from node labels.
	// AzNetwork takes priority — only resolve if newExportIP is still empty.
	if len(newExportIP) == 0 && s.opts.allowedNetworksMode == constants.AllowedNetworksModeMulti && len(s.opts.allowedNetworks) > 0 {
		multiIPs, err := s.getIpsFromAllowedNetworks(ctx, req.GetNodeId())
		if err != nil {
			csmlog.WithContext(ctx).WithFields(csmlog.Fields{
				"operation": "ControllerPublishVolume",
				"mode":      s.opts.allowedNetworksMode,
				"node_id":   req.GetNodeId(),
				"success":   false,
			}).Errorf("multi-NIC: failed to resolve NFS IPs from node labels: %v", err)
			return nil, status.Error(codes.Internal, fmt.Sprintf("multi-NIC: failed to resolve NFS IPs from node labels: %v", err))
		}
		if len(multiIPs) > 0 {
			newExportIP = multiIPs
			csmlog.WithContext(ctx).WithFields(csmlog.Fields{
				"operation": "ControllerPublishVolume",
				"mode":      s.opts.allowedNetworksMode,
				"node_id":   req.GetNodeId(),
				"ip_count":  len(multiIPs),
				"ips":       multiIPs,
				"success":   true,
			}).Infof("multi-NIC: resolved %d NFS IPs from node labels: %v", len(multiIPs), multiIPs)
		}
	}

	volID := req.GetVolumeId()
	if volID == "" {
		return nil, status.Error(codes.InvalidArgument,
			GetMessageWithReqID(runID, "volume ID is required"))
	}

	volName, exportID, accessZone, clusterName, err := id.ParseNormalizedVolumeID(ctx, volID)
	if err != nil {
		return nil, status.Error(codes.NotFound, GetMessageWithReqID(runID, "failed to parse volume ID '%s', error : '%v'", volID, err))
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error %v", err.Error())
		return nil, err
	}

	logFields[clusterName] = clusterName

	csmlog.WithContext(ctx).Debugf("Cluster Name: %v", clusterName)

	if err := s.autoProbe(ctx, isiConfig); err != nil {
		csmlog.WithContext(ctx).Error("Failed to probe with error: " + err.Error())
		return nil, err
	}

	if exportID == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid export ID")
	}

	if exportPath = volumeContext[ExportPathParam]; exportPath == "" {
		// if not in request, calculate it from the export
		exportPath, err = getExportPathFromExportID(ctx, isiConfig, exportID, accessZone)
		if err != nil {
			csmlog.WithContext(ctx).Infof("Could not get export path by export ID: %v", err)
			exportPath = isilonfs.GetPathForVolume(isiConfig.IsiPath, volName)
		}
	}
	csmlog.WithContext(ctx).Infof("Export path: %s", exportPath)
	isROVolumeFromSnapshot = isiConfig.isiSvc.isROVolumeFromSnapshot(volumeContext["Path"], accessZone)

	if isROVolumeFromSnapshot {
		csmlog.WithContext(ctx).Info("Volume source is snapshot")
		if export, err := isiConfig.isiSvc.GetExportWithPathAndZone(ctx, exportPath, accessZone); err != nil || export == nil {
			return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "error retrieving export for %s", exportPath))
		}
	} else if volumeContext["ProvisioningMode"] != "directory" {
		// Skip the per-volume GetVolumeWithIsiPath check; existence was verified during CreateVolume.
		isiPath = isilonfs.GetIsiPathFromExportPath(exportPath)
		vol, err := isiConfig.isiSvc.GetVolumeWithIsiPath(ctx, isiPath, "", volName)
		if err != nil || vol.Name == "" {
			return nil, status.Error(codes.Internal,
				GetMessageWithReqID(runID, "failure checking volume status before controller publish: %s",
					err.Error()))
		}
	}

	nodeID := req.GetNodeId()

	if nodeID == "" {
		return nil, status.Error(codes.NotFound,
			GetMessageWithReqID(runID, "node ID is required"))
	}

	vc := req.GetVolumeCapability()
	if vc == nil {
		return nil, status.Error(codes.InvalidArgument,
			GetMessageWithReqID(runID, "volume capability is required"))
	}

	am := vc.GetAccessMode()
	if am == nil {
		return nil, status.Error(codes.InvalidArgument,
			GetMessageWithReqID(runID, "access mode is required"))
	}

	if am.Mode == csi.VolumeCapability_AccessMode_UNKNOWN {
		return nil, status.Error(codes.InvalidArgument,
			GetMessageWithReqID(runID, errUnknownAccessMode))
	}

	vcs := []*csi.VolumeCapability{req.GetVolumeCapability()}
	if !checkValidAccessTypes(vcs) {
		return nil, status.Error(codes.InvalidArgument,
			GetMessageWithReqID(runID, errUnknownAccessType))
	}

	rootClientEnabled := false
	rootClientEnabledStr := volumeContext[RootClientEnabledParam]
	val, err := strconv.ParseBool(rootClientEnabledStr)
	if err == nil {
		rootClientEnabled = val
	}

	_, _, nodeIP, err := id.ParseNodeID(ctx, nodeID)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to parse node id '%s' with error : %s", nodeID, err.Error())
		return nil, status.Error(codes.NotFound,
			GetMessageWithReqID(runID, "failed to parse node id '%s'", nodeID))
	}

	// Multi-NIC: use multi-IP export count when newExportIP is populated
	var exportCount int64
	if len(newExportIP) > 0 && s.opts.allowedNetworksMode == constants.AllowedNetworksModeMulti {
		exportCount, err = isiConfig.isiSvc.GetExportsCountAttachedToNodeIPs(ctx, newExportIP, accessZone)
	} else {
		exportCount, err = isiConfig.isiSvc.GetExportsCountAttachedToNode(ctx, nodeIP)
	}
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to export count for node id '%s' with error : %s", nodeID, err.Error())
		return nil, status.Error(codes.InvalidArgument,
			GetMessageWithReqID(runID, "failed to export count for node id '%s'", nodeID))
	}

	if s.opts.MaxVolumesPerNode > 0 && exportCount >= s.opts.MaxVolumesPerNode {
		csmlog.WithContext(ctx).Errorf("maximum volume limit reached for node : '%s'", nodeID)
		return nil, status.Error(codes.InvalidArgument,
			GetMessageWithReqID(runID, "maximum volume limit reached for node : '%s'", nodeID))
	}

	// Multiple PVCs share the same NFS export. Concurrent ControllerPublishVolume calls for
	// different directory-backed volumes on the same node can produce duplicate client entries.
	// We serialize per (exportID, accessZone) and skip re-authorization if the node is already
	// in the export client list.

	// Detect directory-backed mode: first try parsing from volume ID (self-contained),
	// then fall back to VolumeContext for backward compatibility
	_, _, _, _, provisioningMode, parseErr := id.ParseVolumeIDWithMode(ctx, volID)
	isDirectoryBacked := (provisioningMode == id.ProvisioningModeDirectory)
	if parseErr == nil && provisioningMode == "" {
		// Volume ID parsed successfully but has no mode token; check VolumeContext as fallback
		if volumeContext["ProvisioningMode"] == "directory" {
			isDirectoryBacked = true
			csmlog.WithContext(ctx).Debugf("Directory-backed mode detected via VolumeContext fallback for volume %s", volID)
		}
	} else if parseErr != nil {
		csmlog.WithContext(ctx).Debugf("Failed to parse volume ID with mode: %v, using VolumeContext fallback", parseErr)
		isDirectoryBacked = volumeContext["ProvisioningMode"] == "directory"
	}

	if isDirectoryBacked {
		// Label PV with directory-backed metadata for queryability (best-effort, non-blocking)
		// For directory-backed volumes, use SharedExportPath from context (the actual shared export)
		// rather than exportPath (which is the full volume path including subdirectory)
		sharedExportPath := volumeContext["SharedExportPath"]
		if sharedExportPath == "" {
			sharedExportPath = exportPath // Fallback for backward compatibility
		}
		go func() {
			labelCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			s.labelDirectoryBackedPV(labelCtx, volName, exportID, accessZone, sharedExportPath)
		}()

		muKey := fmt.Sprintf("%d:%s", exportID, accessZone)
		muVal, _ := s.directoryExportMu.LoadOrStore(muKey, &sync.RWMutex{})
		mu := muVal.(*sync.RWMutex)
		// Use RWMutex to allow concurrent reads (IsHostAlreadyAdded checks) while serializing writes (addClient API calls).
		// This improves cluster bootstrap performance by allowing multiple nodes to check authorization concurrently.
		// The hot path (node already authorized) uses RLock for concurrent reads.
		mu.RLock()
		if isiConfig.isiSvc.IsHostAlreadyAdded(ctx, exportID, accessZone, nodeID) {
			mu.RUnlock()
			csmlog.WithContext(ctx).Infof("Directory-backed: node %s already authorized to shared export %d (zone: %s), skipping re-authorization", nodeID, exportID, accessZone)
			return &csi.ControllerPublishVolumeResponse{}, nil
		}
		mu.RUnlock()

		// Cold path: node not authorized, need to add client. Use Lock to serialize export modifications.
		mu.Lock()
		defer mu.Unlock()

		csmlog.WithContext(ctx).Infof("Directory-backed: authorizing node %s to shared export %d (zone: %s)", nodeID, exportID, accessZone)
	}

	readOnly := (am.Mode == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY)
	adders := s.getExportClientAdders(rootClientEnabled, readOnly, isROVolumeFromSnapshot, isiConfig)
	addClientFunc := adders.addClient
	addClientsFunc := adders.addClients
	normalAdders := s.getExportClientAdders(false, false, false, isiConfig)
	normalClientFunc := normalAdders.addClient
	normalClientsFunc := normalAdders.addClients

	switch am.Mode {
	case csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER:
		if isROVolumeFromSnapshot {
			err = fmt.Errorf("unsupported access mode: '%s'", am.String())
			break
		}

		if !isiConfig.isiSvc.IsHostAlreadyAdded(ctx, exportID, accessZone, id.DummyHostNodeID) {
			if len(newExportIP) > 0 {
				err = isiConfig.isiSvc.AddExportClientByIPWithZone(ctx, clusterName, exportID, accessZone, id.DummyHostNodeID, newExportIP, normalClientFunc, normalClientsFunc, s.opts.allowedNetworksMode)
			} else {
				err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, id.DummyHostNodeID, *isiConfig.IgnoreUnresolvableHosts, isiConfig.isiSvc.AddExportClientByIDWithZone)
			}
		}

		if len(newExportIP) > 0 {
			csmlog.WithContext(ctx).Debugf("AzNetwork label used to publish volume at %s", newExportIP)
			err = isiConfig.isiSvc.AddExportClientByIPWithZone(ctx, clusterName, exportID, accessZone, nodeID, newExportIP, addClientFunc, addClientsFunc, s.opts.allowedNetworksMode)
		} else {
			err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, nodeID, *isiConfig.IgnoreUnresolvableHosts, addClientFunc)
		}

		if err == nil && rootClientEnabled {
			if len(newExportIP) > 0 {
				csmlog.WithContext(ctx).Debugf("AzNetwork label used to publish volume at %s", newExportIP)
				err = isiConfig.isiSvc.AddExportClientByIPWithZone(ctx, clusterName, exportID, accessZone, nodeID, newExportIP, normalClientFunc, normalClientsFunc, s.opts.allowedNetworksMode)
			} else {
				err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, nodeID, *isiConfig.IgnoreUnresolvableHosts, isiConfig.isiSvc.AddExportClientByIDWithZone)
			}
		}
	case csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY:
		// since read-only has higher privileges than root-clients, add to root-clients in exports on powerscale if root client enabled is set to true
		if rootClientEnabled && isROVolumeFromSnapshot {
			csmlog.WithContext(ctx).Debugf("ROVolumeFromSnapshot & rootClientEnabled is set to true, add to root clients")
			if len(newExportIP) > 0 {
				csmlog.WithContext(ctx).Debugf("AzNetwork label used to publish volume at %s", newExportIP)
				err = isiConfig.isiSvc.AddExportClientByIPWithZone(ctx, clusterName, exportID, accessZone, nodeID, newExportIP, addClientFunc, addClientsFunc, s.opts.allowedNetworksMode)
			} else {
				err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, nodeID, *isiConfig.IgnoreUnresolvableHosts, isiConfig.isiSvc.AddExportRootClientByIDWithZone)
			}
		} else {
			if len(newExportIP) > 0 {
				csmlog.WithContext(ctx).Debugf("AzNetwork label used to publish volume at %s", newExportIP)
				err = isiConfig.isiSvc.AddExportClientByIPWithZone(ctx, clusterName, exportID, accessZone, nodeID, newExportIP, addClientFunc, addClientsFunc, s.opts.allowedNetworksMode)
			} else {
				err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, nodeID, *isiConfig.IgnoreUnresolvableHosts, isiConfig.isiSvc.AddExportReadOnlyClientByIDWithZone)
			}
		}
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER:
		if isROVolumeFromSnapshot {
			err = fmt.Errorf("unsupported access mode: '%s'", am.String())
			break
		}
		if !isDirectoryBacked && isiConfig.isiSvc.OtherClientsAlreadyAdded(ctx, exportID, accessZone, nodeID) {
			return nil, status.Error(codes.NotFound, GetMessageWithReqID(runID,
				"export %d in access zone %s already has other clients added to it, and the access mode is %s, thus the request fails", exportID, accessZone, am.Mode))
		}

		if !isiConfig.isiSvc.IsHostAlreadyAdded(ctx, exportID, accessZone, id.DummyHostNodeID) {
			if len(newExportIP) > 0 {
				err = isiConfig.isiSvc.AddExportClientByIPWithZone(ctx, clusterName, exportID, accessZone, id.DummyHostNodeID, newExportIP, normalClientFunc, normalClientsFunc, s.opts.allowedNetworksMode)
			} else {
				err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, id.DummyHostNodeID, *isiConfig.IgnoreUnresolvableHosts, isiConfig.isiSvc.AddExportClientByIDWithZone)
			}
		}
		if len(newExportIP) > 0 {
			csmlog.WithContext(ctx).Debugf("AzNetwork label used to publish volume at %s", newExportIP)
			err = isiConfig.isiSvc.AddExportClientByIPWithZone(ctx, clusterName, exportID, accessZone, nodeID, newExportIP, addClientFunc, addClientsFunc, s.opts.allowedNetworksMode)
		} else {
			err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, nodeID, *isiConfig.IgnoreUnresolvableHosts, addClientFunc)
		}
		if err == nil && rootClientEnabled {
			if len(newExportIP) > 0 {
				csmlog.WithContext(ctx).Debugf("AzNetwork label used to publish volume at %s", newExportIP)
				err = isiConfig.isiSvc.AddExportClientByIPWithZone(ctx, clusterName, exportID, accessZone, nodeID, newExportIP, normalClientFunc, normalClientsFunc, s.opts.allowedNetworksMode)
			} else {
				err = isiConfig.isiSvc.AddExportClientNetworkIdentifierByIDWithZone(ctx, clusterName, exportID, accessZone, nodeID, *isiConfig.IgnoreUnresolvableHosts, isiConfig.isiSvc.AddExportClientByIDWithZone)
			}
		}
	default:
		return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "unsupported access mode: %s", am.String()))
	}

	if err != nil {
		return nil, status.Error(codes.Internal, GetMessageWithReqID(runID,
			"internal error occurred when attempting to add client ip %s to export %d, error : %v", nodeID, exportID, err))
	}
	return &csi.ControllerPublishVolumeResponse{}, nil
}

func (s *service) ValidateVolumeCapabilities(
	ctx context.Context,
	req *csi.ValidateVolumeCapabilitiesRequest) (
	*csi.ValidateVolumeCapabilitiesResponse, error,
) {
	var (
		exportPath string
		isiPath    string
	)

	logFields := csmlog.ExtractFieldsFromContext(ctx)
	// parse the input volume id and fetch it's components
	volID := req.GetVolumeId()
	volName, exportID, accessZone, clusterName, err := id.ParseNormalizedVolumeID(ctx, volID)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error %v ", err.Error())
		return nil, err
	}

	logFields[clusterName] = clusterName

	fields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", fields["csi.requestid"])
	csmlog.WithContext(ctx).Debugf("Cluster Name: %v", clusterName)

	if err := s.autoProbe(ctx, isiConfig); err != nil {
		csmlog.WithContext(ctx).Error("Failed to probe with error: " + err.Error())
		return nil, err
	}

	volumeContext := req.GetVolumeContext()
	if exportPath = volumeContext[ExportPathParam]; exportPath == "" {
		// if not in request, calculate it from the export
		exportPath, err = getExportPathFromExportID(ctx, isiConfig, exportID, accessZone)
		if err != nil {
			csmlog.WithContext(ctx).Infof("Could not get export path by export ID: %v", err)
			exportPath = isilonfs.GetPathForVolume(isiConfig.IsiPath, volName)
		}
	}
	isiPath = isilonfs.GetIsiPathFromExportPath(exportPath)
	vol, err := s.getVolByName(ctx, isiPath, volName, isiConfig)
	if err != nil {
		return nil, status.Error(codes.Internal,
			GetMessageWithReqID(runID, "failure checking volume status for capabilities: %s",
				err.Error()))
	}

	vcs := req.GetVolumeCapabilities()
	supported, reason := validateVolumeCaps(vcs, vol)

	resp := &csi.ValidateVolumeCapabilitiesResponse{}
	if supported {
		// The optional fields volume_context and parameters are not passed.
		confirmed := &csi.ValidateVolumeCapabilitiesResponse_Confirmed{}
		confirmed.VolumeCapabilities = vcs
		resp.Confirmed = confirmed
	} else {
		resp.Message = reason
	}

	return resp, nil
}

func (s *service) ListVolumes(ctx context.Context,
	req *csi.ListVolumesRequest,
) (*csi.ListVolumesResponse, error) {
	startTime := time.Now()
	var err error
	defer func() {
		log := csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			csmlog.FieldComponent: "controller",
			csmlog.FieldOperation: "ListVolumes",
		}).TrackDuration(startTime)
		if err != nil {
			log.Debugf("ListVolumes Failed with error: %v", err)
		} else {
			log.Info("ListVolumes Successful")
		}
	}()

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		csmlog.FieldComponent: "controller",
		csmlog.FieldOperation: "ListVolumes",
	}).Info("ListVolumes called")

	// Validate MaxEntries
	if req.MaxEntries < 0 {
		err = status.Error(codes.InvalidArgument, "Invalid max entries")
		return nil, err
	}

	// Get the default cluster config
	clusterName := s.defaultIsiClusterName
	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error %v", err.Error())
		return nil, err
	}

	// Auto probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		err = status.Error(codes.FailedPrecondition, err.Error())
		return nil, err
	}

	var (
		filesystems []*v2.ContainerChild
		resume      string
	)

	resp := new(csi.ListVolumesResponse)

	// Use the namespace API to list filesystems
	containerPath := isiConfig.IsiPath
	if containerPath == "" {
		containerPath = constants.DefaultIsiPath
	}

	if req.MaxEntries == 0 && req.StartingToken == "" {
		// No restriction - get all filesystems
		filesystems, err = isiConfig.isiSvc.GetFilesystems(ctx, containerPath)
		if err != nil {
			err = status.Error(codes.Internal, "Cannot get filesystems")
			return nil, err
		}
	} else {
		maxEntries := strconv.Itoa(int(req.MaxEntries))
		if req.StartingToken == "" {
			// Get the first page
			filesystems, resume, err = isiConfig.isiSvc.GetFilesystemsWithLimit(ctx, containerPath, maxEntries)
			if err != nil {
				err = status.Error(codes.Internal, "Cannot get filesystems with limit")
				return nil, err
			}
		} else {
			// Continue from previous page
			filesystems, resume, err = isiConfig.isiSvc.GetFilesystemsWithResume(ctx, containerPath, int(req.MaxEntries), req.StartingToken)
			if err != nil {
				var jsonErr *isiApi.JSONError
				var htmlErr *isiApi.HTMLError
				if (errors.As(err, &jsonErr) && jsonErr.StatusCode == http.StatusBadRequest) ||
					(errors.As(err, &htmlErr) && htmlErr.StatusCode == http.StatusBadRequest) {
					err = status.Error(codes.Aborted, "The starting token is not valid")
				} else {
					err = status.Error(codes.Internal, "Cannot get filesystems with resume token")
				}
				return nil, err
			}
		}
		resp.NextToken = resume
	}

	containerPath = fPath.Clean(containerPath)
	exports, exportErr := isiConfig.isiSvc.GetExports(ctx)
	if exportErr != nil {
		err = status.Error(codes.Internal, "Cannot get exports")
		return nil, err
	}

	exportByPath := make(map[string][]isi.Export, len(exports))
	for _, export := range exports {
		if export == nil || export.Paths == nil {
			continue
		}
		for _, exportPath := range *export.Paths {
			cleanPath := fPath.Clean(exportPath)
			exportByPath[cleanPath] = append(exportByPath[cleanPath], export)
		}
	}

	type listVolumeCandidate struct {
		index    int
		fs       *v2.ContainerChild
		fullPath string
		exports  []isi.Export
	}

	candidates := make([]listVolumeCandidate, 0, len(filesystems))
	for i, fs := range filesystems {
		if fs == nil || fs.Name == nil {
			continue
		}

		volName := *fs.Name
		fullPath := ""
		if fs.Path != nil && *fs.Path != "" {
			fullPath = fPath.Join(*fs.Path, volName)
		} else {
			fullPath = isilonfs.GetPathForVolume(containerPath, volName)
		}
		fullPath = fPath.Clean(fullPath)

		candidateExports, ok := exportByPath[fullPath]
		if !ok {
			continue
		}

		candidates = append(candidates, listVolumeCandidate{
			index:    i,
			fs:       fs,
			fullPath: fullPath,
			exports:  candidateExports,
		})
	}

	results := make([]*csi.ListVolumesResponse_Entry, len(filesystems))
	workerCount := listVolumesWorkerCount
	if workerCount <= 0 {
		workerCount = 1
	}
	if workerCount > len(candidates) {
		workerCount = len(candidates)
	}

	if workerCount == 0 {
		resp.Entries = []*csi.ListVolumesResponse_Entry{}
		csmlog.WithContext(ctx).Debugf("ListVolumes returning %d volumes", 0)
		return resp, nil
	}

	candidateQueue := make(chan listVolumeCandidate)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for candidate := range candidateQueue {
				fs := candidate.fs
				volName := *fs.Name
				fullPath := candidate.fullPath

				var export isi.Export
				for _, matchedExport := range candidate.exports {
					if matchedExport == nil {
						continue
					}
					if isCSIManagedVolume(ctx, isiConfig, fullPath, matchedExport) {
						export = matchedExport
						break
					}
				}
				if export == nil {
					csmlog.WithContext(ctx).Debugf("ListVolumes: '%s' is not a CSI-managed volume, skipping", fullPath)
					continue
				}

				accessZone := export.Zone
				if accessZone == "" {
					accessZone = constants.DefaultAccessZone
				}

				volumeID := id.GetNormalizedVolumeID(ctx, volName, export.ID, accessZone, clusterName)
				volume := &csi.Volume{
					VolumeId: volumeID,
					VolumeContext: map[string]string{
						"ID":          strconv.Itoa(export.ID),
						"Name":        volName,
						"Path":        fullPath,
						"AccessZone":  accessZone,
						"ClusterName": clusterName,
						"AzServiceIP": isiConfig.EndpointURL,
					},
				}

				if fs.Type != nil {
					volume.VolumeContext["Type"] = *fs.Type
				}
				if fs.Size != nil {
					volume.VolumeContext["Size"] = strconv.Itoa(*fs.Size)
				}
				if fs.Owner != nil {
					volume.VolumeContext["Owner"] = *fs.Owner
				}
				if fs.Group != nil {
					volume.VolumeContext["Group"] = *fs.Group
				}

				results[candidate.index] = &csi.ListVolumesResponse_Entry{Volume: volume}
			}
		}()
	}

	for _, candidate := range candidates {
		candidateQueue <- candidate
	}
	close(candidateQueue)
	wg.Wait()

	entries := make([]*csi.ListVolumesResponse_Entry, 0, len(results))
	for _, entry := range results {
		if entry != nil {
			entries = append(entries, entry)
		}
	}

	resp.Entries = entries
	csmlog.WithContext(ctx).Debugf("ListVolumes returning %d volumes", len(entries))

	return resp, nil
}

func (s *service) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	fields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", fields["csi.requestid"])
	var (
		startToken  int
		maxEntries  = int(req.GetMaxEntries())
		snapshotID  = req.GetSnapshotId()
		sourceVolID = req.GetSourceVolumeId()
	)

	csmlog.WithContext(ctx).Infof("Request received: snapshotID=%s, sourceVolID=%s, startToken=%s, maxEntries=%d", snapshotID, sourceVolID, req.GetStartingToken(), maxEntries)

	if token := req.GetStartingToken(); token != "" {
		i, err := strconv.ParseInt(token, 10, 64)
		if err != nil {
			csmlog.WithContext(ctx).Errorf("Failed to parse starting token: %s, error=%v", token, err)
			return nil, status.Error(codes.Aborted, GetMessageWithReqID(runID, "unable to parse StartingToken: %v into uint32", token))
		}
		startToken = int(i)
		csmlog.WithContext(ctx).Debugf("Parsed starting token: startToken=%d", startToken)
	}

	snapshots, nextToken, err := s.listPowerScaleSnapshots(ctx, startToken, maxEntries, snapshotID, sourceVolID)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to list snapshots: %v", err)
		return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "failed to list snapshots: %v", err.Error()))
	}

	if len(snapshots) == 0 {
		csmlog.WithContext(ctx).Info("No snapshots found")
		return &csi.ListSnapshotsResponse{}, nil
	}

	entries := make([]*csi.ListSnapshotsResponse_Entry, len(snapshots))
	for i, snap := range snapshots {
		csmlog.WithContext(ctx).Debugf("Snapshot entry: index=%d, ID=%d, Name=%s, Path=%s, State=%s", i, snap.ID, snap.Name, snap.Path, snap.State)
		entries[i] = &csi.ListSnapshotsResponse_Entry{
			Snapshot: s.getCSISnapshot(snap.Name, snap.Path, snap.Created, snap.Size),
		}
	}

	csmlog.WithContext(ctx).Debugf("Returning snapshot list: count=%d, nextToken=%s", len(entries), nextToken)
	return &csi.ListSnapshotsResponse{
		Entries:   entries,
		NextToken: nextToken,
	}, nil
}

func (s *service) listPowerScaleSnapshots(ctx context.Context, startToken, maxEntries int, snapID, srcID string) (isi.SnapshotList, string, error) {
	csmlog.WithContext(ctx).Infof("Entering listPowerScaleSnapshots: snapID=%s, srcID=%s", snapID, srcID)

	var filteredSnapshots isi.SnapshotList
	var totalSnapshots int

	for _, config := range s.getIsilonClusters() {
		snapshots, err := config.isiSvc.GetSnapshots(ctx)
		if err != nil {
			csmlog.WithContext(ctx).WithFields(csmlog.Fields{"cluster": config.ClusterName}).Errorf("Failed to get snapshots: %v", err)
			continue
		}
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{"cluster": config.ClusterName}).Debugf("Fetched snapshots: count=%d", len(snapshots))

		totalSnapshots += len(snapshots)

		if startToken < totalSnapshots {
			var snapsToProcess int
			if maxEntries == 0 {
				snapsToProcess = len(snapshots)
			} else {
				snapsToProcess = maxEntries - len(filteredSnapshots)
			}

			processedSnaps := 0
			for _, snap := range snapshots {
				if shouldIncludeSnapshot(ctx, snap, snapID, srcID) {
					csmlog.WithContext(ctx).WithFields(csmlog.Fields{"cluster": config.ClusterName, "snapshotID": snap.ID}).Debugf("Including snapshot: ID=%d", snap.ID)
					normalizeSnapshot(ctx, snap, config, s)
					filteredSnapshots = append(filteredSnapshots, snap)
					processedSnaps++

					if processedSnaps == snapsToProcess {
						break
					}
				}
			}

			if len(filteredSnapshots) >= startToken+maxEntries {
				break
			}
		}
	}

	if startToken > totalSnapshots {
		err := fmt.Errorf("startingToken=%d > totalSnapshots=%d", startToken, totalSnapshots)
		csmlog.WithContext(ctx).Errorf("Invalid starting token: %v", err)
		return nil, "", status.Errorf(codes.Aborted, "invalid starting token, error: %s", err.Error())
	}

	remaining := totalSnapshots - startToken
	if maxEntries == 0 || maxEntries > remaining {
		maxEntries = remaining
	}

	nextToken := ""
	if startToken+maxEntries < totalSnapshots {
		nextToken = fmt.Sprintf("%d", startToken+len(filteredSnapshots))
	}

	csmlog.WithContext(ctx).Infof("Returning snapshot slice: start=%d, count=%d, nextToken=%s", startToken, len(filteredSnapshots), nextToken)
	return filteredSnapshots[startToken:], nextToken, nil
}

func shouldIncludeSnapshot(ctx context.Context, snap isi.Snapshot, snapID, srcID string) bool {
	if snapID != "" {
		id, _, _, _ := id.ParseNormalizedSnapshotID(ctx, snapID)
		return strconv.FormatInt(snap.ID, 10) == id
	}
	if srcID != "" {
		srcName, _, _, _, _ := id.ParseNormalizedVolumeID(ctx, srcID)
		return strings.EqualFold(srcName, isilonfs.GetVolumeNameFromExportPath(snap.Path))
	}
	return true
}

// normalizeSnapshot updates the snapshot name and path with normalized IDs to convert isi snapshot into csi snapshot.
// It replaces the snapshot name with a normalized snapshot ID (e.g. 12345=_=_=cluster1=_=_=zone1)
// and the snapshot path with a normalized volume ID of the source volume (e.g. k8s-e89c9d089e=_=_=19=_=_=csi0zone=_=_=cluster1).
func normalizeSnapshot(ctx context.Context, snap isi.Snapshot, config *IsilonClusterConfig, s *service) {
	csmlog.WithContext(ctx).WithFields(csmlog.Fields{"snapshotID": snap.ID}).Debugf("Normalizing snapshot: ID=%d", snap.ID)

	snap.Name = id.GetNormalizedSnapshotID(ctx, strconv.FormatInt(snap.ID, 10), config.ClusterName, s.opts.AccessZone)
	volName := isilonfs.GetVolumeNameFromExportPath(snap.Path)
	export, _ := config.isiSvc.GetExportWithPath(ctx, snap.Path)

	if export != nil {
		snap.Path = id.GetNormalizedVolumeID(ctx, volName, export.ID, export.Zone, config.ClusterName)
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{"snapshotID": snap.ID}).Debugf("Normalized with export: exportID=%d, zone=%s", export.ID, export.Zone)
	}
}

func (s *service) ControllerUnpublishVolume(
	ctx context.Context,
	req *csi.ControllerUnpublishVolumeRequest) (
	*csi.ControllerUnpublishVolumeResponse, error,
) {
	logFields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", logFields["csi.requestid"])

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		csmlog.FieldComponent: "controller",
		csmlog.FieldOperation: "ControllerUnpublishVolume",
		csmlog.FieldProtocol:  "NFS",
		csmlog.FieldVolumeID:  req.GetVolumeId(),
		csmlog.FieldNodeID:    req.GetNodeId(),
	}).Info("ControllerUnpublishVolume called")

	// set noProbeOnStart to false so subsequent calls can lead to probe
	noProbeOnStart.Store(false)
	azNetwork := ""

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "ControllerUnpublishVolumeRequest.VolumeId is empty"))
	}

	volumeName, exportID, accessZone, clusterName, err := id.ParseNormalizedVolumeID(ctx, req.VolumeId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "failed to parse volume ID %s, error : %s", req.VolumeId, err.Error()))
	}

	// Get the PV with the given volumeName
	csmlog.WithContext(ctx).Debugf("Getting PV with name: %s", volumeName)
	pv, volErr := s.k8sclient.CoreV1().PersistentVolumes().Get(ctx, volumeName, metav1.GetOptions{})
	if volErr != nil {
		csmlog.WithContext(ctx).Warnf("Failed to get PV %s: %v", volumeName, volErr)
		// Not returning error code here as there is an authorization upgrade scenario where PV might not be found when it was created with tenant prefix
	} else {
		csmlog.WithContext(ctx).Debugf("Got PV: %s", pv.Name)
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error : %v ", err.Error())
		return nil, err
	}

	logFields[clusterName] = clusterName

	csmlog.WithContext(ctx).Debugf("Cluster Name: %v", clusterName)

	// auto probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		return nil, status.Error(codes.FailedPrecondition, GetMessageWithReqID(runID, "error %s", err.Error()))
	}

	if volErr == nil {
		var ok bool
		azNetwork, ok = pv.Spec.CSI.VolumeAttributes["AzNetwork"]
		if !ok {
			csmlog.WithContext(ctx).Debugf("AZNetwork attribute not found in PV %s", pv.Name)
		} else if azNetwork == "" {
			csmlog.WithContext(ctx).Debugf("AZNetwork value is empty in PV %s", pv.Name)
		}

		// Conditional deauthorization for directory-backed volumes:
		// Check if node has other directory-backed volumes on the same shared export.
		// Only remove node IP if this is the last volume from this export on this node.
		if pv.Spec.CSI != nil && pv.Spec.CSI.VolumeAttributes["ProvisioningMode"] == "directory" {
			// Acquire per-export mutex for thread-safe client list modification.
			// Use RWMutex with Lock() (write lock) since this modifies the export client list.
			// Hold the lock through IP removal to prevent race with concurrent ControllerPublishVolume.
			muKey := fmt.Sprintf("%d:%s", exportID, accessZone)
			muVal, _ := s.directoryExportMu.LoadOrStore(muKey, &sync.RWMutex{})
			mu := muVal.(*sync.RWMutex)
			mu.Lock()
			defer mu.Unlock() // Hold lock through IP removal to prevent DIR-5 race condition

			// Check if node has other directory-backed volumes on this shared export
			hasOtherVolumes, checkErr := s.hasOtherDirectoryBackedVolumesOnExport(ctx, req.GetNodeId(), exportID, accessZone, volumeName)
			if checkErr != nil {
				csmlog.WithContext(ctx).Warnf("Failed to check for other volumes on export %d: %v (skipping IP removal for safety)", exportID, checkErr)
				return &csi.ControllerUnpublishVolumeResponse{}, nil
			}

			if hasOtherVolumes {
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"exportID":    exportID,
					"accessZone":  accessZone,
					"nodeID":      req.GetNodeId(),
					"clusterName": clusterName,
				}).Info("Directory-backed volume: other volumes exist on this export for this node, retaining node IP authorization")
				return &csi.ControllerUnpublishVolumeResponse{}, nil
			}

			csmlog.WithContext(ctx).WithFields(csmlog.Fields{
				"exportID":    exportID,
				"accessZone":  accessZone,
				"nodeID":      req.GetNodeId(),
				"clusterName": clusterName,
			}).Info("Directory-backed volume: last volume on this export for this node, removing node IP authorization")
			// Continue to standard IP removal logic below (lock is held through removal)
		}
	}
	if azNetwork != "" {
		ips, err := s.getIpsFromAZNetworkLabel(ctx, req.NodeId, azNetwork)
		if err != nil {
			csmlog.WithContext(ctx).Debugf("No matching IP(s) found from AZNetwork label %s", azNetwork)
			return nil, status.Error(codes.FailedPrecondition, GetMessageWithReqID(runID, "error %s", err.Error()))
		}
		csmlog.WithContext(ctx).Debugf("Using IPs %s from AZNetwork %s to remove from export", ips, azNetwork)

		if err := isiConfig.isiSvc.RemoveExportClientByIPsWithZone(ctx, exportID, accessZone, ips, *isiConfig.IgnoreUnresolvableHosts); err != nil {
			if strings.Contains(err.Error(), "No such file or directory") {
				_, delErr := s.DeleteVolume(ctx, &csi.DeleteVolumeRequest{VolumeId: req.VolumeId})
				if delErr != nil {
					return nil, delErr
				}
			} else {
				return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "error encountered when trying to remove clients %s from export %d with access zone %s on cluster %s, error %s", ips, exportID, accessZone, clusterName, err.Error()))
			}
		}
	} else {
		// AZNetwork is not set, use existing behavior or multi-NIC removal
		nodeID := req.GetNodeId()
		if nodeID == "" {
			return nil, status.Error(codes.InvalidArgument,
				GetMessageWithReqID(runID, "node ID is required"))
		}

		// Multi-NIC: if mode=multi and allowedNetworks configured, resolve all NFS IPs and remove them
		if s.opts.allowedNetworksMode == constants.AllowedNetworksModeMulti && len(s.opts.allowedNetworks) > 0 {
			multiIPs, ipErr := s.getIpsFromAllowedNetworks(ctx, nodeID)
			if ipErr != nil {
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"operation": "ControllerUnpublishVolume",
					"mode":      s.opts.allowedNetworksMode,
					"node_id":   nodeID,
					"success":   false,
				}).Warnf("multi-NIC: failed to resolve NFS IPs for unpublish, falling back to nodeID removal: %v", ipErr)
			} else if len(multiIPs) > 0 {
				csmlog.WithContext(ctx).Infof("multi-NIC: removing %d NFS IPs from export %d: %v", len(multiIPs), exportID, multiIPs)
				if err := isiConfig.isiSvc.RemoveExportClientByIPsWithZone(ctx, exportID, accessZone, multiIPs, *isiConfig.IgnoreUnresolvableHosts); err != nil {
					if strings.Contains(err.Error(), "No such file or directory") {
						_, delErr := s.DeleteVolume(ctx, &csi.DeleteVolumeRequest{VolumeId: req.VolumeId})
						if delErr != nil {
							return nil, delErr
						}
					} else {
						return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "error encountered when trying to remove clients %v from export %d with access zone %s on cluster %s, error %s", multiIPs, exportID, accessZone, clusterName, err.Error()))
					}
				}
				return &csi.ControllerUnpublishVolumeResponse{}, nil
			}
		}

		csmlog.WithContext(ctx).Debug("Removing export client by node ID")
		csmlog.WithContext(ctx).Debugf("ignoreUnresolvableHosts value is '%t', for clusterName '%s'", *isiConfig.IgnoreUnresolvableHosts, clusterName)

		if err := isiConfig.isiSvc.RemoveExportClientByIDWithZone(ctx, exportID, accessZone, nodeID, *isiConfig.IgnoreUnresolvableHosts); err != nil {
			if strings.Contains(err.Error(), "No such file or directory") {
				_, delErr := s.DeleteVolume(ctx, &csi.DeleteVolumeRequest{VolumeId: req.VolumeId})
				if delErr != nil {
					return nil, delErr
				}
			} else {
				return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "error encountered when trying to remove client %s from export %d with access zone %s on cluster %s, error %s", nodeID, exportID, accessZone, clusterName, err.Error()))
			}
		}
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

// hasOtherDirectoryBackedVolumesOnExport checks if a node has other directory-backed PVs
// (excluding the one being unpublished) that use the same shared export.
// Returns (true, nil) if other volumes exist, (false, nil) if none exist, or (false, err) on failure.
//
// This function uses VolumeAttachments to determine which volumes are attached to a node,
// which works correctly for all access modes (RWO, RWX, ROX). Using PV.Spec.NodeAffinity
// would fail for RWX/ROX volumes since Kubernetes does not set NodeAffinity for multi-node volumes.
func (s *service) hasOtherDirectoryBackedVolumesOnExport(ctx context.Context, nodeID string, exportID int, accessZone, excludeVolumeName string) (bool, error) {
	// Extract node name from nodeID (format: nodeName=X&fqdn=Y&ip=Z)
	nodeName, _, _, err := id.ParseNodeID(ctx, nodeID)
	if err != nil {
		return false, fmt.Errorf("failed to parse node ID: %w", err)
	}

	attachmentList, err := k8sListVolumeAttachmentsFunc(ctx, s.k8sclient, metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list VolumeAttachments: %w", err)
	}

	// Build a set of PV names attached to this node (excluding the one being unpublished)
	attachedPVs := make(map[string]bool)
	for _, attachment := range attachmentList.Items {
		// Check if attachment is for this node and is currently attached
		if attachment.Spec.NodeName != nodeName {
			continue
		}
		if !attachment.Status.Attached {
			continue
		}
		// Get PV name from attachment source
		if attachment.Spec.Source.PersistentVolumeName == nil {
			continue
		}
		pvName := *attachment.Spec.Source.PersistentVolumeName
		// Skip the volume being unpublished
		if pvName == excludeVolumeName {
			continue
		}
		attachedPVs[pvName] = true
	}

	// If no other volumes are attached to this node, return early
	if len(attachedPVs) == 0 {
		return false, nil
	}

	// Get each attached PV individually — O(PVs attached to this node), not O(all PVs in cluster).
	// This avoids a full cluster-wide PV list on every unpublish call.
	for pvName := range attachedPVs {
		pv, pvErr := k8sGetPersistentVolumeFunc(ctx, s.k8sclient, pvName, metav1.GetOptions{})
		if pvErr != nil {
			csmlog.WithContext(ctx).Debugf("Failed to get PV %s: %v", pvName, pvErr)
			continue
		}

		// Check if it's a CSI PowerScale volume
		if pv.Spec.CSI == nil || pv.Spec.CSI.Driver != constants.PluginName {
			continue
		}

		// Check if it's directory-backed
		if pv.Spec.CSI.VolumeAttributes["ProvisioningMode"] != "directory" {
			continue
		}

		// Parse export ID from volume handle
		_, pvExportID, pvAccessZone, _, parseErr := id.ParseNormalizedVolumeID(ctx, pv.Spec.CSI.VolumeHandle)
		if parseErr != nil {
			csmlog.WithContext(ctx).Debugf("Failed to parse volume ID %s: %v", pv.Spec.CSI.VolumeHandle, parseErr)
			continue
		}

		// Check if it's on the same export
		if pvExportID != exportID || pvAccessZone != accessZone {
			continue
		}

		// Only count PVs that are actively bound (not Released/Failed/Pending)
		if pv.Status.Phase != corev1.VolumeBound {
			continue
		}

		csmlog.WithContext(ctx).Debugf("Found other directory-backed volume %s on export %d for node %s", pv.Name, exportID, nodeName)
		return true, nil
	}

	return false, nil
}

// labelDirectoryBackedPV patches a PV with labels and annotations identifying it as directory-backed.
// This enables efficient label-selector queries in the future and satisfies FR-10 (Volume Metadata).
// The patch is idempotent — re-patching an already-labeled PV is a no-op from K8s perspective.
// Failures are logged as warnings and do not block the publish operation.
func (s *service) labelDirectoryBackedPV(ctx context.Context, pvName string, exportID int, accessZone, exportPath string) {
	if s.k8sclient == nil {
		return
	}
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				"powerscale.csi.dell.com/provisioning-mode": "directory",
				"powerscale.csi.dell.com/shared-export-id":  strconv.Itoa(exportID),
			},
			"annotations": map[string]string{
				"powerscale.csi.dell.com/access-zone":        accessZone,
				"powerscale.csi.dell.com/shared-export-path": exportPath,
			},
		},
	}
	patchBytes, err := jsonMarshalFunc(patch)
	if err != nil {
		csmlog.WithContext(ctx).Warnf("labelDirectoryBackedPV: failed to marshal patch for PV %s: %v", pvName, err)
		return
	}
	if _, err := s.k8sclient.CoreV1().PersistentVolumes().Patch(
		ctx, pvName, k8stypes.MergePatchType, patchBytes, metav1.PatchOptions{},
	); err != nil {
		csmlog.WithContext(ctx).Warnf("labelDirectoryBackedPV: failed to patch PV %s: %v", pvName, err)
	} else {
		csmlog.WithContext(ctx).Debugf("labelDirectoryBackedPV: successfully labeled PV %s with provisioning-mode=directory, shared-export-id=%d", pvName, exportID)
	}
}

// getIpsFromAZNetworkLabel retrieves the IP(s) from the AZNetwork label
// on a node. It searches for a node label that matches the given AZNetwork
// and returns the corresponding IP(s) if found.
//
// Parameters:
//
//	ctx (context.Context): The context for the function call
//	azNetwork (string): The AZNetwork to search for in the node labels. E.g. 10.0.0.0/24
//
// Returns:
//
//	[]string: The array of IP(s) associated with the matching AZNetwork label, or empty if not found
//	error: Any error that occurs during the function call
func (s *service) getIpsFromAZNetworkLabel(ctx context.Context, nodeID, azNetwork string) ([]string, error) {
	// Get node labels
	nodeName, _, _, err := id.ParseNodeID(ctx, nodeID)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get Node Name with error %v", err.Error())
		return nil, err
	}
	labels, err := getNodeLabelsWithNameFunc(s)(nodeName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to get Node Labels with error %v", err.Error())
		return nil, err
	}

	// Find the node label with IP that belongs to AZNetwork
	// Example: csi-isilon.dellemc.com/az-192.168.1.0-24-192.168.1.1
	pluginName := regexp.QuoteMeta(constants.PluginName)
	pattern := regexp.MustCompile(fmt.Sprintf("^%s\\/az-([0-9\\.]+)-([0-9]+)-([0-9\\.]+)$", pluginName))

	// Array of IPs that match the given AZNetwork
	ips := []string{}

	for key, value := range labels {
		// Found the node with IP that belongs to the AZNetwork
		if match := pattern.FindStringSubmatch(key); len(match) == 4 {
			csmlog.WithContext(ctx).Debugf("Key: %s, Value: %s\n", key, value)

			exportIP := match[3]
			csmlog.WithContext(ctx).Debugf("Export IP %s from node label", exportIP)

			if csiutils.IPInCIDR(exportIP, azNetwork) {
				ips = append(ips, exportIP)
			}
		}
	}

	if len(ips) > 0 {
		return ips, nil
	}
	return ips, fmt.Errorf("failed to match AZNetwork to get IPs for export %s", azNetwork)
}

// getIpsFromAllowedNetworks reads node labels and returns all IPs that match
// the allowedNetworks CIDRs. Follows the same label pattern as getIpsFromAZNetworkLabel.
func (s *service) getIpsFromAllowedNetworks(ctx context.Context, nodeID string) ([]string, error) {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"operation":   "getIpsFromAllowedNetworks",
			"duration_ms": duration.Milliseconds(),
			"node_id":     nodeID,
			"mode":        s.opts.allowedNetworksMode,
		}).Debugf("IP resolution completed in %dms", duration.Milliseconds())
	}()

	nodeName, _, _, err := id.ParseNodeID(ctx, nodeID)
	if err != nil {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"operation": "getIpsFromAllowedNetworks",
			"node_id":   nodeID,
			"mode":      s.opts.allowedNetworksMode,
			"success":   false,
		}).Errorf("failed to get Node Name with error %v", err.Error())
		return nil, err
	}
	labels, err := getNodeLabelsWithNameFunc(s)(nodeName)
	if err != nil {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"operation": "getIpsFromAllowedNetworks",
			"node_id":   nodeID,
			"mode":      s.opts.allowedNetworksMode,
			"success":   false,
		}).Errorf("failed to get Node Labels with error %v", err.Error())
		return nil, err
	}

	pluginName := regexp.QuoteMeta(constants.PluginName)
	// Pattern matches node labels in format: {pluginName}/az-{network}-{prefix}-{ip}
	// Example: csi-isilon.dellemc.com/az-123.123.1.0-24-123.123.1.42
	// Capture groups: [1]=network CIDR, [2]=prefix length, [3]=IP address
	pattern := regexp.MustCompile(fmt.Sprintf("^%s\\/az-([0-9\\.]+)-([0-9]+)-([0-9\\.]+)$", pluginName))

	var ips []string
	for key := range labels {
		if match := pattern.FindStringSubmatch(key); len(match) == 4 {
			exportIP := match[3]
			for _, allowedCIDR := range s.opts.allowedNetworks {
				if csiutils.IPInCIDR(exportIP, allowedCIDR) {
					ips = append(ips, exportIP)
					break
				}
			}
		}
	}

	if len(ips) > 0 {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"operation": "getIpsFromAllowedNetworks",
			"node_id":   nodeID,
			"mode":      s.opts.allowedNetworksMode,
			"ip_count":  len(ips),
			"success":   true,
		}).Infof("Multi-NIC: found %d IPs from allowedNetworks in node labels", len(ips))
		return ips, nil
	}
	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		"operation":        "getIpsFromAllowedNetworks",
		"node_id":          nodeID,
		"mode":             s.opts.allowedNetworksMode,
		"allowed_networks": s.opts.allowedNetworks,
		"success":          false,
	}).Warnf("no IPs in node labels match allowedNetworks %v", s.opts.allowedNetworks)
	return ips, fmt.Errorf("no IPs in node labels match allowedNetworks %v", s.opts.allowedNetworks)
}

func (s *service) GetCapacity(
	ctx context.Context,
	req *csi.GetCapacityRequest) (
	*csi.GetCapacityResponse, error,
) {
	var clusterName string
	params := req.GetParameters()

	logFields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", logFields["csi.requestid"])

	if _, ok := params[ClusterNameParam]; ok {
		if params[ClusterNameParam] == "" {
			clusterName = s.defaultIsiClusterName
		} else {
			clusterName = params[ClusterNameParam]
		}
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error %v", err.Error())
		return nil, err
	}

	logFields[clusterName] = clusterName

	csmlog.WithContext(ctx).Debugf("Cluster Name: %v", clusterName)

	if err := s.autoProbe(ctx, isiConfig); err != nil {
		csmlog.WithContext(ctx).Error("Failed to probe with error: " + err.Error())
		return nil, err
	}

	// pass the key(s) to rest api
	keyArray := []string{"ifs.bytes.avail"}

	stat, err := isiConfig.isiSvc.GetStatistics(ctx, keyArray)
	if err != nil || len(stat.StatsList) < 1 {
		return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "Could not retrieve capacity. %s", err.Error()))
	}
	if stat.StatsList[0].Error != "" {
		return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "Could not retrieve capacity. Data returned error %s", stat.StatsList[0].Error))
	}
	remainingCapInBytes := stat.StatsList[0].Value

	return &csi.GetCapacityResponse{
		AvailableCapacity: remainingCapInBytes,
	}, nil
}

func (s *service) ControllerGetCapabilities(
	_ context.Context,
	_ *csi.ControllerGetCapabilitiesRequest) (
	*csi.ControllerGetCapabilitiesResponse, error,
) {
	capabilities := []*csi.ControllerServiceCapability{
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_MODIFY_VOLUME,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_GET_CAPACITY,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
				},
			},
		},
	}

	healthMonitorCapabilities := []*csi.ControllerServiceCapability{
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_VOLUME_CONDITION,
				},
			},
		},
		{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: csi.ControllerServiceCapability_RPC_GET_VOLUME,
				},
			},
		},
	}

	if s.opts.IsHealthMonitorEnabled {
		capabilities = append(capabilities, healthMonitorCapabilities...)
	}

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: capabilities,
	}, nil
}

func (s *service) controllerProbe(ctx context.Context, clusterConfig *IsilonClusterConfig) error {
	if err := s.validateOptsParameters(clusterConfig); err != nil {
		return fmt.Errorf("controller probe failed : '%v'", err)
	}

	if clusterConfig.isiSvc == nil {
		var err error
		clusterConfig.isiSvc, err = s.GetIsiService(ctx, clusterConfig, csmlog.GetLevel())
		if clusterConfig.isiSvc == nil {
			return errors.New("clusterConfig.isiSvc (type isiService) is nil, probe failed")
		}
		if err != nil {
			return err
		}
	}

	if err := clusterConfig.isiSvc.TestConnection(ctx); err != nil {
		return fmt.Errorf("controller probe failed : '%v'", err)
	}

	csmlog.WithContext(ctx).Debug("controller probe succeeded")

	return nil
}

// CreateSnapshot creates a snapshot.
// If Parameters["VolumeIDList"] has a comma separated list of additional volumes, they will be
// snapshotted in a consistency group with the primary volume in CreateSnapshotRequest.SourceVolumeId.
func (s *service) CreateSnapshot(
	ctx context.Context,
	req *csi.CreateSnapshotRequest) (
	*csi.CreateSnapshotResponse, error,
) {
	logFields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", logFields["csi.requestid"])

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		csmlog.FieldComponent: "controller",
		csmlog.FieldOperation: "CreateSnapshot",
		csmlog.FieldProtocol:  "NFS",
		csmlog.FieldVolumeID:  req.GetSourceVolumeId(),
	}).Info("CreateSnapshot called")

	// parse the input volume id and fetch it's components
	_, exportID, accessZone, clusterName, err := id.ParseNormalizedVolumeID(ctx, req.GetSourceVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, " ReqID=%s %s", runID, err.Error())
	}

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error %v", err.Error())
		return nil, err
	}

	logFields[clusterName] = clusterName

	csmlog.WithContext(ctx).Debugf("Cluster Name: %v", clusterName)

	// auto probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, " ReqID=%s %s", runID, err.Error())
	}

	var (
		snapshotNew isi.Snapshot
		isiPath     string
	)

	// Get export for the source volume
	export, err := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	if len(*export.Paths) == 0 {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("can't find paths for export with ID %d", exportID))
	}
	exportPath := (*export.Paths)[0]

	// Detect if source volume is directory-backed by parsing volume ID mode
	srcVolumeName, _, _, _, srcProvisioningMode, parseErr := id.ParseVolumeIDWithMode(ctx, req.GetSourceVolumeId())
	isDirectoryBacked := (parseErr == nil && srcProvisioningMode == id.ProvisioningModeDirectory)

	// For directory-backed volumes, exportPath is the shared export (e.g., /ifs/k8s/shared)
	// For export-backed volumes, exportPath contains the volume name (e.g., /ifs/data/pvc-uuid)
	if isDirectoryBacked {
		// Directory-backed: use exportPath directly as the base path
		// The volume is a subdirectory under the shared export
		isiPath = exportPath
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"exportPath":       exportPath,
			"srcVolumeName":    srcVolumeName,
			"provisioningMode": "directory",
			"sharedExportPath": isiPath,
		}).Debug("CreateSnapshot: detected directory-backed source volume")
	} else {
		// Export-backed: extract parent directory from export path
		isiPath = isilonfs.GetIsiPathFromExportPath(exportPath)
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"exportPath":       exportPath,
			"srcVolumeName":    srcVolumeName,
			"provisioningMode": "export",
			"isiPath":          isiPath,
		}).Debug("CreateSnapshot: detected export-backed source volume")
	}

	// validate request and get details of the request
	// srcVolumeID: source volume ID
	// snapshotName: name of the snapshot that need to be created
	srcVolumeID, snapshotName, err := s.validateCreateSnapshotRequest(ctx, req, isiPath, isiConfig)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, " ReqID=%s %s", runID, err.Error())
	}

	csmlog.WithContext(ctx).Infof("snapshot name is '%s' and source volume ID is '%s' access Zone is '%s'", snapshotName, srcVolumeID, accessZone)
	// check if snapshot already exists
	var snapshotByName isi.Snapshot
	csmlog.WithContext(ctx).Infof("check for existence of snapshot '%s'", snapshotName)
	if snapshotByName, err = isiConfig.isiSvc.GetSnapshot(ctx, snapshotName); snapshotByName != nil {
		if fPath.Base(snapshotByName.Path) == srcVolumeID {
			// return the existent snapshot
			return s.getCreateSnapshotResponse(ctx, strconv.FormatInt(snapshotByName.ID, 10), req.GetSourceVolumeId(), snapshotByName.Created, isiConfig.isiSvc.GetSnapshotSize(ctx, isiPath, snapshotName, accessZone), clusterName, accessZone), nil
		}
		// return already exists error
		return nil, status.Error(codes.AlreadyExists,
			GetMessageWithReqID(runID, "a snapshot with name '%s' already exists but is "+
				"incompatible with the specified source volume id '%s'", snapshotName, req.GetSourceVolumeId()))
	}

	// create new snapshot for source direcory
	if err = validateSnapshotIQLicense(ctx, isiConfig); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, " ReqID=%s %s", runID, err.Error())
	}
	// Construct the full volume path for snapshot creation
	// For directory-backed: isiPath is the shared export path, srcVolumeID is the directory name
	// For export-backed: isiPath is the parent directory, srcVolumeID is the volume name
	path := isilonfs.GetPathForVolume(isiPath, srcVolumeID)

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		"snapshotPath":    path,
		"isiPath":         isiPath,
		"srcVolumeID":     srcVolumeID,
		"directoryBacked": isDirectoryBacked,
	}).Info("CreateSnapshot: creating snapshot at path")

	if snapshotNew, err = isiConfig.isiSvc.CreateSnapshot(ctx, path, snapshotName); err != nil {
		return nil, status.Errorf(codes.Internal, " ReqID=%s %s", runID, err.Error())
	}
	_, _ = isiConfig.isiSvc.GetSnapshot(ctx, snapshotName)

	csmlog.WithContext(ctx).Infof("snapshot creation is successful")
	// return the response
	return s.getCreateSnapshotResponse(ctx, strconv.FormatInt(snapshotNew.ID, 10), req.GetSourceVolumeId(), snapshotNew.Created, isiConfig.isiSvc.GetSnapshotSize(ctx, isiPath, snapshotName, accessZone), clusterName, accessZone), nil
}

// validateCreateSnapshotRequest validate the input params in CreateSnapshotRequest
func (s *service) validateCreateSnapshotRequest(
	ctx context.Context,
	req *csi.CreateSnapshotRequest, isiPath string, isiConfig *IsilonClusterConfig,
) (string, string, error) {
	logFields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", logFields["csi.requestid"])

	srcVolumeID, _, _, clusterName, err := id.ParseNormalizedVolumeID(ctx, req.GetSourceVolumeId())
	if err != nil {
		return "", "", status.Errorf(codes.InvalidArgument, " ReqID=%s %s", runID, err.Error())
	}

	logFields[clusterName] = clusterName

	csmlog.WithContext(ctx).Debugf("Cluster Name: %v", clusterName)

	if !isiConfig.isiSvc.IsVolumeExistent(ctx, isiPath, srcVolumeID, "") {
		return "", "", status.Error(codes.InvalidArgument,
			GetMessageWithReqID(runID, "source volume id is invalid"))
	}

	snapshotName := req.GetName()
	if snapshotName == "" {
		return "", "", status.Error(codes.InvalidArgument,
			GetMessageWithReqID(runID, "name cannot be empty"))
	}

	return srcVolumeID, snapshotName, nil
}

var getUtilsGetNormalizedSnapshotID = id.GetNormalizedSnapshotID

func (s *service) getCreateSnapshotResponse(ctx context.Context, snapshotID string, sourceVolumeID string, creationTime, sizeInBytes int64, clusterName string, accessZone string) *csi.CreateSnapshotResponse {
	snapID := getUtilsGetNormalizedSnapshotID(ctx, snapshotID, clusterName, accessZone)
	return &csi.CreateSnapshotResponse{
		Snapshot: s.getCSISnapshot(snapID, sourceVolumeID, creationTime, sizeInBytes),
	}
}

func (s *service) getCSISnapshot(snapshotID string, sourceVolumeID string, creationTime, sizeInBytes int64) *csi.Snapshot {
	ts := &timestamppb.Timestamp{
		Seconds: creationTime,
	}

	vi := &csi.Snapshot{
		SizeBytes:      sizeInBytes,
		SnapshotId:     snapshotID,
		SourceVolumeId: sourceVolumeID,
		CreationTime:   ts,
		ReadyToUse:     true,
	}

	return vi
}

func (s *service) DeleteSnapshot(
	ctx context.Context,
	req *csi.DeleteSnapshotRequest) (
	*csi.DeleteSnapshotResponse, error,
) {
	fields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", fields["csi.requestid"])

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		csmlog.FieldComponent: "controller",
		csmlog.FieldOperation: "DeleteSnapshot",
		csmlog.FieldProtocol:  "NFS",
	}).Info("DeleteSnapshot called")
	if req.GetSnapshotId() == "" {
		return nil, status.Error(codes.InvalidArgument, GetMessageWithReqID(runID, "snapshot id to be deleted is required"))
	}
	// parse the input snapshot id and fetch it's components
	snapshotID, clusterName, accessZone, err := id.ParseNormalizedSnapshotID(ctx, req.GetSnapshotId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to parse snapshot ID %s, error : %v", req.GetSnapshotId(), err))
	}
	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error %v ", err.Error())
		return nil, err
	}

	fields[clusterName] = clusterName

	csmlog.WithContext(ctx).Debugf("Cluster Name: %v", clusterName)

	if err := s.autoProbe(ctx, isiConfig); err != nil {
		csmlog.WithContext(ctx).Error("Failed to probe with error: " + err.Error())
		return nil, err
	}

	id, err := strconv.ParseInt(snapshotID, 10, 64)
	if err != nil {
		csmlog.WithContext(ctx).Warnf("snapshot ID '%s' is not a valid integer", snapshotID)
		return &csi.DeleteSnapshotResponse{}, nil
	}
	snapshot, err := isiConfig.isiSvc.GetSnapshot(ctx, snapshotID)
	// Idempotency check
	if err != nil {
		jsonError, ok := err.(*isiApi.JSONError)
		if !ok {
			csmlog.WithContext(ctx).Error("type casting from error to JSONError failed, attempting to determine the error by parsing the error msg instead of the status code")
			// Check the error message if failed to convert the error to JSONError
			if snapshot == nil && strings.Contains(err.Error(), "not found") {
				return &csi.DeleteSnapshotResponse{}, nil
			}
			// Internal server error if the error is not about "not found"
			return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "cannot check the existence of the snapshot: %s", err.Error()))
		}

		if jsonError.StatusCode == 404 {
			return &csi.DeleteSnapshotResponse{}, nil
		}
		return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "cannot check the existence of the snapshot: %s", err.Error()))
	}

	// Get snapshot path
	snapshotSourceVolumeIsiPath, _ := isiConfig.isiSvc.GetSnapshotSourceVolumeIsiPath(ctx, snapshotID)
	csmlog.WithContext(ctx).Infof("Snapshot source volume isiPath is '%s'", snapshotSourceVolumeIsiPath)
	snapshotIsiPath, err := isiConfig.isiSvc.GetSnapshotIsiPath(ctx, snapshotSourceVolumeIsiPath, snapshotID, accessZone)
	if err != nil {
		return nil, status.Errorf(codes.Internal, " ReqID%s error %s", runID, err.Error())
	}
	csmlog.WithContext(ctx).Debugf("The Isilon directory path of snapshot is= %v", snapshotIsiPath)

	export, err := isiConfig.isiSvc.GetExportWithPathAndZone(ctx, snapshotIsiPath, accessZone)
	if err != nil {
		// internal error
		return nil, err
	}

	deleteSnapshot := true
	// Check if there are any RO volumes created from this snapshot
	// Note: This is true only for RO volumes from snapshots
	if export != nil {
		if err := s.processSnapshotTrackingDirectoryDuringDeleteSnapshot(ctx, export, snapshotIsiPath, accessZone, &deleteSnapshot, isiConfig); err != nil {
			csmlog.WithContext(ctx).Errorf("Failed to get RO volume from snapshot %v ", err.Error())
			return nil, err
		}
	}

	if deleteSnapshot {
		err = isiConfig.isiSvc.DeleteSnapshot(ctx, id, "")
		if err != nil {
			// Check for dependency errors (writable snapshot volumes depend on this snapshot)
			if isDependencyError(err) {
				depMsg := fmt.Sprintf("Snapshot '%s' has dependent writable volumes and cannot be deleted. Delete dependent volumes first.", snapshotID)
				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					csmlog.FieldComponent: "controller",
					csmlog.FieldOperation: "DeleteSnapshot",
					"reason":              "SnapshotHasDependents",
				}).Warn(depMsg)

				// Emit Kubernetes Event for snapshot dependency errors
				s.emitSnapshotDependencyEvent(ctx, snapshotID, depMsg)

				return nil, status.Error(codes.FailedPrecondition, GetMessageWithReqID(runID, "%s", depMsg))
			}
			return nil, status.Error(codes.Internal, GetMessageWithReqID(runID, "error deleting snapshot: %s", err.Error()))
		}
	}
	csmlog.WithContext(ctx).Infof("Snapshot with id '%s' deleted", snapshotID)
	return &csi.DeleteSnapshotResponse{}, nil
}

func (s *service) processSnapshotTrackingDirectoryDuringDeleteSnapshot(
	ctx context.Context,
	export isi.Export,
	snapshotIsiPath,
	accessZone string,
	deleteSnapshot *bool,
	isiConfig *IsilonClusterConfig,
) error {
	// get Zone details
	zone, err := isiConfig.isiSvc.GetZoneByName(ctx, accessZone)
	if err != nil {
		return err
	}

	// Populate names for snapshot's tracking dir and snapshot delete marker
	isiPath, snapshotName, _ := isiConfig.isiSvc.GetSnapshotIsiPathComponents(snapshotIsiPath, zone.Path)
	snapshotTrackingDir := isiConfig.isiSvc.GetSnapshotTrackingDirName(snapshotName)
	snapshotTrackingDirDeleteMarker := fPath.Join(snapshotTrackingDir, DeleteSnapshotMarker)

	// Check if the snapshot tracking dir is present (this indicates
	// there were some RO volumes created from this snapshot)
	// Get subdirectories count of snapshot tracking dir.
	// Every directory will have two subdirectory entries . and ..
	totalSubDirectories, _ := isiConfig.isiSvc.GetSubDirectoryCount(ctx, isiPath, snapshotTrackingDir)

	// There are no more volumes present which were created using this snapshot
	// Every directory will have two subdirectories . and ..
	if totalSubDirectories == IgnoreDotAndDotDotSubDirs || totalSubDirectories == 0 {
		if err := isiConfig.isiSvc.UnexportByIDWithZone(ctx, export.ID, accessZone); err != nil {
			return err
		}

		// Delete snapshot tracking directory
		if err := isiConfig.isiSvc.DeleteVolume(ctx, isiPath, snapshotTrackingDir); err != nil {
			csmlog.WithContext(ctx).Errorf("error while deleting snapshot tracking directory '%s'", fPath.Join(isiPath, snapshotTrackingDir))
		}
	} else {
		*deleteSnapshot = false
		// Set a marker in snapshot tracking dir to delete snapshot, once
		// all the volumes created from this snapshot were deleted
		csmlog.WithContext(ctx).Debugf("set DeleteSnapshotMarker marker in snapshot tracking dir")
		if err := isiConfig.isiSvc.CreateVolume(ctx, isiPath, snapshotTrackingDirDeleteMarker, isiConfig.IsiVolumePathPermissions); err != nil {
			return err
		}
	}

	return nil
}

// Validate volume capabilities
func validateVolumeCaps(
	vcs []*csi.VolumeCapability,
	_ isi.Volume,
) (bool, string) {
	var (
		supported = true
		reason    string
	)
	// Check that all access types are valid
	if !checkValidAccessTypes(vcs) {
		return false, errUnknownAccessType
	}

	for _, vc := range vcs {
		am := vc.GetAccessMode()
		if am == nil {
			continue
		}
		switch am.Mode {
		case csi.VolumeCapability_AccessMode_UNKNOWN:
			supported = false
			reason = errUnknownAccessMode
		case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:
		case csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER:
		case csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER:
		case csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY:
			supported = false
			reason = errNoSingleNodeReader
		case csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY:
		case csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER:
			supported = false
			reason = errNoMultiNodeSingleWriter
		case csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER:
		default:
			// This is to guard against new access modes not understood
			supported = false
			reason = errUnknownAccessMode
		}
	}

	return supported, reason
}

func checkValidAccessTypes(vcs []*csi.VolumeCapability) bool {
	for _, vc := range vcs {
		if vc == nil {
			continue
		}
		atmount := vc.GetMount()
		if atmount != nil {
			continue
		}
		// Unknown access type, we should reject it.
		return false
	}
	return true
}

func addMetaData(params map[string]string) map[string]string {
	// CSI specific metadata header for authorization
	headerMetadata := make(map[string]string)
	if _, ok := params[csiPersistentVolumeName]; ok {
		headerMetadata[headerPersistentVolumeName] = params[csiPersistentVolumeName]
	}

	if _, ok := params[csiPersistentVolumeClaimName]; ok {
		headerMetadata[headerPersistentVolumeClaimName] = params[csiPersistentVolumeClaimName]
	}

	if _, ok := params[csiPersistentVolumeClaimNamespace]; ok {
		headerMetadata[headerPersistentVolumeClaimNamespace] = params[csiPersistentVolumeClaimNamespace]
	}
	return headerMetadata
}

func isCSIManagedVolume(ctx context.Context, isiConfig *IsilonClusterConfig, fullPath string, export isi.Export) bool {
	if quotaID, quotaErr := isilonfs.GetQuotaIDFromDescription(ctx, export); quotaErr == nil && quotaID != "" {
		csmlog.WithContext(ctx).Debugf("volume '%s' identified as CSI-managed via CSI-tagged export description", fullPath)
		return true
	}

	if exportHasDummyHostClient(ctx, export) {
		csmlog.WithContext(ctx).Debugf("volume '%s' identified as CSI-managed via dummy localhost export client", fullPath)
		return true
	}

	metadata, metaErr := isiConfig.isiSvc.GetVolumeMetaData(ctx, fullPath)
	if metaErr == nil && metadata != nil {
		_, hasPVName := metadata[headerPersistentVolumeName]
		_, hasPVCName := metadata[headerPersistentVolumeClaimName]
		_, hasPVCNamespace := metadata[headerPersistentVolumeClaimNamespace]
		if hasPVName || hasPVCName || hasPVCNamespace {
			csmlog.WithContext(ctx).Debugf("volume '%s' identified as CSI-managed via metadata headers", fullPath)
			return true
		}
	} else if metaErr != nil {
		csmlog.WithContext(ctx).Debugf("could not query metadata for path '%s': %v", fullPath, metaErr)
	}

	csmlog.WithContext(ctx).Debugf("volume '%s' does not have CSI fingerprints; skipping", fullPath)

	return false
}

func exportHasDummyHostClient(ctx context.Context, export isi.Export) bool {
	if export == nil {
		return false
	}

	clientName, clientFQDN, clientIP, err := id.ParseNodeID(ctx, id.DummyHostNodeID)
	if err != nil {
		csmlog.WithContext(ctx).Debugf("failed to parse dummy host node ID '%s': %v", id.DummyHostNodeID, err)
		return false
	}

	clients := []string{}
	readOnlyClients := []string{}
	readWriteClients := []string{}
	rootClients := []string{}
	if export.Clients != nil {
		clients = *export.Clients
	}
	if export.ReadOnlyClients != nil {
		readOnlyClients = *export.ReadOnlyClients
	}
	if export.ReadWriteClients != nil {
		readWriteClients = *export.ReadWriteClients
	}
	if export.RootClients != nil {
		rootClients = *export.RootClients
	}

	if strutil.IsStringInSlices(clientName, clients, readOnlyClients, readWriteClients, rootClients) {
		return true
	}
	if strutil.IsStringInSlices(clientFQDN, clients, readOnlyClients, readWriteClients, rootClients) {
		return true
	}
	if clientIP != "" && strutil.IsStringInSlices(clientIP, clients, readOnlyClients, readWriteClients, rootClients) {
		return true
	}

	return false
}

func (s *service) ControllerGetVolume(ctx context.Context,
	req *csi.ControllerGetVolumeRequest,
) (*csi.ControllerGetVolumeResponse, error) {
	logFields := csmlog.ExtractFieldsFromContext(ctx)
	runID := fmt.Sprintf("%v", logFields["csi.requestid"])

	abnormal := false
	message := ""
	var volume isi.Volume

	volID := req.GetVolumeId()
	if volID == "" {
		return nil, status.Error(codes.FailedPrecondition, GetMessageWithReqID(runID, "no VolumeID found in request"))
	}

	volName, exportID, accessZone, clusterName, err := id.ParseNormalizedVolumeID(ctx, volID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, " ReqID=%s error %s", runID, err.Error())
	}

	logFields[clusterName] = clusterName

	csmlog.WithContext(ctx).Debugf("Cluster Name: %v", clusterName)

	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error %v", err.Error())
		return nil, err
	}

	isiPath := isiConfig.IsiPath

	isiPathFromParams, err := s.validateIsiPath(ctx, volName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed get isiPath %v", err.Error())
	}

	if isiPathFromParams != isiPath && isiPathFromParams != "" {
		csmlog.WithContext(ctx).Debugf("overriding isiPath with value from StorageClass %v", isiPathFromParams)
		isiPath = isiPathFromParams
	}

	if err := s.autoProbe(ctx, isiConfig); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, " ReqID=%s error %s", runID, err.Error())
	}

	// check if volume exists
	if !isiConfig.isiSvc.IsVolumeExistent(ctx, isiPath, volName, "") {
		abnormal = true
		message = fmt.Sprintf("volume does not exists at this path %v", isiPath)
	}

	// Fetch volume details
	if !abnormal {
		volume, err = isiConfig.isiSvc.GetVolumeWithIsiPath(ctx, isiPath, "", volName)
		if err != nil {
			abnormal = true
			message = fmt.Sprintf("error in getting '%s' volume '%v'", volName, err)
		}
	}

	if abnormal {
		return &csi.ControllerGetVolumeResponse{
			Volume: nil,
			Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: abnormal,
					Message:  message,
				},
			},
		}, nil
	}

	// Fetch export clients list
	exports, err := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone)
	if err != nil {
		return &csi.ControllerGetVolumeResponse{
			Volume: &csi.Volume{
				VolumeId: volume.Name,
			},
			Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
				PublishedNodeIds: nil,
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: true,
					Message:  "unable to fetch export list",
				},
			},
		}, nil
	}

	// remove localhost from the clients
	exportList := removeString(*exports.Clients, "localhost")
	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			VolumeId: volume.Name,
		},
		Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
			PublishedNodeIds: exportList,
			VolumeCondition: &csi.VolumeCondition{
				Abnormal: abnormal,
				Message:  "Volume is healthy",
			},
		},
	}, nil
}

// Map of supported mutable parameters for ControllerModifyVolume
var supportedMutableParams = map[string]bool{
	"AdvisoryLimit": true,
	"SoftLimit":     true,
	"SoftGracePrd":  true,
}

// validate the mutable parameter keys
func validateMutableParamKeys(mutableParams map[string]string) error {
	for key, value := range mutableParams {
		// Check if parameter is supported
		if !supportedMutableParams[key] {
			return status.Errorf(codes.InvalidArgument,
				"unsupported mutable parameter '%s'; supported mutable parameters are: AdvisoryLimit, SoftLimit, SoftGracePrd",
				key)
		}
		// Convert and validate parameter value
		intValue, convErr := strconv.ParseInt(value, 10, 64)
		if convErr != nil {
			return status.Errorf(codes.InvalidArgument,
				"mutable parameter '%s' must be a valid integer, got '%s'",
				key, value)
		}
		// Validate non-negative
		if intValue < 0 {
			return status.Errorf(codes.InvalidArgument,
				"mutable parameter '%s' must be a non-negative integer, got '%d'",
				key, intValue)
		}
	}
	return nil
}

// ControllerModifyVolume modifies a volume's mutable properties (CSI 1.12)
func (s *service) ControllerModifyVolume(
	ctx context.Context,
	req *csi.ControllerModifyVolumeRequest,
) (*csi.ControllerModifyVolumeResponse, error) {
	startTime := time.Now()
	var err error
	defer func() {
		log := csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			csmlog.FieldComponent: "controller",
			csmlog.FieldOperation: "ControllerModifyVolume",
		}).TrackDuration(startTime)
		if err != nil {
			log.Infof("ControllerModifyVolume Failed with error: %v", err)
		} else {
			log.Info("ControllerModifyVolume Successful")
		}
	}()

	csiVolID := req.GetVolumeId()
	if csiVolID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume_id is required")
	}

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		csmlog.FieldComponent: "controller",
		csmlog.FieldOperation: "ControllerModifyVolume",
		csmlog.FieldVolumeID:  csiVolID,
	}).Info("ControllerModifyVolume called")

	// Parse volume ID
	volName, exportID, accessZone, clusterName, err := id.ParseNormalizedVolumeID(ctx, csiVolID)
	if err != nil {
		err = status.Errorf(codes.NotFound, "invalid volume ID format: %s", err.Error())
		return nil, err
	}

	// Get Isilon config
	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to get Isilon config with error %v", err.Error())
		return nil, err
	}

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		csmlog.FieldOperation: "ControllerModifyVolume",
		csmlog.FieldArrayID:   isiConfig.Endpoint,
	}).Debugf("Cluster Name: %v", clusterName)

	// Auto probe
	if err := s.autoProbe(ctx, isiConfig); err != nil {
		err = status.Error(codes.FailedPrecondition, err.Error())
		return nil, err
	}

	// Check if quota is enabled on the volume
	if !s.opts.QuotaEnabled {
		err = status.Error(codes.FailedPrecondition, "quota not enabled for volume")
		return nil, err
	}

	// Detect directory-backed mode using volume ID provisioning mode
	isiPath := isiConfig.IsiPath
	_, _, _, _, provisioningMode, parseErr := id.ParseVolumeIDWithMode(ctx, csiVolID)
	directoryBacked := (provisioningMode == id.ProvisioningModeDirectory)

	// Get export to determine the actual export path for fallback detection
	export, err := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone)
	if err != nil {
		err = status.Errorf(codes.NotFound, "export '%d' not found in access zone '%s': %v", exportID, accessZone, err)
		return nil, err
	}

	exportPath := ""
	if export.Paths != nil && len(*export.Paths) > 0 {
		exportPath = (*export.Paths)[0]
	}

	detectionMethod := "volumeID"
	if parseErr != nil || provisioningMode == "" {
		// Fallback: For directory-backed volumes, the export path is the shared export (parent directory)
		// For export-backed volumes, the export path contains the volume name as the last component
		expectedVolumePath := isilonfs.GetPathForVolume(isiPath, volName)
		directoryBacked = (exportPath != expectedVolumePath)
		detectionMethod = "pathComparison"
		if parseErr != nil {
			csmlog.WithContext(ctx).Debugf("Failed to parse volume ID with mode: %v, using path-based detection", parseErr)
		}
	}

	if directoryBacked {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"volumeName":       volName,
			"exportPath":       exportPath,
			"provisioningMode": provisioningMode,
			"detectionMethod":  detectionMethod,
		}).Info("Directory-backed volume detected in ControllerModifyVolume")
	}

	// Get the current quota: directory-backed uses path-based lookup, export-backed uses export description
	var quota isi.Quota
	if directoryBacked {
		// For directory-backed volumes, the quota is on the subdirectory, not the shared export
		volumePath := isilonfs.GetPathForVolume(exportPath, volName)
		csmlog.WithContext(ctx).Debugf("attempting to get quota for directory-backed volume at path '%s'", volumePath)
		quota, err = isiConfig.isiSvc.GetQuotaByPath(ctx, volumePath)
	} else {
		quota, err = isiConfig.isiSvc.GetVolumeQuota(ctx, volName, exportID, accessZone)
	}

	if err != nil {
		err = status.Errorf(codes.NotFound, "volume not found or quota does not exist: %s", err.Error())
		return nil, err
	}

	if quota == nil {
		err = status.Errorf(codes.NotFound, "volume not found, quota is nil for volume: %s", volName)
		return nil, err
	}

	// Validate mutable_parameters is not empty
	if len(req.GetMutableParameters()) == 0 {
		csmlog.WithContext(ctx).Infof("ControllerModifyVolume: no mutable parameters provided for volume %s, returning success", csiVolID)
		return &csi.ControllerModifyVolumeResponse{}, nil
	}

	// Validate and convert mutable parameters
	params := make(map[string]interface{})
	reqMutableParams := req.GetMutableParameters()

	if err := validateMutableParamKeys(reqMutableParams); err != nil {
		return nil, err
	}

	for key, value := range reqMutableParams {
		// Convert parameter value
		intValue, _ := strconv.ParseInt(value, 10, 64)

		// Convert limits from %age to values
		var convertedLimit int64
		if key == "SoftLimit" || key == "AdvisoryLimit" {
			if intValue > 100 {
				err = status.Error(codes.InvalidArgument, fmt.Sprintf("mutable parameter %s must be between 0 and 100, got %d", key, intValue))
				return nil, err
			}
			if quota.Thresholds.Hard == 0 {
				err = status.Errorf(codes.FailedPrecondition, "quota hard limit is 0 for volume %s; cannot convert %s from percentage", csiVolID, key)
				return nil, err
			}
			convertedLimit = (intValue * quota.Thresholds.Hard) / 100
			params[key] = convertedLimit
		} else {
			// Update intValue for SoftGracePrd
			params["SoftGracePrd"] = intValue
		}
	}

	// Idempotency check: compare requested values with current values
	needsUpdate := false
	if advisoryLimit, ok := params["AdvisoryLimit"]; ok {
		if advisoryLimit != quota.Thresholds.Advisory {
			needsUpdate = true
		}
	}
	if softLimit, ok := params["SoftLimit"]; ok {
		if softLimit != quota.Thresholds.Soft {
			needsUpdate = true
		}
	}
	if softGrace, ok := params["SoftGracePrd"]; ok {
		if softGrace != quota.Thresholds.SoftGrace {
			needsUpdate = true
		}
	}

	// If no update is needed (idempotent), return success immediately
	if !needsUpdate {
		csmlog.WithContext(ctx).Info("ControllerModifyVolume: no changes needed (idempotent)")
		return &csi.ControllerModifyVolumeResponse{}, nil
	}

	// Modify the quota using isiService
	if err = isiConfig.isiSvc.ModifyQuota(ctx, quota.ID, params); err != nil {
		err = status.Errorf(codes.Internal, "failed to modify volume quota: %s", err.Error())
		return nil, err
	}
	csmlog.WithContext(ctx).Infof("ControllerModifyVolume: quota modified successfully for volume: %s with parameters: %v", volName, params)

	return &csi.ControllerModifyVolumeResponse{}, nil
}

func removeString(exportList []string, strToRemove string) []string {
	for index, export := range exportList {
		if export == strToRemove {
			return append(exportList[:index], exportList[index+1:]...)
		}
	}
	return exportList
}

// isDependencyError checks whether an error from OneFS indicates a snapshot dependency conflict
// (e.g., HTTP 409 Conflict when writable snapshot volumes depend on the snapshot being deleted).
func isDependencyError(err error) bool {
	if err == nil {
		return false
	}
	// Check for JSON API error with 409 status code
	if jsonErr, ok := err.(*isiApi.JSONError); ok {
		if jsonErr.StatusCode == http.StatusConflict {
			return true
		}
	}
	// Check error message for specific dependency indicators
	// More specific than generic "conflict" to avoid false positives
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "has dependent") ||
		strings.Contains(errMsg, "dependency") ||
		(strings.Contains(errMsg, "conflict") && (strings.Contains(errMsg, "snapshot") || strings.Contains(errMsg, "writable")))
}

// emitSnapshotDependencyEvent emits a Kubernetes Event when snapshot deletion is blocked due to dependencies.
// Event type Warning, reason SnapshotHasDependents.
func (s *service) emitSnapshotDependencyEvent(ctx context.Context, snapshotID, message string) {
	if s.k8sclient == nil {
		csmlog.WithContext(ctx).Debug("Cannot emit Kubernetes Event: k8sclient is nil")
		return
	}

	// Get namespace from environment variables
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = os.Getenv("X_CSI_DRIVER_NAMESPACE")
	}
	if namespace == "" {
		csmlog.WithContext(ctx).Debug("Cannot emit Kubernetes Event: namespace not set in environment variables")
		return
	}

	// Parse snapshot ID to get the volume snapshot name
	// Snapshot IDs are in format: <snapshot-id>=<cluster>=<access-zone>
	// We'll use the snapshot ID as the involved object name
	snapshotName := snapshotID
	if parts := strings.Split(snapshotID, "="); len(parts) > 0 {
		snapshotName = parts[0]
	}

	// Create the Event object
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("snapshot-%s-%d", snapshotName, time.Now().Unix()),
			Namespace: namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "VolumeSnapshot",
			Name:      snapshotName,
			Namespace: namespace,
		},
		Reason:  "SnapshotHasDependents",
		Message: message,
		Type:    corev1.EventTypeWarning,
		Source: corev1.EventSource{
			Component: constants.PluginName,
		},
		FirstTimestamp: metav1.NewTime(time.Now()),
		LastTimestamp:  metav1.NewTime(time.Now()),
		Count:          1,
	}

	// Attempt to create the event
	_, err := s.k8sclient.CoreV1().Events(namespace).Create(ctx, event, metav1.CreateOptions{})
	if err != nil {
		csmlog.WithContext(ctx).Warnf("Failed to emit Kubernetes Event for snapshot dependency: %v", err)
	} else {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"snapshot_id": snapshotID,
			"event_type":  corev1.EventTypeWarning,
			"reason":      "SnapshotHasDependents",
		}).Info("Emitted Kubernetes Event for snapshot dependency")
	}
}
