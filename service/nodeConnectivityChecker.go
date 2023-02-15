package service

/*
 Copyright (c) 2022 Dell Inc, or its subsidiaries.

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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/dell/csi-isilon/v2/common/utils"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

const (
	nodeStatus  = "/node-status"
	arrayStatus = "/array-status"
)

// pollingFrequency in seconds
var pollingFrequencyInSeconds int64

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
	port := utils.ParseUintFromContext(ctx, constants.EnvPodmonAPIPORT)
	if port == 0 {
		// If the port number cannot be fetched, set it to default
		apiPort = ":" + constants.DefaultPodmonAPIPortNumber
		log.Debugf("set podmon API port to default %s", apiPort)
		return
	}
	apiPort = fmt.Sprintf(":%d", port)
	log.Debugf("set podmon API port to %s", apiPort)
}

// reads the pollingFrequency from Env, sets default if not found
func setPollingFrequency(ctx context.Context) int64 {
	pollRate, err := utils.ParseInt64FromContext(ctx, constants.EnvPodmonArrayConnectivityPollRate)
	if err != nil || pollRate == 0 {
		log.Debugf("use default pollingFrequency %d seconds, err %v", constants.DefaultPodmonPollRate, err)
		return constants.DefaultPodmonPollRate
	}
	log.Debugf("use pollingFrequency as %d seconds", pollRate)
	return pollRate
}

// MarshalSyncMapToJSON marshal the sync Map to Json
func MarshalSyncMapToJSON(m *sync.Map) ([]byte, error) {
	tmpMap := make(map[string]ArrayConnectivityStatus)
	m.Range(func(k, v interface{}) bool {
		tmpMap[k.(string)] = v.(ArrayConnectivityStatus)
		return true
	})
	log.Debugf("map value is %+v", tmpMap)
	return json.Marshal(tmpMap)
}

// startAPIService reads nodes to array status periodically
func (s *service) startAPIService(ctx context.Context) {
	isPodmonEnabled := utils.ParseBooleanFromContext(ctx, constants.EnvPodmonEnabled)
	if !isPodmonEnabled {
		log.Info("podmon is not enabled")
		return
	}

	pollingFrequencyInSeconds = setPollingFrequency(ctx)
	setAPIPort(ctx)

	//start methods based on mode
	if strings.EqualFold(s.mode, constants.ModeController) {
		log.Info("controller mode, don't need to start apiRouter")
		return
	}
	s.startNodeToArrayConnectivityCheck(ctx)
	s.apiRouter(ctx)
}

// apiRouter serves http requests
func (s *service) apiRouter(ctx context.Context) {
	log.Infof("starting http server on port %s", apiPort)
	//create a new router
	router := mux.NewRouter()
	//route to connectivity status
	router.HandleFunc(nodeStatus, nodeHealth).Methods("GET")
	router.HandleFunc(arrayStatus, connectivityStatus).Methods("GET")
	router.HandleFunc(arrayStatus+"/"+"{arrayId}", getArrayConnectivityStatus).Methods("GET")
	//start http server to serve requests
	server := &http.Server{
		Addr:         apiPort,
		Handler:      router,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	}
	err := server.ListenAndServe()
	if err != nil {
		log.Errorf("unable to start http server to serve status requests due to %s", err)
	}
}

// getArrayConnectivityStatus lists status of the requested array
func getArrayConnectivityStatus(w http.ResponseWriter, r *http.Request) {
	arrayID := mux.Vars(r)["arrayId"]
	log.Infof("GetArrayConnectivityStatus called for array %s \n", arrayID)
	status, found := probeStatus.Load(arrayID)
	if !found {
		//specify status code
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		//update response writer
		fmt.Fprintf(w, "array %s not found \n", arrayID)
		return
	}
	//convert status struct to JSON
	jsonResponse, err := json.Marshal(status)
	if err != nil {
		log.Errorf("error %s during marshaling to json", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		return
	}
	log.Infof("sending response %+v for array %s \n", status, arrayID)
	//update response
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(jsonResponse)
	if err != nil {
		log.Errorf("unable to write response %s", err)
	}
}

// nodeHealth states if node is up
func nodeHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "node is up and running \n")
}

// connectivityStatus Returns array connectivity status
func connectivityStatus(w http.ResponseWriter, r *http.Request) {
	log.Infof("connectivityStatus called, urr status is %v \n", probeStatus)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	//convert struct to JSON
	jsonResponse, err := MarshalSyncMapToJSON(probeStatus)
	if err != nil {
		log.Errorf("error %s during marshaling to json", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		return
	}
	log.Info("sending connectivityStatus for all clusters ")
	_, err = w.Write(jsonResponse)
	if err != nil {
		log.Errorf("unable to write response %s", err)
	}

}

// startNodeToArrayConnectivityCheck starts connectivityTest as one goroutine for each cluster
func (s *service) startNodeToArrayConnectivityCheck(ctx context.Context) {
	log.Debug("startNodeToArrayConnectivityCheck called")
	probeStatus = new(sync.Map)
	isilonClusters := s.getIsilonClusters()
	for _, cluster := range isilonClusters {
		//start one goroutine for each cluster, so each cluster's nodeProbe is run concurrently
		go s.testConnectivityAndUpdateStatus(ctx, cluster, timeout)
	}
	log.Infof("startNodeToArrayConnectivityCheck is running probes at pollingFrequency %d ", pollingFrequencyInSeconds/2)
}

// testConnectivityAndUpdateStatus runs probe to test connectivity from node to array
// updates probeStatus map[array]ArrayConnectivityStatus
func (s *service) testConnectivityAndUpdateStatus(ctx context.Context, cluster *IsilonClusterConfig, timeout time.Duration) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("panic occurred in testConnectivityAndUpdateStatus:%s for clsuter %s", err, cluster)
		}
		//if panic occurs restart
		go s.testConnectivityAndUpdateStatus(ctx, cluster, timeout)
	}()
	var status ArrayConnectivityStatus
	for {
		//add timeout to context
		timeOutCtx, cancel := context.WithTimeout(ctx, timeout)
		log.Debugf("Running probe for cluster %s at time %v \n", cluster.ClusterName, time.Now())
		if existingStatus, ok := probeStatus.Load(cluster.ClusterName); !ok {
			log.Debugf("%s not in probeStatus ", cluster.ClusterName)
		} else {
			if status, ok = existingStatus.(ArrayConnectivityStatus); !ok {
				log.Errorf("failed to extract ArrayConnectivityStatus for cluster '%s'", cluster.ClusterName)
			}
		}
		log.Debugf("cluster %s , status is %+v", cluster.ClusterName, status)
		//run nodeProbe to test connectivity
		err := s.nodeProbe(timeOutCtx, cluster)
		if err == nil {
			log.Debugf("Probe successful for %s", cluster.ClusterName)
			status.LastSuccess = time.Now().Unix()
		} else {
			log.Debugf("Probe failed for isilon cluster '%s' error:'%s'", cluster.ClusterName, err)
		}
		status.LastAttempt = time.Now().Unix()
		log.Debugf("cluster %s , storing status %+v", cluster.ClusterName, status)
		probeStatus.Store(cluster.ClusterName, status)
		cancel()
		//sleep for half the pollingFrequency and run check again
		time.Sleep(time.Second * time.Duration(pollingFrequencyInSeconds/2))
	}
}
