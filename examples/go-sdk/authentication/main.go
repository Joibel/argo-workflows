package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-workflows/v3/pkg/apiclient"
	wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
)

func main() {
	var (
		authMode = flag.String("mode", "kubeconfig", "authentication mode: kubeconfig, incluster, token, grpc")
		namespace = flag.String("namespace", "default", "namespace to list workflows")

		// For kubeconfig mode
		kubeconfig = flag.String("kubeconfig", defaultKubeconfig(), "path to kubeconfig file")

		// For gRPC mode
		argoServer = flag.String("argo-server", "localhost:2746", "Argo Server address")
		token      = flag.String("token", os.Getenv("ARGO_TOKEN"), "bearer token for authentication")
	)
	flag.Parse()

	ctx := context.Background()

	fmt.Printf("=== Authentication Example ===\n")
	fmt.Printf("Mode: %s\n\n", *authMode)

	switch *authMode {
	case "kubeconfig":
		demonstrateKubeconfig(ctx, *kubeconfig, *namespace)
	case "incluster":
		demonstrateInCluster(ctx, *namespace)
	case "token":
		demonstrateTokenAuth(ctx, *kubeconfig, *namespace)
	case "grpc":
		demonstrateGRPC(ctx, *argoServer, *token, *namespace)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", *authMode)
		fmt.Fprintf(os.Stderr, "Valid modes: kubeconfig, incluster, token, grpc\n")
		os.Exit(1)
	}
}

// demonstrateKubeconfig shows authentication using kubeconfig file
func demonstrateKubeconfig(ctx context.Context, kubeconfigPath, namespace string) {
	fmt.Println("--- Authentication via Kubeconfig ---")
	fmt.Printf("Kubeconfig path: %s\n", kubeconfigPath)

	// Load config from kubeconfig file
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading kubeconfig: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Loaded kubeconfig\n")
	fmt.Printf("  API Server: %s\n", config.Host)
	fmt.Printf("  Auth: %s\n", describeAuth(config))

	// Create clientset
	clientset := wfclientset.NewForConfigOrDie(config)
	wfClient := clientset.ArgoprojV1alpha1().Workflows(namespace)

	// Test by listing workflows
	list, err := wfClient.List(ctx, metav1.ListOptions{Limit: 5})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing workflows: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Successfully authenticated and connected\n")
	fmt.Printf("  Found %d workflow(s) in namespace '%s'\n", len(list.Items), namespace)

	fmt.Println("\nUsage:")
	fmt.Println("  Best for: Development, CLI tools, kubectl-like access")
	fmt.Println("  Requires: Valid kubeconfig file with cluster credentials")
}

// demonstrateInCluster shows authentication from within a pod
func demonstrateInCluster(ctx context.Context, namespace string) {
	fmt.Println("--- Authentication via In-Cluster Config ---")

	// Load in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading in-cluster config: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nNote: This mode only works when running inside a Kubernetes pod\n")
		fmt.Fprintf(os.Stderr, "      with a service account that has appropriate RBAC permissions.\n")
		os.Exit(1)
	}

	fmt.Printf("✓ Loaded in-cluster config\n")
	fmt.Printf("  API Server: %s\n", config.Host)
	fmt.Printf("  Service Account: %s\n", os.Getenv("KUBERNETES_SERVICE_ACCOUNT"))

	// Create clientset
	clientset := wfclientset.NewForConfigOrDie(config)
	wfClient := clientset.ArgoprojV1alpha1().Workflows(namespace)

	// Test by listing workflows
	list, err := wfClient.List(ctx, metav1.ListOptions{Limit: 5})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing workflows: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Successfully authenticated and connected\n")
	fmt.Printf("  Found %d workflow(s) in namespace '%s'\n", len(list.Items), namespace)

	fmt.Println("\nUsage:")
	fmt.Println("  Best for: Applications running inside Kubernetes")
	fmt.Println("  Requires: Pod with ServiceAccount having RBAC permissions")
	fmt.Println("\nExample RBAC:")
	fmt.Println("  apiVersion: v1")
	fmt.Println("  kind: ServiceAccount")
	fmt.Println("  metadata:")
	fmt.Println("    name: workflow-client")
	fmt.Println("  ---")
	fmt.Println("  apiVersion: rbac.authorization.k8s.io/v1")
	fmt.Println("  kind: Role")
	fmt.Println("  metadata:")
	fmt.Println("    name: workflow-client-role")
	fmt.Println("  rules:")
	fmt.Println("  - apiGroups: [\"argoproj.io\"]")
	fmt.Println("    resources: [\"workflows\"]")
	fmt.Println("    verbs: [\"get\", \"list\", \"create\"]")
}

