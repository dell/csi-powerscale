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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"google.golang.org/grpc/metadata"
	"io/ioutil"
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dell/csi-isilon/common/k8sutils"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/csi-isilon/common/constants"
	"github.com/dell/csi-isilon/common/utils"
	"github.com/dell/csi-isilon/core"
	"github.com/dell/gocsi"
	csictx "github.com/dell/gocsi/context"
	isi "github.com/dell/goisilon"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//To maintain runid for Non debug mode. Note: CSI will not generate runid if CSI_DEBUG=false
var runid int64
var isilonConfigFile string

// Manifest is the SP's manifest.
var Manifest = map[string]string{
	"url":    "http://github.com/dell/csi-isilon",
	"semver": core.SemVer,
	"commit": core.CommitSha32,
	"formed": core.CommitTime.Format(time.RFC1123),
}

// Service is the CSI service provider.
type Service interface {
	csi.ControllerServer
	csi.IdentityServer
	csi.NodeServer
	BeforeServe(context.Context, *gocsi.StoragePlugin, net.Listener) error
}

// Opts defines service configuration options.
type Opts struct {
	Port                  string
	AccessZone            string
	Path                  string
	Insecure              bool
	AutoProbe             bool
	QuotaEnabled          bool
	DebugEnabled          bool
	Verbose               uint
	NfsV3                 bool
	CustomTopologyEnabled bool
	KubeConfigPath        string
	allowedNetworks       []string
}

type service struct {
	opts                  Opts
	mode                  string
	nodeID                string
	nodeIP                string
	statisticsCounter     int
	isiClusters           *sync.Map
	defaultIsiClusterName string
}

//IsilonClusters To unmarshal secret.json file
type IsilonClusters struct {
	IsilonClusters []IsilonClusterConfig `json:"isilonClusters"`
}

//IsilonClusterConfig To hold config details of a isilon cluster
type IsilonClusterConfig struct {
	ClusterName      string `json:"clusterName"`
	IsiIP            string `json:"isiIP"`
	IsiPort          string `json:"isiPort,omitempty"`
	EndpointURL      string
	User             string `json:"username"`
	Password         string `json:"password"`
	IsiInsecure      *bool  `json:"isiInsecure,omitempty"`
	IsiPath          string `json:"isiPath,omitempty"`
	IsDefaultCluster bool   `json:"isDefaultCluster,omitempty"`
	isiSvc           *isiService
}

//To display the IsilonClusterConfig of a cluster
func (s IsilonClusterConfig) String() string {
	return fmt.Sprintf("ClusterName: %s, IsiIP: %s, IsiPort: %s, EndpointURL: %s, User: %s, IsiInsecure: %v, IsiPath: %s, IsDefaultCluster: %v, isiSvc: %v",
		s.ClusterName, s.IsiIP, s.IsiPort, s.EndpointURL, s.User, *s.IsiInsecure, s.IsiPath, s.IsDefaultCluster, s.isiSvc)
}

// New returns a new Service.
func New() Service {
	return &service{}
}

func (s *service) initializeServiceOpts(ctx context.Context) {
	// Get the SP's operating mode.
	s.mode = csictx.Getenv(ctx, gocsi.EnvVarMode)

	opts := Opts{}

	if port, ok := csictx.LookupEnv(ctx, constants.EnvPort); ok {
		opts.Port = port
	} else {
		// If the port number cannot be fetched, set it to default
		opts.Port = constants.DefaultPortNumber
	}

	if path, ok := csictx.LookupEnv(ctx, constants.EnvPath); ok {
		if path == "" {
			path = constants.DefaultIsiPath
		}
		opts.Path = path
	} else {
		opts.Path = constants.DefaultIsiPath
	}

	if accessZone, ok := csictx.LookupEnv(ctx, constants.EnvAccessZone); ok {
		if accessZone == "" {
			accessZone = constants.DefaultAccessZone
		}
		opts.AccessZone = accessZone
	} else {
		opts.AccessZone = constants.DefaultAccessZone
	}

	if nodeNameEnv, ok := csictx.LookupEnv(ctx, constants.EnvNodeName); ok {
		s.nodeID = nodeNameEnv
	}

	if nodeIPEnv, ok := csictx.LookupEnv(ctx, constants.EnvNodeIP); ok {
		s.nodeIP = nodeIPEnv
	}

	if kubeConfigPath, ok := csictx.LookupEnv(ctx, constants.EnvKubeConfigPath); ok {
		opts.KubeConfigPath = kubeConfigPath
	}

	if cfgFile, ok := csictx.LookupEnv(ctx, constants.EnvIsilonConfigFile); ok {
		isilonConfigFile = cfgFile
	} else {
		isilonConfigFile = constants.IsilonConfigFile
	}

	opts.allowedNetworks = utils.ParseArrayFromContext(ctx, constants.EnvAllowedNetworks)
	opts.QuotaEnabled = utils.ParseBooleanFromContext(ctx, constants.EnvQuotaEnabled)
	opts.Insecure = utils.ParseBooleanFromContext(ctx, constants.EnvInsecure)
	opts.AutoProbe = utils.ParseBooleanFromContext(ctx, constants.EnvAutoProbe)
	opts.DebugEnabled = utils.ParseBooleanFromContext(ctx, constants.EnvDebug)
	opts.Verbose = utils.ParseUintFromContext(ctx, constants.EnvVerbose)
	opts.NfsV3 = utils.ParseBooleanFromContext(ctx, constants.EnvNfsV3)
	opts.CustomTopologyEnabled = utils.ParseBooleanFromContext(ctx, constants.EnvCustomTopologyEnabled)

	s.opts = opts
}

