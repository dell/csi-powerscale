package service

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"
)

// timeout for making http requests
const timeout = time.Second * 5

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

	req, err := http.NewRequestWithContext(timeOutCtx, "GET", url, nil)
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
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
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
	//responseObject has last success and last attempt timestamp in Unix format
	timeDiff := statusResponse.LastAttempt - statusResponse.LastSuccess
	tolerance := setPollingFrequency(ctx)
	currTime := time.Now().Unix()
	//checking if the status response is stale and connectivity test is still running
	//since nodeProbe is run at frequency tolerance/2, ideally below check should never be true
	if (currTime - statusResponse.LastAttempt) > tolerance*2 {
		log.Errorf("seems like connectivity test is not being run, current time is %d and last run was at %d", currTime, statusResponse.LastAttempt)
		//considering connectivity is broken
		return false, nil
	}
	log.Debugf("last connectivity was  %d sec back, tolerance is %d sec", timeDiff, tolerance)
	//give 2s leeway for tolerance check
	if timeDiff <= tolerance+2 {
		return true, nil
	}
	return false, nil
}
