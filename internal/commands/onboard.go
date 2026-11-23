package commands

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"text/template"

	"github.com/urfave/cli/v3"

	"github.com/hay-kot/prassets/internal/config"
)

//go:embed onboard.md.tmpl
var onboardGuide string

// The guide quotes key templates such as {{ .Branch }}, so it uses [[ ]].
var onboardTemplate = template.Must(template.New("onboard").Delims("[[", "]]").Parse(onboardGuide))

// Repo file states in onboard output.
const (
	repoFilePresent       = "present"
	repoFileAbsent        = "absent"
	repoFileInvalid       = "invalid"
	repoFileNotApplicable = "not_applicable"
)

type onboardState struct {
	ConfigPath     string     `json:"config_path"`
	ConfigStatus   string     `json:"config_status"`
	ConfigError    string     `json:"config_error,omitempty"`
	Repo           string     `json:"repo,omitempty"`
	RepoError      string     `json:"repo_error,omitempty"`
	RepoFile       string     `json:"repo_file,omitempty"`
	RepoFileStatus string     `json:"repo_file_status"`
	RepoFileError  string     `json:"repo_file_error,omitempty"`
	Entry          string     `json:"entry,omitempty"`
	EntryError     string     `json:"entry_error,omitempty"`
	Store          string     `json:"store,omitempty"`
	Prefix         string     `json:"prefix"`
	Types          []typeInfo `json:"types"`
	NextStep       string     `json:"next_step"`
}

func newOnboardCmd(flags *Flags) *cli.Command {
	return &cli.Command{
		Name:  "onboard",
		Usage: "explain how pra is set up for this repo and how to onboard it (for agents and new users)",
		Description: `Prints the state of this checkout (global config, repo id, entry, .prassets.yaml,
types), the next step, where each setting belongs, and the rules agents follow.
It never fails because of missing or broken config; it reports it instead.
--format json prints only the state.`,
		Action: func(ctx context.Context, c *cli.Command) error {
			st := gatherOnboardState(ctx, flags)
			if flags.Format == FormatJSON {
				return writeJSON(c.Root().Writer, st)
			}
			return onboardTemplate.Execute(c.Root().Writer, st)
		},
	}
}

func gatherOnboardState(ctx context.Context, flags *Flags) onboardState {
	st := onboardState{Types: []typeInfo{}, RepoFileStatus: repoFileNotApplicable}

	path, err := flags.configPath()
	if err != nil {
		st.ConfigStatus, st.ConfigError = "invalid", err.Error()
	}
	st.ConfigPath = path
	var global *config.Global
	if err == nil {
		global, err = config.Load(path)
		switch {
		case errors.Is(err, config.ErrNotFound):
			st.ConfigStatus = "missing"
		case err != nil:
			st.ConfigStatus, st.ConfigError = "invalid", err.Error()
		default:
			st.ConfigStatus = "ok"
		}
	}

	co, err := detectCheckout(ctx, flags)
	if err != nil {
		st.RepoError = err.Error()
	} else {
		st.Repo = co.repo.ID()
	}

	var repoFile *config.RepoFile
	repoFilePath := ""
	if co.repoFilePath != "" {
		st.RepoFile = co.repoFilePath
		repoFile, repoFilePath, err = co.loadRepoFile()
		switch {
		case err != nil:
			st.RepoFileStatus, st.RepoFileError = repoFileInvalid, err.Error()
		case repoFile != nil:
			st.RepoFileStatus = repoFilePresent
		default:
			st.RepoFileStatus = repoFileAbsent
		}
	}

	if global != nil && st.Repo != "" && st.RepoFileStatus != repoFileInvalid {
		resolved, err := global.Resolve(co.repo, repoFile, repoFilePath)
		if err != nil {
			st.EntryError = err.Error()
		} else {
			st.Entry, st.Store, st.Prefix = resolved.Entry, resolved.StoreName, resolved.Prefix
			st.Types = (&session{flags: flags, global: global, resolved: resolved, workTree: co.workTree}).typeInfos()
		}
	}
	st.NextStep = nextOnboardStep(st)
	return st
}

func nextOnboardStep(st onboardState) string {
	switch {
	case st.ConfigStatus == "missing":
		return fmt.Sprintf("There is no global config at %s. Ask the user to run `pra config init` and fill in a store. The user owns that file, because it holds credentials.", st.ConfigPath)
	case st.ConfigStatus == "invalid":
		return "Fix the global config error above. `pra schema global` prints the schema. Ask the user first, because the file holds credentials."
	case st.RepoError != "":
		return "pra cannot tell which repo this is. Run it inside a checkout with an origin remote, or pass --repo host/owner/name."
	case st.RepoFileStatus == repoFileInvalid:
		return "Fix .prassets.yaml (error above). `pra schema repo` prints its schema."
	case st.EntryError != "":
		return fmt.Sprintf("The global config has no entry for %s and no default. Ask the user to add one under repos: (see \"Map the repo to a store\").", st.Repo)
	case st.Entry == "default":
		return "The repo uses the default entry and works as is. Add .prassets.yaml only if the repo needs types beyond pr. Then run `pra config check` to test credentials and bucket access."
	default:
		return "The repo has its own entry. Run `pra config check` to test credentials and bucket access."
	}
}
