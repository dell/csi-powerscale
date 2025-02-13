package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	arrayId := "array1"
	status := ArrayConnectivityStatus{
		LastSuccess: time.Now().Unix(),
		LastAttempt: time.Now().Unix(),
	}
	probeStatus.Store(arrayId, status)
	// Test case: Successful response
	w := httptest.NewRecorder()
	connectivityStatus(w, httptest.NewRequest("GET", "/connectivity-status", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.True(t, strings.Contains(w.Body.String(), "array1"))
}
