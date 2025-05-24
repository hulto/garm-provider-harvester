module garm-provider-harvester

go 1.22.2

require (
	github.com/cloudbase/garm-provider-common v0.1.4
	github.com/harvester/harvester-sdk-go v0.2.0 // Required for go mod tidy to see the import
	k8s.io/client-go v0.26.1
)

replace (
	// Replace with local mock SDK due to fetching issues
	github.com/harvester/harvester-sdk-go => ./internal/harvester/mock_sdk
	k8s.io/api => k8s.io/api v0.26.1
	k8s.io/apimachinery => k8s.io/apimachinery v0.26.1
	k8s.io/client-go => k8s.io/client-go v0.26.1
	k8s.io/component-base => k8s.io/component-base v0.26.1
)

// Adding indirect dependencies that might cause issues if not pinned
// These versions are typically compatible with k8s v0.26.x
require (
	github.com/go-logr/logr v1.2.3 // indirect; Common logger
	github.com/gogo/protobuf v1.3.2 // indirect; Used by k8s
	github.com/google/gofuzz v1.1.0 // indirect; Used by k8s
	github.com/json-iterator/go v1.1.12 // indirect; Used by k8s
	github.com/spf13/pflag v1.0.5 // indirect; Used by k8s
	golang.org/x/net v0.28.0 // indirect; Using a version compatible with go 1.22
	golang.org/x/oauth2 v0.10.0 // indirect; Using a version compatible with go 1.22
	golang.org/x/sys v0.24.0 // indirect; Using a version compatible with go 1.22
	golang.org/x/term v0.23.0 // indirect; Using a version compatible with go 1.22
	golang.org/x/text v0.17.0 // indirect; Using a version compatible with go 1.22
	google.golang.org/protobuf v1.31.0 // indirect; Common protobuf
	gopkg.in/yaml.v2 v2.4.0 // indirect; Common yaml
	k8s.io/klog/v2 v2.80.1 // indirect; Used by k8s
	k8s.io/utils v0.0.0-20221128185143-99ec85e7a448 // indirect; Used by k8s
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect; Used by controller-runtime
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect; Used by k8s
	sigs.k8s.io/yaml v1.3.0 // indirect; Used by controller-runtime
)

require (
	github.com/BurntSushi/toml v1.5.0
	github.com/google/uuid v1.6.0
	github.com/stretchr/testify v1.9.0
	k8s.io/apimachinery v0.26.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/gorilla/handlers v1.5.2 // indirect
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/minio/sio v0.4.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/teris-io/shortid v0.0.0-20220617161101-71ec9f2aa569 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
