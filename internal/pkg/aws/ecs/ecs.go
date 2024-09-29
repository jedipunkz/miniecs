package ecs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	log "github.com/sirupsen/logrus"
)

type ECSResource struct {
	client *ecs.Client

	Clusters   []Cluster
	Services   []Service
	Tasks      []Task
	Containers []Container

	Region string
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

func (e *ECSResource) ExecuteCommand(input ecs.ExecuteCommandInput) error {
	ctx := context.TODO()

	input = ecs.ExecuteCommandInput{
		Cluster:     aws.String(*input.Cluster),
		Command:     aws.String(*input.Command),
		Container:   aws.String(*input.Container),
		Interactive: true,
		Task:        aws.String(*input.Task),
	}

	if e.client == nil {
		log.Fatal("e.client is nil")
	}
	execCommandOutput, err := e.client.ExecuteCommand(ctx, &input)
	if err != nil {
		log.Fatal(err)
	}

	ssmSession := execCommandOutput.Session
	sessionInfo, err := json.Marshal(ssmSession)
	if err != nil {
		log.Fatal(err)
	}

	target := fmt.Sprintf("ecs:%s_%s_%s", *input.Cluster, *input.Task, *input.Container)

	ssmTarget := struct {
		Target string `json:"Target"`
	}{
		Target: target,
	}
	targetJSON, err := json.Marshal(ssmTarget)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling target information: %v\n", err)
	}

	ssmEndpoint := "https://ssm." + e.Region + ".amazonaws.com"

	cmd := exec.Command(
		"session-manager-plugin",
		string(sessionInfo),
		e.Region,
		"StartSession",
		"", // os.Getenv("AWS_PROFILE"),
		string(targetJSON),
		ssmEndpoint,
	)

	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err = cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running session-manager-plugin: %v\n", err)
	}

	return nil
}

func NewEcs(cfg aws.Config, region string) *ECSResource {
	return &ECSResource{
		client:     ecs.NewFromConfig(cfg),
		Clusters:   []Cluster{},
		Services:   []Service{},
		Tasks:      []Task{},
		Containers: []Container{},
		Region:     region,
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
