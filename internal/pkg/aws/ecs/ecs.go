package ecs

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/jedipunkz/miniecs/internal/pkg/exec"
	log "github.com/sirupsen/logrus"
)

const (
	waitServiceStablePollingInterval = 15 * time.Second
	waitServiceStableMaxTry          = 80
)

type api interface {
	ExecuteCommand(input *ecs.ExecuteCommandInput) (*ecs.ExecuteCommandOutput, error)
	ListTasks(input *ecs.ListTasksInput) (*ecs.ListTasksOutput, error)
	ListClusters(input *ecs.ListClustersInput) (*ecs.ListClustersOutput, error)
	ListServices(input *ecs.ListServicesInput) (*ecs.ListServicesOutput, error)
}

type ssmSessionStarter interface {
	StartSession(ssmSession *ecs.Session) error
}

// ECS wraps an AWS ECS client.
type ECS struct {
	client         api
	newSessStarter func() ssmSessionStarter

	clusters []string
	services []string

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

// Error is printing execute command err
func (e *ErrExecuteCommand) Error() string {
	return fmt.Sprintf("execute command: %s", e.err.Error())
}

// Error is printing get task command err
func (e *ErrGetTask) Error() string {
	return fmt.Sprintf("get task command: %s", e.err.Error())
}

// NewEcs returns a Service configured against the input session.
func NewEcs(s *session.Session, err error) *ECS {
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
func (e *ECS) ExecuteCommand(in ExecuteCommandInput) (err error) {
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

// GetTask is
func (e *ECS) GetTask(cluster, family string) (result *ecs.ListTasksOutput, err error) {
	getTaskCmdresp, err := e.client.ListTasks(&ecs.ListTasksInput{
		Cluster: aws.String(cluster),
		Family:  aws.String(family),
	})
	if err != nil {
		return nil, &ErrGetTask{err: err}
	}
	return getTaskCmdresp, nil
}

// GetClusters is function to get list clusters
func (e *ECS) GetClusters() ([]string, error) {
	resultClusters, err := e.client.ListClusters(&ecs.ListClustersInput{})
	if err != nil {
		return nil, err
	}

	var c []string
	for _, cluster := range resultClusters.ClusterArns {
		clusterArr := strings.Split(*cluster, "/")
		c = append(c, clusterArr[len(clusterArr)-1])
	}
	return c, nil
}

// GetServices is function to get list services
func (e *ECS) GetServices(cluster string) ([]string, error) {
	inputService := &ecs.ListServicesInput{
		Cluster: aws.String(cluster),
	}
	resultServices, err := e.client.ListServices(inputService)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	var s []string
	for _, service := range resultServices.ServiceArns {
		serviceArr := strings.Split(*service, "/")
		s = append(s, serviceArr[len(serviceArr)-1])
	}
	return s, nil
}
