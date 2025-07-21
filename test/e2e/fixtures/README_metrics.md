# Metrics Testing with Baseline Tracking

This package provides utilities for testing Prometheus metrics by tracking baseline values and checking for expected increases, rather than checking absolute values.

## Problem

Previously, metrics tests were flawed because they assumed metric values would start at zero:

```go
// ❌ FLAWED: Assumes absolute value
Contains(`workflowtemplate_triggered_total{cluster_scope="true",name="basic",namespace="argo",phase="New"} 1`)
```

This approach failed when:
- Tests ran multiple times
- Tests had retries
- Other tests ran before and incremented the same metrics

## Solution

Use `MetricBaseline` to capture current metric values before running tests, then check for expected increases:

```go
// ✅ CORRECT: Capture baseline first
baseline := s.captureBaseline(
    `workflowtemplate_triggered_total{cluster_scope="true",name="basic",namespace="argo",phase="New"}`,
    `workflowtemplate_triggered_total{cluster_scope="true",name="basic",namespace="argo",phase="Running"}`,
    `workflowtemplate_triggered_total{cluster_scope="true",name="basic",namespace="argo",phase="Succeeded"}`,
)

// ... run your test workflow ...

// ✅ CORRECT: Check for expected increases
baseline.ExpectIncrease(map[string]float64{
    `workflowtemplate_triggered_total{cluster_scope="true",name="basic",namespace="argo",phase="New"}`:       1,
    `workflowtemplate_triggered_total{cluster_scope="true",name="basic",namespace="argo",phase="Running"}`:   1,
    `workflowtemplate_triggered_total{cluster_scope="true",name="basic",namespace="argo",phase="Succeeded"}`: 1,
})
```

## Usage

### 1. For MetricsSuite tests (Recommended: Define once approach)

Use the `captureBaseline` helper method to avoid duplication:

```go
func (s *MetricsSuite) TestMyMetrics() {
    // Define expected increases once at the start
    expectedIncreases := map[string]float64{
        `my_metric{label="value"}`: 2,           // Expect increase of 2
        `another_metric{different="labels"}`: 1, // Expect increase of 1
    }

    // Capture baseline metrics from the map
    baseline := s.captureBaseline(expectedIncreases)

    s.Given().
        Workflow(`@testdata/my-workflow.yaml`).
        When().
        SubmitWorkflow().
        WaitForWorkflow(fixtures.ToBeSucceeded).
        Then().
        ExpectWorkflow(func(t *testing.T, metadata *metav1.ObjectMeta, status *wfv1.WorkflowStatus) {
            // Check that metrics increased by expected amounts
            baseline.ExpectStoredIncrease()
        })
}
```



### 2. For other test types

Create a `MetricBaseline` directly:

```go
expectedIncreases := map[string]float64{
    `my_metric{labels}`: 1,
}
baseline := fixtures.NewMetricBaseline(t, func() *httpexpect.Expect { 
    return httpexpect.New(t, "https://localhost:9090/metrics") 
})
baseline.CaptureBaseline(expectedIncreases)

// ... run test ...

baseline.ExpectStoredIncrease()
```

## Key Benefits

1. **No false positives**: Tests won't fail due to previous test runs
2. **Works with retries**: Handles test retry scenarios correctly  
3. **Handles missing metrics**: If a metric doesn't exist initially, assumes baseline of 0
4. **Clear intent**: Makes it obvious what increase you expect
5. **Better debugging**: Logs baseline, current, and expected values
6. **Robust floating point comparison**: Uses `assert.InDelta` instead of `assert.Equal` to handle floating point precision issues

## Metric Pattern Format

Metric patterns should match the exact Prometheus line format:
- Include the full metric name with labels: `metric_name{label1="value1",label2="value2"}`
- For metrics without labels: `metric_name`
- Do not include the value part (the number)
