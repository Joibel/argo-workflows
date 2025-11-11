// Package v1alpha1 contains API Schema definitions for Argo Workflows v1alpha1.
//
// This package defines the core Kubernetes Custom Resource Definitions (CRDs) for Argo Workflows,
// including Workflow, WorkflowTemplate, ClusterWorkflowTemplate, and CronWorkflow resources.
//
// # Overview
//
// The v1alpha1 API provides Go types for all Argo Workflows resources:
//
//   - Workflow - A single workflow execution
//   - WorkflowTemplate - Reusable workflow definition (namespace-scoped)
//   - ClusterWorkflowTemplate - Reusable workflow definition (cluster-scoped)
//   - CronWorkflow - Scheduled workflow execution
//   - WorkflowEventBinding - Event-driven workflow triggers
//   - WorkflowTaskSet - Container for task execution
//   - WorkflowTaskResult - Task execution results
//   - WorkflowArtifactGCTask - Artifact garbage collection
//
// # Basic Usage
//
// Creating a workflow:
//
//	import (
//	    wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
//	    corev1 "k8s.io/api/core/v1"
//	    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//	)
//
//	workflow := &wfv1.Workflow{
//	    ObjectMeta: metav1.ObjectMeta{
//	        GenerateName: "my-workflow-",
//	    },
//	    Spec: wfv1.WorkflowSpec{
//	        Entrypoint: "main",
//	        Templates: []wfv1.Template{
//	            {
//	                Name: "main",
//	                Container: &corev1.Container{
//	                    Image:   "busybox",
//	                    Command: []string{"echo", "hello world"},
//	                },
//	            },
//	        },
//	    },
//	}
//
// # Workflow Structure
//
// A Workflow consists of:
//   - Metadata: Name, namespace, labels, annotations
//   - Spec: Workflow definition (entrypoint, templates, arguments, etc.)
//   - Status: Execution state (phase, nodes, outputs, etc.)
//
// # Template Types
//
// Templates define the steps in a workflow:
//
//   - Container - Run a container
//   - ContainerSet - Run multiple containers
//   - Script - Run a script
//   - Resource - Manipulate Kubernetes resources
//   - DAG - Direct Acyclic Graph of tasks
//   - Steps - Sequential/parallel steps
//   - Suspend - Pause workflow execution
//   - HTTP - Make HTTP requests
//   - Plugin - Custom plugin execution
//   - Data - Data transformations
//
// # Workflow Phases
//
// Workflows progress through these phases:
//
//   - Pending - Workflow is queued
//   - Running - Workflow is executing
//   - Succeeded - Workflow completed successfully
//   - Failed - Workflow failed
//   - Error - Workflow encountered an error
//   - Skipped - Workflow was skipped
//   - Omitted - Workflow was omitted
//
// # Parameters and Arguments
//
// Pass data to workflows:
//
//	workflow.Spec.Arguments = wfv1.Arguments{
//	    Parameters: []wfv1.Parameter{
//	        {Name: "message", Value: wfv1.AnyStringPtr("Hello")},
//	    },
//	}
//
// Access in templates:
//
//	Args: []string{"{{inputs.parameters.message}}"}
//
// # Artifacts
//
// Define input/output artifacts:
//
//	template.Inputs = wfv1.Inputs{
//	    Artifacts: []wfv1.Artifact{
//	        {
//	            Name: "data",
//	            Path: "/tmp/data",
//	            S3: &wfv1.S3Artifact{
//	                Key: "my-data.txt",
//	            },
//	        },
//	    },
//	}
//
// # Node Status
//
// Track execution progress:
//
//	for nodeName, node := range workflow.Status.Nodes {
//	    fmt.Printf("Node %s: %s\n", nodeName, node.Phase)
//	}
//
// # WorkflowTemplate
//
// Create reusable templates:
//
//	template := &wfv1.WorkflowTemplate{
//	    ObjectMeta: metav1.ObjectMeta{
//	        Name: "my-template",
//	    },
//	    Spec: wfv1.WorkflowSpec{
//	        Entrypoint: "main",
//	        Templates: []wfv1.Template{...},
//	    },
//	}
//
// Reference in workflows:
//
//	workflow.Spec.WorkflowTemplateRef = &wfv1.WorkflowTemplateRef{
//	    Name: "my-template",
//	}
//
// # CronWorkflow
//
// Schedule workflows:
//
//	cronWf := &wfv1.CronWorkflow{
//	    ObjectMeta: metav1.ObjectMeta{
//	        Name: "my-cron",
//	    },
//	    Spec: wfv1.CronWorkflowSpec{
//	        Schedule: "*/5 * * * *", // Every 5 minutes
//	        WorkflowSpec: wfv1.WorkflowSpec{...},
//	    },
//	}
//
// # Helper Methods
//
// The types provide useful helper methods:
//
//	// Check if workflow is complete
//	if !workflow.Status.FinishedAt.IsZero() {
//	    // Workflow finished
//	}
//
//	// Get workflow phase
//	phase := workflow.Status.Phase
//
//	// Check if node succeeded
//	if node.Phase == wfv1.NodeSucceeded {
//	    // Node succeeded
//	}
//
// # Type Constants
//
// Common constants are defined:
//
//   - NodePhases: Pending, Running, Succeeded, Failed, Error, Skipped, Omitted
//   - NodeTypes: Pod, Container, Steps, StepGroup, DAG, TaskGroup, Retry
//   - TemplateTypes: Container, ContainerSet, Script, Resource, DAG, Steps, Suspend, HTTP, Plugin, Data
//   - ArtifactGCStrategy: OnWorkflowCompletion, OnWorkflowDeletion, Never
//
// # Examples
//
// See examples directory for complete examples:
//   - examples/go-sdk/basic-workflow/
//   - examples/go-sdk/workflow-template/
//   - examples/*.yaml - 270+ workflow examples
//
// # Documentation
//
// For more information:
//   - Go SDK Guide: docs/go-sdk-guide.md
//   - Workflow Spec: https://argo-workflows.readthedocs.io/en/latest/fields/
//   - Examples: https://github.com/argoproj/argo-workflows/tree/main/examples
//
// +groupName=argoproj.io
// +k8s:deepcopy-gen=package,register
// +k8s:openapi-gen=true
package v1alpha1
