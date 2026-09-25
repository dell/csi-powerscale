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
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── parseChownBool ────────────────────────────────────────────────────────

func TestParseChownBool_UnsetUsesDefault(t *testing.T) {
	result := parseChownBool(context.Background(), "X_TEST_CB_UNSET_1234", true)
	assert.True(t, result)
}

func TestParseChownBool_ValidTrue(t *testing.T) {
	t.Setenv("X_TEST_CB_TRUE", "true")
	result := parseChownBool(context.Background(), "X_TEST_CB_TRUE", false)
	assert.True(t, result)
}

func TestParseChownBool_ValidFalse(t *testing.T) {
	t.Setenv("X_TEST_CB_FALSE", "false")
	result := parseChownBool(context.Background(), "X_TEST_CB_FALSE", true)
	assert.False(t, result)
}

func TestParseChownBool_InvalidWarnsAndDefaults(t *testing.T) {
	t.Setenv("X_TEST_CB_BAD", "notabool")
	result := parseChownBool(context.Background(), "X_TEST_CB_BAD", true)
	assert.True(t, result)
}

func TestParseChownBool_WhitespaceOnlyUsesDefault(t *testing.T) {
	t.Setenv("X_TEST_CB_WS", "   ")
	result := parseChownBool(context.Background(), "X_TEST_CB_WS", false)
	assert.False(t, result)
}

func TestParseChownBool_EmptyUsesDefault(t *testing.T) {
	t.Setenv("X_TEST_CB_EMPTY", "")
	result := parseChownBool(context.Background(), "X_TEST_CB_EMPTY", true)
	assert.True(t, result)
}

func TestParseChownBool_True(t *testing.T) {
	t.Setenv("X_TEST_CB_TRUE_UPPER", "TRUE")
	result := parseChownBool(context.Background(), "X_TEST_CB_TRUE_UPPER", false)
	assert.True(t, result)
}

func TestParseChownBool_One(t *testing.T) {
	t.Setenv("X_TEST_CB_ONE", "1")
	result := parseChownBool(context.Background(), "X_TEST_CB_ONE", false)
	assert.True(t, result)
}

func TestParseChownBool_Zero(t *testing.T) {
	t.Setenv("X_TEST_CB_ZERO", "0")
	result := parseChownBool(context.Background(), "X_TEST_CB_ZERO", true)
	assert.False(t, result)
}

// ── parseChownInt ────────────────────────────────────────────────────────

func TestParseChownInt_UnsetUsesDefault(t *testing.T) {
	result := parseChownInt(context.Background(), "X_TEST_CI_UNSET_1234", 100, 1)
	assert.Equal(t, 100, result)
}

func TestParseChownInt_ValidValue(t *testing.T) {
	t.Setenv("X_TEST_CI_VALID", "50")
	result := parseChownInt(context.Background(), "X_TEST_CI_VALID", 100, 1)
	assert.Equal(t, 50, result)
}

func TestParseChownInt_InvalidWarnsAndDefaults(t *testing.T) {
	t.Setenv("X_TEST_CI_BAD", "notanint")
	result := parseChownInt(context.Background(), "X_TEST_CI_BAD", 100, 1)
	assert.Equal(t, 100, result)
}

func TestParseChownInt_BelowMinimumWarnsAndDefaults(t *testing.T) {
	t.Setenv("X_TEST_CI_BELOW_MIN", "3")
	result := parseChownInt(context.Background(), "X_TEST_CI_BELOW_MIN", 100, 5)
	assert.Equal(t, 100, result)
}

func TestParseChownInt_ZeroWarnsAndDefaults(t *testing.T) {
	t.Setenv("X_TEST_CI_ZERO", "0")
	result := parseChownInt(context.Background(), "X_TEST_CI_ZERO", 100, 1)
	assert.Equal(t, 100, result)
}

func TestParseChownInt_NegativeWarnsAndDefaults(t *testing.T) {
	t.Setenv("X_TEST_CI_NEG", "-1")
	result := parseChownInt(context.Background(), "X_TEST_CI_NEG", 100, 1)
	assert.Equal(t, 100, result)
}

func TestParseChownInt_WhitespaceOnlyUsesDefault(t *testing.T) {
	t.Setenv("X_TEST_CI_WS", "   ")
	result := parseChownInt(context.Background(), "X_TEST_CI_WS", 50, 1)
	assert.Equal(t, 50, result)
}

func TestParseChownInt_EmptyUsesDefault(t *testing.T) {
	t.Setenv("X_TEST_CI_EMPTY", "")
	result := parseChownInt(context.Background(), "X_TEST_CI_EMPTY", 75, 1)
	assert.Equal(t, 75, result)
}

func TestParseChownInt_ValidAtMinimum(t *testing.T) {
	t.Setenv("X_TEST_CI_MIN", "5")
	result := parseChownInt(context.Background(), "X_TEST_CI_MIN", 100, 5)
	assert.Equal(t, 5, result)
}

func TestParseChownInt_AboveMinimum(t *testing.T) {
	t.Setenv("X_TEST_CI_ABOVE", "10")
	result := parseChownInt(context.Background(), "X_TEST_CI_ABOVE", 100, 5)
	assert.Equal(t, 10, result)
}

func TestParseChownInt_LargeValue(t *testing.T) {
	t.Setenv("X_TEST_CI_LARGE", "1000")
	result := parseChownInt(context.Background(), "X_TEST_CI_LARGE", 0, 1)
	assert.Equal(t, 1000, result)
}
