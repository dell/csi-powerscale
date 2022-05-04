package service

import (
	"context"
	"fmt"
	"github.com/dell/csi-isilon/common/utils"
	podmon "github.com/dell/dell-csi-extensions/podmon"
)

func (s *service) ValidateVolumeHostConnectivity(ctx context.Context, req *podmon.ValidateVolumeHostConnectivityRequest) (*podmon.ValidateVolumeHostConnectivityResponse, error) {
	ctx, log, _ := GetRunIDLog(ctx)
	log.Infof("ValidateVolumeHostConnectivity called %+v", req)
	rep := &podmon.ValidateVolumeHostConnectivityResponse{
		Messages: make([]string, 0),
	}

	if (len(req.GetVolumeIds()) == 0 || len(req.GetVolumeIds()) == 0) && req.GetNodeId() == "" {
		// This is a nop call just testing the interface is present
		rep.Messages = append(rep.Messages, "ValidateVolumeHostConnectivity is implemented")
		return rep, nil
	}

	systemIDs := make(map[string]bool)
	systemID := req.GetArrayId()
	if systemID == "" {
		foundOne := s.getArrayIdsFromVolumes(ctx, systemIDs, req.GetVolumeIds())
		if !foundOne {
			systemID = s.defaultIsiClusterName
			systemIDs[systemID] = true
		}
	}

	if req.GetNodeId() == "" {
		return nil, fmt.Errorf("The NodeID is a required field")
	}

	clusterName := s.defaultIsiClusterName

	//Get cluster config
	isiConfig, err := s.getIsilonConfig(ctx, &clusterName)
	if err != nil {
		log.Error("Failed to get Isilon config with error ", err.Error())
		return nil, err
	}

	//set cluster context
	ctx, log = setClusterContext(ctx, clusterName)
	log.Debugf("Cluster Name: %v", clusterName)

	// Go through each of the systemIDs
	for systemID := range systemIDs {
		// First - check if the array is visible from the node
		var checkError error
		checkError = s.checkIfNodeIsConnected(ctx, systemID, req.GetNodeId(), rep)
		if checkError != nil {
			return rep, checkError
		}
	}

	clients, err := isiConfig.isiSvc.IsIOInProgress(ctx)

	if clients != nil {
		for _, c := range clients.ClientsList {
			if c.Protocol == "nfs3" || c.Protocol == "nfs4" {
				_, _, clientIP, _ := utils.ParseNodeID(ctx, req.GetNodeId())
				if clientIP == c.RemoteAddr {
					rep.IosInProgress = true
				}
			}
		}
	}
	log.Infof("ValidateVolumeHostConnectivity reply %+v", rep)
	return rep, nil
}

func (s *service) getArrayIdsFromVolumes(ctx context.Context, systemIDs map[string]bool, requestVolumeIds []string) bool {
	ctx, log, _ := GetRunIDLog(ctx)
	var err error
	var systemID string
	var foundAtLeastOne bool
	for _, volumeID := range requestVolumeIds {
		// Extract clusterName from the volume ID (if any volumes in the request)
		if _, _, _, systemID, err = utils.ParseNormalizedVolumeID(ctx, volumeID); err != nil {
			log.Warnf("Error getting Cluster Name for %s - %s", volumeID, err.Error())
		}
		if systemID != "" {
			if _, exists := systemIDs[systemID]; !exists {
				foundAtLeastOne = true
				systemIDs[systemID] = true
				log.Infof("Using systemID from %s, %s", volumeID, systemID)
			}
		} else {
			log.Infof("Could not extract systemID from %s", volumeID)
		}
	}
	return foundAtLeastOne
}

// checkIfNodeIsConnected looks at the 'nodeId' to determine if there is connectivity to the 'arrayId' array.
//The 'rep' object will be filled with the results of the check.
func (s *service) checkIfNodeIsConnected(ctx context.Context, arrayID string, nodeID string, rep *podmon.ValidateVolumeHostConnectivityResponse) error {
	ctx, log, _ := GetRunIDLog(ctx)
	log.Infof("Checking if array %s is connected to node %s", arrayID, nodeID)
	var message string
	rep.Connected = false

	_, _, nodeIP, err := utils.ParseNodeID(ctx, nodeID)
	if err != nil {
		log.Errorf("failed to parse node ID '%s'", nodeID)
		return fmt.Errorf("failed to parse node ID")
	}

	//form url to call array on node
	url := "http://" + nodeIP + apiPort + arrayStatus + "/" + arrayID
	connected, err := s.queryArrayStatus(ctx, url)
	if err != nil {
		message = fmt.Sprintf("connectivity unknown for array %s to node %s due to %s", arrayID, nodeID, err)
		log.Error(message)
		rep.Messages = append(rep.Messages, message)
		log.Errorf(err.Error())
	}

	if connected {
		rep.Connected = true
		message = fmt.Sprintf("array %s is connected to node %s", arrayID, nodeID)
	} else {
		message = fmt.Sprintf("array %s is not connected to node %s", arrayID, nodeID)
	}
	log.Info(message)
	rep.Messages = append(rep.Messages, message)
	return nil
}
