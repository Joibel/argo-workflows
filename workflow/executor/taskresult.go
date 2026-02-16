package executor

import (
	"context"
	"encoding/json"
	"os"

	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-workflows/v4/pkg/apis/workflow"
	wfv1 "github.com/argoproj/argo-workflows/v4/pkg/apis/workflow/v1alpha1"
	"github.com/argoproj/argo-workflows/v4/workflow/common"
)

func (we *WorkflowExecutor) upsertTaskResult(ctx context.Context, result wfv1.NodeResult, labels map[string]string) error {
	if !we.taskResultCreated {
		err := we.createTaskResult(ctx, result, labels)
		if apierr.IsAlreadyExists(err) {
			return we.patchTaskResult(ctx, result, labels)
		}
		if err != nil {
			return err
		}
	} else {
		err := we.patchTaskResult(ctx, result, labels)
		if err != nil {
			if apierr.IsNotFound(err) {
				return we.createTaskResult(ctx, result, labels)
			}
			return err
		}
	}
	return nil
}

func (we *WorkflowExecutor) patchTaskResult(ctx context.Context, result wfv1.NodeResult, labels map[string]string) error {
	ctx, span := we.Tracing.StartPatchTaskResult(ctx)
	defer span.End()
	taskResult := &wfv1.WorkflowTaskResult{NodeResult: result}
	if len(labels) > 0 {
		taskResult.ObjectMeta = metav1.ObjectMeta{Labels: labels}
	}
	data, err := json.Marshal(taskResult)
	if err != nil {
		return err
	}
	_, err = we.taskResultClient.Patch(ctx,
		we.nodeID,
		types.MergePatchType,
		data,
		metav1.PatchOptions{},
	)
	return err
}

func (we *WorkflowExecutor) createTaskResult(ctx context.Context, result wfv1.NodeResult, labels map[string]string) error {
	ctx, span := we.Tracing.StartCreateTaskResult(ctx)
	defer span.End()
	taskResult := &wfv1.WorkflowTaskResult{
		TypeMeta: metav1.TypeMeta{
			APIVersion: workflow.APIVersion,
			Kind:       workflow.WorkflowTaskResultKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: we.nodeID,
		},
		NodeResult: result,
	}
	defaultLabels := map[string]string{
		common.LabelKeyWorkflow:               we.workflow,
		common.LabelKeyReportOutputsCompleted: "false",
	}
	for k, v := range labels {
		defaultLabels[k] = v
	}
	taskResult.SetLabels(defaultLabels)
	taskResult.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: workflow.APIVersion,
		Kind:       workflow.WorkflowKind,
		Name:       we.workflow,
		UID:        we.workflowUID,
	}})

	if v := os.Getenv(common.EnvVarInstanceID); v != "" {
		taskResult.Labels[common.LabelKeyControllerInstanceID] = v
	}
	_, err := we.taskResultClient.Create(ctx,
		taskResult,
		metav1.CreateOptions{},
	)
	we.taskResultCreated = err == nil || apierr.IsAlreadyExists(err)
	return err
}
