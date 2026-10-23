package cmd

import (
	"context"
	"errors"
	"testing"

	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/stretchr/testify/assert"
)

func TestCreateExecuteCommandInput(t *testing.T) {
	loginSetFlags.shell = "sh"
	resource := myecs.ECSResource{
		Clusters: []myecs.ECSCluster{
			{
				ClusterName: "test-cluster",
				Services: []myecs.ECSService{
					{
						ServiceName: "test-service",
						Tasks: []myecs.ECSTask{
							{
								TaskArn: "arn:aws:ecs:us-east-1:123456789012:task/test-task",
								Containers: []myecs.ECSContainer{
									{
										ContainerName: "test-container",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result := createExecuteCommandInput(resource)

	assert.Equal(t, "test-cluster", *result.Cluster)
	assert.Equal(t, "test-container", *result.Container)
	assert.Equal(t, "arn:aws:ecs:us-east-1:123456789012:task/test-task", *result.Task)
	assert.Equal(t, "sh", *result.Command)
}

// fakeLoginClient records the calls fetchAllECSResources makes so tests can
// assert which clusters are fetched.
type fakeLoginClient struct {
	gotListClusters int
	gotGetClusters  []string
	resources       []myecs.ECSResource
	getErr          error
}

func (f *fakeLoginClient) ListClusters(ctx context.Context) error {
	f.gotListClusters++
	return nil
}

func (f *fakeLoginClient) GetClusterResources(
	ctx context.Context,
	cluster myecs.ECSCluster,
) ([]myecs.ECSResource, error) {
	f.gotGetClusters = append(f.gotGetClusters, cluster.ClusterName)
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.resources, nil
}

func TestSelectTargetClusters(t *testing.T) {
	tests := []struct {
		name         string
		clusterFlag  string
		allClusters  []myecs.ECSCluster
		wantClusters []string
		wantErr      string
	}{
		{
			name:         "empty flag selects all clusters",
			clusterFlag:  "",
			allClusters:  []myecs.ECSCluster{{ClusterName: "c1"}, {ClusterName: "c2"}},
			wantClusters: []string{"c1", "c2"},
		},
		{
			name:         "flag selects the named cluster only",
			clusterFlag:  "c2",
			allClusters:  []myecs.ECSCluster{{ClusterName: "c1"}, {ClusterName: "c2"}, {ClusterName: "c3"}},
			wantClusters: []string{"c2"},
		},
		{
			name:         "unknown cluster returns an error",
			clusterFlag:  "missing",
			allClusters:  []myecs.ECSCluster{{ClusterName: "c1"}},
			wantClusters: nil,
			wantErr:      `cluster "missing" not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectTargetClusters(tt.allClusters, tt.clusterFlag)

			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			names := make([]string, len(got))
			for i, cluster := range got {
				names[i] = cluster.ClusterName
			}
			assert.Equal(t, tt.wantClusters, names)
		})
	}
}

func TestFetchAllECSResources(t *testing.T) {
	tests := []struct {
		name          string
		clusterFlag   string
		allClusters   []myecs.ECSCluster
		wantFetched   []string
		wantResources int
		clientErr     string
		wantErr       string
	}{
		{
			name:          "empty flag fetches every cluster",
			clusterFlag:   "",
			allClusters:   []myecs.ECSCluster{{ClusterName: "c1"}, {ClusterName: "c2"}},
			wantFetched:   []string{"c1", "c2"},
			wantResources: 2,
		},
		{
			name:          "flag fetches only the named cluster",
			clusterFlag:   "c2",
			allClusters:   []myecs.ECSCluster{{ClusterName: "c1"}, {ClusterName: "c2"}},
			wantFetched:   []string{"c2"},
			wantResources: 1,
		},
		{
			name:          "unknown cluster fails without fetching",
			clusterFlag:   "missing",
			allClusters:   []myecs.ECSCluster{{ClusterName: "c1"}},
			wantFetched:   nil,
			wantResources: 0,
			wantErr:       `cluster "missing" not found`,
		},
		{
			name:          "client error is returned",
			clusterFlag:   "",
			allClusters:   []myecs.ECSCluster{{ClusterName: "c1"}},
			wantFetched:   []string{"c1"},
			wantResources: 0,
			clientErr:     "boom",
			wantErr:       "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalCluster := loginSetFlags.cluster
			loginSetFlags.cluster = tt.clusterFlag
			defer func() { loginSetFlags.cluster = originalCluster }()

			var clientErr error
			if tt.clientErr != "" {
				clientErr = errors.New(tt.clientErr)
			}
			client := &fakeLoginClient{
				resources: []myecs.ECSResource{{}},
				getErr:    clientErr,
			}

			got, err := fetchAllECSResources(context.Background(), client, tt.allClusters)

			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantFetched, client.gotGetClusters)
			assert.Equal(t, tt.wantResources, len(got))
		})
	}
}
