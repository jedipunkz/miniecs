package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/ktr0731/go-fuzzyfinder"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var loginSetFlags struct {
	region  string
	cluster string
	shell   string
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "login cluster, service",
	Run:   runLoginCmd,
}

func runLoginCmd(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	var ecsResources []myecs.ECSResource

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(loginSetFlags.region))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	if os.Getenv("AWS_PROFILE") == "" {
		log.Fatal("set AWS_PROFILE environment variable to use")
	}

	e := myecs.NewEcs(cfg, loginSetFlags.region)
	if e == nil {
		log.Fatal("failed to initialize ECS client")
	}

	err = e.ListClusters(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, cluster := range e.Clusters {
		err = e.ListServices(ctx, cluster.ClusterName)
		if err != nil {
			log.Fatal(err)
		}

		for _, service := range e.Services {
			err := e.GetTasks(ctx, cluster.ClusterName, service.ServiceName)
			if err != nil {
				log.Fatal(err)
			}

			if len(e.Tasks) > 0 {
				task := e.Tasks[0] // set first task

				e.Containers = nil

				err := e.ListContainers(ctx, task.TaskDefinition)
				if err != nil {
					log.Fatal(err)
				}

				ecsResource := myecs.ECSResource{
					Clusters:   []myecs.Cluster{cluster},
					Services:   []myecs.Service{service},
					Tasks:      []myecs.Task{task},
					Containers: e.Containers,
				}
				ecsResources = append(ecsResources, ecsResource)
			}
		}
	}

	idx, err := fuzzyfinder.FindMulti(
		ecsResources,
		func(i int) string {
			return ecsResources[i].Clusters[0].ClusterName + " " +
				ecsResources[i].Services[0].ServiceName + " " +
				ecsResources[i].Containers[0].ContainerName
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			return fmt.Sprintf(
				"Cluster: %s\nService: %s\nContainer: %s\n",
				ecsResources[i].Clusters[0].ClusterName,
				ecsResources[i].Services[0].ServiceName,
				ecsResources[i].Containers[0].ContainerName,
			)
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := login(e, idx, ecsResources); err != nil {
		log.Fatal(err)
	}
}

func login(e *myecs.ECSResource, idx []int, myecs []myecs.ECSResource) error {
	in := ecs.ExecuteCommandInput{}
	in.Cluster = &myecs[idx[0]].Clusters[0].ClusterName
	in.Container = &myecs[idx[0]].Containers[0].ContainerName
	in.Task = &myecs[idx[0]].Tasks[0].TaskArn

	if loginSetFlags.shell != "" {
		in.Command = &loginSetFlags.shell
	} else {
		defaultShell := "sh"
		in.Command = &defaultShell
	}

	log.WithFields(log.Fields{
		"cluster":   *in.Cluster,
		"task":      *in.Task,
		"container": *in.Container,
		"command":   *in.Command,
	}).Info("ECS Execute Login with These Parameters")

	if err := e.ExecuteCommand(in); err != nil {
		log.Fatalf("failed to execute command: %v", err)
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
