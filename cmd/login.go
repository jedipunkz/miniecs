package cmd

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/ktr0731/go-fuzzyfinder"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	confFile     = "miniecs"
	defaultShell = "sh"
)

var loginSetFlags struct {
	region  string
	cluster string
	shell   string
}

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "login cluster, service",
	Run: func(cmd *cobra.Command, args []string) {
		var execECS ExecECS
		var execECSs ExecECSs

		e := myecs.NewEcs(session.NewSessionWithOptions(session.Options{
			Config: aws.Config{
				CredentialsChainVerboseErrors: aws.Bool(true),
				Region:                        aws.String(loginSetFlags.region),
			},
		}))

		// without specifying cluster, will search all clusters
		if loginSetFlags.cluster == "" {
			if err := e.ListClusters(); err != nil {
				log.Fatal(err)
			}
			for _, cluster := range e.Clusters {
				if err := e.ListServices(cluster); err != nil {
					log.Fatal(err)
				}
				execECS.Cluster = cluster
				for _, service := range e.Services {
					if err := e.GetTaskDefinition(cluster, service); err != nil {
						log.Fatal(err)
					}
					if err := e.GetContainerName(e.TaskDefinition); err != nil {
						log.Fatal(err)
					}
					for i := range e.Containers {
						execECS.Cluster = cluster
						execECS.Service = service
						execECS.TaskDefinition = e.TaskDefinition
						execECS.Container = e.Containers[i]
						execECSs = append(execECSs, execECS)
					}
					e.Containers = nil
				}
			}
		} else { // with specifying cluster option
			if err := e.ListServices(loginSetFlags.cluster); err != nil {
				log.Fatal(err)
			}
			execECS.Cluster = loginSetFlags.cluster
			for _, service := range e.Services {
				if err := e.GetTaskDefinition(loginSetFlags.cluster, service); err != nil {
					log.Fatal(err)
				}
				if err := e.GetContainerName(e.TaskDefinition); err != nil {
					log.Fatal(err)
				}
				for i := range e.Containers {
					execECS.Cluster = loginSetFlags.cluster
					execECS.Service = service
					execECS.TaskDefinition = e.TaskDefinition
					execECS.Container = e.Containers[i]
					execECSs = append(execECSs, execECS)
				}
				e.Containers = nil
			}
		}

		idx, err := fuzzyfinder.FindMulti(
			execECSs,
			func(i int) string {
				return execECSs[i].Cluster + "::" +
					execECSs[i].Service + "::" +
					execECSs[i].Container
			},
			fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
				if i == -1 {
					return ""
				}
				return fmt.Sprintf(
					"Cluster: %s\nService: %s\nContainer: %s\nCommand: %s",
					execECSs[i].Cluster,
					execECSs[i].Service,
					execECSs[i].Container,
					execECS.Shell)
			}))
		if err != nil {
			log.Fatal(err)
		}

		execECS.Cluster = execECSs[idx[0]].Cluster
		execECS.Service = execECSs[idx[0]].Service
		execECS.Container = execECSs[idx[0]].Container
		if err := e.GetTask(
			execECSs[idx[0]].Cluster,
			execECSs[idx[0]].TaskDefinition); err != nil {
			log.Fatal(err)
		}
		execECS.Task = *e.Task.TaskArns[0]

		if err = execECS.login(e); err != nil {
			log.Fatal(err)
		}
	},
}

func (l *ExecECS) login(e *myecs.ECS) error {
	in := myecs.ExecuteCommandInput{}
	in.Cluster = l.Cluster
	in.Container = l.Container
	in.Task = l.Task
	if loginSetFlags.shell != "" {
		in.Command = loginSetFlags.shell
	} else {
		in.Command = defaultShell
	}

	log.WithFields(log.Fields{
		"cluster":   l.Cluster,
		"service":   l.Service,
		"task":      l.Task,
		"container": l.Container,
		"command":   l.Shell,
	}).Info("ECS Execute Login with These Parameters")

	if err := e.ExecuteCommand(in); err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.Flags().StringVarP(
		&loginSetFlags.region, "region", "", "", "Region Name")
	if err := loginCmd.MarkFlagRequired("region"); err != nil {
		log.Fatal(err)
	}
	loginCmd.Flags().StringVarP(
		&loginSetFlags.cluster, "cluster", "", "", "ECS Cluster Name")
	loginCmd.Flags().StringVarP(
		&loginSetFlags.shell, "shell", "", "", "Login Shell")
}
