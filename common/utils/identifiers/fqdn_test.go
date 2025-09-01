/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package identifiers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetOwnFQDN(t *testing.T) {
	fqdn, err := GetOwnFQDN()
	if err != nil {
		t.Skip("Skipping test: Unable to get FQDN")
	}
	assert.NoError(t, err)
	assert.NotEmpty(t, fqdn)
}

func TestGetFQDNByIP(t *testing.T) {
	ctx := context.Background()

	t.Run("Valid IP with FQDN", func(t *testing.T) {
		// Use a public IP that is likely to return a valid FQDN
		fqdn, err := GetFQDNByIP(ctx, "8.8.8.8") // Google's public DNS server
		if err == nil {
			assert.NotEmpty(t, fqdn, "Expected a non-empty FQDN")
		}
	})

	t.Run("Invalid IP should return error", func(t *testing.T) {
		fqdn, err := GetFQDNByIP(ctx, "256.256.256.256") // Invalid IP
		assert.Error(t, err, "Expected an error for invalid IP")
		assert.Empty(t, fqdn, "Expected empty FQDN for invalid IP")
	})

	t.Run("Non-resolvable IP should return error", func(t *testing.T) {
		fqdn, err := GetFQDNByIP(ctx, "192.0.2.1") // IP in TEST-NET-1 (unlikely to resolve)
		assert.Error(t, err, "Expected an error for non-resolvable IP")
		assert.Empty(t, fqdn, "Expected empty FQDN for non-resolvable IP")
	})
}
