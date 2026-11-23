// Package gitctx reads the repo identity and current branch from git.
package gitctx

import (
	"fmt"
	"net/url"
	"strings"
)

// Repo identifies a repository as host/owner/name.
type Repo struct {
	Host  string
	Owner string
	Name  string
}

// ID returns host/owner/name.
func (r Repo) ID() string {
	return r.Host + "/" + r.Owner + "/" + r.Name
}

// ParseID parses a repo id such as "github.com/hay-kot/prassets". The owner
// may hold several segments, as GitLab subgroups do.
func ParseID(id string) (Repo, error) {
	id = strings.Trim(strings.ToLower(strings.TrimSpace(id)), "/")
	id = strings.TrimSuffix(id, ".git")
	parts := strings.Split(id, "/")
	if len(parts) < 3 {
		return Repo{}, fmt.Errorf("invalid repo id %q: want host/owner/name", id)
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return Repo{}, fmt.Errorf("invalid repo id %q: want host/owner/name", id)
		}
	}
	return Repo{
		Host:  parts[0],
		Owner: strings.Join(parts[1:len(parts)-1], "/"),
		Name:  parts[len(parts)-1],
	}, nil
}

// ParseRemoteURL normalizes an SSH, scp-style or HTTPS remote URL to a Repo.
func ParseRemoteURL(raw string) (Repo, error) {
	raw = strings.TrimSpace(raw)
	var host, path string
	switch {
	case strings.Contains(raw, "://"):
		u, err := url.Parse(raw)
		if err != nil {
			return Repo{}, fmt.Errorf("parse remote url: %w", err)
		}
		host, path = u.Hostname(), u.Path
	case strings.Contains(raw, ":"):
		// scp-style: [user@]host:owner/name.git
		before, after, _ := strings.Cut(raw, ":")
		if i := strings.LastIndex(before, "@"); i >= 0 {
			before = before[i+1:]
		}
		host, path = before, after
	default:
		return Repo{}, fmt.Errorf("unsupported remote url %q; pass --repo host/owner/name", raw)
	}
	if host == "" {
		return Repo{}, fmt.Errorf("remote url %q has no host; pass --repo host/owner/name", raw)
	}
	repo, err := ParseID(host + "/" + strings.Trim(path, "/"))
	if err != nil {
		return Repo{}, fmt.Errorf("remote url %q: %w", raw, err)
	}
	return repo, nil
}
