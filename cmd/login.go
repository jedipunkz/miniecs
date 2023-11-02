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
		var ecsInfo ECSInfo
		var ecsInfos ECSInfos

		e := myecs.NewEcs(session.NewSessionWithOptions(session.Options{
			Config: aws.Config{
				CredentialsChainVerboseErrors: aws.Bool(true),
				Region:                        aws.String(loginSetFlags.region),
			},
		}))

		ecsInfos = ecsInfo.fetchListECSs(e)

		idx, err := fuzzyfinder.FindMulti(
			ecsInfos,
			func(i int) string {
				return ecsInfos[i].Cluster + " " +
					ecsInfos[i].Service + " " +
					ecsInfos[i].Container
			},
			fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
				if i == -1 {
					return ""
				}
				return fmt.Sprintf(
					"Cluster: %s\nService: %s\nContainer: %s\nCommand: %s",
					ecsInfos[i].Cluster,
					ecsInfos[i].Service,
					ecsInfos[i].Container,
					ecsInfo.Shell)
			}))
		if err != nil {
			log.Fatal(err)
		}

		ecsInfo.Cluster = ecsInfos[idx[0]].Cluster
		ecsInfo.Service = ecsInfos[idx[0]].Service
		ecsInfo.Container = ecsInfos[idx[0]].Container

		if err := e.GetTask(
			ecsInfos[idx[0]].Cluster,
			ecsInfos[idx[0]].TaskDefinition); err != nil {
			log.Fatal(err)
		}

		ecsInfo.Task = *e.Task.TaskArns[0]

		if err = ecsInfo.login(e); err != nil {
			log.Fatal(err)
		}
	},
}

func (l *ECSInfo) fetchListECSs(e *myecs.ECS) ECSInfos {
	var ecsInfos ECSInfos

	clusters := []string{loginSetFlags.cluster}
	if loginSetFlags.cluster == "" {
		if err := e.ListClusters(); err != nil {
			log.Fatal(err)
		}
		clusters = e.Clusters
	}

	for _, cluster := range clusters {
		if err := e.ListServices(cluster); err != nil {
			log.Fatal(err)
		}
		l.Cluster = cluster
		for _, service := range e.Services {
			if err := e.GetTaskDefinition(l.Cluster, service); err != nil {
				log.Fatal(err)
			}

			if err := e.GetContainerName(e.TaskDefinition); err != nil {
				log.Fatal(err)
			}

			for i := range e.Containers {
				l.Service = service
				l.TaskDefinition = e.TaskDefinition
				l.Container = e.Containers[i]
				ecsInfos = append(ecsInfos, *l)
			}
			e.Containers = nil
		}
	}
	return ecsInfos
}

func (l *ECSInfo) login(e *myecs.ECS) error {
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
