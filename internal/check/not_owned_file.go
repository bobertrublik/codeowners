package check

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"go.szostok.io/codeowners/internal/api"
	"go.szostok.io/codeowners/internal/ctxutil"
	"go.szostok.io/codeowners/pkg/codeowners"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"gopkg.in/pipe.v2"
)

type NotOwnedFileConfig struct {
	// TrustWorkspace sets the global gif config
	// to trust a given repository path
	// see: https://github.com/actions/checkout/issues/766
	TrustWorkspace bool     `envconfig:"default=false"`
	SkipPatterns   []string `envconfig:"optional"`
	Subdirectories []string `envconfig:"optional"`
}

type NotOwnedFile struct {
	skipPatterns   map[string]struct{}
	subDirectories []string
	trustWorkspace bool
}

func NewNotOwnedFile(cfg NotOwnedFileConfig) *NotOwnedFile {
	skip := map[string]struct{}{}
	for _, p := range cfg.SkipPatterns {
		skip[p] = struct{}{}
	}

	return &NotOwnedFile{
		skipPatterns:   skip,
		subDirectories: cfg.Subdirectories,
		trustWorkspace: cfg.TrustWorkspace,
	}
}

func (c *NotOwnedFile) Check(ctx context.Context, in api.Input) (output api.Output, err error) {
	if ctxutil.ShouldExit(ctx) {
		return api.Output{}, ctx.Err()
	}

	var bldr api.OutputBuilder

	if len(in.CodeownersEntries) == 0 {
		bldr.ReportIssue("The CODEOWNERS file is empty. The files in the repository don't have any owner.")
		return bldr.Output(), nil
	}

	patterns := c.patternsToBeIgnored(in.CodeownersEntries)

	if err := c.trustWorkspaceIfNeeded(in.RepoDir); err != nil {
		return api.Output{}, err
	}

	statusOut, err := c.GitCheckStatus(in.RepoDir)
	if err != nil {
		return api.Output{}, err
	}
	if len(statusOut) != 0 {
		bldr.ReportIssue("git state is dirty: commit all changes before executing this check")
		return bldr.Output(), nil
	}

	defer func() {
		errReset := c.GitResetCurrentBranch(in.RepoDir)
		if err != nil {
			output = api.Output{}
			err = multierror.Append(err, errReset).ErrorOrNil()
		}
	}()

	err = c.AppendToGitignoreFile(in.RepoDir, patterns)
	if err != nil {
		return api.Output{}, err
	}

	err = c.GitRemoveIgnoredFiles(in.RepoDir)
	if err != nil {
		return api.Output{}, err
	}

	out, err := c.GitListFiles(in.RepoDir)
	if err != nil {
		return api.Output{}, err
	}

	lsOut := strings.TrimSpace(out)
	if lsOut != "" {
		lines := strings.Split(lsOut, "\n")
		msg := fmt.Sprintf("Found %d not owned files (skipped patterns: %q):\n%s", len(lines), c.skipPatternsList(), c.ListFormatFunc(lines))
		bldr.ReportIssue(msg)
	}

	return bldr.Output(), nil
}

func (c *NotOwnedFile) patternsToBeIgnored(entries []codeowners.Entry) []string {
	var patterns []string
	for _, entry := range entries {
		if _, found := c.skipPatterns[entry.Pattern]; found {
			continue
		}
		patterns = append(patterns, entry.Pattern)
	}

	return patterns
}

func (c *NotOwnedFile) AppendToGitignoreFile(repoDir string, patterns []string) error {
	f, err := os.OpenFile(path.Join(repoDir, ".gitignore"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	defer f.Close()

	content := strings.Builder{}
	// ensure we are starting from new line
	content.WriteString("\n")
	for _, p := range patterns {
		content.WriteString(fmt.Sprintf("%s\n", p))
	}

	_, err = f.WriteString(content.String())
	if err != nil {
		return err
	}
	return nil
}

func (c *NotOwnedFile) GitRemoveIgnoredFiles(repoDir string) error {
	gitrm := pipe.Script(
		pipe.ChDir(repoDir),
		pipe.Line(
			pipe.Exec("git", "ls-files", "-ci", "--exclude-standard", "-z"),
			pipe.Exec("xargs", "-0", "-r", "git", "rm", "--cached"),
		),
	)

	_, stderr, err := pipe.DividedOutput(gitrm)
	if err != nil {
		return errors.Wrap(err, string(stderr))
	}
	return nil
}

func (c *NotOwnedFile) GitCheckStatus(repoDir string) ([]byte, error) {
	gitstate := pipe.Script(
		pipe.ChDir(repoDir),
		pipe.Exec("git", "status", "--porcelain"),
	)

	out, stderr, err := pipe.DividedOutput(gitstate)
	if err != nil {
		return nil, errors.Wrap(err, string(stderr))
	}

	return out, nil
}

func (c *NotOwnedFile) GitResetCurrentBranch(repoDir string) error {
	gitreset := pipe.Script(
		pipe.ChDir(repoDir),
		pipe.Exec("git", "reset", "--hard"),
	)
	_, stderr, err := pipe.DividedOutput(gitreset)
	if err != nil {
		return errors.Wrap(err, string(stderr))
	}
	return nil
}

func (c *NotOwnedFile) GitListFiles(repoDir string) (string, error) {
	args := []string{"ls-files"}
	args = append(args, c.subDirectories...)

	gitls := pipe.Script(
		pipe.ChDir(repoDir),
		pipe.Exec("git", args...),
	)

	stdout, stderr, err := pipe.DividedOutput(gitls)
	if err != nil {
		return "", errors.Wrap(err, string(stderr))
	}

	return string(stdout), nil
}

func (c *NotOwnedFile) trustWorkspaceIfNeeded(repo string) error {
	if !c.trustWorkspace {
		return nil
	}

	gitadd := pipe.Exec("git", "config", "--global", "--add", "safe.directory", repo)
	_, stderr, err := pipe.DividedOutput(gitadd)
	if err != nil {
		return errors.Wrap(err, string(stderr))
	}

	return nil
}

func (c *NotOwnedFile) skipPatternsList() string {
	list := make([]string, 0, len(c.skipPatterns))
	for k := range c.skipPatterns {
		list = append(list, k)
	}
	return strings.Join(list, ",")
}

// ListFormatFunc is a basic formatter that outputs
// a bullet point list of the pattern.
func (c *NotOwnedFile) ListFormatFunc(es []string) string {
	points := make([]string, len(es))
	for i, err := range es {
		points[i] = fmt.Sprintf("            * %s", err)
	}

	return strings.Join(points, "\n")
}

// Name returns human-readable name of the validator
func (NotOwnedFile) Name() string {
	return "[Experimental] Not Owned File Checker"
}
