package exec

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
)

const (
	ssmPluginBinaryName = "session-manager-plugin"
	startSessionAction  = "StartSession"
)

// SSMPluginCommand represents commands that can be run to trigger the ssm plugin.
type SSMPluginCommand struct {
	sess *session.Session
	runner
	http httpClient
}

// NewSSMPluginCommand returns a SSMPluginCommand.
func NewSSMPluginCommand(s *session.Session) SSMPluginCommand {
	return SSMPluginCommand{
		runner: NewCmd(),
		sess:   s,
		http:   http.DefaultClient,
	}
}

// StartSession starts a session using the ssm plugin.
func (s SSMPluginCommand) StartSession(ssmSess *ecs.Session) error {
	response, err := json.Marshal(ssmSess)
	if err != nil {
		return fmt.Errorf("marshal session response: %w", err)
	}
	if err := s.runner.InteractiveRun(ssmPluginBinaryName,
		[]string{string(response), aws.StringValue(s.sess.Config.Region), startSessionAction}); err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	return nil
}
