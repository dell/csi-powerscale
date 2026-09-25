// Copyright © 2022-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	fromctx "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/fromcontext"
	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
	"github.com/gorilla/mux"
)

const (
	nodeStatus  = "/node-status"
	arrayStatus = "/array-status"
)

// pollingFrequency in seconds
var (
	pollingFrequencyInSeconds int64
	pollingFrequencyLock      sync.Mutex
)

// port for API calls
var apiPort string

// probeStatus map[string]ArrayConnectivityStatus
var probeStatus *sync.Map

// ArrayConnectivityStatus Status of the array probe
type ArrayConnectivityStatus struct {
	LastSuccess int64 `json:"lastSuccess"` // connectivity status
	LastAttempt int64 `json:"lastAttempt"` // last timestamp attempted to check connectivity
}

func setAPIPort(ctx context.Context) {
	port := fromctx.GetUint(ctx, constants.EnvPodmonAPIPORT)
	if port == 0 {
		// If the port number cannot be fetched, set it to default
		apiPort = ":" + constants.DefaultPodmonAPIPortNumber
		csmlog.WithContext(ctx).Debugf("set podmon API port to default %s", apiPort)
		return
	}
	apiPort = fmt.Sprintf(":%d", port)
	csmlog.WithContext(ctx).Debugf("set podmon API port to %s", apiPort)
}

// reads the pollingFrequency from Env, sets default if not found
func setPollingFrequency(ctx context.Context) int64 {
	pollRate, err := fromctx.GetInt64(ctx, constants.EnvPodmonArrayConnectivityPollRate)
	if err != nil || pollRate == 0 {
		csmlog.WithContext(ctx).Debugf("use default pollingFrequency %d seconds, err %v", constants.DefaultPodmonPollRate, err)
		return constants.DefaultPodmonPollRate
	}
	csmlog.WithContext(ctx).Debugf("use pollingFrequency as %d seconds", pollRate)
	return pollRate
}

// podmonAuthMiddleware returns a middleware that validates Bearer token authentication
// for the podmon API endpoints. If the token is empty, authentication is skipped
// for backward compatibility with deployments that have not yet configured a token.
func podmonAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := PodmonAPIToken
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}
		const bearerPrefix = "Bearer "
		// RFC 6750: the "Bearer" scheme is case-insensitive
		if !strings.HasPrefix(strings.ToLower(authHeader), strings.ToLower(bearerPrefix)) {
			http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
			return
		}
		provided := strings.TrimSpace(authHeader[len(bearerPrefix):])
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// MarshalSyncMapToJSON marshal the sync Map to Json
var MarshalSyncMapToJSON = func(m *sync.Map) ([]byte, error) {
	tmpMap := make(map[string]ArrayConnectivityStatus)
	m.Range(func(k, v interface{}) bool {
		// Ensure the value is of type ArrayConnectivityStatus
		if status, ok := v.(ArrayConnectivityStatus); ok {
			tmpMap[k.(string)] = status
		}
		return true
	})
	csmlog.Debugf("map value is %+v", tmpMap)
	return json.Marshal(tmpMap)
}

// startAPIService reads nodes to array status periodically
func (s *service) startAPIService(ctx context.Context) {
	isPodmonEnabled := fromctx.GetBoolean(ctx, constants.EnvPodmonEnabled)
	if !isPodmonEnabled {
		csmlog.WithContext(ctx).Info("podmon is not enabled")
		return
	}
	pollingFrequencyLock.Lock()
	pollingFrequencyInSeconds = setPollingFrequency(ctx)
	pollingFrequencyLock.Unlock()
	setAPIPort(ctx)

	// start methods based on mode
	if strings.EqualFold(s.mode, constants.ModeController) {
		csmlog.WithContext(ctx).Info("controller mode, don't need to start apiRouter")
		return
	}
	s.startNodeToArrayConnectivityCheck(ctx)
	s.apiRouter(ctx)
}

// apiRouter serves http requests
func (s *service) apiRouter(_ context.Context) {
	csmlog.Infof("starting http server on port %s", apiPort)
	// create a new router
	router := mux.NewRouter()
	// route to connectivity status
	router.HandleFunc(nodeStatus, nodeHealth).Methods("GET")
	router.HandleFunc(arrayStatus, connectivityStatus).Methods("GET")
	router.HandleFunc(arrayStatus+"/"+"{arrayId}", getArrayConnectivityStatus).Methods("GET")
	router.Use(podmonAuthMiddleware)
	// start http server to serve requests
	server := &http.Server{
		Addr:         apiPort,
		Handler:      router,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	}
	err := server.ListenAndServe()
	if err != nil {
		csmlog.Errorf("unable to start http server to serve status requests due to %s", err)
	}
}

