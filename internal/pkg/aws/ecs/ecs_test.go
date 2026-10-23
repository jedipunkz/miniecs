package ecs

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// errAccessDenied stands in for an AWS API error. Assertions use
// errors.Is on it to verify that wrapped and aggregated errors keep
// their chain intact.
var errAccessDenied = errors.New("access denied")

type MockECSClient struct {
	mock.Mock
}

func (m *MockECSClient) ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.ListClustersOutput), args.Error(1)
}

func (m *MockECSClient) ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.ListServicesOutput), args.Error(1)
}

func (m *MockECSClient) ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.ListTasksOutput), args.Error(1)
}

func (m *MockECSClient) DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.DescribeTasksOutput), args.Error(1)
}

func (m *MockECSClient) DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.DescribeTaskDefinitionOutput), args.Error(1)
}

func (m *MockECSClient) ExecuteCommand(ctx context.Context, params *ecs.ExecuteCommandInput, optFns ...func(*ecs.Options)) (*ecs.ExecuteCommandOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.ExecuteCommandOutput), args.Error(1)
}

func TestNewECS(t *testing.T) {
	cfg := aws.Config{}
	region := "ap-northeast-1"

	ecsResource := NewECS(cfg, region)

	assert.NotNil(t, ecsResource)
	assert.Equal(t, region, ecsResource.Region)
	assert.Empty(t, ecsResource.Clusters)
	assert.NotNil(t, ecsResource.client)
	assert.NotNil(t, ecsResource.execRunner)
}

func TestListClusters(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	mockClient.On("ListClusters", mock.Anything, &ecs.ListClustersInput{}).Return(&ecs.ListClustersOutput{
		ClusterArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:cluster/cluster1", "arn:aws:ecs:ap-northeast-1:123456789012:cluster/cluster2"},
	}, nil)

	err := ecsResource.ListClusters(context.Background())
	assert.NoError(t, err)
	assert.Len(t, ecsResource.Clusters, 2)
	assert.Equal(t, "cluster1", ecsResource.Clusters[0].ClusterName)
	assert.Equal(t, "cluster2", ecsResource.Clusters[1].ClusterName)
}

func TestListServices(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	mockClient.On("ListServices", mock.Anything, &ecs.ListServicesInput{
		Cluster: aws.String(clusterName),
	}).Return(&ecs.ListServicesOutput{
		ServiceArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:service/test-cluster/service1", "arn:aws:ecs:ap-northeast-1:123456789012:service/test-cluster/service2"},
	}, nil)

	// Initialize clusters first
	ecsResource.Clusters = []ECSCluster{{ClusterName: clusterName}}

	err := ecsResource.ListServices(context.Background(), clusterName)
	assert.NoError(t, err)
	assert.Len(t, ecsResource.Clusters, 1)
}

func TestGetTasks(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	serviceName := "test-service"
	taskArn := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-id"
	taskDefinitionArn := "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test-task:1"

	mockClient.On("ListTasks", mock.Anything, &ecs.ListTasksInput{
		Cluster:     aws.String(clusterName),
		ServiceName: aws.String(serviceName),
	}).Return(&ecs.ListTasksOutput{
		TaskArns: []string{taskArn},
	}, nil)

	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{taskArn},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{
			{
				TaskArn:           aws.String(taskArn),
				TaskDefinitionArn: aws.String(taskDefinitionArn),
			},
		},
	}, nil)

	err := ecsResource.GetTasks(context.Background(), clusterName, serviceName)
	assert.NoError(t, err)
}

func TestListContainersForTask(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	taskDefinition := "test-task:1"
	containerName := "test-container"

	mockClient.On("DescribeTaskDefinition", mock.Anything, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinition),
	}).Return(&ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &types.TaskDefinition{
			ContainerDefinitions: []types.ContainerDefinition{
				{
					Name: aws.String(containerName),
				},
			},
		},
	}, nil)

	containers, err := ecsResource.ListContainersForTask(context.Background(), taskDefinition)
	assert.NoError(t, err)
	assert.Len(t, containers, 1)
	assert.Equal(t, containerName, containers[0].ContainerName)
}

func TestListContainersForTaskNilTaskDefinition(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	taskDefinition := "test-task:1"

	mockClient.On("DescribeTaskDefinition", mock.Anything, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinition),
	}).Return(&ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: nil,
	}, nil)

	_, err := ecsResource.ListContainersForTask(context.Background(), taskDefinition)
	assert.ErrorContains(t, err, "no task definition returned for: "+taskDefinition)
}

