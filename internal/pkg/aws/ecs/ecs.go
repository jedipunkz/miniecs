package ecs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// sessionManagerPluginName is the fixed binary executed for ECS Exec
// sessions. It stays constant: the executable must never be chosen from
// user input, flags or AWS responses.
const sessionManagerPluginName = "session-manager-plugin"

// ssmPluginInstallDocsURL points to the AWS install instructions shown
// when the plugin is missing, so the failure is actionable.
const ssmPluginInstallDocsURL = "https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-plugin.html"

// ssmEndpointURLEnvName mirrors the service-level endpoint override of
// the AWS SDKs (AWS_ENDPOINT_URL_<SERVICE_ID>).
const ssmEndpointURLEnvName = "AWS_ENDPOINT_URL_SSM"

// globalEndpointURLEnvName is the SDK-wide endpoint override that applies
// to all services, including SSM.
const globalEndpointURLEnvName = "AWS_ENDPOINT_URL"

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

	// Preflight before the ExecuteCommand API call: creating a session
	// that the user cannot start leaves an unusable session behind.
	if _, err := exec.LookPath(sessionManagerPluginName); err != nil {
		return fmt.Errorf("%s not found in PATH: install it following %s: %w",
			sessionManagerPluginName, ssmPluginInstallDocsURL, err)
	}

	ctx := context.TODO()
	preparedInput, err := e.buildExecuteCommandInput(input)
	if err != nil {
		return fmt.Errorf("failed to build execute command input: %w", err)
	}

	execCommandOutput, err := e.client.ExecuteCommand(ctx, &preparedInput)
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	sessionInfo, err := json.Marshal(execCommandOutput.Session)
	if err != nil {
		return fmt.Errorf("failed to marshal session info: %w", err)
	}

	target := fmt.Sprintf("ecs:%s_%s_%s", *preparedInput.Cluster, *preparedInput.Task, *preparedInput.Container)
	targetJSON, err := e.buildSSMTargetJSON(target)
	if err != nil {
		return fmt.Errorf("failed to create target JSON: %w", err)
	}

	cmd := e.buildSessionManagerCommand(sessionInfo, targetJSON)
	return e.execRunner.RunCommand(cmd)
}

// buildExecuteCommandInput validates the required fields and returns a
// rebuilt input. Nil or empty fields would either panic when dereferenced
// or be sent to the ECS API as empty values, so they are rejected here
// before any API call.
func (e *ECSResource) buildExecuteCommandInput(input ecs.ExecuteCommandInput) (ecs.ExecuteCommandInput, error) {
	if err := requireString(input.Cluster, "cluster"); err != nil {
		return ecs.ExecuteCommandInput{}, err
	}
	if err := requireString(input.Command, "command"); err != nil {
		return ecs.ExecuteCommandInput{}, err
	}
	if err := requireString(input.Container, "container"); err != nil {
		return ecs.ExecuteCommandInput{}, err
	}
	if err := requireString(input.Task, "task"); err != nil {
		return ecs.ExecuteCommandInput{}, err
	}

	return ecs.ExecuteCommandInput{
		Cluster:     aws.String(*input.Cluster),
		Command:     aws.String(*input.Command),
		Container:   aws.String(*input.Container),
		Interactive: true,
		Task:        aws.String(*input.Task),
	}, nil
}

func requireString(value *string, name string) error {
	if value == nil || *value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
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
	cmd := exec.Command(
		sessionManagerPluginName,
		string(sessionInfo),
		e.Region,
		"StartSession",
		"",
		string(targetJSON),
		resolveSSMEndpoint(e.Region),
	)

	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd
}

// resolveSSMEndpoint returns the SSM endpoint that session-manager-plugin
// connects to. It honors the SDK endpoint-overrides first (the
// service-level AWS_ENDPOINT_URL_SSM wins over the global AWS_ENDPOINT_URL,
// mirroring SDK precedence) and falls back to deriving the host from the
// region's partition DNS suffix.
//
// The full SDK endpoint resolver data lives in an internal SDK module
// (github.com/aws/aws-sdk-go-v2/internal/endpoints/v2), which cannot be
// imported outside the SDK; partition suffix matching is the smallest
// approach that fixes China regions, while FIPS / dual-stack / custom
// endpoints stay covered by the overrides above.
func resolveSSMEndpoint(region string) string {
	if endpoint := os.Getenv(ssmEndpointURLEnvName); endpoint != "" {
		return endpoint
	}
	if endpoint := os.Getenv(globalEndpointURLEnvName); endpoint != "" {
		return endpoint
	}
	return fmt.Sprintf("https://ssm.%s.%s", region, ssmHostSuffix(region))
}

