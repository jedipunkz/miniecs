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

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(listSetFlags.region))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	ecsClient := myecs.NewClient(cfg, listSetFlags.region)
	if ecsClient == nil {
		log.Fatal("failed to initialize ECS client")
	}

	rows, err := listECSTable(ctx, ecsClient)
	if err != nil {
		log.Fatal(err)
	}

	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeader([]string{
			"Cluster",
			"Service",
			"Task Definition",
			"Container"}))
	for _, row := range rows {
		if err := table.Append(row); err != nil {
			log.Fatal(err)
		}
	}
	if err := table.Render(); err != nil {
		log.Fatal(err)
	}
}

// listECSTable fetches the flat targets of the selected clusters and
// renders the table rows for them.
func listECSTable(ctx context.Context, ecsClient ecsTargetLister) ([][]string, error) {
	targets, err := fetchTargets(ctx, ecsClient, listSetFlags.cluster)
	if err != nil {
		return nil, err
	}

	return buildECSRows(targets), nil
}

// buildECSRows renders the table rows for the flat targets. One target
// describes exactly one container, so the row granularity is one row per
// container target.
func buildECSRows(targets []myecs.Target) [][]string {
	var rows [][]string
	for _, target := range targets {
		rows = append(rows, []string{
			target.Cluster,
			target.Service,
			shortenTaskDefArn(target.TaskDefinition),
			target.Container,
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
