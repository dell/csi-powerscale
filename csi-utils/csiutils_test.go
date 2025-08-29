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

package csiutils

import (
	"errors"
	"net"
	"os"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetNFSClientIP(t *testing.T) {
	defaulInterfaceAddrsFn := interfaceAddrs
	defaultParseCIDRFn := parseCIDR

	afterEach := func() {
		interfaceAddrs = defaulInterfaceAddrsFn
		parseCIDR = defaultParseCIDRFn
	}

	tests := []struct {
		name             string
		allowedNetworks  []string
		interfaceAddrsFn func() ([]net.Addr, error)
		parseCIDRFn      func(s string) (net.IP, *net.IPNet, error)
		expectError      bool
	}{
		{
			name: "Valid_Network",
			allowedNetworks: []string{
				"10.247.96.0/21",
				"10.244.0.0/24",
			},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return []net.Addr{
					&net.IPNet{
						IP:   net.IPv4(10, 244, 0, 0),
						Mask: net.CIDRMask(24, 32),
					},
				}, nil
			},
			expectError: false,
		},
		{
			name: "No_Matching_Network",
			allowedNetworks: []string{
				"192.168.1.0/24", // No matching IP in this range
			},
			expectError: true,
		},
		{
			name: "Invalid_CIDR_Format",
			allowedNetworks: []string{
				"10.247.96.999/21", // Invalid CIDR
			},
			expectError: true,
		},
		{
			name: "Invalid_CIDR_Format_Parsing_Error",
			allowedNetworks: []string{
				"invalid_subnet", // This will trigger net.ParseCIDR() failure
			},
			expectError: true,
		},
		{
			name:            "Empty_Network_List",
			allowedNetworks: []string{},
			expectError:     true,
		},
		{
			name:            "Error_getting_network_interfaces",
			allowedNetworks: []string{},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return nil, errors.New("error")
			},
			expectError: true,
		},
		{
			name:            "Error_parsing_cidr",
			allowedNetworks: []string{},
			parseCIDRFn: func(_ string) (net.IP, *net.IPNet, error) {
				return nil, nil, errors.New("error")
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.interfaceAddrsFn != nil {
				interfaceAddrs = tt.interfaceAddrsFn
			}
			if tt.parseCIDRFn != nil {
				parseCIDR = tt.parseCIDRFn
			}
			defer afterEach()

			ip, err := GetNFSClientIP(tt.allowedNetworks)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				t.Logf("Received expected error: %v", err)
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				t.Logf("Detected IP: %s", ip)
			}
		})
	}
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

func TestRemoveExistingCSISockFile(t *testing.T) {
	const testSockFile = "/tmp/test.sock"

	tests := []struct {
		name  string
		setup func()
		want  error
	}{
		{
			name: "remove the sock file",
			setup: func() {
				// set necessary env vars and queue for cleanup after the test run
				os.Setenv(constants.EnvCSIEndpoint, testSockFile)
				t.Cleanup(func() { os.Unsetenv(constants.EnvCSIEndpoint) })

				// Create a test socket file
				file, err := os.Create(testSockFile)
				if err != nil {
					t.Fatalf("Failed to create test socket file: %v", err)
					return
				}
				t.Cleanup(func() {
					file.Close()
					os.Remove(testSockFile)
				})
			},
			want: nil,
		},
		{
			name: "sock file does not exist",
			setup: func() {
				// set necessary env vars
				os.Setenv(constants.EnvCSIEndpoint, testSockFile)
				t.Cleanup(func() { os.Unsetenv(constants.EnvCSIEndpoint) })
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			err := RemoveExistingCSISockFile()
			if err != tt.want {
				t.Errorf("RemoveExistingCSISockFile() error = %v, wantErr %v", err, tt.want)
			}
		})
	}
}

func TestIpInCIDR(t *testing.T) {
	tests := []struct {
		name     string
		ipStr    string
		cidrStr  string
		expected bool
	}{
		{
			name:     "simple IP in CIDR",
			ipStr:    "192.168.1.1",
			cidrStr:  "192.168.1.0/24",
			expected: true,
		},
		{
			name:     "IP not in CIDR",
			ipStr:    "192.168.2.1",
			cidrStr:  "192.168.1.0/24",
			expected: false,
		},
		{
			name:     "invalid IP",
			ipStr:    "256.1.1.1",
			cidrStr:  "192.168.1.0/24",
			expected: false,
		},
		{
			name:     "invalid CIDR",
			ipStr:    "192.168.1.1",
			cidrStr:  "192.168.1.0/33",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := IPInCIDR(tt.ipStr, tt.cidrStr)
			if actual != tt.expected {
				t.Errorf("IPInCIDR(%q, %q) = %v, want %v", tt.ipStr, tt.cidrStr, actual, tt.expected)
			}
		})
	}
}
