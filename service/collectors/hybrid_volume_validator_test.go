/*
 Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.

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

package collectors

import (
	"context"
	"fmt"
	"testing"
)

// MockK8sValidator is a mock implementation of VolumeValidator for testing
type MockK8sValidator struct {
	shouldReturn bool
	shouldError  bool
}

func (m *MockK8sValidator) IsDriverManaged(_ context.Context, _ string) (bool, error) {
	if m.shouldError {
		return false, fmt.Errorf("mock K8s validation error")
	}
	return m.shouldReturn, nil
}

func (m *MockK8sValidator) RefreshCache(_ context.Context) error {
	if m.shouldError {
		return fmt.Errorf("mock K8s refresh cache error")
	}
	return nil
}

func TestHybridVolumeValidator_PathBasedOnly(t *testing.T) {
	validator := NewHybridVolumeValidator("/ifs/data/csi")

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Volume under CSI path",
			path:     "/ifs/data/csi/csivol-abc123",
			expected: true,
		},
		{
			name:     "Volume under CSI path with nested dirs",
			path:     "/ifs/data/csi/subdir/csivol-abc123",
			expected: true,
		},
		{
			name:     "Volume not under CSI path",
			path:     "/ifs/data/manual-volume",
			expected: false,
		},
		{
			name:     "Volume in different path",
			path:     "/ifs/other/csivol-abc123",
			expected: false,
		},
		{
			name:     "Empty path",
			path:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.IsDriverManaged(context.Background(), tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHybridVolumeValidator_WithK8sValidation(t *testing.T) {
	mockK8s := &MockK8sValidator{shouldReturn: true, shouldError: false}
	validator := NewHybridVolumeValidatorWithK8s("/ifs/data/csi", mockK8s)

	tests := []struct {
		name        string
		path        string
		k8sReturn   bool
		k8sError    bool
		expected    bool
		expectError bool
	}{
		{
			name:        "Volume under CSI path and K8s validation passes",
			path:        "/ifs/data/csi/csivol-abc123",
			k8sReturn:   true,
			k8sError:    false,
			expected:    true,
			expectError: false,
		},
		{
			name:        "Volume under CSI path but K8s validation fails",
			path:        "/ifs/data/csi/csivol-abc123",
			k8sReturn:   false,
			k8sError:    false,
			expected:    false,
			expectError: false,
		},
		{
			name:        "Volume under CSI path but K8s validation errors (falls back to path-based)",
			path:        "/ifs/data/csi/csivol-abc123",
			k8sReturn:   false,
			k8sError:    true,
			expected:    true,
			expectError: false,
		},
		{
			name:        "Volume not under CSI path (K8s validation not called)",
			path:        "/ifs/data/manual-volume",
			k8sReturn:   true,
			k8sError:    false,
			expected:    false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockK8s.shouldReturn = tt.k8sReturn
			mockK8s.shouldError = tt.k8sError

			result, err := validator.IsDriverManaged(context.Background(), tt.path)

			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHybridVolumeValidator_K8sValidationToggle(t *testing.T) {
	mockK8s := &MockK8sValidator{shouldReturn: true, shouldError: false}
	validator := NewHybridVolumeValidator("/ifs/data/csi")

	// Initially K8s validation is disabled
	if validator.IsK8sValidationEnabled() {
		t.Errorf("expected K8s validation to be disabled initially")
	}

	// Enable K8s validation
	validator.SetK8sValidator(mockK8s)
	if !validator.IsK8sValidationEnabled() {
		t.Errorf("expected K8s validation to be enabled after SetK8sValidator")
	}

	// Disable K8s validation
	validator.DisableK8sValidation()
	if validator.IsK8sValidationEnabled() {
		t.Errorf("expected K8s validation to be disabled after DisableK8sValidation")
	}
}

func TestHybridVolumeValidator_NilK8sValidator(t *testing.T) {
	validator := NewHybridVolumeValidatorWithK8s("/ifs/data/csi", nil)

	// Should not enable K8s validation if nil validator is passed
	if validator.IsK8sValidationEnabled() {
		t.Errorf("expected K8s validation to be disabled with nil validator")
	}

	// Should still work with path-based filtering
	result, err := validator.IsDriverManaged(context.Background(), "/ifs/data/csi/csivol-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Errorf("expected true for path-based filtering")
	}
}

func TestHybridVolumeValidator_RefreshCache(t *testing.T) {
	mockK8s := &MockK8sValidator{shouldReturn: true, shouldError: false}
	validator := NewHybridVolumeValidatorWithK8s("/ifs/data/csi", mockK8s)

	// Refresh cache with K8s validation enabled
	err := validator.RefreshCache(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Refresh cache with K8s validation disabled
	validator.DisableK8sValidation()
	err = validator.RefreshCache(context.Background())
	if err != nil {
		t.Fatalf("unexpected error with K8s validation disabled: %v", err)
	}

	// Refresh cache with K8s validation error
	mockK8s.shouldError = true
	validator.SetK8sValidator(mockK8s)
	err = validator.RefreshCache(context.Background())
	if err == nil {
		t.Errorf("expected error when K8s validator refresh fails")
	}
}

func TestHybridVolumeValidator_RefreshCache_PathBasedOnly(t *testing.T) {
	validator := NewHybridVolumeValidator("/ifs/data/csi")

	// Refresh cache with path-based only (no K8s validator)
	err := validator.RefreshCache(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHybridVolumeValidator_RefreshCache_EmptyPath(t *testing.T) {
	validator := NewHybridVolumeValidator("")

	// Refresh cache with empty path
	err := validator.RefreshCache(context.Background())
	if err != nil {
		t.Fatalf("unexpected error with empty path: %v", err)
	}
}

func TestHybridVolumeValidator_DifferentCSIPaths(t *testing.T) {
	tests := []struct {
		name     string
		csiPath  string
		testPath string
		expected bool
	}{
		{
			name:     "Custom CSI path",
			csiPath:  "/custom/csi/path",
			testPath: "/custom/csi/path/vol-123",
			expected: true,
		},
		{
			name:     "Root CSI path",
			csiPath:  "/",
			testPath: "/vol-123",
			expected: true,
		},
		{
			name:     "Nested CSI path",
			csiPath:  "/ifs/data/csi/volumes",
			testPath: "/ifs/data/csi/volumes/subdir/vol-123",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewHybridVolumeValidator(tt.csiPath)
			result, err := validator.IsDriverManaged(context.Background(), tt.testPath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHybridVolumeValidator_RefreshCache_MultipleCalls(t *testing.T) {
	mockK8s := &MockK8sValidator{shouldReturn: true, shouldError: false}
	validator := NewHybridVolumeValidatorWithK8s("/ifs/data/csi", mockK8s)

	// Multiple refresh calls should work
	for i := 0; i < 3; i++ {
		err := validator.RefreshCache(context.Background())
		if err != nil {
			t.Fatalf("unexpected error on refresh %d: %v", i, err)
		}
	}
}

func TestHybridVolumeValidator_RefreshCache_K8sStaleData(t *testing.T) {
	mockK8s := &MockK8sValidator{shouldReturn: true, shouldError: false}
	validator := NewHybridVolumeValidatorWithK8s("/ifs/data/csi", mockK8s)

	// First refresh
	err := validator.RefreshCache(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on first refresh: %v", err)
	}

	// Second refresh with different return value
	mockK8s.shouldReturn = false
	err = validator.RefreshCache(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on second refresh: %v", err)
	}
}

func TestHybridVolumeValidator_RefreshCache_Concurrent(t *testing.T) {
	mockK8s := &MockK8sValidator{shouldReturn: true, shouldError: false}
	validator := NewHybridVolumeValidatorWithK8s("/ifs/data/csi", mockK8s)

	// Concurrent refresh calls
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func() {
			err := validator.RefreshCache(context.Background())
			if err != nil {
				t.Errorf("unexpected error in concurrent refresh: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestHybridVolumeValidator_RefreshCache_K8sError(t *testing.T) {
	mockK8s := &MockK8sValidator{shouldReturn: false, shouldError: true}
	validator := NewHybridVolumeValidatorWithK8s("/ifs/data/csi", mockK8s)

	// RefreshCache should return error when K8s refresh fails
	err := validator.RefreshCache(context.Background())
	if err == nil {
		t.Fatal("expected error on refresh with K8s error, got nil")
	}
}

func TestHybridVolumeValidator_IsDriverManaged_AfterRefresh(t *testing.T) {
	mockK8s := &MockK8sValidator{shouldReturn: true, shouldError: false}
	validator := NewHybridVolumeValidatorWithK8s("/ifs/data/csi", mockK8s)

	// Refresh cache first
	err := validator.RefreshCache(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on refresh: %v", err)
	}

	// Now check if path is driver managed
	isManaged, err := validator.IsDriverManaged(context.Background(), "/ifs/data/csi/vol1")
	if err != nil {
		t.Fatalf("unexpected error on IsDriverManaged: %v", err)
	}
	if !isManaged {
		t.Errorf("expected path to be driver managed after refresh")
	}
}

// U-HYBRID-REFRESH-NIL-K8S-VALIDATOR: RefreshCache with nil k8sValidator but K8s validation enabled
func TestHybridVolumeValidator_RefreshCache_NilK8sValidatorEnabled(t *testing.T) {
	validator := NewHybridVolumeValidator("/ifs/data/csi")

	// Manually enable K8s validation without setting a validator
	// This tests the path where enableK8sValidation is true but k8sValidator is nil
	// Since we can't directly set enableK8sValidation, we'll test with SetK8sValidator(nil)
	validator.SetK8sValidator(nil)

	// Should not panic and should return nil (no-op)
	err := validator.RefreshCache(context.Background())
	if err != nil {
		t.Fatalf("unexpected error with nil K8s validator: %v", err)
	}
}
