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

type configLoader interface {
	LoadDefaultConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error)
}

type defaultConfigLoader struct{}

func (d *defaultConfigLoader) LoadDefaultConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx, optFns...)
}

var defaultLoader configLoader = &defaultConfigLoader{}

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

	selectedIndices, err := showResourcePicker(ecsResources)
	if err != nil {
		log.Fatal(err)
	}

	if err := executeLogin(ecsClient, selectedIndices, ecsResources); err != nil {
		log.Fatal(err)
	}
}

func initializeECSClient(ctx context.Context) (*myecs.ECSResource, error) {
	cfg, err := defaultLoader.LoadDefaultConfig(ctx, config.WithRegion(loginSetFlags.region))
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

func showResourcePicker(ecsResources []myecs.ECSResource) ([]int, error) {
	return fuzzyfinder.FindMulti(
		ecsResources,
		func(i int) string {
			if len(ecsResources[i].Clusters) > 0 {
				return ecsResources[i].Clusters[0].ClusterName
			}
			return ""
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			if len(ecsResources[i].Clusters) > 0 {
				return fmt.Sprintf(
					"Cluster: %s\n",
					ecsResources[i].Clusters[0].ClusterName,
				)
			}
			return ""
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
		Container: &shell,
		Task:      &shell,
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
