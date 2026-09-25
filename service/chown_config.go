// Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package service

import (
	"context"
	"strconv"
	"strings"

	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
	csictx "github.com/Ecosystems/container-storage-modules/src/gocsi/context"
)

func parseChownBool(ctx context.Context, key string, defaultValue bool) bool {
	value, ok := csictx.LookupEnv(ctx, key)
	if !ok {
		return defaultValue
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		csmlog.WithContext(ctx).Warnf("invalid value %q for %s, defaulting to %t", value, key, defaultValue)
		return defaultValue
	}

	return parsed
}

func parseChownInt(ctx context.Context, key string, defaultValue, minValue int) int {
	value, ok := csictx.LookupEnv(ctx, key)
	if !ok {
		return defaultValue
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(trimmed)
	if err != nil || parsed < minValue {
		csmlog.WithContext(ctx).Warnf("invalid value %q for %s, defaulting to %d", value, key, defaultValue)
		return defaultValue
	}

	return parsed
}
