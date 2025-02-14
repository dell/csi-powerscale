package k8sutils

/*
 Copyright (c) 2020-2022 Dell Inc, or its subsidiaries.

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

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dell/gofsutil"
	"github.com/kubernetes-csi/csi-lib-utils/leaderelection"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var buildConfigFromFlags = clientcmd.BuildConfigFromFlags
var newForConfig = kubernetes.NewForConfig
var inClusterConfig = rest.InClusterConfig

var fsInfo = func(ctx context.Context, path string) (int64, int64, int64, int64, int64, int64, error) {
	return gofsutil.FsInfo(ctx, path)
}

type leaderElection interface {
	Run() error
	WithNamespace(namespace string)
}

// CreateKubeClientSet - Returns kubeclient set
func CreateKubeClientSet(kubeconfig string) (*kubernetes.Clientset, error) {
	var clientset *kubernetes.Clientset
	if kubeconfig != "" {
		// use the current context in kubeconfig
		config, err := buildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
		// create the clientset
		clientset, err = newForConfig(config)
		if err != nil {
			return nil, err
		}
	} else {
		config, err := inClusterConfig()
		if err != nil {
			return nil, err
		}
		// creates the clientset
		clientset, err = newForConfig(config)
		if err != nil {
			return nil, err
		}
	}
	return clientset, nil
}

// LeaderElection - Initialize leader election
func LeaderElection(clientset *kubernetes.Clientset, lockName string, namespace string,
	leaderElectionRenewDeadline, leaderElectionLeaseDuration, leaderElectionRetryPeriod time.Duration, runFunc func(ctx context.Context),
) {
	le := leaderelection.NewLeaderElection(clientset, lockName, runFunc)
	le.WithNamespace(namespace)
	le.WithLeaseDuration(leaderElectionLeaseDuration)
	le.WithRenewDeadline(leaderElectionRenewDeadline)
	le.WithRetryPeriod(leaderElectionRetryPeriod)
	if err := le.Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to initialize leader election: %v", err)
		os.Exit(1)
	}
}

// GetStats - Returns the stats for the volume mounted on given volume path
func GetStats(ctx context.Context, volumePath string) (int64, int64, int64, int64, int64, int64, error) {
	availableBytes, totalBytes, usedBytes, totalInodes, freeInodes, usedInodes, err := fsInfo(ctx, volumePath)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, status.Error(codes.Internal, fmt.Sprintf(
			"failed to get volume stats: %s", err))
	}
	return availableBytes, totalBytes, usedBytes, totalInodes, freeInodes, usedInodes, err
}
