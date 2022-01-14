package cmd

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	confFile = "miniecs"
)

// Config is struct
type Config struct {
	ECSs []ECS
}

// ECS is struct
type ECS struct {
	Cluster   string
	Service   string
	Container string
	Command   string
}

// ECSs is struct for list of ECS
type ECSs []ECS

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "login cluster, service",
	Run: func(cmd *cobra.Command, args []string) {
		var ecs ECS
		var ecss ECSs

		shell := viper.GetString("shell")

		e := myecs.NewEcs(session.NewSession())
		if err := e.ListClusters(); err != nil {
			log.Fatal(err)
		}
		for _, cluster := range e.Clusters {
			if err := e.ListServices(cluster); err != nil {
				log.Fatal(err)
			}
			for _, service := range e.Services {
				ecs.Cluster = cluster
				ecs.Service = service

				if err := e.GetService(cluster, service); err != nil {
					log.Fatal(err)
				}
				if err := e.GetContainerName(e.TaskDefinition); err != nil {
					log.Fatal(err)
				}
				for _, container := range e.Containers {
					ecs.Container = container

					ecss = append(ecss, ecs)
				}
			}
		}

		idx, err := fuzzyfinder.FindMulti(
			ecss,
			func(i int) string {
				return ecss[i].Cluster + "::" + ecss[i].Service + "::" + ecss[i].Container
			},
			fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
				if i == -1 {
					return ""
				}
				return fmt.Sprintf("Cluster: %s\nService: %s\nContainer: %s\nCommand: %s",
					ecss[i].Cluster,
					ecss[i].Service,
					ecss[i].Container,
					shell)
			}))
		if err != nil {
			log.Fatal(err)
		}

		if err := e.GetService(ecss[idx[0]].Cluster, ecss[idx[0]].Service); err != nil {
			log.Fatal(err)
		}

		in := myecs.ExecuteCommandInput{}
		in.Cluster = ecss[idx[0]].Cluster
		in.Container = ecss[idx[0]].Container
		if err := e.GetTask(ecss[idx[0]].Cluster, e.TaskDefinition); err != nil {
			log.Fatal(err)
		}
		in.Task = *e.Task.TaskArns[0] // login first task
		in.Command = shell

		log.WithFields(log.Fields{
			"cluster":   in.Cluster,
			"service":   ecss[idx[0]].Service,
			"task":      in.Task,
			"container": in.Container,
			"command":   in.Command,
		}).Info("ECS Execute Login with These Parameters")

		if err := e.ExecuteCommand(in); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)

	home, err := homedir.Dir()
	if err != nil {
		log.Fatal(err)
	}

	viper.SetConfigType("yaml")
	viper.AddConfigPath(home)
	viper.SetConfigName(confFile)

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal(err)
	}
}
