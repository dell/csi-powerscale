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
	"strings"

	"github.com/dell/csi-isilon/v2/common/utils/logging"
)

// SnapshotIDSeparator is the separator that separates snapshot id and cluster name (two components that a normalized snapshot ID is comprised of)
const SnapshotIDSeparator = "=_=_="

// GetNormalizedSnapshotID combines snapshotID ID and cluster name and access zone to form the normalized snapshot ID
// e.g. 12345 + cluster1 + accessZone => 12345=_=_=cluster1=_=_=zone1
func GetNormalizedSnapshotID(ctx context.Context, snapshotID, clusterName, accessZone string) string {
	log := logging.GetRunIDLogger(ctx)

	snapID := fmt.Sprintf("%s%s%s%s%s", snapshotID, SnapshotIDSeparator, clusterName, SnapshotIDSeparator, accessZone)

	log.Debugf("combined snapshot id '%s' access zone '%s' and cluster name '%s' to form normalized snapshot ID '%s'",
		snapshotID, accessZone, clusterName, snapID)

	return snapID
}

// ParseNormalizedSnapshotID parses the normalized snapshot ID(using SnapshotIDSeparator) to extract the snapshot ID and cluster name(optional) that make up the normalized snapshot ID
// e.g. 12345 => 12345, ""
// e.g. 12345=_=_=cluster1=_=_=zone => 12345, cluster1, zone
func ParseNormalizedSnapshotID(ctx context.Context, snapID string) (string, string, string, error) {
	log := logging.GetRunIDLogger(ctx)
	tokens := strings.Split(snapID, SnapshotIDSeparator)
	if len(tokens) < 1 {
		return "", "", "", fmt.Errorf("snapshot ID '%s' cannot be split into tokens", snapID)
	}

	snapshotID := tokens[0]
	var clusterName, accessZone string
	if len(tokens) > 1 {
		clusterName = tokens[1]
		if len(tokens) > 2 {
			accessZone = tokens[2]
		} else {
			return "", "", "", fmt.Errorf("access zone not found in snapshot ID '%s'", snapID)
		}
	}

	log.Debugf("normalized snapshot ID '%s' parsed into snapshot ID '%s' and cluster name '%s'",
		snapID, snapshotID, clusterName)

	return snapshotID, clusterName, accessZone, nil
}
