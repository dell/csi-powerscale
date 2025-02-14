package service

import (
	"context"
	"fmt"
	"testing"

	isi "github.com/dell/goisilon"
	"github.com/dell/goisilon/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockClient struct {
	mock.Mock
}

func (m *MockClient) APIVersion() uint8 {
	// Implement the logic for the APIVersion method
	// Return the desired API version and error
	return uint8(2)
}

func (m *MockClient) GetAuthToken() string {
	// Implement the logic for the APIVersion method
	// Return the desired API version and error
	return ""
}

func (m *MockClient) GetCSRFToken() string {
	// Implement the logic for the APIVersion method
	// Return the desired API version and error
	return ""
}

func (m *MockClient) GetReferer() string {
	// Implement the logic for the APIVersion method
	// Return the desired API version and error
	return ""
}

func (m *MockClient) SetAuthToken(token string) {

}

func (m *MockClient) SetCSRFToken(token string) {

}

func (m *MockClient) SetReferer(token string) {

}

func (m *MockClient) VolumePath(token string) string {
	return ""
}

func (m *MockClient) User() string {
	return ""
}

func (m *MockClient) VolumesPath() string {
	return ""
}

func (m *MockClient) Group() string {
	return ""
}

func (m *MockClient) Delete(
	ctx context.Context,
	path, id string,
	params api.OrderedValues, headers map[string]string,
	resp interface{},
) error {
	return nil
}

func (m *MockClient) Do(
	ctx context.Context,
	method, path, id string,
	params api.OrderedValues,
	body, resp interface{},
) error {
	return nil
}

func (m *MockClient) DoWithHeaders(
	ctx context.Context,
	method, path, id string,
	params api.OrderedValues, headers map[string]string,
	body, resp interface{},
) error {
	return nil
}

func (m *MockClient) Get(
	ctx context.Context,
	path, id string,
	params api.OrderedValues, headers map[string]string,
	resp interface{},
) error {
	return nil
}

func (m *MockClient) Post(
	ctx context.Context,
	path, id string,
	params api.OrderedValues, headers map[string]string,
	body, resp interface{},
) error {
	return nil
}

func (m *MockClient) Put(
	ctx context.Context,
	path, id string,
	params api.OrderedValues, headers map[string]string,
	body, resp interface{},
) error {
	return nil
}

func TestCreateQuota(t *testing.T) {
	testCases := []struct {
		name            string
		isiPath         string
		volName         string
		softLimit       string
		advisoryLimit   string
		softGracePrd    string
		sizeInBytes     int64
		quotaEnabled    bool
		expectedQuotaID string
		expectedError   error
	}{
		{
			name:            "Invalid advisory limit",
			isiPath:         "/ifs/data/csi-isilon",
			volName:         "volume3",
			softLimit:       "70",
			advisoryLimit:   "invalid",
			softGracePrd:    "30",
			sizeInBytes:     100,
			quotaEnabled:    true,
			expectedQuotaID: "",
			expectedError:   fmt.Errorf("invalid advisory limit"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			mockClient := &MockClient{}

			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: mockClient,
				},
			}

			_, err := svc.CreateQuota(ctx, tc.isiPath, tc.volName, tc.softLimit, tc.advisoryLimit, tc.softGracePrd, tc.sizeInBytes, tc.quotaEnabled)
			assert.NoError(t, err)
		})
	}
}
