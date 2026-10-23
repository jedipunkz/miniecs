package ecs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	mockClient.On("ListClusters", mock.Anything, &ecs.ListClustersInput{MaxResults: aws.Int32(100)}).Return(&ecs.ListClustersOutput{
		ClusterArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:cluster/cluster1", "arn:aws:ecs:ap-northeast-1:123456789012:cluster/cluster2"},
	}, nil)

	err := ecsResource.ListClusters(context.Background())
	assert.NoError(t, err)
	assert.Len(t, ecsResource.Clusters, 2)
	assert.Equal(t, "cluster1", ecsResource.Clusters[0].ClusterName)
	assert.Equal(t, "cluster2", ecsResource.Clusters[1].ClusterName)
}

func TestListClustersPaginated(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	mockClient.On("ListClusters", mock.Anything, &ecs.ListClustersInput{MaxResults: aws.Int32(100)}).Return(&ecs.ListClustersOutput{
		ClusterArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:cluster/cluster1", "arn:aws:ecs:ap-northeast-1:123456789012:cluster/cluster2"},
		NextToken:   aws.String("page-2"),
	}, nil)
	mockClient.On("ListClusters", mock.Anything, &ecs.ListClustersInput{
		MaxResults: aws.Int32(100),
		NextToken:  aws.String("page-2"),
	}).Return(&ecs.ListClustersOutput{
		ClusterArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:cluster/cluster3"},
	}, nil)

	err := ecsResource.ListClusters(context.Background())
	assert.NoError(t, err)
	assert.Len(t, ecsResource.Clusters, 3)
	assert.Equal(t, "cluster1", ecsResource.Clusters[0].ClusterName)
	assert.Equal(t, "cluster2", ecsResource.Clusters[1].ClusterName)
	assert.Equal(t, "cluster3", ecsResource.Clusters[2].ClusterName)
}

func TestListClustersListError(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	mockClient.On("ListClusters", mock.Anything, &ecs.ListClustersInput{MaxResults: aws.Int32(100)}).Return(
		(*ecs.ListClustersOutput)(nil), errors.New("access denied"))

	err := ecsResource.ListClusters(context.Background())
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to list clusters")
	assert.ErrorContains(t, err, "access denied")
}

func TestListServices(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	mockClient.On("ListServices", mock.Anything, &ecs.ListServicesInput{
		Cluster:    aws.String(clusterName),
		MaxResults: aws.Int32(100),
	}).Return(&ecs.ListServicesOutput{
		ServiceArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:service/test-cluster/service1", "arn:aws:ecs:ap-northeast-1:123456789012:service/test-cluster/service2"},
	}, nil)

	// Initialize clusters first
	ecsResource.Clusters = []ECSCluster{{ClusterName: clusterName}}

	err := ecsResource.ListServices(context.Background(), clusterName)
	assert.NoError(t, err)
	assert.Len(t, ecsResource.Clusters, 1)
}

func TestListServicesPaginated(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	mockClient.On("ListServices", mock.Anything, &ecs.ListServicesInput{
		Cluster:    aws.String(clusterName),
		MaxResults: aws.Int32(100),
	}).Return(&ecs.ListServicesOutput{
		ServiceArns: []string{
			"arn:aws:ecs:ap-northeast-1:123456789012:service/test-cluster/service1",
			"arn:aws:ecs:ap-northeast-1:123456789012:service/test-cluster/service2",
		},
		NextToken: aws.String("page-2"),
	}, nil)
	mockClient.On("ListServices", mock.Anything, &ecs.ListServicesInput{
		Cluster:    aws.String(clusterName),
		MaxResults: aws.Int32(100),
		NextToken:  aws.String("page-2"),
	}).Return(&ecs.ListServicesOutput{
		ServiceArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:service/test-cluster/service3"},
	}, nil)

	// Initialize clusters first
	ecsResource.Clusters = []ECSCluster{{ClusterName: clusterName}}

	err := ecsResource.ListServices(context.Background(), clusterName)
	assert.NoError(t, err)
	assert.Len(t, ecsResource.Clusters, 1)
	assert.Len(t, ecsResource.Clusters[0].Services, 3)
	assert.Equal(t, "service1", ecsResource.Clusters[0].Services[0].ServiceName)
	assert.Equal(t, "service3", ecsResource.Clusters[0].Services[2].ServiceName)
}

