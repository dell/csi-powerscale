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
	"fmt"
	"regexp"

	csmlog "github.com/dell/csmlog"
)

const (
	// NodeIDSeparator is the separator that separates node name and IP Address
	NodeIDSeparator = "=#=#="

	// DummyHostNodeID is nodeID used for adding dummy client in client field of export
	DummyHostNodeID = "localhost=#=#=localhost=#=#=127.0.0.1"
)

// NodeIDPattern is the regex pattern that identifies the NodeID
var NodeIDPattern = regexp.MustCompile(fmt.Sprintf("^(.+)%s(.+)%s(.+)$", NodeIDSeparator, NodeIDSeparator))

// ParseNodeID parses NodeID to node name, node FQDN and IP address using pattern '^(.+)=#=#=(.+)=#=#=(.+)'
func ParseNodeID(ctx context.Context, nodeID string) (string, string, string, error) {
	log := csmlog.GetLogger().WithContext(ctx)

	matches := NodeIDPattern.FindStringSubmatch(nodeID)

	if len(matches) < 4 {
		return "", "", "", fmt.Errorf("node ID '%s' cannot match the expected '^(.+)=#=#=(.+)=#=#=(.+)$' pattern", nodeID)
	}

	log.Debugf("Node ID '%s' parsed into node name '%s', node FQDN '%s' and IP address '%s'",
		nodeID, matches[1], matches[2], matches[3])

	return matches[1], matches[2], matches[3], nil
}
