// Copyright © 2021-2026 Dell Inc. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package csiutils

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	"github.com/Ecosystems/container-storage-modules/src/csmlog"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var interfaceAddrs = func() ([]net.Addr, error) {
	return net.InterfaceAddrs()
}

// RemoveExistingCSISockFile When the sock file that the gRPC server is going to be listening on already exists, error will be thrown saying the address is already in use, thus remove it first
var RemoveExistingCSISockFile = func() error {
	protoAddr := os.Getenv(constants.EnvCSIEndpoint)

	csmlog.Debugf("check if sock file '%s' has already been created", protoAddr)

	if protoAddr == "" {
		return nil
	}

	if _, err := os.Stat(protoAddr); !os.IsNotExist(err) {

		csmlog.Debugf("sock file '%s' already exists, remove it", protoAddr)

		if err := os.RemoveAll(protoAddr); err != nil {

			csmlog.Debugf("error removing sock file '%s'"+"failed with error : %s", protoAddr, err.Error())

			return fmt.Errorf(
				"failed to remove sock file: '%s', error '%v'", protoAddr, err)
		}

		csmlog.Debugf("sock file '%s' removed", protoAddr)

	} else {
		csmlog.Debugf("sock file '%s' does not exist yet, move along", protoAddr)
	}

	return nil
}

// extractIPFromAddr extracts the IP address from a net.Addr interface.
// It handles both *net.IPNet and *net.IPAddr types, returning nil for unsupported types.
func extractIPFromAddr(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	default:
		return nil
	}
}

// GetNFSClientIP is used to fetch IP address from networks on which NFS traffic is allowed.
//
// Performance: This function has O(n*m) complexity where n is the number of network interfaces
// and m is the number of allowed networks. This is acceptable for typical deployments where
// both n and m are small (usually < 10 each). If n or m grow large, consider a prefix-aware
// data structure (e.g. a radix trie keyed by network prefix) instead of a linear scan, since
// CIDR containment cannot be reduced to an O(1) hash-map lookup.
//
// IPv6 Limitation: This function only considers IPv4 addresses (ip.To4() != nil).
// IPv6 addresses (and any allowedNetworks entries that are IPv6 CIDRs) are silently ignored,
// since they can never match. This is intentional as the PowerScale NFS driver currently only
// supports IPv4 for NFS traffic.
func GetNFSClientIP(allowedNetworks []string) (string, error) {
	if allowedNetworks == nil {
		return "", fmt.Errorf("allowedNetworks parameter cannot be nil")
	}

	// Parse allowedNetworks into IPNet objects for IP containment checking
	// Do this early to filter invalid CIDRs before querying network interfaces.
	// Invalid CIDRs are logged as warnings but don't prevent valid ones from being used.
	// This allows the driver to function with partial valid configuration while
	// providing visibility into configuration errors that need to be fixed.
	parsedNetworks := make([]*net.IPNet, 0, len(allowedNetworks))
	for _, cnet := range allowedNetworks {
		_, cidrNet, err := net.ParseCIDR(cnet)
		if err != nil {
			csmlog.Warnf("Invalid CIDR in allowedNetworks, skipping: %s (error: %v)", cnet, err)
			continue
		}
		parsedNetworks = append(parsedNetworks, cidrNet)
	}

	// Early exit if no valid networks to match against
	if len(parsedNetworks) == 0 {
		csmlog.Warnf("No valid networks in allowedNetworks %v", allowedNetworks)
		return "", fmt.Errorf("no valid networks in allowedNetworks %v", allowedNetworks)
	}

	addrs, err := interfaceAddrs()
	if err != nil {
		csmlog.Errorf("Encountered error while fetching system IP addresses: %+v\n", err.Error())
		return "", err
	}

	// Collect discovered IPv4 addresses for error reporting
	// Pre-allocate with capacity to avoid slice reallocations during the loop
	discoveredIPs := make([]string, 0, len(addrs))

	for _, addr := range addrs {
		ip := extractIPFromAddr(addr)
		if ip == nil || ip.To4() == nil {
			continue
		}

		discoveredIPs = append(discoveredIPs, ip.String())
		csmlog.Debugf("IP address: %s", ip)
		for _, cidrNet := range parsedNetworks {
			if cidrNet.Contains(ip) {
				csmlog.Infof("Found IP address: %s in network %s", ip, cidrNet)
				return ip.String(), nil
			}
		}
	}

	// If a valid IP address matching allowedNetworks is not found return error.
	// Include discovered IPs in the error message to aid debugging.
	return "", fmt.Errorf("no valid IP address found matching against allowedNetworks %v; discovered IPs: %v", allowedNetworks, discoveredIPs)
}

