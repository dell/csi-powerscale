package service

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
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/akutz/gournal"
	"github.com/dell/csi-isilon/v2/common/k8sutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gopkg.in/yaml.v3"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/dell/csi-isilon/v2/common/utils"
	"github.com/dell/csi-isilon/v2/core"
	commonext "github.com/dell/dell-csi-extensions/common"
	podmon "github.com/dell/dell-csi-extensions/podmon"
	csiext "github.com/dell/dell-csi-extensions/replication"
	vgsext "github.com/dell/dell-csi-extensions/volumeGroupSnapshot"
	"github.com/dell/gocsi"
	csictx "github.com/dell/gocsi/context"
	isi "github.com/dell/goisilon"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// To maintain runid for Non debug mode. Note: CSI will not generate runid if CSI_DEBUG=false
var (
	runid            int64
	isilonConfigFile string
)

// DriverConfigParamsFile is the name of the input driver config params file
var DriverConfigParamsFile string

// Manifest is the SP's manifest.
var Manifest = map[string]string{
	"url":    "http://github.com/dell/csi-isilon",
	"semver": core.SemVer,
	"commit": core.CommitSha32,
	"formed": core.CommitTime.Format(time.RFC1123),
}

var noProbeOnStart bool

// Service is the CSI service provider.
type Service interface {
	csi.ControllerServer
	csi.IdentityServer
	csi.NodeServer
	BeforeServe(context.Context, *gocsi.StoragePlugin, net.Listener) error
	RegisterAdditionalServers(server *grpc.Server)
}

// Opts defines service configuration options.
type Opts struct {
	Port                      string
	AccessZone                string
	Path                      string
	IsiVolumePathPermissions  string
	SkipCertificateValidation bool
	AutoProbe                 bool
	QuotaEnabled              bool
	Verbose                   uint
	CustomTopologyEnabled     bool
	KubeConfigPath            string
	allowedNetworks           []string
	MaxVolumesPerNode         int64
	isiAuthType               uint8
	IsHealthMonitorEnabled    bool
	IgnoreUnresolvableHosts   bool
	replicationContextPrefix  string
	replicationPrefix         string
}

type service struct {
	opts                  Opts
	mode                  string
	nodeID                string
	nodeIP                string
	statisticsCounter     int
	isiClusters           *sync.Map
	defaultIsiClusterName string
	k8sclient             *kubernetes.Clientset
}

// IsilonClusters To unmarshal secret.yaml file
type IsilonClusters struct {
	IsilonClusters []IsilonClusterConfig `yaml:"isilonClusters"`
}

// IsilonClusterConfig To hold config details of a isilon cluster
type IsilonClusterConfig struct {
	ClusterName               string `yaml:"clusterName"`
	Endpoint                  string `yaml:"endpoint"`
	EndpointPort              string `yaml:"endpointPort,omitempty"`
	MountEndpoint             string `yaml:"mountEndpoint,omitempty"` // This field is used to retain the orignal Endpoint after CSM-Authorization has been injected
	EndpointURL               string
	accessZone                string `yaml:"accessZone,omitempty"`
	User                      string `yaml:"username"`
	Password                  string `yaml:"password"`
	SkipCertificateValidation *bool  `yaml:"skipCertificateValidation,omitempty"`
	IsiPath                   string `yaml:"isiPath,omitempty"`
	IsiVolumePathPermissions  string `yaml:"isiVolumePathPermissions,omitempty"`
	IsDefault                 *bool  `yaml:"isDefault,omitempty"`
	ReplicationCertificateID  string `yaml:"replicationCertificateID,omitempty"`
	IgnoreUnresolvableHosts   *bool  `yaml:"ignoreUnresolvableHosts,omitempty"`
	isiSvc                    *isiService
}

