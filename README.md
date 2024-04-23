# K8S
https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-conditions
https://kubebyexample.com/learning-paths/application-development-kubernetes/lesson-1-running-containerized-applications-1
https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md
https://github.com/golang-standards/project-layout
https://medium.com/@xcoulon/kubernetes-configmap-hot-reload-in-action-with-viper-d413128a1c9a

track api calls by kubectl
```shell
kubectl get pods --v=6
```

```shell
# go mod tidy
go generate
GOLIVE_CONFIG=./deployments/compose/config/config-local.yaml go run ./cmd/k8s-monitor -v=6
```

# Stack

https://blog.mimacom.com/k8s-watch-resources/

https://go.dev/doc/tutorial/
https://go.dev/tour/list
https://www.alexedwards.net/blog/an-introduction-to-packages-imports-and-modules
https://betterstack.com/community/guides/logging/logging-in-go/
https://dev.to/ankit01oss/7-github-projects-to-make-you-a-better-go-developer-2nmh

private repo access: https://www.digitalocean.com/community/tutorials/how-to-use-a-private-go-module-in-your-own-project

next => docker + incluster config (k3d + local registry with https://k3d.io/v5.2.0/usage/registries/)

## Testing
https://go.dev/wiki/TableDrivenTests
https://github.com/stretchr/testify
https://github.com/smartystreets/goconvey
https://github.com/onsi/ginkgo
https://github.com/gavv/httpexpect

## Tools
* [k8s watch](https://blog.mimacom.com/k8s-watch-resources/)
* [yaml](https://pkg.go.dev/gopkg.in/yaml.v3?utm_source=godoc)
  https://github.com/samber/lo

# Golive Client Generation

* [openapi-client-generator deepmap](https://github.com/deepmap/oapi-codegen)
* [doc-openapi-generator](https://medium.com/@kyodo-tech/go-client-code-generation-from-swagger-and-openapi-a0576831836c)
  https://blog.carlana.net/post/2016-11-27-how-to-use-go-generate/

## Procedure
* go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@latest
* oapi-codegen -config golive-gen.yaml tem-api.yaml
* go mod tidy


# Configuration

https://github.com/peterbourgon/ff
https://dev.to/ilyakaznacheev/a-clean-way-to-pass-configs-in-a-go-application-1g64
https://pkg.go.dev/sigs.k8s.io/yaml#section-readme

# RBAC ?

GET on Deployment/StatefulSet/Pod

# Status Monitoring

# Helm Chart or Native Operator

Build (inside [helm chart](./charts/k8s-monitor)):
```shell
helm package .
```

Install:

create a values file in *local/values.yaml* based on [template values](./charts/k8s-monitor/values.yaml).
run
```shell
helm -n golive-dev install golive-monitor ./k8s-monitor-0.1.0.tgz --values ./local/values.yaml
```

# Others
https://pkg.go.dev/helm.sh/helm/v3@v3.12.0/pkg/release#HookEvent
https://pkg.go.dev/helm.sh/helm/v3@v3.12.0/pkg/release

https://gist.github.com/PrasadG193/589975a55ed992a7b138a53c3d0d1836
SliceFromJSON // https://dev.to/plutov/writing-rest-api-client-in-go-3fkg
https://blog.mimacom.com/k8s-watch-resources/


# TODO
String Template on Version => https://blog.logrocket.com/using-golang-templates/
Attribute Auto Creation ?
Status Mapping Strategy => based on replica status ?
Override existing value ? change category of environment ?
attribute mapping
filter events in effective manner
cache golive data to avoid requesting all the time ? => load on our cluster !!!! Event mode (30 seconds cache)


# Golang
Type assertions: https://go.dev/ref/spec#Type_assertions
Type conversion: https://go.dev/tour/basics/13


# Pattern
Conversion type k8s
```go
deployment := &appsv1.Deployment{}
err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), deployment)
```
