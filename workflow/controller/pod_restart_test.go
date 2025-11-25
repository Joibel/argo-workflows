package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/argoproj/argo-workflows/v3/workflow/common"
)

func TestMainContainerNeverStarted(t *testing.T) {
	tests := []struct {
		name     string
		pod      *apiv1.Pod
		tmpl     *wfv1.Template
		expected bool
	}{
		{
			name: "pod with no container statuses (never scheduled)",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase:             apiv1.PodFailed,
					ContainerStatuses: []apiv1.ContainerStatus{},
				},
			},
			tmpl:     nil,
			expected: true,
		},
		{
			name: "main container in waiting state",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodFailed,
					ContainerStatuses: []apiv1.ContainerStatus{
						{
							Name: common.MainContainerName,
							State: apiv1.ContainerState{
								Waiting: &apiv1.ContainerStateWaiting{
									Reason:  "ContainerCreating",
									Message: "Container is creating",
								},
							},
						},
					},
				},
			},
			tmpl:     nil,
			expected: true,
		},
		{
			name: "main container ran and terminated",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodFailed,
					ContainerStatuses: []apiv1.ContainerStatus{
						{
							Name: common.MainContainerName,
							State: apiv1.ContainerState{
								Terminated: &apiv1.ContainerStateTerminated{
									ExitCode:   1,
									StartedAt:  metav1.Now(),
									FinishedAt: metav1.Now(),
								},
							},
						},
					},
				},
			},
			tmpl:     nil,
			expected: false,
		},
		{
			name: "main container was running",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodFailed,
					ContainerStatuses: []apiv1.ContainerStatus{
						{
							Name: common.MainContainerName,
							State: apiv1.ContainerState{
								Running: &apiv1.ContainerStateRunning{
									StartedAt: metav1.Now(),
								},
							},
						},
					},
				},
			},
			tmpl:     nil,
			expected: false,
		},
		{
			name: "main container waiting for pod initializing",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodFailed,
					ContainerStatuses: []apiv1.ContainerStatus{
						{
							Name: common.MainContainerName,
							State: apiv1.ContainerState{
								Waiting: &apiv1.ContainerStateWaiting{
									Reason: "PodInitializing",
								},
							},
						},
					},
				},
			},
			tmpl:     nil,
			expected: true,
		},
		{
			name: "main container terminated but never had startedAt",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodFailed,
					ContainerStatuses: []apiv1.ContainerStatus{
						{
							Name: common.MainContainerName,
							State: apiv1.ContainerState{
								Terminated: &apiv1.ContainerStateTerminated{
									ExitCode: 137,
									Reason:   "OOMKilled",
									// No StartedAt - container was killed before starting
								},
							},
						},
					},
				},
			},
			tmpl:     nil,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mainContainerNeverStarted(tt.pod, tt.tmpl)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRestartableReason(t *testing.T) {
	tests := []struct {
		name       string
		reason     string
		message    string
		conditions []apiv1.PodCondition
		expected   bool
	}{
		{
			name:     "evicted due to DiskPressure",
			reason:   "Evicted",
			message:  "The node had condition: [DiskPressure]",
			expected: true,
		},
		{
			name:     "evicted due to MemoryPressure",
			reason:   "Evicted",
			message:  "The node had condition: [MemoryPressure]",
			expected: true,
		},
		{
			name:     "preempted",
			reason:   "Preempted",
			message:  "",
			expected: true,
		},
		{
			name:     "preempted in message",
			reason:   "",
			message:  "Pod was preempted by higher priority pod",
			expected: true,
		},
		{
			name:     "node shutdown",
			reason:   "NodeShutdown",
			message:  "",
			expected: true,
		},
		{
			name:     "node affinity",
			reason:   "NodeAffinity",
			message:  "",
			expected: true,
		},
		{
			name:     "unexpected admission error",
			reason:   "UnexpectedAdmissionError",
			message:  "",
			expected: true,
		},
		{
			name:     "OOMKilled is not restartable",
			reason:   "OOMKilled",
			message:  "",
			expected: false,
		},
		{
			name:     "generic error",
			reason:   "Error",
			message:  "some error",
			expected: false,
		},
		{
			name:     "disruption target condition",
			reason:   "",
			message:  "",
			conditions: []apiv1.PodCondition{
				{
					Type:   "DisruptionTarget",
					Status: apiv1.ConditionTrue,
				},
			},
			expected: true,
		},
		{
			name:     "disruption target condition false",
			reason:   "",
			message:  "",
			conditions: []apiv1.PodCondition{
				{
					Type:   "DisruptionTarget",
					Status: apiv1.ConditionFalse,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRestartableReason(tt.reason, tt.message, tt.conditions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnalyzePodForRestart(t *testing.T) {
	tests := []struct {
		name          string
		pod           *apiv1.Pod
		tmpl          *wfv1.Template
		shouldRestart bool
		neverStarted  bool
	}{
		{
			name: "running pod should not restart",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodRunning,
				},
			},
			shouldRestart: false,
			neverStarted:  false,
		},
		{
			name: "succeeded pod should not restart",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase: apiv1.PodSucceeded,
				},
			},
			shouldRestart: false,
			neverStarted:  false,
		},
		{
			name: "evicted pod that never started should restart",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase:   apiv1.PodFailed,
					Reason:  "Evicted",
					Message: "The node had condition: [DiskPressure]",
					ContainerStatuses: []apiv1.ContainerStatus{
						{
							Name: common.MainContainerName,
							State: apiv1.ContainerState{
								Waiting: &apiv1.ContainerStateWaiting{
									Reason: "ContainerCreating",
								},
							},
						},
					},
				},
			},
			shouldRestart: true,
			neverStarted:  true,
		},
		{
			name: "evicted pod that ran should not restart",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase:   apiv1.PodFailed,
					Reason:  "Evicted",
					Message: "The node had condition: [DiskPressure]",
					ContainerStatuses: []apiv1.ContainerStatus{
						{
							Name: common.MainContainerName,
							State: apiv1.ContainerState{
								Terminated: &apiv1.ContainerStateTerminated{
									ExitCode:   137,
									StartedAt:  metav1.Now(),
									FinishedAt: metav1.Now(),
								},
							},
						},
					},
				},
			},
			shouldRestart: false,
			neverStarted:  false,
		},
		{
			name: "failed pod with non-restartable reason should not restart",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Phase:  apiv1.PodFailed,
					Reason: "OOMKilled",
					ContainerStatuses: []apiv1.ContainerStatus{
						{
							Name: common.MainContainerName,
							State: apiv1.ContainerState{
								Waiting: &apiv1.ContainerStateWaiting{},
							},
						},
					},
				},
			},
			shouldRestart: false,
			neverStarted:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzePodForRestart(tt.pod, tt.tmpl)
			assert.Equal(t, tt.shouldRestart, result.shouldRestart)
			assert.Equal(t, tt.neverStarted, result.neverStarted)
		})
	}
}

