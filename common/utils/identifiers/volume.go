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
package identifiers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/dell/csmlog"
)

// VolumeIDSeparator is the separator that separates volume name and export ID (two components that a normalized volume ID is comprised of)
const VolumeIDSeparator = "=_=_="

// GetNormalizedVolumeID combines volume name (i.e. the directory name), export ID, access zone and clusterName to form the normalized volume ID
// e.g. k8s-e89c9d089e + 19 + csi0zone + cluster1 => k8s-e89c9d089e=_=_=19=_=_=csi0zone=_=_=cluster1
func GetNormalizedVolumeID(ctx context.Context, volName string, exportID int, accessZone, clusterName string) string {
	log := csmlog.GetLogger().WithContext(ctx)

	volID := fmt.Sprintf("%s%s%s%s%s%s%s", volName, VolumeIDSeparator, strconv.Itoa(exportID), VolumeIDSeparator, accessZone, VolumeIDSeparator, clusterName)

	log.Debugf("combined volume name '%s' with export ID '%d', access zone '%s' and cluster name '%s' to form volume ID '%s'",
		volName, exportID, accessZone, clusterName, volID)

	return volID
}

// ParseNormalizedVolumeID parses the volume ID(using VolumeIDSeparator) to extract the volume name, export ID, access zone and cluster name(optional) that make up the volume ID
// e.g. k8s-e89c9d089e=_=_=19=_=_=csi0zone => k8s-e89c9d089e, 19, csi0zone, ""
// e.g. k8s-e89c9d089e=_=_=19=_=_=csi0zone=_=_=cluster1 => k8s-e89c9d089e, 19, csi0zone, cluster1
func ParseNormalizedVolumeID(ctx context.Context, volID string) (string, int, string, string, error) {
	log := csmlog.GetLogger().WithContext(ctx)
	tokens := strings.Split(volID, VolumeIDSeparator)
	if len(tokens) < 3 {
		return "", 0, "", "", fmt.Errorf("volume ID '%s' cannot be split into tokens", volID)
	}

	volumeName := tokens[0]

	exportID, err := strconv.Atoi(tokens[1])
	if err != nil {
		return "", 0, "", "", err
	}

	accessZone := tokens[2]

	var clusterName string
	if len(tokens) > 3 {
		clusterName = tokens[3]
	}

	log.Debugf("volume ID '%s' parsed into volume name '%s', export ID '%d', access zone '%s' and cluster name '%s'",
		volID, volumeName, exportID, accessZone, clusterName)

	return volumeName, exportID, accessZone, clusterName, nil
}
