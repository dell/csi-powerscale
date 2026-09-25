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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
	"github.com/Ecosystems/container-storage-modules/src/gofsutil"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func publishVolume(
	ctx context.Context,
	req *csi.NodePublishVolumeRequest,
	nfsExportURL string,
) error {
	return publishVolumeFunc(ctx, req, nfsExportURL)
}

var (
	getGetMountsFunc = func() func(ctx context.Context) ([]gofsutil.Info, error) {
		return gofsutil.GetMounts
	}

	getMountFunc = func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
		return gofsutil.Mount
	}

	getUnmountFunc = func() func(ctx context.Context, target string) error {
		return gofsutil.Unmount
	}

	getOsRemoveAllFunc = func() func(name string) error {
		return os.RemoveAll
	}

	recordAuthFailureFunc = func() {
		// Default no-op; will be set by service if metrics enabled
	}

	publishVolumeFunc = func(
		ctx context.Context,
		req *csi.NodePublishVolumeRequest,
		nfsExportURL string,
	) error {
		volCap := req.GetVolumeCapability()
		if volCap == nil {
			return status.Error(codes.InvalidArgument,
				"Volume Capability is required")
		}

		accMode := volCap.GetAccessMode()
		if accMode == nil {
			return status.Error(codes.InvalidArgument,
				"Volume Access Mode is required")
		}
		mntVol := volCap.GetMount()
		if mntVol == nil {
			return status.Error(codes.InvalidArgument, "Invalid access type")
		}

		var mntOptions []string
		mntOptions = mntVol.GetMountFlags()
		csmlog.WithContext(ctx).Infof("The mountOptions received are: %s", mntOptions)

		// Inject xprtsec mount option when mTLS is enabled.
		// This is only triggered when NFSTransportSecurity is explicitly set
		// to "mtls" in the StorageClass; non-mTLS volumes are untouched.
		if nfsSec := req.GetVolumeContext()[constants.NFSTransportSecurityParam]; IsMTLSEnabled(nfsSec) {
			if !containsPrefix(mntOptions, "xprtsec=") {
				mntOptions = append(mntOptions, "xprtsec=mtls")
				csmlog.WithContext(ctx).Infof("Injected mount option 'xprtsec=mtls' for mTLS enforcement")
			}
		}

		target := req.GetTargetPath()
		if target == "" {
			return status.Error(codes.InvalidArgument,
				"Target Path is required")
		}

		// make sure target is created
		_, err := mkdir(ctx, target)
		if err != nil {
			return status.Error(codes.FailedPrecondition, fmt.Sprintf("Could not create '%s': '%s'", target, err.Error()))
		}
		roFlag := req.GetReadonly()
		rwOption := "rw"
		if roFlag {
			rwOption = "ro"
		}

		mntOptions = append(mntOptions, rwOption)

		f := csmlog.Fields{
			"ID":         req.VolumeId,
			"TargetPath": target,
			"ExportPath": nfsExportURL,
			"AccessMode": accMode.GetMode(),
		}
		csmlog.WithContext(ctx).WithFields(f).Info("Node publish volume params ")
		mnts, err := getGetMountsFunc()(ctx)
		if err != nil {
			return status.Errorf(codes.Internal,
				"could not reliably determine existing mount status: '%s'",
				err.Error())
		}

		if len(mnts) != 0 {
			for _, m := range mnts {
				// check for idempotency
				// same volume
				if m.Device == nfsExportURL {
					if m.Path == target {
						// as per specs, T1=T2, P1=P2 - return OK
						if contains(m.Opts, rwOption) {
							csmlog.WithContext(ctx).WithFields(f).Debug(
								"mount already in place with same options",
							)
							return nil
						}
						// T1=T2, P1!=P2 - return AlreadyExists
						csmlog.WithContext(ctx).WithFields(f).Error("Mount point already in use by device with different options")
						return status.Error(codes.AlreadyExists, "Mount point already in use by device with different options")
					}
					// T1!=T2, P1==P2 || P1 != P2 - return FailedPrecondition for single node
					if accMode.GetMode() == csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER ||
						accMode.GetMode() == csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY ||
						accMode.GetMode() == csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER {
						csmlog.WithContext(ctx).WithFields(f).Error("Mount point already in use for same device")
						return status.Error(codes.FailedPrecondition, "Mount point already in use for same device")
					}
				}
			}
		}

		csmlog.WithContext(ctx).Infof("The mountOptions being used for mount are: %s", mntOptions)
		if err := getMountFunc()(ctx, nfsExportURL, target, "nfs", mntOptions...); err != nil {
			count := 0
			errmsg := err.Error()
			// Both substring validation is for NFSv3 and NFSv4 errors resp.
			for (strings.Contains(strings.ToLower(errmsg), "access denied by server while mounting") || strings.Contains(strings.ToLower(errmsg), "no such file or directory")) && count < 5 {
				// Record authentication failure on "access denied by server while mounting"
				if strings.Contains(strings.ToLower(errmsg), "access denied by server while mounting") && metricsEnabled {
					recordAuthFailureFunc()
				}
				time.Sleep(2 * time.Second)
				csmlog.WithContext(ctx).Infof("Mount retry attempt-%d", count)
				err = getMountFunc()(ctx, nfsExportURL, target, "nfs", mntOptions...)
				if err != nil {
					errmsg = err.Error()
				} else {
					break
				}
				count++
			}
			if err != nil {
				// Record authentication failure on final error if it's access denied
				if strings.Contains(strings.ToLower(err.Error()), "access denied by server while mounting") && metricsEnabled {
					recordAuthFailureFunc()
				}

				csmlog.WithContext(ctx).Errorf("%v", err)

				return err
			}
		}
		return nil
	}
)

