package cmd

import (
	"github.com/devon-mar/regexupdater/regexupdater"
	log "github.com/sirupsen/logrus"
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
		log.WithError(err).Fatal("Error initializing regexupdater")
	}

	var ret int
	for _, u := range config.Updates {
		logger := log.WithField("update", u.Name)
		if err := ru.Process(u, logger); err != nil {
			logger.WithError(err).Error("Error updating")
			ret++
		}
	}

	return ret
}
