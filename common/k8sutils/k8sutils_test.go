package k8sutils

import (
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
