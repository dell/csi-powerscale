package service

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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	isiapi "github.com/dell/goisilon/api"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
)

var (
	debug bool

	stepHandlersErrors struct {
		FindVolumeIDError          bool
		GetVolByIDError            bool
		GetStoragePoolsError       bool
		GetStatisticsError         bool
		CreateSnapshotError        bool
		RemoveVolumeError          bool
		InstancesError             bool
		VolInstanceError           bool
		StatsError                 bool
		StartingTokenInvalidError  bool
		GetSnapshotError           bool
		DeleteSnapshotError        bool
		ExportNotFoundError        bool
		VolumeNotExistError        bool
		CreateQuotaError           bool
		UpdateQuotaError           bool
		CreateExportError          bool
		GetExportInternalError     bool
		GetExportByIDNotFoundError bool
		UnexportError              bool
		DeleteQuotaError           bool
		QuotaNotFoundError         bool
		DeleteVolumeError          bool
	}
)

// This file contains HTTP handlers for mocking to the Isilon OneFS REST API.
var isilonRouter http.Handler
var testControllerHasNoConnection bool
var testNodeHasNoConnection bool

// getFileHandler returns an http.Handler that
func getHandler() http.Handler {
	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.Printf("handler called: %s %s", r.Method, r.URL)
			if isilonRouter == nil {
				getRouter().ServeHTTP(w, r)
			}
		})

	debug = false

	return handler
}

