package cmd

import (
	"log/slog"
	"os"

	"github.com/devon-mar/regexupdater/regexupdater"
	"github.com/spf13/cobra"
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
			slog.Error("exactly one argument is required")
			os.Exit(1)
		} else if len(args) == 1 {
			prID = args[0]
		} else {
			if envPR := os.Getenv(envRegexUpdaterPRID); envPR != "" {
				prID = envPR
			} else {
				slog.Error("PR ID is required.")
				os.Exit(1)
			}
		}
		ru, err := regexupdater.NewUpdater(config)
		if err != nil {
			slog.Error("Error initializing regexupdater", "err", err)
			os.Exit(1)
		}
		branch, err := ru.DeletePRBranch(prID)
		if err != nil {
			slog.Error("Error deleting branch.", "pr", prID, "err", err)
			os.Exit(1)
		}
		slog.Info("Deleted branch.", "branch", branch)
	},
}

func init() {
	rootCmd.AddCommand(deletePRBranchCmd)
}
