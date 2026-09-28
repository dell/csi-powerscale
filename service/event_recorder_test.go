// Copyright © 2019-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
)

// capturingEventRecorder is a custom event recorder that captures events for assertions
type capturingEventRecorder struct {
	events      []capturedEvent
	lastObjRef  *corev1.ObjectReference
	lastMessage string
}

type capturedEvent struct {
	objRef    *corev1.ObjectReference
	eventType string
	reason    string
	message   string
}

func (c *capturingEventRecorder) Event(object runtime.Object, eventType, reason, message string) {
	if ref, ok := object.(*corev1.ObjectReference); ok {
		c.lastObjRef = ref
		c.events = append(c.events, capturedEvent{
			objRef:    ref,
			eventType: eventType,
			reason:    reason,
			message:   message,
		})
	}
	c.lastMessage = message
}

func (c *capturingEventRecorder) Eventf(object runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	c.Event(object, eventType, reason, fmt.Sprintf(messageFmt, args...))
}

func (c *capturingEventRecorder) AnnotatedEventf(object runtime.Object, _ map[string]string, eventType, reason, messageFmt string, args ...interface{}) {
	c.Event(object, eventType, reason, fmt.Sprintf(messageFmt, args...))
}

// TestNewControllerEventRecorder tests the newControllerEventRecorder function
func TestNewControllerEventRecorder(t *testing.T) {
	t.Run("successful event recorder creation", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		recorder, broadcaster, err := newControllerEventRecorder(clientset)

		assert.NoError(t, err)
		assert.NotNil(t, recorder)
		assert.NotNil(t, broadcaster)

		// Clean up
		broadcaster.Shutdown()
	})

	t.Run("nil clientset returns error", func(t *testing.T) {
		recorder, broadcaster, err := newControllerEventRecorder(nil)

		assert.Error(t, err)
		assert.Nil(t, recorder)
		assert.Nil(t, broadcaster)
		assert.Contains(t, err.Error(), "kubernetes clientset is nil")
	})
}

// TestCreateEventRecorder tests the createEventRecorder function
func TestCreateEventRecorder(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		recorder, broadcaster, err := createEventRecorder(clientset)

		assert.NoError(t, err)
		assert.NotNil(t, recorder)
		assert.NotNil(t, broadcaster)

		// Clean up
		broadcaster.Shutdown()
	})
}

// TestInitEventRecorder tests the initEventRecorder method
func TestInitEventRecorder(t *testing.T) {
	ctx := context.Background()

	t.Run("successful initialization", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		svc := &service{
			k8sclient: clientset,
		}

		svc.initEventRecorder(ctx)

		assert.NotNil(t, svc.eventRecorder)
		assert.NotNil(t, svc.eventBroadcaster)

		// Clean up
		svc.shutdownEventRecorder()
	})

	t.Run("nil k8sclient skips initialization", func(t *testing.T) {
		svc := &service{
			k8sclient: nil,
		}

		svc.initEventRecorder(ctx)

		assert.Nil(t, svc.eventRecorder)
		assert.Nil(t, svc.eventBroadcaster)
	})

	t.Run("newEventRecorderFunc failure logs warning", func(t *testing.T) {
		// Save original function
		originalFunc := newEventRecorderFunc
		defer func() { newEventRecorderFunc = originalFunc }()

		// Mock the function to return an error
		newEventRecorderFunc = func(_ kubernetes.Interface) (record.EventRecorder, record.EventBroadcaster, error) {
			return nil, nil, errors.New("mock error")
		}

		clientset := fake.NewSimpleClientset()
		svc := &service{
			k8sclient: clientset,
		}

		svc.initEventRecorder(ctx)

		assert.Nil(t, svc.eventRecorder)
		assert.Nil(t, svc.eventBroadcaster)
	})
}

