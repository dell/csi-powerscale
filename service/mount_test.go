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
//

package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ecosystems/container-storage-modules/src/gofsutil"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Test_publishVolume_AuthFailureMetric verifies that authentication failure metric
// is recorded when NFS mount fails with "access denied by server while mounting"
func Test_publishVolume_AuthFailureMetric(t *testing.T) {
	// Save original functions
	defaultGetMountFunc := getMountFunc
	defaultGetGetMountsFunc := getGetMountsFunc
	defaultRecordAuthFailureFunc := recordAuthFailureFunc
	defaultMetricsEnabled := metricsEnabled

	defer func() {
		getMountFunc = defaultGetMountFunc
		getGetMountsFunc = defaultGetGetMountsFunc
		recordAuthFailureFunc = defaultRecordAuthFailureFunc
		metricsEnabled = defaultMetricsEnabled
	}()

	t.Run("metrics disabled - no recording", func(t *testing.T) {
		authFailureCalled := false
		recordAuthFailureFunc = func() {
			authFailureCalled = true
		}
		metricsEnabled = false

		// Mock GetMounts to return empty list
		getGetMountsFunc = func() func(ctx context.Context) ([]gofsutil.Info, error) {
			return func(_ context.Context) ([]gofsutil.Info, error) {
				return []gofsutil.Info{}, nil
			}
		}

		// Simulate auth failure on mount
		getMountFunc = func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
			return func(_ context.Context, _, _, _ string, _ ...string) error {
				return fmt.Errorf("access denied by server while mounting")
			}
		}

		req := &csi.NodePublishVolumeRequest{
			VolumeId:   "vol-1",
			TargetPath: filepath.Join(t.TempDir(), "target"),
			VolumeCapability: &csi.VolumeCapability{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						MountFlags: []string{},
					},
				},
			},
		}

		err := publishVolume(context.Background(), req, "192.0.2.181:/ifs/data/csi/vol-1")

		// Should fail with auth error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")

		// Auth failure metric should NOT have been called (metrics disabled)
		assert.False(t, authFailureCalled, "RecordAuthFailure should NOT be called when metrics disabled")
	})

	t.Run("metrics enabled - recording occurs", func(t *testing.T) {
		authFailureCalled := false
		recordAuthFailureFunc = func() {
			authFailureCalled = true
		}
		metricsEnabled = true

		// Mock GetMounts to return empty list
		getGetMountsFunc = func() func(ctx context.Context) ([]gofsutil.Info, error) {
			return func(_ context.Context) ([]gofsutil.Info, error) {
				return []gofsutil.Info{}, nil
			}
		}

		// Simulate auth failure on mount
		getMountFunc = func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
			return func(_ context.Context, _, _, _ string, _ ...string) error {
				return fmt.Errorf("access denied by server while mounting")
			}
		}

		req := &csi.NodePublishVolumeRequest{
			VolumeId:   "vol-1",
			TargetPath: filepath.Join(t.TempDir(), "target"),
			VolumeCapability: &csi.VolumeCapability{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						MountFlags: []string{},
					},
				},
			},
		}

		err := publishVolume(context.Background(), req, "192.0.2.181:/ifs/data/csi/vol-1")

		// Should fail with auth error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")

		// Auth failure metric should have been called (metrics enabled)
		assert.True(t, authFailureCalled, "RecordAuthFailure should be called when metrics enabled")
	})

	t.Run("metrics enabled - other error no recording", func(t *testing.T) {
		authFailureCalled := false
		recordAuthFailureFunc = func() {
			authFailureCalled = true
		}
		metricsEnabled = true

		// Mock GetMounts to return empty list
		getGetMountsFunc = func() func(ctx context.Context) ([]gofsutil.Info, error) {
			return func(_ context.Context) ([]gofsutil.Info, error) {
				return []gofsutil.Info{}, nil
			}
		}

		// Simulate non-auth error on mount
		getMountFunc = func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
			return func(_ context.Context, _, _, _ string, _ ...string) error {
				return fmt.Errorf("network unreachable")
			}
		}

		req := &csi.NodePublishVolumeRequest{
			VolumeId:   "vol-1",
			TargetPath: filepath.Join(t.TempDir(), "target"),
			VolumeCapability: &csi.VolumeCapability{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						MountFlags: []string{},
					},
				},
			},
		}

		err := publishVolume(context.Background(), req, "192.0.2.181:/ifs/data/csi/vol-1")

		// Should fail with error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "network unreachable")

		// Auth failure metric should NOT have been called (not an auth error)
		assert.False(t, authFailureCalled, "RecordAuthFailure should NOT be called for non-auth errors")
	})
}

