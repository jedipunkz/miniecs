package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
)

func TestListECSTable(t *testing.T) {
	tests := []struct {
		name        string
		clusterFlag string
		client      *fakeTargetLister
		wantRows    [][]string
		wantErr     string
	}{
		{
			name:        "no cluster flag and no loaded clusters means no rows and no error",
			clusterFlag: "",
			client:      &fakeTargetLister{},
			wantRows:    nil,
		},
		{
			name:        "no cluster flag fetches every cluster",
			clusterFlag: "",
			client: &fakeTargetLister{
				clusters: []string{"c1", "c2"},
				services: map[string][]string{"c1": {"s1"}, "c2": {"s2"}},
				targets: map[string][]myecs.Target{
					"c1/s1": {{Cluster: "c1", Service: "s1", TaskDefinition: "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/api:42", Container: "app"}},
					"c2/s2": {{Cluster: "c2", Service: "s2", TaskDefinition: "web:1", Container: "web"}},
				},
			},
			wantRows: [][]string{
				{"c1", "s1", "api:42", "app"},
				{"c2", "s2", "web:1", "web"},
			},
		},
		{
			name:        "cluster flag fetches only the named cluster",
			clusterFlag: "c2",
			client: &fakeTargetLister{
				clusters: []string{"c1", "c2"},
				services: map[string][]string{"c1": {"s1"}, "c2": {"s2"}},
				targets: map[string][]myecs.Target{
					"c1/s1": {{Cluster: "c1", Service: "s1", TaskDefinition: "api:42", Container: "app"}},
					"c2/s2": {{Cluster: "c2", Service: "s2", TaskDefinition: "web:1", Container: "web"}},
				},
			},
			wantRows: [][]string{
				{"c2", "s2", "web:1", "web"},
			},
		},
		{
			name:        "unknown cluster returns an error",
			clusterFlag: "missing",
			client: &fakeTargetLister{
				clusters: []string{"c1"},
			},
			wantErr: `cluster "missing" not found`,
		},
		{
			name: "client error is returned",
			client: &fakeTargetLister{
				listErr: assert.AnError,
			},
			wantErr: assert.AnError.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listSetFlags.cluster = tt.clusterFlag
			defer func() { listSetFlags.cluster = "" }()

			rows, err := listECSTable(context.Background(), tt.client)

			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				assert.Nil(t, rows)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantRows, rows)
		})
	}
}

func TestBuildECSRows(t *testing.T) {
	target := func(cluster, service, taskDefinition, container string) myecs.Target {
		return myecs.Target{
			Cluster:        cluster,
			Service:        service,
			TaskArn:        "arn:aws:ecs:ap-northeast-1:123456789012:task/prod/task-1",
			TaskDefinition: taskDefinition,
			Container:      container,
		}
	}

	tests := []struct {
		name     string
		targets  []myecs.Target
		wantRows [][]string
	}{
		{
			name: "full row per target with shortened task definition",
			targets: []myecs.Target{
				target("prod", "api", "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/api:42", "app"),
			},
			wantRows: [][]string{
				{"prod", "api", "api:42", "app"},
			},
		},
		{
			name: "one row per container target",
			targets: []myecs.Target{
				target("prod", "web", "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/api:42", "app"),
				target("prod", "web", "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/api:42", "sidecar"),
			},
			wantRows: [][]string{
				{"prod", "web", "api:42", "app"},
				{"prod", "web", "api:42", "sidecar"},
			},
		},
		{
			name: "already abbreviated task definition passes through",
			targets: []myecs.Target{
				target("dev", "web", "web:1", "nginx"),
			},
			wantRows: [][]string{
				{"dev", "web", "web:1", "nginx"},
			},
		},
		{
			name:     "no targets yields no rows",
			wantRows: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantRows, buildECSRows(tt.targets))
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
