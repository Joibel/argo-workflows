package controller

import (
	"strconv"
	"strings"

	apiv1 "k8s.io/api/core/v1"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/argoproj/argo-workflows/v3/workflow/common"
)

// restartableFailureReason represents a failure reason that qualifies for automatic restart.
// These are typically infrastructure-level failures that are transient and not caused by user code.
type restartableFailureReason string

const (
	// restartableReasonEvicted indicates the pod was evicted by the kubelet due to node pressure
	// (e.g., DiskPressure, MemoryPressure, PIDPressure).
	restartableReasonEvicted restartableFailureReason = "Evicted"

	// restartableReasonPreempted indicates the pod was preempted by a higher priority pod.
	restartableReasonPreempted restartableFailureReason = "Preempted"

	// restartableReasonNodeShutdown indicates the pod was terminated because the node is shutting down.
	restartableReasonNodeShutdown restartableFailureReason = "NodeShutdown"

	// restartableReasonNodeAffinity indicates the pod was terminated because node affinity/selector
	// no longer matches (e.g., node labels changed).
	restartableReasonNodeAffinity restartableFailureReason = "NodeAffinity"

	// restartableReasonUnexpectedAdmissionError indicates an unexpected error during pod admission.
	restartableReasonUnexpectedAdmissionError restartableFailureReason = "UnexpectedAdmissionError"
)

// restartableReasons is the set of pod failure reasons that qualify for automatic restart.
var restartableReasons = map[restartableFailureReason]bool{
	restartableReasonEvicted:                  true,
	restartableReasonPreempted:                true,
	restartableReasonNodeShutdown:             true,
	restartableReasonNodeAffinity:             true,
	restartableReasonUnexpectedAdmissionError: true,
}

// podRestartInfo contains information about whether a pod should be restarted.
type podRestartInfo struct {
	// shouldRestart indicates whether the pod should be restarted.
	shouldRestart bool
	// reason is the failure reason if the pod failed.
	reason string
	// message provides additional details about the failure.
	message string
	// neverStarted indicates whether the pod's main container never entered Running state.
	neverStarted bool
}

// analyzePodForRestart analyzes a failed pod to determine if it should be automatically restarted.
// A pod qualifies for restart if:
// 1. It failed (pod.Status.Phase == PodFailed)
// 2. Its main container never entered the Running state
// 3. The failure reason is one of the restartable reasons (Evicted, Preempted, etc.)
func analyzePodForRestart(pod *apiv1.Pod, tmpl *wfv1.Template) podRestartInfo {
	info := podRestartInfo{
		shouldRestart: false,
		reason:        pod.Status.Reason,
		message:       pod.Status.Message,
	}

	// Only consider pods that have failed
	if pod.Status.Phase != apiv1.PodFailed {
		return info
	}

	// Check if the main container ever started running
	info.neverStarted = mainContainerNeverStarted(pod, tmpl)

	// If the main container ran, this is not a restart candidate
	// (the user's code executed, so this is a real failure, not an infrastructure issue)
	if !info.neverStarted {
		return info
	}

	// Check if the failure reason is one that qualifies for restart
	if isRestartableReason(pod.Status.Reason, pod.Status.Message, pod.Status.Conditions) {
		info.shouldRestart = true
	}

	return info
}

// mainContainerNeverStarted checks if the main container(s) never entered the Running state.
// This indicates the pod failed before any user code could execute.
func mainContainerNeverStarted(pod *apiv1.Pod, tmpl *wfv1.Template) bool {
	if len(pod.Status.ContainerStatuses) == 0 {
		// No container statuses means the pod likely never got scheduled or started
		return true
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
	if _, ok := restartableReasons[restartableFailureReason(reason)]; ok {
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

// getEvictionReason extracts the eviction reason from a pod's status message.
// Returns empty string if no eviction reason found.
func getEvictionReason(pod *apiv1.Pod) string {
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

// getFailedPodRestartCount returns the current restart count for a node from workflow annotations.
func getFailedPodRestartCount(wf *wfv1.Workflow, nodeID string) int32 {
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

// setFailedPodRestartCount sets the restart count for a node in workflow annotations.
// Returns true if the annotation was changed.
func setFailedPodRestartCount(wf *wfv1.Workflow, nodeID string, count int32) bool {
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

// incrementFailedPodRestartCount increments and returns the new restart count for a node.
func incrementFailedPodRestartCount(wf *wfv1.Workflow, nodeID string) int32 {
	count := getFailedPodRestartCount(wf, nodeID) + 1
	setFailedPodRestartCount(wf, nodeID, count)
	return count
}
