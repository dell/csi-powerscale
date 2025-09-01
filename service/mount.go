/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

/*
Copyright (c) 2019-2025 Dell Inc, or its subsidiaries.

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
package service

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/gofsutil"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TODO: All WithFields call containing logrus have to be converted to log
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

	publishVolumeFunc = func(
		ctx context.Context,
		req *csi.NodePublishVolumeRequest,
		nfsExportURL string,
	) error {
		// Fetch log handler
		ctx, log := GetLogger(ctx)

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
		log.Infof("The mountOptions received are: %s", mntOptions)

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

		f := logrus.Fields{
			"ID":         req.VolumeId,
			"TargetPath": target,
			"ExportPath": nfsExportURL,
			"AccessMode": accMode.GetMode(),
		}
		logrus.WithFields(f).Info("Node publish volume params ")
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
							logrus.WithFields(f).Debug(
								"mount already in place with same options")
							return nil
						}
						// T1=T2, P1!=P2 - return AlreadyExists
						logrus.WithFields(f).Error("Mount point already in use by device with different options")
						return status.Error(codes.AlreadyExists, "Mount point already in use by device with different options")
					}
					// T1!=T2, P1==P2 || P1 != P2 - return FailedPrecondition for single node
					if accMode.GetMode() == csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER ||
						accMode.GetMode() == csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY ||
						accMode.GetMode() == csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER {
						logrus.WithFields(f).Error("Mount point already in use for same device")
						return status.Error(codes.FailedPrecondition, "Mount point already in use for same device")
					}
				}
			}
		}

		log.Infof("The mountOptions being used for mount are: %s", mntOptions)
		if err := getMountFunc()(context.Background(), nfsExportURL, target, "nfs", mntOptions...); err != nil {
			count := 0
			errmsg := err.Error()
			// Both substring validation is for NFSv3 and NFSv4 errors resp.
			for (strings.Contains(strings.ToLower(errmsg), "access denied by server while mounting") || (strings.Contains(strings.ToLower(errmsg), "no such file or directory"))) && count < 5 {
				time.Sleep(2 * time.Second)
				log.Infof("Mount retry attempt-%d", count)
				err = getMountFunc()(context.Background(), nfsExportURL, target, "nfs", mntOptions...)
				if err != nil {
					errmsg = err.Error()
				} else {
					break
				}
				count++
			}
			if err != nil {
				log.Errorf("%v", err)
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
	// Fetch log handler
	ctx, log := GetLogger(ctx)

	target := req.GetTargetPath()
	if target == "" {
		return status.Error(codes.InvalidArgument,
			"Target Path is required")
	}

	log.Debugf("attempting to unmount '%s'", target)
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
	log.Debugf("unmounting '%s' succeeded", target)

	// Remove the target path after unmounting
	if err := getOsRemoveAllFunc()(target); err != nil {
		return status.Errorf(codes.Internal,
			"error removing target path '%s': '%s'", target, err.Error())
	}
	log.Debugf("removing target path '%s' succeeded", target)

	return nil
}

// mkdir creates the directory specified by path if needed.
// return pair is a bool flag of whether dir was created, and an error
func mkdir(ctx context.Context, path string) (bool, error) {
	_, log := GetLogger(ctx)
	st, err := os.Stat(path)
	if err == nil {
		if !st.IsDir() {
			return false, fmt.Errorf("existing path is not a directory")
		}
		return false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		log.WithField("dir", path).WithError(err).Error("Unable to stat dir")
		return false, err
	}

	// Case when there is error and the error is fs.ErrNotExists.
	if err := os.MkdirAll(path, 0o750); err != nil {
		log.WithField("dir", path).WithError(err).Error("Unable to create dir")
		return false, err
	}

	log.WithField("path", path).Debug("created directory")
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

func isVolumeMounted(ctx context.Context, filterStr string, target string) (bool, error) {
	// Fetch log handler
	ctx, log := GetLogger(ctx)

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
			log.Debugf("target '%s' does not exist", target)
			return mounted, nil
		}
	}
	// No mount exists also means not published
	log.Debugf("target '%s' does not exist", target)
	return false, nil
}
