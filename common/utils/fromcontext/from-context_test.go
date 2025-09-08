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

package fromcontext

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseArrayFromContext(t *testing.T) {
	ctx := context.Background()
	arrYAML := "- item1\n- item2\n- item3"
	os.Setenv("TEST_ARRAY", arrYAML)
	defer os.Unsetenv("TEST_ARRAY")

	val, err := GetArray(ctx, "TEST_ARRAY")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(val) != 3 || val[0] != "item1" || val[1] != "item2" || val[2] != "item3" {
		t.Errorf("Unexpected parsed array: %v", val)
	}
}

func TestParseInt64FromContext(t *testing.T) {
	ctx := context.Background()
	os.Setenv("TEST_INT64", "-100")
	defer os.Unsetenv("TEST_INT64")

	val, err := GetInt64(ctx, "TEST_INT64")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if val != -100 {
		t.Errorf("Expected -100, got %v", val)
	}
}

func TestParseUnitFromContext(t *testing.T) {
	ctx := context.Background()

	// Test case: Valid integer value
	os.Setenv("TEST_UINT", "42")
	defer os.Unsetenv("TEST_UINT")

	val := GetUint(ctx, "TEST_UINT")
	assert.Equal(t, uint(42), val, "Expected 42, got %v", val)

	// Test case: Invalid integer value (error case)
	os.Setenv("TEST_UINT_INVALID", "not_a_number")
	defer os.Unsetenv("TEST_UINT_INVALID")

	val = GetUint(ctx, "TEST_UINT_INVALID")
	assert.Equal(t, uint(0), val, "Expected 0 due to parsing error, got %v", val)

	// Test case: Missing environment variable
	val = GetUint(ctx, "NON_EXISTENT_KEY")
	assert.Equal(t, uint(0), val, "Expected 0 for non-existent key, got %v", val)
}

func TestParseBooleanFromContext(t *testing.T) {
	ctx := context.Background()

	// Test Case 1: Valid "true" value
	t.Run("Valid true boolean", func(t *testing.T) {
		os.Setenv("TEST_BOOL", "true")
		defer os.Unsetenv("TEST_BOOL")

		result := GetBoolean(ctx, "TEST_BOOL")
		assert.True(t, result, "Expected true")
	})

	// Test Case 2: Valid "false" value
	t.Run("Valid false boolean", func(t *testing.T) {
		os.Setenv("TEST_BOOL", "false")
		defer os.Unsetenv("TEST_BOOL")

		result := GetBoolean(ctx, "TEST_BOOL")
		assert.False(t, result, "Expected false")
	})

	// Test Case 3: Invalid boolean value (error condition)
	t.Run("Invalid boolean value", func(t *testing.T) {
		os.Setenv("TEST_BOOL", "notaboolean") // Invalid input
		defer os.Unsetenv("TEST_BOOL")

		result := GetBoolean(ctx, "TEST_BOOL")
		assert.False(t, result, "Expected false due to invalid boolean value")
	})

	// Test Case 4: Environment variable not set
	t.Run("Missing environment variable", func(t *testing.T) {
		os.Unsetenv("TEST_BOOL") // Ensure the variable is not set

		result := GetBoolean(ctx, "TEST_BOOL")
		assert.False(t, result, "Expected false when the environment variable is missing")
	})
}

func TestParseInt64FromContext1(t *testing.T) {
	ctx := context.Background()

	// Test Case 1: Valid int64 value
	t.Run("Valid int64 value", func(t *testing.T) {
		os.Setenv("TEST_INT", "123456789")
		defer os.Unsetenv("TEST_INT")

		result, err := GetInt64(ctx, "TEST_INT")
		assert.NoError(t, err, "Expected no error for valid int64 value")
		assert.Equal(t, int64(123456789), result, "Expected parsed int64 value")
	})

	// Test Case 2: Invalid int64 value (error condition)
	t.Run("Invalid int64 value", func(t *testing.T) {
		os.Setenv("TEST_INT", "notanumber") // Invalid input
		defer os.Unsetenv("TEST_INT")

		result, err := GetInt64(ctx, "TEST_INT")
		assert.Error(t, err, "Expected error for invalid int64 value")
		assert.Equal(t, int64(0), result, "Expected default int64 value (0) on error")
	})

	// Test Case 3: Missing environment variable
	t.Run("Missing environment variable", func(t *testing.T) {
		os.Unsetenv("TEST_INT") // Ensure the variable is not set

		result, err := GetInt64(ctx, "TEST_INT")
		assert.NoError(t, err, "Expected no error when environment variable is missing")
		assert.Equal(t, int64(0), result, "Expected default int64 value (0) when env variable is not set")
	})
}

func TestParseArrayFromContext1(t *testing.T) {
	ctx := context.Background()

	t.Run("Valid YAML array", func(t *testing.T) {
		arrYAML := "- item1\n- item2\n- item3"
		os.Setenv("TEST_ARRAY", arrYAML)
		defer os.Unsetenv("TEST_ARRAY")

		val, err := GetArray(ctx, "TEST_ARRAY")
		assert.Nil(t, err)
		assert.Equal(t, []string{"item1", "item2", "item3"}, val)
	})

	t.Run("Invalid YAML format", func(t *testing.T) {
		invalidYAML := "{invalid_yaml}"
		os.Setenv("TEST_ARRAY", invalidYAML)
		defer os.Unsetenv("TEST_ARRAY")

		val, err := GetArray(ctx, "TEST_ARRAY")
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid array value for 'TEST_ARRAY'")
		assert.Empty(t, val)
	})

	t.Run("Key not found", func(t *testing.T) {
		os.Unsetenv("TEST_ARRAY")

		val, err := GetArray(ctx, "TEST_ARRAY")
		assert.Nil(t, err)
		assert.Empty(t, val)
	})
}
