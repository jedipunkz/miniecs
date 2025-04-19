package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// mockExit is a test function that replaces os.Exit
func mockExit(code int) {
	// Do nothing for testing
}

func TestExecute(t *testing.T) {
	// Save original command and exit function
	originalCmd := rootCmd
	originalExit := osExit
	defer func() {
		rootCmd = originalCmd
		osExit = originalExit
	}()

	// Test successful execution
	t.Run("success", func(t *testing.T) {
		// Create test command
		rootCmd = &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}

		// Execute test
		Execute()
	})

	// Test error handling
	t.Run("error", func(t *testing.T) {
		// Replace os.Exit with mock
		osExit = mockExit

		// Create test command that returns error
		rootCmd = &cobra.Command{
			Use: "test",
			RunE: func(cmd *cobra.Command, args []string) error {
				return assert.AnError
			},
		}

		// Execute test
		Execute()
	})
}

func TestRootCmdFlags(t *testing.T) {
	// Test toggle flag configuration
	t.Run("toggle_flag_configuration", func(t *testing.T) {
		// Get flag
		toggleFlag := rootCmd.Flags().Lookup("toggle")
		assert.NotNil(t, toggleFlag, "toggle flag should exist")
		assert.Equal(t, "t", toggleFlag.Shorthand, "toggle flag shorthand should be 't'")
		assert.Equal(t, "false", toggleFlag.DefValue, "toggle flag default value should be 'false'")
		assert.Equal(t, "Help message for toggle", toggleFlag.Usage, "toggle flag usage message should be correct")
	})
}

func TestRootCmdProperties(t *testing.T) {
	// Test command properties
	t.Run("command_properties", func(t *testing.T) {
		assert.Equal(t, "miniecs", rootCmd.Use, "command name should be correct")
		assert.Equal(t, "A brief description of your application", rootCmd.Short, "short description should be correct")
		assert.Contains(t, rootCmd.Long, "A longer description", "long description should be correct")
	})
} 