package utils

/*
 Copyright (c) 2019 Dell Inc, or its subsidiaries.

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
	gournal "github.com/akutz/gournal"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/csi-isilon/common/constants"
	isi "github.com/dell/goisilon"
	"github.com/google/uuid"
	csictx "github.com/rexray/gocsi/context"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ConfigureLogger sets log level for the logger
func ConfigureLogger(debugEnabled bool) context.Context {

	clientCtx := context.Background()

	if debugEnabled {

		//logrus
		log.SetLevel(log.DebugLevel)

		//gournal
		clientCtx = context.WithValue(
			clientCtx,
			gournal.LevelKey(),
			gournal.DebugLevel)

		gournal.DefaultLevel = gournal.DebugLevel

		log.Infof("log level for logrus and gournal configured to DEBUG")

		return clientCtx

	}

	// logrus
	log.SetLevel(log.InfoLevel)

	// gournal
	clientCtx = context.WithValue(
		clientCtx,
		gournal.LevelKey(),
		gournal.InfoLevel)

	gournal.DefaultLevel = gournal.InfoLevel

	log.Infof("log level for logrus and gournal configured to INFO")

	return clientCtx
}

// ParseBooleanFromContext parses an environment variable into a boolean value. If an error is encountered, default is set to false, and error is logged
func ParseBooleanFromContext(ctx context.Context, key string) bool {
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

// ParseUintFromContext parses an environment variable into a uint value. If an error is encountered, default is set to 0, and error is logged
func ParseUintFromContext(ctx context.Context, key string) uint {
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

// RemoveExistingCSISockFile When the sock file that the gRPC server is going to be listening on already exists, error will be thrown saying the address is already in use, thus remove it first
func RemoveExistingCSISockFile() error {

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

	id, err := uuid.NewUUID()
	if err != nil {
		log.Errorf("error generating UUID : '%s'", err)
		return "", err
	}

	return id.String(), nil
}

// LogMap logs the key-value entries of a given map
func LogMap(mapName string, m map[string]string) {

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

// VolumeIDPattern is the regex pattern that identifies the quota id set in the export's description field set by csi driver
var VolumeIDPattern = regexp.MustCompile(fmt.Sprintf("^(.+)%s(\\d+)%s(.+)$", VolumeIDSeparator, VolumeIDSeparator))

// NodeIDSeparator is the separator that separates node name and IP Address
var NodeIDSeparator = "=#=#="

// NodeIDPattern is the regex pattern that identifies the NodeID
var NodeIDPattern = regexp.MustCompile(fmt.Sprintf("^(.+)%s(.+)%s(.+)$", NodeIDSeparator, NodeIDSeparator))

// ExportConflictMessagePattern is the regex pattern that identifies the error message of export conflict
var ExportConflictMessagePattern = regexp.MustCompile(fmt.Sprintf("^Export rules (\\d+) and (\\d+) conflict on '(.+)'$"))

// GetQuotaIDWithCSITag formats a given quota id with the CSI tag, e.g. AABpAQEAAAAAAAAAAAAAQA0AAAAAAAAA -> CSI_QUOTA_ID:AABpAQEAAAAAAAAAAAAAQA0AAAAAAAAA
func GetQuotaIDWithCSITag(quotaID string) string {

	if quotaID == "" {
		return ""
	}

	return fmt.Sprintf("%s%s", CSIQuotaIDPrefix, quotaID)
}

// GetQuotaIDFromDescription extracts quota id from the description field of export
func GetQuotaIDFromDescription(export isi.Export) (string, error) {

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
func GetFQDNByIP(ip string) (string, error) {
	names, err := net.LookupAddr(ip)
	if err != nil {
		log.Errorf("error getting FQDN: '%s'", err)
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

// GetNormalizedVolumeID combines volume name (i.e. the directory name), export ID and access zone to form the normalized volume ID
// e.g. k8s-e89c9d089e + 19 + csi0zone => k8s-e89c9d089e=_=_=19=_=_=csi0zone
func GetNormalizedVolumeID(volName string, exportID int, accessZone string) string {

	volID := fmt.Sprintf("%s%s%s%s%s", volName, VolumeIDSeparator, strconv.Itoa(exportID), VolumeIDSeparator, accessZone)

	log.Debugf("combined volume name '%s' with export ID '%d' and access zone '%s' to form volume ID '%s'",
		volName, exportID, accessZone, volID)

	return volID
}

// ParseNormalizedVolumeID parses the volume ID (following the pattern '^(.+)=_=_=(d+)=_=_=(.+)$') to extract the volume name, export ID and access zone that make up the volume ID
// e.g. k8s-e89c9d089e=_=_=19=_=_=csi0zone => k8s-e89c9d089e, 19, csi0zone
func ParseNormalizedVolumeID(volID string) (string, int, string, error) {

	matches := VolumeIDPattern.FindStringSubmatch(volID)

	if len(matches) < 4 {
		return "", 0, "", fmt.Errorf("volume ID '%s' cannot match the expected '^(.+)=_=_=(d+)=_=_=(.+)$' pattern", volID)
	}

	exportID, err := strconv.Atoi(matches[2])
	if err != nil {
		return "", 0, "", err
	}

	log.Debugf("volume ID '%s' parsed into volume name '%s', export ID '%d' and access zone '%s'",
		volID, matches[1], exportID, matches[3])

	return matches[1], exportID, matches[3], nil
}

//ParseNodeID parses NodeID to node name, node FQDN and IP address using pattern '^(.+)=#=#=(.+)=#=#=(.+)'
func ParseNodeID(nodeID string) (string, string, string, error) {
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
