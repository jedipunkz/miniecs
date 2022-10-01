package cmd

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var setFlags struct {
	region    string
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
		var execECS ExecECS

		e := myecs.NewEcs(session.NewSessionWithOptions(session.Options{
			Config: aws.Config{
				CredentialsChainVerboseErrors: aws.Bool(true),
				Region:                        aws.String(setFlags.region),
			},
		}))

		if err := e.GetTaskDefinition(setFlags.cluster, setFlags.service); err != nil {
			log.Fatal(err)
		}

		execECS.Cluster = setFlags.cluster
		execECS.Service = setFlags.service
		execECS.Container = setFlags.container
		execECS.Command = setFlags.command

		if err := e.GetTask(execECS.Cluster, e.TaskDefinition); err != nil {
			log.Fatal(err)
		}

		execECS.Task = *e.Task.TaskArns[0]

		if err := execECS.exec(e); err != nil {
			log.Fatal(err)
		}
	},
}

func (l *ExecECS) exec(e *myecs.ECS) error {
	in := myecs.ExecuteCommandInput{}
	in.Cluster = l.Cluster
	in.Container = l.Container

	if err := e.GetTask(l.Cluster, e.TaskDefinition); err != nil {
		log.Fatal(err)
	}

	in.Task = *e.Task.TaskArns[0] // select first task
	in.Command = l.Command

	log.WithFields(log.Fields{
		"cluster":   l.Cluster,
		"service":   l.Service,
		"task":      l.Task,
		"container": l.Container,
		"command":   l.Command,
	}).Info("ECS Execute Login with These Parameters")

	if err := e.ExecuteCommand(in); err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringVarP(&setFlags.region, "region", "", "", "Region Name")
	if err := execCmd.MarkFlagRequired("region"); err != nil {
		log.Fatal(err)
	}
	execCmd.Flags().StringVarP(&setFlags.cluster, "cluster", "", "", "ECS Cluster Name")
	if err := execCmd.MarkFlagRequired("cluster"); err != nil {
		log.Fatal(err)
	}
	execCmd.Flags().StringVarP(&setFlags.container, "container", "", "", "Container Name")
	if err := execCmd.MarkFlagRequired("container"); err != nil {
		log.Fatal(err)
	}
	execCmd.Flags().StringVarP(&setFlags.service, "service", "", "", "ECS Service Name")
	if err := execCmd.MarkFlagRequired("service"); err != nil {
		log.Fatal(err)
	}
	execCmd.Flags().StringVarP(&setFlags.command, "command", "", "", "Command Name")
	if err := execCmd.MarkFlagRequired("command"); err != nil {
		log.Fatal(err)
	}
}
