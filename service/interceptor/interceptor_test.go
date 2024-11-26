/*
 *
 * Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package interceptor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/akutz/gosync"
	"github.com/container-storage-interface/spec/lib/go/csi"
	controller "github.com/dell/csi-isilon/v2/service"
	"github.com/dell/csi-metadata-retriever/retriever"
	csictx "github.com/dell/gocsi/context"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	validBlockVolumeID = "39bb1b5f-5624-490d-9ece-18f7b28a904e/192.168.0.1/scsi"
	validNfsVolumeID   = "39bb1b5f-5624-490d-9ece-18f7b28a904e/192.168.0.2/nfs"
	testID             = "111"
)

func getSleepHandler(millisec int) grpc.UnaryHandler {
	return func(_ context.Context, _ interface{}) (interface{}, error) {
		fmt.Println("start sleep")
		time.Sleep(time.Duration(millisec) * time.Millisecond)
		fmt.Println("stop sleep")
		return nil, nil
	}
}

func testHandler(ctx context.Context, _ interface{}) (interface{}, error) {
	return ctx, nil
}

func TestRewriteRequestIDInterceptor_RequestIDExist(t *testing.T) {
	handleInterceptor := NewRewriteRequestIDInterceptor()
	md := metadata.Pairs()
	ctx := metadata.NewIncomingContext(context.Background(), md)
	md[csictx.RequestIDKey] = []string{testID}

	newCtx, _ := handleInterceptor(ctx, nil, nil, testHandler)
	requestID, ok := newCtx.(context.Context).Value(csictx.RequestIDKey).(string)

	assert.Equal(t, ok, true)
	assert.Equal(t, requestID, fmt.Sprintf("%s-%s", csictx.RequestIDKey, testID))
}

func TestGetLockWithName(t *testing.T) {
	// Create a new lockProvider instance
	lockProvider := &lockProvider{
		volNameLocks: make(map[string]gosync.TryLocker),
	}

	// Create a new context
	ctx := context.Background()

	// Call the GetLockWithName function
	lock, err := lockProvider.GetLockWithName(ctx, "test")

	// Assert the expected lock
	if lock == nil {
		t.Errorf("Expected lock to be non-nil, but it was nil")
	}

	// Assert the expected error
	if err != nil {
		t.Errorf("Expected error to be nil, but it was %v", err)
	}

	// Call the GetLockWithName function again with the same name
	lock2, err2 := lockProvider.GetLockWithName(ctx, "test")

	// Assert the expected lock
	if lock2 == nil {
		t.Errorf("Expected lock to be non-nil, but it was nil")
	}

	// Assert the expected error
	if err2 != nil {
		t.Errorf("Expected error to be nil, but it was %v", err2)
	}

	// Assert that the two locks are the same
	if lock != lock2 {
		t.Errorf("Expected locks to be the same, but they were different")
	}
}

func TestNewCustomSerialLock(t *testing.T) {
	ctx := context.Background()
	serialLock := NewCustomSerialLock()

	runTest := func(req1 interface{}, req2 interface{}) error {
		wg := sync.WaitGroup{}
		h := getSleepHandler(300)
		wg.Add(1)
		go func() {
			_, _ = serialLock(ctx, req1, nil, h)
			wg.Done()
		}()
		time.Sleep(time.Millisecond * 100)
		_, err := serialLock(ctx, req2, nil, h)
		wg.Wait()
		return err
	}

	t.Run("ControllerPublishVolume for same volume concurrent call", func(t *testing.T) {
		err := runTest(&csi.ControllerPublishVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.ControllerPublishVolumeRequest{VolumeId: validBlockVolumeID})
		assert.Nil(t, err)
	})

	t.Run("ControllerPublishVolume for different volumes", func(t *testing.T) {
		err := runTest(&csi.ControllerPublishVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.ControllerPublishVolumeRequest{VolumeId: validNfsVolumeID})
		assert.Nil(t, err)
	})

	t.Run("ControllerUnpublishVolume for same volume concurrent call", func(t *testing.T) {
		err := runTest(&csi.ControllerUnpublishVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.ControllerUnpublishVolumeRequest{VolumeId: validBlockVolumeID})
		assert.Nil(t, err)
	})

	t.Run("ControllerUnpublishVolume for different volumes", func(t *testing.T) {
		err := runTest(&csi.ControllerUnpublishVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.ControllerUnpublishVolumeRequest{VolumeId: validNfsVolumeID})
		assert.Nil(t, err)
	})

	t.Run("CreateVolume for same volume concurrent call", func(t *testing.T) {
		err := runTest(&csi.CreateVolumeRequest{Name: validBlockVolumeID},
			&csi.CreateVolumeRequest{Name: validBlockVolumeID})
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "pending")
	})

	t.Run("CreateVolume for different volumes", func(t *testing.T) {
		err := runTest(&csi.CreateVolumeRequest{Name: validBlockVolumeID},
			&csi.CreateVolumeRequest{Name: validNfsVolumeID})
		assert.Nil(t, err)
	})

	t.Run("DeleteVolume for same volume concurrent call", func(t *testing.T) {
		err := runTest(&csi.DeleteVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.DeleteVolumeRequest{VolumeId: validBlockVolumeID})
		assert.Nil(t, err)
	})

	t.Run("DeleteVolume for different volumes", func(t *testing.T) {
		err := runTest(&csi.DeleteVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.DeleteVolumeRequest{VolumeId: validNfsVolumeID})
		assert.Nil(t, err)
	})

	t.Run("NodeStage for same volume concurrent call", func(t *testing.T) {
		err := runTest(&csi.NodeStageVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.NodeStageVolumeRequest{VolumeId: validBlockVolumeID})
		assert.Nil(t, err)
	})

	t.Run("NodeStage for different volumes", func(t *testing.T) {
		err := runTest(&csi.NodeStageVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.NodeStageVolumeRequest{VolumeId: validNfsVolumeID})
		assert.Nil(t, err)
	})

	t.Run("NodeUnstage for same volume concurrent call", func(t *testing.T) {
		err := runTest(&csi.NodeUnstageVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.NodeUnstageVolumeRequest{VolumeId: validBlockVolumeID})
		assert.Nil(t, err)
	})

	t.Run("NodeUnstage for different volumes", func(t *testing.T) {
		err := runTest(&csi.NodeUnstageVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.NodeUnstageVolumeRequest{VolumeId: validNfsVolumeID})
		assert.Nil(t, err)
	})

	t.Run("NodePublish for same volume concurrent call", func(t *testing.T) {
		err := runTest(&csi.NodePublishVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.NodePublishVolumeRequest{VolumeId: validBlockVolumeID})
		assert.Nil(t, err)
	})

	t.Run("NodePublish and NodeStage for same volume concurrent call", func(t *testing.T) {
		err := runTest(&csi.NodeStageVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.NodePublishVolumeRequest{VolumeId: validBlockVolumeID})
		assert.Nil(t, err)
	})

	t.Run("NodeUnpublishVolume for same volume concurrent call", func(t *testing.T) {
		err := runTest(&csi.NodeUnpublishVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.NodeUnpublishVolumeRequest{VolumeId: validBlockVolumeID})
		assert.Nil(t, err)
	})

	t.Run("NodeUnpublishVolume for different volumes", func(t *testing.T) {
		err := runTest(&csi.NodeUnpublishVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.NodeUnpublishVolumeRequest{VolumeId: validNfsVolumeID})
		assert.Nil(t, err)
	})
}

func TestCreateVolume(t *testing.T) {
	// Create a new lockProvider instance
	lockProvider := &lockProvider{
		volIDLocks:   make(map[string]gosync.TryLocker),
		volNameLocks: make(map[string]gosync.TryLocker),
	}

	// Create a new interceptor instance
	i := &interceptor{
		opts: opts{
			locker:                lockProvider,
			MetadataSidecarClient: &mockMetadataSidecarClient{},
			timeout:               time.Second,
		},
	}

	// Create a new context
	ctx := context.Background()

	// Create a new CreateVolumeRequest
	req := &csi.CreateVolumeRequest{
		Name: "test-volume",
		Parameters: map[string]string{
			controller.KeyCSIPVCName:      "test-pvc",
			controller.KeyCSIPVCNamespace: "test-namespace",
		},
	}

	// Create a new UnaryHandler
	handler := func(_ context.Context, _ interface{}) (interface{}, error) {
		return &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				VolumeId: "test-volume-id",
			},
		}, nil
	}

	// Call the createVolume function
	res, err := i.createVolume(ctx, req, nil, handler)

	// Assert the expected response
	if res == nil {
		t.Errorf("Expected non-nil response, but it was nil")
	}

	// Assert the expected error
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}

	// Assert the expected volume ID
	if res.(*csi.CreateVolumeResponse).Volume.VolumeId != "test-volume-id" {
		t.Errorf("Expected volume ID to be %s, but it was %s", "test-volume-id", res.(*csi.CreateVolumeResponse).Volume.VolumeId)
	}
}

// mockMetadataSidecarClient is a mock implementation of the retriever.MetadataRetrieverClient interface
type mockMetadataSidecarClient struct{}

// GetPVCLabels is a mock implementation of the GetPVCLabels method
func (c *mockMetadataSidecarClient) GetPVCLabels(_ context.Context, _ *retriever.GetPVCLabelsRequest) (*retriever.GetPVCLabelsResponse, error) {
	return &retriever.GetPVCLabelsResponse{
		Parameters: map[string]string{
			"test-key": "test-value",
		},
	}, nil
}
