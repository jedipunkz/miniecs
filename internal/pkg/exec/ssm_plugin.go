package exec

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/ecs"
)

const (
    ssmPluginBinaryName = "session-manager-plugin"
    startSessionAction  = "StartSession"
)

type Runner interface {
    InteractiveRun(name string, args []string) error
}

// SSMPluginCommand represents commands that can be run to trigger the ssm plugin.
type SSMPluginCommand struct {
    client *ecs.Client
    runner Runner
    http   httpClient
}

// NewSSMPluginCommand returns a SSMPluginCommand.
func NewSSMPluginCommand() (SSMPluginCommand, error) {
    cfg, err := config.LoadDefaultConfig(context.TODO())
    if err != nil {
        return SSMPluginCommand{}, err
    }

    client := ecs.NewFromConfig(cfg)
    return SSMPluginCommand{
        runner: NewCmd(),
        client: client,
        http:   http.DefaultClient,
    }, nil
}

// StartSession starts a session using the ssm plugin.
func (s SSMPluginCommand) StartSession() error {
    response, err := json.Marshal(s.client)
    if err != nil {
        return fmt.Errorf("marshal session response: %w", err)
    }
    if err := s.runner.InteractiveRun(ssmPluginBinaryName,
        []string{string(response), aws.ToString(s.client.Config.Region), startSessionAction}); err != nil {
        return fmt.Errorf("start session: %w", err)
    }
    return nil
}