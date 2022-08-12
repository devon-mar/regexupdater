package cmd

import (
	"github.com/devon-mar/regexupdater/regexupdater"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// currentVersionCmd represents the currentVersion command
var currentVersionCmd = &cobra.Command{
	Use:   "current-versions",
	Short: "Retrieves the current version for each update.",
	Run: func(cmd *cobra.Command, args []string) {
		exit(currentVersions())
	},
}

func init() {
	rootCmd.AddCommand(currentVersionCmd)
}

func currentVersions() int {
	ru, err := regexupdater.NewUpdater(config)
	if err != nil {
		log.WithError(err).Fatal("Error initializing regexupdater")
	}

	var ret int
	for _, u := range config.Updates {
		logger := log.WithField("update", u.Name)
		v, err := ru.CurrentVersion(u)
		if err != nil {
			logger.WithError(err).Error("Error updating")
			ret++
		} else {
			if v.SV != nil {
				logger.Infof("current version: %s (%s)", v.V, v.SV)
			} else {
				logger.Infof("current version: %s", v.V)
			}
		}
	}

	return ret
}
