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

/*
Copyright (c) 2025 Dell Inc, or its subsidiaries.

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

package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	csiext "github.com/dell/dell-csi-extensions/replication"
	isi "github.com/dell/goisilon"
	v11 "github.com/dell/goisilon/api/v11"
	"github.com/dell/goisilon/mocks"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var anyArgs = []interface{}{mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything}

// setUpSvcForFailbackDiscardLocal sets up an Isilon cluster config with mock client for local and remote clusters
// there are 9 different mock calls used for a sucsessful run.
// failStep is used to determine which of the 9 calls shoud fail to test error handling. An int outside of 1-9 range means call will succeed.
// TODO: function can be made more granular by replacing .Times() calls with seperate mocks to allow greater percision
func setUpSvcForFailbackDiscardLocal(failStep int) (*IsilonClusterConfig, *IsilonClusterConfig) {
	mockClient := &MockClient{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	localIsiConfig := &IsilonClusterConfig{
		IsiPath: "/ifs/data",
		isiSvc:  svc,
	}
	remoteIsiConfig := &IsilonClusterConfig{
		IsiPath: "/ifs/data",
		isiSvc:  svc,
	}

	if failStep != 1 {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(**v11.Policies)
			*resp = &v11.Policies{
				Policy: []v11.Policy{
					{
						ID:   "test-id",
						Name: "test-name",
					},
				},
			}
		}).Times(3)
	} else {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Times(3)
	}

	if failStep != 2 {
		svc.client.API.(*MockClient).On("Put", anyArgs...).Return(nil).Times(3)
	} else {
		svc.client.API.(*MockClient).On("Put", anyArgs...).Return(errors.New("mock error")).Times(3)
	}

	if failStep != 3 {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(**v11.TargetPolicies)
			*resp = &v11.TargetPolicies{
				Policy: []v11.TargetPolicy{
					{
						ID:                    "test-id",
						Name:                  "test-name",
						FailoverFailbackState: ResyncPolicyCreated,
					},
				},
			}
		}).Times(1)
	} else {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Times(1)
	}

	if failStep != 4 {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(**v11.Policies)
			*resp = &v11.Policies{
				Policy: []v11.Policy{
					{
						ID:      "test-id",
						Name:    "test-name",
						Enabled: true,
					},
				},
			}
		}).Times(2)
	} else {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Times(2)
	}
	if failStep != 5 {
		svc.client.API.(*MockClient).On("Get", anyArgs[0:6]...).Return(nil).Times(2)
	} else {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Times(2)
	}
	if failStep != 6 {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(**v11.TargetPolicies)
			*resp = &v11.TargetPolicies{
				Policy: []v11.TargetPolicy{
					{
						ID:                    "test-id",
						Name:                  "test-name",
						FailoverFailbackState: WritesEnabled,
					},
				},
			}
		}).Times(2)
	} else {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Times(2)
	}

	if failStep != 7 {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(**v11.TargetPolicies)
			*resp = &v11.TargetPolicies{
				Policy: []v11.TargetPolicy{
					{
						ID:                    "test-id",
						Name:                  "test-name",
						FailoverFailbackState: ResyncPolicyCreated,
					},
				},
			}
		}).Times(1)
	} else {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Times(2)
	}

	if failStep != 8 {
		svc.client.API.(*MockClient).On("Get", anyArgs[0:6]...).Return(nil).Times(1)
	} else {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Times(1)
	}
	if failStep != 9 {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(**v11.Policies)
			*resp = &v11.Policies{
				Policy: []v11.Policy{
					{
						ID:      "test-id",
						Name:    "test-name",
						Enabled: true,
					},
				},
			}
		}).Times(1)
	} else {
		svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Times(1)
	}

	svc.client.API.(*MockClient).On("Post", anyArgs...).Return(nil)
	svc.client.API.(*MockClient).On("Delete", anyArgs...).Return(nil)
	return localIsiConfig, remoteIsiConfig
}

func Test_failbackDiscardLocal(t *testing.T) {
	tests := []struct {
		name     string
		failStep int
		wantErr  string
	}{
		{
			name:     "good run",
			failStep: 0,
			wantErr:  "",
		},
		{
			name:     "Test TGT mirror policy fails to reach enabled",
			failStep: 4,
			wantErr:  "TGT mirror policy couldn't reach enabled condition",
		},
		{
			name:     "Test Syncing Policy Fails",
			failStep: 5,
			wantErr:  "policy sync failed",
		},
		{
			name:     "Test allowing writes on local site fails",
			failStep: 6,
			wantErr:  "allow writes on local site failed",
		},
		{
			name:     "Test setting policy to automatic fails",
			failStep: 9,
			wantErr:  "can't set local policy to automatic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localIsiConfig, remoteIsiConfig := setUpSvcForFailbackDiscardLocal(tt.failStep)

			err := failbackDiscardLocal(context.Background(), localIsiConfig, remoteIsiConfig, "vgstest-Five_Minutes", logrus.NewEntry(logrus.New()))
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func Test_synchronize(t *testing.T) {
	mockClient := &MockClient{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	localIsiConfig := &IsilonClusterConfig{
		IsiPath: "/ifs/data",
		isiSvc:  svc,
	}
	remoteIsiConfig := &IsilonClusterConfig{
		IsiPath: "/ifs/data",
		isiSvc:  svc,
	}

	ppName := "vgstest-Five_Minutes"

	// Negative case - when policy sync failed
	svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("policy sync failed")).Run(nil).Times(1)
	err := synchronize(context.Background(), localIsiConfig, remoteIsiConfig, ppName, logrus.NewEntry(logrus.New()))
	assert.Error(t, err)

	// Positive cases
	svc.client.API.(*MockClient).Calls = nil
	svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**v11.Policies)
		*resp = &v11.Policies{
			Policy: []v11.Policy{
				{
					ID:      "test-id",
					Name:    "test-name",
					Enabled: true,
				},
			},
		}
	}).Times(1)
	svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**v11.Jobs)
		*resp = &v11.Jobs{
			Job: []v11.Job{
				{
					ID:     "test-id",
					Action: "sync",
				},
			},
		}
	}).Times(2)

	svc.client.API.(*MockClient).On("Post", anyArgs...).Return(nil).Run(nil).Times(1)
	svc.client.API.(*MockClient).On("Get", anyArgs[0:6]...).Return(nil).Times(1)

	svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**v11.Policies)
		*resp = &v11.Policies{
			Policy: []v11.Policy{
				{
					ID:      "test-id",
					Name:    "test-name",
					Enabled: true,
				},
			},
		}
	}).Times(1)
	svc.client.API.(*MockClient).On("Put", anyArgs...).Return(nil).Run(nil).Times(1)
	svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**v11.Jobs)
		*resp = &v11.Jobs{
			Job: []v11.Job{
				{
					ID: "test-id",
				},
			},
		}
	}).Times(1)

	err = synchronize(context.Background(), localIsiConfig, remoteIsiConfig, ppName, logrus.NewEntry(logrus.New()))
	assert.NoError(t, err)
}

func Test_suspend(t *testing.T) {
	mockClient := &MockClient{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	localIsiConfig := &IsilonClusterConfig{
		IsiPath: "/ifs/data",
		isiSvc:  svc,
	}
	remoteIsiConfig := &IsilonClusterConfig{
		IsiPath: "/ifs/data",
		isiSvc:  svc,
	}

	ppName := "vgstest-Five_Minutes"

	// Negative case - can't disable local policy
	svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("can't disable local policy")).Run(nil).Times(1)
	err := suspend(context.Background(), localIsiConfig, remoteIsiConfig, ppName, logrus.NewEntry(logrus.New()))
	assert.Error(t, err)

	// Negative case - policy couldn't reach disabled condition
	svc.client.API.(*MockClient).Calls = nil
	svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**v11.Policies)
		*resp = &v11.Policies{
			Policy: []v11.Policy{
				{
					ID:   "test-id",
					Name: "test-name",
				},
			},
		}
	}).Times(1)
	svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("policy couldn't reach disabled condition")).Run(nil).Times(1)

	err = suspend(context.Background(), localIsiConfig, remoteIsiConfig, ppName, logrus.NewEntry(logrus.New()))
	assert.Error(t, err)

	// Positive cases
	svc.client.API.(*MockClient).Calls = nil
	svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**v11.Policies)
		*resp = &v11.Policies{
			Policy: []v11.Policy{
				{
					ID:   "test-id",
					Name: "test-name",
				},
			},
		}
	}).Times(2)

	err = suspend(context.Background(), localIsiConfig, remoteIsiConfig, ppName, logrus.NewEntry(logrus.New()))
	assert.NoError(t, err)
}

// This test is a WIP, currently tests "happy path" in failbackDiscardRemote
func Test_failbackDiscardRemote(t *testing.T) {
	mockClient := &MockClient{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	localIsiConfig := &IsilonClusterConfig{
		IsiPath: "/ifs/data",
		isiSvc:  svc,
	}
	remoteIsiConfig := &IsilonClusterConfig{
		IsiPath: "/ifs/data",
		isiSvc:  svc,
	}

	svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**v11.Policies)
		*resp = &v11.Policies{
			Policy: []v11.Policy{
				{
					ID:   "test-id",
					Name: "test-name",
				},
			},
		}
	}).Times(2)

	svc.client.API.(*MockClient).On("Put", anyArgs...).Return(nil).Times(3)

	svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**v11.TargetPolicies)
		*resp = &v11.TargetPolicies{
			Policy: []v11.TargetPolicy{
				{
					ID:                    "test-id",
					Name:                  "test-name",
					FailoverFailbackState: WritesDisabled,
				},
			},
		}
	}).Times(1)

	svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**v11.Policies)
		*resp = &v11.Policies{
			Policy: []v11.Policy{
				{
					ID:      "test-id",
					Name:    "test-name",
					Enabled: true,
				},
			},
		}
	}).Times(2)

	err := failbackDiscardRemote(context.Background(), localIsiConfig, remoteIsiConfig, "vgstest-Five_Minutes", logrus.NewEntry(logrus.New()))
	assert.NoError(t, err)
}

func Test_getGroupLinkState(t *testing.T) {
	type args struct {
		localP   v11.Policy
		localTP  v11.TargetPolicy
		remoteP  v11.Policy
		remoteTP v11.TargetPolicy
	}

	// we will set these to the v11 fields in the for loop
	var isiLocalp isi.Policy
	var isiLocalTP isi.TargetPolicy
	var isiRemoteP isi.Policy
	var isiRemoteTP isi.TargetPolicy

	tests := []struct {
		name string
		args args
		want csiext.StorageProtectionGroupStatus_State
	}{
		{
			name: "state is StorageProtectionGroupStatus_SYNCHRONIZED",
			args: args{
				localP: v11.Policy{
					// setting name to nil marks this policy as nil for testing
					Name: "nil",
				},
				localTP: v11.TargetPolicy{
					FailoverFailbackState: WritesDisabled,
				},
				remoteP: v11.Policy{
					Enabled: true,
				},
				remoteTP: v11.TargetPolicy{
					Name: "nil",
				},
			},
			want: csiext.StorageProtectionGroupStatus_SYNCHRONIZED,
		},
		{
			name: "state is StorageProtectionGroupStatus_SUSPENDED",
			args: args{
				localP: v11.Policy{
					Enabled: false,
				},
				localTP: v11.TargetPolicy{
					Name: "nil",
				},
				remoteP: v11.Policy{
					Name: "nil",
				},
				remoteTP: v11.TargetPolicy{
					FailoverFailbackState: WritesDisabled,
				},
			},
			want: csiext.StorageProtectionGroupStatus_SUSPENDED,
		},
		{
			name: "state is StorageProtectionGroupStatus_FAILEDOVER",
			args: args{
				localP: v11.Policy{
					Enabled: false,
				},
				localTP: v11.TargetPolicy{
					Name: "nil",
				},
				remoteP: v11.Policy{
					Name: "nil",
				},
				remoteTP: v11.TargetPolicy{
					FailoverFailbackState: WritesEnabled,
				},
			},
			want: csiext.StorageProtectionGroupStatus_FAILEDOVER,
		},
		{
			name: "state is StorageProtectionGroupStatus_UNKNOWN",
			args: args{
				localP: v11.Policy{
					Name: "nil",
				},
				localTP: v11.TargetPolicy{
					Name: "nil",
				},
				remoteP: v11.Policy{
					Name: "nil",
				},
				remoteTP: v11.TargetPolicy{
					FailoverFailbackState: WritesEnabled,
				},
			},
			want: csiext.StorageProtectionGroupStatus_UNKNOWN,
		},
		{
			name: "state is StorageProtectionGroupStatus_FAILEDOVER",
			args: args{
				localP: v11.Policy{
					Enabled: false,
				},
				localTP: v11.TargetPolicy{
					Name: "nil",
				},
				remoteP: v11.Policy{
					Name: "nil",
				},
				remoteTP: v11.TargetPolicy{
					FailoverFailbackState: WritesEnabled,
				},
			},
			want: csiext.StorageProtectionGroupStatus_FAILEDOVER,
		},
		{
			name: "state is StorageProtectionGroupStatus_FAILEDOVER second case",
			args: args{
				localP: v11.Policy{
					Name: "nil",
				},
				localTP: v11.TargetPolicy{
					FailoverFailbackState: WritesEnabled,
				},
				remoteP: v11.Policy{
					Name: "nil",
				},
				remoteTP: v11.TargetPolicy{
					Name: "nil",
				},
			},
			want: csiext.StorageProtectionGroupStatus_FAILEDOVER,
		},
		{
			name: "state is StorageProtectionGroupStatus_FAILEDOVER third case",
			args: args{
				localP: v11.Policy{
					Name: "nil",
				},
				localTP: v11.TargetPolicy{
					FailoverFailbackState: WritesEnabled,
				},
				remoteP: v11.Policy{
					Enabled: true,
				},
				remoteTP: v11.TargetPolicy{
					Name: "nil",
				},
			},
			want: csiext.StorageProtectionGroupStatus_FAILEDOVER,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// set all policies to nil to start
			isiLocalp = nil
			isiLocalTP = nil
			isiRemoteP = nil
			isiRemoteTP = nil

			// if polciy in test case is not marked to be nil via name field, assign it to test case policy
			if tt.args.localP.Name != "nil" {
				isiLocalp = &tt.args.localP
			}
			if tt.args.localTP.Name != "nil" {
				isiLocalTP = &tt.args.localTP
			}
			if tt.args.remoteP.Name != "nil" {
				isiRemoteP = &tt.args.remoteP
			}
			if tt.args.remoteTP.Name != "nil" {
				isiRemoteTP = &tt.args.remoteTP
			}

			if got := getGroupLinkState(isiLocalp, isiLocalTP, isiRemoteP, isiRemoteTP, false); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getGroupLinkState returned state: %v, expected state to be: %v", got, tt.want)
			}
		})
	}
}

func Test_reprotect(t *testing.T) {
	// Create a new instance of the isiService struct
	localSvc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: &mocks.Client{},
		},
	}
	remoteSvc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: &mocks.Client{},
		},
	}

	type args struct {
		ctx             context.Context
		localIsiConfig  *IsilonClusterConfig
		remoteIsiConfig *IsilonClusterConfig
		vgName          string
		log             *logrus.Entry
	}
	tests := []struct {
		name     string
		args     args
		setMocks func()
		wantErr  bool
	}{
		{
			name: "cannot get local target policy",
			args: args{
				ctx: context.Background(),
				localIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  localSvc,
				},
				remoteIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  remoteSvc,
				},
				vgName: "csi-vg-test",
				log:    logrus.NewEntry(logrus.New()),
			},
			setMocks: func() {
				// mocks function: localIsiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
				localSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/target/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(
						errors.New("not found"),
					).Once()
			},
			wantErr: true,
		},
		{
			name: "local target policy is not write-enabled",
			args: args{
				ctx: context.Background(),
				localIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  localSvc,
				},
				remoteIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  remoteSvc,
				},
				vgName: "csi-vg-test",
				log:    logrus.NewEntry(logrus.New()),
			},
			setMocks: func() {
				// mocks function: localIsiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
				localSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/target/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.TargetPolicies)
					*resp = &v11.TargetPolicies{
						Policy: []v11.TargetPolicy{
							{
								ID:                    "test-id",
								Name:                  "test-name",
								FailoverFailbackState: isi.WritesDisabled,
							},
						},
					}
				}).Once()
			},
			wantErr: true,
		},
		{
			name: "cannot get remote SyncIQ policy",
			args: args{
				ctx: context.Background(),
				localIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  localSvc,
				},
				remoteIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  remoteSvc,
				},
				vgName: "csi-vg-test",
				log:    logrus.NewEntry(logrus.New()),
			},
			setMocks: func() {
				// mocks function: localIsiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
				localSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/target/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.TargetPolicies)
					*resp = &v11.TargetPolicies{
						Policy: []v11.TargetPolicy{
							{
								ID:                    "test-id",
								Name:                  "test-name",
								FailoverFailbackState: isi.WritesEnabled,
							},
						},
					}
				}).Once()

				// mocks function: remoteIsiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
				remoteSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("not found")).Once()
			},
			wantErr: true,
		},
		{
			name: "fail to delete remote SyncIQ policy",
			args: args{
				ctx: context.Background(),
				localIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  localSvc,
				},
				remoteIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  remoteSvc,
				},
				vgName: "csi-vg-test",
				log:    logrus.NewEntry(logrus.New()),
			},
			setMocks: func() {
				// mocks function: localIsiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
				localSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/target/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.TargetPolicies)
					*resp = &v11.TargetPolicies{
						Policy: []v11.TargetPolicy{
							{
								ID:                    "test-id",
								Name:                  "test-name",
								FailoverFailbackState: isi.WritesEnabled,
							},
						},
					}
				}).Once()

				// mocks function: remoteIsiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
				remoteSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.Policies)
					*resp = &v11.Policies{
						Policy: []v11.Policy{
							{
								ID:         "test-id",
								Name:       "test-name",
								JobDelay:   5,
								TargetPath: "target-path",
								SourcePath: "source-path",
							},
						},
					}
				}).Once()

				// mocks function: remoteIsiConfig.isiSvc.client.DeletePolicy(ctx, ppName)
				remoteSvc.client.API.(*mocks.Client).On("Delete", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("error: unable to delete policy")).Once()
			},
			wantErr: true,
		},
		{
			name: "fail to create new local SyncIQ policy",
			args: args{
				ctx: context.Background(),
				localIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  localSvc,
				},
				remoteIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  remoteSvc,
				},
				vgName: "csi-vg-test",
				log:    logrus.NewEntry(logrus.New()),
			},
			setMocks: func() {
				// mocks function: localIsiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
				localSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/target/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.TargetPolicies)
					*resp = &v11.TargetPolicies{
						Policy: []v11.TargetPolicy{
							{
								ID:                    "test-id",
								Name:                  "test-name",
								FailoverFailbackState: isi.WritesEnabled,
							},
						},
					}
				}).Once()

				// mocks function: remoteIsiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
				remoteSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.Policies)
					*resp = &v11.Policies{
						Policy: []v11.Policy{
							{
								ID:         "test-id",
								Name:       "test-name",
								JobDelay:   5,
								TargetPath: "target-path",
								SourcePath: "source-path",
							},
						},
					}
				}).Once()

				// mocks function: remoteIsiConfig.isiSvc.client.DeletePolicy(ctx, ppName)
				remoteSvc.client.API.(*mocks.Client).On("Delete", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()

				// mocks function: localIsiConfig.isiSvc.client.CreatePolicy(ctx, ppName, remotePolicy.JobDelay,
				localSvc.client.API.(*mocks.Client).On("Post", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("error: unable to create policy")).Once()
			},
			wantErr: true,
		},
		{
			name: "fail to get initial sync status",
			args: args{
				ctx: context.Background(),
				localIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  localSvc,
				},
				remoteIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  remoteSvc,
				},
				vgName: "csi-vg-test",
				log:    logrus.NewEntry(logrus.New()),
			},
			setMocks: func() {
				// mocks function: localIsiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
				localSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/target/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.TargetPolicies)
					*resp = &v11.TargetPolicies{
						Policy: []v11.TargetPolicy{
							{
								ID:                    "test-id",
								Name:                  "test-name",
								FailoverFailbackState: isi.WritesEnabled,
							},
						},
					}
				}).Once()

				// mocks function: remoteIsiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
				remoteSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.Policies)
					*resp = &v11.Policies{
						Policy: []v11.Policy{
							{
								ID:         "test-id",
								Name:       "test-name",
								JobDelay:   5,
								TargetPath: "target-path",
								SourcePath: "source-path",
							},
						},
					}
				}).Once()

				// mocks function: remoteIsiConfig.isiSvc.client.DeletePolicy(ctx, ppName)
				remoteSvc.client.API.(*mocks.Client).On("Delete", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()

				// mocks function: localIsiConfig.isiSvc.client.CreatePolicy(ctx, ppName, remotePolicy.JobDelay,
				localSvc.client.API.(*mocks.Client).On("Post", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()

				// mocks function: localIsiConfig.isiSvc.client.WaitForPolicyLastJobState(ctx, ppName, isi.RUNNING)
				localSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("error: failed to get policy")).Once()
			},
			wantErr: true,
		},
		{
			name: "success when policy last job state is RUNNING",
			args: args{
				ctx: context.Background(),
				localIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  localSvc,
				},
				remoteIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  remoteSvc,
				},
				vgName: "csi-vg-test",
				log:    logrus.NewEntry(logrus.New()),
			},
			setMocks: func() {
				// mocks function: localIsiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
				localSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/target/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.TargetPolicies)
					*resp = &v11.TargetPolicies{
						Policy: []v11.TargetPolicy{
							{
								ID:                    "test-id",
								Name:                  "test-name",
								FailoverFailbackState: isi.WritesEnabled,
							},
						},
					}
				}).Once()

				// mocks function: remoteIsiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
				remoteSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.Policies)
					*resp = &v11.Policies{
						Policy: []v11.Policy{
							{
								ID:         "test-id",
								Name:       "test-name",
								JobDelay:   5,
								TargetPath: "target-path",
								SourcePath: "source-path",
							},
						},
					}
				}).Once()

				// mocks function: remoteIsiConfig.isiSvc.client.DeletePolicy(ctx, ppName)
				remoteSvc.client.API.(*mocks.Client).On("Delete", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()

				// mocks function: localIsiConfig.isiSvc.client.CreatePolicy(ctx, ppName, remotePolicy.JobDelay,
				localSvc.client.API.(*mocks.Client).On("Post", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()

				// mocks function: localIsiConfig.isiSvc.client.WaitForPolicyLastJobState(ctx, ppName, isi.RUNNING)
				localSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.Policies)
					*resp = &v11.Policies{
						Policy: []v11.Policy{
							{
								ID:           "test-id",
								Name:         "test-name",
								JobDelay:     5,
								TargetPath:   "target-path",
								SourcePath:   "source-path",
								LastJobState: isi.RUNNING,
							},
						},
					}
				})
			},
			wantErr: false,
		},
		{
			name: "success when policy last job state is FINISHED",
			args: args{
				ctx: context.Background(),
				localIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  localSvc,
				},
				remoteIsiConfig: &IsilonClusterConfig{
					IsiPath: "/ifs/data",
					isiSvc:  remoteSvc,
				},
				vgName: "csi-vg-test",
				log:    logrus.NewEntry(logrus.New()),
			},
			setMocks: func() {
				// mocks function: localIsiConfig.isiSvc.client.GetTargetPolicyByName(ctx, ppName)
				localSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/target/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.TargetPolicies)
					*resp = &v11.TargetPolicies{
						Policy: []v11.TargetPolicy{
							{
								ID:                    "test-id",
								Name:                  "test-name",
								FailoverFailbackState: isi.WritesEnabled,
							},
						},
					}
				}).Once()

				// mocks function: remoteIsiConfig.isiSvc.client.GetPolicyByName(ctx, ppName)
				remoteSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.Policies)
					*resp = &v11.Policies{
						Policy: []v11.Policy{
							{
								ID:         "test-id",
								Name:       "test-name",
								JobDelay:   5,
								TargetPath: "target-path",
								SourcePath: "source-path",
							},
						},
					}
				}).Once()

				// mocks function: remoteIsiConfig.isiSvc.client.DeletePolicy(ctx, ppName)
				remoteSvc.client.API.(*mocks.Client).On("Delete", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()

				// mocks function: localIsiConfig.isiSvc.client.CreatePolicy(ctx, ppName, remotePolicy.JobDelay,
				localSvc.client.API.(*mocks.Client).On("Post", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil).Once()

				// mocks function: localIsiConfig.isiSvc.client.WaitForPolicyLastJobState(ctx, ppName, isi.RUNNING)
				localSvc.client.API.(*mocks.Client).On("Get", mock.Anything, "/platform/11/sync/policies/", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**v11.Policies)
					*resp = &v11.Policies{
						Policy: []v11.Policy{
							{
								ID:           "test-id",
								Name:         "test-name",
								JobDelay:     5,
								TargetPath:   "target-path",
								SourcePath:   "source-path",
								LastJobState: isi.FINISHED,
							},
						},
					}
				})
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setMocks()

			if err := reprotect(tt.args.ctx, tt.args.localIsiConfig, tt.args.remoteIsiConfig, tt.args.vgName, tt.args.log); (err != nil) != tt.wantErr {
				t.Errorf("reprotect() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
