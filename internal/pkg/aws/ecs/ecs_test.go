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

func (m *MockECSClient) ExecuteCommand(ctx context.Context, params *ecs.ExecuteCommandInput, optFns ...func(*ecs.Options)) (*ecs.ExecuteCommandOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ecs.ExecuteCommandOutput), args.Error(1)
}

func TestNewClient(t *testing.T) {
	cfg := aws.Config{}
	region := "ap-northeast-1"

	client := NewClient(cfg, region)

	assert.NotNil(t, client)
	assert.Equal(t, region, client.Region)
	assert.NotNil(t, client.client)
	assert.NotNil(t, client.execRunner)
}

func TestListClusters(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

	mockClient.On("ListClusters", mock.Anything, &ecs.ListClustersInput{MaxResults: aws.Int32(100)}).Return(&ecs.ListClustersOutput{
		ClusterArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:cluster/cluster1", "arn:aws:ecs:ap-northeast-1:123456789012:cluster/cluster2"},
	}, nil)

	names, err := client.ListClusters(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, []string{"cluster1", "cluster2"}, names)
}

func TestListClustersPaginated(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

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

	names, err := client.ListClusters(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, []string{"cluster1", "cluster2", "cluster3"}, names)
}

func TestListClustersListError(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

	mockClient.On("ListClusters", mock.Anything, &ecs.ListClustersInput{MaxResults: aws.Int32(100)}).Return(
		(*ecs.ListClustersOutput)(nil), errors.New("access denied"))

	_, err := client.ListClusters(context.Background())
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to list clusters")
	assert.ErrorContains(t, err, "access denied")
}

func TestListServices(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	mockClient.On("ListServices", mock.Anything, &ecs.ListServicesInput{
		Cluster:    aws.String(clusterName),
		MaxResults: aws.Int32(100),
	}).Return(&ecs.ListServicesOutput{
		ServiceArns: []string{"arn:aws:ecs:ap-northeast-1:123456789012:service/test-cluster/service1", "arn:aws:ecs:ap-northeast-1:123456789012:service/test-cluster/service2"},
	}, nil)

	names, err := client.ListServices(context.Background(), clusterName)
	assert.NoError(t, err)
	assert.Equal(t, []string{"service1", "service2"}, names)
}

func TestListServicesPaginated(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

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

	names, err := client.ListServices(context.Background(), clusterName)
	assert.NoError(t, err)
	assert.Equal(t, []string{"service1", "service2", "service3"}, names)
}

func TestListServicesListError(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	mockClient.On("ListServices", mock.Anything, &ecs.ListServicesInput{
		Cluster:    aws.String(clusterName),
		MaxResults: aws.Int32(100),
	}).Return((*ecs.ListServicesOutput)(nil), errors.New("access denied"))

	_, err := client.ListServices(context.Background(), clusterName)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to list services")
	assert.ErrorContains(t, err, "access denied")
}

func TestListTargets(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

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
	}, nil).Once()

	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{taskArn},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{
			{
				TaskArn:           aws.String(taskArn),
				TaskDefinitionArn: aws.String(taskDefinitionArn),
				Containers: []types.Container{
					{
						Name:       aws.String("app"),
						Image:      aws.String("nginx:1"),
						LastStatus: aws.String("RUNNING"),
					},
				},
			},
		},
	}, nil).Once()

	targets, err := client.ListTargets(context.Background(), clusterName, serviceName)
	assert.NoError(t, err)
	assert.Equal(t, []Target{{
		Cluster:        clusterName,
		Service:        serviceName,
		TaskArn:        taskArn,
		TaskDefinition: taskDefinitionArn,
		Container:      "app",
		Image:          "nginx:1",
		Status:         "RUNNING",
	}}, targets)
	mockClient.AssertExpectations(t)
}

func TestListTargetsPaginated(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

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
	}, nil).Once()
	mockClient.On("ListTasks", mock.Anything, &ecs.ListTasksInput{
		Cluster:     aws.String(clusterName),
		ServiceName: aws.String(serviceName),
		MaxResults:  aws.Int32(100),
		NextToken:   aws.String("page-2"),
	}).Return(&ecs.ListTasksOutput{
		TaskArns: []string{taskArn2},
	}, nil).Once()

	newDescribedTask := func(taskArn string) types.Task {
		return types.Task{
			TaskArn:           aws.String(taskArn),
			TaskDefinitionArn: aws.String(taskDefinitionArn),
			Containers: []types.Container{
				{Name: aws.String("app"), LastStatus: aws.String("RUNNING")},
			},
		}
	}
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{taskArn1, taskArn2},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{newDescribedTask(taskArn1), newDescribedTask(taskArn2)},
	}, nil).Once()

	targets, err := client.ListTargets(context.Background(), clusterName, serviceName)
	assert.NoError(t, err)
	assert.Len(t, targets, 2)
	assert.Equal(t, taskArn1, targets[0].TaskArn)
	assert.Equal(t, taskArn2, targets[1].TaskArn)
	mockClient.AssertExpectations(t)
}

