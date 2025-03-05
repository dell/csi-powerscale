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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNodeHealth(t *testing.T) {
	// Create a new http.ResponseWriter and http.Request for testing
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/node-status", nil)

	// Call the nodeHealth function
	nodeHealth(w, r)

	// Check the response status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, but got %d", http.StatusOK, w.Code)
	}

	// Check the response body
	expectedResponse := "node is up and running \n"
	if w.Body.String() != expectedResponse {
		t.Errorf("Expected response body '%s', but got '%s'", expectedResponse, w.Body.String())
	}
}

func TestGetArrayConnectivityStatus(t *testing.T) {
	// Setup router
	router := mux.NewRouter()
	router.HandleFunc("/arrayStatus/{arrayId}", getArrayConnectivityStatus).Methods("GET")

	// Initialize probeStatus
	probeStatus = &sync.Map{}

	// Populate probeStatus with test data
	probeStatus.Store("cluster1", ArrayConnectivityStatus{LastSuccess: 1617181723, LastAttempt: 1617181724})
	probeStatus.Store("cluster2", ArrayConnectivityStatus{LastSuccess: 1617181725, LastAttempt: 1617181726})

	// Test cases
	tests := []struct {
		arrayID       string
		expectedCode  int
		expectedBody  string
		mockStatus    interface{}
		mockStatusSet bool
	}{
		{
			arrayID:       "existingArray",
			expectedCode:  http.StatusOK,
			expectedBody:  `{"status":"connected"}`,
			mockStatus:    map[string]string{"status": "connected"},
			mockStatusSet: true,
		},
		{
			arrayID:       "nonExistingArray",
			expectedCode:  http.StatusNotFound,
			expectedBody:  "array nonExistingArray not found \n",
			mockStatusSet: false,
		},
		{
			arrayID:       "errorArray",
			expectedCode:  http.StatusInternalServerError,
			expectedBody:  "",
			mockStatus:    make(chan int), // This will cause json.Marshal to fail
			mockStatusSet: true,
		},
	}

	for _, tt := range tests {
		if tt.mockStatusSet {
			probeStatus.Store(tt.arrayID, tt.mockStatus)
		} else {
			probeStatus.Delete(tt.arrayID)
		}

		req, err := http.NewRequest("GET", fmt.Sprintf("/arrayStatus/%s", tt.arrayID), nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, tt.expectedCode, rr.Code)
		if tt.expectedCode == http.StatusOK {
			assert.JSONEq(t, tt.expectedBody, rr.Body.String())
		} else {
			assert.Equal(t, tt.expectedBody, rr.Body.String())
		}
	}
}

// Mock for MarshalSyncMapToJSON
type MockMarshal struct {
	mock.Mock
}

func (m *MockMarshal) MarshalSyncMapToJSON(_ *sync.Map) ([]byte, error) {
	args := m.Called()
	return nil, args.Error(1)
}

func TestConnectivityStatus_Success(t *testing.T) {
	// Set up the test logger
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	// Initialize probeStatus
	probeStatus = &sync.Map{}

	// Populate probeStatus with test data
	probeStatus.Store("cluster1", ArrayConnectivityStatus{LastSuccess: 1617181723, LastAttempt: 1617181724})
	probeStatus.Store("cluster2", ArrayConnectivityStatus{LastSuccess: 1617181725, LastAttempt: 1617181726})

	// Create a request to pass to our handler
	req, err := http.NewRequest("GET", "/arrayStatus", nil)
	assert.NoError(t, err)

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Create a handler function
	handler := http.HandlerFunc(connectivityStatus)

	// Call the handler with our ResponseRecorder and request
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check the Content-Type header is what we expect
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	// Check the response body is what we expect
	expectedResponse, err := MarshalSyncMapToJSON(probeStatus)
	assert.NoError(t, err)
	assert.JSONEq(t, string(expectedResponse), rr.Body.String())
}

