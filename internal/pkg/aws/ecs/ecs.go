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
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	log "github.com/sirupsen/logrus"
)

type ECSExecRunner interface {
	RunCommand(cmd *exec.Cmd) error
}

type DefaultECSExecRunner struct{}

func (r *DefaultECSExecRunner) RunCommand(cmd *exec.Cmd) error {
	return cmd.Run()
}

type ECSClient interface {
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
	DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
	ExecuteCommand(ctx context.Context, params *ecs.ExecuteCommandInput, optFns ...func(*ecs.Options)) (*ecs.ExecuteCommandOutput, error)
}

type ECSResource struct {
	client     ECSClient
	execRunner ECSExecRunner

	Clusters []ECSCluster

	Region string
}

type ECSCluster struct {
	ClusterName string
	ClusterArn  string
	Services    []ECSService
}

type ECSService struct {
	ServiceName string
	ServiceArn  string
	ClusterName string
	Tasks       []ECSTask
}

type ECSTask struct {
	TaskArn        string
	TaskDefinition string
	ServiceName    string
	ClusterName    string
	Containers     []ECSContainer
	LastStatus     string
	DesiredStatus  string
}

type ECSContainer struct {
	ContainerName string
	ContainerArn  string
	TaskArn       string
	Shell         string
	Status        string
	Image         string
}

func NewECS(cfg aws.Config, region string) *ECSResource {
	return &ECSResource{
		client:     ecs.NewFromConfig(cfg),
		execRunner: &DefaultECSExecRunner{},
		Clusters:   []ECSCluster{},
		Region:     region,
	}
}

func newECSForTesting(client ECSClient, region string) *ECSResource {
	return &ECSResource{
		client:     client,
		execRunner: &DefaultECSExecRunner{},
		Clusters:   []ECSCluster{},
		Region:     region,
	}
}

func (e *ECSResource) ExecuteCommand(input ecs.ExecuteCommandInput) error {
	if e.client == nil {
		return fmt.Errorf("ECS client is not initialized")
	}

	ctx := context.TODO()
	preparedInput := e.buildExecuteCommandInput(input)

	execCommandOutput, err := e.client.ExecuteCommand(ctx, &preparedInput)
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	sessionInfo, err := json.Marshal(execCommandOutput.Session)
	if err != nil {
		return fmt.Errorf("failed to marshal session info: %w", err)
	}

	target := fmt.Sprintf("ecs:%s_%s_%s", *input.Cluster, *input.Task, *input.Container)
	targetJSON, err := e.buildSSMTargetJSON(target)
	if err != nil {
		return fmt.Errorf("failed to create target JSON: %w", err)
	}

	cmd := e.buildSessionManagerCommand(sessionInfo, targetJSON)
	return e.execRunner.RunCommand(cmd)
}

func (e *ECSResource) buildExecuteCommandInput(input ecs.ExecuteCommandInput) ecs.ExecuteCommandInput {
	return ecs.ExecuteCommandInput{
		Cluster:     aws.String(*input.Cluster),
		Command:     aws.String(*input.Command),
		Container:   aws.String(*input.Container),
		Interactive: true,
		Task:        aws.String(*input.Task),
	}
}

func (e *ECSResource) buildSSMTargetJSON(target string) ([]byte, error) {
	ssmTarget := struct {
		Target string `json:"Target"`
	}{
		Target: target,
	}
	return json.Marshal(ssmTarget)
}

