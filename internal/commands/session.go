package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/hay-kot/prassets/internal/config"
	"github.com/hay-kot/prassets/internal/gitctx"
	"github.com/hay-kot/prassets/internal/layout"
	"github.com/hay-kot/prassets/internal/paths"
	"github.com/hay-kot/prassets/internal/secrets"
	"github.com/hay-kot/prassets/internal/store"
)

func (f *Flags) configPath() (string, error) {
	if f.ConfigFile != "" {
		return paths.Expand(f.ConfigFile)
	}
	return config.DefaultPath()
}

func (f *Flags) loadGlobal() (*config.Global, error) {
	p, err := f.configPath()
	if err != nil {
		return nil, err
	}
	return config.Load(p)
}

// session is the resolved state for one command run in one repo.
type session struct {
	flags    *Flags
	global   *config.Global
	resolved *config.Resolved
	// workTree is the git work tree of the current directory, or "".
	workTree string
}

func openSession(ctx context.Context, flags *Flags) (*session, error) {
	global, err := flags.loadGlobal()
	if err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	workTree, treeErr := gitctx.TopLevel(ctx, cwd)
	if treeErr != nil {
		workTree = ""
	}

	var cwdRepo *gitctx.Repo
	if workTree != "" {
		if u, err := gitctx.RemoteURL(ctx, workTree, flags.Remote); err == nil {
			if r, err := gitctx.ParseRemoteURL(u); err == nil {
				cwdRepo = &r
			}
		}
	}

	var repo gitctx.Repo
	switch {
	case flags.Repo != "":
		if repo, err = gitctx.ParseID(flags.Repo); err != nil {
			return nil, &UsageError{Err: fmt.Errorf("--repo: %w", err)}
		}
	case workTree == "":
		return nil, fmt.Errorf("not in a git repository (%w); pass --repo host/owner/name", treeErr)
	default:
		u, err := gitctx.RemoteURL(ctx, workTree, flags.Remote)
		if err != nil {
			return nil, err
		}
		if repo, err = gitctx.ParseRemoteURL(u); err != nil {
			return nil, err
		}
	}

	// .prassets.yaml belongs to the checkout, so only apply it when the
	// checkout is the repo being resolved.
	var repoFile *config.RepoFile
	repoFilePath := ""
	if workTree != "" && cwdRepo != nil && cwdRepo.ID() == repo.ID() {
		p := filepath.Join(workTree, config.RepoFileName)
		if repoFile, err = config.LoadRepoFile(p); err != nil {
			return nil, err
		}
		if repoFile != nil {
			repoFilePath = p
		}
	}

	resolved, err := global.Resolve(repo, repoFile, repoFilePath)
	if err != nil {
		return nil, err
	}
	log.Debug().Str("repo", repo.ID()).Str("entry", resolved.Entry).Str("store", resolved.StoreName).Msg("resolved repo")
	return &session{flags: flags, global: global, resolved: resolved, workTree: workTree}, nil
}

// branch returns the slugified branch: --branch, else the checked-out branch.
func (s *session) branch(ctx context.Context) (string, error) {
	if s.flags.Branch != "" {
		return layout.Slugify(s.flags.Branch), nil
	}
	if s.workTree == "" {
		return "", usageErrorf("not in a git repository; pass --branch")
	}
	b, err := gitctx.CurrentBranch(ctx, s.workTree)
	if err != nil {
		return "", err
	}
	return layout.Slugify(b), nil
}

// keyData returns the template data for typ, reading the branch only when
// the template needs it.
func (s *session) keyData(ctx context.Context, typ config.Type, slug, file string) (layout.Data, error) {
	switch {
	case typ.Template.UsesSlug() && slug == "":
		return layout.Data{}, usageErrorf("type %s needs --slug (key %s)", typ.Name, typ.Template.Text())
	case !typ.Template.UsesSlug() && slug != "":
		return layout.Data{}, usageErrorf("type %s takes no --slug (key %s)", typ.Name, typ.Template.Text())
	}
	branch := ""
	if typ.Template.UsesBranch() {
		var err error
		if branch, err = s.branch(ctx); err != nil {
			return layout.Data{}, err
		}
	}
	return s.resolved.Data(branch, slug, file), nil
}

func (s *session) url(key string) string {
	return publicURL(s.resolved.Store.PublicURL, key)
}

// renderSecrets renders the credentials of the repo's store only.
func renderSecrets(ctx context.Context, r *config.Resolved) (map[string]string, error) {
	resolver := secrets.NewResolver()
	out := map[string]string{}
	for _, f := range r.Store.SecretFields() {
		if f.Template == "" {
			continue
		}
		t, err := secrets.Parse(f.Template)
		if err != nil {
			return nil, fmt.Errorf("store %s: %s: %w", r.StoreName, f.Name, err)
		}
		v, err := resolver.Render(ctx, t)
		if err != nil {
			return nil, fmt.Errorf("store %s: %s: %w", r.StoreName, f.Name, err)
		}
		out[f.Name] = v
	}
	return out, nil
}

func (s *session) store(ctx context.Context) (*store.Store, error) {
	creds, err := renderSecrets(ctx, s.resolved)
	if err != nil {
		return nil, err
	}
	st := s.resolved.Store
	return store.New(store.Options{
		Endpoint:     st.Endpoint,
		Bucket:       st.Bucket,
		Region:       st.RegionOrDefault(),
		Insecure:     st.Insecure,
		AccessKey:    creds["access_key"],
		SecretKey:    creds["secret_key"],
		SessionToken: creds["session_token"],
	})
}