// Split TestMkdir into separate tests to test each case
func TestMkdirCreateDir(t *testing.T) {
	ctx := context.Background()

	// Make temp directory to use for testing
	basepath, err := os.MkdirTemp("/tmp", "*")
	assert.NoError(t, err)
	defer os.RemoveAll(basepath)
	path := basepath + "/test"

	// Test creating a directory
	created, err := mkdir(ctx, path)
	assert.NoError(t, err)
	assert.True(t, created)
}

func TestMkdirExistingDir(t *testing.T) {
	ctx := context.Background()

	// Make temp directory to use for testing
	basepath, err := os.MkdirTemp("/tmp", "*")
	assert.NoError(t, err)
	defer os.RemoveAll(basepath)

	path := basepath + "/test"

	// Pre-create the directory
	err = os.Mkdir(path, 0o755)
	assert.NoError(t, err)

	// Test creating an existing directory
	created, err := mkdir(ctx, path)
	assert.NoError(t, err)
	assert.False(t, created)
}

func TestMkdirExistingFile(t *testing.T) {
	ctx := context.Background()

	// Make temp directory to use for testing
	basepath, err := os.MkdirTemp("/tmp", "*")
	assert.NoError(t, err)
	defer os.RemoveAll(basepath)

	path := basepath + "/file"
	file, err := os.Create(path)
	assert.NoError(t, err)
	file.Close()

	// Test creating a directory with an existing file path
	created, err := mkdir(ctx, path)
	assert.Error(t, err)
	assert.False(t, created)
}

func TestMkdirNotExistingFile(t *testing.T) {
	ctx := context.Background()

	// Make temp directory to use for testing
	basepath, err := os.MkdirTemp("/tmp", "*")
	assert.NoError(t, err)
	defer os.RemoveAll(basepath)

	path := ""
	file, err := os.Create(path)
	assert.Error(t, err)
	file.Close()

	// Test creating a directory with an existing file path
	created, err := mkdir(ctx, path)
	assert.Error(t, err)
	assert.False(t, created)
}