// demonstrateTokenAuth shows authentication using bearer token
func demonstrateTokenAuth(ctx context.Context, kubeconfigPath, namespace string) {
	fmt.Println("--- Authentication via Bearer Token ---")

	// Get token from environment or file
	token := os.Getenv("KUBE_TOKEN")
	if token == "" {
		// Try to read from service account token file
		tokenFile := "/var/run/secrets/kubernetes.io/serviceaccount/token"
		data, err := os.ReadFile(tokenFile)
		if err == nil {
			token = string(data)
			fmt.Printf("✓ Loaded token from: %s\n", tokenFile)
		}
	}

	if token == "" {
		fmt.Fprintf(os.Stderr, "Error: No token available\n")
		fmt.Fprintf(os.Stderr, "Set KUBE_TOKEN environment variable or run inside a pod\n")
		os.Exit(1)
	}

	// Load base config from kubeconfig to get server URL
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading kubeconfig: %v\n", err)
		os.Exit(1)
	}

	// Override with token auth
	config.BearerToken = token
	config.BearerTokenFile = ""

	fmt.Printf("✓ Configured bearer token authentication\n")
	fmt.Printf("  API Server: %s\n", config.Host)
	fmt.Printf("  Token length: %d characters\n", len(token))

	// Create clientset
	clientset := wfclientset.NewForConfigOrDie(config)
	wfClient := clientset.ArgoprojV1alpha1().Workflows(namespace)

	// Test by listing workflows
	list, err := wfClient.List(ctx, metav1.ListOptions{Limit: 5})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing workflows: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Successfully authenticated and connected\n")
	fmt.Printf("  Found %d workflow(s) in namespace '%s'\n", len(list.Items), namespace)

	fmt.Println("\nUsage:")
	fmt.Println("  Best for: Service accounts, automation, CI/CD")
	fmt.Println("  Requires: Valid bearer token with appropriate permissions")
	fmt.Println("\nGet token with:")
	fmt.Println("  kubectl create token <service-account-name>")
}

// demonstrateGRPC shows authentication to Argo Server
func demonstrateGRPC(ctx context.Context, argoServer, token, namespace string) {
	fmt.Println("--- Authentication via Argo Server (gRPC) ---")
	fmt.Printf("Server: %s\n", argoServer)

	// Create Argo Server client
	ctx, client, err := apiclient.NewClientFromOptsWithContext(ctx, apiclient.Opts{
		ArgoServerOpts: apiclient.ArgoServerOpts{
			URL:    argoServer,
			Secure: true,
		},
		AuthSupplier: func() string {
			return token
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating client: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Connected to Argo Server\n")

	// Get workflow service client
	serviceClient := client.NewWorkflowServiceClient(ctx)

	// Test by listing workflows
	list, err := serviceClient.ListWorkflows(ctx, &apiclient.WorkflowListRequest{
		Namespace: namespace,
		ListOptions: &metav1.ListOptions{
			Limit: 5,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing workflows: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Successfully authenticated and connected\n")
	fmt.Printf("  Found %d workflow(s) in namespace '%s'\n", len(list.Items), namespace)

	fmt.Println("\nUsage:")
	fmt.Println("  Best for: Remote access, web applications, external tools")
	fmt.Println("  Requires: Argo Server endpoint and bearer token")
	fmt.Println("\nGet token with:")
	fmt.Println("  kubectl -n argo get secret <secret-name> -o jsonpath='{.data.token}' | base64 -d")
	fmt.Println("\nOr create service account token:")
	fmt.Println("  kubectl -n argo create token argo-server")
}

func describeAuth(config *rest.Config) string {
	if config.BearerToken != "" || config.BearerTokenFile != "" {
		return "Bearer Token"
	}
	if config.CertFile != "" || config.CertData != nil {
		return "Client Certificate"
	}
	if config.Username != "" {
		return "Basic Auth"
	}
	return "Unknown"
}

func defaultKubeconfig() string {
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}
