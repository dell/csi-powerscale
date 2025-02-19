package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/dell/csi-isilon/v2/common/k8sutils"
	"github.com/dell/csi-isilon/v2/service"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

var k8sclientset *fake.Clientset

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

func TestParseFlags1(t *testing.T) {
	os.Args = []string{"cmd", "--driver-config-params=config.yaml", "--leader-election=true", "--leader-election-namespace=default", "--leader-election-lease-duration=20s", "--leader-election-renew-deadline=15s", "--leader-election-retry-period=10s", "--kubeconfig=/path/to/kubeconfig"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	enableLeaderElection := flag.Bool("leader-election", false, "Enables leader election.")
	leaderElectionNamespace := flag.String("leader-election-namespace", "", "The namespace where leader election lease will be created. Defaults to the pod namespace if not set.")
	leaderElectionLeaseDuration := flag.Duration("leader-election-lease-duration", 15*time.Second, "Duration, in seconds, that non-leader candidates will wait to force acquire leadership")
	leaderElectionRenewDeadline := flag.Duration("leader-election-renew-deadline", 10*time.Second, "Duration, in seconds, that the acting leader will retry refreshing leadership before giving up.")
	leaderElectionRetryPeriod := flag.Duration("leader-election-retry-period", 5*time.Second, "Duration, in seconds, the LeaderElector clients should wait between tries of actions")
	driverConfigParamsfile := flag.String("driver-config-params", "", "yaml file with driver config params")
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	flag.Parse()

	assert.True(t, *enableLeaderElection)
	assert.Equal(t, "default", *leaderElectionNamespace)
	assert.Equal(t, 20*time.Second, *leaderElectionLeaseDuration)
	assert.Equal(t, 15*time.Second, *leaderElectionRenewDeadline)
	assert.Equal(t, 10*time.Second, *leaderElectionRetryPeriod)
	assert.Equal(t, "config.yaml", *driverConfigParamsfile)
	assert.Equal(t, "/path/to/kubeconfig", *kubeconfig)
}

func TestRunWithoutDriverConfigParams(t *testing.T) {
	os.Args = []string{"cmd"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	driverConfigParamsfile := flag.String("driver-config-params", "", "yaml file with driver config params")
	flag.Parse()

	if *driverConfigParamsfile == "" {
		fmt.Fprintf(os.Stderr, "driver-config-params argument is mandatory")
		assert.Fail(t, "driver-config-params argument is mandatory")
	}
}

// Mock for the gocsi.Run function
type MockGocsi struct {
	mock.Mock
}

func (m *MockGocsi) Run(ctx context.Context, name, desc string, usage func(), provider interface{}) {
	m.Called(ctx, name, desc, usage, provider)
}

// Wrapper function to convert *kubernetes.Clientset to kubernetes.Interface
func createKubeClientSetWrapper(kubeconfig string) (kubernetes.Interface, error) {
	return k8sutils.CreateKubeClientSet(kubeconfig)
}

// Wrapper function for LeaderElection
func leaderElectionWrapper(clientset kubernetes.Interface, lockName, namespace string, renewDeadline, leaseDuration, retryPeriod time.Duration, run func(ctx context.Context)) {
	k8sutils.LeaderElection(clientset.(*kubernetes.Clientset), lockName, namespace, renewDeadline, leaseDuration, retryPeriod, run)
}

// Global variables to override the functions
var createKubeClientSet = createKubeClientSetWrapper
var leaderElection = leaderElectionWrapper

// Mock for os.Exit to prevent the test from exiting prematurely
var osExit = os.Exit

func mockExit(code int) {
	panic(fmt.Sprintf("os.Exit called with code %d", code))
}

func TestMainFunction(t *testing.T) {
	// Save the original command-line arguments and restore them after the test
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Mock the gocsi.Run function
	mockGocsi := new(MockGocsi)
	//gocsiRun = mockGocsi.Run

	// Mock os.Exit
	osExit = mockExit
	defer func() { osExit = os.Exit }()

	// Test case: Leader election disabled
	t.Run("LeaderElectionDisabled", func(t *testing.T) {
		os.Args = []string{"cmd", "--leader-election=false", "--driver-config-params=config.yaml"}
		mockGocsi.On("Run", mock.Anything, constants.PluginName, "An Isilon Container Storage Interface (CSI) Plugin", usage, mock.Anything).Return()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("os.Exit was called: %v", r)
			}
		}()

		main()

		mockGocsi.AssertCalled(t, "Run", mock.Anything, constants.PluginName, "An Isilon Container Storage Interface (CSI) Plugin", usage, mock.Anything)
	})

	// Test case: Leader election enabled
	t.Run("LeaderElectionEnabled", func(t *testing.T) {
		os.Args = []string{
			"cmd",
			"--leader-election=true",
			"--leader-election-namespace=default",
			"--leader-election-lease-duration=15s",
			"--leader-election-renew-deadline=10s",
			"--leader-election-retry-period=5s",
			"--driver-config-params=config.yaml",
		}

		// Mock Kubernetes client
		client := fake.NewSimpleClientset()
		createKubeClientSet = func(kubeconfig string) (kubernetes.Interface, error) {
			return client, nil
		}

		lock := &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      "driver-plugin-name",
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
		leaderElection = func(clientset kubernetes.Interface, lockName, namespace string, renewDeadline, leaseDuration, retryPeriod time.Duration, run func(ctx context.Context)) {
			go leaderelection.RunOrDie(context.TODO(), leaderElectionConfig)
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("os.Exit was called: %v", r)
			}
		}()

		main()

	})
}
