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

func TestRootCmdProperties(t *testing.T) {
	t.Run("command_properties", func(t *testing.T) {
		assert.Equal(t, "miniecs", rootCmd.Use, "command name should be correct")
		assert.Equal(t,
			"List ECS resources and log in to containers through ECS Exec",
			rootCmd.Short, "short description should be correct")
		assert.Contains(t, rootCmd.Long, "ECS Exec", "long description should describe the tool")
	})
}

func TestRegionFlag(t *testing.T) {
	regionFlag := rootCmd.PersistentFlags().Lookup("region")
	assert.NotNil(t, regionFlag, "region flag should be a persistent root flag")
	assert.Equal(t, "", regionFlag.DefValue, "region flag should default to empty")

	t.Run("subcommands_inherit_region_without_requiring_it", func(t *testing.T) {
		for _, c := range []*cobra.Command{listCmd, loginCmd} {
			inherited := c.InheritedFlags().Lookup("region")
			assert.NotNil(t, inherited, "region flag should be inherited by %q", c.Name())
			assert.Empty(t, inherited.Annotations["cobra_annotation_bash_completion_one_required_flag"],
				"region flag should not be required for %q", c.Name())
		}
	})
}
