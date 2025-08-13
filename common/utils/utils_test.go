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

// revive:disable:var-naming
package utils

// revive:enable:var-naming

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	isi "github.com/dell/goisilon"
	apiv2 "github.com/dell/goisilon/api/v2"
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

func TestGetNewUUID(t *testing.T) {
	id, err := GetNewUUID()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if _, err := uuid.Parse(id); err != nil {
		t.Errorf("expected valid UUID, got %s", id)
	}
}

func TestLogMap(_ *testing.T) {
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

func TestGetQuotaIDFromDescription(t *testing.T) {
	tests := []struct {
		name        string
		export      isi.Export
		expectedID  string
		expectedErr error
	}{
		{
			name: "Valid Quota ID",
			export: &apiv2.Export{
				ID:          1,
				Paths:       &[]string{"/path/to/export"},
				Description: "CSI_QUOTA_ID:12345",
			},
			expectedID: "12345",
		},
		{
			name: "No Quota ID in Description",
			export: &apiv2.Export{
				ID:          2,
				Paths:       &[]string{"/another/path"},
				Description: "Some user-defined text",
			},
			expectedID: "",
		},
		{
			name: "Empty Description",
			export: &apiv2.Export{
				ID:          3,
				Paths:       &[]string{"/empty/description"},
				Description: "",
			},
			expectedID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			quotaID, err := GetQuotaIDFromDescription(ctx, tt.export)
			assert.Equal(t, tt.expectedID, quotaID)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestParseUnitFromContext(t *testing.T) {
	ctx := context.Background()

	// Test case: Valid integer value
	os.Setenv("TEST_UINT", "42")
	defer os.Unsetenv("TEST_UINT")

	val := ParseUintFromContext(ctx, "TEST_UINT")
	assert.Equal(t, uint(42), val, "Expected 42, got %v", val)

	// Test case: Invalid integer value (error case)
	os.Setenv("TEST_UINT_INVALID", "not_a_number")
	defer os.Unsetenv("TEST_UINT_INVALID")

	val = ParseUintFromContext(ctx, "TEST_UINT_INVALID")
	assert.Equal(t, uint(0), val, "Expected 0 due to parsing error, got %v", val)

	// Test case: Missing environment variable
	val = ParseUintFromContext(ctx, "NON_EXISTENT_KEY")
	assert.Equal(t, uint(0), val, "Expected 0 for non-existent key, got %v", val)
}

func TestRemoveExistingCSISockFile(t *testing.T) {
	const testSockFile = "/tmp/test.sock"
	os.Setenv("CSI_ENDPOINT", testSockFile)
	defer os.Unsetenv("CSI_ENDPOINT")

	// Create a test socket file
	file, err := os.Create(testSockFile)
	if err != nil {
		t.Fatalf("Failed to create test socket file: %v", err)
	}
	file.Close()

	// Test: Successful removal
	err = RemoveExistingCSISockFile()
	assert.NoError(t, err, "Expected no error in normal removal")

	// Ensure file is deleted
	_, err = os.Stat(testSockFile)
	assert.True(t, os.IsNotExist(err), "Expected file to be removed, but it still exists")

	// ---- ERROR CASE ----
	// Create the file again
	file, err = os.Create(testSockFile)
	if err != nil {
		t.Fatalf("Failed to recreate test socket file: %v", err)
	}
	file.Close()

	// Cleanup
	os.Remove(testSockFile)
}

func TestParseBooleanFromContext(t *testing.T) {
	ctx := context.Background()

	// Test Case 1: Valid "true" value
	t.Run("Valid true boolean", func(t *testing.T) {
		os.Setenv("TEST_BOOL", "true")
		defer os.Unsetenv("TEST_BOOL")

		result := ParseBooleanFromContext(ctx, "TEST_BOOL")
		assert.True(t, result, "Expected true")
	})

	// Test Case 2: Valid "false" value
	t.Run("Valid false boolean", func(t *testing.T) {
		os.Setenv("TEST_BOOL", "false")
		defer os.Unsetenv("TEST_BOOL")

		result := ParseBooleanFromContext(ctx, "TEST_BOOL")
		assert.False(t, result, "Expected false")
	})

	// Test Case 3: Invalid boolean value (error condition)
	t.Run("Invalid boolean value", func(t *testing.T) {
		os.Setenv("TEST_BOOL", "notaboolean") // Invalid input
		defer os.Unsetenv("TEST_BOOL")

		result := ParseBooleanFromContext(ctx, "TEST_BOOL")
		assert.False(t, result, "Expected false due to invalid boolean value")
	})

	// Test Case 4: Environment variable not set
	t.Run("Missing environment variable", func(t *testing.T) {
		os.Unsetenv("TEST_BOOL") // Ensure the variable is not set

		result := ParseBooleanFromContext(ctx, "TEST_BOOL")
		assert.False(t, result, "Expected false when the environment variable is missing")
	})
}

func TestParseInt64FromContext1(t *testing.T) {
	ctx := context.Background()

	// Test Case 1: Valid int64 value
	t.Run("Valid int64 value", func(t *testing.T) {
		os.Setenv("TEST_INT", "123456789")
		defer os.Unsetenv("TEST_INT")

		result, err := ParseInt64FromContext(ctx, "TEST_INT")
		assert.NoError(t, err, "Expected no error for valid int64 value")
		assert.Equal(t, int64(123456789), result, "Expected parsed int64 value")
	})

	// Test Case 2: Invalid int64 value (error condition)
	t.Run("Invalid int64 value", func(t *testing.T) {
		os.Setenv("TEST_INT", "notanumber") // Invalid input
		defer os.Unsetenv("TEST_INT")

		result, err := ParseInt64FromContext(ctx, "TEST_INT")
		assert.Error(t, err, "Expected error for invalid int64 value")
		assert.Equal(t, int64(0), result, "Expected default int64 value (0) on error")
	})

	// Test Case 3: Missing environment variable
	t.Run("Missing environment variable", func(t *testing.T) {
		os.Unsetenv("TEST_INT") // Ensure the variable is not set

		result, err := ParseInt64FromContext(ctx, "TEST_INT")
		assert.NoError(t, err, "Expected no error when environment variable is missing")
		assert.Equal(t, int64(0), result, "Expected default int64 value (0) when env variable is not set")
	})
}

func TestGetPathForVolume1(t *testing.T) {
	t.Run("Empty isiPath returns default /ifs/ path", func(t *testing.T) {
		volName := "testVolume"
		expectedPath := path.Join("/ifs/", volName)

		actualPath := GetPathForVolume("", volName)

		assert.Equal(t, expectedPath, actualPath, "Expected default path /ifs/<volName>")
	})
}

func TestParseArrayFromContext1(t *testing.T) {
	ctx := context.Background()

	t.Run("Valid YAML array", func(t *testing.T) {
		arrYAML := "- item1\n- item2\n- item3"
		os.Setenv("TEST_ARRAY", arrYAML)
		defer os.Unsetenv("TEST_ARRAY")

		val, err := ParseArrayFromContext(ctx, "TEST_ARRAY")
		assert.Nil(t, err)
		assert.Equal(t, []string{"item1", "item2", "item3"}, val)
	})

	t.Run("Invalid YAML format", func(t *testing.T) {
		invalidYAML := "{invalid_yaml}"
		os.Setenv("TEST_ARRAY", invalidYAML)
		defer os.Unsetenv("TEST_ARRAY")

		val, err := ParseArrayFromContext(ctx, "TEST_ARRAY")
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid array value for 'TEST_ARRAY'")
		assert.Empty(t, val)
	})

	t.Run("Key not found", func(t *testing.T) {
		os.Unsetenv("TEST_ARRAY")

		val, err := ParseArrayFromContext(ctx, "TEST_ARRAY")
		assert.Nil(t, err)
		assert.Empty(t, val)
	})
}

func TestGetFQDNByIP(t *testing.T) {
	ctx := context.Background()

	t.Run("Valid IP with FQDN", func(t *testing.T) {
		// Use a public IP that is likely to return a valid FQDN
		fqdn, err := GetFQDNByIP(ctx, "8.8.8.8") // Google's public DNS server
		if err == nil {
			assert.NotEmpty(t, fqdn, "Expected a non-empty FQDN")
		}
	})

	t.Run("Invalid IP should return error", func(t *testing.T) {
		fqdn, err := GetFQDNByIP(ctx, "256.256.256.256") // Invalid IP
		assert.Error(t, err, "Expected an error for invalid IP")
		assert.Empty(t, fqdn, "Expected empty FQDN for invalid IP")
	})

	t.Run("Non-resolvable IP should return error", func(t *testing.T) {
		fqdn, err := GetFQDNByIP(ctx, "192.0.2.1") // IP in TEST-NET-1 (unlikely to resolve)
		assert.Error(t, err, "Expected an error for non-resolvable IP")
		assert.Empty(t, fqdn, "Expected empty FQDN for non-resolvable IP")
	})
}

func TestTrimVolumePath(t *testing.T) {
	t.Run("Path with multiple separators", func(t *testing.T) {
		volPath := "/path/to/volume/"
		expected := "/path/to/volume/"
		result := TrimVolumePath(volPath)
		assert.Equal(t, expected, result, "Expected the trimmed path to be '/path/to/volume/'")
	})

	t.Run("Path with single separator", func(t *testing.T) {
		volPath := "/volume/"
		expected := "/volume/"
		result := TrimVolumePath(volPath)
		assert.Equal(t, expected, result, "Expected the trimmed path to be '/volume/'")
	})

	t.Run("Path without separator", func(t *testing.T) {
		volPath := "volume"
		expected := "volume"
		result := TrimVolumePath(volPath)
		assert.Equal(t, expected, result, "Expected the trimmed path to be 'volume'")
	})

	t.Run("Empty path", func(t *testing.T) {
		volPath := ""
		expected := ""
		result := TrimVolumePath(volPath)
		assert.Equal(t, expected, result, "Expected the trimmed path to be an empty string")
	})

	t.Run("Path with underscores", func(t *testing.T) {
		volPath := "/path/to/volume/volume_with_underscores"
		expected := "/path/to/volume/"
		result := TrimVolumePath(volPath)
		assert.Equal(t, expected, result, "Expected the trimmed path to be '/path/to/volume/'")
	})
}

func TestRemoveAuthorizationVolPrefix(t *testing.T) {
	t.Run("Prefix found in volume name", func(t *testing.T) {
		volPrefix := "csi"
		volName := "my-volume-csi-xyz"
		expected := "csi-xyz"
		result, err := RemoveAuthorizationVolPrefix(volPrefix, volName)
		assert.NoError(t, err, "Expected no error")
		assert.Equal(t, expected, result, "Expected the result to be 'csi-xyz'")
	})

	t.Run("Prefix is the first part of the volume name", func(t *testing.T) {
		volPrefix := "tn1"
		volName := "tn1-tn1-xyz"
		expected := "tn1-xyz"
		result, err := RemoveAuthorizationVolPrefix(volPrefix, volName)
		assert.NoError(t, err, "Expected no error")
		assert.Equal(t, expected, result, "Expected the result to be 'tn1-xyz'")
	})

	t.Run("Volume name is empty", func(t *testing.T) {
		volPrefix := "csi"
		volName := ""
		expectedError := "csiVolPrefix csi is not found in the volume name "
		result, err := RemoveAuthorizationVolPrefix(volPrefix, volName)
		assert.Error(t, err, "Expected an error")
		assert.EqualError(t, err, expectedError, "Expected the error message to match")
		assert.Equal(t, "", result, "Expected the result to be an empty string")
	})
}
