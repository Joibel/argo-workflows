# Watch Workflow Example

This example demonstrates how to submit a workflow and watch its progress in real-time.

## What it does

- Submits a workflow that runs for a few seconds
- Watches the workflow using field selectors
- Prints phase changes and status updates in real-time
- Handles watch events (Added, Modified, Deleted)
- Displays final workflow details upon completion

## Prerequisites

- Go 1.21+
- Access to a Kubernetes cluster with Argo Workflows installed
- Kubeconfig configured for cluster access

## Running the example

```bash
# Use default kubeconfig
go run main.go

# Specify custom kubeconfig and namespace
go run main.go -kubeconfig /path/to/kubeconfig -namespace argo
```

## Expected output

```
Submitting workflow...
✓ Workflow watch-example-abc123 submitted

Watching workflow progress...
─────────────────────────────────────────────
[00:00] Workflow created
[00:01] Phase: Pending
[00:03] Phase: Running
         Started at: 2025-01-15T10:30:00Z
[00:08] Phase: Succeeded
─────────────────────────────────────────────
✓ Workflow completed!
  Final Phase: Succeeded
  Started: 2025-01-15T10:30:00Z
  Finished: 2025-01-15T10:30:08Z
  Duration: 8s

Node Details:
  - watch-example-abc123: Succeeded
```

## Code walkthrough

1. **Create workflow**: Define a workflow with a sleep task
2. **Submit workflow**: Create the workflow using the client
3. **Setup watch**: Create a watch with field selector for specific workflow
4. **Process events**: Handle Added, Modified, and Deleted events
5. **Track state**: Monitor phase changes and print updates
6. **Complete**: Display final status when workflow finishes

## Key concepts

### Field Selectors

Field selectors allow watching specific resources:

```go
fieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", name))
watchIf, err := wfClient.Watch(ctx, metav1.ListOptions{
    FieldSelector: fieldSelector.String(),
})
```

### Watch Events

The watch returns three types of events:
- `watch.Added`: Resource was created
- `watch.Modified`: Resource was updated
- `watch.Deleted`: Resource was deleted

### Context Handling

The example uses `select` to handle both context cancellation and watch events:

```go
select {
case <-ctx.Done():
    return ctx.Err()
case event := <-watchIf.ResultChan():
    // Process event
}
```

## Next steps

- See `basic-workflow` for simple workflow submission
- See `grpc-client` for remote Argo Server access
- See `workflow-template` for reusable templates
