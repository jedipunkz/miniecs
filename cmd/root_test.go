package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func mockExit(code int) {
	// Do nothing for testing
}

func TestExecute(t *testing.T) {
	originalCmd := rootCmd
	originalExit := osExit
	defer func() {
		rootCmd = originalCmd
		osExit = originalExit
	}()

	t.Run("success", func(t *testing.T) {
		rootCmd = &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}

		Execute()
	})

	t.Run("error", func(t *testing.T) {
		osExit = mockExit

		rootCmd = &cobra.Command{
			Use: "test",
			RunE: func(cmd *cobra.Command, args []string) error {
				return assert.AnError
			},
		}

		Execute()
	})
}

func TestRootCmdFlags(t *testing.T) {
	t.Run("toggle_flag_configuration", func(t *testing.T) {
		toggleFlag := rootCmd.Flags().Lookup("toggle")
		assert.NotNil(t, toggleFlag, "toggle flag should exist")
		assert.Equal(t, "t", toggleFlag.Shorthand, "toggle flag shorthand should be 't'")
		assert.Equal(t, "false", toggleFlag.DefValue, "toggle flag default value should be 'false'")
		assert.Equal(t, "Help message for toggle", toggleFlag.Usage, "toggle flag usage message should be correct")
	})
}

func TestRootCmdProperties(t *testing.T) {
	t.Run("command_properties", func(t *testing.T) {
		assert.Equal(t, "miniecs", rootCmd.Use, "command name should be correct")
		assert.Equal(t, "A brief description of your application", rootCmd.Short, "short description should be correct")
		assert.Contains(t, rootCmd.Long, "A longer description", "long description should be correct")
	})
}
