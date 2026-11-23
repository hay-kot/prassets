package layout

import "regexp"

// Matcher decides which keys belong to one of a repo's types and so may be
// replaced or deleted. Every other key is read-only to the CLI.
type Matcher struct {
	patterns []*regexp.Regexp
}

// NewMatcher builds a Matcher for the templates under prefix.
func NewMatcher(prefix, host, owner, repo string, templates ...*Template) (*Matcher, error) {
	m := &Matcher{patterns: make([]*regexp.Regexp, 0, len(templates))}
	for _, t := range templates {
		p, err := t.Pattern(prefix, host, owner, repo)
		if err != nil {
			return nil, err
		}
		m.patterns = append(m.patterns, p)
	}
	return m, nil
}

// Writable reports whether key matches one of the templates.
func (m *Matcher) Writable(key string) bool {
	if CheckKey(key) != nil {
		return false
	}
	for _, p := range m.patterns {
		if p.MatchString(key) {
			return true
		}
	}
	return false
}
