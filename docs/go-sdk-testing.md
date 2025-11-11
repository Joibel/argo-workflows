# Go SDK Examples - Testing and CI Integration

This document explains how the Go SDK examples are tested and integrated into CI.

## Structure

The Go SDK examples are located in `examples/go-sdk/` with the following structure:

```
examples/go-sdk/
├── README.md              # Overview and quick start
├── basic-workflow/        # Simple workflow submission
│   ├── main.go
│   ├── go.mod
│   ├── go.sum
│   └── README.md
├── watch-workflow/        # Workflow progress tracking
├── grpc-client/           # Argo Server gRPC client
├── workflow-template/     # WorkflowTemplate usage
└── authentication/        # Authentication methods
```

Each example is a **separate Go module** with its own `go.mod` file. This design:
- Makes examples easy to copy and use independently
- Ensures examples work with published versions
- Uses `replace` directives to test against local code

## CI Integration

### Makefile Target

The examples are tested using the `test-go-sdk` Makefile target:

```makefile
.PHONY: test-go-sdk
test-go-sdk: ## Compile all Go SDK examples to ensure they build
	@echo "Testing Go SDK examples..."
	@for dir in examples/go-sdk/*/; do \
		if [ -f "$$dir/go.mod" ]; then \
			echo "Building $$dir..."; \
			(cd "$$dir" && go mod tidy && go build -o /dev/null .) || exit 1; \
		fi \
	done
	@echo "All Go SDK examples compiled successfully"
```

This target:
1. Finds all directories in `examples/go-sdk/` with a `go.mod` file
2. Runs `go mod tidy` to ensure dependencies are correct
3. Compiles each example with `go build`
4. Exits with error if any example fails to compile

### GitHub Actions Workflow

The examples are tested in the CI workflow (`.github/workflows/ci-build.yaml`) as part of the E2E test matrix:

```yaml
matrix:
  include:
    # ... other tests ...
    - test: test-go-sdk
      profile: minimal
      use-api: false
    - test: test-java-sdk
      profile: minimal
      use-api: true
    - test: test-python-sdk
      profile: minimal
      use-api: true
```

This runs alongside other SDK tests like `test-java-sdk` and `test-python-sdk`.

The test runs when:
- Code generation files change (`codegen == 'true'`)
- Test files change (`tests == 'true'`)
- This ensures examples are tested when SDK code changes

## Local Testing

### Test all examples

```bash
make test-go-sdk
```

### Test specific example

```bash
cd examples/go-sdk/basic-workflow
go mod tidy
go build
./basic-workflow --help
```

### Test with local changes

The examples use `replace` directives in their `go.mod` files:

```go
replace github.com/argoproj/argo-workflows/v3 => ../../..
```

This means they automatically use the local codebase, making it easy to:
1. Modify SDK code
2. Test examples against changes
3. Ensure examples work with new features

## Adding New Examples

To add a new Go SDK example:

1. **Create the directory:**
   ```bash
   mkdir examples/go-sdk/my-example
   cd examples/go-sdk/my-example
   ```

2. **Initialize the module:**
   ```bash
   go mod init github.com/argoproj/argo-workflows/v3/examples/go-sdk/my-example
   ```

3. **Add the replace directive:**
   ```bash
   echo 'replace github.com/argoproj/argo-workflows/v3 => ../../..' >> go.mod
   ```

4. **Add dependencies:**
   ```bash
   go get github.com/argoproj/argo-workflows/v3@v3.0.0-00010101000000-000000000000
   go get k8s.io/apimachinery@v0.33.1
   go get k8s.io/client-go@v0.33.1
   # ... other dependencies
   ```

5. **Write the code:**
   ```go
   // main.go
   package main

   func main() {
       // Your example code
   }
   ```

6. **Test it:**
   ```bash
   go mod tidy
   go build
   ./my-example
   ```

7. **Add a README.md** explaining what the example does

8. **Test in CI:**
   ```bash
   # From repo root
   make test-go-sdk
   ```

The new example will automatically be picked up by the CI system!

## Troubleshooting

### "version invalid: should be v3, not v0"

The version in `go.mod` should be:
```go
require (
    github.com/argoproj/argo-workflows/v3 v3.0.0-00010101000000-000000000000
    // ...
)
```

Not:
```go
require (
    github.com/argoproj/argo-workflows/v3 v0.0.0  // Wrong!
    // ...
)
```

### "undefined: wfclientset.WorkflowInterface"

Import the typed client package:
```go
import (
    wfclientset "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
    v1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
)

func myFunc(wfClient v1alpha1.WorkflowInterface) {
    // ...
}
```

### Examples fail to compile locally

Ensure you're in the repo root with all dependencies:
```bash
cd /path/to/argo-workflows
go mod download
make test-go-sdk-examples
```

## Benefits of This Approach

1. **User-friendly**: Examples are standalone and easy to copy
2. **CI-tested**: All examples are verified to compile on every PR
3. **Documentation**: Examples serve as executable documentation
4. **Maintenance**: Broken examples are caught immediately
5. **Local testing**: Easy to test against local SDK changes

## Future Enhancements

Potential improvements:
- Add runtime tests (requires cluster access in CI)
- Test examples against multiple Go versions
- Add example output verification
- Create example test suite with expected behaviors
