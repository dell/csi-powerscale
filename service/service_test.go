package service

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
import (
	"fmt"
	"github.com/dell/csi-isilon/common/constants"
	"net/http"
	_ "net/http/pprof"
	"os"
	"testing"

	"github.com/cucumber/godog"
)

func TestMain(m *testing.M) {
	status := 0

	go http.ListenAndServe("localhost:6060", nil)
	fmt.Printf("starting godog...\n")

	configFile := "mock/secret/secret.yaml"
	os.Setenv(constants.EnvIsilonConfigFile, configFile)
	status = godog.RunWithOptions("godog", func(s *godog.Suite) {
		FeatureContext(s)
	}, godog.Options{
		Format: "pretty",
		Paths:  []string{"features"},
		Tags:   "~todo",
	})
	fmt.Printf("godog finished\n")

	if st := m.Run(); st > status {
		status = st
	}

	fmt.Printf("status %d\n", status)

	os.Exit(status)
}
