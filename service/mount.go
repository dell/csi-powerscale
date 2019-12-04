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
	"fmt"
	"os"

	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/gofsutil"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func publishVolume(
	req *csi.NodePublishVolumeRequest,
	nfsExportURL string, nfsV3 bool) error {

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
	target := req.GetTargetPath()
	if target == "" {
		return status.Error(codes.InvalidArgument,
			"Target Path is required")
	}

	// make sure target is created
	_, err := mkdir(target)
	if err != nil {
		return status.Error(codes.FailedPrecondition, fmt.Sprintf("Could not create '%s': '%s'", target, err.Error()))
	}
	roFlag := req.GetReadonly()
	rwOption := "rw"
	if roFlag {
		rwOption = "ro"
	}

	if nfsV3 {
		rwOption = fmt.Sprintf("%s,%s", rwOption, "vers=3")
	}

	f := log.Fields{
		"ID":         req.VolumeId,
		"TargetPath": target,
		"ExportPath": nfsExportURL,
		"AccessMode": accMode.GetMode(),
	}
	log.WithFields(f).Info("Node publish volume params ")
	ctx := context.Background()
	mnts, err := gofsutil.GetMounts(ctx)
	if err != nil {
		return status.Errorf(codes.Internal,
			"could not reliably determine existing mount status: '%s'",
			err.Error())
	}
	if len(mnts) != 0 {
		for _, m := range mnts {
			// check for idempotency
			//same volume
			if m.Device == nfsExportURL {
				if m.Path == target {
					//as per specs, T1=T2, P1=P2 - return OK
					if contains(m.Opts, rwOption) {
						log.WithFields(f).Debug(
							"mount already in place with same options")
						return nil
					}
					//T1=T2, P1!=P2 - return AlreadyExists
					log.WithFields(f).Error("Mount point already in use by device with different options")
					return status.Error(codes.AlreadyExists, "Mount point already in use by device with different options")
				}
				//T1!=T2, P1==P2 || P1 != P2 - return FailedPrecondition for single node
				if accMode.GetMode() == csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER ||
					accMode.GetMode() == csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY {
					log.WithFields(f).Error("Mount point already in use for same device")
					return status.Error(codes.FailedPrecondition, "Mount point already in use for same device")
				}

			}
		}
	}

	if err := gofsutil.Mount(context.Background(), nfsExportURL, target, "nfs", rwOption); err != nil {
		log.Errorf("%v", err)
		return err
	}
	return nil
}

// unpublishVolume removes the mount to the target path
func unpublishVolume(
	req *csi.NodeUnpublishVolumeRequest, filterStr string) error {
	target := req.GetTargetPath()
	if target == "" {
		return status.Error(codes.InvalidArgument,
			"Target Path is required")
	}

	log.Debugf("attempting to unmount '%s'", target)
	ctx := context.Background()
	mnts, err := gofsutil.GetMounts(ctx)
	if err != nil {
		return status.Errorf(codes.Internal,
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
					break
				}
			}
		}
		if mounted == false {
			log.Debugf("target '%s' does not exist", target)
			return nil
		}
	} else {
		// No mount exists also means not published
		log.Debugf("target '%s' does not exist", target)
		return nil
	}
	if err := gofsutil.Unmount(context.Background(), target); err != nil {
		return status.Errorf(codes.Internal,
			"error unmounting target'%s': '%s'", target, err.Error())
	}
	log.Debugf("unmounting '%s' succeeded", target)

	return nil
}

// mkdir creates the directory specified by path if needed.
// return pair is a bool flag of whether dir was created, and an error
func mkdir(path string) (bool, error) {
	st, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err := os.Mkdir(path, 0750); err != nil {
			log.WithField("dir", path).WithError(
				err).Error("Unable to create dir")
			return false, err
		}
		log.WithField("path", path).Debug("created directory")
		return true, nil
	}
	if !st.IsDir() {
		return false, fmt.Errorf("existing path is not a directory")
	}
	return false, nil
}

func contains(list []string, item string) bool {
	for _, x := range list {
		if x == item {
			return true
		}
	}
	return false
}