// ValidateCreateVolumeRequest validates the CreateVolumeRequest parameter for a CreateVolume operation
func (s *service) ValidateCreateVolumeRequest(
	req *csi.CreateVolumeRequest) (int64, error) {
	cr := req.GetCapacityRange()
	sizeInBytes, err := validateVolSize(cr)
	if err != nil {
		return 0, err
	}

	volumeName := req.GetName()
	if volumeName == "" {
		return 0, status.Error(codes.InvalidArgument,
			"name cannot be empty")
	}

	vcs := req.GetVolumeCapabilities()
	isBlock := isVolumeTypeBlock(vcs)
	if isBlock {
		return 0, errors.New("raw block requested from NFS Volume")
	}

	return sizeInBytes, nil
}

func isVolumeTypeBlock(vcs []*csi.VolumeCapability) bool {
	for _, vc := range vcs {
		if at := vc.GetBlock(); at != nil {
			return true
		}
	}
	return false
}

// ValidateDeleteVolumeRequest validates the DeleteVolumeRequest parameter for a DeleteVolume operation
func (s *service) ValidateDeleteVolumeRequest(ctx context.Context,
	req *csi.DeleteVolumeRequest) error {

	if req.GetVolumeId() == "" {
		return status.Error(codes.InvalidArgument,
			"no volume id is provided by the DeleteVolumeRequest instance")
	}

	_, _, _, _, err := utils.ParseNormalizedVolumeID(ctx, req.GetVolumeId())
	if err != nil {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("failed to parse volume ID '%s', error : '%v'", req.GetVolumeId(), err))
	}

	return nil
}

func (s *service) probeAllClusters(ctx context.Context) error {
	isilonClusters := s.getIsilonClusters()

	probeSuccessCount := 0
	for i := range isilonClusters {
		err := s.probe(ctx, isilonClusters[i])
		if err == nil {
			probeSuccessCount++
		} else {
			log.Debugf("Probe failed for isilon cluster '%s' error:'%s'", isilonClusters[i].ClusterName, err)
		}
	}

	if probeSuccessCount == 0 {
		return fmt.Errorf("probe of all isilon clusters failed")
	}

	return nil
}

func (s *service) probe(ctx context.Context, clusterConfig *IsilonClusterConfig) error {

	log.Debugf("calling probe for cluster '%s'", clusterConfig.ClusterName)
	// Do a controller probe
	if strings.EqualFold(s.mode, constants.ModeController) {
		if err := s.controllerProbe(ctx, clusterConfig); err != nil {
			return err
		}
	} else if strings.EqualFold(s.mode, constants.ModeNode) {
		if err := s.nodeProbe(ctx, clusterConfig); err != nil {
			return err
		}
	} else {
		return status.Error(codes.FailedPrecondition,
			"Service mode not set")
	}

	return nil
}

func (s *service) probeOnStart(ctx context.Context) error {

	if utils.ParseBooleanFromContext(ctx, constants.EnvNoProbeOnStart) {

		log.Debug("X_CSI_ISILON_NO_PROBE_ON_START is true, skip 'probeOnStart'")

		return nil
	}

	log.Debug("X_CSI_ISILON_NO_PROBE_ON_START is false, executing 'probeOnStart'")

	return s.probeAllClusters(ctx)
}