func TestGetEvictionReason(t *testing.T) {
	tests := []struct {
		name     string
		pod      *apiv1.Pod
		expected string
	}{
		{
			name: "evicted with DiskPressure",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Reason:  "Evicted",
					Message: "The node had condition: [DiskPressure]",
				},
			},
			expected: "DiskPressure",
		},
		{
			name: "evicted with MemoryPressure",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Reason:  "Evicted",
					Message: "The node had condition: [MemoryPressure]",
				},
			},
			expected: "MemoryPressure",
		},
		{
			name: "not evicted",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Reason:  "OOMKilled",
					Message: "Container killed",
				},
			},
			expected: "",
		},
		{
			name: "evicted without bracket format",
			pod: &apiv1.Pod{
				Status: apiv1.PodStatus{
					Reason:  "Evicted",
					Message: "Node out of resources",
				},
			},
			expected: "Evicted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getEvictionReason(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFailedPodRestartCount(t *testing.T) {
	t.Run("get count from empty workflow", func(t *testing.T) {
		wf := &wfv1.Workflow{}
		count := getFailedPodRestartCount(wf, "node-123")
		assert.Equal(t, int32(0), count)
	})

	t.Run("get count from workflow with no matching annotation", func(t *testing.T) {
		wf := &wfv1.Workflow{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					common.AnnotationKeyFailedPodRestartCountPrefix + "other-node": "5",
				},
			},
		}
		count := getFailedPodRestartCount(wf, "node-123")
		assert.Equal(t, int32(0), count)
	})

	t.Run("get count from workflow with matching annotation", func(t *testing.T) {
		wf := &wfv1.Workflow{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					common.AnnotationKeyFailedPodRestartCountPrefix + "node-123": "3",
				},
			},
		}
		count := getFailedPodRestartCount(wf, "node-123")
		assert.Equal(t, int32(3), count)
	})

	t.Run("set count on workflow with no annotations", func(t *testing.T) {
		wf := &wfv1.Workflow{}
		changed := setFailedPodRestartCount(wf, "node-123", 2)
		assert.True(t, changed)
		assert.Equal(t, "2", wf.Annotations[common.AnnotationKeyFailedPodRestartCountPrefix+"node-123"])
	})

	t.Run("set same count returns false", func(t *testing.T) {
		wf := &wfv1.Workflow{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					common.AnnotationKeyFailedPodRestartCountPrefix + "node-123": "2",
				},
			},
		}
		changed := setFailedPodRestartCount(wf, "node-123", 2)
		assert.False(t, changed)
	})

	t.Run("increment count", func(t *testing.T) {
		wf := &wfv1.Workflow{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					common.AnnotationKeyFailedPodRestartCountPrefix + "node-123": "2",
				},
			},
		}
		newCount := incrementFailedPodRestartCount(wf, "node-123")
		assert.Equal(t, int32(3), newCount)
		assert.Equal(t, "3", wf.Annotations[common.AnnotationKeyFailedPodRestartCountPrefix+"node-123"])
	})

	t.Run("increment count from zero", func(t *testing.T) {
		wf := &wfv1.Workflow{}
		newCount := incrementFailedPodRestartCount(wf, "node-123")
		assert.Equal(t, int32(1), newCount)
		assert.Equal(t, "1", wf.Annotations[common.AnnotationKeyFailedPodRestartCountPrefix+"node-123"])
	})
}
