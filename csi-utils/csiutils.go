/*
Copyright (c) 2021-2025 Dell Inc, or its subsidiaries.

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
	"fmt"
	"net"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/dell/csi-isilon/v2/common/utils/logging"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var interfaceAddrs = func() ([]net.Addr, error) {
	return net.InterfaceAddrs()
}

var parseCIDR = func(s string) (net.IP, *net.IPNet, error) {
	return net.ParseCIDR(s)
}

// RemoveExistingCSISockFile When the sock file that the gRPC server is going to be listening on already exists, error will be thrown saying the address is already in use, thus remove it first
var RemoveExistingCSISockFile = func() error {
	log := logging.GetLogger()
	protoAddr := os.Getenv(constants.EnvCSIEndpoint)

	log.Debugf("check if sock file '%s' has already been created", protoAddr)

	if protoAddr == "" {
		return nil
	}

	if _, err := os.Stat(protoAddr); !os.IsNotExist(err) {

		log.Debugf("sock file '%s' already exists, remove it", protoAddr)

		if err := os.RemoveAll(protoAddr); err != nil {

			log.WithError(err).Debugf("error removing sock file '%s'", protoAddr)

			return fmt.Errorf(
				"failed to remove sock file: '%s', error '%v'", protoAddr, err)
		}

		log.Debugf("sock file '%s' removed", protoAddr)

	} else {
		log.Debugf("sock file '%s' does not exist yet, move along", protoAddr)
	}

	return nil
}

// GetNFSClientIP is used to fetch IP address from networks on which NFS traffic is allowed
func GetNFSClientIP(allowedNetworks []string) (string, error) {
	var nodeIP string
	log := logging.GetLogger()
	addrs, err := interfaceAddrs()
	if err != nil {
		log.Errorf("Encountered error while fetching system IP addresses: %+v\n", err.Error())
		return "", err
	}

	// Populate map to optimize the algorithm for O(n)
	networks := make(map[string]bool)
	for _, cnet := range allowedNetworks {
		networks[cnet] = false
	}

	for _, a := range addrs {
		switch v := a.(type) {
		case *net.IPNet:
			if v.IP.To4() != nil {
				ip, cnet, err := parseCIDR(a.String())
				log.Debugf("IP address: %s and Network: %s", ip, cnet)
				if err != nil {
					log.Errorf("Encountered error while parsing IP address %v", a)
					continue
				}

				if _, ok := networks[cnet.String()]; ok {
					log.Infof("Found IP address: %s", ip)
					nodeIP = ip.String()
					return nodeIP, nil
				}
			}
		}
	}

	// If a valid IP address matching allowedNetworks is not found return error
	if nodeIP == "" {
		return "", fmt.Errorf("no valid IP address found matching against allowedNetworks %v", allowedNetworks)
	}

	return nodeIP, nil
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

	return &(am.Mode), nil
}

func IPInCIDR(ipStr, cidrStr string) bool {
	ip := net.ParseIP(ipStr)
	_, cidrNet, err := net.ParseCIDR(cidrStr)
	if err != nil || ip == nil {
		return false
	}
	return cidrNet.Contains(ip)
}
