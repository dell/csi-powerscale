/*
Copyright (c) 2019-2025 Dell Inc, or its subsidiaries.

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
	"os"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
)

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
	type args struct {
		ctx       context.Context
		filterStr string
		target    string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "root check",
			args: args{
				ctx:       context.Background(),
				filterStr: "",
				target:    "/",
			},
			want:    true,
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
	type args struct {
		ctx       context.Context
		req       *csi.NodeUnpublishVolumeRequest
		filterStr string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
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
			name: "unmount should fail on /",
			args: args{
				ctx: context.Background(),
				req: &csi.NodeUnpublishVolumeRequest{
					VolumeId:   "test",
					TargetPath: "/",
				},
				filterStr: "",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := unpublishVolume(tt.args.ctx, tt.args.req, tt.args.filterStr); (err != nil) != tt.wantErr {
				t.Errorf("unpublishVolume() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_publishVolume(t *testing.T) {
	type args struct {
		ctx       context.Context
		req       *csi.NodePublishVolumeRequest
		filterStr string
	}
	tests := []struct {
		name    string
		before  func() error
		args    args
		wantErr bool
	}{
		{
			name: "empty request",
			args: args{
				ctx:       context.Background(),
				req:       &csi.NodePublishVolumeRequest{},
				filterStr: "",
			},
			wantErr: true,
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
		},
		{
			name: "failure to create target path",
			before: func() error {
				return os.WriteFile("/test/bad", []byte("dummy-content"), 0o600)
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
					TargetPath: "/test/bad",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.before != nil {
				if err := tt.before(); err != nil {
					t.Errorf("publishVolume() error in before() function %v", err)
				}
			}

			if err := publishVolume(tt.args.ctx, tt.args.req, tt.args.filterStr); (err != nil) != tt.wantErr {
				t.Errorf("publishVolume() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
