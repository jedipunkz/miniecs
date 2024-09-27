package cmd

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
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
	Run:   runLoginCmd,
}

func runLoginCmd(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	var ecsResources []myecs.ECSResource

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(loginSetFlags.region))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	e := myecs.NewEcs(cfg)
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
				task := e.Tasks[0] // 最初のタスクのみを選択

				// コンテナ情報をリセット
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

	fmt.Println("ECS Resources:")
	for _, resource := range ecsResources {
		fmt.Printf("Clusters: %v\n", resource.Clusters)
		fmt.Printf("Services: %v\n", resource.Services)
		fmt.Printf("Tasks: %v\n", resource.Tasks)
		fmt.Printf("Containers: %v\n", resource.Containers)
	}
	fmt.Println("ECSResources:")
	fmt.Println(ecsResources)

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
				"Cluster: %s\nService: %s\nContainer: %s\nCommand: %s",
				ecsResources[i].Clusters[0].ClusterName,
				ecsResources[i].Services[0].ServiceName,
				ecsResources[i].Containers[0].ContainerName,
			)
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Selected indices:", idx)

	if err := login(idx, ecsResources); err != nil {
		log.Fatal(err)
	}
}

func login(idx []int, e []myecs.ECSResource) error {
	in := myecs.ExecuteCommandInput{}
	in.Cluster = e[idx[0]].Clusters[0].ClusterName
	in.Service = e[idx[0]].Services[0].ServiceName
	in.Container = e[idx[0]].Containers[0].ContainerName
	in.Task = e[idx[0]].Tasks[0].TaskArn

	if loginSetFlags.shell != "" {
		in.Command = loginSetFlags.shell
	} else {
		in.Command = defaultShell
	}

	log.WithFields(log.Fields{
		"cluster":   in.Cluster,
		"service":   in.Service,
		"task":      in.Task,
		"container": in.Container,
		"command":   in.Command,
	}).Info("ECS Execute Login with These Parameters")

	if err := e[idx[0]].ExecuteCommand(in); err != nil {
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
