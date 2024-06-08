package cmd

import (
	"log/slog"
	"os"

	"github.com/devon-mar/regexupdater/regexupdater"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Process updates.",
	Run: func(cmd *cobra.Command, args []string) {
		exit(run())
	},
}
var dryRun *bool

func init() {
	rootCmd.AddCommand(runCmd)
	dryRun = runCmd.Flags().Bool("dry", false, "Don't make any changes")
}

func run() int {
	if *dryRun {
		config.DryRun = true
	}
	ru, err := regexupdater.NewUpdater(config)
	if err != nil {
		slog.Error("Error initializing regexupdater", "err", err)
		os.Exit(1)
	}

	var ret int
	for _, u := range config.Updates {
		logger := slog.With("update", u.Name)
		if err := ru.Process(u, logger); err != nil {
			logger.Error("Error updating", "err", err)
			ret++
		}
	}

	return ret
}
