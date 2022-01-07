package cmd

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var setFlags struct {
	cluster   string
	service   string
	container string
	family    string
	command   string
}

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "execute ecs subcommand",
	Long: `a subcommand for ecs execute to login ecs container on task.
with parameters where ecs cluster, container name and command.`,
	Run: func(cmd *cobra.Command, args []string) {
		e := myecs.NewEcs(session.NewSession())

		if err := e.GetService(setFlags.cluster, setFlags.service); err != nil {
			log.Fatal(err)
		}

		fmt.Println(e.TaskDefinition)

		in := myecs.ExecuteCommandInput{}
		in.Cluster = setFlags.cluster
		in.Container = setFlags.container
		if err := e.GetTask(setFlags.cluster, e.TaskDefinition); err != nil {
			log.Fatal(err)
		}
		in.Task = *e.Task.TaskArns[0] // select first task
		in.Command = setFlags.command
		if err := e.ExecuteCommand(in); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringVarP(&setFlags.cluster, "cluster", "", "", "ECS Cluster Name")
	if err := execCmd.MarkFlagRequired("cluster"); err != nil {
		log.Fatalln(err)
	}
	execCmd.Flags().StringVarP(&setFlags.container, "container", "", "", "Container Name")
	if err := execCmd.MarkFlagRequired("container"); err != nil {
		log.Fatalln(err)
	}
	execCmd.Flags().StringVarP(&setFlags.service, "service", "", "", "ECS Service Name")
	if err := execCmd.MarkFlagRequired("service"); err != nil {
		log.Fatalln(err)
	}
	execCmd.Flags().StringVarP(&setFlags.command, "command", "", "", "Command Name")
	if err := execCmd.MarkFlagRequired("command"); err != nil {
		log.Fatalln(err)
	}
}
