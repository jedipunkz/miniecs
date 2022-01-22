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

// LoginECS is struct for login info to ECS Container
type LoginECS struct {
	Cluster        string
	Service        string
	Task           string
	TaskDefinition string
	Container      string
	Command        string
	Shell          string
}

// LoginECSs is struct for list of LoginECS
type LoginECSs []LoginECS

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "login cluster, service",
	Run: func(cmd *cobra.Command, args []string) {
		var loginECS LoginECS
		var loginECSs LoginECSs

		if err := loginECS.getShell(); err != nil {
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
			loginECS.Cluster = cluster
			for _, service := range e.Services {
				if err := e.GetTaskDefinition(cluster, service); err != nil {
					log.Fatal(err)
				}
				if err := e.GetContainerName(e.TaskDefinition); err != nil {
					log.Fatal(err)
				}
				for i := range e.Containers {
					loginECS.Cluster = cluster
					loginECS.Service = service
					loginECS.TaskDefinition = e.TaskDefinition
					loginECS.Container = e.Containers[i]
					loginECSs = append(loginECSs, loginECS)
				}
				e.Containers = nil
			}
		}

		idx, err := fuzzyfinder.FindMulti(
			loginECSs,
			func(i int) string {
				return loginECSs[i].Cluster + "::" + loginECSs[i].Service + "::" + loginECSs[i].Container
			},
			fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
				if i == -1 {
					return ""
				}
				return fmt.Sprintf("Cluster: %s\nService: %s\nContainer: %s\nCommand: %s",
					loginECSs[i].Cluster,
					loginECSs[i].Service,
					loginECSs[i].Container,
					loginECS.Shell)
			}))
		if err != nil {
			log.Fatal(err)
		}

		loginECS.Cluster = loginECSs[idx[0]].Cluster
		loginECS.Service = loginECSs[idx[0]].Service
		loginECS.Container = loginECSs[idx[0]].Container
		if err := e.GetTask(loginECSs[idx[0]].Cluster, loginECSs[idx[0]].TaskDefinition); err != nil {
			log.Fatal(err)
		}
		loginECS.Task = *e.Task.TaskArns[0]

		if err = loginECS.login(e); err != nil {
			log.Fatal(err)
		}
	},
}

func (l *LoginECS) getShell() error {
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

func (l *LoginECS) login(e *myecs.ECS) error {
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
