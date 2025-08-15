package service

/*
 Copyright (c) 2022-2025 Dell Inc, or its subsidiaries.

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
	"io"
	"net/http"
	"time"
)

// timeout for making http requests
var timeout = time.Second * 5

var (
	GetHTTPNewRequestWithContext = http.NewRequestWithContext
	GetIoReadAll                 = io.ReadAll
	getTimeNow                   = time.Now
	getPollingFrequency          = func(ctx context.Context) int64 {
		return setPollingFrequency(ctx)
	}
)

// queryStatus make API call to the specified url to retrieve connection status
func (s *service) queryArrayStatus(ctx context.Context, url string) (bool, error) {
	ctx, log, _ := GetRunIDLog(ctx)
	defer func() {
		if err := recover(); err != nil {
			log.Println("panic occurred in queryStatus:", err)
		}
	}()
	log.Infof("Calling API %s with timeout %v", url, timeout)
	timeOutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := GetHTTPNewRequestWithContext(timeOutCtx, "GET", url, nil)
	if err != nil {
		log.Errorf("failed to create request for API %s due to %s ", url, err.Error())
		return false, err
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	log.Debugf("Making %s url request %+v", url, req)

	client := &http.Client{}
	resp, err := client.Do(req)
	log.Debugf("Received response %+v for url %s", resp, url)
	if err != nil {
		log.Errorf("failed to call API %s due to %s ", url, err.Error())
		return false, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing HTTP response: %s", err.Error())
		}
	}()
	bodyBytes, err := GetIoReadAll(resp.Body)
	if err != nil {
		log.Errorf("failed to read API response due to %s ", err.Error())
		return false, err
	}
	var statusResponse ArrayConnectivityStatus
	err = json.Unmarshal(bodyBytes, &statusResponse)
	if err != nil {
		log.Errorf("unable to unmarshal and determine connectivity due to %s ", err)
		return false, err
	}
	log.Infof("API Response received is %+v\n", statusResponse)
	// responseObject has last success and last attempt timestamp in Unix format
	timeDiff := statusResponse.LastAttempt - statusResponse.LastSuccess
	tolerance := getPollingFrequency(ctx)
	currTime := getTimeNow().Unix()
	// checking if the status response is stale and connectivity test is still running
	// since nodeProbe is run at frequency tolerance/2, ideally below check should never be true
	if (currTime - statusResponse.LastAttempt) > tolerance*2 {
		log.Errorf("seems like connectivity test is not being run, current time is %d and last run was at %d", currTime, statusResponse.LastAttempt)
		// considering connectivity is broken
		return false, nil
	}
	log.Debugf("last connectivity was %d sec back, tolerance is %d sec", timeDiff, tolerance)
	// give 2s leeway for tolerance check
	if timeDiff <= tolerance+2 {
		return true, nil
	}
	return false, nil
}
