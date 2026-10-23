package cmd

import (
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

	result, err := createExecuteCommandInput(resource)

	assert.NoError(t, err)
	assert.Equal(t, "test-cluster", *result.Cluster)
	assert.Equal(t, "test-container", *result.Container)
	assert.Equal(t, "arn:aws:ecs:us-east-1:123456789012:task/test-task", *result.Task)
	assert.Equal(t, "sh", *result.Command)
}

func TestExtractTaskAndContainer(t *testing.T) {
	// baseResource returns a fresh resource each call: mutators write to
	// slice backing arrays, which would otherwise leak between cases.
	baseResource := func() myecs.ECSResource {
		return myecs.ECSResource{
			Clusters: []myecs.ECSCluster{{
				ClusterName: "test-cluster",
				Services: []myecs.ECSService{{
					Tasks: []myecs.ECSTask{{
						TaskArn: "arn:aws:ecs:us-east-1:123456789012:task/test-task",
						Containers: []myecs.ECSContainer{{
							ContainerName: "test-container",
						}},
					}},
				}},
			}},
		}
	}

	cases := []struct {
		name    string
		mutator func(resource *myecs.ECSResource)
		wantMsg string
	}{
		{
			name:    "no clusters",
			mutator: func(r *myecs.ECSResource) { r.Clusters = nil },
			wantMsg: "no cluster information",
		},
		{
			name: "no services",
			mutator: func(r *myecs.ECSResource) {
				r.Clusters[0].Services = nil
			},
			wantMsg: "no service information",
		},
		{
			name: "no tasks",
			mutator: func(r *myecs.ECSResource) {
				r.Clusters[0].Services[0].Tasks = nil
			},
			wantMsg: "no task information",
		},
		{
			name: "empty task ARN",
			mutator: func(r *myecs.ECSResource) {
				r.Clusters[0].Services[0].Tasks[0].TaskArn = ""
			},
			wantMsg: "task has no task ARN",
		},
		{
			name: "no containers",
			mutator: func(r *myecs.ECSResource) {
				r.Clusters[0].Services[0].Tasks[0].Containers = nil
			},
			wantMsg: "has no containers",
		},
		{
			name: "empty container name",
			mutator: func(r *myecs.ECSResource) {
				r.Clusters[0].Services[0].Tasks[0].Containers[0].ContainerName = ""
			},
			wantMsg: "has no container name",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resource := baseResource()
			tc.mutator(&resource)
			_, _, err := extractTaskAndContainer(resource)
			assert.ErrorContains(t, err, tc.wantMsg)
		})
	}

	containerName, taskArn, err := extractTaskAndContainer(baseResource())
	assert.NoError(t, err)
	assert.Equal(t, "test-container", containerName)
	assert.Equal(t, "arn:aws:ecs:us-east-1:123456789012:task/test-task", taskArn)
}

func TestExecuteLoginRejectsInvalidSelection(t *testing.T) {
	err := executeLogin(&myecs.ECSResource{}, nil)
	assert.ErrorContains(t, err, "no resource selected")

	// The resource has no data, so executeLogin must fail in
	// createExecuteCommandInput before touching the ECS client (which is
	// nil here and would fail with "ECS client is not initialized"
	// instead).
	err = executeLogin(&myecs.ECSResource{}, []myecs.ECSResource{{}})
	assert.ErrorContains(t, err, "no cluster information")
}