func getRouter() http.Handler {
	isilonRouter := mux.NewRouter()
	isilonRouter.HandleFunc("/platform/latest/", handleNewAPI)
	isilonRouter.HandleFunc("/platform/2/protocols/nfs/exports/", handleExportUpdate).Methods("PUT")
	isilonRouter.HandleFunc("/platform/2/protocols/nfs/exports/{export_id}", handleModifyExport).Methods("PUT")
	isilonRouter.HandleFunc("/platform/2/protocols/nfs/exports/{export_id}", handleUnexportPath).Methods("DELETE").Queries("zone", "System")
	isilonRouter.HandleFunc("/platform/2/protocols/nfs/exports/{id}", handleGetExportByID).Methods("GET")
	isilonRouter.HandleFunc("/platform/2/protocols/nfs/exports/", handleCreateExport).Methods("POST")
	// Do NOT change the sequence of the following four lines, the first three are subsets of the fourth
	isilonRouter.HandleFunc("/platform/2/protocols/nfs/exports/", handleGetExportWithPathAndZone).Methods("GET").Queries("path", "/ifs/data/csi-isilon/volume1", "zone", "System")
	isilonRouter.HandleFunc("/platform/2/protocols/nfs/exports/", handleGetExportsWithLimit).Methods("GET").Queries("limit", "")
	isilonRouter.HandleFunc("/platform/2/protocols/nfs/exports/", handleGetExportsWithResume).Methods("GET").Queries("resume", "")
	isilonRouter.HandleFunc("/platform/2/protocols/nfs/exports/", handleGetExports).Methods("GET")
	isilonRouter.HandleFunc("/platform/3/statistics/current", handleStatistics)
	isilonRouter.HandleFunc("/platform/3/cluster/config/", handleGetClusterConfig)
	// Do NOT change the sequence of the following lines, the first is the subset of the second,
	// thus if the sequence is reversed, the query with "metadata" will be wrongly resolved.
	isilonRouter.HandleFunc("/namespace/ifs/data/csi-isilon/{volume_id}", handleGetVolumeSize).Methods("GET").Queries("detail", "size", "max-depth", "-1")
	isilonRouter.HandleFunc("/namespace/ifs/data/csi-isilon/volume1", handleGetExistentVolumeMetadata).Methods("GET").Queries("metadata", "")
	isilonRouter.HandleFunc("/namespace/ifs/data/csi-isilon/volume1", handleGetVolumeWithoutMetadata).Methods("GET")

	isilonRouter.HandleFunc("/namespace/ifs/data/csi-isilon/volume2", handleGetExistentVolume).Methods("GET")
	isilonRouter.HandleFunc("/namespace/ifs/data/csi-isilon/volume1", handleCopySnapshot).Methods("PUT").
		Headers("X-Isi-Ifs-Copy-Source", "/namespace/ifs/.snapshot/existent_snapshot_name/data/csi-isilon/nfs_1").Queries("merge", "True")
	isilonRouter.HandleFunc("/namespace/ifs/data/csi-isilon/volume1", handleCopyVolume).Methods("PUT").
		Headers("X-Isi-Ifs-Copy-Source", "/namespace/ifs/data/csi-isilon/volume2").Queries("merge", "True")
	isilonRouter.HandleFunc("/namespace/ifs/data/csi-isilon/volume1", handleVolumeCreation).Methods("PUT")
	isilonRouter.HandleFunc("/platform/5/quota/license/", handleGetQuotaLicense).Methods("GET")
	isilonRouter.HandleFunc("/platform/1/quota/quotas/{quota_id}", handleGetQuotaByID).Methods("GET")
	isilonRouter.HandleFunc("/platform/1/quota/quotas/", handleCreateQuota).Methods("POST")
	isilonRouter.HandleFunc("/platform/1/quota/quotas/{quota_id}", handleDeleteQuotaByID).Methods("DELETE")
	isilonRouter.HandleFunc("/platform/1/quota/quotas/{quota_id}", handleUpdateQuotaByID).Methods("PUT")

	isilonRouter.HandleFunc("/namespace/ifs/data/csi-isilon/{volume_id}", handleDeleteVolume).Methods("DELETE").Queries("recursive", "true")
	isilonRouter.HandleFunc("/namespace/ifs/data/csi-isilon/{id}", handleGetVolume).Methods("GET").Queries("metadata", "")
	isilonRouter.HandleFunc("/namespace/ifs/data/csi-isilon/{id}", handleGetVolumeWithoutMetadata).Methods("GET")
	isilonRouter.HandleFunc("/namespace/ifs/data/csi-isilon/{id}", handleVolumeCreation).Methods("PUT")

	isilonRouter.HandleFunc("/platform/1/snapshot/snapshots/", handleCreateSnapshot).Methods("POST")
	isilonRouter.HandleFunc("/platform/1/snapshot/snapshots/create_snapshot_name/", handleGetNonexistentSnapshot).Methods("GET")
	isilonRouter.HandleFunc("/platform/1/snapshot/snapshots/1/", handleGetNonexistentSnapshot).Methods("GET")
	isilonRouter.HandleFunc("/platform/1/snapshot/snapshots/existent_snapshot_name/", handleGetExistentSnapshot).Methods("GET")
	isilonRouter.HandleFunc("/platform/1/snapshot/snapshots/2/", handleGetExistentSnapshot).Methods("GET")
	isilonRouter.HandleFunc("/platform/1/snapshot/snapshots/existent_comp_snapshot_name/", handleGetExistentCompatibleSnapshot).Methods("GET")
	isilonRouter.HandleFunc("/platform/1/snapshot/snapshots/3/", handleGetExistentCompatibleSnapshot).Methods("GET")
	isilonRouter.HandleFunc("/platform/1/snapshot/snapshots/{snapshot_id}/", handleDeleteSnapshot).Methods("DELETE")
	isilonRouter.HandleFunc("/platform/1/snapshot/snapshots/{snapshot_id}/", handleGetSnapshotByID).Methods("GET")
	isilonRouter.HandleFunc("/namespace/ifs/.snapshot/{snapshot_name}/data/csi-isilon/{volume_id}", handleGetSnapshotSize).Methods("GET").Queries("detail", "size", "max-depth", "-1")
	//
	isilonRouter.HandleFunc("/namespace/ifs/data/csi-isilon/{id}", handleGetVolumeWithoutMetadata).Methods("GET").Queries("query", "", "limit", "2", "max-depth", "-1", "detail", "type,container_path,size,mode,owner,group,name")

	return isilonRouter
}

