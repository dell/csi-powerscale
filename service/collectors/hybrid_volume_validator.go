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
	"strings"

	"github.com/Ecosystems/container-storage-modules/src/csmlog"
)

// VolumeValidator is an interface for validating if a volume is driver-managed
type VolumeValidator interface {
	IsDriverManaged(ctx context.Context, path string) (bool, error)
	RefreshCache(ctx context.Context) error
}

// HybridVolumeValidator combines path-based filtering (fast) with optional
// Kubernetes validation (accurate) to identify driver-managed volumes.
//
// Flow:
//  1. Fast path filter: Check if volume is under the configured isiPath
//  2. Optional K8s validation: If enabled, verify volume exists in Kubernetes
//     with the correct CSI driver provisioner
type HybridVolumeValidator struct {
	isiPath             string
	k8sValidator        VolumeValidator
	enableK8sValidation bool
}

// NewHybridVolumeValidator creates a new HybridVolumeValidator with path-based filtering only
func NewHybridVolumeValidator(isiPath string) *HybridVolumeValidator {
	return &HybridVolumeValidator{
		isiPath:             isiPath,
		enableK8sValidation: false,
	}
}

// NewHybridVolumeValidatorWithK8s creates a new HybridVolumeValidator with both
// path-based and Kubernetes validation
func NewHybridVolumeValidatorWithK8s(isiPath string, k8sValidator VolumeValidator) *HybridVolumeValidator {
	return &HybridVolumeValidator{
		isiPath:             isiPath,
		k8sValidator:        k8sValidator,
		enableK8sValidation: k8sValidator != nil,
	}
}

// IsDriverManaged checks if a volume is driver-managed using the hybrid approach
func (v *HybridVolumeValidator) IsDriverManaged(ctx context.Context, path string) (bool, error) {
	// Step 1: Fast path-based filtering
	if !strings.HasPrefix(path, v.isiPath) {
		csmlog.Debugf("HybridVolumeValidator: path %s does not match isiPath %s", path, v.isiPath)
		return false, nil
	}

	// Step 2: Optional Kubernetes validation for additional accuracy
	if v.enableK8sValidation && v.k8sValidator != nil {
		csmlog.Debugf("HybridVolumeValidator: K8s validation enabled, checking path %s", path)
		isManaged, err := v.k8sValidator.IsDriverManaged(ctx, path)
		if err != nil {
			// K8s validation failed, fall back to path-based filtering
			csmlog.Warnf("HybridVolumeValidator: K8s validation failed for path %s, falling back to path-based filtering: %v", path, err)
			// Path-based filtering is sufficient for identifying driver-managed volumes
			return true, nil
		}
		csmlog.Debugf("HybridVolumeValidator: K8s validation result for path %s: isManaged=%v", path, isManaged)
		return isManaged, nil
	}

	// If K8s validation is disabled, path-based filtering is sufficient
	csmlog.Debugf("HybridVolumeValidator: K8s validation disabled, using path-based filtering for path %s", path)
	return true, nil
}

// SetK8sValidator enables Kubernetes validation after creation
func (v *HybridVolumeValidator) SetK8sValidator(k8sValidator VolumeValidator) {
	if k8sValidator != nil {
		v.k8sValidator = k8sValidator
		v.enableK8sValidation = true
	}
}

// DisableK8sValidation disables Kubernetes validation
func (v *HybridVolumeValidator) DisableK8sValidation() {
	v.enableK8sValidation = false
}

// IsK8sValidationEnabled returns whether K8s validation is currently enabled
func (v *HybridVolumeValidator) IsK8sValidationEnabled() bool {
	return v.enableK8sValidation
}

// RefreshCache refreshes the validator's cache (implements VolumeValidator interface)
func (v *HybridVolumeValidator) RefreshCache(ctx context.Context) error {
	if v.enableK8sValidation && v.k8sValidator != nil {
		// Call the K8s validator's RefreshCache which internally calls fetchAndCachePVs
		if k8sChecker, ok := v.k8sValidator.(*K8sMetadataChecker); ok {
			return k8sChecker.RefreshCache(ctx)
		}
		return v.k8sValidator.RefreshCache(ctx)
	}
	return nil
}
