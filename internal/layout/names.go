// Package layout builds object keys from key templates and decides which keys
// the CLI may write or delete.
package layout

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Lower case only, because keys are case sensitive and URLs get retyped.
var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// ValidateName reports an error when name is not a valid file name or slug.
// what names the value in the error, such as "file name" or "slug".
func ValidateName(what, name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid %s %q: use lower case letters, digits, '.', '-' and '_', starting with a letter or digit", what, name)
	}
	return nil
}

// Slugify lower cases s, turns '/' and every character outside [a-z0-9._-]
// into '-', collapses runs of '-' and trims leading and trailing '-'.
func Slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		keep := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_'
		if !keep {
			if !lastDash {
				b.WriteByte('-')
			}
			lastDash = true
			continue
		}
		b.WriteRune(r)
		lastDash = false
	}
	return strings.Trim(b.String(), "-")
}

// NormalizeFileName turns the base name of path into a valid key file name:
// the stem goes through Slugify and the extension is lower cased. It returns
// an error when nothing valid is left, such as for ".png".
func NormalizeFileName(path string) (string, error) {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	name := Slugify(stem) + strings.ToLower(ext)
	if err := ValidateName("file name", name); err != nil {
		return "", fmt.Errorf("cannot normalize %q: %w; pass --as to name it", base, err)
	}
	return name, nil
}
