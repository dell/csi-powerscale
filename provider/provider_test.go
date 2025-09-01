/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
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

/*
Copyright (c) 2025 Dell Inc, or its subsidiaries.

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

package provider

import (
	"bytes"
	"errors"
	"testing"

	csiutils "github.com/dell/csi-isilon/v2/csi-utils"
	"github.com/dell/gocsi"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// Mocking utility functions
var (
	mockGetLogger = func() *logrus.Entry {
		logger := logrus.New()
		logger.SetLevel(logrus.DebugLevel)
		var logBuffer bytes.Buffer
		logger.SetOutput(&logBuffer)
		return logrus.NewEntry(logger)
	}
	mockRemoveExistingCSISockFile = func() error {
		return errors.New("failed to remove existing CSI sock file")
	}
)

func TestNew(t *testing.T) {
	// Inject the mock functions
	newTest := New()

	// Type assertion to access the fields of gocsi.StoragePlugin
	plugin, ok := newTest.(*gocsi.StoragePlugin)
	assert.True(t, ok, "newTest should be of type *gocsi.StoragePlugin")

	// Assertions
	assert.NotNil(t, plugin)
	assert.Equal(t, plugin.Controller, plugin.Identity)
	assert.Equal(t, plugin.Controller, plugin.Node)
	assert.Len(t, plugin.Interceptors, 2)
	assert.Len(t, plugin.ServerOpts, 1)
	assert.NotNil(t, plugin.BeforeServe)
	assert.NotNil(t, plugin.RegisterAdditionalServers)
	assert.Len(t, plugin.EnvVars, 2)

	// Test case: Removing existing CSI sock file succeeds
	csiutils.RemoveExistingCSISockFile = func() error {
		return errors.New("failed to remove existing CSI sock file")
	}

	newTest = New()
	plugin, ok = newTest.(*gocsi.StoragePlugin)
	assert.True(t, ok, "newTest should be of type *gocsi.StoragePlugin")

	// Assertions
	assert.NotNil(t, plugin)
	assert.Equal(t, plugin.Controller, plugin.Identity)
	assert.Equal(t, plugin.Controller, plugin.Node)
	assert.Len(t, plugin.Interceptors, 2)
	assert.Len(t, plugin.ServerOpts, 1)
	assert.NotNil(t, plugin.BeforeServe)
	assert.NotNil(t, plugin.RegisterAdditionalServers)
	assert.Len(t, plugin.EnvVars, 2)
}
