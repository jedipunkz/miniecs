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
	item := selectableItem{
		cluster: myecs.ECSCluster{
			ClusterName: "test-cluster",
		},
		service: myecs.ECSService{
			ServiceName: "test-service",
		},
		task: myecs.ECSTask{
			TaskArn: "arn:aws:ecs:us-east-1:123456789012:task/test-task",
		},
		container: myecs.ECSContainer{
			ContainerName: "test-container",
		},
	}

	result := createExecuteCommandInput(item)

	assert.Equal(t, "test-cluster", *result.Cluster)
	assert.Equal(t, "test-container", *result.Container)
	assert.Equal(t, "arn:aws:ecs:us-east-1:123456789012:task/test-task", *result.Task)
	assert.Equal(t, "sh", *result.Command)
}

func TestGetShell(t *testing.T) {
	tests := []struct {
		name     string
		shell    string
		expected string
	}{
		{
			name:     "default shell when flag is empty",
			shell:    "",
			expected: "sh",
		},
		{
			name:     "shell flag overrides default",
			shell:    "bash",
			expected: "bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loginSetFlags.shell = tt.shell
			defer func() { loginSetFlags.shell = "" }()

			assert.Equal(t, tt.expected, getShell())
		})
	}
	loginSetFlags.shell = ""
}

func TestBuildSelectableItems(t *testing.T) {
	key := func(item selectableItem) string {
		return item.cluster.ClusterName + "/" +
			item.service.ServiceName + "/" +
			item.task.TaskArn + "/" +
			item.container.ContainerName
	}

	svcTask := func(taskArn string, containerNames ...string) myecs.ECSTask {
		task := myecs.ECSTask{TaskArn: taskArn}
		for _, name := range containerNames {
			task.Containers = append(task.Containers, myecs.ECSContainer{ContainerName: name})
		}
		return task
	}

	tests := []struct {
		name     string
		input    []myecs.ECSResource
		expected []string
	}{
		{
			name:     "no resources",
			input:    []myecs.ECSResource{},
			expected: nil,
		},
		{
			name: "resource without clusters is skipped",
			input: []myecs.ECSResource{
				{Clusters: []myecs.ECSCluster{}},
			},
			expected: nil,
		},
		{
			name: "service without tasks yields no items",
			input: []myecs.ECSResource{
				{
					Clusters: []myecs.ECSCluster{
						{
							ClusterName: "cluster-1",
							Services: []myecs.ECSService{
								{ServiceName: "service-1"},
							},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "one cluster, service, task and container",
			input: []myecs.ECSResource{
				{
					Clusters: []myecs.ECSCluster{
						{
							ClusterName: "cluster-1",
							Services: []myecs.ECSService{
								{
									ServiceName: "service-1",
									Tasks:       []myecs.ECSTask{svcTask("task-1", "container-1")},
								},
							},
						},
					},
				},
			},
			expected: []string{"cluster-1/service-1/task-1/container-1"},
		},
		{
			name: "multiple containers and services are flattened in order",
			input: []myecs.ECSResource{
				{
					Clusters: []myecs.ECSCluster{
						{
							ClusterName: "cluster-1",
							Services: []myecs.ECSService{
								{
									ServiceName: "service-1",
									Tasks:       []myecs.ECSTask{svcTask("task-1", "container-a", "container-b")},
								},
								{
									ServiceName: "service-2",
									Tasks:       []myecs.ECSTask{svcTask("task-2", "container-c")},
								},
							},
						},
					},
				},
			},
			expected: []string{
				"cluster-1/service-1/task-1/container-a",
				"cluster-1/service-1/task-1/container-b",
				"cluster-1/service-2/task-2/container-c",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := buildSelectableItems(tt.input)
			var got []string
			for _, item := range items {
				got = append(got, key(item))
			}
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestExecuteLoginPropagatesError(t *testing.T) {
	// A zero-value ECSResource has no initialized client, so
	// ExecuteCommand fails and executeLogin must return the error.
	item := selectableItem{
		cluster:   myecs.ECSCluster{ClusterName: "cluster-1"},
		service:   myecs.ECSService{ServiceName: "service-1"},
		task:      myecs.ECSTask{TaskArn: "task-1"},
		container: myecs.ECSContainer{ContainerName: "container-1"},
	}

	err := executeLogin(&myecs.ECSResource{}, item)

	assert.Error(t, err)
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