// To display the IsilonClusterConfig of a cluster
func (s IsilonClusterConfig) String() string {
	return fmt.Sprintf("ClusterName: %s, Endpoint: %s, EndpointPort: %s, EndpointURL: %s, User: %s, SkipCertificateValidation: %v, IsiPath: %s, IsiVolumePathPermissions: %s, IsDefault: %v, IgnoreUnresolvableHosts: %v, AccessZone: %s, isiSvc: %v",
		s.ClusterName, s.Endpoint, s.EndpointPort, s.EndpointURL, s.User, *s.SkipCertificateValidation, s.IsiPath, s.IsiVolumePathPermissions, *s.IsDefault, *s.IgnoreUnresolvableHosts, s.accessZone, s.isiSvc)
}

// New returns a new Service.
func New() Service {
	return &service{}
}

func (s *service) initializeServiceOpts(ctx context.Context) error {
	log := utils.GetLogger()
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

	if isiVolumePathPermissions, ok := csictx.LookupEnv(ctx, constants.EnvIsiVolumePathPermissions); ok {
		if isiVolumePathPermissions == "" {
			isiVolumePathPermissions = constants.DefaultIsiVolumePathPermissions
		}
		opts.IsiVolumePathPermissions = isiVolumePathPermissions
	} else {
		opts.IsiVolumePathPermissions = constants.DefaultIsiVolumePathPermissions
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
	if replicationContextPrefix, ok := csictx.LookupEnv(ctx, constants.EnvReplicationContextPrefix); ok {
		opts.replicationContextPrefix = replicationContextPrefix + "/"
	}
	if replicationPrefix, ok := csictx.LookupEnv(ctx, constants.EnvReplicationPrefix); ok {
		opts.replicationPrefix = replicationPrefix
	}
	if MaxVolumesPerNode, err := utils.ParseInt64FromContext(ctx, constants.EnvMaxVolumesPerNode); err != nil {
		log.Warnf("error while parsing env variable '%s', %s, defaulting to 0", constants.EnvMaxVolumesPerNode, err)
		opts.MaxVolumesPerNode = 0
	} else {
		opts.MaxVolumesPerNode = MaxVolumesPerNode
	}

	allowedNetworks, err := utils.ParseArrayFromContext(ctx, constants.EnvAllowedNetworks)
	if err != nil {
		log.Errorf("error while parsing allowedNetworks, %v", err)
		return err
	}
	opts.allowedNetworks = allowedNetworks

	opts.QuotaEnabled = utils.ParseBooleanFromContext(ctx, constants.EnvQuotaEnabled)
	opts.SkipCertificateValidation = utils.ParseBooleanFromContext(ctx, constants.EnvSkipCertificateValidation)
	opts.isiAuthType = uint8(utils.ParseUintFromContext(ctx, constants.EnvIsiAuthType))
	opts.AutoProbe = utils.ParseBooleanFromContext(ctx, constants.EnvAutoProbe)
	opts.Verbose = utils.ParseUintFromContext(ctx, constants.EnvVerbose)
	opts.CustomTopologyEnabled = utils.ParseBooleanFromContext(ctx, constants.EnvCustomTopologyEnabled)
	opts.IsHealthMonitorEnabled = utils.ParseBooleanFromContext(ctx, constants.EnvIsHealthMonitorEnabled)
	opts.IgnoreUnresolvableHosts = utils.ParseBooleanFromContext(ctx, constants.EnvIgnoreUnresolvableHosts)

	s.opts = opts

	if c, err := k8sutils.CreateKubeClientSet(s.opts.KubeConfigPath); err == nil {
		s.k8sclient = c
	}

	return nil
}

// ValidateCreateVolumeRequest validates the CreateVolumeRequest parameter for a CreateVolume operation
func (s *service) ValidateCreateVolumeRequest(
	req *csi.CreateVolumeRequest,
) (int64, error) {
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
	req *csi.DeleteVolumeRequest,
) error {
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
	ctx, log := GetLogger(ctx)

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
	ctx, log := GetLogger(ctx)
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
	} else if strings.EqualFold(s.mode, "") {
		log.Warn("Service mode not set, attempting both controller and node probe")
		controllerErr := s.controllerProbe(ctx, clusterConfig)
		if controllerErr != nil {
			return fmt.Errorf("probe failed")
		}

		nodeProbeErr := s.nodeProbe(ctx, clusterConfig)
		if nodeProbeErr != nil {
			return fmt.Errorf("probe failed")
		}
	} else {
		return status.Error(codes.FailedPrecondition,
			"Invalid mode")
	}

	return nil
}

