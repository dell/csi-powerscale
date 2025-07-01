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
package fromcontext

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dell/csi-isilon/v2/common/utils/logging"
	csictx "github.com/dell/gocsi/context"
	"gopkg.in/yaml.v3"
)

// GetBoolean parses an environment variable into a boolean value. If an error is encountered, default is set to false, and error is logged
func GetBoolean(ctx context.Context, key string) bool {
	log := logging.GetRunIDLogger(ctx)
	if val, ok := csictx.LookupEnv(ctx, key); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			log.WithField(key, val).Debugf(
				"invalid boolean value for '%s', defaulting to false", key)
			return false
		}
		return b
	}
	return false
}

// GetArray parses an environment variable into an array of string
func GetArray(ctx context.Context, key string) ([]string, error) {
	var values []string

	if val, ok := csictx.LookupEnv(ctx, key); ok {
		err := yaml.Unmarshal([]byte(val), &values)
		if err != nil {
			return values, fmt.Errorf("invalid array value for '%s'", key)
		}
	}
	return values, nil
}

// GetUint parses an environment variable into a uint value. If an error is encountered, default is set to 0, and error is logged
func GetUint(ctx context.Context, key string) uint {
	log := logging.GetRunIDLogger(ctx)
	if val, ok := csictx.LookupEnv(ctx, key); ok {
		i, err := strconv.ParseUint(val, 10, 0)
		if err != nil {
			log.WithField(key, val).Debugf(
				"invalid int value for '%s', defaulting to 0", key)
			return 0
		}
		return uint(i)
	}
	return 0
}

// GetInt64 parses an environment variable into an int64 value.
func GetInt64(ctx context.Context, key string) (int64, error) {
	if val, ok := csictx.LookupEnv(ctx, key); ok {
		i, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid int64 value '%v' specified for '%s'", val, key)
		}
		return i, nil
	}
	return 0, nil
}
