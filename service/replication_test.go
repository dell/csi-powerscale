package service

import (
	"testing"

	isi "github.com/dell/goisilon"
	v11 "github.com/dell/goisilon/api/v11"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/context"
)

var anyArgs = []interface{}{mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything}

// This test is a WIP, currently tests "happy path" in replication
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
