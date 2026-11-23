package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/url"
	"path"
	"strings"
	"text/tabwriter"
)

// Result is one object in command output.
type Result struct {
	Key         string `json:"key"`
	URL         string `json:"url"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// Content types for the files people usually upload, so the result does not
// depend on the host's mime tables.
var contentTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".mov":  "video/quicktime",
	".pdf":  "application/pdf",
	".txt":  "text/plain; charset=utf-8",
	".json": "application/json",
	".zip":  "application/zip",
}

// contentType returns the Content-Type for a file name, or "" when the
// extension is unknown.
func contentType(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if ext == "" {
		return ""
	}
	if ct, ok := contentTypes[ext]; ok {
		return ct
	}
	return mime.TypeByExtension(ext)
}

func publicURL(base, key string) string {
	segs := strings.Split(key, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.TrimRight(base, "/") + "/" + strings.Join(segs, "/")
}

func markdown(r Result) string {
	name := path.Base(r.Key)
	if strings.HasPrefix(r.ContentType, "image/") {
		alt := strings.TrimSuffix(name, path.Ext(name))
		return fmt.Sprintf("![%s](%s)", alt, r.URL)
	}
	return fmt.Sprintf("[%s](%s)", name, r.URL)
}

// writeLine prints one result for put, as it finishes.
func writeLine(w io.Writer, format string, r Result) {
	switch format {
	case FormatMD:
		_, _ = fmt.Fprintln(w, markdown(r))
	default:
		_, _ = fmt.Fprintln(w, r.URL)
	}
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// writeListing prints ls output: a size and URL table, Markdown, or JSON.
func writeListing(w io.Writer, format string, results []Result) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, results)
	case FormatMD:
		for _, r := range results {
			_, _ = fmt.Fprintln(w, markdown(r))
		}
		return nil
	default:
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
		for _, r := range results {
			_, _ = fmt.Fprintf(tw, "%d\t%s\t\n", r.Size, r.URL)
		}
		return tw.Flush()
	}
}
