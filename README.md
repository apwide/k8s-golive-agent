# k8s-golive-agent

## Usage

### Run the app locally
Run the controller (make sure your kubeconfig use the correct current context):
```shell
go mod tidy
go run ./cmd/k8s-golive-agent
go run ./cmd/k8s-golive-agent -kubeconfig=~/.kube/config -goliveconfig=./deployments/compose/config/config-template.yaml -v=2
```

### Configuration

[Documentation is here](./DOCKERHUB.md) (used to push documentation on dockerhub)
[Configuration examples are here](./examples/README.md)

### Update Golive API Client
```shell
curl https://golive.apwide.net/public/swagger/json | jq > ./pkg/golive/golive.json
go generate pkg/golive/golive.go
```

### RBAC Required when running in-cluster mode
[Kubernetes permissions required by the controller](./examples/base/rbac.yaml)

## TODO
* Attribute Initialization and/or Auto Creation in SendEnvironmentInfo ?
* Override existing value ? change category of environment ?

## Tips
track api calls done by kubectl, add **--v=6**
```shell
kubectl get pods --v=6
```

check vulnerability:
```shell
# with the correct version of go
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

build:
```shell
# updated dev
go mod tidy
# build
go build -o k8s-golive-agent ./cmd/k8s-golive-agent/main.go
```

instal version of go:
```shell
go install golang.org/dl/go1.25.2@latest
# update $PATH in shell rc file
```

## Information

### Design

* a client to talk to Golive: [./pkg/golive](./pkg/golive)
* a configuration to customize app behavior: [./pkg/k8s/config.go](./pkg/k8s/config.go)
* a controller using configuration to listen some POD events: [./pkg/k8s/controller.go](./pkg/k8s/controller.go)
* a handler using configuration to extract data from Resources and Push them to GOlive: [./pkg/k8s/handler.go](./pkg/k8s/handler.go)
* a cache to store data sent to Golive and avoid sending several time same data (save soldier Golive): [./pkg/cache/cache.go](./pkg/cache/cache.og)

The controller:
* is listening for POD and check if pod match a listener
* when matched, search for pod owner (Deployment, StatefulSet, DaemonSet) and wrap in a MetaResource to simplify data extraction
* use configuration to extract label, annotation, jsonPath, transform them (with template), and use them as app, cat, env name.
* build and send payload if not in cache

Controller is based on [k8s-client boiler plate](https://github.com/kubernetes/client-go/tree/master/examples/workqueue) to build a controller:
* provide watching and cache mechanisms for accessing kube api (get pods/deployments/namespace...) in an effective manner.
* use queue management and push key of resource which must be treated.
* support concurrency

### Error mgt

* panic in case of initialization error: wrong config, golive not reachable...
* log in case of handler error to not stop the controller:
  * in case of temporary error, notify controller of the error to retry it.
  * in case of irrecoverable error, log and consider event as treated to avoid retrying it infinitely.

### API Generation
Api generation made with [openapi-client-generator](https://github.com/deepmap/oapi-codegen).

Use recommended [tools dependencies management](https://github.com/deepmap/oapi-codegen?tab=readme-ov-file#install):
* dependencies on [tools.go](./tools/tools.go) with macro on it
* generate command defined on [golive.go](./pkg/golive/golive.go)

## Resources
### Learning
* https://github.com/ardanlabs/gotraining
* https://github.com/inancgumus/learngo
* https://github.com/quii/learn-go-with-tests
* [go lang project layout](https://github.com/golang-standards/project-layout)
* [patterns](https://github.com/tmrts/go-patterns)
* [type assertions](https://go.dev/ref/spec#Type_assertions) vs [type conversion](https://go.dev/tour/basics/13)

### Libraries
* [io](https://github.com/samber/lo)
* [libraries](https://github.com/avelino/awesome-go)
* [k8s watch](https://blog.mimacom.com/k8s-watch-resources/)
* [openapi-client-generator deepmap](https://github.com/deepmap/oapi-codegen)
* [flag](https://github.com/peterbourgon/ff)

test:
* https://go.dev/wiki/TableDrivenTests
* https://github.com/stretchr/testify
* https://github.com/smartystreets/goconvey
* https://github.com/onsi/ginkgo
* https://github.com/gavv/httpexpect

### How to
* [k8s logging](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md)
* [config map hot reload](https://medium.com/@xcoulon/kubernetes-configmap-hot-reload-in-action-with-viper-d413128a1c9a)
