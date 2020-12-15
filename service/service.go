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
	"errors"
	"fmt"
	"github.com/dell/csi-isilon/common/k8sutils"
	"net"
	"runtime"
	"strings"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/csi-isilon/common/constants"
	"github.com/dell/csi-isilon/common/utils"
	"github.com/dell/csi-isilon/core"
	isi "github.com/dell/goisilon"
	"github.com/rexray/gocsi"
	csictx "github.com/rexray/gocsi/context"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	Endpoint              string
	Port                  string
	EndpointURL           string
	User                  string
	Password              string
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
}

type service struct {
	opts              Opts
	mode              string
	nodeID            string
	nodeIP            string
	isiSvc            *isiService
	statisticsCounter int
}

// New returns a new Service.
func New() Service {
	return &service{}
}

func (s *service) initializeService(ctx context.Context) {
	// Get the SP's operating mode.
	s.mode = csictx.Getenv(ctx, gocsi.EnvVarMode)

	opts := Opts{}

	if port, ok := csictx.LookupEnv(ctx, constants.EnvPort); ok {
		opts.Port = port
	} else {
		// If the port number cannot be fetched, set it to default
		opts.Port = constants.DefaultPortNumber
	}

	if ep, ok := csictx.LookupEnv(ctx, constants.EnvEndpoint); ok {
		opts.Endpoint = ep
		opts.EndpointURL = fmt.Sprintf("https://%s:%s", ep, opts.Port)
	}
	if user, ok := csictx.LookupEnv(ctx, constants.EnvUser); ok {
		opts.User = user
	}
	if pw, ok := csictx.LookupEnv(ctx, constants.EnvPassword); ok {
		opts.Password = pw
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

	opts.QuotaEnabled = utils.ParseBooleanFromContext(ctx, constants.EnvQuotaEnabled)
	opts.Insecure = utils.ParseBooleanFromContext(ctx, constants.EnvInsecure)
	opts.AutoProbe = utils.ParseBooleanFromContext(ctx, constants.EnvAutoProbe)
	opts.DebugEnabled = utils.ParseBooleanFromContext(ctx, constants.EnvDebug)
	opts.Verbose = utils.ParseUintFromContext(ctx, constants.EnvVerbose)
	opts.NfsV3 = utils.ParseBooleanFromContext(ctx, constants.EnvNfsV3)
	opts.CustomTopologyEnabled = utils.ParseBooleanFromContext(ctx, constants.EnvCustomTopologyEnabled)

	s.opts = opts

	clientCtx := utils.ConfigureLogger(opts.DebugEnabled)

	s.isiSvc, _ = s.GetIsiService(clientCtx)
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
func (s *service) ValidateDeleteVolumeRequest(
	req *csi.DeleteVolumeRequest) error {

	if req.GetVolumeId() == "" {
		return status.Error(codes.InvalidArgument,
			"no volume id is provided by the DeleteVolumeRequest instance")
	}

	_, _, _, err := utils.ParseNormalizedVolumeID(req.GetVolumeId())
	if err != nil {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("failed to parse volume ID '%s', error : '%v'", req.GetVolumeId(), err))
	}

	return nil
}

func (s *service) probe(ctx context.Context) error {

	log.Debug("calling probe")

	// Do a controller probe
	if strings.EqualFold(s.mode, constants.ModeController) {
		if err := s.controllerProbe(ctx); err != nil {
			return err
		}
	} else if strings.EqualFold(s.mode, constants.ModeNode) {
		if err := s.nodeProbe(ctx); err != nil {
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

	return s.probe(ctx)
}

func (s *service) autoProbe(ctx context.Context) error {

	if s.isiSvc != nil {
		log.Debug("isiSvc already initialized, skip probing")
		return nil
	}

	if !s.opts.AutoProbe {
		return status.Error(codes.FailedPrecondition,
			"isiSvc not initialized, but auto probe is not enabled")
	}

	log.Debug("start auto-probing")
	return s.probe(ctx)
}

func (s *service) GetIsiClient(clientCtx context.Context) (*isi.Client, error) {

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
					s.opts.Endpoint = tList[1]
					s.opts.EndpointURL = fmt.Sprintf("https://%s:%s", s.opts.Endpoint, s.opts.Port)
					customTopologyFound = true
				} else {
					log.Errorf("Fetching PowerScale FQDN/IP from topology label %s:%s failed, using isiIP "+
						"%s as PowerScale FQDN/IP", lkey, lval, s.opts.Endpoint)
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
		s.opts.EndpointURL,
		s.opts.Insecure,
		s.opts.Verbose,
		s.opts.User,
		"",
		s.opts.Password,
		s.opts.Path,
	)
	if err != nil {
		log.Errorf("init client failed : '%s'", err.Error())
		return nil, err
	}

	return client, nil
}

func (s *service) GetIsiService(clientCtx context.Context) (*isiService, error) {
	var isiClient *isi.Client
	var err error
	if isiClient, err = s.GetIsiClient(clientCtx); err != nil {

		return nil, err
	}

	return &isiService{
		endpoint: s.opts.Endpoint,
		client:   isiClient,
	}, nil
}

func (s *service) validateOptsParameters() error {

	if s.opts.User == "" || s.opts.Password == "" || s.opts.Endpoint == "" {

		return fmt.Errorf("invalid isi service parameters, at least one of endpoint, userame and password is empty. endpoint : endpoint '%s', username : '%s'", s.opts.Endpoint, s.opts.User)
	}

	return nil
}

func (s *service) logServiceStats() {

	fields := map[string]interface{}{
		"endpoint":     s.opts.Endpoint,
		"user":         s.opts.User,
		"password":     "",
		"path":         s.opts.Path,
		"insecure":     s.opts.Insecure,
		"autoprobe":    s.opts.AutoProbe,
		"accesspoint":  s.opts.AccessZone,
		"quotaenabled": s.opts.QuotaEnabled,
		"mode":         s.mode,
	}

	if s.opts.Password != "" {
		fields["password"] = "******"
	}

	log.WithFields(fields).Infof("Configured '%s'", constants.PluginName)
}

func (s *service) BeforeServe(
	ctx context.Context, sp *gocsi.StoragePlugin, lis net.Listener) error {
	s.initializeService(ctx)
	s.logServiceStats()
	return s.probeOnStart(ctx)
}

// GetCSINodeID gets the id of the CSI node which regards the node name as node id
func (s *service) GetCSINodeID(ctx context.Context) (string, error) {
	// if the node id has already been initialized, return it
	if s.nodeID != "" {
		return s.nodeID, nil
	}
	// node id couldn't be read from env variable while initializing service, return with error
	return "", errors.New("Cannot get node id")
}

// GetCSINodeIP gets the IP of the CSI node
func (s *service) GetCSINodeIP(ctx context.Context) (string, error) {
	// if the node ip has already been initialized, return it
	if s.nodeIP != "" {
		return s.nodeIP, nil
	}
	// node id couldn't be read from env variable while initializing service, return with error
	return "", errors.New("Cannot get node IP")
}

func (s *service) getVolByName(isiPath, volName string) (isi.Volume, error) {

	// The `GetVolume` API returns a slice of volumes, but when only passing
	// in a volume ID, the response will be just the one volume
	vol, err := s.isiSvc.GetVolume(isiPath, "", volName)
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
