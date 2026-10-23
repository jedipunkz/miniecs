package cmd

import (
	"context"
	"io"
	"os"
	"strings"

	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list clusters, services",
	RunE:  runListCmd,
}

// runListCmd fetches the targets and prints them as a table. With the
// --cluster flag set only that cluster is fetched.
func runListCmd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	region, err := cmd.Flags().GetString("region")
	if err != nil {
		return err
	}
	cluster, err := cmd.Flags().GetString("cluster")
	if err != nil {
		return err
	}

	ecsClient, err := initializeECSClient(ctx, region)
	if err != nil {
		return err
	}

	rows, err := listECSTable(ctx, ecsClient, cluster)
	if err != nil {
		return err
	}

	return writeECSRows(os.Stdout, rows)
}

// listECSTable fetches the flat targets of the selected clusters and
// returns the table rows for them.
func listECSTable(ctx context.Context, ecsClient ecsTargetLister, cluster string) ([][]string, error) {
	targets, err := fetchTargets(ctx, ecsClient, cluster)
	if err != nil {
		return nil, err
	}

	return buildECSRows(targets), nil
}

// writeECSRows renders the table rows to the writer. One row describes
// exactly one container target.
func writeECSRows(w io.Writer, rows [][]string) error {
	table := tablewriter.NewTable(w,
		tablewriter.WithHeader([]string{
			"Cluster",
			"Service",
			"Task Definition",
			"Container"}))
	for _, row := range rows {
		if err := table.Append(row); err != nil {
			return err
		}
	}
	return table.Render()
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
	listCmd.Flags().String(
		"cluster", "", "ECS Cluster Name")
}
