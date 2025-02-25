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
	"testing"
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
