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

// Target is the flat per-container value the CLI consumes. One Target
// describes exactly one container of one task, so callers never navigate
// a cluster → service → task hierarchy.
type Target struct {
	Cluster        string
	Service        string
	TaskArn        string
	TaskDefinition string
	Container      string
	Image          string
	Status         string
}

// describedTask carries the task-level fields extracted from
// DescribeTasks. Its containers are resolved separately per task
// definition.
type describedTask struct {
	TaskArn        string
	TaskDefinition string
	LastStatus     string
}

// containerInfo is one container definition entry of a task definition.
type containerInfo struct {
	Name  string
	Image string
}

// Client wraps the ECS API client and exposes the listing and execution
// operations on top of it. It holds no resource state: every method
// returns its data, so call order does not matter.
type Client struct {
	client     ECSClient
	execRunner ECSExecRunner

	Region string
}

func NewClient(cfg aws.Config, region string) *Client {
	return &Client{
		client:     ecs.NewFromConfig(cfg),
		execRunner: &DefaultECSExecRunner{},
		Region:     region,
	}
}

func newClientForTesting(client ECSClient, region string) *Client {
	return &Client{
		client:     client,
		execRunner: &DefaultECSExecRunner{},
		Region:     region,
	}
}

func (c *Client) ExecuteCommand(ctx context.Context, target Target, shell string) error {
	if c.client == nil {
		return fmt.Errorf("ECS client is not initialized")
	}

	// Preflight before the ExecuteCommand API call: creating a session
	// that the user cannot start leaves an unusable session behind.
	if _, err := exec.LookPath(sessionManagerPluginName); err != nil {
		return fmt.Errorf("%s not found in PATH: install it following %s: %w",
			sessionManagerPluginName, ssmPluginInstallDocsURL, err)
	}

	preparedInput, err := buildExecuteCommandInput(target, shell)
	if err != nil {
		return fmt.Errorf("failed to build execute command input: %w", err)
	}

	execCommandOutput, err := c.client.ExecuteCommand(ctx, &preparedInput)
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	sessionInfo, err := json.Marshal(execCommandOutput.Session)
	if err != nil {
		return fmt.Errorf("failed to marshal session info: %w", err)
	}

	ssmTarget := fmt.Sprintf("ecs:%s_%s_%s", *preparedInput.Cluster, *preparedInput.Task, *preparedInput.Container)
	targetJSON, err := c.buildSSMTargetJSON(ssmTarget)
	if err != nil {
		return fmt.Errorf("failed to create target JSON: %w", err)
	}

	cmd := c.buildSessionManagerCommand(sessionInfo, targetJSON)
	return c.execRunner.RunCommand(cmd)
}

// buildExecuteCommandInput validates the required fields and returns a
// rebuilt input. Empty fields would be sent to the ECS API as empty
// values, so they are rejected here before any API call.
func buildExecuteCommandInput(target Target, shell string) (ecs.ExecuteCommandInput, error) {
	if err := requireNonEmpty(target.Cluster, "cluster"); err != nil {
		return ecs.ExecuteCommandInput{}, err
	}
	if err := requireNonEmpty(target.TaskArn, "task"); err != nil {
		return ecs.ExecuteCommandInput{}, err
	}
	if err := requireNonEmpty(target.Container, "container"); err != nil {
		return ecs.ExecuteCommandInput{}, err
	}
	if err := requireNonEmpty(shell, "shell"); err != nil {
		return ecs.ExecuteCommandInput{}, err
	}

	return ecs.ExecuteCommandInput{
		Cluster:     aws.String(target.Cluster),
		Command:     aws.String(shell),
		Container:   aws.String(target.Container),
		Interactive: true,
		Task:        aws.String(target.TaskArn),
	}, nil
}

