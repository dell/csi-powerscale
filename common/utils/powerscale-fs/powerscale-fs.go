/*
Copyright (c) 2019-2025 Dell Inc, or its subsidiaries.

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
package powerscalefs

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	csmlog "github.com/dell/csmlog"
	isi "github.com/dell/gopowerscale"
)

// CSIQuotaIDPrefix is the CSI tag for quota id stored in the export's description field set by csi driver
const CSIQuotaIDPrefix = "CSI_QUOTA_ID:"

var (
	// QuotaIDPattern the regex pattern that identifies the quota id set in the export's description field set by csi driver
	QuotaIDPattern = regexp.MustCompile(fmt.Sprintf("^%s(.*)", CSIQuotaIDPrefix))

	// ExportConflictMessagePattern is the regex pattern that identifies the error message of export conflict
	ExportConflictMessagePattern = regexp.MustCompile(fmt.Sprintf("^Export rules (\\d+) and (\\d+) conflict on '(.+)'$"))
)

// GetQuotaIDWithCSITag formats a given quota id with the CSI tag, e.g. AABpAQEAAAAAAAAAAAAAQA0AAAAAAAAA -> CSI_QUOTA_ID:AABpAQEAAAAAAAAAAAAAQA0AAAAAAAAA
func GetQuotaIDWithCSITag(quotaID string) string {
	if quotaID == "" {
		return ""
	}

	return fmt.Sprintf("%s%s", CSIQuotaIDPrefix, quotaID)
}

// GetQuotaIDFromDescription extracts quota id from the description field of export
func GetQuotaIDFromDescription(ctx context.Context, export isi.Export) (string, error) {
	log := csmlog.GetLogger().WithContext(ctx)

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
