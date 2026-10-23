package cmd

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
)

// loadAWSConfig loads the SDK configuration shared by the list and login
// commands. The region flag, when set, overrides the region of the AWS
// configuration (environment or shared config files). Both must provide a
// region, because every ECS API call is region-scoped.
func loadAWSConfig(ctx context.Context, regionFlag string) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, fmt.Errorf("unable to load SDK config: %w", err)
	}

	if regionFlag != "" {
		cfg.Region = regionFlag
	}
	if cfg.Region == "" {
		return aws.Config{}, fmt.Errorf(
			"no AWS region configured: use --region or set AWS_REGION in the environment")
	}

	return cfg, nil
}

// initializeECSClient loads the SDK configuration and builds the ECS
// client the commands talk to. Shared by list and login.
func initializeECSClient(ctx context.Context, regionFlag string) (*myecs.Client, error) {
	cfg, err := loadAWSConfig(ctx, regionFlag)
	if err != nil {
		return nil, err
	}

	return myecs.NewClient(cfg, cfg.Region), nil
}
