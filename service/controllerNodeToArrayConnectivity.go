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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
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
	defer func() {
		if err := recover(); err != nil {
			csmlog.WithContext(ctx).Infof("panic occurred in queryStatus: %v", err)
		}
	}()
	csmlog.WithContext(ctx).Infof("Calling API %s with timeout %v", url, timeout)
	timeOutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := GetHTTPNewRequestWithContext(timeOutCtx, "GET", url, nil)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to create request for API %s due to %s ", url, err.Error())
		return false, err
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	if PodmonAPIToken != "" {
		req.Header.Set("Authorization", "Bearer "+PodmonAPIToken)
	}
	csmlog.WithContext(ctx).Debugf("Making %s url request %+v", url, req)

	client := &http.Client{}
	// Validate URL scheme before making request to prevent SSRF
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return false, fmt.Errorf("unsupported URL scheme: %s", req.URL.Scheme)
	}
	resp, err := client.Do(req) // #nosec G704 - URL scheme validation implemented above
	csmlog.WithContext(ctx).Debugf("Received response %+v for url %s", resp, url)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to call API %s due to %s ", url, err.Error())
		return false, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			csmlog.WithContext(ctx).Infof("Error closing HTTP response: %s", err.Error())
		}
	}()
	bodyBytes, err := GetIoReadAll(resp.Body)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("failed to read API response due to %s ", err.Error())
		return false, err
	}
	var statusResponse ArrayConnectivityStatus
	err = json.Unmarshal(bodyBytes, &statusResponse)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("unable to unmarshal and determine connectivity due to %s ", err)
		return false, err
	}
	csmlog.WithContext(ctx).Infof("API Response received is %+v\n", statusResponse)
	// responseObject has last success and last attempt timestamp in Unix format
	timeDiff := statusResponse.LastAttempt - statusResponse.LastSuccess
	tolerance := getPollingFrequency(ctx)
	currTime := getTimeNow().Unix()
	// checking if the status response is stale and connectivity test is still running
	// since nodeProbe is run at frequency tolerance/2, ideally below check should never be true
	if (currTime - statusResponse.LastAttempt) > tolerance*2 {
		csmlog.WithContext(ctx).Errorf("seems like connectivity test is not being run, current time is %d and last run was at %d", currTime, statusResponse.LastAttempt)
		// considering connectivity is broken
		return false, nil
	}
	csmlog.WithContext(ctx).Debugf("last connectivity was %d sec back, tolerance is %d sec", timeDiff, tolerance)
	// give 2s leeway for tolerance check
	if timeDiff <= tolerance+2 {
		return true, nil
	}
	return false, nil
}
