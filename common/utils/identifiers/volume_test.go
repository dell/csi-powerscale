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
