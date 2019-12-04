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
package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/godog"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/csi-isilon/provider"
	"github.com/rexray/gocsi/utils"
	"google.golang.org/grpc"
)

const (
	datadir = "/tmp/datadir"
)

var grpcClient *grpc.ClientConn

func TestMain(m *testing.M) {
	var stop func()
	ctx := context.Background()
	fmt.Printf("calling startServer")
	grpcClient, stop = startServer(ctx)
	fmt.Printf("back from startServer")
	time.Sleep(5 * time.Second)

	// Make the file needed for NodeStage:
	//  /tmp/datadir    -- for file system mounts
	fmt.Printf("Checking '%s'\n", datadir)
	var fileMode os.FileMode
	fileMode = 0777
	err := os.Mkdir(datadir, fileMode)
	if err != nil && !os.IsExist(err) {
		fmt.Printf("'%s': '%s'\n", datadir, err)
	}

	exitVal := godog.RunWithOptions("godog", func(s *godog.Suite) {
		FeatureContext(s)
	}, godog.Options{
		Format: "pretty",
		Paths:  []string{os.Args[len(os.Args)-2]},
		Tags:   os.Args[len(os.Args)-1],
	})
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

func startServer(ctx context.Context) (*grpc.ClientConn, func()) {
	// Create a new SP instance and serve it with a piped connection.
	sp := provider.New()
	lis, err := utils.GetCSIEndpointListener()
	if err != nil {
		fmt.Printf("couldn't open listener: '%s'\n", err.Error())
		return nil, nil
	}
	fmt.Printf("lis: '%v'\n", lis)
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
		grpc.WithInsecure(),
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
