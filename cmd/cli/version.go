

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of servify",
	Long:  `All software has versions. This is servify's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version: %s\nCommit: %s\nBuildTime: %s\n", Version, Commit, BuildTime)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