func TestListTargetsListError(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	serviceName := "test-service"
	mockClient.On("ListTasks", mock.Anything, &ecs.ListTasksInput{
		Cluster:     aws.String(clusterName),
		ServiceName: aws.String(serviceName),
		MaxResults:  aws.Int32(100),
	}).Return((*ecs.ListTasksOutput)(nil), errors.New("access denied"))

	_, err := client.ListTargets(context.Background(), clusterName, serviceName)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to list tasks")
	assert.ErrorContains(t, err, "access denied")
}

func TestParseTaskContainers(t *testing.T) {
	containers, err := parseTaskContainers([]types.Container{
		{
			Name:       aws.String("app"),
			Image:      aws.String("nginx:1"),
			LastStatus: aws.String("RUNNING"),
		},
		{
			// Optional fields may be missing while the task starts up.
			Name: aws.String("sidecar"),
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, []containerInfo{
		{Name: "app", Image: "nginx:1", LastStatus: "RUNNING"},
		{Name: "sidecar"},
	}, containers)
}

func TestParseTaskContainersRejectsNilName(t *testing.T) {
	_, err := parseTaskContainers([]types.Container{{Image: aws.String("nginx:1")}})
	assert.ErrorContains(t, err, "container name is nil")
}

func TestDescribeTasksAggregatesFailures(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	okArn := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-ok"
	badArn1 := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-bad-1"
	badArn2 := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-bad-2"
	taskDefArn := "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test-task:1"

	notFoundReason := "task not found"

	// One batch carries all requested ARNs. badArn1 is reported in
	// Failures, badArn2 is absent from both Tasks and Failures.
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{okArn, badArn1, badArn2},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{{
			TaskArn:           aws.String(okArn),
			TaskDefinitionArn: aws.String(taskDefArn),
			Containers:        []types.Container{{Name: aws.String("app")}},
		}},
		Failures: []types.Failure{{
			Arn:    aws.String(badArn1),
			Reason: aws.String(notFoundReason),
		}},
	}, nil).Once()

	tasks, err := client.describeTasks(context.Background(), clusterName, []string{okArn, badArn1, badArn2})
	assert.Len(t, tasks, 1)
	assert.Equal(t, okArn, tasks[0].TaskArn)

	assert.Error(t, err)
	assert.ErrorContains(t, err, badArn1)
	assert.ErrorContains(t, err, notFoundReason)
	assert.ErrorContains(t, err, badArn2)
	assert.ErrorContains(t, err, "task not found: "+badArn2)
}

func TestDescribeTasksNoFailures(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

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
			Containers:        []types.Container{{Name: aws.String("app")}},
		}},
	}, nil).Once()

	tasks, err := client.describeTasks(context.Background(), clusterName, []string{okArn})
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	mockClient.AssertExpectations(t)
}

// newTaskArn builds the ARN of task number i of a cluster, so batching
// tests can generate any task count.
func newTaskArn(cluster string, i int) string {
	return fmt.Sprintf("arn:aws:ecs:ap-northeast-1:123456789012:task/%s/task-%03d", cluster, i)
}

func TestDescribeTasksBatchesOverBatchSize(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	arns := make([]string, 150)
	for i := range arns {
		arns[i] = newTaskArn(clusterName, i)
	}

	newDescribedTasks := func(bat []string) []types.Task {
		tasks := make([]types.Task, len(bat))
		for i, arn := range bat {
			tasks[i] = types.Task{
				TaskArn:           aws.String(arn),
				TaskDefinitionArn: aws.String("arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test-task:1"),
				Containers:        []types.Container{{Name: aws.String("app")}},
			}
		}
		return tasks
	}

	// 150 tasks split into one full batch of 100 and one of 50.
	firstBatch, secondBatch := arns[:100], arns[100:]
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   firstBatch,
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{Tasks: newDescribedTasks(firstBatch)}, nil).Once()
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   secondBatch,
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{Tasks: newDescribedTasks(secondBatch)}, nil).Once()

	tasks, err := client.describeTasks(context.Background(), clusterName, arns)
	assert.NoError(t, err)
	assert.Len(t, tasks, 150)
	mockClient.AssertExpectations(t)
}