func Test_isVolumeMounted(t *testing.T) {
	defaultGetGetMountsFunc := getGetMountsFunc

	after := func() {
		getGetMountsFunc = defaultGetGetMountsFunc
	}

	type args struct {
		ctx       context.Context
		filterStr string
		target    string
	}
	tests := []struct {
		name             string
		getGetMountsFunc func() func(ctx context.Context) ([]gofsutil.Info, error)
		args             args
		want             bool
		wantErr          bool
	}{
		{
			name: "failure in getting mounts",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					return nil, fmt.Errorf("injected error for unit test")
				}
			},
			args: args{
				ctx:       context.Background(),
				filterStr: "",
				target:    "/",
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "no mounts exist",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					return nil, nil
				}
			},
			args: args{
				ctx:       context.Background(),
				filterStr: "",
				target:    "/",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "not found",
			args: args{
				ctx:       context.Background(),
				filterStr: "test",
				target:    "/test",
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer after()

			if tt.getGetMountsFunc != nil {
				getGetMountsFunc = tt.getGetMountsFunc
			}

			got, err := isVolumeMounted(tt.args.ctx, tt.args.filterStr, tt.args.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("isVolumeMounted() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isVolumeMounted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_contains(t *testing.T) {
	type args struct {
		list []string
		item string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "empty list",
			args: args{
				list: []string{},
				item: "test",
			},
			want: false,
		},
		{
			name: "single item in list",
			args: args{
				list: []string{"test"},
				item: "test",
			},
			want: true,
		},
		{
			name: "multiple items in list",
			args: args{
				list: []string{"test1", "test2", "test3"},
				item: "test2",
			},
			want: true,
		},
		{
			name: "item not in list",
			args: args{
				list: []string{"test1", "test2", "test3"},
				item: "test4",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contains(tt.args.list, tt.args.item); got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_unpublishVolume(t *testing.T) {
	defaultGetGetMountsFunc := getGetMountsFunc
	defaultGetMountFunc := getMountFunc
	defaultGetUnmountFunc := getUnmountFunc
	defaultGetOsRemoveAllFunc := getOsRemoveAllFunc

	after := func() {
		getGetMountsFunc = defaultGetGetMountsFunc
		getMountFunc = defaultGetMountFunc
		getUnmountFunc = defaultGetUnmountFunc
		getOsRemoveAllFunc = defaultGetOsRemoveAllFunc
	}

	type args struct {
		ctx       context.Context
		req       *csi.NodeUnpublishVolumeRequest
		filterStr string
	}
	tests := []struct {
		name               string
		getGetMountsFunc   func() func(ctx context.Context) ([]gofsutil.Info, error)
		getMountFunc       func() func(ctx context.Context, source, target, fsType string, opts ...string) error
		getUnmountFunc     func() func(ctx context.Context, target string) error
		getOsRemoveAllFunc func() func(path string) error
		args               args
		wantErr            bool
	}{
		{
			name: "empty request",
			args: args{
				ctx:       context.Background(),
				req:       &csi.NodeUnpublishVolumeRequest{},
				filterStr: "",
			},
			wantErr: true,
		},
		{
			name: "volume not mounted",
			args: args{
				ctx: context.Background(),
				req: &csi.NodeUnpublishVolumeRequest{
					VolumeId:   "test",
					TargetPath: "/test",
				},
				filterStr: "test",
			},
			wantErr: false,
		},
		{
			name: "error determining volume is mounted",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					return nil, fmt.Errorf("injected error for unit test")
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodeUnpublishVolumeRequest{
					VolumeId:   "test",
					TargetPath: "/test",
				},
				filterStr: "test",
			},
			wantErr: true,
		},
		{
			name: "failure to unmount",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					mounts := []gofsutil.Info{
						{
							Path:   "/data1",
							Opts:   []string{"rw", "remount"},
							Device: "testdata1",
						},
					}
					return mounts, nil
				}
			},
			getUnmountFunc: func() func(ctx context.Context, target string) error {
				return func(_ context.Context, _ string) error {
					return fmt.Errorf("injected error for unit test")
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodeUnpublishVolumeRequest{
					VolumeId:   "testdata1",
					TargetPath: "/data1",
				},
				filterStr: "data1",
			},
			wantErr: true,
		},
		{
			name: "successful unmount",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					mounts := []gofsutil.Info{
						{
							Path:   "/data1",
							Opts:   []string{"rw", "remount"},
							Device: "testdata1",
						},
					}
					return mounts, nil
				}
			},
			getUnmountFunc: func() func(ctx context.Context, target string) error {
				return func(_ context.Context, _ string) error {
					return nil
				}
			},
			getOsRemoveAllFunc: func() func(path string) error {
				return func(_ string) error {
					return nil
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodeUnpublishVolumeRequest{
					VolumeId:   "testdata1",
					TargetPath: "/data1",
				},
				filterStr: "data1",
			},
			wantErr: false,
		},
		{
			name: "cleanup fails after unmount",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					mounts := []gofsutil.Info{
						{
							Path:   "/data1",
							Opts:   []string{"rw", "remount"},
							Device: "testdata1",
						},
					}
					return mounts, nil
				}
			},
			getUnmountFunc: func() func(ctx context.Context, target string) error {
				return func(_ context.Context, _ string) error {
					return nil
				}
			},
			getOsRemoveAllFunc: func() func(path string) error {
				return func(_ string) error {
					return fmt.Errorf("injected error for unit test")
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodeUnpublishVolumeRequest{
					VolumeId:   "testdata1",
					TargetPath: "/data1",
				},
				filterStr: "data1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer after()

			if tt.getGetMountsFunc != nil {
				getGetMountsFunc = tt.getGetMountsFunc
			}

			if tt.getMountFunc != nil {
				getMountFunc = tt.getMountFunc
			}

			if tt.getUnmountFunc != nil {
				getUnmountFunc = tt.getUnmountFunc
			}

			if tt.getOsRemoveAllFunc != nil {
				getOsRemoveAllFunc = tt.getOsRemoveAllFunc
			}

			if err := unpublishVolume(tt.args.ctx, tt.args.req, tt.args.filterStr); (err != nil) != tt.wantErr {
				t.Errorf("unpublishVolume() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_publishVolume(t *testing.T) {
	defaultGetGetMountsFunc := getGetMountsFunc
	defaultGetMountFunc := getMountFunc
	defaultGetUnmountFunc := getUnmountFunc

	mountCallsInTest := 0

	after := func() {
		getGetMountsFunc = defaultGetGetMountsFunc
		getMountFunc = defaultGetMountFunc
		getUnmountFunc = defaultGetUnmountFunc
		mountCallsInTest = 0
	}

	type args struct {
		ctx          context.Context
		req          *csi.NodePublishVolumeRequest
		nfsExportURL string
	}
	tests := []struct {
		name             string
		getGetMountsFunc func() func(ctx context.Context) ([]gofsutil.Info, error)
		getMountFunc     func() func(ctx context.Context, source, target, fsType string, opts ...string) error
		getUnmountFunc   func() func(ctx context.Context, target string) error
		args             args
		wantErr          bool
		errorChecker     func(t *testing.T, err error)
	}{
		{
			name: "empty request",
			args: args{
				ctx:          context.Background(),
				req:          &csi.NodePublishVolumeRequest{},
				nfsExportURL: "",
			},
			wantErr: true,
			errorChecker: func(t *testing.T, err error) {
				assert.Equal(t, codes.InvalidArgument, status.Code(err))
			},
		},
		{
			name: "empty access mode",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId:         "ut-vol",
					VolumeCapability: &csi.VolumeCapability{},
				},
			},
			wantErr: true,
			errorChecker: func(t *testing.T, err error) {
				assert.Equal(t, codes.InvalidArgument, status.Code(err))
			},
		},
		{
			name: "invalid access mode",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId: "ut-vol",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
						},
					},
				},
			},
			wantErr: true,
			errorChecker: func(t *testing.T, err error) {
				assert.Equal(t, codes.InvalidArgument, status.Code(err))
			},
		},
		{
			name: "invalid target path",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId: "ut-vol",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
					},
				},
			},
			wantErr: true,
			errorChecker: func(t *testing.T, err error) {
				assert.Equal(t, codes.InvalidArgument, status.Code(err))
			},
		},
		{
			name: "failure to create target path",
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId: "ut-vol",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
					},
					TargetPath: "/dev/null",
				},
			},
			wantErr: true,
			errorChecker: func(t *testing.T, err error) {
				assert.Equal(t, codes.FailedPrecondition, status.Code(err))
			},
		},
		{
			name: "failure to get mounts",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					return nil, fmt.Errorf("injected error for unit test")
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId: "ut-vol",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
					},
					TargetPath: filepath.Join(t.TempDir(), "/mount-test"),
				},
			},
			wantErr: true,
			errorChecker: func(t *testing.T, err error) {
				assert.Equal(t, codes.Internal, status.Code(err))
			},
		},
		{
			name: "do not mount when already mounted with same options",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					mounts := []gofsutil.Info{
						{
							Path: "/tmp",
							Opts: []string{"rw", "remount"},
						},
					}
					return mounts, nil
				}
			},
			getMountFunc: func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
				return func(_ context.Context, _, _, _ string, _ ...string) error {
					return fmt.Errorf("mount function should not be called T1=T2, P1=P2 ret OK")
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId: "ut-vol",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
					},
					TargetPath: "/tmp",
				},
			},
			wantErr: false,
		},
		{
			name: "return error when already mounted with different options",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					mounts := []gofsutil.Info{
						{
							Path: "/tmp",
							Opts: []string{"ro", "remount"},
						},
					}
					return mounts, nil
				}
			},
			getMountFunc: func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
				return func(_ context.Context, _, _, _ string, _ ...string) error {
					return fmt.Errorf("mount function should not be called T1=T2, P1!=P2 ret FailedPrecondition")
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId: "ut-vol",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
					},
					TargetPath: "/tmp",
				},
			},
			wantErr: true,
			errorChecker: func(t *testing.T, err error) {
				assert.Equal(t, codes.AlreadyExists, status.Code(err))
			},
		},
		{
			name: "return error when mount point in use by same device,singlenodewriter",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					mounts := []gofsutil.Info{
						{
							Path:   "/tmp/notsameastargetpath",
							Opts:   []string{"rw", "remount"},
							Device: "server:/export/path",
						},
					}
					return mounts, nil
				}
			},
			getMountFunc: func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
				return func(_ context.Context, _, _, _ string, _ ...string) error {
					return fmt.Errorf("mount function should not be called T1!=T2, P1==P2 || P1!=P2 FailedPrecondition")
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId: "ut-vol",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
					},
					TargetPath: "/tmp",
				},
				nfsExportURL: "server:/export/path",
			},
			wantErr: true,
			errorChecker: func(t *testing.T, err error) {
				assert.Equal(t, codes.FailedPrecondition, status.Code(err))
			},
		},
		{
			name: "return error when mount point in use by same device,singlenodereaderonly",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					mounts := []gofsutil.Info{
						{
							Path:   "/tmp/notsameastargetpath",
							Opts:   []string{"rw", "remount"},
							Device: "server:/export/path",
						},
					}
					return mounts, nil
				}
			},
			getMountFunc: func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
				return func(_ context.Context, _, _, _ string, _ ...string) error {
					return fmt.Errorf("mount function should not be called T1!=T2, P1==P2 || P1!=P2 FailedPrecondition")
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId: "ut-vol",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
						},
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
					},
					TargetPath: "/tmp",
				},
				nfsExportURL: "server:/export/path",
			},
			wantErr: true,
			errorChecker: func(t *testing.T, err error) {
				assert.Equal(t, codes.FailedPrecondition, status.Code(err))
			},
		},
		{
			name: "return error when mount point in use by same device,singlenodesinglewriter",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					mounts := []gofsutil.Info{
						{
							Path:   "/tmp/notsameastargetpath",
							Opts:   []string{"rw", "remount"},
							Device: "server:/export/path",
						},
					}
					return mounts, nil
				}
			},
			getMountFunc: func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
				return func(_ context.Context, _, _, _ string, _ ...string) error {
					return fmt.Errorf("mount function should not be called T1!=T2, P1==P2 || P1!=P2 FailedPrecondition")
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId: "ut-vol",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER,
						},
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
					},
					TargetPath: "/tmp",
				},
				nfsExportURL: "server:/export/path",
			},
			wantErr: true,
			errorChecker: func(t *testing.T, err error) {
				assert.Equal(t, codes.FailedPrecondition, status.Code(err))
			},
		},
		{
			name: "mount returns access denied by server while mounting",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					return []gofsutil.Info{}, nil
				}
			},
			getMountFunc: func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
				return func(_ context.Context, _, _, _ string, _ ...string) error {
					return fmt.Errorf("injected error: access denied by server while mounting")
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId: "ut-vol",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
					},
					TargetPath: "/tmp",
				},
				nfsExportURL: "server:/export/path",
			},
			wantErr: true,
			errorChecker: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "injected error: access denied by server while mounting")
			},
		},
		{
			name: "mount returns no such file or directory",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					return []gofsutil.Info{}, nil
				}
			},
			getMountFunc: func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
				return func(_ context.Context, _, _, _ string, _ ...string) error {
					return fmt.Errorf("injected error: no such file or directory")
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId: "ut-vol",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
					},
					TargetPath: "/tmp",
				},
				nfsExportURL: "server:/export/path",
			},
			wantErr: true,
			errorChecker: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "injected error: no such file or directory")
			},
		},
		{
			name: "mount successful after retry",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					return []gofsutil.Info{}, nil
				}
			},
			getMountFunc: func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
				return func(_ context.Context, _, _, _ string, _ ...string) error {
					if mountCallsInTest < 4 {
						mountCallsInTest++
						return fmt.Errorf("injected error %d: no such file or directory", mountCallsInTest)
					}
					return nil
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId: "ut-vol",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
					},
					TargetPath: "/tmp",
				},
				nfsExportURL: "server:/export/path",
			},
			wantErr: false,
		},
		{
			name: "support ro mount and validate mount options",
			getGetMountsFunc: func() func(ctx context.Context) ([]gofsutil.Info, error) {
				return func(_ context.Context) ([]gofsutil.Info, error) {
					return []gofsutil.Info{}, nil
				}
			},
			getMountFunc: func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
				return func(_ context.Context, nfsExportURL, target, fsType string, mntOptions ...string) error {
					assert.Contains(t, nfsExportURL, "server:/export/path")
					assert.Contains(t, target, "/tmp")
					assert.Contains(t, fsType, "nfs")
					assert.Contains(t, mntOptions, "ro")
					return nil
				}
			},
			args: args{
				ctx: context.Background(),
				req: &csi.NodePublishVolumeRequest{
					VolumeId: "ut-vol",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
					},
					TargetPath: "/tmp",
					Readonly:   true,
				},
				nfsExportURL: "server:/export/path",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer after()

			if tt.getGetMountsFunc != nil {
				getGetMountsFunc = tt.getGetMountsFunc
			}

			if tt.getMountFunc != nil {
				getMountFunc = tt.getMountFunc
			}

			if tt.getUnmountFunc != nil {
				getUnmountFunc = tt.getUnmountFunc
			}

			err := publishVolume(tt.args.ctx, tt.args.req, tt.args.nfsExportURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("publishVolume() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil && tt.errorChecker != nil {
				tt.errorChecker(t, err)
			}
		})
	}
}

