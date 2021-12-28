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
		clusters, err := e.GetClusters()
		if err != nil {
			log.Fatal(err)
		}

		var ecsMatrix [][]string

		for _, cluster := range clusters {
			services, err := e.GetServices(cluster)
			if err != nil {
				log.Fatal(err)
			}
			for _, service := range services {
				ecsMatrix = append(ecsMatrix, []string{cluster, service})
			}
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Cluster Name", "Service Name"})
		table.SetFooter([]string{"Total", strconv.Itoa(len(ecsMatrix))}) // Add Footer
		table.SetBorder(true)
		table.AppendBulk(ecsMatrix)
		table.Render()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}