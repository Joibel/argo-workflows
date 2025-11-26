package controller

import (
	"strings"

	apiv1 "k8s.io/api/core/v1"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/argoproj/argo-workflows/v3/workflow/common"
)

// podRestartInfo contains information about whether a pod should be restarted.
type podRestartInfo struct {
	shouldRestart bool
	reason        string
	neverStarted  bool
}

// analyzePodForRestart analyzes a failed pod to determine if it should be automatically restarted.
// A pod qualifies for restart if:
// 1. It failed (pod.Status.Phase == PodFailed)
// 2. Its main container never entered the Running state
// 3. The failure reason is a restartable infrastructure failure
func analyzePodForRestart(pod *apiv1.Pod, tmpl *wfv1.Template) podRestartInfo {
	info := podRestartInfo{
		reason: pod.Status.Reason,
	}

	if pod.Status.Phase != apiv1.PodFailed {
		return info
	}

	info.neverStarted = mainContainerNeverStarted(pod, tmpl)
	if !info.neverStarted {
		return info
	}

	info.shouldRestart = isRestartableReason(pod.Status.Reason)
	return info
}

// mainContainerNeverStarted checks if the main container(s) never entered the Running state.
func mainContainerNeverStarted(pod *apiv1.Pod, tmpl *wfv1.Template) bool {
	if len(pod.Status.ContainerStatuses) == 0 {
		return true
	}

	for _, status := range pod.Status.ContainerStatuses {
		isMainContainer := false
		if tmpl != nil {
			isMainContainer = tmpl.IsMainContainerName(status.Name)
		} else {
			isMainContainer = status.Name == common.MainContainerName
		}

		if isMainContainer {
			if status.State.Running != nil || status.LastTerminationState.Running != nil {
				return false
			}
			if status.State.Terminated != nil && !status.State.Terminated.StartedAt.IsZero() {
				return false
			}
		}
	}

	return true
}

// isRestartableReason checks if the pod failure reason qualifies for automatic restart.
// These reasons indicate infrastructure-level failures set by the kubelet:
// - Evicted: node pressure eviction (DiskPressure, MemoryPressure, etc.)
// - NodeShutdown: graceful node shutdown
// - NodeAffinity: node affinity/selector no longer matches
// - UnexpectedAdmissionError: unexpected error during pod admission
func isRestartableReason(reason string) bool {
	switch reason {
	case "Evicted", "NodeShutdown", "NodeAffinity", "UnexpectedAdmissionError":
		return true
	}
	return false
}

// getEvictionReason extracts the eviction reason from a pod's status message.
// Returns empty string if no eviction reason found.
func getEvictionReason(pod *apiv1.Pod) string {
	if pod.Status.Reason != "Evicted" {
		return ""
	}

	// Format: "The node had condition: [DiskPressure]"
	msg := pod.Status.Message
	if idx := strings.Index(msg, "["); idx != -1 {
		if endIdx := strings.Index(msg[idx:], "]"); endIdx != -1 {
			return msg[idx+1 : idx+endIdx]
		}
	}

	return pod.Status.Reason
}
