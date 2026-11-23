package layout

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"text/template/parse"
)

// Data is what key templates can read. Branch is already slugified.
type Data struct {
	Host   string
	Owner  string
	Repo   string
	Branch string
	Slug   string
	File   string
}

func (d Data) fields() map[string]string {
	return map[string]string{
		fieldHost:   d.Host,
		fieldOwner:  d.Owner,
		fieldRepo:   d.Repo,
		fieldBranch: d.Branch,
		fieldSlug:   d.Slug,
		fieldFile:   d.File,
	}
}

const (
	fieldHost   = "Host"
	fieldOwner  = "Owner"
	fieldRepo   = "Repo"
	fieldBranch = "Branch"
	fieldSlug   = "Slug"
	fieldFile   = "File"
)

// BuiltinPR is the key template of the built-in pr type.
const BuiltinPR = "pr/{{ .Branch }}/{{ .File }}"

// Key templates come from .prassets.yaml, which is untrusted repo content, so
// they only ever get pure functions. Never add env, cmd, vault or file here.
var keyFuncs = template.FuncMap{
	"lower":   strings.ToLower,
	"slugify": Slugify,
}

// Template is a parsed key template.
type Template struct {
	text   string
	tmpl   *template.Template
	fields map[string]bool
}

// ParseKey parses a key template for a type. The template must reference
// .File and may only reference the fields of Data.
func ParseKey(text string) (*Template, error) {
	t, err := parseTemplate(text)
	if err != nil {
		return nil, err
	}
	if !t.fields[fieldFile] {
		return nil, fmt.Errorf("key template %q must reference .File", text)
	}
	sample := Data{Host: "example.com", Owner: "owner", Repo: "repo", Branch: "branch", Slug: "slug", File: "file.png"}
	if !t.UsesSlug() {
		sample.Slug = ""
	}
	if _, err := t.Render(sample); err != nil {
		return nil, err
	}
	return t, nil
}

// ParsePrefix parses a prefix template. A prefix names the area of the bucket
// a repo owns, so it may only reference .Host, .Owner and .Repo.
func ParsePrefix(text string) (*Template, error) {
	t, err := parseTemplate(text)
	if err != nil {
		return nil, err
	}
	for f := range t.fields {
		if f != fieldHost && f != fieldOwner && f != fieldRepo {
			return nil, fmt.Errorf("prefix template %q may only reference .Host, .Owner and .Repo, not .%s", text, f)
		}
	}
	prefix, err := t.execute(Data{Host: "example.com", Owner: "owner", Repo: "repo", Branch: "", Slug: "", File: ""})
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(prefix, "/") || strings.Contains(prefix, "//") {
		return nil, fmt.Errorf("prefix template %q renders %q, which must not start with '/' or contain '//'", text, prefix)
	}
	return t, nil
}

func parseTemplate(text string) (*Template, error) {
	tmpl, err := template.New("key").Option("missingkey=error").Funcs(keyFuncs).Parse(text)
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", text, err)
	}
	fields := map[string]bool{}
	for _, tt := range tmpl.Templates() {
		if tt.Tree == nil {
			continue
		}
		if err := collectFields(tt.Root, fields); err != nil {
			return nil, fmt.Errorf("template %q: %w", text, err)
		}
	}
	allowed := Data{}.fields()
	for f := range fields {
		if _, ok := allowed[f]; !ok {
			return nil, fmt.Errorf("template %q references unknown field .%s (one of .Host, .Owner, .Repo, .Branch, .Slug, .File)", text, f)
		}
	}
	return &Template{text: text, tmpl: tmpl, fields: fields}, nil
}

// collectFields walks the parse tree and records every top-level field the
// template reads, such as Slug for {{ .Slug }} or {{ $.Slug | lower }}.
func collectFields(node parse.Node, fields map[string]bool) error {
	switch n := node.(type) {
	case nil:
		return nil
	case *parse.ListNode:
		if n == nil {
			return nil
		}
		for _, c := range n.Nodes {
			if err := collectFields(c, fields); err != nil {
				return err
			}
		}
	case *parse.ActionNode:
		return collectFields(n.Pipe, fields)
	case *parse.PipeNode:
		if n == nil {
			return nil
		}
		for _, c := range n.Cmds {
			if err := collectFields(c, fields); err != nil {
				return err
			}
		}
	case *parse.CommandNode:
		for _, a := range n.Args {
			if err := collectFields(a, fields); err != nil {
				return err
			}
		}
	case *parse.FieldNode:
		fields[n.Ident[0]] = true
	case *parse.VariableNode:
		if len(n.Ident) > 1 && n.Ident[0] == "$" {
			fields[n.Ident[1]] = true
		}
	case *parse.ChainNode:
		return collectFields(n.Node, fields)
	case *parse.IfNode:
		return collectBranch(&n.BranchNode, fields)
	case *parse.RangeNode:
		return collectBranch(&n.BranchNode, fields)
	case *parse.WithNode:
		return collectBranch(&n.BranchNode, fields)
	case *parse.TemplateNode:
		return collectFields(n.Pipe, fields)
	case *parse.DotNode:
		return errors.New("bare {{ . }} is not allowed; reference a field such as .File")
	default:
		// Text, strings, numbers, booleans, identifiers and comments read no data.
	}
	return nil
}

