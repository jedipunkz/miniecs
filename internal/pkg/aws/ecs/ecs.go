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

type CommandRunner interface {
	Run(cmd *exec.Cmd) error
}

type DefaultCommandRunner struct{}

func (r *DefaultCommandRunner) Run(cmd *exec.Cmd) error {
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
	client ECSClient
	runner CommandRunner

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
		runner:     &DefaultCommandRunner{},
		Clusters:   []Cluster{},
		Services:   []Service{},
		Tasks:      []Task{},
		Containers: []Container{},
		Region:     region,
	}
}

func NewEcsWithClient(client ECSClient, region string) *ECSResource {
	return &ECSResource{
		client:     client,
		runner:     &DefaultCommandRunner{},
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

	ctx := context.TODO()
	preparedInput := prepareExecuteCommandInput(input)

	execCommandOutput, err := e.client.ExecuteCommand(ctx, &preparedInput)
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	sessionInfo, err := json.Marshal(execCommandOutput.Session)
	if err != nil {
		return fmt.Errorf("failed to marshal session info: %w", err)
	}

	target := fmt.Sprintf("ecs:%s_%s_%s", *input.Cluster, *input.Task, *input.Container)
	targetJSON, err := createTargetJSON(target)
	if err != nil {
		return fmt.Errorf("failed to create target JSON: %w", err)
	}

	cmd := createSessionManagerCommand(e.Region, sessionInfo, targetJSON)
	return e.runner.Run(cmd)
}

func prepareExecuteCommandInput(input ecs.ExecuteCommandInput) ecs.ExecuteCommandInput {
	return ecs.ExecuteCommandInput{
		Cluster:     aws.String(*input.Cluster),
		Command:     aws.String(*input.Command),
		Container:   aws.String(*input.Container),
		Interactive: true,
		Task:        aws.String(*input.Task),
	}
}

func createTargetJSON(target string) ([]byte, error) {
	ssmTarget := struct {
		Target string `json:"Target"`
	}{
		Target: target,
	}
	return json.Marshal(ssmTarget)
}

func createSessionManagerCommand(region string, sessionInfo, targetJSON []byte) *exec.Cmd {
	ssmEndpoint := "https://ssm." + region + ".amazonaws.com"

	cmd := exec.Command(
		"session-manager-plugin",
		string(sessionInfo),
		region,
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

	clusterNames := extractClusterNames(resultClusters.ClusterArns)
	e.Clusters = createClusters(clusterNames)
	return nil
}

func extractClusterNames(clusterArns []string) []string {
	var names []string
	for _, cluster := range clusterArns {
		parts := strings.Split(cluster, "/")
		names = append(names, parts[len(parts)-1])
	}
	return names
}

func createClusters(names []string) []Cluster {
	var clusters []Cluster
	for _, name := range names {
		clusters = append(clusters, Cluster{ClusterName: name})
	}
	return clusters
}

func (e *ECSResource) ListServices(ctx context.Context, cluster string) error {
	inputService := &ecs.ListServicesInput{Cluster: aws.String(cluster)}
	resultServices, err := e.client.ListServices(ctx, inputService)
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	serviceNames := extractServiceNames(resultServices.ServiceArns)
	e.Services = createServices(serviceNames)
	return nil
}

func extractServiceNames(serviceArns []string) []string {
	var names []string
	for _, service := range serviceArns {
		parts := strings.Split(service, "/")
		names = append(names, parts[len(parts)-1])
	}
	return names
}

func createServices(names []string) []Service {
	var services []Service
	for _, name := range names {
		services = append(services, Service{ServiceName: name})
	}
	return services
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

	tasks, err := e.describeTasks(ctx, cluster, resultTasks.TaskArns)
	if err != nil {
		return err
	}

	e.Tasks = tasks
	return nil
}

func (e *ECSResource) describeTasks(ctx context.Context, cluster string, taskArns []string) ([]Task, error) {
	var tasks []Task
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

func (e *ECSResource) describeTask(ctx context.Context, cluster, taskArn string) (*Task, error) {
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

	return &Task{
		TaskArn:        taskArn,
		TaskDefinition: *taskDefinitionArn,
	}, nil
}

func (e *ECSResource) describeTaskDefinition(ctx context.Context, taskDefinitionArn *string) (*ecs.DescribeTaskDefinitionOutput, error) {
	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: taskDefinitionArn,
	}
	return e.client.DescribeTaskDefinition(ctx, input)
}

func (e *ECSResource) ListContainers(ctx context.Context, taskDefinition string) error {
	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinition),
	}

	result, err := e.client.DescribeTaskDefinition(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to describe task definition: %w", err)
	}

	e.Containers = extractContainers(result.TaskDefinition.ContainerDefinitions)
	return nil
}

func extractContainers(containerDefinitions []types.ContainerDefinition) []Container {
	var containers []Container
	for _, container := range containerDefinitions {
		containers = append(containers, Container{
			ContainerName: *container.Name,
		})
	}
	return containers
}

func (e *ECSResource) CollectECSResources(ctx context.Context) error {
	err := e.ListClusters(ctx)
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	for _, cluster := range e.Clusters {
		err = e.ListServices(ctx, cluster.ClusterName)
		if err != nil {
			return fmt.Errorf("failed to list services for cluster %s: %w", cluster.ClusterName, err)
		}

		for _, service := range e.Services {
			err = e.GetTasks(ctx, cluster.ClusterName, service.ServiceName)
			if err != nil {
				return fmt.Errorf("failed to get tasks for service %s: %w", service.ServiceName, err)
			}
		}
	}

	return nil
}

func (e *ECSResource) CollectServicesAndContainers(ctx context.Context, cluster Cluster) ([]ECSResource, error) {
	var resources []ECSResource

	if err := e.ListServices(ctx, cluster.ClusterName); err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	for _, service := range e.Services {
		serviceResources, err := e.CollectTasksAndContainers(ctx, cluster, service)
		if err != nil {
			return nil, err
		}
		resources = append(resources, serviceResources...)
	}

	return resources, nil
}

func (e *ECSResource) CollectTasksAndContainers(ctx context.Context, cluster Cluster, service Service) ([]ECSResource, error) {
	var resources []ECSResource

	if err := e.GetTasks(ctx, cluster.ClusterName, service.ServiceName); err != nil {
		return nil, fmt.Errorf("failed to get tasks: %w", err)
	}

	if len(e.Tasks) == 0 {
		return resources, nil
	}

	task := e.Tasks[0]
	e.Containers = nil

	if err := e.ListContainers(ctx, task.TaskDefinition); err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	for _, container := range e.Containers {
		resources = append(resources, ECSResource{
			Clusters:   []Cluster{cluster},
			Services:   []Service{service},
			Tasks:      []Task{task},
			Containers: []Container{container},
		})
	}

	return resources, nil
}