func (s *service) autoProbe(ctx context.Context, isiConfig *IsilonClusterConfig) error {

	if isiConfig.isiSvc != nil {
		log.Debug("isiSvc already initialized, skip probing")
		return nil
	}

	if !s.opts.AutoProbe {
		return status.Error(codes.FailedPrecondition,
			"isiSvc not initialized, but auto probe is not enabled")
	}

	log.Debug("start auto-probing")
	return s.probe(ctx, isiConfig)
}

func (s *service) GetIsiClient(clientCtx context.Context, isiConfig *IsilonClusterConfig) (*isi.Client, error) {

	// First we fetch node labels using kubernetes API and check, if label
	// <provisionerName>.dellemc.com/<powerscalefqdnorip>: <provisionerName>
	// exists on node, if exists we use corresponding PowerScale FQDN or IP for creating connection
	// to PowerScale Array or else we fallback to using isiIP

	customTopologyFound := false
	if s.opts.CustomTopologyEnabled {
		k8sclientset, err := k8sutils.CreateKubeClientSet(s.opts.KubeConfigPath)
		if err != nil {
			log.Errorf("init client failed for custom topology: '%s'", err.Error())
			return nil, err
		}
		// access the API to fetch node object
		node, _ := k8sclientset.CoreV1().Nodes().Get(context.TODO(), s.nodeID, v1.GetOptions{})
		log.Debugf("Node %s details\n", node)

		// Iterate node labels and check if required label is available
		for lkey, lval := range node.Labels {
			log.Infof("Label is: %s:%s\n", lkey, lval)
			if strings.HasPrefix(lkey, constants.PluginName+"/") && lval == constants.PluginName {
				log.Infof("Topology label %s:%s available on node", lkey, lval)
				tList := strings.SplitAfter(lkey, "/")
				if len(tList) != 0 {
					isiConfig.IsiIP = tList[1]
					isiConfig.EndpointURL = fmt.Sprintf("https://%s:%s", isiConfig.IsiIP, isiConfig.IsiPort)
					customTopologyFound = true
				} else {
					log.Errorf("Fetching PowerScale FQDN/IP from topology label %s:%s failed, using isiIP "+
						"%s as PowerScale FQDN/IP", lkey, lval, isiConfig.IsiIP)
				}
				break
			}
		}
	}

	if s.opts.CustomTopologyEnabled && !customTopologyFound {
		log.Errorf("init client failed for custom topology")
		return nil, errors.New("init client failed for custom topology")
	}

	client, err := isi.NewClientWithArgs(
		clientCtx,
		isiConfig.EndpointURL,
		*isiConfig.IsiInsecure,
		s.opts.Verbose,
		isiConfig.User,
		"",
		isiConfig.Password,
		isiConfig.IsiPath,
	)
	if err != nil {
		log.Errorf("init client failed for isilon cluster '%s': '%s'", isiConfig.ClusterName, err.Error())
		return nil, err
	}

	return client, nil
}

func (s *service) GetIsiService(clientCtx context.Context, isiConfig *IsilonClusterConfig) (*isiService, error) {
	var isiClient *isi.Client
	var err error
	if isiClient, err = s.GetIsiClient(clientCtx, isiConfig); err != nil {
		return nil, err
	}

	return &isiService{
		endpoint: isiConfig.IsiIP,
		client:   isiClient,
	}, nil
}

func (s *service) validateOptsParameters(clusterConfig *IsilonClusterConfig) error {
	if clusterConfig.User == "" || clusterConfig.Password == "" || clusterConfig.IsiIP == "" {
		return fmt.Errorf("invalid isi service parameters, at least one of endpoint, username and password is empty. endpoint : endpoint '%s', username : '%s'", clusterConfig.IsiIP, clusterConfig.User)
	}

	return nil
}

func (s *service) logServiceStats() {

	fields := map[string]interface{}{
		"path":         s.opts.Path,
		"insecure":     s.opts.Insecure,
		"autoprobe":    s.opts.AutoProbe,
		"accesspoint":  s.opts.AccessZone,
		"quotaenabled": s.opts.QuotaEnabled,
		"mode":         s.mode,
	}

	log.WithFields(fields).Infof("Configured '%s'", constants.PluginName)
}

