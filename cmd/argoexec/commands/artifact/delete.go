package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-workflows/v3/cmd/argo/commands/client"
	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	workflow "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	wfv1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	"github.com/argoproj/argo-workflows/v3/util/retry"
	"github.com/argoproj/argo-workflows/v3/util/env"
	log "github.com/sirupsen/logrus"
	waitutil "github.com/argoproj/argo-workflows/v3/util/wait"
	executor "github.com/argoproj/argo-workflows/v3/workflow/artifacts"
	"github.com/argoproj/argo-workflows/v3/workflow/common"
)

func NewArtifactDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "delete",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {

			namespace := client.Namespace()
			clientConfig := client.GetConfig()

			if podName, ok := os.LookupEnv(common.EnvVarArtifactGCPodHash); ok {

				config, err := clientConfig.ClientConfig()
				workflowInterface := workflow.NewForConfigOrDie(config)
				if err != nil {
					return err
				}

				artifactGCTaskInterface := workflowInterface.ArgoprojV1alpha1().WorkflowArtifactGCTasks(namespace)
				labelSelector := fmt.Sprintf("%s = %s", common.LabelKeyArtifactGCPodHash, podName)

				err = deleteArtifacts(labelSelector, cmd.Context(), artifactGCTaskInterface)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
}


type response struct {
	Task     *v1alpha1.WorkflowArtifactGCTask
	Err      error
}

func deleteArtifacts(labelSelector string, ctx context.Context, artifactGCTaskInterface wfv1alpha1.WorkflowArtifactGCTaskInterface) error {
	log.Infof("Welcome to delete 1.0")
	taskList, err := artifactGCTaskInterface.List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return err
	}
	taskWorkers := env.LookupEnvIntOr(common.EnvAgentTaskWorkers, 16)

	taskQueue := make(chan *v1alpha1.WorkflowArtifactGCTask)
	responseQueue := make(chan response)
	for i := 0; i < taskWorkers; i++ {
		go deleteWorker(ctx, taskQueue, responseQueue)
	}

	for _, task := range taskList.Items {
		taskQueue <- &task
	}
	close(taskQueue)
	completed := 0
	for {
		response, _ := <- responseQueue
		log.Infof("Delete response received, completed %d", completed)
		task, err := response.Task, response.Err
		if err != nil {
			return err
		}
		if task == nil {
			log.Infof("Completed")
			completed++
			if completed >= taskWorkers {
				break
			}
		} else {
			patch, err := json.Marshal(map[string]interface{}{"status": v1alpha1.ArtifactGCStatus{ArtifactResultsByNode: task.Status.ArtifactResultsByNode}})
			if err != nil {
				return err
			}
			_, err = artifactGCTaskInterface.Patch(context.Background(), task.Name, types.MergePatchType, patch, metav1.PatchOptions{}, "status")
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func deleteWorker(ctx context.Context, taskQueue chan *v1alpha1.WorkflowArtifactGCTask, responseQueue chan response) {
	for {
		task, ok := <- taskQueue
		if !ok {
			// Done
			log.Infof("Worker is done")
			log.Infof("Worker sending done")
			responseQueue <- response{Task: nil, Err: nil}
			return
		}
		log.Infof("Worker has task")
		task.Status.ArtifactResultsByNode = make(map[string]v1alpha1.ArtifactResultNodeStatus)
		for nodeName, artifactNodeSpec := range task.Spec.ArtifactsByNode {
			var archiveLocation *v1alpha1.ArtifactLocation
			artResultNodeStatus := v1alpha1.ArtifactResultNodeStatus{ArtifactResults: make(map[string]v1alpha1.ArtifactResult)}
			if artifactNodeSpec.ArchiveLocation != nil {
				archiveLocation = artifactNodeSpec.ArchiveLocation
			}

			var resources resources
			resources.Files = make(map[string][]byte) // same resources for every artifact
			for _, artifact := range artifactNodeSpec.Artifacts {
				if archiveLocation != nil {
					err := artifact.Relocate(archiveLocation)
					if err != nil {
						responseQueue <- response{Task: task, Err: err}
						continue
					}
				}
				drv, err := executor.NewDriver(ctx, &artifact, resources)
				if err != nil {
					responseQueue <- response{Task: task, Err: err}
					continue
				}

				err = waitutil.Backoff(retry.DefaultRetry, func() (bool, error) {
					err = drv.Delete(&artifact)
					if err != nil {
						errString := err.Error()
						artResultNodeStatus.ArtifactResults[artifact.Name] = v1alpha1.ArtifactResult{Name: artifact.Name, Success: false, Error: &errString}
						return false, err
					}
					artResultNodeStatus.ArtifactResults[artifact.Name] = v1alpha1.ArtifactResult{Name: artifact.Name, Success: true, Error: nil}
					return true, err
				})
			}
			task.Status.ArtifactResultsByNode[nodeName] = artResultNodeStatus
		}
		log.Infof("Worker sending complete")
		responseQueue <- response{Task: task, Err: nil}
	}
}

type resources struct {
	Files map[string][]byte
}

func (r resources) GetSecret(ctx context.Context, name, key string) (string, error) {

	path := filepath.Join(common.SecretVolMountPath, name, key)
	if file, ok := r.Files[path]; ok {
		return string(file), nil
	}

	file, err := os.ReadFile(path)
	if err != nil {
		return "", err
	} else {
		r.Files[path] = file
		return string(file), err
	}
}

func (r resources) GetConfigMapKey(ctx context.Context, name, key string) (string, error) {
	return "", fmt.Errorf("not supported")
}
