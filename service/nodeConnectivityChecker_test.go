package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
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

func TestConnectivityStatus(t *testing.T) {
	arrayID := "array1"
	status := ArrayConnectivityStatus{
		LastSuccess: time.Now().Unix(),
		LastAttempt: time.Now().Unix(),
	}
	probeStatus.Store(arrayID, status)
	// Test case: Successful response
	w := httptest.NewRecorder()
	connectivityStatus(w, httptest.NewRequest("GET", "/connectivity-status", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.True(t, strings.Contains(w.Body.String(), "array1"))
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
		isJSON        bool
	}{
		{
			arrayID:       "existingArray",
			expectedCode:  http.StatusOK,
			expectedBody:  `{"status":"connected"}`,
			mockStatus:    map[string]string{"status": "connected"},
			mockStatusSet: true,
			isJSON:        true,
		},
		{
			arrayID:       "nonExistingArray",
			expectedCode:  http.StatusNotFound,
			expectedBody:  "array nonExistingArray not found \n",
			mockStatusSet: false,
			isJSON:        false,
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
		if tt.isJSON {
			assert.JSONEq(t, tt.expectedBody, rr.Body.String())
		} else {
			assert.Equal(t, tt.expectedBody, rr.Body.String())
		}
	}
}
