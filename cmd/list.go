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
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list clusters, services",
	Run:   runlistCmd,
}

func runlistCmd(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(listSetFlags.region))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	e := myecs.NewEcs(cfg, listSetFlags.region)
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
	table.SetBorder(true)
	table.AppendBulk(ecsTable)
	table.Render()
}

func listECSTable(ctx context.Context, e *myecs.ECSResource) ([][]string, error) {
	var ecsTable [][]string

	if listSetFlags.cluster == "" {
		for _, cluster := range e.Clusters {
			resources, err := e.CollectServicesAndContainers(ctx, cluster)
			if err != nil {
				return nil, err
			}

			for _, resource := range resources {
				ecsTable = append(ecsTable, []string{
					resource.Clusters[0].ClusterName,
					resource.Services[0].ServiceName,
					resource.Tasks[0].TaskDefinition,
					resource.Containers[0].ContainerName,
				})
			}
		}
	} else {
		cluster := myecs.Cluster{ClusterName: listSetFlags.cluster}
		resources, err := e.CollectServicesAndContainers(ctx, cluster)
		if err != nil {
			return nil, err
		}

		for _, resource := range resources {
			ecsTable = append(ecsTable, []string{
				resource.Clusters[0].ClusterName,
				resource.Services[0].ServiceName,
				resource.Tasks[0].TaskDefinition,
				resource.Containers[0].ContainerName,
			})
		}
	}

	return ecsTable, nil
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(
		&listSetFlags.region, "region", "", "", "Region Name")
	if err := listCmd.MarkFlagRequired("region"); err != nil {
		log.Fatal(err)
	}
	listCmd.Flags().StringVarP(
		&listSetFlags.cluster, "cluster", "", "", "ECS Cluster Name")
}
