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
	log.Infof("request is %+v", req)

	client := &http.Client{
		Transport: nil,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Jar:     nil,
		Timeout: 0,
	}
	resp, err := client.Do(req)
	log.Infof("response is %+v", resp)
	if err != nil {
		if resp != nil && resp.StatusCode == 302 {
			log.Errorf("got redirect %+v", resp.Header.Get("Location"))
		}
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
	log.Infof("API Response as struct %+v\n", statusResponse)
	//responseObject has last success and alst attempt timestamp in Unix format
	timeDiff := statusResponse.LastAttempt - statusResponse.LastSuccess
	tolerance := 2 * pollingFrequencyInSeconds
	log.Debugf("timeDiff %v, tolerance %v", timeDiff, tolerance)
	if timeDiff < tolerance {
		return true, nil
	}
	return false, nil
}
