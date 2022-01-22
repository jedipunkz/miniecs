package cmd

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	confFile     = "miniecs"
	defaultShell = "sh"
)

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "login cluster, service",
	Run: func(cmd *cobra.Command, args []string) {
		var execECS ExecECS
		var execECSs ExecECSs

		if err := execECS.getShell(); err != nil {
			log.Fatal(err)
		}

		e := myecs.NewEcs(session.NewSession())
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

		idx, err := fuzzyfinder.FindMulti(
			execECSs,
			func(i int) string {
				return execECSs[i].Cluster + "::" + execECSs[i].Service + "::" + execECSs[i].Container
			},
			fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
				if i == -1 {
					return ""
				}
				return fmt.Sprintf("Cluster: %s\nService: %s\nContainer: %s\nCommand: %s",
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
		if err := e.GetTask(execECSs[idx[0]].Cluster, execECSs[idx[0]].TaskDefinition); err != nil {
			log.Fatal(err)
		}
		execECS.Task = *e.Task.TaskArns[0]

		if err = execECS.login(e); err != nil {
			log.Fatal(err)
		}
	},
}

func (l *ExecECS) getShell() error {
	l.Shell = defaultShell

	home, err := homedir.Dir()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stat(home + "/" + confFile + ".yaml"); err == nil {
		viper.SetConfigType("yaml")
		viper.AddConfigPath(home)
		viper.SetConfigName(confFile)

		if err := viper.ReadInConfig(); err != nil {
			log.Fatal(err)
			return err
		}
		l.Shell = viper.GetString("shell")
	}
	return nil
}

func (l *ExecECS) login(e *myecs.ECS) error {
	in := myecs.ExecuteCommandInput{}
	in.Cluster = l.Cluster
	in.Container = l.Container
	in.Task = l.Task
	in.Command = l.Shell
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
}
