# Argo Workflows Go SDK Guide

The Argo Workflows Go SDK allows you to interact with Argo Workflows programmatically from Go applications. This guide covers installation, authentication, and common usage patterns.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Client Architecture](#client-architecture)
- [Authentication](#authentication)
- [Common Operations](#common-operations)
- [Working with Workflow Templates](#working-with-workflow-templates)
- [Advanced Topics](#advanced-topics)
- [Best Practices](#best-practices)
- [Examples](#examples)

## Installation

Add the Argo Workflows SDK to your Go project:

```bash
go get github.com/argoproj/argo-workflows/v3@latest
```

### Minimum Requirements

- Go 1.21 or later
- Kubernetes 1.28+ (if using Kubernetes client)
- Argo Workflows 3.4+ installed in your cluster

## Quick Start

Here's a simple example that submits a workflow:

```go
package main

import (
    "context"
    "fmt"

    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/tools/clientcmd"

    wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
    wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
)

func main() {
    ctx := context.Background()

    // Load kubeconfig
    config, err := clientcmd.BuildConfigFromFlags("", "/path/to/.kube/config")
    if err != nil {
        panic(err)
    }

    // Create workflow client
    wfClient := wfclientset.NewForConfigOrDie(config).
        ArgoprojV1alpha1().
        Workflows("default")

    // Define a simple workflow
    workflow := &wfv1.Workflow{
        ObjectMeta: metav1.ObjectMeta{
            GenerateName: "hello-world-",
        },
        Spec: wfv1.WorkflowSpec{
            Entrypoint: "hello",
            Templates: []wfv1.Template{
                {
                    Name: "hello",
                    Container: &corev1.Container{
                        Image:   "busybox",
                        Command: []string{"echo", "hello world"},
                    },
                },
            },
        },
    }

    // Submit workflow
    created, err := wfClient.Create(ctx, workflow, metav1.CreateOptions{})
    if err != nil {
        panic(err)
    }

    fmt.Printf("Workflow %s submitted successfully\n", created.Name)
}
```

## Client Architecture

The Argo Workflows Go SDK provides two different client approaches for different use cases:

### 1. Kubernetes Client (Direct CRD Access)

**Use when:**
- Running inside a Kubernetes cluster
- You have kubeconfig access
- You want native Kubernetes API patterns
- You need watch/list operations with field selectors

**Package:** `github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned`

```go
import (
    wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
    "k8s.io/client-go/tools/clientcmd"
)

// From kubeconfig
config, _ := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
clientset := wfclientset.NewForConfigOrDie(config)
wfClient := clientset.ArgoprojV1alpha1().Workflows(namespace)

// From in-cluster config
config, _ := rest.InClusterConfig()
clientset := wfclientset.NewForConfigOrDie(config)
```

### 2. Argo Server Client (gRPC/HTTP)

**Use when:**
- Accessing Argo Server remotely
- You don't have direct Kubernetes access
- You need service-oriented operations (retry, stop, suspend)
- Working with archived workflows

**Package:** `github.com/argoproj/argo-workflows/v3/pkg/apiclient`

```go
import (
    "github.com/argoproj/argo-workflows/v3/pkg/apiclient"
)

ctx, client, err := apiclient.NewClientFromOptsWithContext(ctx, apiclient.Opts{
    ArgoServerOpts: apiclient.ArgoServerOpts{
        URL: "localhost:2746",
    },
    AuthSupplier: func() string { return bearerToken },
})
if err != nil {
    panic(err)
}

serviceClient := client.NewWorkflowServiceClient(ctx)
```

### Comparison

| Feature | Kubernetes Client | Argo Server Client |
|---------|-------------------|-------------------|
| **Access Method** | Direct K8s API | gRPC/HTTP |
| **Authentication** | Kubeconfig/ServiceAccount | Bearer token/SSO |
| **Network** | Cluster access required | Remote access supported |
| **Operations** | CRUD, Watch, Patch | CRUD + Retry/Stop/Suspend |
| **Archived Workflows** | No | Yes |
| **Field Selectors** | Yes | Limited |
| **In-Cluster** | Optimal | Possible |

## Authentication

### Kubernetes Client Authentication

#### Using Kubeconfig

```go
import (
    "k8s.io/client-go/tools/clientcmd"
    wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
)

// Default kubeconfig location
config, err := clientcmd.BuildConfigFromFlags("",
    filepath.Join(os.Getenv("HOME"), ".kube", "config"))

// Custom kubeconfig location
config, err := clientcmd.BuildConfigFromFlags("", "/custom/path/to/kubeconfig")

// Create clientset
clientset := wfclientset.NewForConfig(config)
```

#### Using In-Cluster Config (for Pods)

```go
import (
    "k8s.io/client-go/rest"
    wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
)

config, err := rest.InClusterConfig()
if err != nil {
    panic(err)
}

clientset := wfclientset.NewForConfig(config)
```

#### Using Service Account

When running in a pod, ensure your ServiceAccount has appropriate RBAC permissions:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: workflow-client
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: workflow-client-role
  namespace: default
rules:
- apiGroups: ["argoproj.io"]
  resources: ["workflows", "workflowtemplates", "cronworkflows"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: workflow-client-binding
  namespace: default
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: workflow-client-role
subjects:
- kind: ServiceAccount
  name: workflow-client
  namespace: default
```

### Argo Server Client Authentication

#### Using Bearer Token

```go
import (
    "os"
    "github.com/argoproj/argo-workflows/v3/pkg/apiclient"
)

ctx, client, err := apiclient.NewClientFromOptsWithContext(ctx, apiclient.Opts{
    ArgoServerOpts: apiclient.ArgoServerOpts{
        URL:    "localhost:2746",
        Secure: true, // Use TLS
    },
    AuthSupplier: func() string {
        return os.Getenv("ARGO_TOKEN")
    },
})
```

#### Using Kubeconfig (Argo Server in Kubernetes mode)

```go
import (
    "k8s.io/client-go/tools/clientcmd"
    "github.com/argoproj/argo-workflows/v3/pkg/apiclient"
)

ctx, client, err := apiclient.NewClientFromOptsWithContext(ctx, apiclient.Opts{
    ArgoServerOpts: apiclient.ArgoServerOpts{
        URL: "localhost:2746",
    },
    ClientConfigSupplier: func() clientcmd.ClientConfig {
        loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
        return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
            loadingRules,
            &clientcmd.ConfigOverrides{},
        )
    },
})
```

#### Insecure Mode (Development Only)

```go
ctx, client, err := apiclient.NewClientFromOptsWithContext(ctx, apiclient.Opts{
    ArgoServerOpts: apiclient.ArgoServerOpts{
        URL:                "localhost:2746",
        Secure:             false,
        InsecureSkipVerify: true, // Skip TLS verification
    },
    AuthSupplier: func() string { return token },
})
```

## Common Operations

### Creating Workflows

#### From Struct

```go
workflow := &wfv1.Workflow{
    ObjectMeta: metav1.ObjectMeta{
        GenerateName: "my-workflow-",
        Labels: map[string]string{
            "app": "my-app",
        },
    },
    Spec: wfv1.WorkflowSpec{
        Entrypoint: "main",
        Templates: []wfv1.Template{
            {
                Name: "main",
                Container: &corev1.Container{
                    Image:   "alpine:latest",
                    Command: []string{"sh", "-c"},
                    Args:    []string{"echo hello"},
                },
            },
        },
    },
}

created, err := wfClient.Create(ctx, workflow, metav1.CreateOptions{})
```

#### From YAML

```go
import (
    "os"
    "sigs.k8s.io/yaml"
)

// Read YAML file
data, err := os.ReadFile("workflow.yaml")
if err != nil {
    panic(err)
}

// Unmarshal into Workflow
var workflow wfv1.Workflow
if err := yaml.Unmarshal(data, &workflow); err != nil {
    panic(err)
}

// Submit
created, err := wfClient.Create(ctx, &workflow, metav1.CreateOptions{})
```

### Listing Workflows

```go
// List all workflows in namespace
list, err := wfClient.List(ctx, metav1.ListOptions{})
if err != nil {
    panic(err)
}

for _, wf := range list.Items {
    fmt.Printf("Workflow: %s, Phase: %s\n", wf.Name, wf.Status.Phase)
}

// List with label selector
list, err = wfClient.List(ctx, metav1.ListOptions{
    LabelSelector: "app=my-app",
})

// List with field selector
list, err = wfClient.List(ctx, metav1.ListOptions{
    FieldSelector: "status.phase=Running",
})
```

### Getting Workflow Details

```go
wf, err := wfClient.Get(ctx, "workflow-name", metav1.GetOptions{})
if err != nil {
    panic(err)
}

fmt.Printf("Name: %s\n", wf.Name)
fmt.Printf("Phase: %s\n", wf.Status.Phase)
fmt.Printf("Started: %s\n", wf.Status.StartedAt)
fmt.Printf("Finished: %s\n", wf.Status.FinishedAt)

// Access node statuses
for nodeName, nodeStatus := range wf.Status.Nodes {
    fmt.Printf("Node %s: %s\n", nodeName, nodeStatus.Phase)
}
```

### Watching Workflows

```go
import (
    "k8s.io/apimachinery/pkg/fields"
    "k8s.io/apimachinery/pkg/watch"
)

// Watch specific workflow
fieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", workflowName))
watchIf, err := wfClient.Watch(ctx, metav1.ListOptions{
    FieldSelector: fieldSelector.String(),
})
if err != nil {
    panic(err)
}
defer watchIf.Stop()

// Process events
for {
    select {
    case <-ctx.Done():
        return ctx.Err()
    case event, ok := <-watchIf.ResultChan():
        if !ok {
            return nil // Watch closed
        }

        wf, ok := event.Object.(*wfv1.Workflow)
        if !ok {
            continue
        }

        switch event.Type {
        case watch.Added:
            fmt.Printf("Workflow added: %s\n", wf.Name)
        case watch.Modified:
            fmt.Printf("Workflow %s: %s\n", wf.Name, wf.Status.Phase)
        case watch.Deleted:
            fmt.Printf("Workflow deleted: %s\n", wf.Name)
        }

        // Check if finished
        if !wf.Status.FinishedAt.IsZero() {
            fmt.Printf("Workflow completed: %s\n", wf.Status.Phase)
            return nil
        }
    }
}
```

### Deleting Workflows

```go
// Delete single workflow
err := wfClient.Delete(ctx, "workflow-name", metav1.DeleteOptions{})

// Delete with propagation policy
err = wfClient.Delete(ctx, "workflow-name", metav1.DeleteOptions{
    PropagationPolicy: &deletePropagationBackground,
})

// Delete collection (multiple workflows)
err = wfClient.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
    LabelSelector: "app=my-app",
})
```

### Using Argo Server Client

```go
import (
    workflowpkg "github.com/argoproj/argo-workflows/v3/pkg/apiclient/workflow"
)

// Create service client
serviceClient := client.NewWorkflowServiceClient(ctx)

// Create workflow
created, err := serviceClient.CreateWorkflow(ctx, &workflowpkg.WorkflowCreateRequest{
    Namespace: "default",
    Workflow:  &workflow,
})

// List workflows
list, err := serviceClient.ListWorkflows(ctx, &workflowpkg.WorkflowListRequest{
    Namespace: "default",
})

// Get workflow
wf, err := serviceClient.GetWorkflow(ctx, &workflowpkg.WorkflowGetRequest{
    Namespace: "default",
    Name:      "workflow-name",
})

// Retry workflow
updated, err := serviceClient.RetryWorkflow(ctx, &workflowpkg.WorkflowRetryRequest{
    Namespace: "default",
    Name:      "workflow-name",
})

// Stop workflow
updated, err = serviceClient.StopWorkflow(ctx, &workflowpkg.WorkflowStopRequest{
    Namespace: "default",
    Name:      "workflow-name",
})

// Suspend workflow
updated, err = serviceClient.SuspendWorkflow(ctx, &workflowpkg.WorkflowSuspendRequest{
    Namespace: "default",
    Name:      "workflow-name",
})

// Resume workflow
updated, err = serviceClient.ResumeWorkflow(ctx, &workflowpkg.WorkflowResumeRequest{
    Namespace: "default",
    Name:      "workflow-name",
})
```

## Working with Workflow Templates

### Creating WorkflowTemplates

```go
wftClient := clientset.ArgoprojV1alpha1().WorkflowTemplates(namespace)

template := &wfv1.WorkflowTemplate{
    ObjectMeta: metav1.ObjectMeta{
        Name: "my-template",
    },
    Spec: wfv1.WorkflowSpec{
        Entrypoint: "main",
        Templates: []wfv1.Template{
            {
                Name: "main",
                Container: &corev1.Container{
                    Image:   "alpine:latest",
                    Command: []string{"sh", "-c"},
                    Args:    []string{"echo hello from template"},
                },
            },
        },
    },
}

created, err := wftClient.Create(ctx, template, metav1.CreateOptions{})
```

### Submitting from WorkflowTemplate

```go
// Reference a WorkflowTemplate
workflow := &wfv1.Workflow{
    ObjectMeta: metav1.ObjectMeta{
        GenerateName: "from-template-",
    },
    Spec: wfv1.WorkflowSpec{
        WorkflowTemplateRef: &wfv1.WorkflowTemplateRef{
            Name: "my-template",
        },
    },
}

created, err := wfClient.Create(ctx, workflow, metav1.CreateOptions{})
```

### Working with CronWorkflows

```go
cronClient := clientset.ArgoprojV1alpha1().CronWorkflows(namespace)

cronWf := &wfv1.CronWorkflow{
    ObjectMeta: metav1.ObjectMeta{
        Name: "my-cron-workflow",
    },
    Spec: wfv1.CronWorkflowSpec{
        Schedule: "*/5 * * * *", // Every 5 minutes
        WorkflowSpec: wfv1.WorkflowSpec{
            Entrypoint: "main",
            Templates: []wfv1.Template{
                {
                    Name: "main",
                    Container: &corev1.Container{
                        Image:   "alpine:latest",
                        Command: []string{"date"},
                    },
                },
            },
        },
    },
}

created, err := cronClient.Create(ctx, cronWf, metav1.CreateOptions{})
```

## Advanced Topics

### Using Informers for Event-Driven Applications

Informers provide efficient caching and watching of resources:

```go
import (
    "k8s.io/client-go/tools/cache"
    wfinformers "github.com/argoproj/argo-workflows/v3/pkg/client/informers/externalversions"
)

// Create informer factory
informerFactory := wfinformers.NewSharedInformerFactory(clientset, 0)
wfInformer := informerFactory.Argoproj().V1alpha1().Workflows()

// Add event handlers
wfInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
        wf := obj.(*wfv1.Workflow)
        fmt.Printf("Workflow added: %s\n", wf.Name)
    },
    UpdateFunc: func(oldObj, newObj interface{}) {
        wf := newObj.(*wfv1.Workflow)
        fmt.Printf("Workflow updated: %s, phase: %s\n", wf.Name, wf.Status.Phase)
    },
    DeleteFunc: func(obj interface{}) {
        wf := obj.(*wfv1.Workflow)
        fmt.Printf("Workflow deleted: %s\n", wf.Name)
    },
})

// Start informer
stopCh := make(chan struct{})
defer close(stopCh)
informerFactory.Start(stopCh)
informerFactory.WaitForCacheSync(stopCh)

// Keep running
<-stopCh
```

### Using Listers for Efficient Querying

```go
import (
    wflisters "github.com/argoproj/argo-workflows/v3/pkg/client/listers/workflow/v1alpha1"
)

// Create lister from informer
lister := wfInformer.Lister()

// List workflows (from cache)
workflows, err := lister.Workflows(namespace).List(labels.Everything())

// Get specific workflow (from cache)
wf, err := lister.Workflows(namespace).Get("workflow-name")
```

### Testing with Fake Clients

```go
import (
    fakewfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/fake"
)

// Create fake clientset for testing
fakeClient := fakewfclientset.NewSimpleClientset()
wfClient := fakeClient.ArgoprojV1alpha1().Workflows(namespace)

// Use as normal
created, err := wfClient.Create(ctx, &workflow, metav1.CreateOptions{})
```

## Best Practices

### 1. Always Use Context

Pass context through your application for cancellation and timeout control:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

wf, err := wfClient.Create(ctx, &workflow, metav1.CreateOptions{})
```

### 2. Handle Errors Appropriately

```go
import (
    apierrors "k8s.io/apimachinery/pkg/api/errors"
)

wf, err := wfClient.Get(ctx, name, metav1.GetOptions{})
if err != nil {
    if apierrors.IsNotFound(err) {
        // Workflow doesn't exist
        fmt.Printf("Workflow %s not found\n", name)
    } else {
        // Other error
        return fmt.Errorf("failed to get workflow: %w", err)
    }
}
```

### 3. Use GenerateName for Unique Workflows

```go
workflow := &wfv1.Workflow{
    ObjectMeta: metav1.ObjectMeta{
        GenerateName: "my-workflow-", // Will generate my-workflow-xyz123
    },
    // ...
}
```

### 4. Set Resource Limits

```go
Template: wfv1.Template{
    Container: &corev1.Container{
        Resources: corev1.ResourceRequirements{
            Requests: corev1.ResourceList{
                corev1.ResourceMemory: resource.MustParse("100Mi"),
                corev1.ResourceCPU:    resource.MustParse("100m"),
            },
            Limits: corev1.ResourceList{
                corev1.ResourceMemory: resource.MustParse("200Mi"),
                corev1.ResourceCPU:    resource.MustParse("200m"),
            },
        },
    },
}
```

### 5. Use Structured Logging

```go
import (
    "github.com/argoproj/argo-workflows/v3/util/logging"
)

ctx := logging.WithLogger(context.Background(), logger)
// SDK will automatically use logger from context
```

## Examples

See the [`examples/go-sdk/`](../examples/go-sdk/) directory for complete, working examples:

- **basic-workflow/** - Simple workflow submission
- **grpc-client/** - Using Argo Server gRPC client
- **watch-workflow/** - Watching workflow progress
- **workflow-template/** - Working with templates
- **authentication/** - Different auth methods

## Additional Resources

- [Migration Guide](./go-sdk-migration-guide.md) - Migrating from older versions
- [API Reference](https://pkg.go.dev/github.com/argoproj/argo-workflows/v3)
- [Workflow Examples](../examples/) - 270+ YAML examples
- [Argo Workflows Documentation](https://argo-workflows.readthedocs.io/)

## Getting Help

- [Slack Channel](https://argoproj.github.io/community/join-slack)
- [GitHub Issues](https://github.com/argoproj/argo-workflows/issues)
- [GitHub Discussions](https://github.com/argoproj/argo-workflows/discussions)