func (s *service) BeforeServe(
	ctx context.Context, sp *gocsi.StoragePlugin, lis net.Listener) error {
	s.initializeServiceOpts(ctx)
	s.logServiceStats()

	//Update the storage array list
	s.isiClusters = new(sync.Map)
	err := s.syncIsilonConfigs(ctx)
	if err != nil {
		return err
	}

	//Dynamically load the config
	go s.loadIsilonConfigs(ctx, isilonConfigFile)

	return s.probeOnStart(ctx)
}

func (s *service) loadIsilonConfigs(ctx context.Context, configFile string) error {
	log.Info("Updating cluster config details")
	watcher, _ := fsnotify.NewWatcher()
	defer watcher.Close()

	parentFolder, _ := filepath.Abs(filepath.Dir(configFile))
	log.Debug("Config folder: ", parentFolder)
	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create == fsnotify.Create && event.Name == parentFolder+"/..data" {
					log.Infof("**************** Cluster config file modified. Updating cluster config details: %s****************", event.Name)
					err := s.syncIsilonConfigs(ctx)
					if err != nil {
						log.Debug("Cluster configuration array length:", s.getIsilonClusterLength())
						log.Error("Invalid configuration in secret.json. Error:", err)
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error("cluster config file load error:", err)
			}
		}
	}()
	err := watcher.Add(parentFolder)
	if err != nil {
		log.Error("Unable to add file watcher for folder ", parentFolder)
		return err
	}
	<-done
	return nil
}

//Returns the size of arrays
func (s *service) getIsilonClusterLength() (length int) {
	length = 0
	s.isiClusters.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	return
}

var syncMutex sync.Mutex

//Reads the credentials from secrets and initialize all arrays.
func (s *service) syncIsilonConfigs(ctx context.Context) error {
	log.Info("************* Synchronizing Isilon Clusters' config **************")
	syncMutex.Lock()
	defer syncMutex.Unlock()

	configBytes, err := ioutil.ReadFile(isilonConfigFile)
	if err != nil {
		return fmt.Errorf("file ('%s') error: %v", isilonConfigFile, err)
	}

	if string(configBytes) != "" {
		log.Debugf("Current isilon configs:")
		s.isiClusters.Range(handler)
		newIsilonConfigs, defaultClusterName, err := s.getNewIsilonConfigs(configBytes)
		if err != nil {
			return err
		}

		// Update the isiClusters sync.Map
		s.isiClusters.Range(func(key interface{}, value interface{}) bool {
			s.isiClusters.Delete(key)
			return true
		})

		for k, v := range newIsilonConfigs {
			s.isiClusters.Store(k, v)
		}
		log.Debugf("New isilon configs:")
		s.isiClusters.Range(handler)

		s.defaultIsiClusterName = defaultClusterName
		if s.defaultIsiClusterName == "" {
			log.Warnf("no default cluster name/config available")
		}
	} else {
		return errors.New("isilon cluster details are not provided in isilon-creds secret")
	}

	return nil
}