// TestShutdownEventRecorder tests the shutdownEventRecorder method
func TestShutdownEventRecorder(t *testing.T) {
	t.Run("shutdown with real broadcaster", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		svc := &service{
			k8sclient: clientset,
		}

		// Initialize first
		svc.initEventRecorder(context.Background())
		assert.NotNil(t, svc.eventBroadcaster)

		// Shutdown should not panic
		assert.NotPanics(t, func() {
			svc.shutdownEventRecorder()
		})
	})

	t.Run("shutdown without broadcaster does not panic", func(t *testing.T) {
		svc := &service{
			eventBroadcaster: nil,
		}

		// Should not panic
		assert.NotPanics(t, func() {
			svc.shutdownEventRecorder()
		})
	})
}

// TestEmitProvisioningEvent tests the emitProvisioningEvent method
func TestEmitProvisioningEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("event emitted successfully with PVC lookup", func(t *testing.T) {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test-pvc",
				Namespace:       "test-namespace",
				UID:             "test-uid-12345",
				ResourceVersion: "12345",
			},
		}
		clientset := fake.NewSimpleClientset(pvc)
		capturer := &capturingEventRecorder{}

		svc := &service{
			k8sclient:     clientset,
			eventRecorder: capturer,
		}

		svc.emitProvisioningEvent(ctx, corev1.EventTypeWarning, "TestReason", "Test message", "test-pvc", "test-namespace")

		assert.Len(t, capturer.events, 1)
		assert.Equal(t, "TestReason", capturer.events[0].reason)
		assert.Equal(t, "Test message", capturer.events[0].message)
		assert.Equal(t, corev1.EventTypeWarning, capturer.events[0].eventType)
		assert.Equal(t, "test-pvc", capturer.events[0].objRef.Name)
		assert.Equal(t, "test-namespace", capturer.events[0].objRef.Namespace)
		assert.Equal(t, "test-uid-12345", string(capturer.events[0].objRef.UID))
	})

	t.Run("event emitted with empty namespace defaults to 'default'", func(t *testing.T) {
		capturer := &capturingEventRecorder{}
		svc := &service{
			k8sclient:     fake.NewSimpleClientset(),
			eventRecorder: capturer,
		}

		svc.emitProvisioningEvent(ctx, corev1.EventTypeWarning, "TestReason", "Test message", "test-pvc", "")

		assert.Len(t, capturer.events, 1)
		assert.Equal(t, "default", capturer.events[0].objRef.Namespace)
	})

	t.Run("event emitted when PVC lookup fails", func(t *testing.T) {
		// Create clientset without the PVC
		clientset := fake.NewSimpleClientset()
		capturer := &capturingEventRecorder{}

		svc := &service{
			k8sclient:     clientset,
			eventRecorder: capturer,
		}

		svc.emitProvisioningEvent(ctx, corev1.EventTypeWarning, "TestReason", "Test message", "nonexistent-pvc", "test-namespace")

		// Event should still be emitted even if PVC lookup fails
		assert.Len(t, capturer.events, 1)
		assert.Equal(t, "TestReason", capturer.events[0].reason)
		assert.Equal(t, "", string(capturer.events[0].objRef.UID)) // UID should be empty
	})

	t.Run("no event emitted when eventRecorder is nil", func(t *testing.T) {
		svc := &service{
			k8sclient:     fake.NewSimpleClientset(),
			eventRecorder: nil,
		}

		// Should not panic and should be a no-op
		assert.NotPanics(t, func() {
			svc.emitProvisioningEvent(ctx, corev1.EventTypeWarning, "TestReason", "Test message", "test-pvc", "test-namespace")
		})
	})

	t.Run("event emitted when k8sclient is nil", func(t *testing.T) {
		capturer := &capturingEventRecorder{}
		svc := &service{
			k8sclient:     nil,
			eventRecorder: capturer,
		}

		svc.emitProvisioningEvent(ctx, corev1.EventTypeWarning, "TestReason", "Test message", "test-pvc", "test-namespace")

		// Event should still be emitted
		assert.Len(t, capturer.events, 1)
		assert.Equal(t, "TestReason", capturer.events[0].reason)
	})

	t.Run("PVC lookup error is handled gracefully", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		// Add a reactor to simulate an error
		clientset.Fake.PrependReactor("get", "persistentvolumeclaims", func(_ k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("simulated API error")
		})

		capturer := &capturingEventRecorder{}
		svc := &service{
			k8sclient:     clientset,
			eventRecorder: capturer,
		}

		svc.emitProvisioningEvent(ctx, corev1.EventTypeWarning, "TestReason", "Test message", "test-pvc", "test-namespace")

		// Event should still be emitted
		assert.Len(t, capturer.events, 1)
		assert.Equal(t, "", string(capturer.events[0].objRef.UID))
	})
}

