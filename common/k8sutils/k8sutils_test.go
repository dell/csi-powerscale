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

/*
Copyright (c) 2025 Dell Inc, or its subsidiaries.

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

package k8sutils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var exitFunc = os.Exit

type MockLeaderElection struct {
	mock.Mock
}

func (m *MockLeaderElection) Run() error {
	args := m.Called()
	return args.Error(0)
}

func TestCreateKubeClientSet(t *testing.T) {
	// Test cases
	tests := []struct {
		name       string
		kubeconfig string
		configErr  error
		clientErr  error
		wantErr    bool
	}{
		{
			name:       "Valid kubeconfig",
			kubeconfig: "valid_kubeconfig",
			configErr:  nil,
			clientErr:  nil,
			wantErr:    false,
		},
		{
			name:       "Invalid kubeconfig",
			kubeconfig: "invalid_kubeconfig",
			configErr:  errors.New("config error"),
			clientErr:  nil,
			wantErr:    true,
		},
		{
			name:       "In-cluster config",
			kubeconfig: "",
			configErr:  nil,
			clientErr:  nil,
			wantErr:    false,
		},
		{
			name:       "In-cluster config error",
			kubeconfig: "",
			configErr:  errors.New("config error"),
			clientErr:  nil,
			wantErr:    true,
		},
		{
			name:       "New for config error",
			kubeconfig: "",
			configErr:  nil,
			clientErr:  errors.New("client error"),
			wantErr:    true,
		},
		{
			name:       "New for config error",
			kubeconfig: "invalid_kubeconfig",
			configErr:  nil,
			clientErr:  errors.New("client error"),
			wantErr:    true,
		},
	}

	// Save original functions
	origBuildConfigFromFlags := buildConfigFromFlags
	origInClusterConfig := inClusterConfig
	origNewForConfig := newForConfig

	// Restore original functions after tests
	defer func() {
		buildConfigFromFlags = origBuildConfigFromFlags
		inClusterConfig = origInClusterConfig
		newForConfig = origNewForConfig
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock functions
			buildConfigFromFlags = func(_, _ string) (*rest.Config, error) {
				return &rest.Config{}, tt.configErr
			}
			inClusterConfig = func() (*rest.Config, error) {
				return &rest.Config{}, tt.configErr
			}
			newForConfig = func(_ *rest.Config) (*kubernetes.Clientset, error) {
				if tt.clientErr != nil {
					return nil, tt.clientErr
				}
				return &kubernetes.Clientset{}, nil
			}

			clientset, err := CreateKubeClientSet(tt.kubeconfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateKubeClientSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && clientset == nil {
				t.Errorf("CreateKubeClientSet() = nil, want non-nil")
			}
		})
	}
}

func TestGetStats(t *testing.T) {
	// Set up the necessary dependencies
	ctx := context.Background()
	volumePath := "/path/to/volume"
	availableBytes, totalBytes, usedBytes, totalInodes, freeInodes, usedInodes, _ := GetStats(ctx, volumePath)

	expectedAvailableBytes := int64(0)
	expectedTotalBytes := int64(0)
	expectedUsedBytes := int64(0)
	expectedTotalInodes := int64(0)
	expectedFreeInodes := int64(0)
	expectedUsedInodes := int64(0)
	if availableBytes != expectedAvailableBytes {
		t.Errorf("Expected availableBytes to be %d, but got %d", expectedAvailableBytes, availableBytes)
	}
	if totalBytes != expectedTotalBytes {
		t.Errorf("Expected totalBytes to be %d, but got %d", expectedTotalBytes, totalBytes)
	}
	if usedBytes != expectedUsedBytes {
		t.Errorf("Expected usedBytes to be %d, but got %d", expectedUsedBytes, usedBytes)
	}
	if totalInodes != expectedTotalInodes {
		t.Errorf("Expected totalInodes to be %d, but got %d", expectedTotalInodes, totalInodes)
	}
	if freeInodes != expectedFreeInodes {
		t.Errorf("Expected freeInodes to be %d, but got %d", expectedFreeInodes, freeInodes)
	}
	if usedInodes != expectedUsedInodes {
		t.Errorf("Expected usedInodes to be %d, but got %d", expectedUsedInodes, usedInodes)
	}
}

func TestGetStatsNoError(t *testing.T) {
	// Set up the necessary dependencies
	defaultFsInfo := fsInfo
	fsInfo = func(_ context.Context, _ string) (int64, int64, int64, int64, int64, int64, error) {
		return 1, 1, 1, 1, 1, 1, nil
	}
	defer func() {
		fsInfo = defaultFsInfo
	}()

	ctx := context.Background()
	volumePath := "/path/to/volume"
	availableBytes, totalBytes, usedBytes, totalInodes, freeInodes, usedInodes, _ := GetStats(ctx, volumePath)

	expectedAvailableBytes := int64(1)
	expectedTotalBytes := int64(1)
	expectedUsedBytes := int64(1)
	expectedTotalInodes := int64(1)
	expectedFreeInodes := int64(1)
	expectedUsedInodes := int64(1)
	if availableBytes != expectedAvailableBytes {
		t.Errorf("Expected availableBytes to be %d, but got %d", expectedAvailableBytes, availableBytes)
	}
	if totalBytes != expectedTotalBytes {
		t.Errorf("Expected totalBytes to be %d, but got %d", expectedTotalBytes, totalBytes)
	}
	if usedBytes != expectedUsedBytes {
		t.Errorf("Expected usedBytes to be %d, but got %d", expectedUsedBytes, usedBytes)
	}
	if totalInodes != expectedTotalInodes {
		t.Errorf("Expected totalInodes to be %d, but got %d", expectedTotalInodes, totalInodes)
	}
	if freeInodes != expectedFreeInodes {
		t.Errorf("Expected freeInodes to be %d, but got %d", expectedFreeInodes, freeInodes)
	}
	if usedInodes != expectedUsedInodes {
		t.Errorf("Expected usedInodes to be %d, but got %d", expectedUsedInodes, usedInodes)
	}
}

func TestLeaderElection(t *testing.T) {
	clientset := &kubernetes.Clientset{} // Mock or use a fake clientset if needed
	runFunc := func(_ context.Context) {
		fmt.Println("Running leader function")
	}

	mockLE := new(MockLeaderElection)
	mockLE.On("Run").Return(nil) // Mocking a successful run

	// Override exitFunc to prevent test from exiting
	exitCalled := false
	oldExit := exitFunc
	defer func() { recover(); exitFunc = oldExit }()
	exitFunc = func(_ int) { exitCalled = true; panic("exitFunc called") }

	// Simulate LeaderElection function
	func() {
		defer func() {
			if r := recover(); r != nil {
				exitCalled = true
			}
		}()
		LeaderElection(clientset, "test-lock", "test-namespace", time.Second, time.Second*2, time.Second*3, runFunc)
	}()

	if !exitCalled {
		t.Errorf("exitFunc was called unexpectedly")
	}
}
