package cmd

import (
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws/session"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// listCmd represents the listcommand
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list subcommand",
	Run: func(cmd *cobra.Command, args []string) {
		e := myecs.NewEcs(session.NewSession())
		if err := e.ListClusters(); err != nil {
			log.Fatal(err)
		}

		var ecsMatrix [][]string

		for _, cluster := range e.Clusters {
			if err := e.ListServices(cluster); err != nil {
				log.Fatal(err)
			}
			for _, service := range e.Services {
				if err := e.GetTaskDefinition(cluster, service); err != nil {
					log.Fatal(err)
				}
				if err := e.GetContainerName(e.TaskDefinition); err != nil {
					log.Fatal(err)
				}
				for i := range e.Containers {
					ecsMatrix = append(ecsMatrix,
						[]string{cluster, service, e.TaskDefinition, e.Containers[i]})
				}
			}
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{
			"Cluster Name",
			"Service Name",
			"TaskDefinition Name",
			"Container Name"})
		table.SetFooter([]string{"Total", "", "", strconv.Itoa(len(ecsMatrix))}) // Add Footer
		table.SetBorder(true)
		table.AppendBulk(ecsMatrix)
		table.Render()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
