package csiutils

import (
	"testing"
)

func TestGetNFSClientIP(t *testing.T) {
	tests := []struct {
		name            string
		allowedNetworks []string
		expectError     bool
	}{
		{
			name: "Valid_Network",
			allowedNetworks: []string{
				"10.247.96.0/21",
				"10.244.0.0/24",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
