package cmd

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"

	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
)

func TestListECSTable(t *testing.T) {
	listSetFlags.cluster = ""
	cfg := aws.Config{}
	ecsResource := myecs.NewECS(cfg, "region")

	// No cluster flag and no loaded clusters means no rows and no error.
	ctx := context.Background()
	rows, err := listECSTable(ctx, ecsResource)
	assert.NoError(t, err)
	assert.Empty(t, rows)
}

func TestBuildECSRows(t *testing.T) {
	taskDefinitionArn := "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/api:42"
	resourceForTask := func(clusterName, serviceName, taskDefinition string, containerNames ...string) myecs.ECSResource {
		containers := make([]myecs.ECSContainer, 0, len(containerNames))
		for _, name := range containerNames {
			containers = append(containers, myecs.ECSContainer{ContainerName: name})
		}
		return myecs.ECSResource{
			Clusters: []myecs.ECSCluster{{
				ClusterName: clusterName,
				Services: []myecs.ECSService{{
					ServiceName: serviceName,
					Tasks: []myecs.ECSTask{{
						TaskDefinition: taskDefinition,
						Containers:     containers,
					}},
				}},
			}},
		}
	}

	tests := []struct {
		name      string
		resources []myecs.ECSResource
		wantRows  [][]string
	}{
		{
			name: "full row per task with hashed task definition",
			resources: []myecs.ECSResource{
				resourceForTask("prod", "api", taskDefinitionArn, "app"),
			},
			wantRows: [][]string{
				{"prod", "api", "api:42", "app"},
			},
		},
		{
			name: "multi-container task joined into one row",
			resources: []myecs.ECSResource{
				resourceForTask("prod", "web", taskDefinitionArn, "app", "sidecar"),
			},
			wantRows: [][]string{
				{"prod", "web", "api:42", "app, sidecar"},
			},
		},
		{
			name: "one row per loaded resource",
			resources: []myecs.ECSResource{
				resourceForTask("prod", "api", taskDefinitionArn, "app"),
				resourceForTask("dev", "web", "arn:aws:ecs:us-east-1:123456789012:task-definition/web:1", "nginx"),
			},
			wantRows: [][]string{
				{"prod", "api", "api:42", "app"},
				{"dev", "web", "web:1", "nginx"},
			},
		},
		{
			name: "resource without cluster is skipped",
			resources: []myecs.ECSResource{
				{},
			},
			wantRows: nil,
		},
		{
			name: "resource without service or task is skipped",
			resources: []myecs.ECSResource{
				{
					Clusters: []myecs.ECSCluster{
						{},
						{ClusterName: "prod"},
					},
				},
			},
			wantRows: nil,
		},
		{
			name:     "no resources yields no rows",
			wantRows: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantRows, buildECSRows(tt.resources))
		})
	}
}

func TestShortenTaskDefArn(t *testing.T) {
	tests := []struct {
		name           string
		taskDefinition string
		want           string
	}{
		{
			name:           "full ARN becomes family:revision",
			taskDefinition: "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/api:42",
			want:           "api:42",
		},
		{
			name:           "family with slash path keeps the path",
			taskDefinition: "arn:aws:ecs:us-east-1:123456789012:task-definition/teamA/web:1",
			want:           "teamA/web:1",
		},
		{
			name:           "family without revision keeps family only",
			taskDefinition: "arn:aws:ecs:us-east-1:123456789012:task-definition/web",
			want:           "web",
		},
		{
			name:           "already abbreviated family:revision passes through",
			taskDefinition: "web:1",
			want:           "web:1",
		},
		{
			name:           "marker without family:revision passes through",
			taskDefinition: "arn:aws:ecs:us-east-1:123456789012:task-definition/",
			want:           "arn:aws:ecs:us-east-1:123456789012:task-definition/",
		},
		{
			name:           "empty string passes through",
			taskDefinition: "",
			want:           "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shortenTaskDefArn(tt.taskDefinition))
		})
	}
}