// getArrayConnectivityStatus lists status of the requested array
func getArrayConnectivityStatus(w http.ResponseWriter, r *http.Request) {
	arrayID := mux.Vars(r)["arrayId"]
	csmlog.Infof("GetArrayConnectivityStatus called for array %s \n", arrayID)
	status, found := probeStatus.Load(arrayID)
	if !found {
		// specify status code
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		// update response writer
		// #nosec G705 - XSS not a concern for internal API endpoint writing plain text
		fmt.Fprintf(w, "array %s not found \n", arrayID)
		return
	}
	// convert status struct to JSON
	jsonResponse, err := json.Marshal(status)
	if err != nil {
		csmlog.Errorf("error %s during marshaling to json", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		return
	}
	csmlog.Infof("sending response %+v for array %s \n", status, arrayID)
	// update response
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		csmlog.Errorf("unable to write response %s", err)
	}
}

// nodeHealth states if node is up
func nodeHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "node is up and running \n")
}

// connectivityStatus Returns array connectivity status
func connectivityStatus(w http.ResponseWriter, _ *http.Request) {
	csmlog.Infof("connectivityStatus called, urr status is %v \n", probeStatus)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// convert struct to JSON
	jsonResponse, err := MarshalSyncMapToJSON(probeStatus)
	if err != nil {
		csmlog.Errorf("error %s during marshaling to json", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		return
	}
	csmlog.Info("sending connectivityStatus for all clusters ")
	_, err = w.Write(jsonResponse)
	if err != nil {
		csmlog.Errorf("unable to write response %s", err)
	}
}

// startNodeToArrayConnectivityCheck starts connectivityTest as one goroutine for each cluster
func (s *service) startNodeToArrayConnectivityCheck(ctx context.Context) {
	csmlog.WithContext(ctx).Debug("startNodeToArrayConnectivityCheck called")
	probeStatus = new(sync.Map)
	isilonClusters := s.getIsilonClusters()
	for _, cluster := range isilonClusters {
		// start one goroutine for each cluster, so each cluster's nodeProbe is run concurrently
		go s.testConnectivityAndUpdateStatus(ctx, cluster, timeout)
	}
	csmlog.WithContext(ctx).Infof("startNodeToArrayConnectivityCheck is running probes at pollingFrequency %d ", pollingFrequencyInSeconds/2)
}

// testConnectivityAndUpdateStatus runs probe to test connectivity from node to array
// updates probeStatus map[array]ArrayConnectivityStatus
func (s *service) testConnectivityAndUpdateStatus(ctx context.Context, cluster *IsilonClusterConfig, timeout time.Duration) {
	defer func() {
		if err := recover(); err != nil {
			csmlog.WithContext(ctx).Errorf("panic occurred in testConnectivityAndUpdateStatus:%s for cluster %s", err, cluster)
		}
		// if panic occurs restart
		go s.testConnectivityAndUpdateStatus(ctx, cluster, timeout)
	}()
	var status ArrayConnectivityStatus
	for {
		select {
		case <-ctx.Done():
			csmlog.WithContext(ctx).Infof("connectivity monitor for cluster %s canceled", cluster.ClusterName)
			return
		default:
		}
		// add timeout to context
		timeOutCtx, cancel := context.WithTimeout(ctx, timeout)
		csmlog.WithContext(ctx).Debugf("Running probe for cluster %s at time %v \n", cluster.ClusterName, time.Now())
		if existingStatus, ok := probeStatus.Load(cluster.ClusterName); !ok {
			csmlog.WithContext(ctx).Debugf("%s not in probeStatus ", cluster.ClusterName)
		} else {
			if status, ok = existingStatus.(ArrayConnectivityStatus); !ok {
				csmlog.WithContext(ctx).Errorf("failed to extract ArrayConnectivityStatus for cluster '%s'", cluster.ClusterName)
			}
		}
		csmlog.WithContext(ctx).Debugf("cluster %s , status is %+v", cluster.ClusterName, status)
		// run nodeProbe to test connectivity
		err := s.nodeProbe(timeOutCtx, cluster)
		if err == nil {
			csmlog.WithContext(ctx).Debugf("Probe successful for %s", cluster.ClusterName)
			status.LastSuccess = time.Now().Unix()
		} else {
			csmlog.WithContext(ctx).Debugf("Probe failed for isilon cluster '%s' error:'%s'", cluster.ClusterName, err)
		}
		status.LastAttempt = time.Now().Unix()
		csmlog.WithContext(ctx).Debugf("cluster %s , storing status %+v", cluster.ClusterName, status)
		probeStatus.Store(cluster.ClusterName, status)
		cancel()
		// sleep for half the pollingFrequency and run check again
		pollingFrequencyLock.Lock()
		time.Sleep(time.Second * time.Duration(pollingFrequencyInSeconds/2))
		pollingFrequencyLock.Unlock()
	}
}
