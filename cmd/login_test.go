package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockConfigLoader struct {
	mock.Mock
}

func (m *mockConfigLoader) LoadDefaultConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	args := m.Called(ctx, optFns)
	if cfg, ok := args.Get(0).(aws.Config); ok {
		return cfg, args.Error(1)
	}
	return aws.Config{}, args.Error(1)
}

type mockECSClient struct {
	mock.Mock
}

func (m *mockECSClient) DescribeClusters(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.DescribeClustersOutput), args.Error(1)
}

func (m *mockECSClient) ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.ListClustersOutput), args.Error(1)
}

func (m *mockECSClient) ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.ListServicesOutput), args.Error(1)
}

func (m *mockECSClient) DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.DescribeServicesOutput), args.Error(1)
}

func (m *mockECSClient) ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.ListTasksOutput), args.Error(1)
}

func (m *mockECSClient) DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.DescribeTasksOutput), args.Error(1)
}

func (m *mockECSClient) DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.DescribeTaskDefinitionOutput), args.Error(1)
}

func (m *mockECSClient) ExecuteCommand(ctx context.Context, params *ecs.ExecuteCommandInput, optFns ...func(*ecs.Options)) (*ecs.ExecuteCommandOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.ExecuteCommandOutput), args.Error(1)
}

func TestInitializeECSClient(t *testing.T) {
	originalLoader := defaultLoader
	defer func() {
		defaultLoader = originalLoader
	}()

	t.Run("without AWS credentials", func(t *testing.T) {
		mockLoader := new(mockConfigLoader)
		defaultLoader = mockLoader

		mockLoader.On("LoadDefaultConfig", mock.Anything, mock.Anything).Return(aws.Config{}, errors.New("missing credentials"))

		loginSetFlags.region = "us-west-2"
		ctx := context.Background()

		client, err := initializeECSClient(ctx)
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "unable to load SDK config")

		mockLoader.AssertExpectations(t)
	})
}

func TestCreateExecuteCommandInput(t *testing.T) {
	// Setup test data
	loginSetFlags.shell = "sh"
	resource := myecs.ECSResource{
		Clusters: []myecs.Cluster{
			{ClusterName: "test-cluster"},
		},
		Services: []myecs.Service{
			{ServiceName: "test-service"},
		},
		Tasks: []myecs.Task{
			{TaskArn: "arn:aws:ecs:region:account:task/test-task"},
		},
		Containers: []myecs.Container{
			{ContainerName: "test-container"},
		},
	}

	// Execute test
	result := createExecuteCommandInput(resource)

	// Verify results
	assert.Equal(t, "test-cluster", *result.Cluster)
	assert.Equal(t, "test-container", *result.Container)
	assert.Equal(t, "arn:aws:ecs:region:account:task/test-task", *result.Task)
	assert.Equal(t, "sh", *result.Command)
}

func TestCollectECSResources(t *testing.T) {
	mockClient := new(mockECSClient)
	ecsResource := myecs.NewEcsWithClient(mockClient, "ap-northeast-1")

	// ListClustersのモック
	clusterName := "test-cluster"
	mockClient.On("ListClusters", mock.Anything, mock.Anything).Return(&ecs.ListClustersOutput{
		ClusterArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:cluster/" + clusterName},
	}, nil)

	// ListServicesのモック
	serviceName := "test-service"
	mockClient.On("ListServices", mock.Anything, &ecs.ListServicesInput{
		Cluster: aws.String(clusterName),
	}).Return(&ecs.ListServicesOutput{
		ServiceArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:service/" + clusterName + "/" + serviceName},
	}, nil)

	// ListTasksのモック
	taskArn := "arn:aws:ecs:ap-northeast-1:123456789012:task/" + clusterName + "/task-id"
	taskDefinitionArn := "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test-task:1"
	mockClient.On("ListTasks", mock.Anything, &ecs.ListTasksInput{
		Cluster:     aws.String(clusterName),
		ServiceName: aws.String(serviceName),
	}).Return(&ecs.ListTasksOutput{
		TaskArns: []string{taskArn},
	}, nil)

	// DescribeTasksのモック
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

	// DescribeTaskDefinitionのモック
	mockClient.On("DescribeTaskDefinition", mock.Anything, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinitionArn),
	}).Return(&ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &types.TaskDefinition{
			Family: aws.String("test-task"),
		},
	}, nil)

	// リソースの収集
	err := ecsResource.CollectECSResources(context.Background())
	assert.NoError(t, err)

	// 結果の検証
	assert.Len(t, ecsResource.Clusters, 1)
	assert.Equal(t, clusterName, ecsResource.Clusters[0].ClusterName)

	assert.Len(t, ecsResource.Services, 1)
	assert.Equal(t, serviceName, ecsResource.Services[0].ServiceName)

	assert.Len(t, ecsResource.Tasks, 1)
	assert.Equal(t, taskArn, ecsResource.Tasks[0].TaskArn)
	assert.Equal(t, "test-task", ecsResource.Tasks[0].TaskDefinition)

	// すべてのモックが期待通り呼び出されたことを確認
	mockClient.AssertExpectations(t)
}