func requireNonEmpty(value, name string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func (c *Client) buildSSMTargetJSON(target string) ([]byte, error) {
	ssmTarget := struct {
		Target string `json:"Target"`
	}{
		Target: target,
	}
	return json.Marshal(ssmTarget)
}

func (c *Client) buildSessionManagerCommand(sessionInfo, targetJSON []byte) *exec.Cmd {
	cmd := exec.Command(
		sessionManagerPluginName,
		string(sessionInfo),
		c.Region,
		"StartSession",
		"",
		string(targetJSON),
		resolveSSMEndpoint(c.Region),
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

// ListClusters returns the cluster names of the region, following
// ListClusters pagination until no NextToken is returned.
func (c *Client) ListClusters(ctx context.Context) ([]string, error) {
	clusterArns, err := c.listClusterARNs(ctx)
	if err != nil {
		return nil, err
	}

	names, err := parseClusterARNs(clusterArns)
	if err != nil {
		return nil, fmt.Errorf("failed to create clusters: %w", err)
	}
	return names, nil
}

// parseClusterARNs maps cluster ARNs to cluster names, rejecting empty
// ARNs. strings.Split always returns at least one element, so the last
// "/"-separated element is the name.
func parseClusterARNs(clusterArns []string) ([]string, error) {
	var names []string
	for _, arn := range clusterArns {
		if arn == "" {
			return nil, fmt.Errorf("empty cluster ARN provided")
		}
		parts := strings.Split(arn, "/")
		names = append(names, parts[len(parts)-1])
	}
	return names, nil
}

// ListServices returns the service names of a cluster, following
// ListServices pagination until no NextToken is returned.
func (c *Client) ListServices(ctx context.Context, cluster string) ([]string, error) {
	serviceArns, err := c.listServiceARNs(ctx, cluster)
	if err != nil {
		return nil, err
	}

	names, err := parseServiceARNs(serviceArns)
	if err != nil {
		return nil, fmt.Errorf("failed to create services: %w", err)
	}
	return names, nil
}

// parseServiceARNs maps service ARNs to service names, rejecting empty
// ARNs. strings.Split always returns at least one element, so the last
// "/"-separated element is the name.
func parseServiceARNs(serviceArns []string) ([]string, error) {
	var names []string
	for _, arn := range serviceArns {
		if arn == "" {
			return nil, fmt.Errorf("empty service ARN provided")
		}
		parts := strings.Split(arn, "/")
		names = append(names, parts[len(parts)-1])
	}
	return names, nil
}

// listTaskArns returns the task ARNs of a service, following ListTasks
// pagination until no NextToken is returned.
func (c *Client) listTaskArns(ctx context.Context, cluster, service string) ([]string, error) {
	paginator := ecs.NewListTasksPaginator(c.client, &ecs.ListTasksInput{
		Cluster:     aws.String(cluster),
		ServiceName: aws.String(service),
		MaxResults:  aws.Int32(100),
	})
	var taskArns []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list tasks: %w", err)
		}
		taskArns = append(taskArns, page.TaskArns...)
	}
	return taskArns, nil
}

// ListTargets lists every task of a service, describes each task and its
// task definition, and returns one flat Target per container. Per-task
// failures are aggregated with errors.Join and returned with no targets:
// a partial list would silently omit tasks the user cannot see, so any
// per-task failure is fatal for this service.
func (c *Client) ListTargets(ctx context.Context, cluster, service string) ([]Target, error) {
	taskArns, err := c.listTaskArns(ctx, cluster, service)
	if err != nil {
		return nil, err
	}

	tasks, err := c.describeTasks(ctx, cluster, taskArns)
	if err != nil {
		return nil, fmt.Errorf("failed to describe tasks: %w", err)
	}

	var targets []Target
	var containerErrs []error
	for _, task := range tasks {
		containers, err := c.listContainersForTask(ctx, task.TaskDefinition)
		if err != nil {
			containerErrs = append(containerErrs, fmt.Errorf("failed to list containers for task %s: %w", task.TaskArn, err))
			continue
		}

		for _, container := range containers {
			targets = append(targets, Target{
				Cluster:        cluster,
				Service:        service,
				TaskArn:        task.TaskArn,
				TaskDefinition: task.TaskDefinition,
				Container:      container.Name,
				Image:          container.Image,
				Status:         task.LastStatus,
			})
		}
	}

	if err := errors.Join(containerErrs...); err != nil {
		return nil, err
	}

	return targets, nil
}

// describeTasks describes every listed task and keeps the ones that
// succeed. A task that fails to describe is skipped so the listing stays
// useful, but the failure is not swallowed: it is aggregated with
// errors.Join and returned, so callers treat the run as failed with the
// real causes instead of showing a silently incomplete list.
func (c *Client) describeTasks(ctx context.Context, cluster string, taskArns []string) ([]describedTask, error) {
	var tasks []describedTask
	var taskErrs []error
	for _, taskArn := range taskArns {
		task, err := c.describeTask(ctx, cluster, taskArn)
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

func (c *Client) describeTask(ctx context.Context, cluster, taskArn string) (*describedTask, error) {
	describeTasksInput := &ecs.DescribeTasksInput{
		Tasks:   []string{taskArn},
		Cluster: aws.String(cluster),
	}
	describeTasksOutput, err := c.client.DescribeTasks(ctx, describeTasksInput)
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
	if task.LastStatus != nil {
		lastStatus = *task.LastStatus
	}

	return &describedTask{
		TaskArn:        taskArn,
		TaskDefinition: *taskDefinitionArn,
		LastStatus:     lastStatus,
	}, nil
}

// listContainersForTask resolves the container definitions of a task
// definition through the DescribeTaskDefinition API.
func (c *Client) listContainersForTask(ctx context.Context, taskDefinition string) ([]containerInfo, error) {
	if taskDefinition == "" {
		return nil, fmt.Errorf("task definition is required")
	}

	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinition),
	}

	result, err := c.client.DescribeTaskDefinition(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe task definition: %w", err)
	}

	if result.TaskDefinition == nil {
		return nil, fmt.Errorf("no task definition returned for: %s", taskDefinition)
	}

	containers, err := parseContainerDefinitions(result.TaskDefinition.ContainerDefinitions)
	if err != nil {
		return nil, fmt.Errorf("failed to create containers: %w", err)
	}
	return containers, nil
}

func parseContainerDefinitions(containerDefinitions []types.ContainerDefinition) ([]containerInfo, error) {
	var containers []containerInfo
	for _, container := range containerDefinitions {
		if container.Name == nil {
			return nil, fmt.Errorf("container name is nil")
		}
		image := ""
		if container.Image != nil {
			image = *container.Image
		}
		containers = append(containers, containerInfo{
			Name:  *container.Name,
			Image: image,
		})
	}
	return containers, nil
}

func (c *Client) listClusterARNs(ctx context.Context) ([]string, error) {
	paginator := ecs.NewListClustersPaginator(c.client, &ecs.ListClustersInput{
		MaxResults: aws.Int32(100),
	})
	var clusterArns []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list clusters: %w", err)
		}
		clusterArns = append(clusterArns, page.ClusterArns...)
	}
	return clusterArns, nil
}

func (c *Client) listServiceARNs(ctx context.Context, cluster string) ([]string, error) {
	paginator := ecs.NewListServicesPaginator(c.client, &ecs.ListServicesInput{
		Cluster:    aws.String(cluster),
		MaxResults: aws.Int32(100),
	})
	var serviceArns []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list services: %w", err)
		}
		serviceArns = append(serviceArns, page.ServiceArns...)
	}
	return serviceArns, nil
}
