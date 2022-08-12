package cmd

import (
	"os"

	"github.com/devon-mar/regexupdater/regexupdater"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
)

const (
	envRegexUpdaterPRID = "REGEX_UPDATER_PR_ID"
)

var deletePRBranchCmd = &cobra.Command{
	Use:   "delete-pr-branch [PRID]",
	Short: "Deletes the head branch of the given PR.",
	Long: `Deletes the head branch of the given PR.

The PR ID can also be given through the environment variable ` + envRegexUpdaterPRID + ".",
	Run: func(cmd *cobra.Command, args []string) {
		var prID string
		if len(args) > 1 {
			log.Fatal("exactly one argument is required")
		} else if len(args) == 1 {
			prID = args[0]
		} else {
			if envPR := os.Getenv(envRegexUpdaterPRID); envPR != "" {
				prID = envPR
			} else {
				log.Fatal("PR ID is required.")
			}
		}
		ru, err := regexupdater.NewUpdater(config)
		if err != nil {
			log.WithError(err).Fatal("Error initializing regexupdater")
		}
		branch, err := ru.DeletePRBranch(prID)
		if err != nil {
			log.WithError(err).Fatal("Error deleting branch.")
		}
		log.WithField("branch", branch).Info("Deleted branch.")
	},
}

func init() {
	rootCmd.AddCommand(deletePRBranchCmd)
}
