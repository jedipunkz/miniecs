package cmd

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/jedipunkz/miniecs/internal/pkg/exec"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	waitServiceStablePollingInterval = 15 * time.Second
	waitServiceStableMaxTry          = 80
)

type api interface {
	ExecuteCommand(input *ecs.ExecuteCommandInput) (*ecs.ExecuteCommandOutput, error)
	ListTasks(input *ecs.ListTasksInput) (*ecs.ListTasksOutput, error)
}

type ssmSessionStarter interface {
	StartSession(ssmSession *ecs.Session) error
}

// ECS wraps an AWS ECS client.
type ECS struct {
	client         api
	newSessStarter func() ssmSessionStarter

	maxServiceStableTries int
	pollIntervalDuration  time.Duration
}

// ErrExecuteCommand occurs when ecs:ExecuteCommand fails.
type ErrExecuteCommand struct {
	err error
}

// ErrGetTask is
type ErrGetTask struct {
	err error
}

// ExecuteCommandInput holds the fields needed to execute commands in a running container.
type ExecuteCommandInput struct {
	Cluster   string
	Command   string
	Task      string
	Container string
}

var setFlags struct {
	cluster   string
	container string
	command   string
}

// Error is printing execute command err
func (e *ErrExecuteCommand) Error() string {
	return fmt.Sprintf("execute command: %s", e.err.Error())
}

// Error is printing get task command err
func (e *ErrGetTask) Error() string {
	return fmt.Sprintf("get task command: %s", e.err.Error())
}

// new returns a Service configured against the input session.
func new(s *session.Session, err error) *ECS {
	return &ECS{
		client: ecs.New(s),
		newSessStarter: func() ssmSessionStarter {
			return exec.NewSSMPluginCommand(s)
		},
		maxServiceStableTries: waitServiceStableMaxTry,
		pollIntervalDuration:  waitServiceStablePollingInterval,
	}
}

// ExecuteCommand executes commands in a running container, and then terminate the session.
func (e *ECS) executeCommand(in ExecuteCommandInput) (err error) {
	execCmdresp, err := e.client.ExecuteCommand(&ecs.ExecuteCommandInput{
		Cluster:     aws.String(in.Cluster),
		Command:     aws.String(in.Command),
		Container:   aws.String(in.Container),
		Interactive: aws.Bool(true),
		Task:        aws.String(in.Task),
	})
	if err != nil {
		return &ErrExecuteCommand{err: err}
	}
	sessID := aws.StringValue(execCmdresp.Session.SessionId)
	if err = e.newSessStarter().StartSession(execCmdresp.Session); err != nil {
		err = fmt.Errorf("start session %s using ssm plugin: %w", sessID, err)
	}
	return err
}

func (e *ECS) getTask(cluster, family string) (result *ecs.ListTasksOutput, err error) {
	getTaskCmdresp, err := e.client.ListTasks(&ecs.ListTasksInput{
		Cluster: aws.String(cluster),
		Family:  aws.String(family),
	})
	if err != nil {
		return nil, &ErrGetTask{err: err}
	}
	return getTaskCmdresp, nil
}

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "execute ecs subcommand",
	Long: `a subcommand for ecs execute to login ecs container on task.
with parameters where ecs cluster, container name and command.`,
	Run: func(cmd *cobra.Command, args []string) {
		e := new(session.NewSession())
		task, err := e.getTask("commandexec", "commandexec")
		if err != nil {
			log.Fatal(err)
		}

		in := ExecuteCommandInput{}
		in.Cluster = setFlags.cluster
		in.Container = setFlags.container
		in.Task = *task.TaskArns[0]
		in.Command = setFlags.command
		err = e.executeCommand(in)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringVarP(&setFlags.cluster, "cluster", "", "", "ECS Cluster Name")
	if err := execCmd.MarkFlagRequired("cluster"); err != nil {
		log.Fatalln(err)
	}
	execCmd.Flags().StringVarP(&setFlags.container, "container", "", "", "Container Name")
	if err := execCmd.MarkFlagRequired("container"); err != nil {
		log.Fatalln(err)
	}
	execCmd.Flags().StringVarP(&setFlags.command, "command", "", "", "Command Name")
	if err := execCmd.MarkFlagRequired("command"); err != nil {
		log.Fatalln(err)
	}
}
