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
