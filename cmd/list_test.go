package cmd

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/stretchr/testify/mock"
)

type mockECSClient struct {
	mock.Mock
	listClustersOutput           *ecs.ListClustersOutput
	listServicesOutput           *ecs.ListServicesOutput
	listTasksOutput              *ecs.ListTasksOutput
	describeTasksOutput          *ecs.DescribeTasksOutput
	describeTaskDefinitionOutput *ecs.DescribeTaskDefinitionOutput
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

func TestListECSTable(t *testing.T) {
	ctx := context.Background()
	clusterName := "test-cluster"
	serviceName := "test-service"
	taskDefinition := "test-task-definition"
	containerName := "test-container"

	mockClient := new(mockECSClient)
	ecsResource := myecs.NewEcsWithClient(mockClient, "region")

	// Setup mock expectations
	mockClient.On("ListClusters", mock.Anything, &ecs.ListClustersInput{}).Return(&ecs.ListClustersOutput{
		ClusterArns: []string{fmt.Sprintf("arn:aws:ecs:region:account:cluster/%s", clusterName)},
	}, nil)

	mockClient.On("ListServices", mock.Anything, &ecs.ListServicesInput{
		Cluster: aws.String(clusterName),
	}).Return(&ecs.ListServicesOutput{
		ServiceArns: []string{fmt.Sprintf("arn:aws:ecs:region:account:service/%s/%s", clusterName, serviceName)},
	}, nil)

	mockClient.On("ListTasks", mock.Anything, &ecs.ListTasksInput{
		Cluster:     aws.String(clusterName),
		ServiceName: aws.String(serviceName),
	}).Return(&ecs.ListTasksOutput{
		TaskArns: []string{"arn:aws:ecs:region:account:task/test-task"},
	}, nil)

	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Cluster: aws.String(clusterName),
		Tasks:   []string{"arn:aws:ecs:region:account:task/test-task"},
	}).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{
			{
				TaskDefinitionArn: aws.String(fmt.Sprintf("arn:aws:ecs:region:account:task-definition/%s:1", taskDefinition)),
				Containers: []types.Container{
					{
						Name: aws.String(containerName),
					},
				},
			},
		},
	}, nil)

	mockClient.On("DescribeTaskDefinition", mock.Anything, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(fmt.Sprintf("arn:aws:ecs:region:account:task-definition/%s:1", taskDefinition)),
	}).Return(&ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &types.TaskDefinition{
			ContainerDefinitions: []types.ContainerDefinition{
				{
					Name: aws.String(containerName),
				},
			},
		},
	}, nil)

	tests := []struct {
		name           string
		cluster        string
		expectedOutput [][]string
	}{
		{
			name:    "list all clusters",
			cluster: "",
			expectedOutput: [][]string{
				{
					clusterName,
					serviceName,
					fmt.Sprintf("arn:aws:ecs:region:account:task-definition/%s:1", taskDefinition),
					containerName,
				},
			},
		},
		{
			name:    "list specific cluster",
			cluster: clusterName,
			expectedOutput: [][]string{
				{
					clusterName,
					serviceName,
					fmt.Sprintf("arn:aws:ecs:region:account:task-definition/%s:1", taskDefinition),
					containerName,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listSetFlags.cluster = tt.cluster

			// Call ListClusters first to populate the clusters
			err := ecsResource.ListClusters(ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output, err := listECSTable(ctx, ecsResource)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(output, tt.expectedOutput) {
				t.Errorf("expected %v, got %v", tt.expectedOutput, output)
			}
		})
	}

	// Verify that all expected mock calls were made
	mockClient.AssertExpectations(t)
}
