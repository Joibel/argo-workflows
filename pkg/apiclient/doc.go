// Package apiclient provides a Go client for interacting with Argo Server.
//
// The apiclient package is one of two client options for the Argo Workflows Go SDK.
// Use this package when you need to connect to Argo Server remotely via gRPC or HTTP,
// or when you need service-oriented operations like Retry, Stop, and Suspend.
//
// # Client Architecture
//
// Argo Workflows provides two client approaches:
//
// 1. Kubernetes Client - Direct CRD access (pkg/client/clientset/versioned)
//   - Best for: In-cluster applications, kubectl-like tools
//   - Authentication: Kubeconfig or ServiceAccount
//   - Operations: CRUD, Watch, Patch
//
// 2. Argo Server Client - This package (pkg/apiclient)
//   - Best for: Remote access, external applications
//   - Authentication: Bearer token, SSO
//   - Operations: CRUD + Retry/Stop/Suspend/Resume
//
// # Quick Start
//
// Connect to Argo Server:
//
//	import (
//	    "context"
//	    "github.com/argoproj/argo-workflows/v3/pkg/apiclient"
//	)
//
//	ctx := context.Background()
//	client, err := apiclient.NewClientFromOptsWithContext(ctx, apiclient.Opts{
//	    ArgoServerOpts: apiclient.ArgoServerOpts{
//	        URL: "localhost:2746",
//	    },
//	    AuthSupplier: func() string { return bearerToken },
//	})
//	if err != nil {
//	    panic(err)
//	}
//
// Create a workflow service client:
//
//	serviceClient := client.NewWorkflowServiceClient(ctx)
//	created, err := serviceClient.CreateWorkflow(ctx, &workflowpkg.WorkflowCreateRequest{
//	    Namespace: "default",
//	    Workflow:  &workflow,
//	})
//
// # Available Service Clients
//
// The Client interface provides access to multiple service clients:
//
//   - WorkflowServiceClient - Create, get, list, delete, retry, stop, suspend workflows
//   - CronWorkflowServiceClient - Manage cron workflows
//   - WorkflowTemplateServiceClient - Manage workflow templates
//   - ClusterWorkflowTemplateServiceClient - Manage cluster-wide templates
//   - ArchivedWorkflowServiceClient - Access archived workflows
//   - InfoServiceClient - Get server information
//   - SyncServiceClient - Synchronization primitives
//
// # Authentication
//
// Bearer Token:
//
//	opts := apiclient.Opts{
//	    ArgoServerOpts: apiclient.ArgoServerOpts{
//	        URL: "localhost:2746",
//	        Secure: true,
//	    },
//	    AuthSupplier: func() string {
//	        return os.Getenv("ARGO_TOKEN")
//	    },
//	}
//
// Using Kubeconfig:
//
//	opts := apiclient.Opts{
//	    ArgoServerOpts: apiclient.ArgoServerOpts{
//	        URL: "localhost:2746",
//	    },
//	    ClientConfigSupplier: func() clientcmd.ClientConfig {
//	        loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
//	        return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
//	            loadingRules,
//	            &clientcmd.ConfigOverrides{},
//	        )
//	    },
//	}
//
// # Connection Modes
//
// The client supports multiple connection modes:
//
//   - gRPC (default) - High-performance binary protocol
//   - HTTP/1 - Fallback for restricted environments
//   - Kubernetes - Direct cluster API access
//   - Offline - Mock client for testing
//
// Select HTTP/1 mode:
//
//	opts.ArgoServerOpts.HTTP1 = true
//
// # TLS and Security
//
// Enable TLS (default):
//
//	opts.ArgoServerOpts.Secure = true
//
// Skip TLS verification (development only):
//
//	opts.ArgoServerOpts.InsecureSkipVerify = true
//
// # Workflow Operations
//
// Create workflow:
//
//	created, err := serviceClient.CreateWorkflow(ctx, &workflowpkg.WorkflowCreateRequest{
//	    Namespace: "default",
//	    Workflow:  &workflow,
//	})
//
// List workflows:
//
//	list, err := serviceClient.ListWorkflows(ctx, &workflowpkg.WorkflowListRequest{
//	    Namespace: "default",
//	})
//
// Get workflow:
//
//	wf, err := serviceClient.GetWorkflow(ctx, &workflowpkg.WorkflowGetRequest{
//	    Namespace: "default",
//	    Name:      "my-workflow",
//	})
//
// Retry workflow:
//
//	updated, err := serviceClient.RetryWorkflow(ctx, &workflowpkg.WorkflowRetryRequest{
//	    Namespace: "default",
//	    Name:      "my-workflow",
//	})
//
// Stop workflow:
//
//	updated, err := serviceClient.StopWorkflow(ctx, &workflowpkg.WorkflowStopRequest{
//	    Namespace: "default",
//	    Name:      "my-workflow",
//	})
//
// Suspend workflow:
//
//	updated, err := serviceClient.SuspendWorkflow(ctx, &workflowpkg.WorkflowSuspendRequest{
//	    Namespace: "default",
//	    Name:      "my-workflow",
//	})
//
// Resume workflow:
//
//	updated, err := serviceClient.ResumeWorkflow(ctx, &workflowpkg.WorkflowResumeRequest{
//	    Namespace: "default",
//	    Name:      "my-workflow",
//	})
//
// # Context Usage
//
// Since v3.7, all operations require a context.Context parameter:
//
//	ctx := context.Background()
//
// With timeout:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
// With cancellation:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	// Call cancel() to stop operations
//
// # Error Handling
//
// Check for specific errors:
//
//	if err != nil {
//	    if ctx.Err() == context.Canceled {
//	        // Operation was cancelled
//	    } else if ctx.Err() == context.DeadlineExceeded {
//	        // Operation timed out
//	    } else {
//	        // Other error
//	    }
//	}
//
// # Examples
//
// See the examples directory for complete, working examples:
//   - examples/go-sdk/grpc-client/ - Complete gRPC client example
//   - examples/go-sdk/authentication/ - Authentication methods
//
// # Documentation
//
// For more information:
//   - Go SDK Guide: docs/go-sdk-guide.md
//   - Migration Guide: docs/go-sdk-migration-guide.md
//   - API Reference: https://pkg.go.dev/github.com/argoproj/argo-workflows/v3
//
// # When to Use This Package vs Kubernetes Client
//
// Use apiclient when:
//   - Accessing Argo Server remotely
//   - You don't have direct Kubernetes access
//   - You need service operations (Retry, Stop, Suspend)
//   - Working with archived workflows
//
// Use Kubernetes client (pkg/client/clientset/versioned) when:
//   - Running inside Kubernetes
//   - You have kubeconfig access
//   - You need efficient watching/listing
//   - You want native Kubernetes API patterns
package apiclient
