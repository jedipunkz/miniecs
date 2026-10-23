package cmd

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
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

// loginECSClient is the subset of *myecs.ECSResource that the login flow
// uses. Defining it here keeps the resource list logic testable with a
// cmd-local fake instead of a real AWS client.
type loginECSClient interface {
	ListClusters(ctx context.Context) error
	GetClusterResources(ctx context.Context, cluster myecs.ECSCluster) ([]myecs.ECSResource, error)
}

func runLoginCmd(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	ecsClient, err := initializeECSClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if err := ecsClient.ListClusters(ctx); err != nil {
		log.Fatal(err)
	}

	ecsResources, err := fetchAllECSResources(ctx, ecsClient, ecsClient.Clusters)
	if err != nil {
		log.Fatal(err)
	}

	selected, err := showResourcePicker(ecsResources)
	if err != nil {
		log.Fatal(err)
	}

	if err := executeLogin(ecsClient, selected); err != nil {
		log.Fatal(err)
	}
}

func initializeECSClient(ctx context.Context) (*myecs.ECSResource, error) {
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

func fetchAllECSResources(ctx context.Context, ecsClient loginECSClient, allClusters []myecs.ECSCluster) ([]myecs.ECSResource, error) {
	targetClusters, err := selectTargetClusters(allClusters, loginSetFlags.cluster)
	if err != nil {
		return nil, err
	}

	var ecsResources []myecs.ECSResource
	for _, cluster := range targetClusters {
		resources, err := ecsClient.GetClusterResources(ctx, cluster)
		if err != nil {
			return nil, err
		}
		ecsResources = append(ecsResources, resources...)
	}

	return ecsResources, nil
}

// selectTargetClusters returns the clusters to fetch resources for. With an
// empty name it returns all clusters; otherwise it returns only the named
// cluster and fails when that cluster does not exist.
func selectTargetClusters(allClusters []myecs.ECSCluster, name string) ([]myecs.ECSCluster, error) {
	if name == "" {
		return allClusters, nil
	}

	for _, cluster := range allClusters {
		if cluster.ClusterName == name {
			return []myecs.ECSCluster{cluster}, nil
		}
	}

	return nil, fmt.Errorf("cluster %q not found", name)
}

type selectableItem struct {
	resourceIndex int
	cluster       myecs.ECSCluster
	service       myecs.ECSService
	task          myecs.ECSTask
	container     myecs.ECSContainer
}

func buildSelectableItems(ecsResources []myecs.ECSResource) []selectableItem {
	var items []selectableItem
	for idx, resource := range ecsResources {
		items = append(items, extractItemsFromResource(idx, resource)...)
	}
	return items
}

func extractItemsFromResource(resourceIndex int, resource myecs.ECSResource) []selectableItem {
	if len(resource.Clusters) == 0 {
		return nil
	}
	cluster := resource.Clusters[0]
	return extractItemsFromServices(resourceIndex, cluster)
}

func extractItemsFromServices(resourceIndex int, cluster myecs.ECSCluster) []selectableItem {
	var items []selectableItem
	for _, service := range cluster.Services {
		items = append(items, extractItemsFromTasks(resourceIndex, cluster, service)...)
	}
	return items
}

func extractItemsFromTasks(resourceIndex int, cluster myecs.ECSCluster, service myecs.ECSService) []selectableItem {
	var items []selectableItem
	for _, task := range service.Tasks {
		items = append(items, extractItemsFromContainers(resourceIndex, cluster, service, task)...)
	}
	return items
}

func extractItemsFromContainers(resourceIndex int, cluster myecs.ECSCluster, service myecs.ECSService, task myecs.ECSTask) []selectableItem {
	var items []selectableItem
	for _, container := range task.Containers {
		items = append(items, selectableItem{
			resourceIndex: resourceIndex,
			cluster:       cluster,
			service:       service,
			task:          task,
			container:     container,
		})
	}
	return items
}

func showResourcePicker(ecsResources []myecs.ECSResource) (selectableItem, error) {
	items := buildSelectableItems(ecsResources)
	if len(items) == 0 {
		return selectableItem{}, fmt.Errorf("no ECS resources found")
	}

	// Only one interactive session can be attached at a time, so use
	// single selection instead of FindMulti. FindMulti results were
	// otherwise silently truncated to the first entry.
	itemIndex, err := fuzzyfinder.Find(
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
		return selectableItem{}, err
	}

	return items[itemIndex], nil
}

func executeLogin(ecsClient *myecs.ECSResource, selected selectableItem) error {
	commandInput := createExecuteCommandInput(selected)

	log.WithFields(log.Fields{
		"cluster":   *commandInput.Cluster,
		"task":      *commandInput.Task,
		"container": *commandInput.Container,
		"command":   *commandInput.Command,
	}).Info("ECS Execute Login with These Parameters")

	return ecsClient.ExecuteCommand(commandInput)
}

func createExecuteCommandInput(item selectableItem) ecs.ExecuteCommandInput {
	shell := getShell()
	clusterName := item.cluster.ClusterName
	containerName := item.container.ContainerName
	taskArn := item.task.TaskArn

	return ecs.ExecuteCommandInput{
		Cluster:   aws.String(clusterName),
		Container: aws.String(containerName),
		Task:      aws.String(taskArn),
		Command:   aws.String(shell),
	}
}

func getShell() string {
	if loginSetFlags.shell != "" {
		return loginSetFlags.shell
	}
	return "sh"
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