func Test_containsPrefix(t *testing.T) {
	tests := []struct {
		name   string
		list   []string
		prefix string
		want   bool
	}{
		{"empty list", []string{}, "xprtsec=", false},
		{"no match", []string{"rw", "hard"}, "xprtsec=", false},
		{"exact match", []string{"xprtsec=mtls"}, "xprtsec=", true},
		{"case insensitive", []string{"Xprtsec=MTLS"}, "xprtsec=", true},
		{"among others", []string{"rw", "xprtsec=mtls", "hard"}, "xprtsec=", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsPrefix(tt.list, tt.prefix)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test_publishVolume_mTLS_xprtsec_injection verifies that when
// NFSTransportSecurity is set to "mtls" in VolumeContext, the mount command
// automatically receives the "xprtsec=mtls" option. Non-mTLS requests must
// remain untouched for backward compatibility.
func Test_publishVolume_mTLS_xprtsec_injection(t *testing.T) {
	defaultGetGetMountsFunc := getGetMountsFunc
	defaultGetMountFunc := getMountFunc

	defer func() {
		getGetMountsFunc = defaultGetGetMountsFunc
		getMountFunc = defaultGetMountFunc
	}()

	// stub GetMounts to return nothing (fresh mount)
	getGetMountsFunc = func() func(ctx context.Context) ([]gofsutil.Info, error) {
		return func(_ context.Context) ([]gofsutil.Info, error) {
			return []gofsutil.Info{}, nil
		}
	}

	tests := []struct {
		name           string
		volumeContext  map[string]string
		userMountFlags []string
		wantXprtsec    bool // expect xprtsec=mtls in mount options
		wantDuplicate  bool // must NOT see a duplicate
	}{
		{
			name:          "mTLS enabled - injects xprtsec=mtls",
			volumeContext: map[string]string{"NFSTransportSecurity": "mtls"},
			wantXprtsec:   true,
		},
		{
			name:          "mTLS enabled case insensitive",
			volumeContext: map[string]string{"NFSTransportSecurity": "MTLS"},
			wantXprtsec:   true,
		},
		{
			name:          "no transport security - no injection",
			volumeContext: map[string]string{},
			wantXprtsec:   false,
		},
		{
			name:          "none transport security - no injection",
			volumeContext: map[string]string{"NFSTransportSecurity": "none"},
			wantXprtsec:   false,
		},
		{
			name:          "tls transport security - no injection (mTLS only)",
			volumeContext: map[string]string{"NFSTransportSecurity": "tls"},
			wantXprtsec:   false,
		},
		{
			name:           "user already provides xprtsec=mtls - no duplicate",
			volumeContext:  map[string]string{"NFSTransportSecurity": "mtls"},
			userMountFlags: []string{"xprtsec=mtls"},
			wantXprtsec:    true,
			wantDuplicate:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedOpts []string

			getMountFunc = func() func(ctx context.Context, source, target, fsType string, opts ...string) error {
				return func(_ context.Context, _, _, _ string, opts ...string) error {
					capturedOpts = opts
					return nil
				}
			}

			req := &csi.NodePublishVolumeRequest{
				VolumeId: "test-vol",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							MountFlags: tt.userMountFlags,
						},
					},
				},
				TargetPath:    filepath.Join(t.TempDir(), "target"),
				VolumeContext: tt.volumeContext,
			}

			err := publishVolume(context.Background(), req, "powerscale.test.local:/ifs/data/test-vol")
			assert.NoError(t, err)

			hasXprtsec := false
			xprtsecCount := 0
			for _, o := range capturedOpts {
				if o == "xprtsec=mtls" {
					hasXprtsec = true
					xprtsecCount++
				}
			}

			if tt.wantXprtsec {
				assert.True(t, hasXprtsec, "expected xprtsec=mtls in mount options, got: %v", capturedOpts)
			} else {
				assert.False(t, hasXprtsec, "did not expect xprtsec=mtls in mount options, got: %v", capturedOpts)
			}

			// Never duplicate
			assert.LessOrEqual(t, xprtsecCount, 1, "xprtsec=mtls must not be duplicated, count=%d opts=%v", xprtsecCount, capturedOpts)
		})
	}
}