func TestListServicesListError(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	mockClient.On("ListServices", mock.Anything, &ecs.ListServicesInput{
		Cluster:    aws.String(clusterName),
		MaxResults: aws.Int32(100),
	}).Return((*ecs.ListServicesOutput)(nil), errors.New("access denied"))

	err := ecsResource.ListServices(context.Background(), clusterName)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to list services")
	assert.ErrorContains(t, err, "access denied")
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
		MaxResults:  aws.Int32(100),
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

func TestGetTasksPaginated(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	serviceName := "test-service"
	taskArn1 := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-1"
	taskArn2 := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-2"
	taskDefinitionArn := "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test-task:1"

	mockClient.On("ListTasks", mock.Anything, &ecs.ListTasksInput{
		Cluster:     aws.String(clusterName),
		ServiceName: aws.String(serviceName),
		MaxResults:  aws.Int32(100),
	}).Return(&ecs.ListTasksOutput{
		TaskArns:  []string{taskArn1},
		NextToken: aws.String("page-2"),
	}, nil)
	mockClient.On("ListTasks", mock.Anything, &ecs.ListTasksInput{
		Cluster:     aws.String(clusterName),
		ServiceName: aws.String(serviceName),
		MaxResults:  aws.Int32(100),
		NextToken:   aws.String("page-2"),
	}).Return(&ecs.ListTasksOutput{
		TaskArns: []string{taskArn2},
	}, nil)

	taskArns := []string{taskArn1, taskArn2}
	for _, taskArn := range taskArns {
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
	}

	err := ecsResource.GetTasks(context.Background(), clusterName, serviceName)
	assert.NoError(t, err)
}

func TestGetTasksListError(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	serviceName := "test-service"
	mockClient.On("ListTasks", mock.Anything, &ecs.ListTasksInput{
		Cluster:     aws.String(clusterName),
		ServiceName: aws.String(serviceName),
		MaxResults:  aws.Int32(100),
	}).Return((*ecs.ListTasksOutput)(nil), errors.New("access denied"))

	err := ecsResource.GetTasks(context.Background(), clusterName, serviceName)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to list tasks")
	assert.ErrorContains(t, err, "access denied")
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
	// putPluginOnPath lets the ExecuteCommand preflight pass so the tests
	// exercise the input validation that follows it.
	putPluginOnPath(t)
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

// FakeExecRunner records the last command instead of running it, so tests
// can inspect the session-manager-plugin invocation without executing
// anything.
type FakeExecRunner struct {
	cmd *exec.Cmd
	err error
}

func (f *FakeExecRunner) RunCommand(cmd *exec.Cmd) error {
	f.cmd = cmd
	return f.err
}

// putPluginOnPath creates a directory containing an executable named
// session-manager-plugin and returns its absolute path (already set as
// PATH by the caller via t.Setenv). The file is never executed.
func putPluginOnPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	plugin := filepath.Join(dir, sessionManagerPluginName)
	if err := os.WriteFile(plugin, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatalf("failed to create fake plugin: %v", err)
	}
	if err := os.Chmod(plugin, 0o755); err != nil {
		t.Fatalf("failed to make fake plugin executable: %v", err)
	}
	t.Setenv("PATH", dir)
}

func TestExecuteCommandWithoutPluginReturnsInstallDocsError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")

	err := ecsResource.ExecuteCommand(ecs.ExecuteCommandInput{
		Cluster:   aws.String("test-cluster"),
		Task:      aws.String("test-task"),
		Container: aws.String("test-container"),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), sessionManagerPluginName)
	assert.Contains(t, err.Error(), ssmPluginInstallDocsURL)
	// The session must not be created when the plugin is missing.
	mockClient.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything)
}

