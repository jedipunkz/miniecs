package cmd

import (
	"context"
	"fmt"

	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/ktr0731/go-fuzzyfinder"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "login cluster, service",
	RunE:  runLoginCmd,
}

// ecsCommandExecutor is the subset of *myecs.Client that opens the
// interactive ECS Exec session.
type ecsCommandExecutor interface {
	ExecuteCommand(ctx context.Context, target myecs.Target, shell string) error
}

// runLoginCmd fetches the targets, lets the user pick one and opens the
// login shell in it.
func runLoginCmd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	region, err := cmd.Flags().GetString("region")
	if err != nil {
		return err
	}
	cluster, err := cmd.Flags().GetString("cluster")
	if err != nil {
		return err
	}
	shellFlag, err := cmd.Flags().GetString("shell")
	if err != nil {
		return err
	}

	ecsClient, err := initializeECSClient(ctx, region)
	if err != nil {
		return err
	}

	targets, err := fetchTargets(ctx, ecsClient, cluster)
	if err != nil {
		return err
	}

	selected, err := showTargetPicker(targets)
	if err != nil {
		return err
	}

	return executeLogin(ctx, ecsClient, selected, shellFlag)
}

func showTargetPicker(targets []myecs.Target) (myecs.Target, error) {
	if len(targets) == 0 {
		return myecs.Target{}, fmt.Errorf("no ECS resources found")
	}

	// Only one interactive session can be attached at a time, so use
	// single selection instead of FindMulti. FindMulti results were
	// otherwise silently truncated to the first entry.
	itemIndex, err := fuzzyfinder.Find(
		targets,
		func(i int) string {
			return fmt.Sprintf("%s %s %s",
				targets[i].Cluster,
				targets[i].Service,
				targets[i].Container,
			)
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			return fmt.Sprintf(
				"Cluster: %s\nService: %s\nContainer: %s\n",
				targets[i].Cluster,
				targets[i].Service,
				targets[i].Container,
			)
		}),
	)
	if err != nil {
		return myecs.Target{}, err
	}

	return targets[itemIndex], nil
}

func executeLogin(ctx context.Context, ecsClient ecsCommandExecutor, target myecs.Target, shellFlag string) error {
	shell := getShell(shellFlag)

	log.WithFields(log.Fields{
		"cluster":   target.Cluster,
		"task":      target.TaskArn,
		"container": target.Container,
		"command":   shell,
	}).Info("ECS Execute Login with These Parameters")

	return ecsClient.ExecuteCommand(ctx, target, shell)
}

// getShell returns the login shell: the --shell value when set, "sh"
// otherwise.
func getShell(shell string) string {
	if shell != "" {
		return shell
	}
	return "sh"
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.Flags().String(
		"cluster", "", "ECS Cluster Name")
	loginCmd.Flags().String(
		"shell", "", "Login Shell")
}
