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
	"fmt"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

// Event reason constants for directory-backed provisioning
const (
	// EventReasonSharedExportNotFound is emitted when the shared export specified
	// in the StorageClass does not exist in the specified access zone.
	EventReasonSharedExportNotFound = "SharedExportNotFound"
)

// pvcLookupTimeout is the timeout for fetching PVC details for event emission
const pvcLookupTimeout = 5 * time.Second

// newEventRecorderFunc is a package-level variable to allow test mocking
var newEventRecorderFunc = newControllerEventRecorder

// newControllerEventRecorder creates a Kubernetes event recorder and broadcaster for the controller service.
// The caller must call broadcaster.Shutdown() during service shutdown to stop background goroutines.
func newControllerEventRecorder(clientset kubernetes.Interface) (record.EventRecorder, record.EventBroadcaster, error) {
	if clientset == nil {
		return nil, nil, fmt.Errorf("kubernetes clientset is nil")
	}
	return createEventRecorder(clientset)
}

// createEventRecorder creates a record.EventRecorder and EventBroadcaster from a Kubernetes clientset.
func createEventRecorder(clientset kubernetes.Interface) (record.EventRecorder, record.EventBroadcaster, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedv1core.EventSinkImpl{Interface: clientset.CoreV1().Events("")})

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, nil, fmt.Errorf("failed to add corev1 to scheme: %w", err)
	}

	eventRecorder := eventBroadcaster.NewRecorder(scheme, corev1.EventSource{Component: constants.PluginName})
	return eventRecorder, eventBroadcaster, nil
}

// initEventRecorder initializes the event recorder for the service.
// This should be called during BeforeServe when running in controller mode.
// If initialization fails, the service continues without event recording capability.
func (s *service) initEventRecorder(ctx context.Context) {
	if s.k8sclient == nil {
		csmlog.WithContext(ctx).Warn("Kubernetes client not available, event recorder will not be initialized")
		return
	}

	eventRecorder, eventBroadcaster, err := newEventRecorderFunc(s.k8sclient)
	if err != nil {
		csmlog.WithContext(ctx).Warnf("Failed to create event recorder: %v (events will not be emitted)", err)
		return
	}

	s.eventRecorder = eventRecorder
	s.eventBroadcaster = eventBroadcaster
	csmlog.WithContext(ctx).Info("Event recorder initialized successfully")
}

// shutdownEventRecorder stops the event broadcaster if it was initialized.
// This should be called during service shutdown.
func (s *service) shutdownEventRecorder() {
	if s.eventBroadcaster != nil {
		s.eventBroadcaster.Shutdown()
	}
}

// emitProvisioningEvent emits a Kubernetes event for provisioning operations.
// If the event recorder is nil, this is a no-op.
// The pvcName and namespace parameters should be extracted from the CreateVolume request parameters.
// If namespace is empty, the event is emitted in "default" as a fallback.
func (s *service) emitProvisioningEvent(ctx context.Context, eventType, reason, message, pvcName, namespace string) {
	if s.eventRecorder == nil {
		return
	}
	if namespace == "" {
		namespace = "default"
	}

	objRef := &corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
		Name:       pvcName,
		Namespace:  namespace,
	}

	// Fetch PVC to get its UID for proper event association in kubectl describe
	if s.k8sclient != nil {
		lookupCtx, cancel := context.WithTimeout(ctx, pvcLookupTimeout)
		defer cancel()
		pvc, err := s.k8sclient.CoreV1().PersistentVolumeClaims(namespace).Get(lookupCtx, pvcName, metav1.GetOptions{})
		if err == nil {
			objRef.UID = pvc.UID
			objRef.ResourceVersion = pvc.ResourceVersion
		} else {
			csmlog.WithContext(ctx).Debugf("Failed to fetch PVC %s/%s for event UID: %v", namespace, pvcName, err)
		}
	}

	s.eventRecorder.Event(objRef, eventType, reason, message)
	csmlog.WithContext(ctx).Infof("Emitted event: type=%s, reason=%s, pvc=%s/%s, message=%s", eventType, reason, namespace, pvcName, message)
}

// emitSharedExportNotFoundEvent emits a SharedExportNotFound event on the PVC.
// This is called when directory-backed provisioning fails because the shared export does not exist.
func (s *service) emitSharedExportNotFoundEvent(ctx context.Context, pvcName, namespace, sharedExportPath, accessZone string) {
	message := fmt.Sprintf("Shared export not found at path '%s' in access zone '%s'. Ensure the shared export is pre-created by an administrator.",
		sharedExportPath, accessZone)
	s.emitProvisioningEvent(ctx, corev1.EventTypeWarning, EventReasonSharedExportNotFound, message, pvcName, namespace)
}
