package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// isolateAWSConfig points the shared config lookup at empty temp files and
// clears the region environment, so a test runs without the machine's AWS
// configuration.
func isolateAWSConfig(t *testing.T) {
	t.Helper()
	emptyFile := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(emptyFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", emptyFile)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", emptyFile)
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_DEFAULT_PROFILE", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
}

func TestLoadAWSConfig(t *testing.T) {
	t.Run("region flag overrides the AWS config region", func(t *testing.T) {
		isolateAWSConfig(t)
		t.Setenv("AWS_REGION", "ap-northeast-1")

		cfg, err := loadAWSConfig(context.Background(), "us-west-2")

		assert.NoError(t, err)
		assert.Equal(t, "us-west-2", cfg.Region)
	})

	t.Run("empty region flag falls back to the AWS config region", func(t *testing.T) {
		isolateAWSConfig(t)
		t.Setenv("AWS_REGION", "ap-northeast-1")

		cfg, err := loadAWSConfig(context.Background(), "")

		assert.NoError(t, err)
		assert.Equal(t, "ap-northeast-1", cfg.Region)
	})

	t.Run("no region in flag or config fails clearly", func(t *testing.T) {
		isolateAWSConfig(t)

		cfg, err := loadAWSConfig(context.Background(), "")

		assert.EqualError(t, err,
			"no AWS region configured: use --region or set AWS_REGION in the environment")
		assert.Empty(t, cfg.Region)
	})

	t.Run("load error is wrapped", func(t *testing.T) {
		isolateAWSConfig(t)
		// A profile absent from the empty config file makes
		// LoadDefaultConfig fail.
		t.Setenv("AWS_PROFILE", "missing-profile")

		cfg, err := loadAWSConfig(context.Background(), "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unable to load SDK config")
		assert.Empty(t, cfg.Region)
	})
}

func TestInitializeECSClient(t *testing.T) {
	isolateAWSConfig(t)

	client, err := initializeECSClient(context.Background(), "ap-northeast-1")

	assert.NoError(t, err)
	assert.Equal(t, "ap-northeast-1", client.Region)
}

func TestInitializeECSClientError(t *testing.T) {
	isolateAWSConfig(t)
	// A profile absent from the empty config file makes the SDK config
	// load fail.
	t.Setenv("AWS_PROFILE", "missing-profile")

	client, err := initializeECSClient(context.Background(), "")

	assert.Error(t, err)
	assert.Nil(t, client)
}
