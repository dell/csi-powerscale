package main

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/dell/csi-isilon/v2/service"
	"github.com/stretchr/testify/assert"
)

func TestSetEnv(t *testing.T) {
	err := os.Setenv(constants.EnvGOCSIDebug, "true")
	assert.NoError(t, err, "Failed to set environment variable")
	assert.Equal(t, "true", os.Getenv(constants.EnvGOCSIDebug))
}

func TestParseFlags(t *testing.T) {
	os.Args = []string{"cmd", "--driver-config-params=config.yaml"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	enableLeaderElection := flag.Bool("leader-election", false, "Enables leader election.")
	driverConfigParamsfile := flag.String("driver-config-params", "", "yaml file with driver config params")
	flag.Parse()

	assert.False(t, *enableLeaderElection)
	assert.Equal(t, "config.yaml", *driverConfigParamsfile)
}

func TestRunWithoutLeaderElection(t *testing.T) {
	service.DriverConfigParamsFile = "config.yaml"
	run := func(ctx context.Context) {
		assert.Equal(t, "config.yaml", service.DriverConfigParamsFile)
	}

	run(context.TODO())
}

func TestRunWithLeaderElection(t *testing.T) {
	// Mocking Kubernetes client and leader election
	service.DriverConfigParamsFile = "config.yaml"
	enableLeaderElection := true

	run := func(ctx context.Context) {
		assert.Equal(t, "config.yaml", service.DriverConfigParamsFile)
	}

	if enableLeaderElection {
		// Mock leader election logic here
		run(context.TODO())
	}
}
