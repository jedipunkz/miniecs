package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/ktr0731/go-fuzzyfinder"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	defaultShell = "sh"
)

var loginSetFlags struct {
	region  string
	cluster string
	shell   string
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "login cluster, service",
	Run:   runLoginCmd,
}

func initECSClient(ctx context.Context, region string) (*myecs.ECSResource, error) {
	if os.Getenv("AWS_PROFILE") == "" {
		return nil, fmt.Errorf("AWS_PROFILE environment variable is not set")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	e := myecs.NewEcs(cfg, region)
	if e == nil {
		return nil, fmt.Errorf("failed to initialize ECS client")
	}

	return e, nil
}

func collectECSResources(ctx context.Context, e *myecs.ECSResource) ([]myecs.ECSResource, error) {
	var ecsResources []myecs.ECSResource

	if err := e.ListClusters(ctx); err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	for _, cluster := range e.Clusters {
		if err := e.ListServices(ctx, cluster.ClusterName); err != nil {
			return nil, fmt.Errorf("failed to list services for cluster %s: %w", cluster.ClusterName, err)
		}

		for _, service := range e.Services {
			resources, err := getResourcesForService(ctx, e, cluster, service)
			if err != nil {
				return nil, err
			}
			ecsResources = append(ecsResources, resources...)
		}
	}

	return ecsResources, nil
}

func getResourcesForService(ctx context.Context, e *myecs.ECSResource, cluster myecs.Cluster, service myecs.Service) ([]myecs.ECSResource, error) {
	var resources []myecs.ECSResource

	if err := e.GetTasks(ctx, cluster.ClusterName, service.ServiceName); err != nil {
		return nil, fmt.Errorf("failed to get tasks for service %s: %w", service.ServiceName, err)
	}

	if len(e.Tasks) == 0 {
		return resources, nil
	}

	task := e.Tasks[0]
	e.Containers = nil

	if err := e.ListContainers(ctx, task.TaskDefinition); err != nil {
		return nil, fmt.Errorf("failed to list containers for task %s: %w", task.TaskDefinition, err)
	}

	for _, container := range e.Containers {
		ecsResource := myecs.ECSResource{
			Clusters:   []myecs.Cluster{cluster},
			Services:   []myecs.Service{service},
			Tasks:      []myecs.Task{task},
			Containers: []myecs.Container{container},
		}
		resources = append(resources, ecsResource)
	}

	return resources, nil
}

func selectResource(resources []myecs.ECSResource) ([]int, error) {
	return fuzzyfinder.FindMulti(
		resources,
		func(i int) string {
			return fmt.Sprintf("%s %s %s",
				resources[i].Clusters[0].ClusterName,
				resources[i].Services[0].ServiceName,
				resources[i].Containers[0].ContainerName)
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			return fmt.Sprintf(
				"Cluster: %s\nService: %s\nContainer: %s\n",
				resources[i].Clusters[0].ClusterName,
				resources[i].Services[0].ServiceName,
				resources[i].Containers[0].ContainerName,
			)
		}),
	)
}

func executeLogin(e *myecs.ECSResource, resource myecs.ECSResource) error {
	shell := loginSetFlags.shell
	if shell == "" {
		shell = defaultShell
	}

	input := ecs.ExecuteCommandInput{
		Cluster:   &resource.Clusters[0].ClusterName,
		Container: &resource.Containers[0].ContainerName,
		Task:      &resource.Tasks[0].TaskArn,
		Command:   &shell,
	}

	log.WithFields(log.Fields{
		"cluster":   *input.Cluster,
		"task":      *input.Task,
		"container": *input.Container,
		"command":   *input.Command,
	}).Info("ECS Execute Login with These Parameters")

	return e.ExecuteCommand(input)
}

func runLoginCmd(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	e, err := initECSClient(ctx, loginSetFlags.region)
	if err != nil {
		log.Fatal(err)
	}

	resources, err := collectECSResources(ctx, e)
	if err != nil {
		log.Fatal(err)
	}

	selectedIdx, err := selectResource(resources)
	if err != nil {
		log.Fatal(err)
	}

	if err := executeLogin(e, resources[selectedIdx[0]]); err != nil {
		log.Fatal(err)
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
