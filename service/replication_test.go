package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	csiext "github.com/dell/dell-csi-extensions/replication"
	isi "github.com/dell/goisilon"
	v11 "github.com/dell/goisilon/api/v11"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var anyArgs = []interface{}{mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything}

// This test is a WIP, currently tests "happy path" in failbackDiscardLocal
func Test_failbackDiscardLocal(t *testing.T) {
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
	}).Times(3)

	svc.client.API.(*MockClient).On("Put", anyArgs...).Return(nil).Times(3)

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

	svc.client.API.(*MockClient).On("Get", anyArgs[0:6]...).Return(nil).Times(2)

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

	err := failbackDiscardLocal(context.Background(), localIsiConfig, remoteIsiConfig, "vgstest-Five_Minutes", logrus.NewEntry(logrus.New()))
	assert.NoError(t, err)
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
