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
	"os"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// func TestGetNormalizedVolumeID(t *testing.T) {
// 	ctx := context.Background()

// 	volID := GetNormalizedVolumeID(ctx, "k8s-e89c9d089e", 19, "csi0zone", "cluster1")

// 	assert.Equal(t, "k8s-e89c9d089e=_=_=19=_=_=csi0zone=_=_=cluster1", volID)
// }

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
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), expectedError)
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

func TestParseNodeID(t *testing.T) {
	tests := []struct {
		name     string
		nodeID   string
		wantName string
		wantFQDN string
		wantIP   string
		wantErr  bool
	}{
		{
			name:     "Valid Node ID",
			nodeID:   "node1=#=#=fqdn.example.com=#=#=192.168.1.1",
			wantName: "node1",
			wantFQDN: "fqdn.example.com",
			wantIP:   "192.168.1.1",
			wantErr:  false,
		},
		{
			name:    "Invalid Node ID - Missing Sections",
			nodeID:  "node1=#=#=fqdn.example.com",
			wantErr: true,
		},
		{
			name:    "Invalid Node ID - Empty String",
			nodeID:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			gotName, gotFQDN, gotIP, err := ParseNodeID(ctx, tt.nodeID)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNodeID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotName != tt.wantName {
				t.Errorf("ParseNodeID() gotName = %v, want %v", gotName, tt.wantName)
			}
			if gotFQDN != tt.wantFQDN {
				t.Errorf("ParseNodeID() gotFQDN = %v, want %v", gotFQDN, tt.wantFQDN)
			}
			if gotIP != tt.wantIP {
				t.Errorf("ParseNodeID() gotIP = %v, want %v", gotIP, tt.wantIP)
			}
		})
	}
}