// TestEmitSharedExportNotFoundEvent tests the emitSharedExportNotFoundEvent method
func TestEmitSharedExportNotFoundEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("SharedExportNotFound event emitted with correct message", func(t *testing.T) {
		capturer := &capturingEventRecorder{}
		svc := &service{
			k8sclient:     fake.NewSimpleClientset(),
			eventRecorder: capturer,
		}

		svc.emitSharedExportNotFoundEvent(ctx, "my-pvc", "my-namespace", "/ifs/shared/export", "System")

		assert.Len(t, capturer.events, 1)
		event := capturer.events[0]

		assert.Equal(t, EventReasonSharedExportNotFound, event.reason)
		assert.Equal(t, corev1.EventTypeWarning, event.eventType)
		assert.Equal(t, "my-pvc", event.objRef.Name)
		assert.Equal(t, "my-namespace", event.objRef.Namespace)
		assert.Contains(t, event.message, "/ifs/shared/export")
		assert.Contains(t, event.message, "System")
		assert.Contains(t, event.message, "Ensure the shared export is pre-created by an administrator")
	})

	t.Run("SharedExportNotFound event with empty namespace", func(t *testing.T) {
		capturer := &capturingEventRecorder{}
		svc := &service{
			k8sclient:     fake.NewSimpleClientset(),
			eventRecorder: capturer,
		}

		svc.emitSharedExportNotFoundEvent(ctx, "my-pvc", "", "/ifs/shared/export", "System")

		assert.Len(t, capturer.events, 1)
		assert.Equal(t, "default", capturer.events[0].objRef.Namespace)
	})

	t.Run("no event when recorder is nil", func(t *testing.T) {
		svc := &service{
			k8sclient:     fake.NewSimpleClientset(),
			eventRecorder: nil,
		}

		// Should not panic
		assert.NotPanics(t, func() {
			svc.emitSharedExportNotFoundEvent(ctx, "my-pvc", "my-namespace", "/ifs/shared/export", "System")
		})
	})
}

// TestEventReasonConstants tests that event reason constants are defined correctly
func TestEventReasonConstants(t *testing.T) {
	t.Run("SharedExportNotFound constant is defined", func(t *testing.T) {
		assert.Equal(t, "SharedExportNotFound", EventReasonSharedExportNotFound)
	})

	t.Run("pvcLookupTimeout is reasonable", func(t *testing.T) {
		assert.Equal(t, int64(5*1e9), int64(pvcLookupTimeout)) // 5 seconds in nanoseconds
	})
}

// TestEmitProvisioningEventObjectReference tests the ObjectReference fields
func TestEmitProvisioningEventObjectReference(t *testing.T) {
	ctx := context.Background()

	t.Run("ObjectReference has correct APIVersion and Kind", func(t *testing.T) {
		capturer := &capturingEventRecorder{}
		svc := &service{
			k8sclient:     fake.NewSimpleClientset(),
			eventRecorder: capturer,
		}

		svc.emitProvisioningEvent(ctx, corev1.EventTypeWarning, "TestReason", "Test message", "test-pvc", "test-namespace")

		assert.Len(t, capturer.events, 1)
		objRef := capturer.events[0].objRef
		assert.Equal(t, "v1", objRef.APIVersion)
		assert.Equal(t, "PersistentVolumeClaim", objRef.Kind)
	})
}