func (e *ECSResource) buildSessionManagerCommand(sessionInfo, targetJSON []byte) *exec.Cmd {
	ssmEndpoint := "https://ssm." + e.Region + ".amazonaws.com"

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

func (e *ECSResource) ListClusters(ctx context.Context) error {
	resultClusters, err := e.client.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	clusters, err := e.parseClusterARNs(resultClusters.ClusterArns)
	if err != nil {
		return fmt.Errorf("failed to create clusters: %w", err)
	}
	e.Clusters = clusters
	return nil
}

func (e *ECSResource) parseClusterARNs(clusterArns []string) ([]ECSCluster, error) {
	var clusters []ECSCluster
	for _, arn := range clusterArns {
		if arn == "" {
			return nil, fmt.Errorf("empty cluster ARN provided")
		}
		parts := strings.Split(arn, "/")
		if len(parts) == 0 {
			return nil, fmt.Errorf("invalid cluster ARN format: %s", arn)
		}
		name := parts[len(parts)-1]
		clusters = append(clusters, ECSCluster{
			ClusterName: name,
			ClusterArn:  arn,
			Services:    []ECSService{},
		})
	}
	return clusters, nil
}

func (e *ECSResource) ListServices(ctx context.Context, cluster string) error {
	inputService := &ecs.ListServicesInput{Cluster: aws.String(cluster)}
	resultServices, err := e.client.ListServices(ctx, inputService)
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	services, err := e.parseServiceARNs(resultServices.ServiceArns, cluster)
	if err != nil {
		return fmt.Errorf("failed to create services: %w", err)
	}
	for i := range e.Clusters {
		if e.Clusters[i].ClusterName == cluster {
			e.Clusters[i].Services = services
			break
		}
	}
	return nil
}

func (e *ECSResource) parseServiceARNs(serviceArns []string, clusterName string) ([]ECSService, error) {
	var services []ECSService
	for _, arn := range serviceArns {
		if arn == "" {
			return nil, fmt.Errorf("empty service ARN provided")
		}
		parts := strings.Split(arn, "/")
		if len(parts) == 0 {
			return nil, fmt.Errorf("invalid service ARN format: %s", arn)
		}
		name := parts[len(parts)-1]
		services = append(services, ECSService{
			ServiceName: name,
			ServiceArn:  arn,
			ClusterName: clusterName,
			Tasks:       []ECSTask{},
		})
	}
	return services, nil
}

func (e *ECSResource) GetTasks(ctx context.Context, cluster, service string) error {
	inputTask := &ecs.ListTasksInput{
		Cluster:     aws.String(cluster),
		ServiceName: aws.String(service),
	}
	resultTasks, err := e.client.ListTasks(ctx, inputTask)
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	_, err = e.describeTasks(ctx, cluster, resultTasks.TaskArns)
	if err != nil {
		return err
	}

	return nil
}

func (e *ECSResource) describeTasks(ctx context.Context, cluster string, taskArns []string) ([]ECSTask, error) {
	var tasks []ECSTask
	for _, taskArn := range taskArns {
		task, err := e.describeTask(ctx, cluster, taskArn)
		if err != nil {
			log.Printf("Failed to describe task %s: %v", taskArn, err)
			continue
		}
		if task != nil {
			tasks = append(tasks, *task)
		}
	}
	return tasks, nil
}

func (e *ECSResource) describeTask(ctx context.Context, cluster, taskArn string) (*ECSTask, error) {
	describeTasksInput := &ecs.DescribeTasksInput{
		Tasks:   []string{taskArn},
		Cluster: aws.String(cluster),
	}
	describeTasksOutput, err := e.client.DescribeTasks(ctx, describeTasksInput)
	if err != nil {
		return nil, fmt.Errorf("failed to describe task: %w", err)
	}

	if len(describeTasksOutput.Tasks) == 0 {
		return nil, fmt.Errorf("task not found: %s", taskArn)
	}

	taskDefinitionArn := describeTasksOutput.Tasks[0].TaskDefinitionArn
	if taskDefinitionArn == nil {
		return nil, fmt.Errorf("task definition ARN is nil for task: %s", taskArn)
	}

	task := describeTasksOutput.Tasks[0]
	lastStatus := ""
	desiredStatus := ""
	if task.LastStatus != nil {
		lastStatus = *task.LastStatus
	}
	if task.DesiredStatus != nil {
		desiredStatus = *task.DesiredStatus
	}

	return &ECSTask{
		TaskArn:        taskArn,
		TaskDefinition: *taskDefinitionArn,
		ClusterName:    cluster,
		LastStatus:     lastStatus,
		DesiredStatus:  desiredStatus,
		Containers:     []ECSContainer{},
	}, nil
}

func (e *ECSResource) ListContainersForTask(ctx context.Context, taskDefinition string) ([]ECSContainer, error) {
	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinition),
	}

	result, err := e.client.DescribeTaskDefinition(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe task definition: %w", err)
	}

	containers, err := e.parseContainerDefinitions(result.TaskDefinition.ContainerDefinitions, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create containers: %w", err)
	}
	return containers, nil
}

func (e *ECSResource) parseContainerDefinitions(containerDefinitions []types.ContainerDefinition, taskArn string) ([]ECSContainer, error) {
	var containers []ECSContainer
	for _, container := range containerDefinitions {
		if container.Name == nil {
			return nil, fmt.Errorf("container name is nil")
		}
		image := ""
		if container.Image != nil {
			image = *container.Image
		}
		containers = append(containers, ECSContainer{
			ContainerName: *container.Name,
			TaskArn:       taskArn,
			Image:         image,
			Status:        "",
		})
	}
	return containers, nil
}

func (e *ECSResource) LoadAllResources(ctx context.Context) error {
	err := e.ListClusters(ctx)
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	for _, cluster := range e.Clusters {
		err = e.ListServices(ctx, cluster.ClusterName)
		if err != nil {
			return fmt.Errorf("failed to list services for cluster %s: %w", cluster.ClusterName, err)
		}

		for _, service := range cluster.Services {
			err = e.GetTasks(ctx, cluster.ClusterName, service.ServiceName)
			if err != nil {
				return fmt.Errorf("failed to get tasks for service %s: %w", service.ServiceName, err)
			}
		}
	}

	return nil
}

func (e *ECSResource) GetClusterResources(ctx context.Context, cluster ECSCluster) ([]ECSResource, error) {
	var resources []ECSResource

	if err := e.ListServices(ctx, cluster.ClusterName); err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	for _, service := range cluster.Services {
		serviceResources, err := e.GetServiceResources(ctx, cluster, service)
		if err != nil {
			return nil, err
		}
		resources = append(resources, serviceResources...)
	}

	return resources, nil
}

func (e *ECSResource) GetServiceResources(ctx context.Context, cluster ECSCluster, service ECSService) ([]ECSResource, error) {
	var resources []ECSResource

	if err := e.GetTasks(ctx, cluster.ClusterName, service.ServiceName); err != nil {
		return nil, fmt.Errorf("failed to get tasks: %w", err)
	}

	tasks, err := e.describeTasks(ctx, cluster.ClusterName, []string{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe tasks: %w", err)
	}

	if len(tasks) == 0 {
		return resources, nil
	}

	task := tasks[0]

	containers, err := e.ListContainersForTask(ctx, task.TaskDefinition)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	for range containers {
		resources = append(resources, ECSResource{
			Clusters: []ECSCluster{cluster},
		})
	}

	return resources, nil
}
