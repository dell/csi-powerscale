/*
Copyright (c) 2019-2025 Dell Inc, or its subsidiaries.

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
package identifiers

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/Showmax/go-fqdn"
	"github.com/dell/csi-isilon/v2/common/utils/logging"
)

// GetFQDNByIP returns the FQDN based on the parsed ip address
func GetFQDNByIP(ctx context.Context, ip string) (string, error) {
	log := logging.GetRunIDLogger(ctx)
	names, err := net.LookupAddr(ip)
	if err != nil {
		log.Debugf("error getting FQDN: '%s'", err)
		return "", err
	}
	// The first one is FQDN
	FQDN := strings.TrimSuffix(names[0], ".")
	return FQDN, nil
}

// GetOwnFQDN returns the FQDN of the node or controller itself
func GetOwnFQDN() (string, error) {
	nodeFQDN := fqdn.Get()
	if nodeFQDN == "unknown" {
		return "", errors.New("cannot get FQDN")
	}
	return nodeFQDN, nil
}
