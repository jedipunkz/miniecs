package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

var osExit = os.Exit

var rootCmd = &cobra.Command{
	Use:   "miniecs",
	Short: "List ECS resources and log in to containers through ECS Exec",
	Long: `miniecs lists ECS clusters, services and containers, and opens an
interactive shell in a container through ECS Exec.

Use "miniecs list" to show the containers of a region as a table and
"miniecs login" to open a shell in one container.`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Execute runs the CLI. The context is cancelled on Ctrl-C so in-flight
// AWS API calls stop, and the command error is printed exactly once here
// instead of at every call site.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		osExit(1)
	}
}

func init() {
	// --region is shared by the subcommands. It is optional: when it is
	// empty the region of the AWS configuration is used.
	rootCmd.PersistentFlags().String(
		"region", "", "Region Name (defaults to the region of the AWS configuration)")
}
