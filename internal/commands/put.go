package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/hay-kot/prassets/internal/config"
	"github.com/hay-kot/prassets/internal/layout"
	"github.com/hay-kot/prassets/internal/store"
)

// Published keys never change content (put refuses to overwrite without
// --force), so browsers and CDNs may keep them for a year.
const cacheControl = "public, max-age=31536000, immutable"

func typeFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "type", Aliases: []string{"t"}, Usage: "asset type (see pra types)", Value: config.PRType},
		&cli.StringFlag{Name: "slug", Aliases: []string{"s"}, Usage: "slug for types whose key uses .Slug"},
	}
}

func newPutCmd(flags *Flags) *cli.Command {
	return &cli.Command{
		Name:      "put",
		Usage:     "upload files and print their public URLs",
		ArgsUsage: "<file>...",
		Description: `Uploads to the pr type for the current branch unless -t picks another type.
File names are normalized to lower case slugs; --as names a single file.
Every key is checked before the first upload, and existing keys are refused
unless --force is given.`,
		Flags: append(typeFlags(),
			&cli.BoolFlag{Name: "force", Usage: "replace existing keys; caches may keep the old copy for up to a year, so prefer a new file name"},
			&cli.StringFlag{Name: "as", Usage: "upload a single file under this name"},
		),
		Action: func(ctx context.Context, c *cli.Command) error {
			return runPut(ctx, c, flags)
		},
	}
}

type upload struct {
	path        string
	name        string
	key         string
	size        int64
	contentType string
}

func runPut(ctx context.Context, c *cli.Command, flags *Flags) error {
	files := c.Args().Slice()
	if len(files) == 0 {
		return usageErrorf("put needs at least one file")
	}
	as := c.String("as")
	if as != "" && len(files) != 1 {
		return usageErrorf("--as names a single file, but %d files were given", len(files))
	}

	s, err := openSession(ctx, flags)
	if err != nil {
		return err
	}
	typ, err := s.resolved.Type(c.String("type"))
	if err != nil {
		return &UsageError{Err: err}
	}
	if _, err := s.keyData(ctx, typ, c.String("slug"), "x"); err != nil {
		return err
	}

	uploads, err := planUploads(ctx, s, typ, c.String("slug"), as, files)
	if err != nil {
		return err
	}

	st, err := s.store(ctx)
	if err != nil {
		return err
	}

	force := c.Bool("force")
	if force {
		log.Warn().Msg("--force replaces existing keys; browsers and CDNs may serve the old copy for up to a year")
	} else {
		for _, u := range uploads {
			exists, err := st.Exists(ctx, u.key)
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("%s already exists; upload under a new name (for example --as %s) or pass --force to replace it", u.key, suggestName(u.name))
			}
		}
	}

	format := flags.Format
	var results []Result
	defer func() {
		if format == FormatJSON {
			if results == nil {
				results = []Result{}
			}
			_ = writeJSON(c.Root().Writer, results)
		}
	}()
	for _, u := range uploads {
		if err := putFile(ctx, st, u); err != nil {
			return err
		}
		r := Result{Key: u.key, URL: s.url(u.key), Size: u.size, ContentType: u.contentType}
		results = append(results, r)
		if format != FormatJSON {
			writeLine(c.Root().Writer, format, r)
		}
	}
	return nil
}

// planUploads names, keys and checks every file before anything is sent.
func planUploads(ctx context.Context, s *session, typ config.Type, slug, as string, files []string) ([]upload, error) {
	uploads := make([]upload, 0, len(files))
	byName := map[string]string{}
	for _, f := range files {
		name := as
		if name != "" {
			if err := layout.ValidateName("--as name", name); err != nil {
				return nil, &UsageError{Err: err}
			}
		} else {
			var err error
			if name, err = layout.NormalizeFileName(f); err != nil {
				return nil, err
			}
			if base := baseName(f); base != name {
				log.Info().Str("file", f).Str("as", name).Msg("renamed")
			}
		}
		if prev, dup := byName[name]; dup {
			return nil, fmt.Errorf("%s and %s both upload as %s; rename one or upload them separately with --as", prev, f, name)
		}
		byName[name] = f

		info, err := os.Stat(f)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s is not a regular file", f)
		}
		ct := contentType(name)
		if ct == "" {
			return nil, fmt.Errorf("%s: unknown file extension, cannot pick a Content-Type", f)
		}

		data, err := s.keyData(ctx, typ, slug, name)
		if err != nil {
			return nil, err
		}
		key, err := layout.BuildKey(s.resolved.Prefix, typ.Template, data)
		if err != nil {
			return nil, err
		}
		uploads = append(uploads, upload{path: f, name: name, key: key, size: info.Size(), contentType: ct})
	}
	return uploads, nil
}

func baseName(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

func suggestName(name string) string {
	dot := strings.LastIndex(name, ".")
	if dot <= 0 {
		return name + "-2"
	}
	return name[:dot] + "-2" + name[dot:]
}

func putFile(ctx context.Context, st *store.Store, u upload) error {
	f, err := os.Open(u.path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := st.Put(ctx, u.key, f, u.size, store.PutOptions{ContentType: u.contentType, CacheControl: cacheControl}); err != nil {
		return err
	}
	log.Debug().Str("key", u.key).Msg("uploaded")
	return nil
}
