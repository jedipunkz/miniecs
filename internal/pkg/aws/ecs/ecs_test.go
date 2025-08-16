package ecs

import (
	"context"
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