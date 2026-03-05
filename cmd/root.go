package cmd

import (
	"fmt"
	"os"

	runcmd "github.com/leefowlercu/go-symphony/cmd/run"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "symphony",
	Short: "Run Symphony orchestration services",
	Long: "Run Symphony orchestration services for tracker-driven coding workflows.\n\n" +
		"The root command provides top-level lifecycle control and delegates concrete runtime " +
		"execution behavior to structured subcommands.",
	Example: "# Start Symphony service loop\n" +
		"  symphony run\n\n" +
		"# Start with explicit workflow file\n" +
		"  symphony run /path/to/WORKFLOW.md",
}

func init() {
	rootCmd.AddCommand(runcmd.RunCmd)
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	return nil
}