func (s *service) probeOnStart(ctx context.Context) error {
	ctx, log := GetLogger(ctx)
	if noProbeOnStart {
		log.Debugf("noProbeOnStart is true , skip probe")
		return nil
	}

	return s.probeAllClusters(ctx)
}

func (s *service) setNoProbeOnStart(ctx context.Context) {
	ctx, log := GetLogger(ctx)
	if utils.ParseBooleanFromContext(ctx, constants.EnvNoProbeOnStart) {
		log.Debug("X_CSI_ISI_NO_PROBE_ON_START is true, set noProbeOnStart to true")
		noProbeOnStart = true
		return
	}
	log.Debug("X_CSI_ISI_NO_PROBE_ON_START is false, set noProbeOnStart to false ")
	noProbeOnStart = false
}

func (s *service) autoProbe(ctx context.Context, isiConfig *IsilonClusterConfig) error {
	ctx, log := GetLogger(ctx)
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

func (s *service) GetIsiClient(clientCtx context.Context, isiConfig *IsilonClusterConfig, logLevel logrus.Level) (*isi.Client, error) {
	clientCtx, log := GetLogger(clientCtx)

	// First we fetch node labels using kubernetes API and check, if label
	// <provisionerName>.dellemc.com/<powerscalefqdnorip>: <provisionerName>
	// exists on node, if exists we use corresponding PowerScale FQDN or IP for creating connection
	// to PowerScale Array or else we fallback to using endpoint

	customTopologyFound := false
	if s.opts.CustomTopologyEnabled {
		labels, err := s.GetNodeLabels()
		if err != nil {
			return nil, err
		}

		// Iterate node labels and check if required label is available
		for lkey, lval := range labels {
			log.Infof("Label is: %s:%s\n", lkey, lval)
			if strings.HasPrefix(lkey, constants.PluginName+"/") && lval == constants.PluginName {
				log.Infof("Topology label %s:%s available on node", lkey, lval)
				tList := strings.SplitAfter(lkey, "/")
				if len(tList) != 0 {
					isiConfig.Endpoint = tList[1]
					isiConfig.EndpointURL = fmt.Sprintf("https://%s:%s", isiConfig.Endpoint, isiConfig.EndpointPort)
					customTopologyFound = true
				} else {
					log.Errorf("Fetching PowerScale FQDN/IP from topology label %s:%s failed, using endpoint "+
						"%s as PowerScale FQDN/IP", lkey, lval, isiConfig.Endpoint)
				}
				break
			}
		}
	}

	if s.opts.CustomTopologyEnabled && !customTopologyFound {
		log.Errorf("init client failed for custom topology")
		return nil, errors.New("init client failed for custom topology")
	}

	if logLevel == logrus.DebugLevel {
		clientCtx = context.WithValue(
			clientCtx,
			gournal.LevelKey(),
			gournal.DebugLevel)

		gournal.DefaultLevel = gournal.DebugLevel
	} else {
		gournalLevel := getGournalLevelFromLogrusLevel(logLevel)
		clientCtx = context.WithValue(
			clientCtx,
			gournal.LevelKey(),
			gournalLevel)

		gournal.DefaultLevel = gournalLevel
	}

	client, err := isi.NewClientWithArgs(
		clientCtx,
		isiConfig.EndpointURL,
		*isiConfig.SkipCertificateValidation,
		s.opts.Verbose,
		isiConfig.User,
		"",
		isiConfig.Password,
		isiConfig.IsiPath,
		isiConfig.IsiVolumePathPermissions,
		*isiConfig.IgnoreUnresolvableHosts,
		s.opts.isiAuthType,
	)
	if err != nil {
		log.Errorf("init client failed for isilon cluster '%s': '%s'", isiConfig.ClusterName, err.Error())
		return nil, err
	}

	return client, nil
}

func getGournalLevelFromLogrusLevel(logLevel logrus.Level) gournal.Level {
	gournalLevel := gournal.ParseLevel(logLevel.String())
	return gournalLevel
}

func (s *service) GetIsiService(clientCtx context.Context, isiConfig *IsilonClusterConfig, logLevel logrus.Level) (*isiService, error) {
	var isiClient *isi.Client
	var err error
	if isiClient, err = s.GetIsiClient(clientCtx, isiConfig, logLevel); err != nil {
		return nil, err
	}

	return &isiService{
		endpoint: isiConfig.Endpoint,
		client:   isiClient,
	}, nil
}

func (s *service) validateOptsParameters(clusterConfig *IsilonClusterConfig) error {
	if clusterConfig.User == "" || clusterConfig.Password == "" || clusterConfig.Endpoint == "" {
		return fmt.Errorf("invalid isi service parameters, at least one of endpoint, username and password is empty. endpoint : endpoint '%s', username : '%s'", clusterConfig.Endpoint, clusterConfig.User)
	}

	return nil
}

func (s *service) logServiceStats() {
	fields := map[string]interface{}{
		"path":                      s.opts.Path,
		"skipCertificateValidation": s.opts.SkipCertificateValidation,
		"autoprobe":                 s.opts.AutoProbe,
		"accesspoint":               s.opts.AccessZone,
		"quotaenabled":              s.opts.QuotaEnabled,
		"mode":                      s.mode,
	}
	// TODO: Replace logrus with log
	logrus.WithFields(fields).Infof("Configured '%s'", constants.PluginName)
}

func (s *service) BeforeServe(
	ctx context.Context, _ *gocsi.StoragePlugin, _ net.Listener,
) error {
	log := utils.GetLogger()

	if err := s.initializeServiceOpts(ctx); err != nil {
		return err
	}

	s.logServiceStats()

	// Update the storage array list
	s.isiClusters = new(sync.Map)
	s.setNoProbeOnStart(ctx)

	// Update config params
	vc := viper.New()
	vc.AutomaticEnv()
	vc.SetConfigFile(DriverConfigParamsFile)
	if err := vc.ReadInConfig(); err != nil {
		log.Warnf("unable to read driver config params from file '%s'. Using defaults.", DriverConfigParamsFile)
	}
	if err := s.updateDriverConfigParams(ctx, vc); err != nil {
		return err
	}

	// Watch for changes to driver config params file
	vc.WatchConfig()
	vc.OnConfigChange(func(_ fsnotify.Event) {
		log.Infof("Driver config params file changed")
		if err := s.updateDriverConfigParams(ctx, vc); err != nil {
			log.Warn(err)
		}
	})

	// Dynamically load the config
	go s.loadIsilonConfigs(ctx, isilonConfigFile)
	go s.startAPIService(ctx)

	return s.probeOnStart(ctx)
}

// RegisterAdditionalServers registers any additional grpc services that use the CSI socket.
func (s *service) RegisterAdditionalServers(server *grpc.Server) {
	_, log := GetLogger(context.Background())
	log.Info("Registering additional GRPC servers")
	csiext.RegisterReplicationServer(server, s)
	vgsext.RegisterVolumeGroupSnapshotServer(server, s)
	podmon.RegisterPodmonServer(server, s)
}

func (s *service) loadIsilonConfigs(ctx context.Context, configFile string) error {
	ctx, log := GetLogger(ctx)
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
				if event.Has(fsnotify.Create) && event.Name == parentFolder+"/..data" {
					log.Infof("**************** Cluster config file modified. Updating cluster config details: %s****************", event.Name)
					// set noProbeOnStart to false so subsequent calls can lead to probe
					noProbeOnStart = false
					err := s.syncIsilonConfigs(ctx)
					if err != nil {
						log.Debug("Cluster configuration array length:", s.getIsilonClusterLength())
						log.Error("Invalid configuration in secret.yaml. Error:", err)
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

// Returns the size of arrays
func (s *service) getIsilonClusterLength() (length int) {
	length = 0
	s.isiClusters.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	return
}

var syncMutex sync.Mutex

// Reads the credentials from secrets and initialize all arrays.
func (s *service) syncIsilonConfigs(ctx context.Context) error {
	ctx, log := GetLogger(ctx)
	log.Info("************* Synchronizing Isilon Clusters' config **************")
	syncMutex.Lock()
	defer syncMutex.Unlock()

	configBytes, err := os.ReadFile(filepath.Clean(isilonConfigFile))
	if err != nil {
		return fmt.Errorf("file ('%s') error: %v", isilonConfigFile, err)
	}

	if string(configBytes) != "" {
		log.Debugf("Current isilon configs:")
		s.isiClusters.Range(handler)
		newIsilonConfigs, defaultClusterName, err := s.getNewIsilonConfigs(ctx, configBytes)
		if err != nil {
			return err
		}

		// Update the isiClusters sync.Map
		s.isiClusters.Range(func(key interface{}, _ interface{}) bool {
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
			log.Errorf("no default cluster name/config available")
		}
	} else {
		return errors.New("isilon cluster details are not provided in isilon-creds secret")
	}
	return nil
}

func unmarshalYAMLContent(configBytes []byte) (*IsilonClusters, error) {
	yamlConfig := new(IsilonClusters)
	err := yaml.Unmarshal(configBytes, yamlConfig)
	return yamlConfig, err
}

func (s *service) getNewIsilonConfigs(ctx context.Context, configBytes []byte) (map[interface{}]interface{}, string, error) {
	var noOfDefaultClusters int
	var defaultIsiClusterName string
	logLevel := utils.GetCurrentLogLevel()
	log := utils.GetLogger()

	var inputConfigs *IsilonClusters
	var yamlErr error
	var err error

	log.Info("reading secret file to validate cluster config details")
	inputConfigs, yamlErr = unmarshalYAMLContent(configBytes)
	if yamlErr != nil {
		log.Errorf("failed to parse isilon clusters' config details as yaml data, error: %v", yamlErr)
		return nil, defaultIsiClusterName, fmt.Errorf("failed to parse isilon clusters' config details as yaml data")
	}

	if len(inputConfigs.IsilonClusters) == 0 {
		return nil, defaultIsiClusterName, errors.New("cluster details are not provided in isilon-creds secret")
	}

	if len(inputConfigs.IsilonClusters) > 1 && s.opts.CustomTopologyEnabled {
		return nil, defaultIsiClusterName, errors.New("custom topology is enabled and it expects single cluster config in secret")
	}

	newIsiClusters := make(map[interface{}]interface{})
	for i, clusterConfig := range inputConfigs.IsilonClusters {
		config := clusterConfig
		log.Debugf("parsing config details for cluster %v", config.ClusterName)
		if config.ClusterName == "" {
			return nil, defaultIsiClusterName, fmt.Errorf("clusterName not provided in secret at index [%d]", i)
		}
		if config.User == "" {
			return nil, defaultIsiClusterName, fmt.Errorf("username not provided for cluster %s at index [%d]", config.ClusterName, i)
		}
		if config.Password == "" {
			return nil, defaultIsiClusterName, fmt.Errorf("password not provided for cluster %s  at index [%d]", config.ClusterName, i)
		}
		if config.Endpoint == "" {
			return nil, defaultIsiClusterName, fmt.Errorf("endpoint not provided for cluster %s at index [%d]", config.ClusterName, i)
		}

		// Let Endpoint be generic.
		// Take out https prefix from it, if present, and let it's consumers to use it the way they want
		if strings.HasPrefix(config.Endpoint, "https://") {
			config.Endpoint = strings.TrimPrefix(config.Endpoint, "https://")
		}

		if config.EndpointPort == "" {
			log.Warnf("using default as EndpointPort not provided for cluster %s in secret at index [%d]", config.ClusterName, i)
			config.EndpointPort = s.opts.Port
		}

		if config.SkipCertificateValidation == nil {
			config.SkipCertificateValidation = &s.opts.SkipCertificateValidation
		}

		if config.IsiPath == "" {
			log.Warnf("using default as IsiPath not provided for cluster %s in secret at index [%d]", config.ClusterName, i)
			config.IsiPath = s.opts.Path
		}

		if config.IsiVolumePathPermissions == "" {
			log.Warnf("using default as IsiVolumePathPermissions not provided for cluster %s in secret at index [%d]", config.ClusterName, i)
			config.IsiVolumePathPermissions = s.opts.IsiVolumePathPermissions
		}

		if config.IgnoreUnresolvableHosts == nil {
			config.IgnoreUnresolvableHosts = &s.opts.IgnoreUnresolvableHosts
		}

		config.EndpointURL = fmt.Sprintf("https://%s:%s", config.Endpoint, config.EndpointPort)
		clientCtx, _ := GetLogger(ctx)
		if !noProbeOnStart {
			config.isiSvc, err = s.GetIsiService(clientCtx, &config, logLevel)
			if err != nil {
				log.Errorf("failed to get isi client for  cluster %s, error: %v", config.ClusterName, err)
			}
		}

		if config.IsDefault == nil {
			defaultBoolValue := false
			config.IsDefault = &defaultBoolValue
		}

		if *config.IsDefault == true {
			noOfDefaultClusters++
			if noOfDefaultClusters > 1 {
				return nil, defaultIsiClusterName, fmt.Errorf("'IsDefault' attribute set for multiple isilon cluster configs in 'isilonClusters': %s. Only one cluster should be marked as default cluster", config.ClusterName)
			}
		}

		if _, ok := newIsiClusters[config.ClusterName]; ok {
			return nil, defaultIsiClusterName, fmt.Errorf("duplicate cluster name [%s] found in input isilonClusters", config.ClusterName)
		}

		newConfig := IsilonClusterConfig{}
		newConfig = config

		newIsiClusters[config.ClusterName] = &newConfig
		if *config.IsDefault {
			defaultIsiClusterName = config.ClusterName
		}

		fields := map[string]interface{}{
			"ClusterName":               config.ClusterName,
			"Endpoint":                  config.Endpoint,
			"EndpointPort":              config.EndpointPort,
			"Username":                  config.User,
			"Password":                  "*******",
			"SkipCertificateValidation": *config.SkipCertificateValidation,
			"IsiPath":                   config.IsiPath,
			"IsiVolumePathPermissions":  config.IsiVolumePathPermissions,
			"IsDefault":                 *config.IsDefault,
			"IgnoreUnresolvableHosts":   *config.IgnoreUnresolvableHosts,
		}
		// TODO: Replace logrus with log
		logrus.WithFields(fields).Infof("new config details set for cluster %s", config.ClusterName)
	}

	return newIsiClusters, defaultIsiClusterName, nil
}

func handler(_, value interface{}) bool {
	_, log := GetLogger(context.Background())
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
	s.isiClusters.Range(func(_ interface{}, value interface{}) bool {
		list = append(list, value.(*IsilonClusterConfig))
		return true
	})
	return list
}

// Update configurable params from configmap
func (s *service) updateDriverConfigParams(ctx context.Context, v *viper.Viper) error {
	log := utils.GetLogger()
	logLevel := constants.DefaultLogLevel
	if v.IsSet(constants.ParamCSILogLevel) {
		inputLogLevel := v.GetString(constants.ParamCSILogLevel)
		if inputLogLevel != "" {
			inputLogLevel = strings.ToLower(inputLogLevel)
			var err error
			logLevel, err = utils.ParseLogLevel(inputLogLevel)
			if err != nil {
				return fmt.Errorf("input log level %q is not valid", inputLogLevel)
			}
		}
	}
	utils.UpdateLogLevel(logLevel)
	log.Infof("log level set to '%s'", logLevel)

	err := s.syncIsilonConfigs(ctx)
	if err != nil {
		return err
	}

	return nil
}

// GetCSINodeID gets the id of the CSI node which regards the node name as node id
func (s *service) GetCSINodeID(_ context.Context) (string, error) {
	// if the node id has already been initialized, return it
	if s.nodeID != "" {
		return s.nodeID, nil
	}
	// node id couldn't be read from env variable while initializing service, return with error
	return "", errors.New("cannot get node id")
}

// GetCSINodeIP gets the IP of the CSI node
func (s *service) GetCSINodeIP(_ context.Context) (string, error) {
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
		// TODO: Replace logrus with log
		logrus.WithFields(fields).Debugf("resource statistics counter: %d", s.statisticsCounter)
	}
}

func (s *service) getIsiPathForVolumeFromClusterConfig(clusterConfig *IsilonClusterConfig) string {
	if clusterConfig.IsiPath == "" {
		return s.opts.Path
	}
	return clusterConfig.IsiPath
}

// Set cluster name in log messages and re-initialize the context
func setClusterContext(ctx context.Context, clusterName string) (context.Context, *logrus.Entry) {
	return setLogFieldsInContext(ctx, clusterName, utils.ClusterName)
}

// Set runID in log messages and re-initialize the context
func setRunIDContext(ctx context.Context, runID string) (context.Context, *logrus.Entry) {
	return setLogFieldsInContext(ctx, runID, utils.RunID)
}

var logMutex sync.Mutex

// Common method to get log and context
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
	ctx, log := GetLogger(ctx)
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

func (s *service) GetNodeLabels() (map[string]string, error) {
	log := utils.GetLogger()
	k8sclientset, err := k8sutils.CreateKubeClientSet(s.opts.KubeConfigPath)
	if err != nil {
		log.Errorf("init client failed: '%s'", err.Error())
		return nil, err
	}
	// access the API to fetch node object
	node, err := k8sclientset.CoreV1().Nodes().Get(context.TODO(), s.nodeID, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	log.Debugf("Node %s details\n", node)

	return node.Labels, nil
}

func (s *service) ProbeController(ctx context.Context,
	_ *commonext.ProbeControllerRequest) (
	*commonext.ProbeControllerResponse, error,
) {
	ctx, log := GetLogger(ctx)

	if !strings.EqualFold(s.mode, "node") {
		log.Debugf("controllerProbe")
		if err := s.probeAllClusters(ctx); err != nil {
			log.Errorf("error in controllerProbe: %s", err.Error())
			return nil, err
		}
	}

	ready := new(wrapperspb.BoolValue)
	ready.Value = true
	rep := new(commonext.ProbeControllerResponse)
	rep.Ready = ready
	rep.Name = constants.PluginName
	rep.VendorVersion = core.SemVer
	rep.Manifest = Manifest

	log.Debug(fmt.Sprintf("ProbeController returning: %v", rep.Ready.GetValue()))

	return rep, nil
}

// WithRP appends Replication Prefix to provided string
func (s *service) WithRP(key string) string {
	return s.opts.replicationPrefix + "/" + key
}

func (s *service) validateIsiPath(ctx context.Context, volName string) (string, error) {
	if s.k8sclient == nil {
		return "", errors.New("no k8s clientset")
	}

	pvc, err := s.k8sclient.CoreV1().PersistentVolumes().Get(ctx, volName, v1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to get PersistentVolume: %w", err)
	}

	if pvc.Spec.StorageClassName == "" {
		return "", nil
	}

	sc, err := s.k8sclient.StorageV1().StorageClasses().Get(ctx, pvc.Spec.StorageClassName, v1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to get StorageClass: %w", err)
	}

	isiPath, ok := sc.Parameters[IsiPathParam]
	if !ok || isiPath == "" {
		return "", nil
	}

	return isiPath, nil
}
