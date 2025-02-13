package provider

import (
	"bytes"
	"errors"
	"testing"
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
	mockRemoveExistingCSISockFile = func() error {
		return nil
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