func TestDescribeTasksAggregatesBatchFailures(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	arns := make([]string, 150)
	for i := range arns {
		arns[i] = newTaskArn(clusterName, i)
	}

	// The first batch fails at the API level while the second succeeds.
	// The error must be returned and the successful tasks kept.
	firstBatch, secondBatch := arns[:100], arns[100:]
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   firstBatch,
		Cluster: aws.String(clusterName),
	}).Return((*ecs.DescribeTasksOutput)(nil), fmt.Errorf("describe failed: %w", errAccessDenied)).Once()
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   secondBatch,
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{{
			TaskArn:           aws.String(secondBatch[0]),
			TaskDefinitionArn: aws.String("arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test-task:1"),
			Containers:        []types.Container{{Name: aws.String("app")}},
		}},
	}, nil).Once()

	tasks, err := client.describeTasks(context.Background(), clusterName, arns)
	assert.Len(t, tasks, 1, "tasks of the successful batch are kept")

	assert.Error(t, err)
	assert.ErrorIs(t, err, errAccessDenied)
	assert.ErrorContains(t, err, "failed to describe task batch")
}

func TestListTargetsPropagatesDescribeTaskFailure(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	serviceName := "test-service"
	taskArn := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-1"

	// ListTargets lists the tasks once and describes them in one batch.
	// The describe failure must fail the whole service listing.
	mockClient.On("ListTasks", mock.Anything, &ecs.ListTasksInput{
		Cluster:     aws.String(clusterName),
		ServiceName: aws.String(serviceName),
		MaxResults:  aws.Int32(100),
	}).Return(&ecs.ListTasksOutput{
		TaskArns: []string{taskArn},
	}, nil).Once()
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{taskArn},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{}, fmt.Errorf("describe failed: %w", errAccessDenied)).Once()

	targets, err := client.ListTargets(context.Background(), clusterName, serviceName)
	assert.Nil(t, targets)
	assert.Error(t, err)
	assert.ErrorIs(t, err, errAccessDenied)
	assert.ErrorContains(t, err, "failed to describe tasks")
	mockClient.AssertExpectations(t)
}

func TestListTargetsAggregatesContainerFailures(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	serviceName := "test-service"
	okTaskArn := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-ok"
	badTaskArn := "arn:aws:ecs:ap-northeast-1:123456789012:task/test-cluster/task-bad"
	taskDefArn := "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test-task:1"

	mockClient.On("ListTasks", mock.Anything, &ecs.ListTasksInput{
		Cluster:     aws.String(clusterName),
		ServiceName: aws.String(serviceName),
		MaxResults:  aws.Int32(100),
	}).Return(&ecs.ListTasksOutput{
		TaskArns: []string{okTaskArn, badTaskArn},
	}, nil).Once()
	// One batch describes both tasks; the container of the bad task has
	// no name, which fails that task only.
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   []string{okTaskArn, badTaskArn},
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{
			{
				TaskArn:           aws.String(okTaskArn),
				TaskDefinitionArn: aws.String(taskDefArn),
				Containers:        []types.Container{{Name: aws.String("app")}},
			},
			{
				TaskArn:           aws.String(badTaskArn),
				TaskDefinitionArn: aws.String(taskDefArn),
				Containers:        []types.Container{{Image: aws.String("nginx:1")}},
			},
		},
	}, nil).Once()

	targets, err := client.ListTargets(context.Background(), clusterName, serviceName)
	assert.Nil(t, targets, "a single failing task must fail the whole service listing, not silently drop the task")
	assert.Error(t, err)
	assert.ErrorContains(t, err, badTaskArn)
	assert.ErrorContains(t, err, "container name is nil")
	mockClient.AssertExpectations(t)
}

