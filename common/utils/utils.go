package utils

/*
 Copyright (c) 2019-2022 Dell Inc, or its subsidiaries.

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
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/Showmax/go-fqdn"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/csi-isilon/v2/common/constants"
	csictx "github.com/dell/gocsi/context"
	isi "github.com/dell/goisilon"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
)

// ParseBooleanFromContext parses an environment variable into a boolean value. If an error is encountered, default is set to false, and error is logged
func ParseBooleanFromContext(ctx context.Context, key string) bool {
	log := GetRunIDLogger(ctx)
	if val, ok := csictx.LookupEnv(ctx, key); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			log.WithField(key, val).Debugf(
				"invalid boolean value for '%s', defaulting to false", key)
			return false
		}
		return b
	}
	return false
}

// ParseArrayFromContext parses an environment variable into an array of string
func ParseArrayFromContext(ctx context.Context, key string) ([]string, error) {
	var values []string

	if val, ok := csictx.LookupEnv(ctx, key); ok {
		err := yaml.Unmarshal([]byte(val), &values)
		if err != nil {
			return values, fmt.Errorf("invalid array value for '%s'", key)
		}
	}
	return values, nil
}

// ParseUintFromContext parses an environment variable into a uint value. If an error is encountered, default is set to 0, and error is logged
func ParseUintFromContext(ctx context.Context, key string) uint {
	log := GetRunIDLogger(ctx)
	if val, ok := csictx.LookupEnv(ctx, key); ok {
		i, err := strconv.ParseUint(val, 10, 0)
		if err != nil {
			log.WithField(key, val).Debugf(
				"invalid int value for '%s', defaulting to 0", key)
			return 0
		}
		return uint(i)
	}
	return 0
}

// ParseInt64FromContext parses an environment variable into an int64 value.
func ParseInt64FromContext(ctx context.Context, key string) (int64, error) {
	if val, ok := csictx.LookupEnv(ctx, key); ok {
		i, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid int64 value '%v' specified for '%s'", val, key)
		}
		return i, nil
	}
	return 0, nil
}

// RemoveExistingCSISockFile When the sock file that the gRPC server is going to be listening on already exists, error will be thrown saying the address is already in use, thus remove it first
var RemoveExistingCSISockFile = func() error {
	log := GetLogger()
	protoAddr := os.Getenv(constants.EnvCSIEndpoint)

	log.Debugf("check if sock file '%s' has already been created", protoAddr)

	if protoAddr == "" {
		return nil
	}

	if _, err := os.Stat(protoAddr); !os.IsNotExist(err) {

		log.Debugf("sock file '%s' already exists, remove it", protoAddr)

		if err := os.RemoveAll(protoAddr); err != nil {

			log.WithError(err).Debugf("error removing sock file '%s'", protoAddr)

			return fmt.Errorf(
				"failed to remove sock file: '%s', error '%v'", protoAddr, err)
		}

		log.Debugf("sock file '%s' removed", protoAddr)

	} else {
		log.Debugf("sock file '%s' does not exist yet, move along", protoAddr)
	}

	return nil
}

// GetNewUUID generates a UUID
func GetNewUUID() (string, error) {
	log := GetLogger()
	id, err := uuid.NewUUID()
	if err != nil {
		log.Errorf("error generating UUID : '%s'", err)
		return "", err
	}

	return id.String(), nil
}

// LogMap logs the key-value entries of a given map
func LogMap(ctx context.Context, mapName string, m map[string]string) {
	log := GetRunIDLogger(ctx)
	log.Debugf("map '%s':", mapName)
	for key, value := range m {
		log.Debugf("    [%s]='%s'", key, value)
	}
}

// RemoveSurroundingQuotes removes the surrounding double quotes of a given string (if there are no surrounding quotes, do nothing)
func RemoveSurroundingQuotes(s string) string {
	if len(s) > 0 && s[0] == '"' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '"' {
		s = s[:len(s)-1]
	}

	return s
}

// CombineTwoStrings combines two string variables isolated by defined sign
func CombineTwoStrings(s1 string, s2 string, sign string) string {
	s := s1 + sign + s2
	return s
}

// CSIQuotaIDPrefix is the CSI tag for quota id stored in the export's description field set by csi driver
var CSIQuotaIDPrefix = "CSI_QUOTA_ID:"

// QuotaIDPattern the regex pattern that identifies the quota id set in the export's description field set by csi driver
var QuotaIDPattern = regexp.MustCompile(fmt.Sprintf("^%s(.*)", CSIQuotaIDPrefix))

// VolumeIDSeparator is the separator that separates volume name and export ID (two components that a normalized volume ID is comprised of)
var VolumeIDSeparator = "=_=_="

// SnapshotIDSeparator is the separator that separates snapshot id and cluster name (two components that a normalized snapshot ID is comprised of)
var SnapshotIDSeparator = "=_=_="

// VolumeIDPattern is the regex pattern that identifies the quota id set in the export's description field set by csi driver
var VolumeIDPattern = regexp.MustCompile(fmt.Sprintf("^(.+)%s(\\d+)%s(.+)$", VolumeIDSeparator, VolumeIDSeparator))

// NodeIDSeparator is the separator that separates node name and IP Address
var NodeIDSeparator = "=#=#="

// NodeIDPattern is the regex pattern that identifies the NodeID
var NodeIDPattern = regexp.MustCompile(fmt.Sprintf("^(.+)%s(.+)%s(.+)$", NodeIDSeparator, NodeIDSeparator))

// ExportConflictMessagePattern is the regex pattern that identifies the error message of export conflict
var ExportConflictMessagePattern = regexp.MustCompile(fmt.Sprintf("^Export rules (\\d+) and (\\d+) conflict on '(.+)'$"))

// DummyHostNodeID is nodeID used for adding dummy client in client field of export
var DummyHostNodeID = "localhost=#=#=localhost=#=#=127.0.0.1"

// GetQuotaIDWithCSITag formats a given quota id with the CSI tag, e.g. AABpAQEAAAAAAAAAAAAAQA0AAAAAAAAA -> CSI_QUOTA_ID:AABpAQEAAAAAAAAAAAAAQA0AAAAAAAAA
func GetQuotaIDWithCSITag(quotaID string) string {
	if quotaID == "" {
		return ""
	}

	return fmt.Sprintf("%s%s", CSIQuotaIDPrefix, quotaID)
}

// GetQuotaIDFromDescription extracts quota id from the description field of export
func GetQuotaIDFromDescription(ctx context.Context, export isi.Export) (string, error) {
	log := GetRunIDLogger(ctx)

	log.Debugf("try to extract quota id from the description field of export (id:'%d', path: '%s', description : '%s')", export.ID, export.Paths, export.Description)

	if export.Description == "" {
		log.Debugf("description field is empty, this could be normal, the backing directory might not have a quota set on it, return normally")
		return "", nil
	}

	matches := QuotaIDPattern.FindStringSubmatch(export.Description)

	if len(matches) < 2 {
		log.Debugf("description field does not match the expected CSI_QUOTA_ID:(.*) pattern, this could be normal, the backing directory might not have a quota set on it and the description is a user-set text irrelevant to the export id, return normally")
		return "", nil
	}

	quotaID := matches[1]

	log.Debugf("quotaID extracted : '%s'", quotaID)

	return quotaID, nil
}

// GetFQDNByIP returns the FQDN based on the parsed ip address
func GetFQDNByIP(ctx context.Context, ip string) (string, error) {
	log := GetRunIDLogger(ctx)
	names, err := net.LookupAddr(ip)
	if err != nil {
		log.Debugf("error getting FQDN: '%s'", err)
		return "", err
	}
	// The first one is FQDN
	FQDN := strings.TrimSuffix(names[0], ".")
	return FQDN, nil
}

// GetOwnFQDN returns the FQDN of the node or controller itself
func GetOwnFQDN() (string, error) {
	nodeFQDN := fqdn.Get()
	if nodeFQDN == "unknown" {
		return "", errors.New("cannot get FQDN")
	}
	return nodeFQDN, nil
}

// IsStringInSlice checks if a string is an element of a string slice
func IsStringInSlice(str string, list []string) bool {
	for _, b := range list {
		if b == str {
			return true
		}
	}

	return false
}

// IsStringInSlices checks if a string is an element of a any of the string slices
func IsStringInSlices(str string, list ...[]string) bool {
	for _, strs := range list {
		if IsStringInSlice(str, strs) {
			return true
		}
	}

	return false
}

// RemoveStringFromSlice returns a slice that is a copy of the input "list" slice with the input "str" string removed
func RemoveStringFromSlice(str string, list []string) []string {
	result := make([]string, 0)

	for _, v := range list {
		if str != v {
			result = append(result, v)
		}
	}

	return result
}

// RemoveStringsFromSlice generates a slice that is a copy of the input "list" slice with elements from the input "strs" slice removed
func RemoveStringsFromSlice(filters []string, list []string) []string {
	result := make([]string, 0)

	for _, str := range list {
		if !IsStringInSlice(str, filters) {
			result = append(result, str)
		}
	}

	return result
}

// GetAccessMode extracts the access mode from the given *csi.ControllerPublishVolumeRequest instance
func GetAccessMode(req *csi.ControllerPublishVolumeRequest) (*csi.VolumeCapability_AccessMode_Mode, error) {
	vc := req.GetVolumeCapability()
	if vc == nil {
		return nil, status.Error(codes.InvalidArgument,
			"volume capability is required")
	}

	am := vc.GetAccessMode()
	if am == nil {
		return nil, status.Error(codes.InvalidArgument,
			"access mode is required")
	}

	if am.Mode == csi.VolumeCapability_AccessMode_UNKNOWN {
		return nil, status.Error(codes.InvalidArgument,
			"unknown access mode")
	}

	return &(am.Mode), nil
}

// GetNormalizedVolumeID combines volume name (i.e. the directory name), export ID, access zone and clusterName to form the normalized volume ID
// e.g. k8s-e89c9d089e + 19 + csi0zone + cluster1 => k8s-e89c9d089e=_=_=19=_=_=csi0zone=_=_=cluster1
func GetNormalizedVolumeID(ctx context.Context, volName string, exportID int, accessZone, clusterName string) string {
	log := GetRunIDLogger(ctx)

	volID := fmt.Sprintf("%s%s%s%s%s%s%s", volName, VolumeIDSeparator, strconv.Itoa(exportID), VolumeIDSeparator, accessZone, VolumeIDSeparator, clusterName)

	log.Debugf("combined volume name '%s' with export ID '%d', access zone '%s' and cluster name '%s' to form volume ID '%s'",
		volName, exportID, accessZone, clusterName, volID)

	return volID
}

// ParseNormalizedVolumeID parses the volume ID(using VolumeIDSeparator) to extract the volume name, export ID, access zone and cluster name(optional) that make up the volume ID
// e.g. k8s-e89c9d089e=_=_=19=_=_=csi0zone => k8s-e89c9d089e, 19, csi0zone, ""
// e.g. k8s-e89c9d089e=_=_=19=_=_=csi0zone=_=_=cluster1 => k8s-e89c9d089e, 19, csi0zone, cluster1
func ParseNormalizedVolumeID(ctx context.Context, volID string) (string, int, string, string, error) {
	log := GetRunIDLogger(ctx)
	tokens := strings.Split(volID, VolumeIDSeparator)
	if len(tokens) < 3 {
		return "", 0, "", "", fmt.Errorf("volume ID '%s' cannot be split into tokens", volID)
	}

	volumeName := tokens[0]

	exportID, err := strconv.Atoi(tokens[1])
	if err != nil {
		return "", 0, "", "", err
	}

	accessZone := tokens[2]

	var clusterName string
	if len(tokens) > 3 {
		clusterName = tokens[3]
	}

	log.Debugf("volume ID '%s' parsed into volume name '%s', export ID '%d', access zone '%s' and cluster name '%s'",
		volID, volumeName, exportID, accessZone, clusterName)

	return volumeName, exportID, accessZone, clusterName, nil
}

// GetNormalizedSnapshotID combines snapshotID ID and cluster name and access zone to form the normalized snapshot ID
// e.g. 12345 + cluster1 + accessZone => 12345=_=_=cluster1=_=_=zone1
func GetNormalizedSnapshotID(ctx context.Context, snapshotID, clusterName, accessZone string) string {
	log := GetRunIDLogger(ctx)

	snapID := fmt.Sprintf("%s%s%s%s%s", snapshotID, SnapshotIDSeparator, clusterName, SnapshotIDSeparator, accessZone)

	log.Debugf("combined snapshot id '%s' access zone '%s' and cluster name '%s' to form normalized snapshot ID '%s'",
		snapshotID, accessZone, clusterName, snapID)

	return snapID
}

// ParseNormalizedSnapshotID parses the normalized snapshot ID(using SnapshotIDSeparator) to extract the snapshot ID and cluster name(optional) that make up the normalized snapshot ID
// e.g. 12345 => 12345, ""
// e.g. 12345=_=_=cluster1=_=_=zone => 12345, cluster1, zone
func ParseNormalizedSnapshotID(ctx context.Context, snapID string) (string, string, string, error) {
	log := GetRunIDLogger(ctx)
	tokens := strings.Split(snapID, SnapshotIDSeparator)
	if len(tokens) < 1 {
		return "", "", "", fmt.Errorf("snapshot ID '%s' cannot be split into tokens", snapID)
	}

	snapshotID := tokens[0]
	var clusterName, accessZone string
	if len(tokens) > 1 {
		clusterName = tokens[1]
		if len(tokens) > 2 {
			accessZone = tokens[2]
		} else {
			return "", "", "", fmt.Errorf("access zone not found in snapshot ID '%s'", snapID)
		}
	}

	log.Debugf("normalized snapshot ID '%s' parsed into snapshot ID '%s' and cluster name '%s'",
		snapID, snapshotID, clusterName)

	return snapshotID, clusterName, accessZone, nil
}

// ParseNodeID parses NodeID to node name, node FQDN and IP address using pattern '^(.+)=#=#=(.+)=#=#=(.+)'
func ParseNodeID(ctx context.Context, nodeID string) (string, string, string, error) {
	log := GetRunIDLogger(ctx)

	matches := NodeIDPattern.FindStringSubmatch(nodeID)

	if len(matches) < 4 {
		return "", "", "", fmt.Errorf("node ID '%s' cannot match the expected '^(.+)=#=#=(.+)=#=#=(.+)$' pattern", nodeID)
	}

	log.Debugf("Node ID '%s' parsed into node name '%s', node FQDN '%s' and IP address '%s'",
		nodeID, matches[1], matches[2], matches[3])

	return matches[1], matches[2], matches[3], nil
}

// GetPathForVolume gets the volume full path by the combination of isiPath and volumeName
func GetPathForVolume(isiPath, volName string) string {
	if isiPath == "" {
		return path.Join("/ifs/", volName)
	}
	if isiPath[len(isiPath)-1:] == "/" {
		return path.Join(isiPath, volName)
	}
	return path.Join(isiPath, "/", volName)
}

// GetIsiPathFromExportPath returns isiPath based on the export path
func GetIsiPathFromExportPath(exportPath string) string {
	if strings.LastIndex(exportPath, "/") == (len(exportPath) - 1) {
		exportPath = fmt.Sprint(exportPath[0:strings.LastIndex(exportPath, "/")])
	}
	return fmt.Sprint(exportPath[0 : strings.LastIndex(exportPath, "/")+1])
}

// GetIsiPathFromPgID returns isiPath based on the pg id
func GetIsiPathFromPgID(exportPath string) string {
	s := strings.Split(exportPath, "::")
	if len(s) != 2 {
		return ""
	}
	return s[1]
}

// GetExportIDFromConflictMessage returns the export id of the export
// which is creating or just created when there occurs a conflict
func GetExportIDFromConflictMessage(message string) int {
	matches := ExportConflictMessagePattern.FindStringSubmatch(message)
	exportID, _ := strconv.Atoi(matches[1])
	return exportID
}

// GetVolumeNameFromExportPath returns volume name based on the export path
func GetVolumeNameFromExportPath(exportPath string) string {
	if strings.LastIndex(exportPath, "/") == (len(exportPath) - 1) {
		exportPath = fmt.Sprint(exportPath[0:strings.LastIndex(exportPath, "/")])
	}
	return fmt.Sprint(exportPath[strings.LastIndex(exportPath, "/")+1:])
}

// GetMessageWithRunID returns message with runID information
func GetMessageWithRunID(runid string, format string, args ...interface{}) string {
	str := fmt.Sprintf(format, args...)
	return fmt.Sprintf(" runid=%s %s", runid, str)
}
