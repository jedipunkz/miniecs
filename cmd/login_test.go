package cmd

import (
	"context"
	"testing"

	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/stretchr/testify/assert"
)

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
			assert.Equal(t, tt.expected, getShell(tt.shell))
		})
	}
}

// fakeCommandExecutor records the ExecuteCommand call so tests can assert
// which target and shell the login flow passes through.
type fakeCommandExecutor struct {
	gotTarget myecs.Target
	gotShell  string
	err       error
}

func (f *fakeCommandExecutor) ExecuteCommand(ctx context.Context, target myecs.Target, shell string) error {
	f.gotTarget = target
	f.gotShell = shell
	return f.err
}

func TestExecuteLoginPassesTargetAndShell(t *testing.T) {
	tests := []struct {
		name      string
		shellFlag string
		wantShell string
	}{
		{
			name:      "shell flag is passed through",
			shellFlag: "bash",
			wantShell: "bash",
		},
		{
			name:      "empty flag uses the default shell",
			shellFlag: "",
			wantShell: "sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeCommandExecutor{}
			target := myecs.Target{
				Cluster:   "test-cluster",
				TaskArn:   "arn:aws:ecs:us-east-1:123456789012:task/test-task",
				Container: "test-container",
			}

			err := executeLogin(context.Background(), runner, target, tt.shellFlag)

			assert.NoError(t, err)
			assert.Equal(t, target, runner.gotTarget)
			assert.Equal(t, tt.wantShell, runner.gotShell)
		})
	}
}

func TestExecuteLoginPropagatesError(t *testing.T) {
	runner := &fakeCommandExecutor{err: assert.AnError}

	err := executeLogin(context.Background(), runner, myecs.Target{
		Cluster:   "cluster-1",
		TaskArn:   "task-1",
		Container: "container-1",
	}, "sh")

	assert.ErrorIs(t, err, assert.AnError)
}