// TestEmitSharedExportNotFoundEventMessageFormat tests the message format
func TestEmitSharedExportNotFoundEventMessageFormat(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name          string
		sharedPath    string
		accessZone    string
		expectedInMsg []string
	}{
		{
			name:          "standard path and zone",
			sharedPath:    "/ifs/data/shared",
			accessZone:    "System",
			expectedInMsg: []string{"/ifs/data/shared", "System", "pre-created by an administrator"},
		},
		{
			name:          "custom access zone",
			sharedPath:    "/ifs/tenant-a/exports",
			accessZone:    "tenant-a-zone",
			expectedInMsg: []string{"/ifs/tenant-a/exports", "tenant-a-zone"},
		},
		{
			name:          "path with special characters",
			sharedPath:    "/ifs/data-2024/shared_export",
			accessZone:    "Zone-1",
			expectedInMsg: []string{"/ifs/data-2024/shared_export", "Zone-1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			capturer := &capturingEventRecorder{}
			svc := &service{
				k8sclient:     fake.NewSimpleClientset(),
				eventRecorder: capturer,
			}

			svc.emitSharedExportNotFoundEvent(ctx, "test-pvc", "test-ns", tc.sharedPath, tc.accessZone)

			assert.Len(t, capturer.events, 1)
			for _, expected := range tc.expectedInMsg {
				assert.True(t, strings.Contains(capturer.events[0].message, expected),
					"Expected message to contain '%s', got '%s'", expected, capturer.events[0].message)
			}
		})
	}
}

// TestNewEventRecorderFuncMocking tests that newEventRecorderFunc can be mocked
func TestNewEventRecorderFuncMocking(t *testing.T) {
	ctx := context.Background()

	t.Run("mock function is called", func(t *testing.T) {
		// Save original
		originalFunc := newEventRecorderFunc
		defer func() { newEventRecorderFunc = originalFunc }()

		mockCalled := false
		mockRecorder := &capturingEventRecorder{}

		// Create a real broadcaster for the test
		clientset := fake.NewSimpleClientset()
		_, realBroadcaster, _ := createEventRecorder(clientset)
		defer realBroadcaster.Shutdown()

		newEventRecorderFunc = func(_ kubernetes.Interface) (record.EventRecorder, record.EventBroadcaster, error) {
			mockCalled = true
			return mockRecorder, realBroadcaster, nil
		}

		svc := &service{
			k8sclient: clientset,
		}

		svc.initEventRecorder(ctx)

		assert.True(t, mockCalled)
		assert.Equal(t, mockRecorder, svc.eventRecorder)
	})
}

// TestEmitProvisioningEventWithRealPVC tests event emission with a real PVC in the fake clientset
func TestEmitProvisioningEventWithRealPVC(t *testing.T) {
	ctx := context.Background()

	t.Run("PVC UID and ResourceVersion are populated", func(t *testing.T) {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "my-pvc",
				Namespace:       "my-namespace",
				UID:             "uid-abc-123",
				ResourceVersion: "rv-456",
			},
		}
		clientset := fake.NewSimpleClientset(pvc)
		capturer := &capturingEventRecorder{}

		svc := &service{
			k8sclient:     clientset,
			eventRecorder: capturer,
		}

		svc.emitProvisioningEvent(ctx, corev1.EventTypeWarning, "TestReason", "Test message", "my-pvc", "my-namespace")

		assert.Len(t, capturer.events, 1)
		assert.Equal(t, "uid-abc-123", string(capturer.events[0].objRef.UID))
		assert.Equal(t, "rv-456", capturer.events[0].objRef.ResourceVersion)
	})
}

// TestEmitProvisioningEventTypes tests different event types
func TestEmitProvisioningEventTypes(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name      string
		eventType string
	}{
		{"Warning event", corev1.EventTypeWarning},
		{"Normal event", corev1.EventTypeNormal},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			capturer := &capturingEventRecorder{}
			svc := &service{
				k8sclient:     fake.NewSimpleClientset(),
				eventRecorder: capturer,
			}

			svc.emitProvisioningEvent(ctx, tc.eventType, "TestReason", "Test message", "test-pvc", "test-namespace")

			assert.Len(t, capturer.events, 1)
			assert.Equal(t, tc.eventType, capturer.events[0].eventType)
		})
	}
}
