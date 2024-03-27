package csiutils

/*
Copyright (c) 2021 Dell Inc, or its subsidiaries.

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
	"fmt"
	"net"

	"github.com/dell/csi-isilon/v2/common/utils"
)

// GetNFSClientIPCandidates lists up the possible IP addresses from networks on which NFS traffic is allowed
func GetNFSClientIPCandidates(allowedNetworks []string) (map[string][]*net.TCPAddr, error) {
	log := utils.GetLogger()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Errorf("Encountered error while fetching system IP addresses: %+v\n", err.Error())
		return nil, err
	}

	// Populate map to optimize the algorithm for O(n)
	networks := make(map[string][]*net.TCPAddr)
	for _, cnet := range allowedNetworks {
		networks[cnet] = make([]*net.TCPAddr, 0, len(addrs))
	}

	found := false
	for _, a := range addrs {
		switch v := a.(type) {
		case *net.IPNet:
			if v.IP.To4() != nil {
				ip, cnet, err := net.ParseCIDR(a.String())
				log.Debugf("IP address: %s and Network: %s", ip, cnet)
				if err != nil {
					log.Errorf("Encountered error while parsing IP address %v", a)
					continue
				}

				cnetStr := cnet.String()
				if network, ok := networks[cnetStr]; ok {
					log.Infof("Found IP address: %s", ip)
					networks[cnetStr] = append(network, &net.TCPAddr{IP: ip})
					found = true
				}
			}
		}
	}

	// If a valid IP address matching allowedNetworks is not found return error
	if !found {
		return nil, fmt.Errorf("no valid IP address found matching against allowedNetworks %v", allowedNetworks)
	}

	return networks, nil
}
