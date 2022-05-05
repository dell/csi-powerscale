package service

import (
	"context"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

//queryStatus make API call to the specified url to retrieve connection status
func (s *service) queryArrayStatus(ctx context.Context, url string) (bool, error) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("panic occurred in queryStatus:", err)
		}
	}()
	log.Infof("Calling API %s", url)

	req, err := http.NewRequest("GET", url, nil)
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
	log.Debugf("last connectivity was  %d sec back, tolerance is %d sec", timeDiff, tolerance)
	if timeDiff < tolerance {
		return true, nil
	}
	return false, nil
}
