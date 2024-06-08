package cmd

import (
	"log/slog"
	"os"

	"github.com/devon-mar/regexupdater/regexupdater"
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
		slog.Error("Error initializing regexupdater", "err", err)
		os.Exit(1)
	}

	var ret int
	for _, u := range config.Updates {
		logger := slog.With("update", u.Name)
		v, err := ru.CurrentVersion(u)
		if err != nil {
			logger.Error("Error updating", "err", err)
			ret++
		} else {
			if v.SV != nil {
				logger.Info("current version", "version", v.V, "semver", v.SV)
			} else {
				logger.Info("current version", "version", v.V)
			}
		}
	}

	return ret
}
