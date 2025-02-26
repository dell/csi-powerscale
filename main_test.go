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

package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/dell/gocsi"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type MockGocsi struct {
	mock.Mock
}

func (m *MockGocsi) Run(ctx context.Context, name, desc, usage string, sp gocsi.StoragePluginProvider) {
	m.Called(ctx, name, desc, usage, sp)
}

var osExit = os.Exit

func expectMockExit(int) {
	osExit(0)
}

func mockExit(int) {
	panic("os.Exit called")
}

func mockCreateKubeClientSet(_ string) (kubernetes.Interface, error) {
	return fake.NewSimpleClientset(), nil
}

func mockLeaderElection(_ kubernetes.Interface, _, _ string, _, _, _ time.Duration, run func(ctx context.Context)) {
	// Mock leader election logic
	run(context.TODO())
}

func TestMainFunctionWithoutLeaderElection(t *testing.T) {
	// Save the original command-line arguments and restore them after the test
	origArgs := os.Args
	// Reset the flag.CommandLine
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	defer func() { os.Args = origArgs }()

	// Mock the gocsi.Run function
	mockGocsi := new(MockGocsi)

	// Mock os.Exit
	osExit = mockExit
	defer func() { osExit = os.Exit }()

	// Test case: Leader election disabled
	t.Run("LeaderElectionDisabled", func(t *testing.T) {
		// Create a new flag set for this test case
		flagSet := flag.NewFlagSet("test", flag.ExitOnError)
		os.Args = []string{"cmd", "--leader-election=false", "--driver-config-params=config.yaml"}
		flagSet.Bool("leader-election", false, "Enables leader election.")
		flagSet.String("driver-config-params", "config.yaml", "yaml file with driver config params")
		flagSet.Parse(os.Args[1:])

		mockGocsi.On("Run", mock.Anything, constants.PluginName, "An Isilon Container Storage Interface (CSI) Plugin", usage, mock.Anything).Return()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("os.Exit was called: %v", r)
			}
		}()

		mainR(mockGocsi.Run, mockCreateKubeClientSet, mockLeaderElection)

		mockGocsi.AssertCalled(t, "Run", mock.Anything, constants.PluginName, "An Isilon Container Storage Interface (CSI) Plugin", usage, mock.Anything)
	})
}

func TestMainFunctionWithLeaderElection(t *testing.T) {
	// Save the original command-line arguments and restore them after the test
	origArgs := os.Args
	// Reset the flag.CommandLine
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	defer func() { os.Args = origArgs }()

	// Mock the gocsi.Run function
	mockGocsi := new(MockGocsi)

	// Mock os.Exit
	osExit = mockExit
	defer func() { osExit = os.Exit }()

	// Set required environment variables for Kubernetes client
	os.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	os.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	defer func() {
		os.Unsetenv("KUBERNETES_SERVICE_HOST")
		os.Unsetenv("KUBERNETES_SERVICE_PORT")
	}()

	// reset os.Args before next test
	os.Args = origArgs

	// Test case: Leader election enabled
	t.Run("LeaderElectionEnabled", func(t *testing.T) {
		// Create a new flag set for this test case
		flagSet := flag.NewFlagSet("test", flag.ExitOnError)
		os.Args = []string{
			"cmd",
			"--leader-election=true",
			"--leader-election-namespace=default",
			"--leader-election-lease-duration=15s",
			"--leader-election-renew-deadline=10s",
			"--leader-election-retry-period=5s",
			"--driver-config-params=config.yaml",
		}
		flagSet.Bool("leader-election", true, "Enables leader election.")
		flagSet.String("leader-election-namespace", "default", "The namespace where leader election lease will be created.")
		flagSet.Duration("leader-election-lease-duration", 15*time.Second, "Duration, in seconds, that non-leader candidates will wait to force acquire leadership")
		flagSet.Duration("leader-election-renew-deadline", 10*time.Second, "Duration, in seconds, that the acting leader will retry refreshing leadership before giving up.")
		flagSet.Duration("leader-election-retry-period", 5*time.Second, "Duration, in seconds, the LeaderElector clients should wait between tries of actions")
		flagSet.String("driver-config-params", "config.yaml", "yaml file with driver config params")
		flagSet.Parse(os.Args[1:])

		// Mock Kubernetes client
		client := fake.NewSimpleClientset()

		lock := &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      "driver-csi-isilon-dellemc-com",
				Namespace: "default",
			},
			Client: client.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: "test-identity",
			},
		}

		leaderElectionConfig := leaderelection.LeaderElectionConfig{
			Lock:          lock,
			LeaseDuration: 15 * time.Second,
			RenewDeadline: 10 * time.Second,
			RetryPeriod:   5 * time.Second,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(_ context.Context) {
					// Perform leader-specific tasks here
				},
				OnStoppedLeading: func() {
				},
				OnNewLeader: func(_ string) {
					// Perform leader-specific tasks here
				},
			},
		}

		// Mock the leaderElection function
		mockLeaderElection := func(_ kubernetes.Interface, lockName, namespace string, renewDeadline, leaseDuration, retryPeriod time.Duration, _ func(ctx context.Context)) {
			require.Equal(t, "driver-csi-isilon-dellemc-com", lockName)
			require.Equal(t, "default", namespace)
			require.Equal(t, 10*time.Second, renewDeadline)
			require.Equal(t, 15*time.Second, leaseDuration)
			require.Equal(t, 5*time.Second, retryPeriod)
			go leaderelection.RunOrDie(context.TODO(), leaderElectionConfig)
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("os.Exit was called: %v", r)
			}
		}()

		mainR(mockGocsi.Run, mockCreateKubeClientSet, mockLeaderElection)
	})
}

func fakeExit(code int) {
	exitCode = code
}

func TestValidateArgs(_ *testing.T) {
	var mu sync.Mutex

	// Capture the output
	_, w, _ := os.Pipe()

	// Replace os.Exit with fakeExit
	oldExit := exitFunc
	exitFunc = fakeExit
	defer func() { exitFunc = oldExit }()

	// Reset exitCode before running the test
	exitCode = 0

	// Use the mutex to protect shared resources
	mu.Lock()
	defer mu.Unlock()

	// Run the validateArgs function
	driverConfigParamsfile := ""
	validateArgs(&driverConfigParamsfile)

	// Close the pipe writer and read the output
	w.Close()
}

func fakeCreateKubeClientSet(_ string) (kubernetes.Interface, error) {
	return nil, errors.New("simulated error")
}

func TestCheckLeaderElectionError(_ *testing.T) {
	// Mock the function to return an error
	err := errors.New("mock error")

	// Capture the output
	_, out, _ := os.Pipe()

	// Replace os.Exit with fakeExit
	oldExit := exitFunc
	exitFunc = fakeExit
	defer func() { exitFunc = oldExit }()

	// Call the function that includes the error handling
	checkLeaderElectionError(err)

	// Restore the original Stderr
	out.Close()
}

var exitCode int

func exitFunc1(code int) {
	// Mock exit function for testing
	exitCode = code
}
