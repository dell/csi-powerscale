package utils

/*
Copyright (c) 2019-2022 Dell Inc, or its subsidiaries.

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
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsStringInSlice(t *testing.T) {
	list := []string{"hello", "world", "jason"}

	assert.True(t, IsStringInSlice("world", list))
	assert.False(t, IsStringInSlice("mary", list))
	assert.False(t, IsStringInSlice("harry", nil))
}

func TestRemoveStringFromSlice(t *testing.T) {
	list := []string{"hello", "world", "jason"}

	result := RemoveStringFromSlice("hello", list)

	assert.Equal(t, 3, len(list))
	assert.Equal(t, 2, len(result))

	result = RemoveStringFromSlice("joe", list)

	assert.Equal(t, 3, len(result))
}

func TestRemoveStringsFromSlice(t *testing.T) {
	list := []string{"hello", "world", "jason"}

	filterList := []string{"hello", "there", "chap", "world"}

	result := RemoveStringsFromSlice(filterList, list)

	assert.Equal(t, 1, len(result))
}

func TestGetNormalizedVolumeID(t *testing.T) {
	ctx := context.Background()

	volID := GetNormalizedVolumeID(ctx, "k8s-e89c9d089e", 19, "csi0zone", "cluster1")

	assert.Equal(t, "k8s-e89c9d089e=_=_=19=_=_=csi0zone=_=_=cluster1", volID)
}

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

func TestParseNormalizedSnapshotID(t *testing.T) {
	ctx := context.Background()

	snapshotID, clusterName, accessZone, err := ParseNormalizedSnapshotID(ctx, "284=_=_=cluster1=_=_=System")

	assert.Equal(t, "284", snapshotID)
	assert.Equal(t, "cluster1", clusterName)
	assert.Equal(t, "System", accessZone)
	assert.Nil(t, err)

	expectedError := "access zone not found in snapshot ID '284=_=_=cluster1'"
	_, _, _, err = ParseNormalizedSnapshotID(ctx, "284=_=_=cluster1")
	if err.Error() != expectedError {
		t.Errorf("expected error '%s', but got '%s'", expectedError, err.Error())
	}
}

func TestGetPathForVolume(t *testing.T) {
	isiPath := "/ifs/data"
	volName := "k8s-123456"
	targetPath1 := GetPathForVolume(isiPath, volName)
	isiPath = "/ifs/data/"
	targetPath2 := GetPathForVolume(isiPath, volName)
	print(targetPath1 + "-" + targetPath2)
	assert.Equal(t, targetPath1, targetPath2)
}

func TestGetIsiPathFromExportPath(t *testing.T) {
	exportPath := "/ifs/data/k8s-123456"
	isiPath1 := GetIsiPathFromExportPath(exportPath)
	exportPath = "/ifs/data/k8s-123456/"
	isiPath2 := GetIsiPathFromExportPath(exportPath)
	print(isiPath1 + "-" + isiPath2)
	assert.Equal(t, isiPath1, isiPath2)
}

func TestGetExportIDFromConflictMessage(t *testing.T) {
	comparation := 82851
	message := fmt.Sprintf("Export rules %d and 82859 conflict on '/ifs/data/csi/Daniel/k8s-fd8d12ede9'", comparation)
	exportID := GetExportIDFromConflictMessage(message)
	assert.Equal(t, exportID, comparation)
}

func TestGetFQDNByIP(t *testing.T) {
	ctx := context.Background()
	fqdn, _ := GetFQDNByIP(ctx, "111.111.111.111")
	fmt.Println(fqdn)
	assert.Equal(t, fqdn, "")
}

func TestGetVolumeNameFromExportPath(t *testing.T) {
	exportPath := "/ifs/data/k8s-123456"
	volName1 := GetVolumeNameFromExportPath(exportPath)
	exportPath = "/ifs/data/k8s-123456/"
	volName2 := GetVolumeNameFromExportPath(exportPath)
	print(volName1 + "-" + volName2)
	assert.Equal(t, volName1, "k8s-123456")
	assert.Equal(t, volName2, "k8s-123456")
}
