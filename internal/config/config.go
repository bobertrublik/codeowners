package config

import "go.szostok.io/codeowners-validator/internal/api"

const (
	DefaultConfigFilename = "codeowners-config.yaml"
	EnvPrefix             = "CODEOWNERS"
)

// Config holds the application configuration
type Config struct {
	CheckFailureLevel                api.SeverityType
	Checks                           []string
	ExperimentalChecks               []string
	GithubAccessToken                string
	GithubBaseURL                    string
	GithubUploadURL                  string
	GithubAppID                      int64
	GithubAppInstallationID          int64
	GithubAppPrivateKey              string
	NotOwnedCheckerSkipPatterns      []string
	NotOwnedCheckerSubdirectories    []string
	NotOwnedCheckerTrustWorkspace    bool
	OwnerCheckerRepository           string
	OwnerCheckerIgnoredOwners        []string
	OwnerCheckerAllowUnownedPatterns bool
	OwnerCheckerOwnersMustBeTeams    bool
	RepositoryPath                   string
}
