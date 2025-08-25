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

package powerscalefs

import (
	"context"
	"fmt"
	"path"
	"testing"

	isi "github.com/dell/goisilon"
	apiv2 "github.com/dell/goisilon/api/v2"
	"github.com/stretchr/testify/assert"
)

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

func TestGetPathForVolume1(t *testing.T) {
	t.Run("Empty isiPath returns default /ifs/ path", func(t *testing.T) {
		volName := "testVolume"
		expectedPath := path.Join("/ifs/", volName)

		actualPath := GetPathForVolume("", volName)

		assert.Equal(t, expectedPath, actualPath, "Expected default path /ifs/<volName>")
	})
}
