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

	e := myecs.NewECS(cfg, listSetFlags.region)
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

	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeader([]string{
			"Cluster",
			"Service",
			"Task Definition",
			"Container"}))
	for _, row := range ecsTable {
		if err := table.Append(row); err != nil {
			log.Fatal(err)
		}
	}
	if err := table.Render(); err != nil {
		log.Fatal(err)
	}
}

func listECSTable(ctx context.Context, e *myecs.ECSResource) ([][]string, error) {
	var ecsTable [][]string

	if listSetFlags.cluster == "" {
		for _, cluster := range e.Clusters {
			resources, err := e.GetClusterResources(ctx, cluster)
			if err != nil {
				return nil, err
			}

			for _, resource := range resources {
				if len(resource.Clusters) > 0 {
					clusterName := resource.Clusters[0].ClusterName
					ecsTable = append(ecsTable, []string{
						clusterName,
						"",
						"",
						"",
					})
				}
			}
		}
	} else {
		cluster := myecs.ECSCluster{ClusterName: listSetFlags.cluster}
		resources, err := e.GetClusterResources(ctx, cluster)
		if err != nil {
			return nil, err
		}

		for _, resource := range resources {
			if len(resource.Clusters) > 0 {
				clusterName := resource.Clusters[0].ClusterName
				ecsTable = append(ecsTable, []string{
					clusterName,
					"",
					"",
					"",
				})
			}
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
