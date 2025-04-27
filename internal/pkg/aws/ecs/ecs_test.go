package ecs

import (
	"context"
	"os/exec"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

type MockCommandRunner struct {
	mock.Mock
}

func (m *MockCommandRunner) Run(cmd *exec.Cmd) error {
	args := m.Called(cmd)
	return args.Error(0)
}

func TestNewEcs(t *testing.T) {
	cfg := aws.Config{}
	region := "ap-northeast-1"

	ecsResource := NewEcs(cfg, region)

	assert.NotNil(t, ecsResource)
	assert.Equal(t, region, ecsResource.Region)
	assert.Empty(t, ecsResource.Clusters)
	assert.Empty(t, ecsResource.Services)
	assert.Empty(t, ecsResource.Tasks)
	assert.Empty(t, ecsResource.Containers)
}

func TestListClusters(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := &ECSResource{
		client: mockClient,
		Region: "ap-northeast-1",
	}

	expectedClusters := []string{"cluster1", "cluster2"}
	mockClient.On("ListClusters", mock.Anything, mock.Anything).Return(&ecs.ListClustersOutput{
		ClusterArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:cluster/cluster1", "arn:aws:ecs:ap-northeast-1:123456789012:cluster/cluster2"},
	}, nil)

	err := ecsResource.ListClusters(context.Background())
	assert.NoError(t, err)
	assert.Len(t, ecsResource.Clusters, 2)
	assert.Equal(t, expectedClusters[0], ecsResource.Clusters[0].ClusterName)
	assert.Equal(t, expectedClusters[1], ecsResource.Clusters[1].ClusterName)
}

func TestListServices(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := &ECSResource{
		client: mockClient,
		Region: "ap-northeast-1",
	}

	clusterName := "test-cluster"
	expectedServices := []string{"service1", "service2"}
	mockClient.On("ListServices", mock.Anything, &ecs.ListServicesInput{
		Cluster: aws.String(clusterName),
	}).Return(&ecs.ListServicesOutput{
		ServiceArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:service/test-cluster/service1", "arn:aws:ecs:ap-northeast-1:123456789012:service/test-cluster/service2"},
	}, nil)

	err := ecsResource.ListServices(context.Background(), clusterName)
	assert.NoError(t, err)
	assert.Len(t, ecsResource.Services, 2)
	assert.Equal(t, expectedServices[0], ecsResource.Services[0].ServiceName)
	assert.Equal(t, expectedServices[1], ecsResource.Services[1].ServiceName)
}

func TestGetTasks(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := &ECSResource{
		client: mockClient,
		Region: "ap-northeast-1",
	}

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

	mockClient.On("DescribeTaskDefinition", mock.Anything, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinitionArn),
	}).Return(&ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &types.TaskDefinition{
			Family: aws.String("test-task"),
		},
	}, nil)

	err := ecsResource.GetTasks(context.Background(), clusterName, serviceName)
	assert.NoError(t, err)
	assert.Len(t, ecsResource.Tasks, 1)
	assert.Equal(t, taskArn, ecsResource.Tasks[0].TaskArn)
	assert.Equal(t, "test-task", ecsResource.Tasks[0].TaskDefinition)
}

func TestListContainers(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := &ECSResource{
		client: mockClient,
		Region: "ap-northeast-1",
	}

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

	err := ecsResource.ListContainers(context.Background(), taskDefinition)
	assert.NoError(t, err)
	assert.Len(t, ecsResource.Containers, 1)
	assert.Equal(t, containerName, ecsResource.Containers[0].ContainerName)
}

func TestExecuteCommand(t *testing.T) {
	mockClient := new(MockECSClient)
	mockRunner := new(MockCommandRunner)
	ecsResource := &ECSResource{
		client: mockClient,
		runner: mockRunner,
		Region: "ap-northeast-1",
	}

	input := ecs.ExecuteCommandInput{
		Cluster:   aws.String("test-cluster"),
		Task:      aws.String("test-task"),
		Container: aws.String("test-container"),
		Command:   aws.String("sh"),
	}

	expectedInput := prepareExecuteCommandInput(input)
	mockClient.On("ExecuteCommand", mock.Anything, &expectedInput).Return(&ecs.ExecuteCommandOutput{
		Session: &types.Session{
			SessionId:  aws.String("test-session"),
			StreamUrl:  aws.String("test-url"),
			TokenValue: aws.String("test-token"),
		},
	}, nil)

	mockRunner.On("Run", mock.AnythingOfType("*exec.Cmd")).Return(nil)

	err := ecsResource.ExecuteCommand(input)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

func TestCollectECSResources(t *testing.T) {
	mockClient := new(MockECSClient)
	ecsResource := &ECSResource{
		client: mockClient,
		Region: "ap-northeast-1",
	}

	clusterName := "test-cluster"
	mockClient.On("ListClusters", mock.Anything, mock.Anything).Return(&ecs.ListClustersOutput{
		ClusterArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:cluster/" + clusterName},
	}, nil)

	serviceName := "test-service"
	mockClient.On("ListServices", mock.Anything, &ecs.ListServicesInput{
		Cluster: aws.String(clusterName),
	}).Return(&ecs.ListServicesOutput{
		ServiceArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:service/" + clusterName + "/" + serviceName},
	}, nil)

	taskArn := "arn:aws:ecs:ap-northeast-1:123456789012:task/" + clusterName + "/task-id"
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

	mockClient.On("DescribeTaskDefinition", mock.Anything, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinitionArn),
	}).Return(&ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &types.TaskDefinition{
			Family: aws.String("test-task"),
		},
	}, nil)

	err := ecsResource.CollectECSResources(context.Background())
	assert.NoError(t, err)

	assert.Len(t, ecsResource.Clusters, 1)
	assert.Equal(t, clusterName, ecsResource.Clusters[0].ClusterName)

	assert.Len(t, ecsResource.Services, 1)
	assert.Equal(t, serviceName, ecsResource.Services[0].ServiceName)

	assert.Len(t, ecsResource.Tasks, 1)
	assert.Equal(t, taskArn, ecsResource.Tasks[0].TaskArn)
	assert.Equal(t, "test-task", ecsResource.Tasks[0].TaskDefinition)
}
