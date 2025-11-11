module github.com/argoproj/argo-workflows/v3/examples/go-sdk/grpc-client

go 1.21

replace github.com/argoproj/argo-workflows/v3 => ../../..

require (
	github.com/argoproj/argo-workflows/v3 v3.0.0-00010101000000-000000000000
	k8s.io/api v0.33.1
	k8s.io/apimachinery v0.33.1
)
