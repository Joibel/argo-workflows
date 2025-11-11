# Client Libraries

This page contains an overview of the client libraries for using the Argo API from various programming languages.

To write applications using the REST API, you do not need to implement the API calls and request/response types
yourself. You can use a client library for the programming language you are using.

Client libraries often handle common tasks such as authentication for you.

## Officially Supported Client Libraries

### Go SDK

The Go SDK is a fully-featured, officially supported client for Argo Workflows. It provides two client approaches:
- **Kubernetes Client** - Direct CRD access for in-cluster applications
- **Argo Server Client** - gRPC/HTTP access for remote applications

**Getting Started:**
```bash
go get github.com/argoproj/argo-workflows/v3@latest
```

**Quick Example:**
```go
import (
    "context"
    wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
    wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
)

clientset := wfclientset.NewForConfigOrDie(config)
wfClient := clientset.ArgoprojV1alpha1().Workflows("default")
workflow, err := wfClient.Create(ctx, &myWorkflow, metav1.CreateOptions{})
```

**Documentation:**
- [Go SDK Guide](./go-sdk-guide.md) - Comprehensive documentation
- [Migration Guide](./go-sdk-migration-guide.md) - Migrating to v3.7+
- [Examples](../examples/go-sdk/) - Working code examples
- [API Reference](https://pkg.go.dev/github.com/argoproj/argo-workflows/v3)

**Key Features:**
- ✓ Full CRUD operations for all resource types
- ✓ Workflow watching and listing with field selectors
- ✓ Remote Argo Server access via gRPC/HTTP
- ✓ In-cluster and kubeconfig authentication
- ✓ Context-aware operations (since v3.7)
- ✓ Comprehensive type definitions with validation

## Community-Supported Client Libraries

The following client libraries are community-maintained with minimal support from the Argo team.

| Language | Client Library                                                                                    | Examples/Docs                                                                                                         |
|----------|---------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------|
| Java     | [Java](https://github.com/argoproj/argo-workflows/blob/main/sdks/java)                            | Auto-generated using OpenAPI Generator                                                                                                       |
| Python   | ⚠️ deprecated [Python](https://github.com/argoproj/argo-workflows/blob/main/sdks/python)           | Use [Hera](#hera-python-sdk) instead. Will be removed in version 3.7 |

## Hera Python SDK

Hera is the go-to Python SDK to make Argo Workflows simple and intuitive. It goes beyond a basic REST interface,
allowing you to easily turn Python functions into script templates, and write whole Workflows in Python:

```py
from hera.workflows import DAG, Workflow, script


@script()
def echo(message: str):
    print(message)


with Workflow(
    generate_name="dag-diamond-",
    entrypoint="diamond",
) as w:
    with DAG(name="diamond"):
        A = echo(name="A", arguments={"message": "A"})
        B = echo(name="B", arguments={"message": "B"})
        C = echo(name="C", arguments={"message": "C"})
        D = echo(name="D", arguments={"message": "D"})
        A >> [B, C] >> D  # Define execution order

w.create()
```

Learn more in the [Hera walk-through](https://hera.readthedocs.io/en/stable/walk-through/quick-start/).