func (s *service) getNewIsilonConfigs(configBytes []byte) (map[interface{}]interface{}, string, error) {
	var noOfDefaultClusters int
	var defaultIsiClusterName string

	jsonConfig := new(IsilonClusters)
	err := json.Unmarshal(configBytes, &jsonConfig)
	if err != nil {
		return nil, defaultIsiClusterName, fmt.Errorf("unable to parse islon clusters' config details [%v]", err)
	}

	if len(jsonConfig.IsilonClusters) == 0 {
		return nil, defaultIsiClusterName, errors.New("cluster details are not provided in isilon-creds secret")
	}

	if len(jsonConfig.IsilonClusters) > 1 && s.opts.CustomTopologyEnabled {
		return nil, defaultIsiClusterName, errors.New("custom topology is enabled and it expects single cluster config in secret")
	}

	newIsiClusters := make(map[interface{}]interface{})
	for i, config := range jsonConfig.IsilonClusters {
		log.Debugf("parsing config details for cluster %v", config.ClusterName)
		if config.ClusterName == "" {
			return nil, defaultIsiClusterName, fmt.Errorf("invalid value for clusterName at index [%d]", i)
		}
		if config.User == "" {
			return nil, defaultIsiClusterName, fmt.Errorf("invalid value for username at index [%d]", i)
		}
		if config.Password == "" {
			return nil, defaultIsiClusterName, fmt.Errorf("invalid value for password at index [%d]", i)
		}
		if config.IsiIP == "" {
			return nil, defaultIsiClusterName, fmt.Errorf("invalid value for isiIP at index [%d]", i)
		}
		if config.IsiPort == "" {
			config.IsiPort = s.opts.Port
		}
		if config.IsiInsecure == nil {
			config.IsiInsecure = &s.opts.Insecure
		}
		if config.IsiPath == "" {
			config.IsiPath = s.opts.Path
		}

		config.EndpointURL = fmt.Sprintf("https://%s:%s", config.IsiIP, config.IsiPort)
		clientCtx := utils.ConfigureLogger(s.opts.DebugEnabled)
		config.isiSvc, _ = s.GetIsiService(clientCtx, &config)

		newConfig := IsilonClusterConfig{}
		newConfig = config

		if config.IsDefaultCluster {
			noOfDefaultClusters++
			if noOfDefaultClusters > 1 {
				return nil, defaultIsiClusterName, fmt.Errorf("'isDefaultCluster' attribute set for multiple isilon cluster configs in 'isilonClusters': %s. Only one cluster should be marked as default cluster", config.ClusterName)
			}
		}

		if _, ok := newIsiClusters[config.ClusterName]; ok {
			return nil, defaultIsiClusterName, fmt.Errorf("duplicate cluster name [%s] found in input isilonClusters", config.ClusterName)
		}
		newIsiClusters[config.ClusterName] = &newConfig
		if config.IsDefaultCluster {
			defaultIsiClusterName = config.ClusterName
		}

		fields := map[string]interface{}{
			"ClusterName":      config.ClusterName,
			"IsiIP":            config.IsiIP,
			"IsiPort":          config.IsiPort,
			"Username":         config.User,
			"Password":         "*******",
			"IsiInsecure":      *config.IsiInsecure,
			"IsiPath":          config.IsiPath,
			"IsDefaultCluster": config.IsDefaultCluster,
		}
		log.WithFields(fields).Infof("new config details for cluster %s", config.ClusterName)
	}
	return newIsiClusters, defaultIsiClusterName, nil
}

func handler(key, value interface{}) bool {
	log.Debugf(value.(*IsilonClusterConfig).String())
	return true
}

// Returns details of a cluster with name clusterName
func (s *service) getIsilonClusterConfig(clusterName string) *IsilonClusterConfig {
	if cluster, ok := s.isiClusters.Load(clusterName); ok {
		return cluster.(*IsilonClusterConfig)
	}
	return nil
}

// Returns details of all isilon clusters
func (s *service) getIsilonClusters() []*IsilonClusterConfig {
	list := make([]*IsilonClusterConfig, 0)
	s.isiClusters.Range(func(key interface{}, value interface{}) bool {
		list = append(list, value.(*IsilonClusterConfig))
		return true
	})
	return list
}

// GetCSINodeID gets the id of the CSI node which regards the node name as node id
func (s *service) GetCSINodeID(ctx context.Context) (string, error) {
	// if the node id has already been initialized, return it
	if s.nodeID != "" {
		return s.nodeID, nil
	}
	// node id couldn't be read from env variable while initializing service, return with error
	return "", errors.New("cannot get node id")
}

// GetCSINodeIP gets the IP of the CSI node
func (s *service) GetCSINodeIP(ctx context.Context) (string, error) {
	// if the node ip has already been initialized, return it
	if s.nodeIP != "" {
		return s.nodeIP, nil
	}
	// node id couldn't be read from env variable while initializing service, return with error
	return "", errors.New("cannot get node IP")
}

func (s *service) getVolByName(ctx context.Context, isiPath, volName string, isiConfig *IsilonClusterConfig) (isi.Volume, error) {

	// The `GetVolume` API returns a slice of volumes, but when only passing
	// in a volume ID, the response will be just the one volume
	vol, err := isiConfig.isiSvc.GetVolume(ctx, isiPath, "", volName)
	if err != nil {
		return nil, err
	}
	return vol, nil
}

// Provide periodic logging of statistics like goroutines and memory
func (s *service) logStatistics() {
	if s.opts.DebugEnabled {
		if s.statisticsCounter = s.statisticsCounter + 1; (s.statisticsCounter % 100) == 0 {
			goroutines := runtime.NumGoroutine()
			memstats := new(runtime.MemStats)
			runtime.ReadMemStats(memstats)
			fields := map[string]interface{}{
				"GoRoutines":   goroutines,
				"HeapAlloc":    memstats.HeapAlloc,
				"HeapReleased": memstats.HeapReleased,
				"StackSys":     memstats.StackSys,
			}
			log.WithFields(fields).Debugf("resource statistics counter: %d", s.statisticsCounter)
		}
	}
}

