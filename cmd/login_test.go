package cmd

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/service/ecs"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
)

func TestExecECS_listECSs(t *testing.T) {
	type fields struct {
		Cluster        string
		Service        string
		Task           string
		TaskDefinition string
		Container      string
		Command        string
		Shell          string
	}
	type args struct {
		e *myecs.ECS
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   ExecECSs
	}{
		{
			name: "listECSs",
			fields: fields{
				Cluster:        "cluster",
				Service:        "service",
				Task:           "task",
				TaskDefinition: "taskdef",
				Container:      "container",
				Command:        "command",
				Shell:          "bash",
			},
			args: args{
				e: &myecs.ECS{
					Clusters:       []string{"Cluster"},
					Services:       []string{"Service"},
					Containers:     []string{"Container"},
					Task:           &ecs.ListTasksOutput{NextToken: nil, TaskArns: []*string{nil}},
					Service:        "service",
					TaskDefinition: "taskdef",
				},
			},
			want: ExecECSs{
				ExecECS{
					Cluster:        "cluster",
					Service:        "service",
					Task:           "task",
					TaskDefinition: "taskdef",
					Container:      "container",
					Command:        "command",
					Shell:          "bash",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &ExecECS{
				Cluster:        tt.fields.Cluster,
				Service:        tt.fields.Service,
				Task:           tt.fields.Task,
				TaskDefinition: tt.fields.TaskDefinition,
				Container:      tt.fields.Container,
				Command:        tt.fields.Command,
				Shell:          tt.fields.Shell,
			}
			if got := l.listECSs(tt.args.e); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExecECS.listECSs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecECS_login(t *testing.T) {
	type fields struct {
		Cluster        string
		Service        string
		Task           string
		TaskDefinition string
		Container      string
		Command        string
		Shell          string
	}
	type args struct {
		e *myecs.ECS
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "login",
			fields: fields{
				Cluster:        "cluster",
				Service:        "service",
				Task:           "task",
				TaskDefinition: "taskdef",
				Container:      "container",
				Command:        "command",
				Shell:          "bash",
			},
			args: args{
				e: &myecs.ECS{
					Clusters:   []string{"test"},
					Services:   []string{"test"},
					Containers: []string{"test"},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &ExecECS{
				Cluster:        tt.fields.Cluster,
				Service:        tt.fields.Service,
				Task:           tt.fields.Task,
				TaskDefinition: tt.fields.TaskDefinition,
				Container:      tt.fields.Container,
				Command:        tt.fields.Command,
				Shell:          tt.fields.Shell,
			}
			if err := l.login(tt.args.e); (err != nil) != tt.wantErr {
				t.Errorf("ExecECS.login() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
