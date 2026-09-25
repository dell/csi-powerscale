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

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeAddr implements net.Addr but is neither *net.IPNet nor *net.IPAddr,
// used to exercise the defensive "default" branch of GetNFSClientIP's type
// switch (e.g. a hypothetical *net.UnixAddr-like value should never cause a
// panic and must simply be skipped).
type fakeAddr struct{}

func (fakeAddr) Network() string { return "fake" }
func (fakeAddr) String() string  { return "fake-addr" }

func TestExtractIPFromAddr(t *testing.T) {
	tests := []struct {
		name     string
		addr     net.Addr
		expected net.IP
	}{
		{
			name:     "IPNet",
			addr:     &net.IPNet{IP: net.IPv4(10, 0, 0, 1), Mask: net.CIDRMask(24, 32)},
			expected: net.IPv4(10, 0, 0, 1),
		},
		{
			name:     "IPAddr",
			addr:     &net.IPAddr{IP: net.IPv4(10, 0, 0, 2)},
			expected: net.IPv4(10, 0, 0, 2),
		},
		{
			name:     "Unsupported_Type",
			addr:     fakeAddr{},
			expected: nil,
		},
		{
			name:     "Nil",
			addr:     nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractIPFromAddr(tt.addr)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetNFSClientIP(t *testing.T) {
	defaulInterfaceAddrsFn := interfaceAddrs

	afterEach := func() {
		interfaceAddrs = defaulInterfaceAddrsFn
	}

	tests := []struct {
		name                string
		allowedNetworks     []string
		interfaceAddrsFn    func() ([]net.Addr, error)
		expectError         bool
		expectedIP          string
		expectedErrMsg      string
		expectedErrMsgParts []string
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
			expectedIP:  "10.244.0.0",
		},
		{
			name: "No_Matching_Network",
			allowedNetworks: []string{
				"192.168.1.0/24", // No matching IP in this range
			},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return []net.Addr{
					&net.IPNet{
						IP:   net.IPv4(10, 244, 0, 55),
						Mask: net.CIDRMask(24, 32),
					},
				}, nil
			},
			expectError: true,
		},
		{
			name: "Invalid_CIDR_Format_All_Invalid",
			allowedNetworks: []string{
				"10.247.96.999/21", // Invalid CIDR
			},
			expectError:    true,
			expectedErrMsg: "no valid networks in allowedNetworks",
		},
		{
			name: "Invalid_CIDR_Format_Parsing_Error_All_Invalid",
			allowedNetworks: []string{
				"invalid_subnet", // This will trigger net.ParseCIDR() failure
			},
			expectError:    true,
			expectedErrMsg: "no valid networks in allowedNetworks",
		},
		{
			name: "Mixed_Valid_Invalid_CIDRs",
			allowedNetworks: []string{
				"10.244.0.0/24",  // Valid CIDR
				"invalid_subnet", // Invalid CIDR - should be skipped with warning
				"10.244.1.0/24",  // Valid CIDR
			},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return []net.Addr{
					&net.IPNet{
						IP:   net.IPv4(10, 244, 0, 55),
						Mask: net.CIDRMask(24, 32),
					},
				}, nil
			},
			expectError: false,
			expectedIP:  "10.244.0.55",
		},
		{
			name:            "Empty_Network_List",
			allowedNetworks: []string{},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return []net.Addr{
					&net.IPNet{
						IP:   net.IPv4(10, 244, 0, 55),
						Mask: net.CIDRMask(24, 32),
					},
				}, nil
			},
			expectError:    true,
			expectedErrMsg: "no valid networks in allowedNetworks",
		},
		{
			name:            "Nil_Network_List",
			allowedNetworks: nil,
			expectError:     true,
			expectedErrMsg:  "allowedNetworks parameter cannot be nil",
		},
		{
			name:            "Error_getting_network_interfaces",
			allowedNetworks: []string{"10.0.0.0/24"},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return nil, errors.New("error")
			},
			expectError: true,
		},
		{
			name: "IP_Containment_Different_Subnet_Masks_ECS01G_1144",
			allowedNetworks: []string{
				"10.244.0.0/16", // Configured as /16
			},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return []net.Addr{
					&net.IPNet{
						IP:   net.IPv4(10, 244, 0, 55), // OS reports /24
						Mask: net.CIDRMask(24, 32),
					},
				}, nil
			},
			expectError: false, // Should match because 10.244.0.55 is within 10.244.0.0/16
			expectedIP:  "10.244.0.55",
		},
		{
			name: "Enhanced_Error_Message_With_Discovered_IPs_ECS01G_1144",
			allowedNetworks: []string{
				"192.168.1.0/24", // No matching IP in this range
			},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return []net.Addr{
					&net.IPNet{
						IP:   net.IPv4(10, 244, 0, 55),
						Mask: net.CIDRMask(24, 32),
					},
					&net.IPNet{
						IP:   net.IPv4(10, 244, 0, 56),
						Mask: net.CIDRMask(24, 32),
					},
				}, nil
			},
			expectError:    true,
			expectedErrMsg: "; discovered IPs:",
			expectedErrMsgParts: []string{
				"10.244.0.55",
				"10.244.0.56",
			},
		},
		{
			name: "IPAddr_Type_Handling_ECS01G_1144",
			allowedNetworks: []string{
				"10.244.0.0/24",
			},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return []net.Addr{
					&net.IPAddr{
						IP: net.IPv4(10, 244, 0, 55),
					},
				}, nil
			},
			expectError: false,
			expectedIP:  "10.244.0.55",
		},
		{
			// Real interface data captured live from Kubernetes cluster node
			// worker-1-kizaqj1qkbiya.domain via:
			//   kubectl debug node/worker-1-kizaqj1qkbiya.domain -it --image=busybox:1.36 \
			//     -- sh -c "chroot /host ip -4 -o addr show"
			// which reported three NICs (ens161, ens192, ens256) each with a real
			// OS-assigned /21 mask: 10.247.101.41/21, 10.247.101.39/21, 10.247.101.42/21.
			// allowedNetworks below is deliberately configured with a broader /16 mask
			// (rather than the real /21), reproducing the exact ECS01G-1144 scenario
			// against real production node IP addresses.
			name: "Real_Cluster_Node_Worker1_Mask_Mismatch_ECS01G_1144",
			allowedNetworks: []string{
				"10.247.0.0/16",
			},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return []net.Addr{
					&net.IPNet{IP: net.IPv4(10, 247, 101, 41), Mask: net.CIDRMask(21, 32)}, // ens161
					&net.IPNet{IP: net.IPv4(10, 247, 101, 39), Mask: net.CIDRMask(21, 32)}, // ens192
					&net.IPNet{IP: net.IPv4(10, 247, 101, 42), Mask: net.CIDRMask(21, 32)}, // ens256
				}, nil
			},
			expectError: false,
			expectedIP:  "10.247.101.41",
		},
		{
			// Real interface data captured live from worker-2-kizaqj1qkbiya.domain
			// (same kubectl debug node method as above): ens161 10.247.101.43/21,
			// ens192 10.247.101.40/21, ens256 10.247.101.44/21. allowedNetworks is
			// configured with a broader /19 (rather than the real /21).
			name: "Real_Cluster_Node_Worker2_Mask_Mismatch_ECS01G_1144",
			allowedNetworks: []string{
				"10.247.96.0/19",
			},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return []net.Addr{
					&net.IPNet{IP: net.IPv4(10, 247, 101, 43), Mask: net.CIDRMask(21, 32)}, // ens161
					&net.IPNet{IP: net.IPv4(10, 247, 101, 40), Mask: net.CIDRMask(21, 32)}, // ens192
					&net.IPNet{IP: net.IPv4(10, 247, 101, 44), Mask: net.CIDRMask(21, 32)}, // ens256
				}, nil
			},
			expectError: false,
			expectedIP:  "10.247.101.43",
		},
		{
			// Exercises the ip.To4() == nil "continue" branch: an IPv6-only
			// interface must be skipped (not matched, not added to
			// discoveredIPs). The error message will include an empty
			// discoveredIPs list since no IPv4 address was found.
			name: "IPv6_Only_Address_Excluded",
			allowedNetworks: []string{
				"10.244.0.0/16",
			},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return []net.Addr{
					&net.IPNet{
						IP:   net.ParseIP("fe80::1"),
						Mask: net.CIDRMask(64, 128),
					},
				}, nil
			},
			expectError:    true,
			expectedErrMsg: "; discovered IPs: []",
		},
		{
			// Exercises the defensive "default" branch of the type switch:
			// an address type that is neither *net.IPNet nor *net.IPAddr
			// must be safely skipped without panicking.
			name: "Unsupported_Addr_Type_Skipped",
			allowedNetworks: []string{
				"10.244.0.0/16",
			},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return []net.Addr{
					fakeAddr{},
					&net.IPNet{
						IP:   net.IPv4(10, 244, 0, 55),
						Mask: net.CIDRMask(24, 32),
					},
				}, nil
			},
			expectError: false,
			expectedIP:  "10.244.0.55",
		},
		{
			// Exercises match-precedence/iteration order: neither the first
			// interface address nor the first allowedNetworks entry matches
			// each other, but the SECOND interface address matches the
			// SECOND allowedNetworks entry. This pins down that the nested
			// loops correctly continue scanning rather than short-circuiting
			// or incorrectly matching on the first (non-matching) pair.
			name: "Second_Address_Matches_Second_Network",
			allowedNetworks: []string{
				"192.168.1.0/24",
				"10.244.0.0/16",
			},
			interfaceAddrsFn: func() ([]net.Addr, error) {
				return []net.Addr{
					&net.IPNet{IP: net.IPv4(172, 16, 0, 5), Mask: net.CIDRMask(16, 32)}, // matches neither network
					&net.IPNet{IP: net.IPv4(10, 244, 5, 9), Mask: net.CIDRMask(24, 32)}, // matches only the second network
				}, nil
			},
			expectError: false,
			expectedIP:  "10.244.5.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.interfaceAddrsFn != nil {
				interfaceAddrs = tt.interfaceAddrsFn
			}
			defer afterEach()

			ip, err := GetNFSClientIP(tt.allowedNetworks)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
				for _, part := range tt.expectedErrMsgParts {
					assert.Contains(t, err.Error(), part)
				}
				t.Logf("Received expected error: %v", err)
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if tt.expectedIP != "" && ip != tt.expectedIP {
					t.Fatalf("Expected IP %s but got %s", tt.expectedIP, ip)
				}
				t.Logf("Detected IP: %s", ip)
			}
		})
	}
}