func TestExecuteCommandRunsPluginWithResolvedEndpoint(t *testing.T) {
	putPluginOnPath(t)
	mockClient := new(MockECSClient)
	ecsResource := newECSForTesting(mockClient, "ap-northeast-1")
	runner := &FakeExecRunner{}
	ecsResource.execRunner = runner

	session := &types.Session{
		SessionId:  aws.String("session-id"),
		StreamUrl:  aws.String("https://stream.example"),
		TokenValue: aws.String("secret-token"),
	}
	cluster, task, container := "test-cluster", "test-task", "test-container"
	mockClient.On("ExecuteCommand", mock.Anything, mock.Anything).Return(&ecs.ExecuteCommandOutput{Session: session}, nil)

	err := ecsResource.ExecuteCommand(ecs.ExecuteCommandInput{
		Cluster:   aws.String(cluster),
		Command:   aws.String("sh"),
		Task:      aws.String(task),
		Container: aws.String(container),
	})

	assert.NoError(t, err)
	mockClient.AssertCalled(t, "ExecuteCommand", mock.Anything, mock.Anything)

	cmd := runner.cmd
	if assert.NotNil(t, cmd) && assert.Len(t, cmd.Args, 7) {
		// Never assert or print the session JSON (Args[1]): it carries
		// credentials.
		assert.Equal(t, sessionManagerPluginName, cmd.Args[0])
		assert.Equal(t, "ap-northeast-1", cmd.Args[2])
		assert.Equal(t, "StartSession", cmd.Args[3])
		assert.Equal(t, "", cmd.Args[4])
		assert.Equal(t, `{"Target":"ecs:test-cluster_test-task_test-container"}`, cmd.Args[5])
		assert.Equal(t, "https://ssm.ap-northeast-1.amazonaws.com", cmd.Args[6])
	}
}

func TestResolveSSMEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		region   string
		ssm      string
		global   string
		expected string
	}{
		{
			name:     "commercial region derives amazonaws.com",
			region:   "ap-northeast-1",
			expected: "https://ssm.ap-northeast-1.amazonaws.com",
		},
		{
			name:     "china region derives amazonaws.com.cn",
			region:   "cn-north-1",
			expected: "https://ssm.cn-north-1.amazonaws.com.cn",
		},
		{
			name:     "china northwest region derives amazonaws.com.cn",
			region:   "cn-northwest-1",
			expected: "https://ssm.cn-northwest-1.amazonaws.com.cn",
		},
		{
			name:     "service specific override wins",
			region:   "us-east-1",
			ssm:      "https://ssm-proxy.corp.internal",
			expected: "https://ssm-proxy.corp.internal",
		},
		{
			name:     "service specific override wins over global",
			region:   "us-east-1",
			ssm:      "https://ssm-proxy.corp.internal",
			global:   "https://dev.example",
			expected: "https://ssm-proxy.corp.internal",
		},
		{
			name:     "global override used as fallback",
			region:   "us-east-1",
			global:   "https://dev.example",
			expected: "https://dev.example",
		},
		{
			name:     "empty override values fall back to derivation",
			region:   "cn-north-1",
			ssm:      "",
			expected: "https://ssm.cn-north-1.amazonaws.com.cn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(ssmEndpointURLEnvName, tt.ssm)
			t.Setenv(globalEndpointURLEnvName, tt.global)

			assert.Equal(t, tt.expected, resolveSSMEndpoint(tt.region))
		})
	}
}

func TestSSMHostSuffix(t *testing.T) {
	assert.Equal(t, "amazonaws.com", ssmHostSuffix("us-east-1"))
	assert.Equal(t, "amazonaws.com", ssmHostSuffix("us-gov-west-1"))
	assert.Equal(t, "amazonaws.com", ssmHostSuffix("eu-west-1"))
	assert.Equal(t, "amazonaws.com.cn", ssmHostSuffix("cn-north-1"))
	assert.Equal(t, "amazonaws.com.cn", ssmHostSuffix("cn-northwest-1"))
	assert.Equal(t, "amazonaws.com", ssmHostSuffix(""))
}
