package ecs

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// benchClient is a minimal ECSClient stub that returns canned responses.
// testify's mock.Mock resolves expectations through reflection on every
// call, which would dominate the measurements, so benchmarks use this
// hand-written stub instead.
type benchClient struct {
	clusterArns    []string
	serviceArns    []string
	taskArns       []string
	taskDefArn     string
	containerCount int
}

func (b *benchClient) ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return &ecs.ListClustersOutput{ClusterArns: b.clusterArns}, nil
}

func (b *benchClient) ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	return &ecs.ListServicesOutput{ServiceArns: b.serviceArns}, nil
}

func (b *benchClient) ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	return &ecs.ListTasksOutput{TaskArns: b.taskArns}, nil
}

func (b *benchClient) DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	tasks := make([]types.Task, len(params.Tasks))
	for i, arn := range params.Tasks {
		tasks[i] = types.Task{
			TaskArn:           aws.String(arn),
			TaskDefinitionArn: aws.String(b.taskDefArn),
			LastStatus:        aws.String("RUNNING"),
			DesiredStatus:     aws.String("RUNNING"),
		}
	}
	return &ecs.DescribeTasksOutput{Tasks: tasks}, nil
}

func (b *benchClient) DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	containers := make([]types.ContainerDefinition, b.containerCount)
	for i := range containers {
		containers[i] = types.ContainerDefinition{
			Name:  aws.String(fmt.Sprintf("container-%d", i)),
			Image: aws.String("nginx:latest"),
		}
	}
	return &ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &types.TaskDefinition{ContainerDefinitions: containers},
	}, nil
}

func (b *benchClient) ExecuteCommand(ctx context.Context, params *ecs.ExecuteCommandInput, optFns ...func(*ecs.Options)) (*ecs.ExecuteCommandOutput, error) {
	return &ecs.ExecuteCommandOutput{}, nil
}

// Benchmark workload shape: clusters x services x tasks per cluster and
// containers per task definition.
const (
	benchClusters   = 3
	benchServices   = 5
	benchTasks      = 10
	benchContainers = 2
	benchRegion     = "ap-northeast-1"
	benchAccountID  = "123456789012"
)

func newBenchClient() *benchClient {
	clusterArns := make([]string, benchClusters)
	for i := range clusterArns {
		clusterArns[i] = fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/cluster-%d", benchRegion, benchAccountID, i)
	}
	serviceArns := make([]string, benchServices)
	for i := range serviceArns {
		serviceArns[i] = fmt.Sprintf("arn:aws:ecs:%s:%s:service/cluster-0/service-%d", benchRegion, benchAccountID, i)
	}
	taskArns := make([]string, benchTasks)
	for i := range taskArns {
		taskArns[i] = fmt.Sprintf("arn:aws:ecs:%s:%s:task/cluster-0/task-%d", benchRegion, benchAccountID, i)
	}
	return &benchClient{
		clusterArns:    clusterArns,
		serviceArns:    serviceArns,
		taskArns:       taskArns,
		taskDefArn:     fmt.Sprintf("arn:aws:ecs:%s:%s:task-definition/task-def-0:1", benchRegion, benchAccountID),
		containerCount: benchContainers,
	}
}

func BenchmarkParseClusterARNs(b *testing.B) {
	arns := make([]string, 100)
	for i := range arns {
		arns[i] = fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/cluster-%d", benchRegion, benchAccountID, i)
	}
	e := &ECSResource{}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := e.parseClusterARNs(arns); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseServiceARNs(b *testing.B) {
	arns := make([]string, 100)
	for i := range arns {
		arns[i] = fmt.Sprintf("arn:aws:ecs:%s:%s:service/cluster-0/service-%d", benchRegion, benchAccountID, i)
	}
	e := &ECSResource{}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := e.parseServiceARNs(arns, "cluster-0"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseContainerDefinitions(b *testing.B) {
	definitions := make([]types.ContainerDefinition, 10)
	for i := range definitions {
		definitions[i] = types.ContainerDefinition{
			Name:  aws.String(fmt.Sprintf("container-%d", i)),
			Image: aws.String("nginx:latest"),
		}
	}
	e := &ECSResource{}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := e.parseContainerDefinitions(definitions, ""); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetClusterResources(b *testing.B) {
	ctx := context.Background()
	e := newECSForTesting(newBenchClient(), benchRegion)
	if err := e.ListClusters(ctx); err != nil {
		b.Fatal(err)
	}
	clusters := e.Clusters
	b.ReportAllocs()
	for b.Loop() {
		for _, cluster := range clusters {
			if _, err := e.GetClusterResources(ctx, cluster); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkLoadAllResources(b *testing.B) {
	ctx := context.Background()
	client := newBenchClient()
	b.ReportAllocs()
	for b.Loop() {
		e := newECSForTesting(client, benchRegion)
		if err := e.LoadAllResources(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
