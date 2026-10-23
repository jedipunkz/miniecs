package ecs

import (
	"context"
	"errors"
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
