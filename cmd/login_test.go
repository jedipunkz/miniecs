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

	result := createExecuteCommandInput(resource)

	assert.Equal(t, "test-cluster", *result.Cluster)
	assert.Equal(t, "test-container", *result.Container)
	assert.Equal(t, "arn:aws:ecs:us-east-1:123456789012:task/test-task", *result.Task)
	assert.Equal(t, "sh", *result.Command)
}
