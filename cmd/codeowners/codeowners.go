package cmd

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.szostok.io/codeowners-validator/internal/check"
	"go.szostok.io/codeowners-validator/internal/envconfig"
	"go.szostok.io/codeowners-validator/internal/load"
	"go.szostok.io/codeowners-validator/internal/runner"
	"go.szostok.io/codeowners-validator/pkg/codeowners"
	"go.szostok.io/version/extension"
)

// Config holds the application configuration
type Config struct {
	RepositoryPath     string
	CheckFailureLevel  check.SeverityType `envconfig:"default=warning"`
	Checks             []string           `envconfig:"optional"`
	ExperimentalChecks []string           `envconfig:"optional"`
}

// NewRoot returns a root cobra.Command for the whole Agent utility.
func NewRoot() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "codeowners",
		Short:        "Ensures the correctness of your CODEOWNERS file.",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			var cfg Config
			err := envconfig.Init(&cfg)
			exitOnError(err)

			log := logrus.New()

			// init checks
			checks, err := load.Checks(cmd.Context(), cfg.Checks, cfg.ExperimentalChecks)
			exitOnError(err)

			// init codeowners entries
			codeownersEntries, err := codeowners.NewFromPath(cfg.RepositoryPath)
			exitOnError(err)

			// run check runner
			absRepoPath, err := filepath.Abs(cfg.RepositoryPath)
			exitOnError(err)

			checkRunner := runner.NewCheckRunner(log, codeownersEntries, absRepoPath, cfg.CheckFailureLevel, checks...)
			checkRunner.Run(cmd.Context())

			if cmd.Context().Err() != nil {
				log.Error("Application was interrupted by operating system")
				os.Exit(2)
			}
			if checkRunner.ShouldExitWithCheckFailure() {
				os.Exit(3)
			}
		},
	}

	rootCmd.AddCommand(
		extension.NewVersionCobraCmd(),
	)

	return rootCmd
}

func exitOnError(err error) {
	if err != nil {
		logrus.Fatal(err)
	}
}
