package k8sutils

import (
	"context"
	"errors"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

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
