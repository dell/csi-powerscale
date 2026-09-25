/*
Copyright (c) 2025-2026 Dell Inc, or its subsidiaries.

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

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ── parseMetricsBool ────────────────────────────────────────────────────────

func TestParseMetricsBool_UnsetUsesDefault(t *testing.T) {
	result := parseMetricsBool(context.Background(), "X_TEST_MB_UNSET_1234", true)
	assert.True(t, result)
}

func TestParseMetricsBool_ValidTrue(t *testing.T) {
	t.Setenv("X_TEST_MB_TRUE", "true")
	result := parseMetricsBool(context.Background(), "X_TEST_MB_TRUE", false)
	assert.True(t, result)
}

func TestParseMetricsBool_ValidFalse(t *testing.T) {
	t.Setenv("X_TEST_MB_FALSE", "false")
	result := parseMetricsBool(context.Background(), "X_TEST_MB_FALSE", true)
	assert.False(t, result)
}

func TestParseMetricsBool_InvalidWarnsAndDefaults(t *testing.T) {
	t.Setenv("X_TEST_MB_BAD", "notabool")
	result := parseMetricsBool(context.Background(), "X_TEST_MB_BAD", true)
	assert.True(t, result)
}

func TestParseMetricsBool_WhitespaceOnlyUsesDefault(t *testing.T) {
	t.Setenv("X_TEST_MB_WS", "   ")
	result := parseMetricsBool(context.Background(), "X_TEST_MB_WS", false)
	assert.False(t, result)
}

// ── parseMetricsDuration ────────────────────────────────────────────────────

func TestParseMetricsDuration_UnsetUsesDefault(t *testing.T) {
	result := parseMetricsDuration(context.Background(), "X_TEST_MD_UNSET_1234", 30*time.Second)
	assert.Equal(t, 30*time.Second, result)
}

func TestParseMetricsDuration_ValidValue(t *testing.T) {
	t.Setenv("X_TEST_MD_VALID", "45s")
	result := parseMetricsDuration(context.Background(), "X_TEST_MD_VALID", 30*time.Second)
	assert.Equal(t, 45*time.Second, result)
}

func TestParseMetricsDuration_InvalidWarnsAndDefaults(t *testing.T) {
	t.Setenv("X_TEST_MD_BAD", "notaduration")
	result := parseMetricsDuration(context.Background(), "X_TEST_MD_BAD", 30*time.Second)
	assert.Equal(t, 30*time.Second, result)
}

func TestParseMetricsDuration_NegativeWarnsAndDefaults(t *testing.T) {
	t.Setenv("X_TEST_MD_NEG", "-5s")
	result := parseMetricsDuration(context.Background(), "X_TEST_MD_NEG", 30*time.Second)
	assert.Equal(t, 30*time.Second, result)
}

func TestParseMetricsDuration_ZeroWarnsAndDefaults(t *testing.T) {
	t.Setenv("X_TEST_MD_ZERO", "0s")
	result := parseMetricsDuration(context.Background(), "X_TEST_MD_ZERO", 30*time.Second)
	assert.Equal(t, 30*time.Second, result)
}

func TestParseMetricsDuration_WhitespaceOnlyUsesDefault(t *testing.T) {
	t.Setenv("X_TEST_MD_WS", "   ")
	result := parseMetricsDuration(context.Background(), "X_TEST_MD_WS", 30*time.Second)
	assert.Equal(t, 30*time.Second, result)
}

func TestParseMetricsInt_UnsetUsesDefault(t *testing.T) {
	result := parseMetricsInt(context.Background(), "X_TEST_MI_UNSET_1234", 100)
	assert.Equal(t, 100, result)
}

func TestParseMetricsInt_ValidValue(t *testing.T) {
	t.Setenv("X_TEST_MI_VALID", "50")
	result := parseMetricsInt(context.Background(), "X_TEST_MI_VALID", 100)
	assert.Equal(t, 50, result)
}

func TestParseMetricsInt_InvalidWarnsAndDefaults(t *testing.T) {
	t.Setenv("X_TEST_MI_BAD", "notanint")
	result := parseMetricsInt(context.Background(), "X_TEST_MI_BAD", 100)
	assert.Equal(t, 100, result)
}

func TestParseMetricsInt_ZeroWarnsAndDefaults(t *testing.T) {
	t.Setenv("X_TEST_MI_ZERO", "0")
	result := parseMetricsInt(context.Background(), "X_TEST_MI_ZERO", 100)
	assert.Equal(t, 100, result)
}

func TestParseMetricsInt_NegativeWarnsAndDefaults(t *testing.T) {
	t.Setenv("X_TEST_MI_NEG", "-1")
	result := parseMetricsInt(context.Background(), "X_TEST_MI_NEG", 100)
	assert.Equal(t, 100, result)
}

func TestParseMetricsInt_WhitespaceOnlyUsesDefault(t *testing.T) {
	t.Setenv("X_TEST_MI_WS", "   ")
	result := parseMetricsInt(context.Background(), "X_TEST_MI_WS", 100)
	assert.Equal(t, 100, result)
}

// ── formatMetricsAddr ───────────────────────────────────────────────────────

func TestFormatMetricsAddr_PortOnly(t *testing.T) {
	assert.Equal(t, ":8443", formatMetricsAddr("8443"))
}

func TestFormatMetricsAddr_AlreadyHasColon(t *testing.T) {
	assert.Equal(t, ":8443", formatMetricsAddr(":8443"))
}

func TestFormatMetricsAddr_EmptyUsesDefault(t *testing.T) {
	assert.Equal(t, ":8443", formatMetricsAddr(""))
}

func TestFormatMetricsAddr_WhitespaceOnlyUsesDefault(t *testing.T) {
	assert.Equal(t, ":8443", formatMetricsAddr("   "))
}

func TestFormatMetricsAddr_WhitespaceTrimmed(t *testing.T) {
	assert.Equal(t, ":9090", formatMetricsAddr("  9090  "))
}

func TestFormatMetricsAddr_WhitespaceWithColon(t *testing.T) {
	assert.Equal(t, ":9090", formatMetricsAddr("  :9090  "))
}