// ssmHostSuffix returns the DNS suffix shared by SSM hosts in the
// region's partition.
func ssmHostSuffix(region string) string {
	switch {
	case strings.HasPrefix(region, "cn-"):
		return "amazonaws.com.cn"
	default:
		return "amazonaws.com"
	}
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

// describeTasks describes every listed task and keeps the ones that
// succeed. A task that fails to describe is skipped so the listing stays
// useful, but the failure is not swallowed: it is aggregated with
// errors.Join and returned, so callers treat the run as failed with the
// real causes instead of showing a silently incomplete list.
func (e *ECSResource) describeTasks(ctx context.Context, cluster string, taskArns []string) ([]ECSTask, error) {
	var tasks []ECSTask
	var taskErrs []error
	for _, taskArn := range taskArns {
		task, err := e.describeTask(ctx, cluster, taskArn)
		if err != nil {
			taskErrs = append(taskErrs, fmt.Errorf("task %s: %w", taskArn, err))
			continue
		}
		if task != nil {
			tasks = append(tasks, *task)
		}
	}
	return tasks, errors.Join(taskErrs...)
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
	if taskDefinition == "" {
		return nil, fmt.Errorf("task definition is required")
	}

	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinition),
	}

	result, err := e.client.DescribeTaskDefinition(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe task definition: %w", err)
	}

	if result.TaskDefinition == nil {
		return nil, fmt.Errorf("no task definition returned for: %s", taskDefinition)
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

	// Find the updated cluster with services from e.Clusters
	var updatedCluster *ECSCluster
	for i := range e.Clusters {
		if e.Clusters[i].ClusterName == cluster.ClusterName {
			updatedCluster = &e.Clusters[i]
			break
		}
	}

	if updatedCluster == nil {
		return nil, fmt.Errorf("cluster not found after listing services: %s", cluster.ClusterName)
	}

	for _, service := range updatedCluster.Services {
		serviceResources, err := e.GetServiceResources(ctx, *updatedCluster, service)
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

	// Get all tasks for this service
	inputTask := &ecs.ListTasksInput{
		Cluster:     aws.String(cluster.ClusterName),
		ServiceName: aws.String(service.ServiceName),
	}
	resultTasks, err := e.client.ListTasks(ctx, inputTask)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	tasks, err := e.describeTasks(ctx, cluster.ClusterName, resultTasks.TaskArns)
	if err != nil {
		return nil, fmt.Errorf("failed to describe tasks: %w", err)
	}

	if len(tasks) == 0 {
		return resources, nil
	}

	// For each task, get containers and create a complete resource. A
	// task whose task definition cannot be described is skipped, but the
	// failures are aggregated with errors.Join and returned (never logged
	// and hidden): a partial list would silently omit tasks the user
	// cannot see, so any per-task failure is fatal for this service.
	var containerErrs []error
	for _, task := range tasks {
		containers, err := e.ListContainersForTask(ctx, task.TaskDefinition)
		if err != nil {
			containerErrs = append(containerErrs, fmt.Errorf("failed to list containers for task %s: %w", task.TaskArn, err))
			continue
		}

		// Update task with containers
		task.Containers = containers

		// Create a complete resource with all hierarchical data
		resource := ECSResource{
			Clusters: []ECSCluster{{
				ClusterName: cluster.ClusterName,
				ClusterArn:  cluster.ClusterArn,
				Services: []ECSService{{
					ServiceName: service.ServiceName,
					ServiceArn:  service.ServiceArn,
					ClusterName: cluster.ClusterName,
					Tasks:       []ECSTask{task},
				}},
			}},
		}
		resources = append(resources, resource)
	}

	if err := errors.Join(containerErrs...); err != nil {
		return nil, err
	}

	return resources, nil
}
