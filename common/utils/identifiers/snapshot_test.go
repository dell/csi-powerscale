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

package identifiers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNormalizedSnapshotID(t *testing.T) {
	ctx := context.Background()

	snapshotID, clusterName, accessZone, err := ParseNormalizedSnapshotID(ctx, "284=_=_=cluster1=_=_=System")

	assert.Equal(t, "284", snapshotID)
	assert.Equal(t, "cluster1", clusterName)
	assert.Equal(t, "System", accessZone)
	assert.Nil(t, err)

	expectedError := "access zone not found in snapshot ID '284=_=_=cluster1'"
	_, _, _, err = ParseNormalizedSnapshotID(ctx, "284=_=_=cluster1")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), expectedError)
}

func TestGetNormalizedSnapshotID(t *testing.T) {
	ctx := context.Background()

	// Test case 1: Valid input
	snapshotID := "12345"
	clusterName := "cluster1"
	accessZone := "zone1"

	expectedSnapID := fmt.Sprintf("%s%s%s%s%s", snapshotID, SnapshotIDSeparator, clusterName, SnapshotIDSeparator, accessZone)
	actualSnapID := GetNormalizedSnapshotID(ctx, snapshotID, clusterName, accessZone)

	assert.Equal(t, expectedSnapID, actualSnapID, "Generated snapshot ID should match the expected format")

	// Test case 2: Edge case - Empty values
	emptySnapID := GetNormalizedSnapshotID(ctx, "", "", "")
	expectedEmptySnapID := fmt.Sprintf("%s%s%s%s%s", "", SnapshotIDSeparator, "", SnapshotIDSeparator, "")
	assert.Equal(t, expectedEmptySnapID, emptySnapID, "Empty values should still generate a valid but empty formatted string")

	// Test case 3: Special characters in input
	specialSnapID := GetNormalizedSnapshotID(ctx, "snap@123", "cluster#name", "zone!1")
	expectedSpecialSnapID := fmt.Sprintf("%s%s%s%s%s", "snap@123", SnapshotIDSeparator, "cluster#name", SnapshotIDSeparator, "zone!1")
	assert.Equal(t, expectedSpecialSnapID, specialSnapID, "Snapshot ID should handle special characters properly")
}