func TestConnectivityStatus_ErrorDuringMarshal(t *testing.T) {
	// Set up the test logger
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	// Initialize probeStatus
	probeStatus = &sync.Map{}

	// Populate probeStatus with test data
	probeStatus.Store("cluster1", ArrayConnectivityStatus{LastSuccess: 1617181723, LastAttempt: 1617181724})
	probeStatus.Store("cluster2", ArrayConnectivityStatus{LastSuccess: 1617181725, LastAttempt: 1617181726})

	// Create a request to pass to our handler
	req, err := http.NewRequest("GET", "/arrayStatus", nil)
	assert.NoError(t, err)

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Create a mock for MarshalSyncMapToJSON
	mockMarshal := new(MockMarshal)
	mockMarshal.On("MarshalSyncMapToJSON").Return(nil, errors.New("mock error"))

	// Replace the original MarshalSyncMapToJSON with the mock
	originalMarshal := MarshalSyncMapToJSON
	MarshalSyncMapToJSON = mockMarshal.MarshalSyncMapToJSON
	defer func() { MarshalSyncMapToJSON = originalMarshal }()

	// Create a handler function
	handler := http.HandlerFunc(connectivityStatus)

	// Call the handler with our ResponseRecorder and request
	handler.ServeHTTP(rr, req)

	// Check the Content-Type header is what we expect
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}

func TestStartNodeToArrayConnectivityCheck(_ *testing.T) {
	svc := &service{
		isiClusters: &sync.Map{},
	}

	// Populate isiClusters with mock data
	cluster1 := &IsilonClusterConfig{ClusterName: "Cluster1"}
	cluster2 := &IsilonClusterConfig{ClusterName: "Cluster2"}
	svc.isiClusters.Store("key1", cluster1)
	svc.isiClusters.Store("key2", cluster2)

	ctx := context.Background()
	svc.startNodeToArrayConnectivityCheck(ctx)
}

func TestSetAPIPort(_ *testing.T) {
	ctx := context.Background()
	os.Setenv(constants.EnvPodmonAPIPORT, "1")
	setAPIPort(ctx)
	os.Unsetenv(constants.EnvPodmonAPIPORT)

	os.Setenv(constants.EnvPodmonAPIPORT, "0")
	setAPIPort(ctx)
	os.Unsetenv(constants.EnvPodmonAPIPORT)
}

func TestSetPollingFrequency(t *testing.T) {
	os.Setenv(constants.EnvPodmonArrayConnectivityPollRate, "5")
	ctx := context.Background()
	pollrate := setPollingFrequency(ctx)
	assert.Equal(t, int64(5), pollrate)
	os.Unsetenv(constants.EnvPodmonArrayConnectivityPollRate)
}

func TestApiRouter(t *testing.T) {
	// Start the service in a separate goroutine
	s := &service{}
	go s.apiRouter(nil)

	tests := []struct {
		name               string
		url                string
		expectedStatusCode int
		expectedBody       string
	}{
		{
			name:         "Test nodeHealth endpoint",
			url:          nodeStatus,
			expectedBody: "node is up and running \n",
		},
		{
			name:         "Test connectivityStatus endpoint",
			url:          arrayStatus,
			expectedBody: "{\"Cluster1\":{\"lastSuccess\":0,\"lastAttempt\":1740390628},\"Cluster2\":{\"lastSuccess\":0,\"lastAttempt\":1740390628}}",
		},
		{
			name:         "Test getArrayConnectivityStatus endpoint",
			url:          arrayStatus + "/123",
			expectedBody: "array 123 not found \n",
		},
	}

	// Delay to allow the server to start
	time.Sleep(100 * time.Millisecond)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get("http://localhost" + apiPort + tt.url)
			assert.NoError(t, err)

			defer resp.Body.Close()
			_, err = io.ReadAll(resp.Body)
			assert.NoError(t, err)
		})
	}
}

func TestStartAPIService(_ *testing.T) {
	var mu sync.Mutex
	s := service{
		mode: "controller",
	}
	ctx := context.Background()

	// Set environment variable
	os.Setenv(constants.EnvPodmonEnabled, "true")
	defer os.Unsetenv(constants.EnvPodmonEnabled)

	// Protect the startAPIService call with a mutex
	mu.Lock()
	s.startAPIService(ctx)
	mu.Unlock()
}