func collectBranch(n *parse.BranchNode, fields map[string]bool) error {
	for _, c := range []parse.Node{n.Pipe, n.List, n.ElseList} {
		if err := collectFields(c, fields); err != nil {
			return err
		}
	}
	return nil
}

// Text returns the template source.
func (t *Template) Text() string { return t.text }

// UsesSlug reports whether the template references .Slug.
func (t *Template) UsesSlug() bool { return t.fields[fieldSlug] }

// UsesBranch reports whether the template references .Branch.
func (t *Template) UsesBranch() bool { return t.fields[fieldBranch] }

func (t *Template) execute(d Data) (string, error) {
	var b strings.Builder
	if err := t.tmpl.Execute(&b, d.fields()); err != nil {
		return "", fmt.Errorf("render template %q: %w", t.text, err)
	}
	return b.String(), nil
}

// Render renders the key template for d. It validates the slug and file name,
// refuses a slug for a template without .Slug, and checks that the key ends
// with the file name.
func (t *Template) Render(d Data) (string, error) {
	if err := ValidateName("file name", d.File); err != nil {
		return "", err
	}
	switch {
	case t.UsesSlug() && d.Slug == "":
		return "", fmt.Errorf("key template %q needs a slug; pass --slug", t.text)
	case !t.UsesSlug() && d.Slug != "":
		return "", fmt.Errorf("key template %q takes no slug", t.text)
	case t.UsesSlug():
		if err := ValidateName("slug", d.Slug); err != nil {
			return "", err
		}
	}
	if t.UsesBranch() {
		if err := validateBranch(d.Branch); err != nil {
			return "", err
		}
	}
	key, err := t.execute(d)
	if err != nil {
		return "", err
	}
	if key != d.File && !strings.HasSuffix(key, "/"+d.File) {
		return "", fmt.Errorf("key template %q renders %q, which must end with /%s", t.text, key, d.File)
	}
	return key, nil
}

var branchPattern = regexp.MustCompile("^" + segmentPattern + "$")

func validateBranch(branch string) error {
	if branch == "" {
		return errors.New("a branch is required; check out a branch or pass --branch")
	}
	if !branchPattern.MatchString(branch) || branch == "." || branch == ".." {
		return fmt.Errorf("invalid branch %q: it must be slugified to lower case letters, digits, '.', '-' and '_'", branch)
	}
	return nil
}

// CheckKey refuses keys that could escape the layout or confuse S3 clients:
// a leading '/', an empty segment, or a '.' or '..' segment.
func CheckKey(key string) error {
	if key == "" {
		return errors.New("empty key")
	}
	if strings.HasPrefix(key, "/") {
		return fmt.Errorf("key %q must not start with '/'", key)
	}
	if strings.Contains(key, "//") {
		return fmt.Errorf("key %q must not contain '//'", key)
	}
	for seg := range strings.SplitSeq(key, "/") {
		if seg == "." || seg == ".." {
			return fmt.Errorf("key %q must not have a '.' or '..' segment", key)
		}
	}
	return nil
}

// BuildKey renders prefix + template for d and checks the result.
func BuildKey(prefix string, t *Template, d Data) (string, error) {
	rest, err := t.Render(d)
	if err != nil {
		return "", err
	}
	key := prefix + rest
	if err := CheckKey(key); err != nil {
		return "", err
	}
	return key, nil
}

// RenderPrefix renders a prefix template for a repo.
func RenderPrefix(t *Template, host, owner, repo string) (string, error) {
	return t.execute(Data{Host: host, Owner: owner, Repo: repo, Branch: "", Slug: "", File: ""})
}

// Sentinels stand in for the variable segments while a template renders to a
// pattern. They are lower case alphanumerics, so lower and slugify keep them.
const (
	sentinelBranch = "zq7branchsentinelzq7"
	sentinelSlug   = "zq7slugsentinelzq7"
	sentinelFile   = "zq7filesentinelzq7.png"
)

const (
	segmentPattern     = `[a-z0-9._-]+`
	nameSegmentPattern = `[a-z0-9][a-z0-9._-]*`
)

// Pattern returns a regexp that matches every key this template can write
// under prefix for the given repo: static text is literal, and .Branch, .Slug
// and .File each match one path segment of [a-z0-9._-]+ (.Slug and .File
// must also start with a letter or digit).
func (t *Template) Pattern(prefix, host, owner, repo string) (*regexp.Regexp, error) {
	d := Data{Host: host, Owner: owner, Repo: repo, Branch: sentinelBranch, Slug: "", File: sentinelFile}
	if t.UsesSlug() {
		d.Slug = sentinelSlug
	}
	rendered, err := t.execute(d)
	if err != nil {
		return nil, err
	}
	expr := regexp.QuoteMeta(prefix + rendered)
	expr = strings.ReplaceAll(expr, regexp.QuoteMeta(sentinelBranch), segmentPattern)
	// Slugs and file names pass ValidateName before any write, so match the
	// same stricter pattern here: "post=.." is not a key put could create.
	for _, s := range []string{sentinelSlug, sentinelFile} {
		expr = strings.ReplaceAll(expr, regexp.QuoteMeta(s), nameSegmentPattern)
	}
	return regexp.Compile("^" + expr + "$")
}

// Dir returns the part of the rendered key before the file name, with a
// trailing '/' (or "" when the key is just the file). ls uses it as a listing
// prefix.
func (t *Template) Dir(d Data) (string, error) {
	d.File = sentinelFile
	key, err := t.Render(d)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(key, sentinelFile), nil
}
