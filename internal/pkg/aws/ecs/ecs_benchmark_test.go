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
		containers := make([]types.Container, b.containerCount)
		for j := range containers {
			containers[j] = types.Container{
				Name:       aws.String(fmt.Sprintf("container-%d", j)),
				Image:      aws.String("nginx:latest"),
				LastStatus: aws.String("RUNNING"),
			}
		}
		tasks[i] = types.Task{
			TaskArn:           aws.String(arn),
			TaskDefinitionArn: aws.String(b.taskDefArn),
			LastStatus:        aws.String("RUNNING"),
			DesiredStatus:     aws.String("RUNNING"),
			Containers:        containers,
		}
	}
	return &ecs.DescribeTasksOutput{Tasks: tasks}, nil
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
	b.ReportAllocs()
	for b.Loop() {
		if _, err := parseClusterARNs(arns); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseServiceARNs(b *testing.B) {
	arns := make([]string, 100)
	for i := range arns {
		arns[i] = fmt.Sprintf("arn:aws:ecs:%s:%s:service/cluster-0/service-%d", benchRegion, benchAccountID, i)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := parseServiceARNs(arns); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseTaskContainers(b *testing.B) {
	containers := make([]types.Container, 10)
	for i := range containers {
		containers[i] = types.Container{
			Name:       aws.String(fmt.Sprintf("container-%d", i)),
			Image:      aws.String("nginx:latest"),
			LastStatus: aws.String("RUNNING"),
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := parseTaskContainers(containers); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkListTargets measures the full resource-loading path: from the
// listed services of every cluster down to the flat per-container
// targets.
func BenchmarkListTargets(b *testing.B) {
	ctx := context.Background()
	client := newClientForTesting(newBenchClient(), benchRegion)
	clusters, err := client.ListClusters(ctx)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		for _, cluster := range clusters {
			services, err := client.ListServices(ctx, cluster)
			if err != nil {
				b.Fatal(err)
			}
			for _, service := range services {
				if _, err := client.ListTargets(ctx, cluster, service); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
}
