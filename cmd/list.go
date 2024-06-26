package cmd

import (
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var listSetFlags struct {
	region  string
	cluster string
}

// listCmd represents the listcommand
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list subcommand",
	Run: func(cmd *cobra.Command, args []string) {
		profile := os.Getenv("AWS_PROFILE")

		e := myecs.NewEcs(session.NewSessionWithOptions(session.Options{
			Config: aws.Config{
				CredentialsChainVerboseErrors: aws.Bool(true),
				Region:                        aws.String(listSetFlags.region),
			},
			SharedConfigState: session.SharedConfigEnable,
			Profile:           profile,
		}))
		if err := e.ListClusters(); err != nil {
			log.Fatal(err)
		}

		ecsMatrix := getECSMatrix(e)

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

func getECSMatrix(e *myecs.ECS) [][]string {
	var ecsMatrix [][]string

	if listSetFlags.cluster == "" {
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
	} else {
		if err := e.ListServices(listSetFlags.cluster); err != nil {
			log.Fatal(err)
		}
		for _, service := range e.Services {
			if err := e.GetTaskDefinition(listSetFlags.cluster, service); err != nil {
				log.Fatal(err)
			}
			if err := e.GetContainerName(e.TaskDefinition); err != nil {
				log.Fatal(err)
			}
			for i := range e.Containers {
				ecsMatrix = append(ecsMatrix,
					[]string{listSetFlags.cluster, service, e.TaskDefinition, e.Containers[i]})
			}
		}
	}

	return ecsMatrix
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&listSetFlags.region, "region", "", "", "Region Name")
	if err := listCmd.MarkFlagRequired("region"); err != nil {
		log.Fatal(err)
	}

	listCmd.Flags().StringVarP(&listSetFlags.cluster, "cluster", "", "", "ECS Cluster Name")
}
