package main

import (
	"context"
	"flag"
	"os"
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

func mockExit(int) {
	panic("os.Exit called")
}

func mockCreateKubeClientSet(kubeconfig string) (kubernetes.Interface, error) {
	return fake.NewSimpleClientset(), nil
}

func mockLeaderElection(clientset kubernetes.Interface, lockName, namespace string, renewDeadline, leaseDuration, retryPeriod time.Duration, run func(ctx context.Context)) {
	// Mock leader election logic
	run(context.TODO())
}

func TestMainFunctionWithoutLeaderElection(t *testing.T) {
	// Save the original command-line arguments and restore them after the test
	origArgs := os.Args
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
				OnStartedLeading: func(ctx context.Context) {
					t.Log("Became the leader")
					// Perform leader-specific tasks here
				},
				OnStoppedLeading: func() {
					t.Log("Stopped being the leader")
				},
				OnNewLeader: func(identity string) {
					if identity == "test-identity" {
						t.Log("Still the leader")
					} else {
						t.Log("New leader elected: ", identity)
					}
				},
			},
		}

		// Mock the leaderElection function
		mockLeaderElection := func(clientset kubernetes.Interface, lockName, namespace string, renewDeadline, leaseDuration, retryPeriod time.Duration, run func(ctx context.Context)) {
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
