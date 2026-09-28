/*
 Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.

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

package collectors

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// U-REG-GAUGE: registerOrGetGaugeVec registers new gauge
func TestRegisterOrGetGaugeVec_NewRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_gauge",
		Help: "Test gauge metric",
	}, []string{"label"})

	result := registerOrGetGaugeVec(reg, gauge)
	assert.NotNil(t, result)
	assert.Same(t, gauge, result)
}

// U-REG-GAUGE-DUP: registerOrGetGaugeVec returns existing on duplicate
func TestRegisterOrGetGaugeVec_DuplicateRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	gauge1 := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_gauge_dup",
		Help: "Test gauge metric",
	}, []string{"label"})

	// First registration
	result1 := registerOrGetGaugeVec(reg, gauge1)
	assert.Same(t, gauge1, result1)

	// Second registration with same name
	gauge2 := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_gauge_dup",
		Help: "Test gauge metric",
	}, []string{"label"})

	result2 := registerOrGetGaugeVec(reg, gauge2)
	assert.NotNil(t, result2)
	assert.Same(t, result1, result2, "should return existing gauge")
}

// U-REG-COUNTER: registerOrGetCounterVec registers new counter
func TestRegisterOrGetCounterVec_NewRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_counter",
		Help: "Test counter metric",
	}, []string{"label"})

	result := RegisterOrGetCounterVec(reg, counter)
	assert.NotNil(t, result)
	assert.Same(t, counter, result)
}

// U-REG-COUNTER-DUP: registerOrGetCounterVec returns existing on duplicate
func TestRegisterOrGetCounterVec_DuplicateRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	counter1 := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_counter_dup",
		Help: "Test counter metric",
	}, []string{"label"})

	// First registration
	result1 := RegisterOrGetCounterVec(reg, counter1)
	assert.Same(t, counter1, result1)

	// Second registration with same name
	counter2 := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_counter_dup",
		Help: "Test counter metric",
	}, []string{"label"})

	result2 := RegisterOrGetCounterVec(reg, counter2)
	assert.NotNil(t, result2)
	assert.Same(t, result1, result2, "should return existing counter")
}

// U-REG-HISTOGRAM: registerOrGetHistogramVec registers new histogram
func TestRegisterOrGetHistogramVec_NewRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	histogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "test_histogram",
		Help: "Test histogram metric",
	}, []string{"label"})

	result := registerOrGetHistogramVec(reg, histogram)
	assert.NotNil(t, result)
	assert.Same(t, histogram, result)
}

// U-REG-HISTOGRAM-DUP: registerOrGetHistogramVec returns existing on duplicate
func TestRegisterOrGetHistogramVec_DuplicateRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	histogram1 := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "test_histogram_dup",
		Help: "Test histogram metric",
	}, []string{"label"})

	// First registration
	result1 := registerOrGetHistogramVec(reg, histogram1)
	assert.Same(t, histogram1, result1)

	// Second registration with same name
	histogram2 := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "test_histogram_dup",
		Help: "Test histogram metric",
	}, []string{"label"})

	result2 := registerOrGetHistogramVec(reg, histogram2)
	assert.NotNil(t, result2)
	assert.Same(t, result1, result2, "should return existing histogram")
}

// U-REG-PANIC: registerOrGetGaugeVec panics on non-AlreadyRegistered error
func TestRegisterOrGetGaugeVec_PanicOnOtherError(t *testing.T) {
	reg := prometheus.NewRegistry()
	// Create a gauge with invalid descriptor to trigger non-AlreadyRegistered error
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "", // Invalid empty name
		Help: "Test gauge metric",
	}, []string{"label"})

	assert.Panics(t, func() {
		registerOrGetGaugeVec(reg, gauge)
	})
}

// U-REG-COUNTER-PANIC: registerOrGetCounterVec panics on non-AlreadyRegistered error
func TestRegisterOrGetCounterVec_PanicOnOtherError(t *testing.T) {
	reg := prometheus.NewRegistry()
	// Create a counter with invalid descriptor to trigger non-AlreadyRegistered error
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "", // Invalid empty name
		Help: "Test counter metric",
	}, []string{"label"})

	assert.Panics(t, func() {
		RegisterOrGetCounterVec(reg, counter)
	})
}

// U-REG-HISTOGRAM-PANIC: registerOrGetHistogramVec panics on non-AlreadyRegistered error
func TestRegisterOrGetHistogramVec_PanicOnOtherError(t *testing.T) {
	reg := prometheus.NewRegistry()
	// Create a histogram with invalid descriptor to trigger non-AlreadyRegistered error
	histogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "", // Invalid empty name
		Help: "Test histogram metric",
	}, []string{"label"})

	assert.Panics(t, func() {
		registerOrGetHistogramVec(reg, histogram)
	})
}

// U-REG-MULTIPLE: Multiple registrations work correctly
func TestRegisterOrGet_MultipleRegistrations(t *testing.T) {
	reg := prometheus.NewRegistry()

	gauge := registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_multi_gauge",
		Help: "Test gauge",
	}, []string{"label"}))

	counter := RegisterOrGetCounterVec(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_multi_counter",
		Help: "Test counter",
	}, []string{"label"}))

	histogram := registerOrGetHistogramVec(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "test_multi_histogram",
		Help: "Test histogram",
	}, []string{"label"}))

	assert.NotNil(t, gauge)
	assert.NotNil(t, counter)
	assert.NotNil(t, histogram)

	// Verify they can be used
	gauge.WithLabelValues("test").Set(1.0)
	counter.WithLabelValues("test").Inc()
	histogram.WithLabelValues("test").Observe(1.0)
}
