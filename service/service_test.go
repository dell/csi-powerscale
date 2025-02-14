package service

/*
 Copyright (c) 2019-2021 Dell Inc, or its subsidiaries.

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
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof" // #nosec G108
	"os"
	"sync"
	"testing"

	"github.com/cucumber/godog"
	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	status := 0

	go http.ListenAndServe("localhost:6060", nil) // #nosec G114
	fmt.Printf("starting godog...\n")

	configFile := "mock/secret/secret.yaml"
	os.Setenv(constants.EnvIsilonConfigFile, configFile)

	opts := godog.Options{
		Format: "pretty",
		Paths:  []string{"features"},
		Tags:   "~todo",
	}

	status = godog.TestSuite{
		Name:                "godogs",
		ScenarioInitializer: FeatureContext,
		Options:             &opts,
	}.Run()

	fmt.Printf("godog finished\n")

	if st := m.Run(); st > status {
		status = st
	}

	fmt.Printf("status %d\n", status)

	os.Exit(status)
}

func TestGetLoggerfunc(t *testing.T) {
	ctx, _ := GetLogger(nil)
	assert.Equal(t, nil, ctx)
}

func TestGetRunIDLogfunc(t *testing.T) {
	ctx, _, _ := GetRunIDLog(nil)
	assert.Equal(t, nil, ctx)
}

func TestGetIsiPathForVolumeFromClusterConfig(t *testing.T) {
	o := Opts{
		Path: "path",
	}
	s := service{
		opts: o,
	}

	isilonConfig := IsilonClusterConfig{
		IsiPath: "",
	}
	path := s.getIsiPathForVolumeFromClusterConfig(&isilonConfig)
	assert.Equal(t, "path", path)

	isilonConfig = IsilonClusterConfig{
		IsiPath: "path/path",
	}
	path = s.getIsiPathForVolumeFromClusterConfig(&isilonConfig)
	assert.Equal(t, "path/path", path)
}

func TestGetCSINodeIP(t *testing.T) {
	s := service{
		nodeIP: "",
	}
	ctx := context.Background()
	_, err := s.GetCSINodeIP(ctx)
	assert.Equal(t, errors.New("cannot get node IP"), err)
}

func TestGetCSINodeID(t *testing.T) {
	s := service{
		nodeID: "",
	}
	ctx := context.Background()
	_, err := s.GetCSINodeID(ctx)
	assert.Equal(t, errors.New("cannot get node id"), err)
}

func TestGetIsilonClusterLength(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*sync.Map)
		expected  int
	}{
		{
			name: "Empty Map",
			setupFunc: func(m *sync.Map) {
				// No setup, map remains empty
			},
			expected: 0,
		},
		{
			name: "Single Entry",
			setupFunc: func(m *sync.Map) {
				m.Store("cluster1", struct{}{})
			},
			expected: 1,
		},
		{
			name: "Multiple Entries",
			setupFunc: func(m *sync.Map) {
				m.Store("cluster1", struct{}{})
				m.Store("cluster2", struct{}{})
				m.Store("cluster3", struct{}{})
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serviceInstance := &service{
				isiClusters: &sync.Map{},
			}

			tt.setupFunc(serviceInstance.isiClusters)
			actual := serviceInstance.getIsilonClusterLength()

			if actual != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, actual)
			}
		})
	}
}

func TestGetNodeLabels(t *testing.T) {
	s := service{
		nodeID: "",
	}
	_, err := s.GetNodeLabels()
	assert.NotEqual(t, nil, err)
}

func TestServiceInitializeServiceOpts(t *testing.T) {
	wantOps := Opts{
		Port:                     "8080",
		Path:                     "/ifs",
		IsiVolumePathPermissions: "0777",
		AccessZone:               "System",
		KubeConfigPath:           "/home/kubeconfig",
		replicationContextPrefix: "prefix/",
		replicationPrefix:        "prefix",
		IgnoreUnresolvableHosts:  false,
	}

	wantEnvNodeName := "node"
	wantEnvNodeIP := "10.0.0.1"
	wantEnvIsilonConfigFile := "X_CSI_ISI_CONFIG_PATH"

	os.Setenv(constants.EnvPort, wantOps.Port)
	os.Setenv(constants.EnvPath, "")
	os.Setenv(constants.EnvIsiVolumePathPermissions, "")
	os.Setenv(constants.EnvAccessZone, "")
	os.Setenv(constants.EnvNodeName, wantEnvNodeName)
	os.Setenv(constants.EnvNodeIP, wantEnvNodeIP)
	os.Setenv(constants.EnvKubeConfigPath, wantOps.KubeConfigPath)
	os.Setenv(constants.EnvIsilonConfigFile, wantEnvIsilonConfigFile)
	os.Setenv(constants.EnvReplicationContextPrefix, "prefix")
	os.Setenv(constants.EnvReplicationPrefix, wantOps.replicationPrefix)

	defer func() {
		os.Unsetenv(constants.EnvPort)
		os.Unsetenv(constants.EnvPath)
		os.Unsetenv(constants.EnvIsiVolumePathPermissions)
		os.Unsetenv(constants.EnvAccessZone)
		os.Unsetenv(constants.EnvNodeName)
		os.Unsetenv(constants.EnvNodeIP)
		os.Unsetenv(constants.EnvKubeConfigPath)
		os.Unsetenv(constants.EnvIsilonConfigFile)
		os.Unsetenv(constants.EnvReplicationContextPrefix)
		os.Unsetenv(constants.EnvReplicationPrefix)
	}()

	serviceInstance := &service{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = context.WithValue(ctx, constants.EnvMaxVolumesPerNode, "test")

	serviceInstance.initializeServiceOpts(ctx)

	assert.Equal(t, wantOps, serviceInstance.opts)
	assert.Equal(t, wantEnvNodeName, serviceInstance.nodeID)
	assert.Equal(t, wantEnvNodeIP, serviceInstance.nodeIP)
}
