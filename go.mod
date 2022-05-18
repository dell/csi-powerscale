module github.com/dell/csi-isilon

require (
	github.com/Showmax/go-fqdn v1.0.0
	github.com/akutz/gournal v0.5.0
	github.com/container-storage-interface/spec v1.5.0
	github.com/cucumber/godog v0.10.0
	github.com/dell/dell-csi-extensions/common v1.0.0
	github.com/dell/dell-csi-extensions/podmon v0.1.0
	github.com/dell/dell-csi-extensions/replication v1.1.0
	github.com/dell/dell-csi-extensions/volumeGroupSnapshot v1.1.0
	github.com/dell/gocsi v1.5.2-0.20220512024127-437bfc6fe1ad
	github.com/dell/gofsutil v1.8.0
	github.com/dell/goisilon v1.7.1-0.20220518061426-9b49506e1dd2
	github.com/fsnotify/fsnotify v1.4.9
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.2.0
	github.com/gorilla/mux v1.8.0
	github.com/kubernetes-csi/csi-lib-utils v0.9.1
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	golang.org/x/net v0.0.0-20211015210444-4f30a5c0130f
	google.golang.org/grpc v1.43.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2

)

require (
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	golang.org/x/text v0.3.7 // indirect
)

go 1.17
