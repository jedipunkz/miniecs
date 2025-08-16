package cmd

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/ktr0731/go-fuzzyfinder"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type loginFlags struct {
	region  string
	cluster string
	shell   string
}

var loginSetFlags loginFlags

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "login cluster, service",
	Run:   runLoginCmd,
}

func runLoginCmd(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	ecsClient, err := initializeECSClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	ecsResources, err := fetchAllECSResources(ctx, ecsClient)
	if err != nil {
		log.Fatal(err)
	}

	selectedResources, err := showResourcePicker(ecsResources)
	if err != nil {
		log.Fatal(err)
	}

	if err := executeLogin(ecsClient, selectedResources); err != nil {
		log.Fatal(err)
	}
}

func initializeECSClient(ctx context.Context) (*myecs.ECSResource, error) {
	// AWS SDKの設定を直接読み込む（シンプルで理解しやすい）
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(loginSetFlags.region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	ecsClient := myecs.NewECS(cfg, loginSetFlags.region)
	if ecsClient == nil {
		return nil, fmt.Errorf("failed to initialize ECS client")
	}

	return ecsClient, nil
}

func fetchAllECSResources(ctx context.Context, ecsClient *myecs.ECSResource) ([]myecs.ECSResource, error) {
	var ecsResources []myecs.ECSResource

	if err := ecsClient.ListClusters(ctx); err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	for _, cluster := range ecsClient.Clusters {
		resources, err := ecsClient.GetClusterResources(ctx, cluster)
		if err != nil {
			return nil, err
		}
		ecsResources = append(ecsResources, resources...)
	}

	return ecsResources, nil
}

func showResourcePicker(ecsResources []myecs.ECSResource) ([]myecs.ECSResource, error) {
	// Build a flat list of selectable items
	type selectableItem struct {
		resourceIndex int
		cluster       myecs.ECSCluster
		service       myecs.ECSService
		task          myecs.ECSTask
		container     myecs.ECSContainer
	}

	var items []selectableItem
	for idx, resource := range ecsResources {
		if len(resource.Clusters) == 0 {
			continue
		}
		cluster := resource.Clusters[0]
		if len(cluster.Services) > 0 {
			for _, service := range cluster.Services {
				if len(service.Tasks) > 0 {
					for _, task := range service.Tasks {
						if len(task.Containers) > 0 {
							for _, container := range task.Containers {
								items = append(items, selectableItem{
									resourceIndex: idx,
									cluster:       cluster,
									service:       service,
									task:          task,
									container:     container,
								})
							}
						}
					}
				}
			}
		}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no ECS resources found")
	}

	selectedIndices, err := fuzzyfinder.FindMulti(
		items,
		func(i int) string {
			return fmt.Sprintf("%s %s %s",
				items[i].cluster.ClusterName,
				items[i].service.ServiceName,
				items[i].container.ContainerName,
			)
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			return fmt.Sprintf(
				"Cluster: %s\nService: %s\nContainer: %s\n",
				items[i].cluster.ClusterName,
				items[i].service.ServiceName,
				items[i].container.ContainerName,
			)
		}),
	)

	if err != nil {
		return nil, err
	}

	// Create resources with selected items
	var selectedResources []myecs.ECSResource
	for _, idx := range selectedIndices {
		selectedItem := items[idx]
		// Create a new resource with the selected item data
		resource := myecs.ECSResource{
			Clusters: []myecs.ECSCluster{{
				ClusterName: selectedItem.cluster.ClusterName,
				ClusterArn:  selectedItem.cluster.ClusterArn,
				Services: []myecs.ECSService{{
					ServiceName: selectedItem.service.ServiceName,
					ServiceArn:  selectedItem.service.ServiceArn,
					ClusterName: selectedItem.cluster.ClusterName,
					Tasks: []myecs.ECSTask{{
						TaskArn:        selectedItem.task.TaskArn,
						TaskDefinition: selectedItem.task.TaskDefinition,
						ServiceName:    selectedItem.service.ServiceName,
						ClusterName:    selectedItem.cluster.ClusterName,
						Containers: []myecs.ECSContainer{{
							ContainerName: selectedItem.container.ContainerName,
							ContainerArn:  selectedItem.container.ContainerArn,
							TaskArn:       selectedItem.task.TaskArn,
							Image:         selectedItem.container.Image,
							Status:        selectedItem.container.Status,
						}},
					}},
				}},
			}},
		}
		selectedResources = append(selectedResources, resource)
	}

	return selectedResources, nil
}

func executeLogin(ecsClient *myecs.ECSResource, selectedResources []myecs.ECSResource) error {
	if len(selectedResources) == 0 {
		return fmt.Errorf("no resource selected")
	}
	selectedResource := selectedResources[0]
	commandInput := createExecuteCommandInput(selectedResource)

	log.WithFields(log.Fields{
		"cluster":   *commandInput.Cluster,
		"task":      *commandInput.Task,
		"container": *commandInput.Container,
		"command":   *commandInput.Command,
	}).Info("ECS Execute Login with These Parameters")

	return ecsClient.ExecuteCommand(commandInput)
}

func createExecuteCommandInput(resource myecs.ECSResource) ecs.ExecuteCommandInput {
	shell := loginSetFlags.shell
	if shell == "" {
		shell = "sh"
	}

	// Extract task and container information from the hierarchical structure
	var containerName, taskArn string
	if len(resource.Clusters) > 0 && len(resource.Clusters[0].Services) > 0 {
		service := resource.Clusters[0].Services[0]
		if len(service.Tasks) > 0 {
			task := service.Tasks[0]
			taskArn = task.TaskArn
			if len(task.Containers) > 0 {
				containerName = task.Containers[0].ContainerName
			}
		}
	}

	return ecs.ExecuteCommandInput{
		Cluster:   &resource.Clusters[0].ClusterName,
		Container: &containerName,
		Task:      &taskArn,
		Command:   &shell,
	}
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.Flags().StringVarP(
		&loginSetFlags.region, "region", "", "", "Region Name")
	if err := loginCmd.MarkFlagRequired("region"); err != nil {
		log.Fatal(err)
	}
	loginCmd.Flags().StringVarP(
		&loginSetFlags.cluster, "cluster", "", "", "ECS Cluster Name")
	loginCmd.Flags().StringVarP(
		&loginSetFlags.shell, "shell", "", "", "Login Shell")
}
