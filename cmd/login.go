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

	ecsResources, err := collectECSResources(ctx, ecsClient)
	if err != nil {
		log.Fatal(err)
	}

	selectedIndices, err := selectECSResource(ecsResources)
	if err != nil {
		log.Fatal(err)
	}

	if err := executeLogin(ecsClient, selectedIndices, ecsResources); err != nil {
		log.Fatal(err)
	}
}

func initializeECSClient(ctx context.Context) (*myecs.ECSResource, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(loginSetFlags.region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	ecsClient := myecs.NewEcs(cfg, loginSetFlags.region)
	if ecsClient == nil {
		return nil, fmt.Errorf("failed to initialize ECS client")
	}

	return ecsClient, nil
}

func collectECSResources(ctx context.Context, ecsClient *myecs.ECSResource) ([]myecs.ECSResource, error) {
	var ecsResources []myecs.ECSResource

	if err := ecsClient.ListClusters(ctx); err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	for _, cluster := range ecsClient.Clusters {
		if err := collectServicesAndContainers(ctx, ecsClient, cluster, &ecsResources); err != nil {
			return nil, err
		}
	}

	return ecsResources, nil
}

func collectServicesAndContainers(ctx context.Context, ecsClient *myecs.ECSResource, cluster myecs.Cluster, ecsResources *[]myecs.ECSResource) error {
	if err := ecsClient.ListServices(ctx, cluster.ClusterName); err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	for _, service := range ecsClient.Services {
		if err := collectTasksAndContainers(ctx, ecsClient, cluster, service, ecsResources); err != nil {
			return err
		}
	}

	return nil
}

func collectTasksAndContainers(ctx context.Context, ecsClient *myecs.ECSResource, cluster myecs.Cluster, service myecs.Service, ecsResources *[]myecs.ECSResource) error {
	if err := ecsClient.GetTasks(ctx, cluster.ClusterName, service.ServiceName); err != nil {
		return fmt.Errorf("failed to get tasks: %w", err)
	}

	if len(ecsClient.Tasks) == 0 {
		return nil
	}

	task := ecsClient.Tasks[0]
	ecsClient.Containers = nil

	if err := ecsClient.ListContainers(ctx, task.TaskDefinition); err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, container := range ecsClient.Containers {
		*ecsResources = append(*ecsResources, myecs.ECSResource{
			Clusters:   []myecs.Cluster{cluster},
			Services:   []myecs.Service{service},
			Tasks:      []myecs.Task{task},
			Containers: []myecs.Container{container},
		})
	}

	return nil
}

func selectECSResource(ecsResources []myecs.ECSResource) ([]int, error) {
	return fuzzyfinder.FindMulti(
		ecsResources,
		func(i int) string {
			return fmt.Sprintf("%s %s %s",
				ecsResources[i].Clusters[0].ClusterName,
				ecsResources[i].Services[0].ServiceName,
				ecsResources[i].Containers[0].ContainerName,
			)
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			return fmt.Sprintf(
				"Cluster: %s\nService: %s\nContainer: %s\n",
				ecsResources[i].Clusters[0].ClusterName,
				ecsResources[i].Services[0].ServiceName,
				ecsResources[i].Containers[0].ContainerName,
			)
		}),
	)
}

func executeLogin(ecsClient *myecs.ECSResource, selectedIndices []int, ecsResources []myecs.ECSResource) error {
	selectedResource := ecsResources[selectedIndices[0]]
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

	return ecs.ExecuteCommandInput{
		Cluster:   &resource.Clusters[0].ClusterName,
		Container: &resource.Containers[0].ContainerName,
		Task:      &resource.Tasks[0].TaskArn,
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
