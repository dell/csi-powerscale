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
