# Authentication Example

This example demonstrates different authentication methods for the Argo Workflows Go SDK.

## Supported Authentication Methods

1. **Kubeconfig** - Standard kubectl-style authentication
2. **In-Cluster** - Service account authentication from within a pod
3. **Bearer Token** - Direct token authentication
4. **gRPC** - Argo Server authentication

## Running the examples

### 1. Kubeconfig Authentication

```bash
go run main.go -mode kubeconfig

# With custom kubeconfig path
go run main.go -mode kubeconfig -kubeconfig /path/to/config

# With namespace
go run main.go -mode kubeconfig -namespace argo
```

**Use when:**
- Developing locally
- Using kubectl
- Have kubeconfig access

### 2. In-Cluster Authentication

```bash
# This only works inside a Kubernetes pod
go run main.go -mode incluster
```

**Use when:**
- Running inside Kubernetes
- Using service accounts
- Building cluster-native applications

**Requirements:**
- Must run in a pod
- ServiceAccount with RBAC permissions

### 3. Bearer Token Authentication

```bash
# Export token
export KUBE_TOKEN=$(kubectl create token my-service-account)

# Run example
go run main.go -mode token
```

**Use when:**
- Automation and CI/CD
- Service-to-service authentication
- Don't have kubeconfig

**Get token:**
```bash
# Create token for service account
kubectl create token <service-account-name>

# Or get from secret
kubectl get secret <secret-name> -o jsonpath='{.data.token}' | base64 -d
```

### 4. gRPC Authentication (Argo Server)

```bash
# Export token
export ARGO_TOKEN=$(kubectl -n argo create token argo-server)

# Run example
go run main.go -mode grpc -argo-server localhost:2746 -token $ARGO_TOKEN
```

**Use when:**
- Accessing Argo Server remotely
- Don't have Kubernetes API access
- Using Argo Server features (archives, etc.)

**Setup:**
```bash
# Port forward Argo Server
kubectl -n argo port-forward svc/argo-server 2746:2746

# Get token
export ARGO_TOKEN=$(kubectl -n argo create token argo-server)
```

## Expected Output

### Kubeconfig Mode
```
=== Authentication Example ===
Mode: kubeconfig

--- Authentication via Kubeconfig ---
Kubeconfig path: /home/user/.kube/config
✓ Loaded kubeconfig
  API Server: https://kubernetes.default.svc
  Auth: Client Certificate
✓ Successfully authenticated and connected
  Found 3 workflow(s) in namespace 'default'

Usage:
  Best for: Development, CLI tools, kubectl-like access
  Requires: Valid kubeconfig file with cluster credentials
```

### In-Cluster Mode
```
=== Authentication Example ===
Mode: incluster

--- Authentication via In-Cluster Config ---
✓ Loaded in-cluster config
  API Server: https://kubernetes.default.svc
  Service Account: workflow-client
✓ Successfully authenticated and connected
  Found 3 workflow(s) in namespace 'default'

Usage:
  Best for: Applications running inside Kubernetes
  Requires: Pod with ServiceAccount having RBAC permissions
```

## Required RBAC Permissions

For in-cluster or token authentication, ensure your ServiceAccount has appropriate permissions:

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

## Choosing the Right Method

| Method | Use Case | Pros | Cons |
|--------|----------|------|------|
| **Kubeconfig** | Development, CLI tools | Easy, standard kubectl flow | Requires kubeconfig file |
| **In-Cluster** | Apps in cluster | Native K8s, no external config | Only works in pods |
| **Token** | Automation, CI/CD | Simple, scriptable | Token management needed |
| **gRPC** | Remote access | Works remotely, Argo features | Requires Argo Server |

## Security Best Practices

1. **Never commit tokens or credentials** to version control
2. **Use environment variables** for sensitive data
3. **Rotate tokens regularly** using short-lived tokens
4. **Follow principle of least privilege** for RBAC permissions
5. **Use TLS** for all production connections
6. **Store credentials securely** using secret management systems

## Troubleshooting

### "Error loading kubeconfig"
- Verify kubeconfig path is correct
- Check kubeconfig file permissions
- Ensure kubeconfig is valid YAML

### "Error loading in-cluster config"
- This only works inside a pod
- Verify ServiceAccount is mounted
- Check RBAC permissions

### "Unauthorized" errors
- Verify token is valid and not expired
- Check RBAC permissions for ServiceAccount
- Ensure namespace access is granted

### "Connection refused"
- For gRPC: Verify Argo Server is running
- Check port forwarding is active
- Verify firewall/network policies

## Next Steps

- See `basic-workflow` for workflow submission
- See `grpc-client` for more Argo Server examples
- See `watch-workflow` for monitoring workflows