func TestListContainersForTaskRejectsEmptyTaskDefinition(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	_, err := ecsResource.ListContainersForTask(context.Background(), "")
	assert.ErrorContains(t, err, "task definition is required")
	mockClient.AssertNotCalled(t, "DescribeTaskDefinition", mock.Anything, mock.Anything)
}

func TestDescribeTasksAggregatesFailures(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	okArn := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-ok"
	badArn1 := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-bad-1"
	badArn2 := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-bad-2"
	taskDefArn := "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test-task:1"

	deniedErr := errors.New("access denied")
	notFoundErr := errors.New("task not found")

	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{okArn},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{{
			TaskArn:           aws.String(okArn),
			TaskDefinitionArn: aws.String(taskDefArn),
		}},
	}, nil)

	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{badArn1},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{}, fmt.Errorf("describe failed: %w", deniedErr))

	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{badArn2},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{}, nil) // Not found: zero tasks returned

	tasks, err := ecsResource.describeTasks(context.Background(), clusterName, []string{okArn, badArn1, badArn2})
	assert.Len(t, tasks, 1)
	assert.Equal(t, okArn, tasks[0].TaskArn)

	assert.Error(t, err)
	assert.ErrorIs(t, err, deniedErr)
	assert.ErrorContains(t, err, notFoundErr.Error())
	assert.ErrorContains(t, err, badArn1)
	assert.ErrorContains(t, err, badArn2)
}

func TestDescribeTasksNoFailures(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	okArn := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-ok"
	taskDefArn := "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test-task:1"

	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{okArn},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{{
			TaskArn:           aws.String(okArn),
			TaskDefinitionArn: aws.String(taskDefArn),
		}},
	}, nil)

	tasks, err := ecsResource.describeTasks(context.Background(), clusterName, []string{okArn})
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
}

func TestGetServiceResourcesPropagatesDescribeTaskFailure(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	serviceName := "test-service"
	taskArn := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-1"
	listInput := &ecs.ListTasksInput{
		Cluster:     aws.String(clusterName),
		ServiceName: aws.String(serviceName),
	}

	// GetServiceResources lists tasks twice: once through GetTasks and
	// once for itself. GetTasks must succeed so the direct describeTasks
	// call is the one that fails.
	mockClient.On("ListTasks", mock.Anything, listInput).Return(&ecs.ListTasksOutput{
		TaskArns: []string{taskArn},
	}, nil).Once()
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{taskArn},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{{
			TaskArn:           aws.String(taskArn),
			TaskDefinitionArn: aws.String("arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test-task:1"),
		}},
	}, nil).Once()
	mockClient.On("ListTasks", mock.Anything, listInput).Return(&ecs.ListTasksOutput{
		TaskArns: []string{taskArn},
	}, nil).Once()
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{taskArn},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{}, fmt.Errorf("describe failed: %w", errAccessDenied)).Once()

	resources, err := ecsResource.GetServiceResources(
		context.Background(),
		ECSCluster{ClusterName: clusterName},
		ECSService{ServiceName: serviceName},
	)
	assert.Nil(t, resources)
	assert.Error(t, err)
	assert.ErrorIs(t, err, errAccessDenied)
	assert.ErrorContains(t, err, "failed to describe tasks")
}

