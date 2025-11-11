# Go SDK Migration Guide

## Migrating to v3.7+

This guide helps you migrate your Go code that uses the Argo Workflows SDK from versions prior to v3.7 to v3.7 and later.

## Overview of Changes

Starting with v3.7, the Argo Workflows Go SDK has been modernized to follow Go best practices by requiring `context.Context` parameters in most API calls. This change brings:

- **Better cancellation support**: Cancel long-running operations when needed
- **Timeout control**: Set deadlines for operations
- **Request tracing**: Pass trace IDs and metadata through context
- **Structured logging**: Context-aware logging throughout the SDK

## Breaking Changes

### 1. Context Parameters Required

Most SDK functions now require a `context.Context` as the first parameter.

#### Kubernetes Client (Clientset)

**Before (v3.6 and earlier):**
```go
import (
    wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
    wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Create workflow
wf, err := wfClient.Create(&workflow, metav1.CreateOptions{})

// Get workflow
wf, err := wfClient.Get(name, metav1.GetOptions{})

// List workflows
list, err := wfClient.List(metav1.ListOptions{})

// Watch workflows
watchIf, err := wfClient.Watch(metav1.ListOptions{})

// Delete workflow
err := wfClient.Delete(name, metav1.DeleteOptions{})
```

**After (v3.7+):**
```go
import (
    "context"
    wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
    wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

ctx := context.Background()

// Create workflow
wf, err := wfClient.Create(ctx, &workflow, metav1.CreateOptions{})

// Get workflow
wf, err := wfClient.Get(ctx, name, metav1.GetOptions{})

// List workflows
list, err := wfClient.List(ctx, metav1.ListOptions{})

// Watch workflows
watchIf, err := wfClient.Watch(ctx, metav1.ListOptions{})

// Delete workflow
err := wfClient.Delete(ctx, name, metav1.DeleteOptions{})
```

#### Argo Server Client (gRPC/HTTP)

The Argo Server client already used context in some places, but now it's more consistent.

**Before (v3.6 and earlier):**
```go
import (
    "github.com/argoproj/argo-workflows/v3/pkg/apiclient"
)

// Create client
ctx, client, err := apiclient.NewClientFromOpts(opts)
serviceClient := client.NewWorkflowServiceClient()
```

**After (v3.7+):**
```go
import (
    "context"
    "github.com/argoproj/argo-workflows/v3/pkg/apiclient"
)

// Create client
ctx, client, err := apiclient.NewClientFromOptsWithContext(context.Background(), opts)
serviceClient := client.NewWorkflowServiceClient(ctx)
```

### 2. Deprecated Functions

The following functions are deprecated in favor of context-aware versions:

| Deprecated | Replacement |
|------------|-------------|
| `apiclient.NewClient()` | `apiclient.NewClientFromOptsWithContext()` |
| `apiclient.NewClientFromOpts()` | `apiclient.NewClientFromOptsWithContext()` |

## Migration Strategy

### Step 1: Add Context Import

Add the `context` package to your imports:

```go
import (
    "context"
    // ... other imports
)
```

### Step 2: Create a Context

At the entry point of your application or request handler, create a context:

```go
// Simple background context
ctx := context.Background()

// Context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Context with cancellation
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
```

### Step 3: Pass Context to SDK Calls

Add `ctx` as the first parameter to all SDK function calls:

```go
// Before
wf, err := wfClient.Create(&workflow, metav1.CreateOptions{})

// After
wf, err := wfClient.Create(ctx, &workflow, metav1.CreateOptions{})
```

### Step 4: Update Error Handling

Some error handling utilities now accept context:

```go
// Before
errors.CheckError(err)

// After
errors.CheckError(ctx, err)
```

## Best Practices

### 1. Use Appropriate Context Types

Choose the right context for your use case:

```go
// Background context for main operations
ctx := context.Background()

// HTTP request context (in web handlers)
ctx := r.Context()

// Context with timeout for operations that should not hang
ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
defer cancel()

// Context with deadline for specific time limits
deadline := time.Now().Add(5 * time.Minute)
ctx, cancel := context.WithDeadline(context.Background(), deadline)
defer cancel()

// Context with cancellation for user-initiated stops
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
// Call cancel() when you want to stop the operation
```

### 2. Propagate Context Through Your Application

Pass the context through your application layers:

```go
func submitWorkflow(ctx context.Context, wfClient v1alpha1.WorkflowInterface, wf *wfv1.Workflow) error {
    created, err := wfClient.Create(ctx, wf, metav1.CreateOptions{})
    if err != nil {
        return err
    }

    return watchWorkflow(ctx, wfClient, created.Name)
}

func watchWorkflow(ctx context.Context, wfClient v1alpha1.WorkflowInterface, name string) error {
    fieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", name))
    watchIf, err := wfClient.Watch(ctx, metav1.ListOptions{
        FieldSelector: fieldSelector.String(),
    })
    if err != nil {
        return err
    }
    defer watchIf.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case event := <-watchIf.ResultChan():
            // Process event
        }
    }
}
```

### 3. Handle Context Cancellation

Always check for context cancellation in long-running operations:

```go
for {
    select {
    case <-ctx.Done():
        // Context was cancelled or timed out
        return ctx.Err()
    case event := <-watchIf.ResultChan():
        // Process event
    }
}
```

### 4. Use Structured Logging

The SDK now supports context-aware logging:

```go
import (
    "github.com/argoproj/argo-workflows/v3/util/logging"
)

// Create context with logger
ctx := logging.WithLogger(context.Background(), logger)

// SDK operations will automatically use the logger from context
wf, err := wfClient.Create(ctx, &workflow, metav1.CreateOptions{})
```

## Common Migration Patterns

