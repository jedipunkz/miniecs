package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockConfigLoader struct {
	mock.Mock
}

func (m *mockConfigLoader) LoadDefaultConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	args := m.Called(ctx, optFns)
	if cfg, ok := args.Get(0).(aws.Config); ok {
		return cfg, args.Error(1)
	}
	return aws.Config{}, args.Error(1)
}

func TestInitializeECSClient(t *testing.T) {
	originalLoader := defaultLoader
	defer func() {
		defaultLoader = originalLoader
	}()

	t.Run("without AWS credentials", func(t *testing.T) {
		mockLoader := new(mockConfigLoader)
		defaultLoader = mockLoader

		mockLoader.On("LoadDefaultConfig", mock.Anything, mock.Anything).Return(aws.Config{}, errors.New("missing credentials"))

		loginSetFlags.region = "us-west-2"
		ctx := context.Background()

		client, err := initializeECSClient(ctx)
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "unable to load SDK config")

		mockLoader.AssertExpectations(t)
	})
}

func TestCreateExecuteCommandInput(t *testing.T) {
	loginSetFlags.shell = "sh"
	resource := myecs.ECSResource{
		Clusters: []myecs.Cluster{
			{ClusterName: "test-cluster"},
		},
		Services: []myecs.Service{
			{ServiceName: "test-service"},
		},
		Tasks: []myecs.Task{
			{TaskArn: "arn:aws:ecs:region:account:task/test-task"},
		},
		Containers: []myecs.Container{
			{ContainerName: "test-container"},
		},
	}

	result := createExecuteCommandInput(resource)

	assert.Equal(t, "test-cluster", *result.Cluster)
	assert.Equal(t, "test-container", *result.Container)
	assert.Equal(t, "arn:aws:ecs:region:account:task/test-task", *result.Task)
	assert.Equal(t, "sh", *result.Command)
}
