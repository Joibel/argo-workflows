//go:build functional

package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/argoproj/argo-workflows/v3/test/e2e/fixtures"
)

type PodRestartSuite struct {
	fixtures.E2ESuite
}

// TestEvictedPodRestarts tests that a pod which is evicted before the main container
// starts is automatically restarted and the workflow eventually succeeds.
func (s *PodRestartSuite) TestEvictedPodRestarts() {
	var firstPodName string

	s.Given().
		Workflow(`
metadata:
  generateName: evicted-restart-
spec:
  entrypoint: main
  templates:
    - name: main
      initContainers:
        - name: delay
          image: alpine:latest
          command: [sleep, "30"]
      script:
        image: alpine:latest
        command: [sh]
        source: |
          echo "pod-ran-successfully"
`).
		When().
		SubmitWorkflow().
		// Wait for the pod to be created and init container to start
		WaitForPod(func(p *corev1.Pod) bool {
			// Wait until pod exists and is running (init container running)
			if p.Status.Phase != corev1.PodRunning && p.Status.Phase != corev1.PodPending {
				return false
			}
			firstPodName = p.Name
			return true
		}).
		And(func() {
			// Patch the pod status to simulate an eviction before main container started
			ctx := context.Background()

			// The patch simulates what happens when a pod is evicted:
			// - Phase becomes Failed
			// - Reason is set to "Evicted"
			// - Message describes the eviction cause (this causes inferFailedReason to return early)
			// - Init containers show terminated state (evicted before completing)
			// - Main container never entered Running state (still in Waiting)
			patch := map[string]interface{}{
				"status": map[string]interface{}{
					"phase":   "Failed",
					"reason":  "Evicted",
					"message": "The node had condition: [DiskPressure]",
					"initContainerStatuses": []map[string]interface{}{
						{
							"name":  "delay",
							"image": "alpine:latest",
							"state": map[string]interface{}{
								"terminated": map[string]interface{}{
									"exitCode": 137,
									"reason":   "Error",
								},
							},
							"ready":        false,
							"restartCount": 0,
						},
					},
					"containerStatuses": []map[string]interface{}{
						{
							"name":  "main",
							"image": "alpine:latest",
							"state": map[string]interface{}{
								"waiting": map[string]interface{}{
									"reason": "PodInitializing",
								},
							},
							"ready":        false,
							"restartCount": 0,
						},
					},
				},
			}
			patchBytes, err := json.Marshal(patch)
			require.NoError(s.T(), err)

			_, err = s.KubeClient.CoreV1().Pods(fixtures.Namespace).Patch(
				ctx,
				firstPodName,
				types.MergePatchType,
				patchBytes,
				metav1.PatchOptions{},
				"status",
			)
			require.NoError(s.T(), err)
			s.T().Logf("Patched pod %s to simulate eviction", firstPodName)
		}).
		WaitForWorkflow(fixtures.ToBeSucceeded, 60*time.Second).
		Then().
		ExpectWorkflow(func(t *testing.T, metadata *metav1.ObjectMeta, status *wfv1.WorkflowStatus) {
			assert.Equal(t, wfv1.WorkflowSucceeded, status.Phase)
		}).
		ExpectWorkflowNode(wfv1.NodeWithDisplayName("main"), func(t *testing.T, n *wfv1.NodeStatus, pod *corev1.Pod) {
			// Verify that FailedPodRestarts was incremented
			assert.Equal(t, int32(1), n.FailedPodRestarts, "expected FailedPodRestarts to be 1")

			// Verify the pod actually ran and produced output
			require.NotNil(t, n.Outputs, "expected node to have outputs")
			require.NotNil(t, n.Outputs.Result, "expected node to have result output")
			assert.Equal(t, "pod-ran-successfully", *n.Outputs.Result, "expected output to match")

			// Verify this is a different pod than the one we evicted
			if pod != nil {
				assert.NotEqual(t, firstPodName, pod.Name, "expected a new pod to be created")
			}
		})
}

func TestPodRestartSuite(t *testing.T) {
	suite.Run(t, new(PodRestartSuite))
}
