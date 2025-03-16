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

func (e *ECSResource) ExecuteCommand(input ecs.ExecuteCommandInput) error {
	if e.client == nil {
		return fmt.Errorf("ECS client is not initialized")
	}

	ctx := context.Background()
	input.Interactive = true

	execCommandOutput, err := e.client.ExecuteCommand(ctx, &input)
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	sessionInfo, err := json.Marshal(execCommandOutput.Session)
	if err != nil {
		return fmt.Errorf("failed to marshal session info: %w", err)
	}

	target := fmt.Sprintf("ecs:%s_%s_%s", *input.Cluster, *input.Task, *input.Container)
	ssmTarget := struct {
		Target string `json:"Target"`
	}{
		Target: target,
	}

	targetJSON, err := json.Marshal(ssmTarget)
	if err != nil {
		return fmt.Errorf("failed to marshal target info: %w", err)
	}

	ssmEndpoint := fmt.Sprintf("https://ssm.%s.amazonaws.com", e.Region)
	cmd := e.createSessionManagerCommand(sessionInfo, targetJSON, ssmEndpoint)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run session-manager-plugin: %w", err)
	}

	return nil
}

func (e *ECSResource) createSessionManagerCommand(sessionInfo, targetJSON []byte, ssmEndpoint string) *exec.Cmd {
	cmd := exec.Command(
		"session-manager-plugin",
		string(sessionInfo),
		e.Region,
		"StartSession",
		"",
		string(targetJSON),
		ssmEndpoint,
	)

	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd
}

func extractResourceName(arn string) string {
	segments := strings.Split(arn, "/")
	return segments[len(segments)-1]
}

func (e *ECSResource) ListClusters(ctx context.Context) error {
	resultClusters, err := e.client.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	e.Clusters = make([]Cluster, 0, len(resultClusters.ClusterArns))
	for _, clusterArn := range resultClusters.ClusterArns {
		e.Clusters = append(e.Clusters, Cluster{
			ClusterName: extractResourceName(clusterArn),
		})
	}

	return nil
}

func (e *ECSResource) ListServices(ctx context.Context, cluster string) error {
	input := &ecs.ListServicesInput{Cluster: aws.String(cluster)}
	resultServices, err := e.client.ListServices(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to list services for cluster %s: %w", cluster, err)
	}

	e.Services = make([]Service, 0, len(resultServices.ServiceArns))
	for _, serviceArn := range resultServices.ServiceArns {
		e.Services = append(e.Services, Service{
			ServiceName: extractResourceName(serviceArn),
		})
	}

	return nil
}

func (e *ECSResource) GetTasks(ctx context.Context, cluster, service string) error {
	tasks, err := e.listTasks(ctx, cluster, service)
	if err != nil {
		return err
	}

	e.Tasks = make([]Task, 0, len(tasks))
	for _, taskArn := range tasks {
		task, err := e.describeTask(ctx, cluster, taskArn)
		if err != nil {
			log.Warnf("Failed to describe task %s: %v", taskArn, err)
			continue
		}
		e.Tasks = append(e.Tasks, task)
	}

	return nil
}

func (e *ECSResource) listTasks(ctx context.Context, cluster, service string) ([]string, error) {
	input := &ecs.ListTasksInput{
		Cluster:     aws.String(cluster),
		ServiceName: aws.String(service),
	}
	result, err := e.client.ListTasks(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks for service %s: %w", service, err)
	}
	return result.TaskArns, nil
}

func (e *ECSResource) describeTask(ctx context.Context, cluster, taskArn string) (Task, error) {
	input := &ecs.DescribeTasksInput{
		Tasks:   []string{taskArn},
		Cluster: aws.String(cluster),
	}
	result, err := e.client.DescribeTasks(ctx, input)
	if err != nil {
		return Task{}, fmt.Errorf("failed to describe task %s: %w", taskArn, err)
	}

	if len(result.Tasks) == 0 {
		return Task{}, fmt.Errorf("no task found with ARN %s", taskArn)
	}

	taskDef, err := e.describeTaskDefinition(ctx, result.Tasks[0].TaskDefinitionArn)
	if err != nil {
		return Task{}, err
	}

	return Task{
		TaskArn:        taskArn,
		TaskDefinition: *taskDef.TaskDefinition.Family,
	}, nil
}

func (e *ECSResource) describeTaskDefinition(ctx context.Context, taskDefinitionArn *string) (*ecs.DescribeTaskDefinitionOutput, error) {
	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: taskDefinitionArn,
	}
	result, err := e.client.DescribeTaskDefinition(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe task definition %s: %w", *taskDefinitionArn, err)
	}
	return result, nil
}

func (e *ECSResource) ListContainers(ctx context.Context, taskDefinition string) error {
	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinition),
	}

	result, err := e.client.DescribeTaskDefinition(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to describe task definition %s: %w", taskDefinition, err)
	}

	e.Containers = make([]Container, 0, len(result.TaskDefinition.ContainerDefinitions))
	for _, container := range result.TaskDefinition.ContainerDefinitions {
		e.Containers = append(e.Containers, Container{
			ContainerName: *container.Name,
		})
	}

	return nil
}
