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
	DescribeServices(input *ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error)
}

type ssmSessionStarter interface {
	StartSession(ssmSession *ecs.Session) error
}

// ECS wraps an AWS ECS client.
type ECS struct {
	client         api
	newSessStarter func() ssmSessionStarter

	Clusters       []string
	Services       []string
	Task           *ecs.ListTasksOutput
	Service        string
	TaskDefinition string

	maxServiceStableTries int
	pollIntervalDuration  time.Duration
}

// ExecuteCommandInput holds the fields needed to execute commands in a running container.
type ExecuteCommandInput struct {
	Cluster   string
	Command   string
	Task      string
	Container string
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

// GetTask to get ECS task family
func (e *ECS) GetTask(cluster, family string) error {
	getTaskCmdresp, err := e.client.ListTasks(&ecs.ListTasksInput{
		Cluster: aws.String(cluster),
		Family:  aws.String(family),
	})
	if err != nil {
		return &ErrGetTask{err: err}
	}
	e.Task = getTaskCmdresp
	if len(e.Task.TaskArns) == 0 {
		log.Println("Task Family not found.")
		return &ErrGetTask{err: err}
	}

	return nil
}

// GetService is function to get ecs service(s)
func (e *ECS) GetService(cluster, service string) error {
	inputService := &ecs.DescribeServicesInput{
		Cluster: aws.String(cluster),
		Services: []*string{
			aws.String(service),
		},
	}
	resultService, err := e.client.DescribeServices(inputService)
	if err != nil {
		log.Fatal(err)
		return &ErrListServices{err: err}
	}
	familySep1 := strings.Split(string(*resultService.Services[0].TaskDefinition), "/")
	familySep2 := strings.Split(string(familySep1[len(familySep1)-1]), ":")
	e.TaskDefinition = familySep2[0]
	return nil
}

// ListClusters is function to get list of clusters
func (e *ECS) ListClusters() error {
	resultClusters, err := e.client.ListClusters(&ecs.ListClustersInput{})
	if err != nil {
		return &ErrListClusters{err: err}
	}

	var c []string
	for _, cluster := range resultClusters.ClusterArns {
		clusterArr := strings.Split(*cluster, "/")
		c = append(c, clusterArr[len(clusterArr)-1])
	}
	e.Clusters = c
	return nil
}

// ListServices is function to get list of services
func (e *ECS) ListServices(cluster string) error {
	inputService := &ecs.ListServicesInput{Cluster: aws.String(cluster)}
	resultServices, err := e.client.ListServices(inputService)
	if err != nil {
		log.Fatal(err)
		return &ErrListServices{err: err}
	}

	var s []string
	for _, service := range resultServices.ServiceArns {
		serviceArr := strings.Split(*service, "/")
		s = append(s, serviceArr[len(serviceArr)-1])
	}
	e.Services = s
	return nil
}
