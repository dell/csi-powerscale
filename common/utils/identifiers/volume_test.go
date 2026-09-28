/*
 *
 * Copyright © 2021-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

func TestParseNormalizedVolumeID(t *testing.T) {
	ctx := context.Background()

	volName, exportID, accessZone, clusterName, err := ParseNormalizedVolumeID(ctx, "k8s-e89c9d089e=_=_=19=_=_=csi0zone=_=_=cluster1")

	assert.Equal(t, "k8s-e89c9d089e", volName)
	assert.Equal(t, 19, exportID)
	assert.Equal(t, "csi0zone", accessZone)
	assert.Equal(t, "cluster1", clusterName)
	assert.Nil(t, err)

	_, _, _, _, err = ParseNormalizedVolumeID(ctx, "totally bogus")
	assert.NotNil(t, err)

	_, _, _, _, err = ParseNormalizedVolumeID(ctx, "k8s-e89c9d089e=_=_=not_an_integer=_=_=csi0zone")
	assert.NotNil(t, err)
}

func TestGetNormalizedVolumeID(t *testing.T) {
	ctx := context.Background()

	// Test case 1: Valid input
	volName := "k8s-e89c9d089e"
	exportID := 19
	accessZone := "csi0zone"
	clusterName := "cluster1"

	expectedVolID := fmt.Sprintf("%s%s%d%s%s%s%s", volName, VolumeIDSeparator, exportID, VolumeIDSeparator, accessZone, VolumeIDSeparator, clusterName)
	actualVolID := GetNormalizedVolumeID(ctx, volName, exportID, accessZone, clusterName)

	assert.Equal(t, expectedVolID, actualVolID, "Generated volume ID should match the expected format")

	// Test case 2: Edge case - Empty values
	emptyVolID := GetNormalizedVolumeID(ctx, "", 0, "", "")
	expectedEmptyVolID := fmt.Sprintf("%s%s%d%s%s%s%s", "", VolumeIDSeparator, 0, VolumeIDSeparator, "", VolumeIDSeparator, "")
	assert.Equal(t, expectedEmptyVolID, emptyVolID, "Empty values should still generate a valid but empty formatted string")

	// Test case 3: Special characters in input
	specialCharVolID := GetNormalizedVolumeID(ctx, "vol@name", 42, "zone#1", "cluster!name")
	expectedSpecialVolID := fmt.Sprintf("%s%s%d%s%s%s%s", "vol@name", VolumeIDSeparator, 42, VolumeIDSeparator, "zone#1", VolumeIDSeparator, "cluster!name")
	assert.Equal(t, expectedSpecialVolID, specialCharVolID, "Volume ID should handle special characters properly")
}

func TestGetDirectoryBackedVolumeID(t *testing.T) {
	ctx := context.Background()

	volName := "vol1"
	exportID := 100
	accessZone := "System"
	clusterName := "cluster1"

	expectedVolID := fmt.Sprintf("%s%s%d%s%s%s%s%s%s", volName, VolumeIDSeparator, exportID, VolumeIDSeparator, accessZone, VolumeIDSeparator, clusterName, VolumeIDSeparator, ProvisioningModeDirectory)
	actualVolID := GetDirectoryBackedVolumeID(ctx, volName, exportID, accessZone, clusterName)

	assert.Equal(t, expectedVolID, actualVolID, "Directory-backed volume ID should include 'directory' token")
	assert.Contains(t, actualVolID, ProvisioningModeDirectory, "Volume ID should contain provisioning mode token")
}

func TestParseVolumeIDWithMode(t *testing.T) {
	ctx := context.Background()

	// Test case 1: Directory-backed volume ID
	dirVolID := "vol1=_=_=100=_=_=System=_=_=cluster1=_=_=directory"
	volName, exportID, accessZone, clusterName, provisioningMode, err := ParseVolumeIDWithMode(ctx, dirVolID)

	assert.Nil(t, err)
	assert.Equal(t, "vol1", volName)
	assert.Equal(t, 100, exportID)
	assert.Equal(t, "System", accessZone)
	assert.Equal(t, "cluster1", clusterName)
	assert.Equal(t, ProvisioningModeDirectory, provisioningMode)

	// Test case 2: Export-backed volume ID (no mode token)
	exportVolID := "vol2=_=_=200=_=_=System=_=_=cluster1"
	volName2, exportID2, accessZone2, clusterName2, provisioningMode2, err2 := ParseVolumeIDWithMode(ctx, exportVolID)

	assert.Nil(t, err2)
	assert.Equal(t, "vol2", volName2)
	assert.Equal(t, 200, exportID2)
	assert.Equal(t, "System", accessZone2)
	assert.Equal(t, "cluster1", clusterName2)
	assert.Equal(t, "", provisioningMode2, "Export-backed volume should have empty provisioning mode")

	// Test case 3: Invalid volume ID
	_, _, _, _, _, err3 := ParseVolumeIDWithMode(ctx, "invalid")
	assert.NotNil(t, err3, "Invalid volume ID should return error")

	// Test case 4: Volume ID without cluster name
	volID4 := "vol3=_=_=300=_=_=System"
	volName4, exportID4, accessZone4, clusterName4, provisioningMode4, err4 := ParseVolumeIDWithMode(ctx, volID4)

	assert.Nil(t, err4)
	assert.Equal(t, "vol3", volName4)
	assert.Equal(t, 300, exportID4)
	assert.Equal(t, "System", accessZone4)
	assert.Equal(t, "", clusterName4)
	assert.Equal(t, "", provisioningMode4)
}
