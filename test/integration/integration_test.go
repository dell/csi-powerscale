/*
Copyright (c) 2019-2022 Dell Inc, or its subsidiaries.

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
package integration_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Showmax/go-fqdn"
	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/dell/csi-isilon/v2/common/k8sutils"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/cucumber/godog"
	"github.com/dell/csi-isilon/v2/provider"
	"github.com/dell/gocsi/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	datadir         = "/tmp/datadir"
	nodeIDSeparator = "=#=#="
)

var grpcClient *grpc.ClientConn

func TestMain(m *testing.M) {
	var stop func()
	ctx := context.Background()
	fmt.Printf("calling startServer")
	grpcClient, stop = startServer(ctx)
	fmt.Printf("back from startServer")
	time.Sleep(5 * time.Second)
	// Set env variables
	host, _ := os.Hostname()
	hostFQDN := fqdn.Get()
	if hostFQDN == "unknown" {
		fmt.Printf("cannot get FQDN")
	}
	nodeIP := os.Getenv("X_CSI_NODE_IP")
	if os.Getenv("X_CSI_CUSTOM_TOPOLOGY_ENABLED") == "true" {
		nodeName := host + nodeIDSeparator + hostFQDN + nodeIDSeparator + nodeIP
		os.Setenv("X_CSI_NODE_NAME", nodeName)
	}
	nodeNameWithoutFQDN := host + nodeIDSeparator + nodeIP + nodeIDSeparator + nodeIP
	os.Setenv("X_CSI_NODE_NAME_NO_FQDN", nodeNameWithoutFQDN)

	// Make the file needed for NodeStage:
	//  /tmp/datadir    -- for file system mounts
	fmt.Printf("Checking '%s'\n", datadir)
	var fileMode os.FileMode
	fileMode = 0o777
	err := os.Mkdir(datadir, fileMode)
	if err != nil && !os.IsExist(err) {
		fmt.Printf("'%s': '%s'\n", datadir, err)
	}

	write, err := os.Create("Powerscale_integration_test_results.xml")
	opts := godog.Options{
		Output: write,
		Format: "junit",
		Paths:  []string{os.Args[len(os.Args)-2]},
		Tags:   os.Args[len(os.Args)-1],
	}

	exitVal := godog.TestSuite{
		Name:                "godog",
		ScenarioInitializer: FeatureContext,
		Options:             &opts,
	}.Run()

	if st := m.Run(); st > exitVal {
		exitVal = st
	}
	stop()
	os.Exit(exitVal)
}

func TestIdentityGetPluginInfo(t *testing.T) {
	ctx := context.Background()
	fmt.Printf("testing GetPluginInfo\n")
	client := csi.NewIdentityClient(grpcClient)
	info, err := client.GetPluginInfo(ctx, &csi.GetPluginInfoRequest{})
	if err != nil {
		fmt.Printf("GetPluginInfo '%s':\n", err.Error())
		t.Error("GetPluginInfo failed")
	} else {
		fmt.Printf("testing GetPluginInfo passed: '%s'\n", info.GetName())
	}
}

func removeNodeLabels(host string) (result bool) {
	k8sclientset, err := k8sutils.CreateKubeClientSet("/etc/kubernetes/admin.conf")
	if err != nil {
		fmt.Printf("init client failed for custom topology: '%s'", err.Error())
		return false
	}

	// access the API to fetch node object
	node, _ := k8sclientset.CoreV1().Nodes().Get(context.TODO(), host, v1.GetOptions{})
	fmt.Printf("Node %s details\n", node)

	// Iterate node labels and check if required label is available and if found remove it
	for lkey, lval := range node.Labels {
		fmt.Printf("Label is: %s:%s\n", lkey, lval)
		if strings.HasPrefix(lkey, constants.PluginName+"/") && lval == constants.PluginName {
			fmt.Printf("Topology label %s:%s available on node", lkey, lval)
			cmd := exec.Command("/bin/bash", "-c", "kubectl label nodes "+host+" "+lkey+"-") // #nosec G204
			err := cmd.Run()
			if err != nil {
				fmt.Printf("Error encountered while removing label from node %s: %s", host, err)
				return false
			}
		}
	}
	return true
}

func applyNodeLabel(host, endpoint string) (result bool) {
	cmd := exec.Command("kubectl", "label", "nodes", host, "csi-isilon.dellemc.com/"+endpoint+"=csi-isilon.dellemc.com") // #nosec G204

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Applying label on node %s failed", host)
		return false
	}
	return true
}

func startServer(ctx context.Context) (*grpc.ClientConn, func()) {
	// Create a new SP instance and serve it with a piped connection.
	sp := provider.New()
	lis, err := utils.GetCSIEndpointListener()
	if err != nil {
		fmt.Printf("couldn't open listener: '%s'\n", err.Error())
		return nil, nil
	}
	fmt.Printf("lis: '%v'\n", lis)

	// Remove any existing labels on the node
	host, err := os.Hostname()
	if err != nil {
		fmt.Printf("couldn't fetch hostname: %s", err)
		return nil, nil
	}

	customTopology := os.Getenv("X_CSI_CUSTOM_TOPOLOGY_ENABLED")
	endPoint := os.Getenv("X_CSI_ISI_ENDPOINT")
	if customTopology == "true" {
		if removeNodeLabels(host) {
			if !applyNodeLabel(host, endPoint) {
				fmt.Printf("Could not remove node label")
				return nil, nil
			}
		}
	}

	go func() {
		fmt.Printf("starting server\n")
		if err := sp.Serve(ctx, lis); err != nil {
			fmt.Printf("http: Server closed")
		}
	}()
	network, addr, err := utils.GetCSIEndpoint()
	if err != nil {
		return nil, nil
	}
	fmt.Printf("network '%v' addr '%v'\n", network, addr)

	clientOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Create a client for the piped connection.
	fmt.Printf("calling gprc.DialContext, ctx '%v', addr '%s', clientOpts '%v'\n", ctx, addr, clientOpts)
	client, err := grpc.DialContext(ctx, "unix:"+addr, clientOpts...)
	if err != nil {
		fmt.Printf("DialContext returned error: '%s'", err.Error())
	}
	fmt.Printf("grpc.DialContext returned ok\n")

	return client, func() {
		client.Close()
		sp.GracefulStop(ctx)
	}
}
