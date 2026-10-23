package cmd

import (
	"context"
	"os"
	"strings"

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
	var resources []myecs.ECSResource

	if listSetFlags.cluster == "" {
		for _, cluster := range e.Clusters {
			loaded, err := e.GetClusterResources(ctx, cluster)
			if err != nil {
				return nil, err
			}
			resources = append(resources, loaded...)
		}
	} else {
		cluster := myecs.ECSCluster{ClusterName: listSetFlags.cluster}
		loaded, err := e.GetClusterResources(ctx, cluster)
		if err != nil {
			return nil, err
		}
		resources = loaded
	}

	return buildECSRows(resources), nil
}

// buildECSRows renders the table rows for the resources loaded by
// GetClusterResources. Each loaded resource describes exactly one task, so
// the row granularity is one row per task: the container column lists all
// container names of that task, joined with ", ". Resources that are
// missing cluster, service or task data are skipped.
func buildECSRows(resources []myecs.ECSResource) [][]string {
	var rows [][]string
	for _, resource := range resources {
		if len(resource.Clusters) == 0 ||
			len(resource.Clusters[0].Services) == 0 ||
			len(resource.Clusters[0].Services[0].Tasks) == 0 {
			continue
		}
		cluster := resource.Clusters[0]
		service := cluster.Services[0]
		task := service.Tasks[0]

		containers := make([]string, 0, len(task.Containers))
		for _, container := range task.Containers {
			containers = append(containers, container.ContainerName)
		}

		rows = append(rows, []string{
			cluster.ClusterName,
			service.ServiceName,
			shortenTaskDefArn(task.TaskDefinition),
			strings.Join(containers, ", "),
		})
	}
	return rows
}

const taskDefinitionARNMarker = ":task-definition/"

// shortenTaskDefArn renders a task definition as family:revision instead of
// the full ARN. Values without the task-definition ARN marker (for example
// an already abbreviated "family:revision") are returned unchanged.
func shortenTaskDefArn(taskDefinition string) string {
	index := strings.Index(taskDefinition, taskDefinitionARNMarker)
	if index < 0 {
		return taskDefinition
	}
	familyRevision := taskDefinition[index+len(taskDefinitionARNMarker):]
	if familyRevision == "" {
		return taskDefinition
	}
	return familyRevision
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
