package cmd

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var listSetFlags struct {
	region  string
	cluster string
	shell   string
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list clusters, services",
	Run:   runlistCmd,
}

func runlistCmd(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// var ecsResources []myecs.ECSResource

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(loginSetFlags.region))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	if os.Getenv("AWS_PROFILE") == "" {
		log.Fatal("set AWS_PROFILE environment variable to use")
	}

	e := myecs.NewEcs(cfg, loginSetFlags.region)
	if e == nil {
		log.Fatal("failed to initialize ECS client")
	}

	err = e.ListClusters(ctx)
	if err != nil {
		log.Fatal(err)
	}

	ecsTable, err := listECSTable(ctx, e)
	if err != nil {
		log.Fatal(err)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{
		"Cluster",
		"Service",
		"Task Definition",
		"Container"})
	// table.SetFooter([]string{"Total", "", "", strconv.Itoa(len(ecsTable))}) // Add Footer
	table.SetBorder(true)
	table.AppendBulk(ecsTable)
	table.Render()
}

func listECSTable(ctx context.Context, e *myecs.ECSResource) ([][]string, error) {
	var ecsTable [][]string

	if listSetFlags.cluster == "" {
		for _, cluster := range e.Clusters {
			if err := e.ListServices(ctx, cluster.ClusterName); err != nil {
				return nil, err
			}
			for _, service := range e.Services {
				if err := e.GetTasks(ctx, cluster.ClusterName, service.ServiceName); err != nil {
					return nil, err
				}

				if err := e.ListContainers(ctx, e.Tasks[0].TaskDefinition); err != nil {
					return nil, err
				}

				for _, container := range e.Containers {
					ecsTable = append(ecsTable, []string{cluster.ClusterName, service.ServiceName, e.Tasks[0].TaskDefinition, container.ContainerName})
				}
			}
		}
	} else {
		for _, service := range e.Services {
			if err := e.GetTasks(ctx, listSetFlags.cluster, service.ServiceName); err != nil {
				return nil, err
			}

			if err := e.ListContainers(ctx, e.Tasks[0].TaskDefinition); err != nil {
				return nil, err
			}

			for _, container := range e.Containers {
				ecsTable = append(ecsTable, []string{listSetFlags.cluster, service.ServiceName, e.Tasks[0].TaskDefinition, container.ContainerName})
			}
		}
	}
	return ecsTable, nil
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
}