func TestGetServiceResourcesAggregatesContainerFailures(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	serviceName := "test-service"
	okTaskArn := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-ok"
	badTaskArn := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-bad"
	okTaskDef := "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/ok:1"
	badTaskDef := "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/bad:1"
	listInput := &ecs.ListTasksInput{
		Cluster:     aws.String(clusterName),
		ServiceName: aws.String(serviceName),
	}

	// describeTask issues one DescribeTasks call per task ARN, and
	// GetServiceResources runs describeTasks twice (once via GetTasks).
	mockClient.On("ListTasks", mock.Anything, listInput).Return(&ecs.ListTasksOutput{
		TaskArns: []string{okTaskArn, badTaskArn},
	}, nil).Twice()
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{okTaskArn},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{{
			TaskArn:           aws.String(okTaskArn),
			TaskDefinitionArn: aws.String(okTaskDef),
		}},
	}, nil).Twice()
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{badTaskArn},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{{
			TaskArn:           aws.String(badTaskArn),
			TaskDefinitionArn: aws.String(badTaskDef),
		}},
	}, nil).Twice()
	mockClient.On("DescribeTaskDefinition", mock.Anything, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(okTaskDef),
	}).Return(&ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &types.TaskDefinition{
			ContainerDefinitions: []types.ContainerDefinition{
				{Name: aws.String("container-1")},
			},
		},
	}, nil)
	mockClient.On("DescribeTaskDefinition", mock.Anything, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(badTaskDef),
	}).Return(&ecs.DescribeTaskDefinitionOutput{}, fmt.Errorf("describe failed: %w", errAccessDenied))

	resources, err := ecsResource.GetServiceResources(
		context.Background(),
		ECSCluster{ClusterName: clusterName},
		ECSService{ServiceName: serviceName},
	)
	assert.Nil(t, resources, "a single failing task must fail the whole service listing, not silently drop the task")
	assert.Error(t, err)
	assert.ErrorIs(t, err, errAccessDenied)
	assert.ErrorContains(t, err, badTaskArn)
}

func TestExecuteCommandRequiresFields(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	empty := ""
	cases := []struct {
		name    string
		mutator func(input *ecs.ExecuteCommandInput)
		wantMsg string
	}{
		{
			name:    "nil cluster",
			mutator: func(input *ecs.ExecuteCommandInput) { input.Cluster = nil },
			wantMsg: "cluster is required",
		},
		{
			name:    "empty cluster",
			mutator: func(input *ecs.ExecuteCommandInput) { input.Cluster = &empty },
			wantMsg: "cluster is required",
		},
		{
			name:    "nil command",
			mutator: func(input *ecs.ExecuteCommandInput) { input.Command = nil },
			wantMsg: "command is required",
		},
		{
			name:    "empty command",
			mutator: func(input *ecs.ExecuteCommandInput) { input.Command = &empty },
			wantMsg: "command is required",
		},
		{
			name:    "nil container",
			mutator: func(input *ecs.ExecuteCommandInput) { input.Container = nil },
			wantMsg: "container is required",
		},
		{
			name:    "empty container",
			mutator: func(input *ecs.ExecuteCommandInput) { input.Container = &empty },
			wantMsg: "container is required",
		},
		{
			name:    "nil task",
			mutator: func(input *ecs.ExecuteCommandInput) { input.Task = nil },
			wantMsg: "task is required",
		},
		{
			name:    "empty task",
			mutator: func(input *ecs.ExecuteCommandInput) { input.Task = &empty },
			wantMsg: "task is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := ecs.ExecuteCommandInput{
				Cluster:   aws.String("test-cluster"),
				Command:   aws.String("sh"),
				Container: aws.String("test-container"),
				Task:      aws.String("test-task"),
			}
			tc.mutator(&input)

			err := ecsResource.ExecuteCommand(input)
			assert.ErrorContains(t, err, tc.wantMsg)
		})
	}

	mockClient.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything)
}

func TestExecuteCommandClientNotInitialized(t *testing.T) {
	ecsResource := &ECSResource{}

	err := ecsResource.ExecuteCommand(ecs.ExecuteCommandInput{})
	assert.ErrorContains(t, err, "ECS client is not initialized")
}

type MockECSExecRunner struct {
	mock.Mock
}

func (m *MockECSExecRunner) RunCommand(cmd *exec.Cmd) error {
	args := m.Called(cmd)
	return args.Error(0)
}

func TestExecuteCommandEndToEnd(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")
	runner := new(MockECSExecRunner)
	ecsResource.execRunner = runner

	mockClient.On("ExecuteCommand", mock.Anything, &ecs.ExecuteCommandInput{
		Cluster:     aws.String("test-cluster"),
		Command:     aws.String("sh"),
		Container:   aws.String("test-container"),
		Interactive: true,
		Task:        aws.String("test-task"),
	}).Return(&ecs.ExecuteCommandOutput{
		Session: &types.Session{},
	}, nil)
	runner.On("RunCommand", mock.Anything).Return(nil)

	err := ecsResource.ExecuteCommand(ecs.ExecuteCommandInput{
		Cluster:   aws.String("test-cluster"),
		Command:   aws.String("sh"),
		Container: aws.String("test-container"),
		Task:      aws.String("test-task"),
	})
	assert.NoError(t, err)
}