// unpublishVolume removes the mount to the target path
func unpublishVolume(
	ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest, filterStr string,
) error {
	target := req.GetTargetPath()
	if target == "" {
		return status.Error(codes.InvalidArgument,
			"Target Path is required")
	}

	csmlog.WithContext(ctx).Debugf("attempting to unmount '%s'", target)
	isMounted, err := isVolumeMounted(ctx, filterStr, target)
	if err != nil {
		return err
	}
	if !isMounted {
		return nil
	}
	if err := getUnmountFunc()(context.Background(), target); err != nil {
		return status.Errorf(codes.Internal,
			"error unmounting target '%s': '%s'", target, err.Error())
	}
	csmlog.WithContext(ctx).Debugf("unmounting '%s' succeeded", target)

	// Remove the target path after unmounting
	if err := getOsRemoveAllFunc()(target); err != nil {
		return status.Errorf(codes.Internal,
			"error removing target path '%s': '%s'", target, err.Error())
	}
	csmlog.WithContext(ctx).Debugf("removing target path '%s' succeeded", target)

	return nil
}

// mkdir creates the directory specified by path if needed.
// return pair is a bool flag of whether dir was created, and an error
func mkdir(ctx context.Context, path string) (bool, error) {
	st, err := os.Stat(path)
	if err == nil {
		if !st.IsDir() {
			return false, fmt.Errorf("existing path is not a directory")
		}
		return false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{"dir": path}).Errorf("Unable to stat dir : %v", err)
		return false, err
	}

	// Case when there is error and the error is fs.ErrNotExists.
	if err := os.MkdirAll(path, 0o750); err != nil {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{"dir": path}).Errorf("Unable to create dir : %v", err)
		return false, err
	}

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{"path": path}).Debug("created directory")
	return true, nil
}

func contains(list []string, item string) bool {
	for _, x := range list {
		if x == item {
			return true
		}
	}
	return false
}

// containsPrefix returns true if any element in list starts with prefix.
func containsPrefix(list []string, prefix string) bool {
	for _, x := range list {
		if strings.HasPrefix(strings.ToLower(x), strings.ToLower(prefix)) {
			return true
		}
	}
	return false
}

func isVolumeMounted(ctx context.Context, filterStr string, target string) (bool, error) {
	mnts, err := getGetMountsFunc()(ctx)
	if err != nil {
		return false, status.Errorf(codes.Internal,
			"could not reliably determine existing mount status: '%s'",
			err.Error())
	}

	if len(mnts) != 0 {
		// Idempotence check not to return error if not published
		mounted := false
		for _, m := range mnts {
			if strings.Contains(m.Device, filterStr) {
				if m.Path == target {
					mounted = true
					return mounted, nil
				}
			}
		}
		if mounted == false {
			csmlog.WithContext(ctx).Debugf("target '%s' does not exist", target)
			return mounted, nil
		}
	}
	// No mount exists also means not published
	csmlog.WithContext(ctx).Debugf("target '%s' does not exist", target)
	return false, nil
}