// handleNewApi implements GET /platform/latest
func handleNewAPI(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	w.Write([]byte("{\"latest\": \"5.1\"}"))
}

// handleExports implements GET /platform/2/protocols/nfs/exports
func handleGetExports(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.GetExportInternalError {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(readFromFile("mock/export/get_all_exports_including_volume2.txt"))
}

// handleCreateExport implements POST /platform/2/protocols/nfs/exports
func handleCreateExport(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.CreateExportError {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(readFromFile("mock/export/create_export_557.txt"))
}

// handleGetClusterConfig implements GET /platform/3/cluster/config/
func handleGetClusterConfig(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection || testNodeHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(readFromFile("mock/cluster/get_cluster_config.txt"))
}

// handleExportUpdate implements PUT /platform/2/protocols/nfs/exports
func handleExportUpdate(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{\"digest\": \"e86d27f991a5597e0b3fbd51269a2636\"}"))
}

func readFromFile(relativeFilePath string) []byte {
	var data []byte
	var err error
	if data, err = ioutil.ReadFile(relativeFilePath); err != nil {
		panic(fmt.Sprintf("failed to read mock file '%s'", relativeFilePath))
	}
	return data
}

// handleGetVolume implements GET /namespace/ifs/data/csi-isilon/volume1
func handleGetVolumeWithoutMetadata(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.VolumeNotExistError {
		w.WriteHeader(http.StatusNotFound)
		w.Write(readFromFile("mock/volume/get_non_existent_volume.txt"))
	}
	w.Write(readFromFile("mock/volume/get_volume2_without_metadata.txt"))
}

// handleGetExportByID implements GET /platform/2/protocols/nfs/exports/{id}
func handleGetExportByID(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.GetExportInternalError {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if stepHandlersErrors.GetExportByIDNotFoundError {
		w.WriteHeader(http.StatusNotFound)
		w.Write(readFromFile("mock/export/export_not_found_by_id.txt"))
		return
	}
	w.Write(readFromFile("mock/export/get_export_557.txt"))
}

// handleVolumeCreation implements PUT /namespace/volume1
func handleVolumeCreation(w http.ResponseWriter, r *http.Request) {
	if stepHandlersErrors.InstancesError {
		writeError(w, "Error retrieving Volume", http.StatusRequestTimeout, codes.Internal)
		return
	}
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	w.Write([]byte("{\"digest\": \"e86d27f991a5597e0b3fbd51269a2636\"}"))
}

// handleGetExistentVolumeMetadata implements GET /namespace/ifs/data/csi-isilon/volume1?metadata
func handleGetExistentVolumeMetadata(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	w.Write([]byte("{\"attrs\": [{}]}"))
}

// handleGetVolume implements GET /namespace/ifs/data/csi-isilon/volume1?metadata
func handleGetVolume(w http.ResponseWriter, r *http.Request) {
	if stepHandlersErrors.VolInstanceError {
		writeError(w, "Error retrieving Volume", http.StatusRequestTimeout, codes.Internal)
		return
	}
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	w.Write([]byte("{\"attrs\": [{}]}"))
}

// handleGetExistentVolume implements GET /namespace/ifs/data/csi-isilon/volume2?metadata
func handleGetExistentVolume(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(readFromFile("mock/volume/get_volume2_without_metadata.txt"))
}

// handleGetQuotaLicense implements GET /platform/5/quota/license
func handleGetQuotaLicense(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(readFromFile("mock/quota/get_quota_license.txt"))
}

// handleCreateQuota implements POST /platform/1/quota/quotas
func handleCreateQuota(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.CreateQuotaError {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	w.Write(readFromFile("mock/quota/create_quota.txt"))
}

// handleDeleteQuotaByID implements DELETE /platform/1/quota/quotas/AABpAQEAAAAAAAAAAAAAQA0AAAAAAAAA
func handleDeleteQuotaByID(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.DeleteQuotaError {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if stepHandlersErrors.QuotaNotFoundError {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(readFromFile("mock/quota/quota_not_found.txt"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
	// response body is empty
	w.Write([]byte(""))
}

// handleGetQuota implements GET /platform/1/quota/quotas/WACnAAEAAAAAAAAAAAAAQBUPAAAAAAAA
func handleGetQuotaByID(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.QuotaNotFoundError {
		w.WriteHeader(http.StatusNotFound)
		w.Write(readFromFile("mock/quota/quota_not_found.txt"))
		return
	}
	w.Write(readFromFile("mock/quota/get_quota_by_id.txt"))
}

// handleUpdateQuotaByID implements PUT /platform/1/quota/quotas/AABpAQEAAAAAAAAAAAAAQA0AAAAAAAAA
func handleUpdateQuotaByID(w http.ResponseWriter, r *http.Request) {
	if stepHandlersErrors.UpdateQuotaError {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	// response body is empty
	w.Write([]byte(""))
}

// handleUnexportPath implements DELETE /platform/2/protocols/nfs/exports/43?zone=System
func handleUnexportPath(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.UnexportError {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	// response body is empty
	w.Write([]byte(""))
}

// handleDeleteVolume implements DELETE /namespace/ifs/data/csi-isilon/volume2?recursive=true
func handleDeleteVolume(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.DeleteVolumeError {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	// response body is empty
	w.Write([]byte(""))
}

// handleModifyExport implements GET /platform/2/protocols/nfs/exports/{id}
func handleModifyExport(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	// response body is empty
	w.Write([]byte(""))
}

// handleStatistics implements GET /platform/3/statistics
func handleStatistics(w http.ResponseWriter, r *http.Request) {
	if stepHandlersErrors.InstancesError {
		writeError(w, "Error retrieving Statistics", http.StatusRequestTimeout, codes.Internal)
		return
	}
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	var str string
	if stepHandlersErrors.StatsError {
		str = "{ \"stats\": [{ \"devid\": 0, \"error\": \"Error\", \"error_code\": 1234,\"key\": \"ifs.bytes.avail\", " +
			"\"time\": 1565035610,\"value\": 81224996814848 }] }"
	} else {
		str = "{ \"stats\": [{ \"devid\": 0, \"error\": null, \"error_code\": null,\"key\": \"ifs.bytes.avail\", " +
			"\"time\": 1565035610,\"value\": 81224996814848 }] }"
	}
	w.Write([]byte(str))
}

// handleGetExportWithPathAndZone GET /platform/2/protocols/nfs/exports?path=/ifs/data/csi-isilon/volume1&zone=System
func handleGetExportWithPathAndZone(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.GetExportInternalError {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if stepHandlersErrors.ExportNotFoundError {
		w.Write(readFromFile("mock/export/get_export_not_found.txt"))
		return
	}
	w.Write(readFromFile("mock/export/get_export_557.txt"))
}

// handleExportGetId implements GET /platform/2/protocols/nfs/exports?limit=2
func handleGetExportsWithLimit(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.GetExportInternalError {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(readFromFile("mock/export/get_exports_with_limit.txt"))
}

// handleExportGetId implements GET /platform/2/protocols/nfs/exports?resume=1-1-MAAA1
func handleGetExportsWithResume(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.GetExportInternalError {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if stepHandlersErrors.StartingTokenInvalidError {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(readFromFile("mock/export/get_exports_with_invalid_resume.txt"))
		return
	}
	w.Write(readFromFile("mock/export/get_exports_with_resume.txt"))
}

// handleGetSnapshotByID implements GET /platform/1/snapshot/snapshots/{snapshot_id}
// This function regards snapshot id 404 as an unexisted snapshot id
func handleGetSnapshotByID(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.GetSnapshotError == true {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if strings.Contains(r.URL.String(), "/404") {
		w.WriteHeader(http.StatusNotFound)
		w.Write(readFromFile("mock/snapshot/get_non_existent_snapshot.txt"))
	}
	w.Write(readFromFile("mock/snapshot/get_existent_snapshot.txt"))
}

// handleDeleteVolume implements DELETE /platform/1/snapshot/snapshots/{snapshot_id}
// This function regards snapshot id 404 as an unexisted snapshot id
func handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.DeleteSnapshotError == true {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if strings.Contains(r.URL.String(), "/404") {
		w.WriteHeader(http.StatusNotFound)
		w.Write(readFromFile("mock/snapshot/get_non_existent_snapshot.txt"))
	}
	w.WriteHeader(http.StatusNoContent)
	// response body is empty
	w.Write([]byte(""))
}

// Write an error code to the response writer
func writeError(w http.ResponseWriter, message string, httpStatus int, errorCode codes.Code) {
	w.WriteHeader(httpStatus)
	resp := isiapi.JSONError{StatusCode: 200, Err: []isiapi.Error{{Code: "", Field: "", Message: ""}}}
	resp.Err[0].Message = message
	resp.StatusCode = httpStatus
	encoder := json.NewEncoder(w)
	err := encoder.Encode(resp)
	if err != nil {
		log.Printf("error encoding json: %s\n", err.Error())
	}
}

// handleCreateSnapshot implements POST /platform/1/snapshot/snapshots/
func handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	if stepHandlersErrors.CreateSnapshotError {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(readFromFile("mock/snapshot/create_snapshot.txt"))
}

// handleGetNonexistentSnapshot implements GET /platform/1/snapshot/snapshots/create_snapshot_name/
func handleGetNonexistentSnapshot(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	writeError(w, "Error retrieving Snapshot", http.StatusNotFound, codes.NotFound)
	w.Write(readFromFile("mock/snapshot/get_non_existent_snapshot.txt"))
}

// handleGetExistentSnapshot implements GET /platform/1/snapshot/snapshots/existent_snapshot_name/
func handleGetExistentSnapshot(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	w.Write(readFromFile("mock/snapshot/get_existent_snapshot.txt"))
}

// handleGetExistentCompatibleSnapshot implements GET /platform/1/snapshot/snapshots/existent_comp_snapshot_name/
func handleGetExistentCompatibleSnapshot(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	w.Write(readFromFile("mock/snapshot/get_existent_compatible_snapshot.txt"))
}

// handleCopySnapshot implements PUT /namespace/ifs/data/csi-isilon/volume1?merge=True
// X-Isi-Ifs-Copy-Source: /namespace/ifs/.snapshot/existent_snapshot_name/data/csi-isilon/nfs_1
func handleCopySnapshot(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	//w.Write(readFromFile("mock/create_snapshot.txt"))
}

// handleCopyVolume implements PUT /namespace/ifs/data/csi-isilon/volume1?merge=True
// X-Isi-Ifs-Copy-Source: /namespace/ifs/data/csi-isilon/volume2
func handleCopyVolume(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	//w.Write(readFromFile("mock/create_snapshot.txt"))
}

// handleGetSnapshotSize implements GET /namespace/ifs/.snapshot/{snapshot_name}/data/csi-isilon/{volume_id}?detail=size&max-depth=-1
func handleGetSnapshotSize(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}

	w.Write(readFromFile("mock/snapshot/get_snapshot_size.txt"))
}

// handleGetVolumeSize implements GET /namespace/ifs/data/csi-isilon/{volume_id}?detail=size&max-depth=-1
func handleGetVolumeSize(w http.ResponseWriter, r *http.Request) {
	if testControllerHasNoConnection {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}

	w.Write(readFromFile("mock/volume/get_volume_size.txt"))
}
