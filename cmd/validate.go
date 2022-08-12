package cmd

import (
	"github.com/devon-mar/regexupdater/feed"
	"github.com/devon-mar/regexupdater/regexupdater"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the config.",
	Run: func(cmd *cobra.Command, args []string) {
		exit(runValidate())
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate() int {
	if err := regexupdater.ValidateConfig(config); err != nil {
		log.WithError(err).Fatal("Error initializing regexupdater")
	}

	var ret int

	feedTypes := make(map[string]string, len(config.Feeds))
	for name, cfg := range config.Feeds {
		feedTypes[name] = cfg.Type
		err := feed.Validate(name, cfg.Type, cfg.Config)
		if err != nil {
			ret++
			log.WithError(err).WithField("feed", name).Error("error validating feed")
		}
	}

	for _, u := range config.Updates {
		if err := feed.ValidateUpdate(feedTypes[u.Feed.Name], u.Feed.Config); err != nil {
			ret++
			log.WithError(err).WithField("update", u.Name).Error("error validating update feed config")
		}

		if u.SecondaryFeed != nil {
			if err := feed.ValidateUpdate(feedTypes[u.SecondaryFeed.Feed.Name], u.SecondaryFeed.Feed.Config); err != nil {
				ret++
				log.WithError(err).WithField("update", u.Name).Error("error validating secondary update feed config")
			}
		}
	}
	return ret
}
