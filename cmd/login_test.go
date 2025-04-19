package cmd

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// モックECSクライアントの定義
type mockECSClient struct {
	mock.Mock
}

func (m *mockECSClient) ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.ListClustersOutput), args.Error(1)
}

func (m *mockECSClient) ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.ListServicesOutput), args.Error(1)
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
	t.Run("without AWS_PROFILE", func(t *testing.T) {
		os.Unsetenv("AWS_PROFILE")
		loginSetFlags.region = "ap-northeast-1"

		client, err := initializeECSClient(context.Background())
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "set AWS_PROFILE environment variable")
	})
}

func TestCollectECSResources(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockECSClient{}

	// モックの応答を設定
	mockClient.On("ListClusters", ctx, &ecs.ListClustersInput{}).Return(
		&ecs.ListClustersOutput{
			ClusterArns: []string{"arn:aws:ecs:region:account:cluster/cluster1"},
		}, nil)

	mockClient.On("ListServices", ctx, &ecs.ListServicesInput{
		Cluster: aws.String("cluster1"),
	}).Return(&ecs.ListServicesOutput{
		ServiceArns: []string{"arn:aws:ecs:region:account:service/service1"},
	}, nil)

	mockClient.On("ListTasks", ctx, &ecs.ListTasksInput{
		Cluster:     aws.String("cluster1"),
		ServiceName: aws.String("service1"),
	}).Return(&ecs.ListTasksOutput{
		TaskArns: []string{"arn:aws:ecs:region:account:task/task1"},
	}, nil)

	mockClient.On("DescribeTasks", ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String("cluster1"),
		Tasks:   []string{"arn:aws:ecs:region:account:task/task1"},
	}).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{
			{
				TaskArn:            aws.String("arn:aws:ecs:region:account:task/task1"),
				TaskDefinitionArn:  aws.String("task-def1"),
			},
		},
	}, nil)

	mockClient.On("DescribeTaskDefinition", ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String("task-def1"),
	}).Return(&ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &types.TaskDefinition{
			ContainerDefinitions: []types.ContainerDefinition{
				{
					Name: aws.String("container1"),
				},
			},
		},
	}, nil)

	ecsResource := myecs.NewEcsWithClient(mockClient, "us-west-2")

	resources, err := collectECSResources(ctx, ecsResource)
	assert.NoError(t, err)
	assert.NotEmpty(t, resources)
}

func TestCreateExecuteCommandInput(t *testing.T) {
	tests := []struct {
		name     string
		resource myecs.ECSResource
		shell    string
		want     ecs.ExecuteCommandInput
	}{
		{
			name: "with default shell",
			resource: myecs.ECSResource{
				Clusters:   []myecs.Cluster{{ClusterName: "test-cluster"}},
				Containers: []myecs.Container{{ContainerName: "test-container"}},
				Tasks:      []myecs.Task{{TaskArn: "test-task"}},
			},
			shell: "",
			want: ecs.ExecuteCommandInput{
				Cluster:   aws.String("test-cluster"),
				Container: aws.String("test-container"),
				Task:      aws.String("test-task"),
				Command:   aws.String("sh"),
			},
		},
		{
			name: "with custom shell",
			resource: myecs.ECSResource{
				Clusters:   []myecs.Cluster{{ClusterName: "test-cluster"}},
				Containers: []myecs.Container{{ContainerName: "test-container"}},
				Tasks:      []myecs.Task{{TaskArn: "test-task"}},
			},
			shell: "bash",
			want: ecs.ExecuteCommandInput{
				Cluster:   aws.String("test-cluster"),
				Container: aws.String("test-container"),
				Task:      aws.String("test-task"),
				Command:   aws.String("bash"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loginSetFlags.shell = tt.shell
			got := createExecuteCommandInput(tt.resource)
			assert.Equal(t, *tt.want.Cluster, *got.Cluster)
			assert.Equal(t, *tt.want.Container, *got.Container)
			assert.Equal(t, *tt.want.Task, *got.Task)
			assert.Equal(t, *tt.want.Command, *got.Command)
		})
	}
}

// ヘルパー関数
func stringPtr(s string) *string {
	return &s
} 