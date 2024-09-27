package ecs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/jedipunkz/miniecs/internal/pkg/exec"
	log "github.com/sirupsen/logrus"
)

const (
	waitServiceStablePollingInterval = 15 * time.Second
	waitServiceStableMaxTry          = 80
)

// type api interface {
// 	ExecuteCommand(input *ecs.ExecuteCommandInput) (*ecs.ExecuteCommandOutput, error)
// 	ListTasks(input *ecs.ListTasksInput) (*ecs.ListTasksOutput, error)
// 	ListClusters(input *ecs.ListClustersInput) (*ecs.ListClustersOutput, error)
// 	ListServices(input *ecs.ListServicesInput) (*ecs.ListServicesOutput, error)
// 	DescribeServices(input *ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error)
// 	DescribeTaskDefinition(input *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error)
// }

type ECSResource struct {
	client         *ecs.Client
	newSessStarter func() ssmSessionStarter

	Clusters              []Cluster
	Services              []Service
	Tasks                 []Task
	Containers            []Container
	maxServiceStableTries int
	pollIntervalDuration  time.Duration
}

type Cluster struct {
	ClusterName string
}

type Service struct {
	ServiceName string
}

type Task struct {
	TaskArn        string
	TaskDefinition string
	Containers     []Container
}

type Container struct {
	ContainerName string
	ContainerArn  string
	Shell         string
}

type ECSResources struct {
	Resources []ECSResource
}

type ssmSessionStarter interface {
	StartSession(ssmSession *ssm.StartSessionOutput) error
}

type ExecuteCommandInput struct {
	Cluster   string
	Command   string
	Task      string
	Container string
}

func (e *ECSResource) ExecuteCommand(input ExecuteCommandInput) (err error) {
	ctx := context.TODO()

	execCmdresp, err := e.client.ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     aws.String(input.Cluster),
		Command:     aws.String(input.Command),
		Container:   aws.String(input.Container),
		Interactive: true,
		Task:        aws.String(input.Task),
	})
	if err != nil {
		return &ErrExecuteCommand{err: err}
	}

	sessID := aws.ToString(execCmdresp.Session.SessionId)
	ssmSessionOutput := &ssm.StartSessionOutput{
		SessionId: execCmdresp.Session.SessionId,
	}
	if err = e.newSessStarter().StartSession(ssmSessionOutput); err != nil {
		err = fmt.Errorf("start session %s using ssm plugin: %w", sessID, err)
	}
	return nil
}

func NewEcs(cfg aws.Config) *ECSResource {
	return &ECSResource{
		client: ecs.NewFromConfig(cfg),
		newSessStarter: func() ssmSessionStarter {
			cmd, _ := exec.NewSSMPluginCommand()
			return cmd
		},
		maxServiceStableTries: waitServiceStableMaxTry,
		pollIntervalDuration:  waitServiceStablePollingInterval,
	}
}

func (e *ECSResource) ListClusters(ctx context.Context) error {
	resultClusters, err := e.client.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return err
	}

	var c []string
	for _, cluster := range resultClusters.ClusterArns {
		clusterArr := strings.Split(cluster, "/")
		c = append(c, clusterArr[len(clusterArr)-1])
	}

	var clusters []Cluster
	for _, cluster := range c {
		clusters = append(clusters, Cluster{ClusterName: cluster})
	}
	e.Clusters = clusters
	return nil
}

func (e *ECSResource) ListServices(ctx context.Context, cluster string) error {
	inputService := &ecs.ListServicesInput{Cluster: aws.String(cluster)}
	resultServices, err := e.client.ListServices(ctx, inputService)
	if err != nil {
		log.Fatal(err)
		return err
	}

	var services []Service
	for _, service := range resultServices.ServiceArns {
		serviceArr := strings.Split(service, "/")
		services = append(services, Service{ServiceName: serviceArr[len(serviceArr)-1]})
	}
	e.Services = services
	return nil
}

func (e *ECSResource) GetTasks(ctx context.Context, cluster, service string) error {
	inputTask := &ecs.ListTasksInput{
		Cluster:     aws.String(cluster),
		ServiceName: aws.String(service),
	}
	resultTasks, err := e.client.ListTasks(ctx, inputTask)
	if err != nil {
		log.Fatal(err)
		return err
	}

	var tasks []Task
	for _, taskArn := range resultTasks.TaskArns {
		describeTasksInput := &ecs.DescribeTasksInput{
			Tasks:   []string{taskArn},
			Cluster: aws.String(cluster),
		}
		describeTasksOutput, err := e.client.DescribeTasks(ctx, describeTasksInput)
		if err != nil {
			log.Fatal(err)
			return err
		}

		if len(describeTasksOutput.Tasks) == 0 {
			log.Printf("Could not found task definition with task arn: %s", taskArn)
			continue
		}

		taskDefinitionArn := describeTasksOutput.Tasks[0].TaskDefinitionArn

		describeTaskDefinitionInput := &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: taskDefinitionArn,
		}
		describeTaskDefinitionOutput, err := e.client.DescribeTaskDefinition(ctx, describeTaskDefinitionInput)
		if err != nil {
			log.Fatal(err)
			return err
		}

		tasks = append(tasks, Task{
			TaskArn:        taskArn,
			TaskDefinition: *describeTaskDefinitionOutput.TaskDefinition.Family,
		})
	}
	e.Tasks = tasks
	return nil
}

func (e *ECSResource) ListContainers(ctx context.Context, taskDefinition string) error {
	inputTaskDefinition := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinition),
	}

	result, err := e.client.DescribeTaskDefinition(context.Background(), inputTaskDefinition)
	if err != nil {
		log.Fatal(err)
		return err
	}

	for _, container := range result.TaskDefinition.ContainerDefinitions {
		e.Containers = append(e.Containers, Container{ContainerName: *container.Name})
	}
	return nil
}