// GetAllNFSClientIPs returns all IP addresses matching the allowedNetworks CIDRs.
// Unlike GetNFSClientIP which returns only the first match, this function collects
// all matching IPv4 addresses for multi-NIC support.
func GetAllNFSClientIPs(allowedNetworks []string) ([]string, error) {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		csmlog.WithFields(csmlog.Fields{
			"operation":              "GetAllNFSClientIPs",
			"duration_ms":            duration.Milliseconds(),
			"allowed_networks_count": len(allowedNetworks),
		}).Debugf("IP discovery completed in %dms", duration.Milliseconds())
	}()

	// Parse allowedNetworks into IPNet objects for IP containment checking
	// This matches the logic in GetNFSClientIP for consistency
	parsedNetworks := make([]*net.IPNet, 0, len(allowedNetworks))
	for _, cnet := range allowedNetworks {
		_, cidrNet, err := net.ParseCIDR(cnet)
		if err != nil {
			csmlog.WithFields(csmlog.Fields{
				"operation": "GetAllNFSClientIPs",
				"cidr":      cnet,
				"success":   false,
			}).Warnf("Invalid CIDR in allowedNetworks, skipping: %s (error: %v)", cnet, err)
			continue
		}
		parsedNetworks = append(parsedNetworks, cidrNet)
	}

	// Early exit if no valid networks to match against
	if len(parsedNetworks) == 0 {
		csmlog.WithFields(csmlog.Fields{
			"operation":        "GetAllNFSClientIPs",
			"allowed_networks": allowedNetworks,
			"parsed_count":     len(parsedNetworks),
			"success":          false,
		}).Warnf("No valid networks in allowedNetworks %v", allowedNetworks)
		return nil, fmt.Errorf("no valid networks in allowedNetworks %v", allowedNetworks)
	}

	addrs, err := interfaceAddrs()
	if err != nil {
		csmlog.WithFields(csmlog.Fields{
			"operation": "GetAllNFSClientIPs",
			"success":   false,
		}).Errorf("Encountered error while fetching system IP addresses: %+v\n", err.Error())
		return nil, err
	}

	var ips []string
	for _, a := range addrs {
		ip := extractIPFromAddr(a)
		if ip == nil || ip.To4() == nil {
			continue
		}

		// Check if this IP is contained in any of the allowed networks
		for _, cidrNet := range parsedNetworks {
			if cidrNet.Contains(ip) {
				csmlog.WithFields(csmlog.Fields{
					"operation": "GetAllNFSClientIPs",
					"ip":        ip.String(),
					"network":   cidrNet.String(),
				}).Debugf("Multi-NIC: found matching IP address: %s in network %s", ip, cidrNet)
				ips = append(ips, ip.String())
				break // IP matched, no need to check other networks
			}
		}
	}

	if len(ips) > constants.MaxInterfaceWarningThreshold {
		csmlog.WithFields(csmlog.Fields{
			"operation":       "GetAllNFSClientIPs",
			"interface_count": len(ips),
			"threshold":       constants.MaxInterfaceWarningThreshold,
		}).Warnf("interface count %d exceeds threshold %d", len(ips), constants.MaxInterfaceWarningThreshold)
	}

	if len(ips) == 0 {
		csmlog.WithFields(csmlog.Fields{
			"operation":        "GetAllNFSClientIPs",
			"allowed_networks": allowedNetworks,
			"success":          false,
		}).Warnf("no valid IP address found matching against allowedNetworks %v", allowedNetworks)
		return nil, fmt.Errorf("no valid IP address found matching against allowedNetworks %v", allowedNetworks)
	}

	csmlog.WithFields(csmlog.Fields{
		"operation":              "GetAllNFSClientIPs",
		"ip_count":               len(ips),
		"allowed_networks_count": len(allowedNetworks),
		"success":                true,
	}).Infof("Multi-NIC: selected %d IPs from allowedNetworks", len(ips))
	return ips, nil
}

// GetAccessMode extracts the access mode from the given *csi.ControllerPublishVolumeRequest instance
func GetAccessMode(req *csi.ControllerPublishVolumeRequest) (*csi.VolumeCapability_AccessMode_Mode, error) {
	vc := req.GetVolumeCapability()
	if vc == nil {
		return nil, status.Error(codes.InvalidArgument,
			"volume capability is required")
	}

	am := vc.GetAccessMode()
	if am == nil {
		return nil, status.Error(codes.InvalidArgument,
			"access mode is required")
	}

	if am.Mode == csi.VolumeCapability_AccessMode_UNKNOWN {
		return nil, status.Error(codes.InvalidArgument,
			"unknown access mode")
	}

	return &am.Mode, nil
}

func IPInCIDR(ipStr, cidrStr string) bool {
	ip := net.ParseIP(ipStr)
	_, cidrNet, err := net.ParseCIDR(cidrStr)
	if err != nil || ip == nil {
		return false
	}
	return cidrNet.Contains(ip)
}