func TestGetAllNFSClientIPs(t *testing.T) {
	defaulInterfaceAddrsFn := interfaceAddrs

	afterEach := func() {
		interfaceAddrs = defaulInterfaceAddrsFn
	}

	t.Run("Multiple match", func(t *testing.T) {
		interfaceAddrs = func() ([]net.Addr, error) {
			return []net.Addr{
				&net.IPNet{IP: net.IPv4(10, 0, 1, 5), Mask: net.CIDRMask(8, 32)},
				&net.IPNet{IP: net.IPv4(10, 0, 2, 5), Mask: net.CIDRMask(8, 32)},
				&net.IPNet{IP: net.IPv4(172, 16, 0, 1), Mask: net.CIDRMask(12, 32)},
			}, nil
		}
		defer afterEach()

		ips, err := GetAllNFSClientIPs([]string{"10.0.0.0/8"})
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ips))
		assert.Contains(t, ips, "10.0.1.5")
		assert.Contains(t, ips, "10.0.2.5")
	})

	t.Run("No match", func(t *testing.T) {
		interfaceAddrs = func() ([]net.Addr, error) {
			return []net.Addr{
				&net.IPNet{IP: net.IPv4(172, 16, 0, 1), Mask: net.CIDRMask(12, 32)},
			}, nil
		}
		defer afterEach()

		_, err := GetAllNFSClientIPs([]string{"10.0.0.0/8"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no valid IP address found")
	})

	t.Run("Multiple CIDRs", func(t *testing.T) {
		interfaceAddrs = func() ([]net.Addr, error) {
			return []net.Addr{
				&net.IPNet{IP: net.IPv4(10, 0, 1, 5), Mask: net.CIDRMask(8, 32)},
				&net.IPNet{IP: net.IPv4(192, 168, 1, 5), Mask: net.CIDRMask(16, 32)},
			}, nil
		}
		defer afterEach()

		ips, err := GetAllNFSClientIPs([]string{"10.0.0.0/8", "192.168.0.0/16"})
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ips))
		assert.Contains(t, ips, "10.0.1.5")
		assert.Contains(t, ips, "192.168.1.5")
	})

	t.Run("Interface error", func(t *testing.T) {
		interfaceAddrs = func() ([]net.Addr, error) {
			return nil, errors.New("interface error")
		}
		defer afterEach()

		_, err := GetAllNFSClientIPs([]string{"10.0.0.0/8"})
		assert.Error(t, err)
	})

	t.Run("Empty network list", func(t *testing.T) {
		_, err := GetAllNFSClientIPs([]string{})
		assert.Error(t, err)
	})

	// Scenario: Warning threshold exceeded (FR-2.2)
	t.Run("Warning threshold exceeded", func(t *testing.T) {
		var addrs []net.Addr
		for i := 1; i <= 33; i++ {
			addrs = append(addrs, &net.IPNet{IP: net.IPv4(10, 0, 1, byte(i)), Mask: net.CIDRMask(8, 32)})
		}
		interfaceAddrs = func() ([]net.Addr, error) {
			return addrs, nil
		}
		defer afterEach()

		ips, err := GetAllNFSClientIPs([]string{"10.0.0.0/8"})
		assert.NoError(t, err)
		assert.Equal(t, 33, len(ips))
	})

	// Scenario: Exactly at warning threshold (FR-2.2)
	t.Run("Exactly at warning threshold", func(t *testing.T) {
		var addrs []net.Addr
		for i := 1; i <= 32; i++ {
			addrs = append(addrs, &net.IPNet{IP: net.IPv4(10, 0, 1, byte(i)), Mask: net.CIDRMask(8, 32)})
		}
		interfaceAddrs = func() ([]net.Addr, error) {
			return addrs, nil
		}
		defer afterEach()

		ips, err := GetAllNFSClientIPs([]string{"10.0.0.0/8"})
		assert.NoError(t, err)
		assert.Equal(t, 32, len(ips))
	})

	// Scenario: Loopback and IPv6 excluded (FR-2.2)
	t.Run("Loopback and IPv6 excluded", func(t *testing.T) {
		interfaceAddrs = func() ([]net.Addr, error) {
			return []net.Addr{
				&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)},
				&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
				&net.IPNet{IP: net.IPv4(10, 0, 1, 5), Mask: net.CIDRMask(8, 32)},
			}, nil
		}
		defer afterEach()

		ips, err := GetAllNFSClientIPs([]string{"10.0.0.0/8"})
		assert.NoError(t, err)
		assert.Equal(t, []string{"10.0.1.5"}, ips)
		assert.NotContains(t, ips, "127.0.0.1")
	})
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
