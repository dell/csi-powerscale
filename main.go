package main

/*
 Copyright (c) 2019 Dell Inc, or its subsidiaries.

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
	"github.com/dell/csi-isilon/common/constants"
	"github.com/dell/csi-isilon/common/k8sutils"
	"os"
	"strings"

	"github.com/dell/csi-isilon/provider"
	"github.com/rexray/gocsi"
)

// main is ignored when this package is built as a go plug-in
func main() {
	enableLeaderElection := flag.Bool("leader-election", false, "Enables leader election.")
	leaderElectionNamespace := flag.String("leader-election-namespace", "", "The namespace where leader election lease will be created. Defaults to the pod namespace if not set.")
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	flag.Parse()

	run := func(ctx context.Context) {
		gocsi.Run(
			ctx,
			constants.PluginName,
			"An Isilon Container Storage Interface (CSI) Plugin",
			usage,
			provider.New())
	}

	if !*enableLeaderElection {
		run(context.TODO())
	} else {
		driverName := strings.Replace(constants.PluginName, ".", "-", -1)
		lockName := fmt.Sprintf("driver-%s", driverName)
		k8sclientset, err := k8sutils.CreateKubeClientSet(*kubeconfig)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to initialize leader election: %v", err)
			os.Exit(1)
		}
		// Attempt to become leader and start the driver
		k8sutils.LeaderElection(k8sclientset, lockName, *leaderElectionNamespace, run)
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

    X_CSI_ISI_INSECURE 
        Specifies that the ISILON Gateway's hostname and certificate chain
	should not be verified.

        The default value is false.

    X_CSI_ISI_SYSTEMNAME
        Specifies the name of the Isilon system to interact with.

        The default value is default.
`
