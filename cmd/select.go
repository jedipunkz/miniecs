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
	Name      string
	Cluster   string
	Service   string
	Container string
	Command   string
}

// selectCmd represents the select command
var selectCmd = &cobra.Command{
	Use:   "select",
	Short: "select cluster, service",
	Run: func(cmd *cobra.Command, args []string) {
		e := myecs.NewEcs(session.NewSession())

		var cfg Config
		var ecss = []ECS{}

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
		if err := viper.Unmarshal(&cfg); err != nil {
			log.Fatal(err)
		}
		ecss = append(ecss, cfg.ECSs...)

		idx, err := fuzzyfinder.FindMulti(
			ecss,
			func(i int) string {
				return ecss[i].Name
			},
			fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
				if i == -1 {
					return ""
				}
				return fmt.Sprintf("Name: %s \nCluster: %s\nService: %s\nContainer: %s\nCommand: %s",
					ecss[i].Name,
					ecss[i].Cluster,
					ecss[i].Service,
					ecss[i].Container,
					ecss[i].Command)
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
		in.Task = *e.Task.TaskArns[0] // select first task
		in.Command = ecss[idx[0]].Command

		log.WithFields(log.Fields{
			"cluster":   in.Cluster,
			"service":   ecss[idx[0]].Service,
			"task":      in.Task,
			"container": in.Container,
			"command":   ecss[idx[0]].Command,
		}).Info("ECS Execute Login with These Parameters")

		if err := e.ExecuteCommand(in); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(selectCmd)
}
