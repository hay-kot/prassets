package commands

import (
	"context"
	"net/url"
	"strings"

	"github.com/hay-kot/prassets/internal/config"
	"github.com/hay-kot/prassets/internal/layout"
)

// resolveKeyArg turns a get or rm argument into a key. It accepts a full key,
// a public URL that pra printed, or a bare file name, which resolves against
// the current branch's PR area.
func (s *session) resolveKeyArg(ctx context.Context, arg string) (string, error) {
	base := strings.TrimRight(s.resolved.Store.PublicURL, "/") + "/"
	if rest, ok := strings.CutPrefix(arg, base); ok {
		key, err := url.PathUnescape(rest)
		if err != nil {
			return "", usageErrorf("invalid URL %q: %v", arg, err)
		}
		return key, layout.CheckKey(key)
	}
	if !strings.Contains(arg, "/") {
		typ, err := s.resolved.Type(config.PRType)
		if err != nil {
			return "", err
		}
		data, err := s.keyData(ctx, typ, "", arg)
		if err != nil {
			return "", err
		}
		return layout.BuildKey(s.resolved.Prefix, typ.Template, data)
	}
	if err := layout.CheckKey(arg); err != nil {
		return "", &UsageError{Err: err}
	}
	return arg, nil
}
