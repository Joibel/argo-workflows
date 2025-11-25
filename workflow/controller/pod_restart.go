package controller

import (
	"strconv"
	"strings"

	apiv1 "k8s.io/api/core/v1"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/argoproj/argo-workflows/v3/workflow/common"
)

// RestartableFailureReason represents a failure reason that qualifies for automatic restart.
// These are typically infrastructure-level failures that are transient and not caused by user code.
type RestartableFailureReason string

const (
	// RestartableReasonEvicted indicates the pod was evicted by the kubelet due to node pressure
	// (e.g., DiskPressure, MemoryPressure, PIDPressure).
	RestartableReasonEvicted RestartableFailureReason = "Evicted"

	// RestartableReasonPreempted indicates the pod was preempted by a higher priority pod.
	RestartableReasonPreempted RestartableFailureReason = "Preempted"

	// RestartableReasonNodeShutdown indicates the pod was terminated because the node is shutting down.
	RestartableReasonNodeShutdown RestartableFailureReason = "NodeShutdown"

	// RestartableReasonNodeAffinity indicates the pod was terminated because node affinity/selector
	// no longer matches (e.g., node labels changed).
	RestartableReasonNodeAffinity RestartableFailureReason = "NodeAffinity"

	// RestartableReasonUnexpectedAdmissionError indicates an unexpected error during pod admission.
	RestartableReasonUnexpectedAdmissionError RestartableFailureReason = "UnexpectedAdmissionError"
)

// restartableReasons is the set of pod failure reasons that qualify for automatic restart.
var restartableReasons = map[RestartableFailureReason]bool{
	RestartableReasonEvicted:                  true,
	RestartableReasonPreempted:                true,
	RestartableReasonNodeShutdown:             true,
	RestartableReasonNodeAffinity:             true,
	RestartableReasonUnexpectedAdmissionError: true,
}

// PodRestartInfo contains information about whether a pod should be restarted.
type PodRestartInfo struct {
	// ShouldRestart indicates whether the pod should be restarted.
	ShouldRestart bool
	// Reason is the failure reason if the pod failed.
	Reason string
	// Message provides additional details about the failure.
	Message string
	// NeverStarted indicates whether the pod's main container never entered Running state.
	NeverStarted bool
}

// AnalyzePodForRestart analyzes a failed pod to determine if it should be automatically restarted.
// A pod qualifies for restart if:
// 1. It failed (pod.Status.Phase == PodFailed)
// 2. Its main container never entered the Running state
// 3. The failure reason is one of the restartable reasons (Evicted, Preempted, etc.)
func AnalyzePodForRestart(pod *apiv1.Pod, tmpl *wfv1.Template) PodRestartInfo {
	info := PodRestartInfo{
		ShouldRestart: false,
		Reason:        pod.Status.Reason,
		Message:       pod.Status.Message,
	}

	// Only consider pods that have failed
	if pod.Status.Phase != apiv1.PodFailed {
		return info
	}

	// Check if the main container ever started running
	info.NeverStarted = mainContainerNeverStarted(pod, tmpl)

	// If the main container ran, this is not a restart candidate
	// (the user's code executed, so this is a real failure, not an infrastructure issue)
	if !info.NeverStarted {
		return info
	}

	// Check if the failure reason is one that qualifies for restart
	if isRestartableReason(pod.Status.Reason, pod.Status.Message, pod.Status.Conditions) {
		info.ShouldRestart = true
	}

	return info
}