### Pattern 1: Simple Workflow Submission

**Before:**
```go
func main() {
    config, _ := clientcmd.BuildConfigFromFlags("", kubeconfig)
    wfClient := wfclientset.NewForConfigOrDie(config).ArgoprojV1alpha1().Workflows("default")

    createdWf, err := wfClient.Create(&workflow, metav1.CreateOptions{})
    if err != nil {
        panic(err)
    }
    fmt.Printf("Created workflow: %s\n", createdWf.Name)
}
```

**After:**
```go
func main() {
    ctx := context.Background()

    config, _ := clientcmd.BuildConfigFromFlags("", kubeconfig)
    wfClient := wfclientset.NewForConfigOrDie(config).ArgoprojV1alpha1().Workflows("default")

    createdWf, err := wfClient.Create(ctx, &workflow, metav1.CreateOptions{})
    if err != nil {
        panic(err)
    }
    fmt.Printf("Created workflow: %s\n", createdWf.Name)
}
```

### Pattern 2: Watching Workflows

**Before:**
```go
func watchWorkflows(wfClient v1alpha1.WorkflowInterface) error {
    watchIf, err := wfClient.Watch(metav1.ListOptions{})
    if err != nil {
        return err
    }
    defer watchIf.Stop()

    for event := range watchIf.ResultChan() {
        wf := event.Object.(*wfv1.Workflow)
        fmt.Printf("Workflow %s is %s\n", wf.Name, wf.Status.Phase)
    }
    return nil
}
```

**After:**
```go
func watchWorkflows(ctx context.Context, wfClient v1alpha1.WorkflowInterface) error {
    watchIf, err := wfClient.Watch(ctx, metav1.ListOptions{})
    if err != nil {
        return err
    }
    defer watchIf.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case event, ok := <-watchIf.ResultChan():
            if !ok {
                return nil
            }
            wf := event.Object.(*wfv1.Workflow)
            fmt.Printf("Workflow %s is %s\n", wf.Name, wf.Status.Phase)
        }
    }
}
```

### Pattern 3: Using Argo Server Client

**Before:**
```go
func submitToArgoServer() error {
    ctx, client, err := apiclient.NewClientFromOpts(apiclient.Opts{
        ArgoServerOpts: apiclient.ArgoServerOpts{URL: "localhost:2746"},
        AuthSupplier:   func() string { return token },
    })
    if err != nil {
        return err
    }

    serviceClient := client.NewWorkflowServiceClient()
    created, err := serviceClient.CreateWorkflow(ctx, &workflowpkg.WorkflowCreateRequest{
        Namespace: "default",
        Workflow:  &workflow,
    })
    return err
}
```

**After:**
```go
func submitToArgoServer(ctx context.Context) error {
    ctx, client, err := apiclient.NewClientFromOptsWithContext(ctx, apiclient.Opts{
        ArgoServerOpts: apiclient.ArgoServerOpts{URL: "localhost:2746"},
        AuthSupplier:   func() string { return token },
    })
    if err != nil {
        return err
    }

    serviceClient := client.NewWorkflowServiceClient(ctx)
    created, err := serviceClient.CreateWorkflow(ctx, &workflowpkg.WorkflowCreateRequest{
        Namespace: "default",
        Workflow:  &workflow,
    })
    return err
}
```

## Automated Migration Tools

For large codebases, you can use these `sed` or text replacement commands to help with migration:

### Find Functions That Need Migration

```bash
# Find Create calls without context
grep -r "wfClient.Create(&" .

# Find Get calls without context
grep -r "wfClient.Get(\"" .

# Find List calls without context
grep -r "wfClient.List(metav1" .
```

### Automated Replacement (use with caution)

```bash
# Add context import (manual verification recommended)
# Add to imports: "context"

# Replace common patterns (review changes carefully!)
# Create: .Create( -> .Create(ctx,
# Get: .Get(" -> .Get(ctx, "
# List: .List(metav1 -> .List(ctx, metav1
# Watch: .Watch(metav1 -> .Watch(ctx, metav1
# Delete: .Delete(" -> .Delete(ctx, "
```

**Note:** Always review automated changes carefully before committing.

## Troubleshooting

### Error: "not enough arguments in call to..."

This indicates you're calling a function that now requires a context parameter.

**Solution:** Add `ctx` as the first parameter.

### Error: "context.Context is nil"

This happens when you pass a nil context.

**Solution:** Use `context.Background()` or another appropriate context.

### Deadlocks or Hangs

If your application hangs, you might be missing context cancellation.

**Solution:** Use `context.WithTimeout` or `context.WithCancel` and ensure proper cleanup.

### "context canceled" Errors

This is normal when a context is cancelled or times out.

**Solution:** Check if the timeout is appropriate, or handle the error gracefully:

```go
if err != nil {
    if ctx.Err() == context.Canceled {
        log.Println("Operation was cancelled")
    } else if ctx.Err() == context.DeadlineExceeded {
        log.Println("Operation timed out")
    } else {
        log.Printf("Error: %v", err)
    }
}
```

## Additional Resources

- [Go SDK Guide](./go-sdk-guide.md)
- [Go Context Package Documentation](https://pkg.go.dev/context)
- [Examples](../examples/go-sdk/)
- [API Reference](https://pkg.go.dev/github.com/argoproj/argo-workflows/v3)

## Getting Help

If you encounter issues during migration:

1. Check the [examples](../examples/go-sdk/) directory for working code
2. Review the [Go SDK Guide](./go-sdk-guide.md)
3. Ask questions in the [Argo Workflows Slack](https://argoproj.github.io/community/join-slack)
4. Open an issue on [GitHub](https://github.com/argoproj/argo-workflows/issues)
