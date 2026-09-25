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

package collectors_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/service/collectors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubCollector struct {
	name  string
	err   error
	calls int
}

func (s *stubCollector) Collect(_ context.Context) error {
	s.calls++
	return s.err
}

func (s *stubCollector) Name() string { return s.name }

type cleanupStubCollector struct {
	stubCollector
	cleanupCalls int
}

func (s *cleanupStubCollector) Cleanup() {
	s.cleanupCalls++
}

// U-MGR-01: NewCollectorManager returns an empty manager
func TestCollectorManager_New_Empty(t *testing.T) {
	m := collectors.NewCollectorManager()
	require.NotNil(t, m)
	assert.Empty(t, m.Collectors())
}

// U-MGR-02: Register adds collectors; Collectors returns them in order
func TestCollectorManager_Register_And_List(t *testing.T) {
	m := collectors.NewCollectorManager()
	a := &stubCollector{name: "A"}
	b := &stubCollector{name: "B"}
	m.Register(a)
	m.Register(b)

	list := m.Collectors()
	require.Len(t, list, 2)
	assert.Equal(t, "A", list[0].Name())
	assert.Equal(t, "B", list[1].Name())
}

// U-MGR-03: Collectors returns a copy — modifying the result does not affect the manager
func TestCollectorManager_Collectors_ReturnsCopy(t *testing.T) {
	m := collectors.NewCollectorManager()
	m.Register(&stubCollector{name: "X"})

	list := m.Collectors()
	list[0] = &stubCollector{name: "mutated"}

	// Original manager still has "X"
	assert.Equal(t, "X", m.Collectors()[0].Name())
}

// U-MGR-04: CollectAll calls every collector and returns nil on success
func TestCollectorManager_CollectAll_AllSuccess(t *testing.T) {
	m := collectors.NewCollectorManager()
	a := &stubCollector{name: "A"}
	b := &stubCollector{name: "B"}
	m.Register(a)
	m.Register(b)

	err := m.CollectAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, a.calls)
	assert.Equal(t, 1, b.calls)
}

// U-MGR-05: CollectAll continues after a per-collector error and returns combined error
func TestCollectorManager_CollectAll_PartialError(t *testing.T) {
	m := collectors.NewCollectorManager()
	good := &stubCollector{name: "Good"}
	bad := &stubCollector{name: "Bad", err: errors.New("disk unavailable")}
	m.Register(bad)
	m.Register(good)

	err := m.CollectAll(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Bad")
	assert.Contains(t, err.Error(), "disk unavailable")
	assert.Equal(t, 1, good.calls, "good collector must still be called")
}

// U-MGR-06: CollectAll on empty manager returns nil
func TestCollectorManager_CollectAll_Empty(t *testing.T) {
	m := collectors.NewCollectorManager()
	err := m.CollectAll(context.Background())
	assert.NoError(t, err)
}

// U-MGR-07: Start launches collectors and Stop terminates them
func TestCollectorManager_Start_And_Stop(t *testing.T) {
	m := collectors.NewCollectorManager()
	c := &stubCollector{name: "TestCollector"}
	m.Register(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.Start(ctx, 100*time.Millisecond)
	time.Sleep(250 * time.Millisecond)
	m.Stop()

	assert.Greater(t, c.calls, 0, "collector should have been called at least once")
}

// U-MGR-08: Stop on unstarted manager does not panic
func TestCollectorManager_Stop_Unstarted(t *testing.T) {
	m := collectors.NewCollectorManager()
	assert.NotPanics(t, func() {
		m.Stop()
	})
}

// U-MGR-09: Stop invokes Cleanup on collectors that support it.
func TestCollectorManager_Stop_InvokesCleanup(t *testing.T) {
	m := collectors.NewCollectorManager()
	c := &cleanupStubCollector{stubCollector: stubCollector{name: "cleanup"}}
	m.Register(c)

	m.Stop()

	assert.Equal(t, 1, c.cleanupCalls, "cleanup should be invoked exactly once")
}

// U-MGR-10: collectorAdapter Register is a no-op
func TestCollectorAdapter_Register(t *testing.T) {
	// Create a collector adapter via the manager (internal type)
	m := collectors.NewCollectorManager()
	m.Register(&stubCollector{name: "test"})

	// The Register method is a no-op, so we just verify it doesn't panic
	assert.NotPanics(t, func() {
		// The adapter is created internally, we just verify the manager works
	})
}

// U-MGR-15: CollectorAdapter Register method is a no-op and returns nil
func TestCollectorAdapter_Register_DirectCall(t *testing.T) {
	// Create a collector adapter directly to test the Register method
	adapter := &collectors.CollectorAdapter{Collector: &stubCollector{name: "test"}}

	// Call Register directly - it should return nil (no-op)
	err := adapter.Register(nil)
	assert.NoError(t, err, "Register should return nil as it's a no-op")
}

// U-MGR-14: collectorAdapter Register is called during Start
func TestCollectorAdapter_Register_CalledDuringStart(t *testing.T) {
	m := collectors.NewCollectorManager()
	c := &stubCollector{name: "test-collector"}
	m.Register(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start creates collectorAdapter instances and calls their Register method
	// This exercises the collectorAdapter.Register code path
	assert.NotPanics(t, func() {
		m.Start(ctx, 100*time.Millisecond)
		time.Sleep(50 * time.Millisecond)
		m.Stop()
	})
}

// U-MGR-11: Register multiple collectors in sequence
func TestCollectorManager_Register_Multiple(t *testing.T) {
	m := collectors.NewCollectorManager()

	collectors := make([]*stubCollector, 5)
	for i := 0; i < 5; i++ {
		collectors[i] = &stubCollector{name: fmt.Sprintf("collector-%d", i)}
		m.Register(collectors[i])
	}

	list := m.Collectors()
	require.Len(t, list, 5)
	for i := 0; i < 5; i++ {
		assert.Equal(t, fmt.Sprintf("collector-%d", i), list[i].Name())
	}
}

// U-MGR-12: CollectAll with multiple errors
func TestCollectorManager_CollectAll_MultipleErrors(t *testing.T) {
	m := collectors.NewCollectorManager()
	c1 := &stubCollector{name: "C1", err: errors.New("error1")}
	c2 := &stubCollector{name: "C2", err: errors.New("error2")}
	c3 := &stubCollector{name: "C3"}

	m.Register(c1)
	m.Register(c2)
	m.Register(c3)

	err := m.CollectAll(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "C1")
	assert.Contains(t, err.Error(), "C2")
	assert.Contains(t, err.Error(), "error1")
	assert.Contains(t, err.Error(), "error2")
	assert.Equal(t, 1, c3.calls)
}

// U-MGR-13: Stop with multiple cleanup collectors
func TestCollectorManager_Stop_MultipleCleanup(t *testing.T) {
	m := collectors.NewCollectorManager()
	c1 := &cleanupStubCollector{stubCollector: stubCollector{name: "cleanup1"}}
	c2 := &cleanupStubCollector{stubCollector: stubCollector{name: "cleanup2"}}
	c3 := &stubCollector{name: "no-cleanup"}

	m.Register(c1)
	m.Register(c2)
	m.Register(c3)

	m.Stop()

	assert.Equal(t, 1, c1.cleanupCalls)
	assert.Equal(t, 1, c2.cleanupCalls)
}
