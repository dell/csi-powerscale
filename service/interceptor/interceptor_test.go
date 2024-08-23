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

	"github.com/container-storage-interface/spec/lib/go/csi"

	//      "github.com/dell/csi-isilon/v2/service/interceptor"

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
	t.Run("NodeStage for same volume concurrent call", func(t *testing.T) {
		err := runTest(&csi.NodeStageVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.NodeStageVolumeRequest{VolumeId: validBlockVolumeID})
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

	t.Run("NodeStage for different volumes", func(t *testing.T) {
		err := runTest(&csi.NodeStageVolumeRequest{VolumeId: validBlockVolumeID},
			&csi.NodeStageVolumeRequest{VolumeId: validNfsVolumeID})
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
}
