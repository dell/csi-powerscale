/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package provider

/*
 Copyright (c) 2021-2022 Dell Inc, or its subsidiaries.

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
	"github.com/dell/csi-isilon/v2/common/utils/logging"
	csiutils "github.com/dell/csi-isilon/v2/csi-utils"
	"github.com/dell/csi-isilon/v2/service"
	"github.com/dell/csi-isilon/v2/service/interceptor"
	"github.com/dell/gocsi"
	"google.golang.org/grpc"
)

// New returns a new Storage Plug-in Provider.
func New() gocsi.StoragePluginProvider {
	log := logging.GetLogger()

	// TODO during the test, for some reason, when the controller & node pods start,
	// the sock files always exist right from the beginning, even if you manually
	// remove them prior to using helm to install the csi driver. Need to find out why.
	// For the time being, manually remove the sock files right at the beginning to
	// avoid the "...address is in use..." error
	if err := csiutils.RemoveExistingCSISockFile(); err != nil {
		log.Error("failed to call utils.RemoveExistingCSISockFile")
	}
	// Get the MaxConcurrentStreams server option and configure it.
	maxStreams := grpc.MaxConcurrentStreams(8)
	serverOptions := make([]grpc.ServerOption, 1)
	serverOptions[0] = maxStreams
	svc := service.New()

	interList := []grpc.UnaryServerInterceptor{
		interceptor.NewCustomSerialLock(),
		interceptor.NewRewriteRequestIDInterceptor(),
	}
	return &gocsi.StoragePlugin{
		Controller:                svc,
		Identity:                  svc,
		Node:                      svc,
		Interceptors:              interList,
		BeforeServe:               svc.BeforeServe,
		ServerOpts:                serverOptions,
		RegisterAdditionalServers: svc.RegisterAdditionalServers,

		EnvVars: []string{
			// Enable request validation
			gocsi.EnvVarSpecReqValidation + "=true",

			// Enable serial volume access
			gocsi.EnvVarSerialVolAccess + "=true",
		},
	}
}
