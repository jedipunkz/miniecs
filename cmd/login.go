package cmd

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/ktr0731/go-fuzzyfinder"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type loginFlags struct {
	region  string
	cluster string
	shell   string
}

var loginSetFlags loginFlags

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "login cluster, service",
	Run:   runLoginCmd,
}

// ecsCommandExecutor is the subset of *myecs.Client that opens the
// interactive ECS Exec session.
type ecsCommandExecutor interface {
	ExecuteCommand(ctx context.Context, target myecs.Target, shell string) error
}

func runLoginCmd(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	ecsClient, err := initializeECSClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	targets, err := fetchTargets(ctx, ecsClient, loginSetFlags.cluster)
	if err != nil {
		log.Fatal(err)
	}

	selected, err := showTargetPicker(targets)
	if err != nil {
		log.Fatal(err)
	}

	if err := executeLogin(ctx, ecsClient, selected); err != nil {
		log.Fatal(err)
	}
}

func initializeECSClient(ctx context.Context) (*myecs.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(loginSetFlags.region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	ecsClient := myecs.NewClient(cfg, loginSetFlags.region)
	if ecsClient == nil {
		return nil, fmt.Errorf("failed to initialize ECS client")
	}

	return ecsClient, nil
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

func executeLogin(ctx context.Context, ecsClient ecsCommandExecutor, target myecs.Target) error {
	shell := getShell()

	log.WithFields(log.Fields{
		"cluster":   target.Cluster,
		"task":      target.TaskArn,
		"container": target.Container,
		"command":   shell,
	}).Info("ECS Execute Login with These Parameters")

	return ecsClient.ExecuteCommand(ctx, target, shell)
}

func getShell() string {
	if loginSetFlags.shell != "" {
		return loginSetFlags.shell
	}
	return "sh"
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.Flags().StringVarP(
		&loginSetFlags.region, "region", "", "", "Region Name")
	if err := loginCmd.MarkFlagRequired("region"); err != nil {
		log.Fatal(err)
	}
	loginCmd.Flags().StringVarP(
		&loginSetFlags.cluster, "cluster", "", "", "ECS Cluster Name")
	loginCmd.Flags().StringVarP(
		&loginSetFlags.shell, "shell", "", "", "Login Shell")
}
