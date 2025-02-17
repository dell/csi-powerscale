package service

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

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