func (s *service) getIsiPathForVolumeFromClusterConfig(clusterConfig *IsilonClusterConfig) string {
	if clusterConfig.IsiPath == "" {
		return s.opts.Path
	}
	return clusterConfig.IsiPath
}

//Set cluster name in log messages and re-initialize the context
func setClusterContext(ctx context.Context, clusterName string) (context.Context, *logrus.Entry) {
	return setLogFieldsInContext(ctx, clusterName, utils.ClusterName)
}

//Set runID in log messages and re-initialize the context
func setRunIDContext(ctx context.Context, runID string) (context.Context, *logrus.Entry) {
	return setLogFieldsInContext(ctx, runID, utils.RunID)
}

var logMutex sync.Mutex

//Common method to get log and context
func setLogFieldsInContext(ctx context.Context, logParam string, logType string) (context.Context, *logrus.Entry) {
	logMutex.Lock()
	defer logMutex.Unlock()

	fields := logrus.Fields{}
	fields, ok := ctx.Value(utils.LogFields).(logrus.Fields)
	if !ok {
		fields = logrus.Fields{}
	}
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields[logType] = logParam
	ulog, ok := ctx.Value(utils.PowerScaleLogger).(*logrus.Entry)
	if !ok {
		ulog = utils.GetLogger().WithFields(fields)
	}
	ulog = ulog.WithFields(fields)
	ctx = context.WithValue(ctx, utils.PowerScaleLogger, ulog)
	ctx = context.WithValue(ctx, utils.LogFields, fields)
	return ctx, ulog
}

// GetLogger creates custom logger handler
func GetLogger(ctx context.Context) (context.Context, *logrus.Entry) {
	var rid string
	fields := logrus.Fields{}
	if ctx == nil {
		return ctx, utils.GetLogger().WithFields(fields)
	}

	headers, ok := metadata.FromIncomingContext(ctx)
	if ok {
		reqid, ok := headers[csictx.RequestIDKey]
		if ok && len(reqid) > 0 {
			rid = reqid[0]
		}
	}

	fields, _ = ctx.Value(utils.LogFields).(logrus.Fields)
	if fields == nil {
		fields = logrus.Fields{}
	}

	if ok {
		fields[utils.RequestID] = rid
	}

	logMutex.Lock()
	defer logMutex.Unlock()
	l := utils.GetLogger()
	logWithFields := l.WithFields(fields)
	ctx = context.WithValue(ctx, utils.PowerScaleLogger, logWithFields)
	ctx = context.WithValue(ctx, utils.LogFields, fields)
	return ctx, logWithFields
}

// GetRunIDLog function returns logger with runID
func GetRunIDLog(ctx context.Context) (context.Context, *logrus.Entry, string) {
	var rid string
	fields := logrus.Fields{}
	if ctx == nil {
		return ctx, utils.GetLogger().WithFields(fields), rid
	}

	headers, ok := metadata.FromIncomingContext(ctx)
	if ok {
		reqid, ok := headers[csictx.RequestIDKey]
		if ok && len(reqid) > 0 {
			rid = reqid[0]
		} else {
			atomic.AddInt64(&runid, 1)
			rid = fmt.Sprintf("%d", runid)
		}
	}

	fields, _ = ctx.Value(utils.LogFields).(logrus.Fields)
	if fields == nil {
		fields = logrus.Fields{}
	}

	if ok {
		fields[utils.RunID] = rid
	}

	logMutex.Lock()
	defer logMutex.Unlock()
	l := utils.GetLogger()
	log := l.WithFields(fields)
	ctx = context.WithValue(ctx, utils.PowerScaleLogger, log)
	ctx = context.WithValue(ctx, utils.LogFields, fields)
	return ctx, log, rid
}

// getIsilonConfig returns the cluster config
func (s *service) getIsilonConfig(ctx context.Context, clusterName *string) (*IsilonClusterConfig, error) {
	if *clusterName == "" {
		log.Infof("Request doesn't include cluster name. Use default cluster '%s'", s.defaultIsiClusterName)
		*clusterName = s.defaultIsiClusterName
		if s.defaultIsiClusterName == "" {
			return nil, fmt.Errorf("no default cluster config available to continue with request")
		}
	}

	isiConfig := s.getIsilonClusterConfig(*clusterName)
	if isiConfig == nil {
		return nil, fmt.Errorf("failed to get cluster config details for clusterName: '%s'", *clusterName)
	}

	return isiConfig, nil
}
