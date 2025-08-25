package main

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
//go:generate go generate ./core

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/dell/csi-isilon/v2/common/k8sutils"
	"github.com/dell/csi-isilon/v2/provider"
	"github.com/dell/csi-isilon/v2/service"
	"github.com/dell/gocsi"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

var exitFunc = os.Exit

func validateArgs(driverConfigParamsfile *string) {
	log.Info("Validating driver config params file argument")
	if *driverConfigParamsfile == "" {
		fmt.Fprintf(os.Stderr, "driver-config-params argument is mandatory")
		exitFunc(1)
	}
}

func checkLeaderElectionError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize leader election: %v", err)
		exitFunc(1)
	}
}

func main() {
	// We always want to enable Request and Response logging(no reason for users to control this)
	_ = os.Setenv(gocsi.EnvVarReqLogging, "true")
	_ = os.Setenv(gocsi.EnvVarRepLogging, "true")

	mainR(gocsi.Run, func(kubeconfig string) (kubernetes.Interface, error) {
		return k8sutils.CreateKubeClientSet(kubeconfig)
	}, func(clientset kubernetes.Interface, lockName, namespace string, renewDeadline, leaseDuration, retryPeriod time.Duration, run func(ctx context.Context)) {
		k8sutils.LeaderElection(clientset.(*kubernetes.Clientset), lockName, namespace, renewDeadline, leaseDuration, retryPeriod, run)
	})
}

func mainR(runFunc func(ctx context.Context, name, desc string, usage string, sp gocsi.StoragePluginProvider), createKubeClientSet func(kubeconfig string) (kubernetes.Interface, error), leaderElection func(clientset kubernetes.Interface, lockName, namespace string, renewDeadline, leaseDuration, retryPeriod time.Duration, run func(ctx context.Context))) {
	enableLeaderElection := flag.Bool("leader-election", false, "Enables leader election.")
	log.Info("Enabling leader election")
	leaderElectionNamespace := flag.String("leader-election-namespace", "", "The namespace where leader election lease will be created. Defaults to the pod namespace if not set.")
	log.Info("leader-election-namespace = " + *leaderElectionNamespace)
	leaderElectionLeaseDuration := flag.Duration("leader-election-lease-duration", 15*time.Second, "Duration, in seconds, that non-leader candidates will wait to force acquire leadership")
	log.Info("leader-election-lease-duration = " + leaderElectionLeaseDuration.String())
	leaderElectionRenewDeadline := flag.Duration("leader-election-renew-deadline", 10*time.Second, "Duration, in seconds, that the acting leader will retry refreshing leadership before giving up.")
	log.Info("leader-election-renew-deadline = " + leaderElectionRenewDeadline.String())
	leaderElectionRetryPeriod := flag.Duration("leader-election-retry-period", 5*time.Second, "Duration, in seconds, the LeaderElector clients should wait between tries of actions")
	log.Info("leader-election-retry-period = " + leaderElectionRetryPeriod.String())
	driverConfigParamsfile := flag.String("driver-config-params", "", "yaml file with driver config params")
	log.Info("driver-config-params = " + *driverConfigParamsfile)
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	flag.Parse()

	validateArgs(driverConfigParamsfile)
	service.DriverConfigParamsFile = *driverConfigParamsfile

	run := func(ctx context.Context) {
		runFunc(
			ctx,
			constants.PluginName,
			"An Isilon Container Storage Interface (CSI) Plugin",
			usage,
			provider.New())
	}

	if *enableLeaderElection == false {
		run(context.TODO())
	} else {
		driverName := strings.Replace(constants.PluginName, ".", "-", -1)
		lockName := fmt.Sprintf("driver-%s", driverName)
		k8sclientset, err := createKubeClientSet(*kubeconfig)
		log.Info("Checking for leader election error")
		checkLeaderElectionError(err)
		// Attempt to become leader and start the driver
		log.Info("Starting leader election")
		leaderElection(k8sclientset, lockName, *leaderElectionNamespace,
			*leaderElectionRenewDeadline, *leaderElectionLeaseDuration, *leaderElectionRetryPeriod, run)
	}
}

const usage = `   X_CSI_ISI_ENDPOINT 
        Specifies the HTTPS endpoint for the Isilon REST API server. This parameter is
        required when running the Controller service.

        The default value is empty.
    
    X_CSI_ISI_PORT
        Specifies the HTTPS port number for the Isilon REST API server.

        The default value is 8080.

    X_CSI_ISI_USER 
        Specifies the user name when authenticating to the Isilon REST API server.

        The default value is admin.

    X_CSI_ISI_PASSWORD 
        Specifies the password of the user defined by X_CSI_ISI_USER to use
        when authenticating to the Isilon REST API server. This parameter is required
        when running the Controller service.

        The default value is empty.

    X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION 
        Specifies that the ISILON Gateway's hostname and certificate chain
	should not be verified.

        The default value is false.

    X_CSI_ISI_SYSTEMNAME
        Specifies the name of the Isilon system to interact with.

        The default value is default.

    X_CSI_ISI_CONFIG_PATH
        Specifies the filepath containing Isilon cluster's config details.
`