func TestGetIsiPathFromPgID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Valid Input", "pg123::/ifs/data", "/ifs/data"},
		{"No Separator", "pg123/ifs/data", ""},
		{"Empty Input", "", ""},
		{"Multiple Separators", "pg123::/ifs/data::extra", ""},
		{"Only Separator", "::", ""},
		{"Leading Separator", "::/ifs/data", "/ifs/data"},
		{"Trailing Separator", "pg123::", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetIsiPathFromPgID(tt.input)
			if got != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestGetMessageWithRunID(t *testing.T) {
	tests := []struct {
		name     string
		runid    string
		format   string
		args     []interface{}
		expected string
	}{
		{"Basic message", "12345", "Process started", nil, " runid=12345 Process started"},
		{"Formatted message", "98765", "Error code: %d", []interface{}{404}, " runid=98765 Error code: 404"},
		{"Multiple arguments", "56789", "User %s logged in at %s", []interface{}{"Alice", "10:00 AM"}, " runid=56789 User Alice logged in at 10:00 AM"},
		{"Empty runID", "", "System rebooting", nil, " runid= System rebooting"},
		{"Empty format", "54321", "", nil, " runid=54321 "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetMessageWithRunID(tt.runid, tt.format, tt.args...)
			if got != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestParseBooleanFromContext(t *testing.T) {
	ctx := context.Background()
	os.Setenv("TEST_BOOL", "true")
	defer os.Unsetenv("TEST_BOOL")

	val := ParseBooleanFromContext(ctx, "TEST_BOOL")
	if val != true {
		t.Errorf("Expected true, got %v", val)
	}
}

func TestParseArrayFromContext(t *testing.T) {
	ctx := context.Background()
	arrYAML := "- item1\n- item2\n- item3"
	os.Setenv("TEST_ARRAY", arrYAML)
	defer os.Unsetenv("TEST_ARRAY")

	val, err := ParseArrayFromContext(ctx, "TEST_ARRAY")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(val) != 3 || val[0] != "item1" || val[1] != "item2" || val[2] != "item3" {
		t.Errorf("Unexpected parsed array: %v", val)
	}
}

func TestParseUintFromContext(t *testing.T) {
	ctx := context.Background()
	os.Setenv("TEST_UINT", "42")
	defer os.Unsetenv("TEST_UINT")

	val := ParseUintFromContext(ctx, "TEST_UINT")
	if val != 42 {
		t.Errorf("Expected 42, got %v", val)
	}
}

func TestParseInt64FromContext(t *testing.T) {
	ctx := context.Background()
	os.Setenv("TEST_INT64", "-100")
	defer os.Unsetenv("TEST_INT64")

	val, err := ParseInt64FromContext(ctx, "TEST_INT64")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if val != -100 {
		t.Errorf("Expected -100, got %v", val)
	}
}

func TestRemoveExistingCSISockFile(t *testing.T) {
	const testSockFile = "/tmp/test.sock"
	os.Setenv("CSI_ENDPOINT", testSockFile)

	// Create a test socket file
	_, err := os.Create(testSockFile)
	if err != nil {
		t.Fatalf("failed to create test socket file: %v", err)
	}

	err = RemoveExistingCSISockFile()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if _, err := os.Stat(testSockFile); !os.IsNotExist(err) {
		t.Errorf("expected file to be removed, but it still exists")
	}
}

func TestGetNewUUID(t *testing.T) {
	id, err := GetNewUUID()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if _, err := uuid.Parse(id); err != nil {
		t.Errorf("expected valid UUID, got %s", id)
	}
}

func TestLogMap(t *testing.T) {
	ctx := context.Background()
	m := map[string]string{"key1": "value1", "key2": "value2"}
	LogMap(ctx, "testMap", m) // Ensure this runs without panic
}
func TestGetOwnFQDN(t *testing.T) {
	fqdn, err := GetOwnFQDN()
	if err != nil {
		t.Skip("Skipping test: Unable to get FQDN")
	}
	assert.NoError(t, err)
	assert.NotEmpty(t, fqdn)
}

func TestGetAccessMode(t *testing.T) {
	// Case 1: Valid access mode
	req := &csi.ControllerPublishVolumeRequest{
		VolumeCapability: &csi.VolumeCapability{
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}
	mode, err := GetAccessMode(req)
	assert.NoError(t, err)
	assert.NotNil(t, mode)
	assert.Equal(t, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER, *mode)

	// Case 2: Nil VolumeCapability
	req = &csi.ControllerPublishVolumeRequest{
		VolumeCapability: nil,
	}
	mode, err = GetAccessMode(req)
	assert.Nil(t, mode)
	assert.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "volume capability is required")

	// Case 3: Nil AccessMode
	req = &csi.ControllerPublishVolumeRequest{
		VolumeCapability: &csi.VolumeCapability{
			AccessMode: nil,
		},
	}
	mode, err = GetAccessMode(req)
	assert.Nil(t, mode)
	assert.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "access mode is required")

	// Case 4: Unknown AccessMode
	req = &csi.ControllerPublishVolumeRequest{
		VolumeCapability: &csi.VolumeCapability{
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
			},
		},
	}
	mode, err = GetAccessMode(req)
	assert.Nil(t, mode)
	assert.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "unknown access mode")
}

func TestRemoveSurroundingQuotes(t *testing.T) {
	// Case 1: String with quotes at both ends
	input := `"hello"`
	expected := "hello"
	result := RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected quotes to be removed")

	// Case 2: String with quotes only at the beginning
	input = `"hello`
	expected = "hello"
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected leading quote to be removed")

	// Case 3: String with quotes only at the end
	input = `hello"`
	expected = "hello"
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected trailing quote to be removed")

	// Case 4: String without quotes
	input = `hello`
	expected = "hello"
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected string to remain unchanged")

	// Case 5: Empty string
	input = ``
	expected = ``
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected empty string to remain unchanged")

	// Case 6: String with only one quote character (should be removed)
	input = `"`
	expected = ``
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected single quote to be removed")

	// Case 7: String with multiple words and surrounding quotes
	input = `"hello world"`
	expected = "hello world"
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected surrounding quotes to be removed")

	// Case 8: String with nested quotes (should only remove outermost quotes)
	input = `""hello""`
	expected = `"hello"`
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected only outermost quotes to be removed")

	// Case 9: String with space and quotes at both ends
	input = `" hello "`
	expected = " hello "
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected surrounding quotes to be removed but preserve spaces")

	// Case 10: String that is just two quotes (should become empty)
	input = `""`
	expected = ``
	result = RemoveSurroundingQuotes(input)
	assert.Equal(t, expected, result, "Expected two quotes to be removed and return empty string")
}

// TestGetQuotaIDWithCSITag tests the GetQuotaIDWithCSITag function
func TestGetQuotaIDWithCSITag(t *testing.T) {
	// Case 1: Valid quota ID
	input := "12345"
	expected := fmt.Sprintf("%s%s", CSIQuotaIDPrefix, input)
	result := GetQuotaIDWithCSITag(input)
	assert.Equal(t, expected, result, "Expected quota ID to be prefixed correctly")

	// Case 2: Empty quota ID
	input = ""
	expected = ""
	result = GetQuotaIDWithCSITag(input)
	assert.Equal(t, expected, result, "Expected empty string to remain unchanged")

	// Case 3: Quota ID with special characters
	input = "!@#$%"
	expected = fmt.Sprintf("%s%s", CSIQuotaIDPrefix, input)
	result = GetQuotaIDWithCSITag(input)
	assert.Equal(t, expected, result, "Expected quota ID with special characters to be prefixed correctly")

	// Case 4: Quota ID with spaces
	input = "quota 123"
	expected = fmt.Sprintf("%s%s", CSIQuotaIDPrefix, input)
	result = GetQuotaIDWithCSITag(input)
	assert.Equal(t, expected, result, "Expected quota ID with spaces to be prefixed correctly")
}

func TestCombineTwoStrings(t *testing.T) {
	// Case 1: Normal strings with a hyphen as a separator
	result := CombineTwoStrings("hello", "world", "-")
	expected := "hello-world"
	assert.Equal(t, expected, result, "Expected 'hello-world'")

	// Case 2: Empty first string
	result = CombineTwoStrings("", "world", "-")
	expected = "-world"
	assert.Equal(t, expected, result, "Expected '-world' when first string is empty")

	// Case 3: Empty second string
	result = CombineTwoStrings("hello", "", "-")
	expected = "hello-"
	assert.Equal(t, expected, result, "Expected 'hello-' when second string is empty")

	// Case 4: Empty separator
	result = CombineTwoStrings("hello", "world", "")
	expected = "helloworld"
	assert.Equal(t, expected, result, "Expected 'helloworld' when separator is empty")

	// Case 5: Both strings empty
	result = CombineTwoStrings("", "", "-")
	expected = "-"
	assert.Equal(t, expected, result, "Expected '-' when both strings are empty")

	// Case 6: Special characters in strings
	result = CombineTwoStrings("foo@", "bar$", "#")
	expected = "foo@#bar$"
	assert.Equal(t, expected, result, "Expected 'foo@#bar$'")

	// Case 7: Long strings
	result = CombineTwoStrings("longstring1", "longstring2", "--")
	expected = "longstring1--longstring2"
	assert.Equal(t, expected, result, "Expected 'longstring1--longstring2'")

	// Case 8: Numeric strings
	result = CombineTwoStrings("123", "456", ":")
	expected = "123:456"
	assert.Equal(t, expected, result, "Expected '123:456' for numeric strings")
}

func TestIsStringInSlices(t *testing.T) {
	// Case 1: String is present in one of the slices
	result := IsStringInSlices("apple", []string{"banana", "cherry"}, []string{"apple", "grape"})
	assert.True(t, result, "Expected 'apple' to be found in one of the slices")

	// Case 2: String is not present in any slice
	result = IsStringInSlices("mango", []string{"banana", "cherry"}, []string{"apple", "grape"})
	assert.False(t, result, "Expected 'mango' not to be found in any slice")

	// Case 3: String is present in multiple slices
	result = IsStringInSlices("apple", []string{"apple", "cherry"}, []string{"apple", "grape"})
	assert.True(t, result, "Expected 'apple' to be found in multiple slices")

	// Case 4: String is present in an empty slice
	result = IsStringInSlices("apple", []string{}, []string{"apple", "grape"})
	assert.True(t, result, "Expected 'apple' to be found even if one slice is empty")

	// Case 5: All slices are empty
	result = IsStringInSlices("apple", []string{}, []string{})
	assert.False(t, result, "Expected 'apple' not to be found in empty slices")

	// Case 6: String is present in a slice containing only itself
	result = IsStringInSlices("orange", []string{"orange"})
	assert.True(t, result, "Expected 'orange' to be found in a single-element slice")

	// Case 7: String is present in a case-sensitive manner
	result = IsStringInSlices("apple", []string{"Apple"}, []string{"aPPle"})
	assert.False(t, result, "Expected 'apple' not to be found due to case sensitivity")

	// Case 8: Empty search string
	result = IsStringInSlices("", []string{"banana", "cherry"}, []string{"apple", "grape"})
	assert.False(t, result, "Expected empty string not to be found in slices")
}