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

	"github.com/dell/csi-isilon/v2/common/utils"
)

var interfaceAddrs = func() ([]net.Addr, error) {
	return net.InterfaceAddrs()
}

var parseCIDR = func(s string) (net.IP, *net.IPNet, error) {
	return net.ParseCIDR(s)
}

// GetNFSClientIP is used to fetch IP address from networks on which NFS traffic is allowed
func GetNFSClientIP(allowedNetworks []string) (string, error) {
	var nodeIP string
	log := utils.GetLogger()
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