func TestListTargetsBatchesDescribeTasks(t *testing.T) {
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

	clusterName := "test-cluster"
	serviceName := "test-service"
	arns := make([]string, 150)
	for i := range arns {
		arns[i] = newTaskArn(clusterName, i)
	}

	newDescribedTasks := func(bat []string) []types.Task {
		tasks := make([]types.Task, len(bat))
		for i, arn := range bat {
			tasks[i] = types.Task{
				TaskArn:           aws.String(arn),
				TaskDefinitionArn: aws.String("arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test-task:1"),
				Containers:        []types.Container{{Name: aws.String("app"), LastStatus: aws.String("RUNNING")}},
			}
		}
		return tasks
	}

	// One ListTasks call, then 150 tasks split into batches of 100 and
	// 50, with no DescribeTaskDefinition calls anywhere.
	mockClient.On("ListTasks", mock.Anything, &ecs.ListTasksInput{
		Cluster:     aws.String(clusterName),
		ServiceName: aws.String(serviceName),
		MaxResults:  aws.Int32(100),
	}).Return(&ecs.ListTasksOutput{TaskArns: arns}, nil).Once()
	firstBatch, secondBatch := arns[:100], arns[100:]
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   firstBatch,
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{Tasks: newDescribedTasks(firstBatch)}, nil).Once()
	mockClient.On("DescribeTasks", mock.Anything, &ecs.DescribeTasksInput{
		Tasks:   secondBatch,
		Cluster: aws.String(clusterName),
	}).Return(&ecs.DescribeTasksOutput{Tasks: newDescribedTasks(secondBatch)}, nil).Once()

	targets, err := client.ListTargets(context.Background(), clusterName, serviceName)
	assert.NoError(t, err)
	assert.Len(t, targets, 150)
	assert.Equal(t, newTaskArn(clusterName, 149), targets[149].TaskArn)
	assert.Equal(t, "RUNNING", targets[149].Status)
	mockClient.AssertExpectations(t)
}

func TestExecuteCommandRequiresFields(t *testing.T) {
	// putPluginOnPath lets the ExecuteCommand preflight pass so the tests
	// exercise the input validation that follows it.
	putPluginOnPath(t)
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")

	full := Target{
		Cluster:   "test-cluster",
		TaskArn:   "test-task",
		Container: "test-container",
	}
	cases := []struct {
		name    string
		target  Target
		shell   string
		wantMsg string
	}{
		{
			name:    "empty cluster",
			target:  Target{TaskArn: "test-task", Container: "test-container"},
			shell:   "sh",
			wantMsg: "cluster is required",
		},
		{
			name:    "empty task",
			target:  Target{Cluster: "test-cluster", Container: "test-container"},
			shell:   "sh",
			wantMsg: "task is required",
		},
		{
			name:    "empty container",
			target:  Target{Cluster: "test-cluster", TaskArn: "test-task"},
			shell:   "sh",
			wantMsg: "container is required",
		},
		{
			name:    "empty shell",
			target:  full,
			shell:   "",
			wantMsg: "shell is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := client.ExecuteCommand(context.Background(), tc.target, tc.shell)
			assert.ErrorContains(t, err, tc.wantMsg)
		})
	}

	mockClient.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything)
}

func TestExecuteCommandClientNotInitialized(t *testing.T) {
	client := &Client{}

	err := client.ExecuteCommand(context.Background(), Target{Cluster: "c"}, "sh")
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
	client := newClientForTesting(mockClient, "ap-northeast-1")

	err := client.ExecuteCommand(context.Background(), Target{
		Cluster:   "test-cluster",
		TaskArn:   "test-task",
		Container: "test-container",
	}, "sh")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), sessionManagerPluginName)
	assert.Contains(t, err.Error(), ssmPluginInstallDocsURL)
	// The session must not be created when the plugin is missing.
	mockClient.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything)
}

func TestExecuteCommandRunsPluginWithResolvedEndpoint(t *testing.T) {
	putPluginOnPath(t)
	mockClient := new(MockECSClient)
	client := newClientForTesting(mockClient, "ap-northeast-1")
	runner := &FakeExecRunner{}
	client.execRunner = runner

	session := &types.Session{
		SessionId:  aws.String("session-id"),
		StreamUrl:  aws.String("https://stream.example"),
		TokenValue: aws.String("secret-token"),
	}
	cluster, task, container := "test-cluster", "test-task", "test-container"
	mockClient.On("ExecuteCommand", mock.Anything, mock.Anything).Return(&ecs.ExecuteCommandOutput{Session: session}, nil)

	err := client.ExecuteCommand(context.Background(), Target{
		Cluster:   cluster,
		TaskArn:   task,
		Container: container,
	}, "sh")

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
