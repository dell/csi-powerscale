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

	csiext "github.com/dell/dell-csi-extensions/replication"

	"strings"

	"golang.org/x/net/context"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"

	"github.com/dell/csi-isilon/common/constants"
	"github.com/dell/csi-isilon/core"
)

func (s *service) GetPluginInfo(
	ctx context.Context,
	req *csi.GetPluginInfoRequest) (
	*csi.GetPluginInfoResponse, error) {

	return &csi.GetPluginInfoResponse{
		Name:          constants.PluginName,
		VendorVersion: core.SemVer,
		Manifest:      Manifest,
	}, nil
}

func (s *service) GetPluginCapabilities(
	ctx context.Context,
	req *csi.GetPluginCapabilitiesRequest) (
	*csi.GetPluginCapabilitiesResponse, error) {

	var rep csi.GetPluginCapabilitiesResponse
	if !strings.EqualFold(s.mode, "node") {
		rep.Capabilities = []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_VolumeExpansion_{
					VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
						Type: csi.PluginCapability_VolumeExpansion_ONLINE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_VolumeExpansion_{
					VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
						Type: csi.PluginCapability_VolumeExpansion_OFFLINE,
					},
				},
			},
		}
	}
	return &rep, nil
}

func (s *service) Probe(
	ctx context.Context,
	req *csi.ProbeRequest) (
	*csi.ProbeResponse, error) {
	ctx, log := GetLogger(ctx)
	ready := new(wrappers.BoolValue)
	ready.Value = true
	rep := new(csi.ProbeResponse)
	rep.Ready = ready

	if err := s.probeAllClusters(ctx); err != nil {
		rep.Ready.Value = false
		return rep, err
	}
	log.Debugf(fmt.Sprintf("Probe returning: %v", rep.Ready.GetValue()))
	return rep, nil
}

func (s *service) GetReplicationCapabilities(ctx context.Context, req *csiext.GetReplicationCapabilityRequest) (*csiext.GetReplicationCapabilityResponse, error) {
	var rep = new(csiext.GetReplicationCapabilityResponse)
	if !strings.EqualFold(s.mode, "node") {
		rep.Capabilities = []*csiext.ReplicationCapability{
			{
				Type: &csiext.ReplicationCapability_Rpc{
					Rpc: &csiext.ReplicationCapability_RPC{
						Type: csiext.ReplicationCapability_RPC_CREATE_REMOTE_VOLUME,
					},
				},
			},
			{
				Type: &csiext.ReplicationCapability_Rpc{
					Rpc: &csiext.ReplicationCapability_RPC{
						Type: csiext.ReplicationCapability_RPC_CREATE_PROTECTION_GROUP,
					},
				},
			},
			{
				Type: &csiext.ReplicationCapability_Rpc{
					Rpc: &csiext.ReplicationCapability_RPC{
						Type: csiext.ReplicationCapability_RPC_DELETE_PROTECTION_GROUP,
					},
				},
			},
			{
				Type: &csiext.ReplicationCapability_Rpc{
					Rpc: &csiext.ReplicationCapability_RPC{
						Type: csiext.ReplicationCapability_RPC_REPLICATION_ACTION_EXECUTION,
					},
				},
			},
			// {
			// 	Type: &csiext.ReplicationCapability_Rpc{
			// 		Rpc: &csiext.ReplicationCapability_RPC{
			// 			Type: csiext.ReplicationCapability_RPC_MONITOR_PROTECTION_GROUP,
			// 		},
			// 	},
			// },
		}
		rep.Actions = []*csiext.SupportedActions{
			{
				Actions: &csiext.SupportedActions_Type{
					Type: csiext.ActionTypes_FAILOVER_REMOTE,
				},
			},
			{
				Actions: &csiext.SupportedActions_Type{
					Type: csiext.ActionTypes_UNPLANNED_FAILOVER_LOCAL,
				},
			},
			{
				Actions: &csiext.SupportedActions_Type{
					Type: csiext.ActionTypes_REPROTECT_LOCAL,
				},
			},
			{
				Actions: &csiext.SupportedActions_Type{
					Type: csiext.ActionTypes_SUSPEND,
				},
			},
			{
				Actions: &csiext.SupportedActions_Type{
					Type: csiext.ActionTypes_RESUME,
				},
			},
			{
				Actions: &csiext.SupportedActions_Type{
					Type: csiext.ActionTypes_SYNC,
				},
			},
		}
	}
	return rep, nil
}