// mainContainerNeverStarted checks if the main container(s) never entered the Running state.
// This indicates the pod failed before any user code could execute.
func mainContainerNeverStarted(pod *apiv1.Pod, tmpl *wfv1.Template) bool {
	// First, check if any container statuses are available
	allStatuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)

	if len(allStatuses) == 0 {
		// No container statuses means the pod likely never got scheduled or started
		return true
	}

	// Check init containers first - if init container never started, main container couldn't have
	for _, status := range pod.Status.InitContainerStatuses {
		if status.Name == common.InitContainerName {
			// If init container is still waiting or terminated without ever running,
			// the main container couldn't have started
			if status.State.Running == nil && status.LastTerminationState.Running == nil {
				// Init container never ran, check if it terminated
				if status.State.Terminated != nil {
					// Init container terminated, which is expected
					continue
				}
				if status.State.Waiting != nil {
					// Init container still waiting - never started
					return true
				}
			}
		}
	}

	// Check main container(s)
	for _, status := range pod.Status.ContainerStatuses {
		isMainContainer := false
		if tmpl != nil {
			isMainContainer = tmpl.IsMainContainerName(status.Name)
		} else {
			// If no template, assume "main" container name
			isMainContainer = status.Name == common.MainContainerName
		}

		if isMainContainer {
			// Check if the main container ever entered Running state
			// We look at both State and LastTerminationState
			if status.State.Running != nil || status.LastTerminationState.Running != nil {
				// Main container entered Running state at some point
				return false
			}

			// Check if terminated - if terminated with a start time, it ran
			if status.State.Terminated != nil && !status.State.Terminated.StartedAt.IsZero() {
				// Container ran and terminated
				return false
			}

			// Container is in Waiting state or Terminated without running - never started
		}
	}

	// If we get here, no main container ever entered Running state
	return true
}

// isRestartableReason checks if the failure reason qualifies for automatic restart.
func isRestartableReason(reason string, message string, conditions []apiv1.PodCondition) bool {
	// Check direct reason match
	if _, ok := restartableReasons[RestartableFailureReason(reason)]; ok {
		return true
	}

	// Check for node condition-based evictions in the message
	// Message format: "The node had condition: [DiskPressure]"
	if reason == "Evicted" || strings.Contains(message, "The node had condition:") {
		return true
	}

	// Check for preemption indicators
	if strings.Contains(message, "Preempted") || strings.Contains(message, "preempted") {
		return true
	}

	// Check for node shutdown indicators
	if strings.Contains(message, "node is shutting down") {
		return true
	}

	// Check pod conditions for DisruptionTarget
	// This condition indicates the pod was evicted due to a disruption
	for _, cond := range conditions {
		if cond.Type == "DisruptionTarget" && cond.Status == apiv1.ConditionTrue {
			return true
		}
	}

	return false
}

// GetEvictionReason extracts the eviction reason from a pod's status message.
// Returns empty string if no eviction reason found.
func GetEvictionReason(pod *apiv1.Pod) string {
	if pod.Status.Reason != "Evicted" {
		return ""
	}

	// Try to extract the node condition from the message
	// Format: "The node had condition: [DiskPressure]"
	msg := pod.Status.Message
	if idx := strings.Index(msg, "["); idx != -1 {
		if endIdx := strings.Index(msg[idx:], "]"); endIdx != -1 {
			return msg[idx+1 : idx+endIdx]
		}
	}

	return pod.Status.Reason
}

// GetFailedPodRestartCount returns the current restart count for a node from workflow annotations.
func GetFailedPodRestartCount(wf *wfv1.Workflow, nodeID string) int32 {
	if wf.Annotations == nil {
		return 0
	}
	key := common.AnnotationKeyFailedPodRestartCountPrefix + nodeID
	if countStr, ok := wf.Annotations[key]; ok {
		if count, err := strconv.ParseInt(countStr, 10, 32); err == nil {
			return int32(count)
		}
	}
	return 0
}

// SetFailedPodRestartCount sets the restart count for a node in workflow annotations.
// Returns true if the annotation was changed.
func SetFailedPodRestartCount(wf *wfv1.Workflow, nodeID string, count int32) bool {
	if wf.Annotations == nil {
		wf.Annotations = make(map[string]string)
	}
	key := common.AnnotationKeyFailedPodRestartCountPrefix + nodeID
	newValue := strconv.FormatInt(int64(count), 10)
	if wf.Annotations[key] != newValue {
		wf.Annotations[key] = newValue
		return true
	}
	return false
}

// IncrementFailedPodRestartCount increments and returns the new restart count for a node.
func IncrementFailedPodRestartCount(wf *wfv1.Workflow, nodeID string) int32 {
	count := GetFailedPodRestartCount(wf, nodeID) + 1
	SetFailedPodRestartCount(wf, nodeID, count)
	return count
}
