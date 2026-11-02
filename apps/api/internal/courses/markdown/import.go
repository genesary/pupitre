package markdown

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/definition"
)

// Module types whose body does not come from the document: their content
// is a URL, a git file, or an inline question list.
const (
	moduleTypeVideo   = "video"
	moduleTypeImage   = "image"
	moduleTypeQuiz    = "quiz"
	moduleTypeModules = "modules"
)

// frontmatterFence delimits the YAML header at the top of a document.
const frontmatterFence = "---"

// directiveMarker names the HTML comment that carries a module's
// non-markdown attributes.
const directiveMarker = "pupitre"

// commentOpen and commentClose delimit that HTML comment.
const (
	commentOpen  = "<!--"
	commentClose = "-->"
)

// fallbackModuleName names a module the document left unnamed.
const fallbackModuleName = "Introduction"

// errNoSlug is returned for a document that names no slug and whose title
// is empty, leaving nothing to derive one from.
var errNoSlug = errors.New("no slug: give one in the request or a title/slug in the frontmatter")

// errUnterminatedFrontmatter is returned for a document that opens a YAML
// header and never closes it.
var errUnterminatedFrontmatter = errors.New("frontmatter opened with --- is never closed")

// errUnterminatedDirective is returned for a module directive comment that
// is never closed.
var errUnterminatedDirective = errors.New("<!--pupitre block is never closed with -->")

// Options controls how Import reads a document.
type Options struct {
	// Split names the heading level that starts a new module. Empty falls
	// back to the document's own `split` frontmatter key, then to
	// SplitNone — the whole document as one module.
	Split string
	// Slug overrides the slug the document declares.
	Slug string
}

// Result is the course a document decodes to, together with anything the
// importer chose to ignore along the way.
type Result struct {
	Slug     string            `json:"slug"`
	Spec     definition.Course `json:"spec"`
	Warnings []string          `json:"warnings,omitempty"`
	// SpecFromDocument reports whether the document carried frontmatter,
	// so a caller merging into an existing course knows whether Spec's
	// course-level fields were authored or defaulted.
	SpecFromDocument bool `json:"specFromDocument"`
}

// frontmatter is the YAML header of a course document: the course fields
// themselves, plus the slug to store them under and the split level the
// body was written at.
type frontmatter struct {
	definition.Course

	Slug  string `json:"slug,omitempty"`
	Split string `json:"split,omitempty"`
}

// Import decodes a markdown document into a course definition.
func Import(document []byte, opts Options) (Result, error) {
	body, header, present, err := splitFrontmatter(normalize(document))
	if err != nil {
		return Result{}, err
	}

	level, err := resolveSplit(opts.Split, header.Split)
	if err != nil {
		return Result{}, err
	}

	modules, warnings, err := modulesFrom(sections(body, level), header.Title)
	if err != nil {
		return Result{}, err
	}

	// Only hint at a split when none was chosen at all. An explicit
	// `split: none` (in the request or the frontmatter) is a decision, not
	// an oversight, so it draws no warning.
	if level == 0 && strings.TrimSpace(opts.Split) == "" && strings.TrimSpace(header.Split) == "" {
		warnings = append(splitHint(body), warnings...)
	}

	result := Result{Spec: header.Course, SpecFromDocument: present}
	result.Spec.Modules = append(result.Spec.Modules, modules...)
	result.Warnings = warnings

	result.Slug = firstNonEmpty(opts.Slug, header.Slug, courseSlug(result.Spec.Title))
	if result.Slug == "" {
		return Result{}, errNoSlug
	}

	if len(result.Spec.Modules) == 0 {
		result.Warnings = append(result.Warnings, "the document produced no module")
	}

	return result, nil
}

// splitHint warns when a document was read as a single module even though
// its body is cut into sections by a run of same-level headings — the
// shape an author gets by pasting several "## …" blocks without setting a
// split level or carrying one in frontmatter. It returns at most one
// warning, naming the shallowest heading level that would split the
// document into more than one module.
func splitHint(body string) []string {
	// A level only splits the document if it appears at least twice: a
	// single heading of that level leaves one module either way.
	const splittingHeadings = 2

	counts := headingCounts(body)

	for level := 1; level <= maxHeadingLevel; level++ {
		if counts[level] < splittingHeadings {
			continue
		}

		name := SplitName(level)

		return []string{fmt.Sprintf(
			"no split level was set, so the whole document became one module; it has %d %s headings — set split to %s to cut it into %d modules",
			counts[level], name, name, counts[level],
		)}
	}

	return nil
}

// resolveSplit picks the split level to cut the body at, preferring the
// caller's choice over the one recorded in the document.
func resolveSplit(requested, declared string) (int, error) {
	for _, candidate := range []string{requested, declared} {
		if strings.TrimSpace(candidate) == "" {
			continue
		}

		level, ok := SplitLevel(candidate)
		if !ok {
			return 0, fmt.Errorf("unknown split level %q: expected none, h1, h2, h3, h4, h5 or h6", candidate) //nolint:err113 // the rejected value belongs in the message
		}

		return level, nil
	}

	return 0, nil
}

// splitFrontmatter separates a document's YAML header from its body,
// reporting whether a header was present at all.
func splitFrontmatter(document string) (body string, header frontmatter, present bool, err error) { //nolint:nonamedreturns // named for gocritic(unnamedResult)
	lines := strings.Split(document, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], " \t") != frontmatterFence {
		return document, header, false, nil
	}

	for index := 1; index < len(lines); index++ {
		if strings.TrimRight(lines[index], " \t") != frontmatterFence {
			continue
		}

		err := definition.DecodeYAML([]byte(strings.Join(lines[1:index], "\n")), &header)
		if err != nil {
			return "", header, false, fmt.Errorf("frontmatter: %w", err)
		}

		return strings.Join(lines[index+1:], "\n"), header, true, nil
	}

	return "", header, false, errUnterminatedFrontmatter
}

// modulesFrom converts a document's sections into modules. The leading
// section is dropped when it holds nothing but the document's own title.
func modulesFrom(parts []section, docTitle string) ([]content.Module, []string, error) {
	modules := make([]content.Module, 0, len(parts))

	var warnings []string

	for _, part := range parts {
		name, lines := part.Heading, part.Lines
		if name == "" {
			name, lines = takeLeadingHeading(lines)
		}

		module, warns, err := buildModule(name, lines, docTitle)
		if err != nil {
			return nil, nil, err
		}

		if module == nil {
			continue
		}

		modules = append(modules, *module)
		warnings = append(warnings, warns...)
	}

	return modules, warnings, nil
}

// takeLeadingHeading pulls a heading off the front of a section that has
// none of its own — the document title above the first split heading, or
// the single heading of an unsplit document — and returns the lines left
// under it.
func takeLeadingHeading(lines []string) (name string, rest []string) { //nolint:nonamedreturns // named for gocritic(unnamedResult)
	for index, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		depth, text := heading(line)
		if depth == 0 {
			return "", lines
		}

		return text, lines[index+1:]
	}

	return "", lines
}

// buildModule assembles one module from a section's name, directive and
// markdown body. It returns a nil module for a section that holds nothing
// at all, which is how the blank run before the first heading disappears.
func buildModule(name string, lines []string, docTitle string) (*content.Module, []string, error) {
	payload, rest, err := takeDirective(lines)
	if err != nil {
		return nil, nil, err
	}

	body := trimBlank(rest)
	if payload == nil && body == "" && name == "" {
		return nil, nil, nil
	}

	module := content.Module{}

	if payload != nil {
		err = definition.DecodeYAML(payload, &module)
		if err != nil {
			return nil, nil, fmt.Errorf("module %q directive: %w", firstNonEmpty(name, docTitle), err)
		}
	}

	if module.Name == "" {
		module.Name = firstNonEmpty(name, docTitle, fallbackModuleName)
	}

	if module.Type == "" {
		module.Type = content.ModuleTypeText
	}

	return &module, attachBody(&module, body), nil
}

// attachBody puts the section's markdown on the module unless the module
// already says its content comes from somewhere else, and returns a
// warning for every body it had to drop.
func attachBody(module *content.Module, body string) []string {
	if body == "" {
		if module.Type == content.ModuleTypeText && !module.HasGitContent() {
			return []string{fmt.Sprintf("module %q has no content", module.Name)}
		}

		return nil
	}

	reason := bodyRejection(*module)
	if reason == "" {
		module.InlineContent = body

		return nil
	}

	return []string{fmt.Sprintf("module %q: markdown body ignored, %s", module.Name, reason)}
}

// bodyRejection explains why a module cannot take the markdown written
// under its heading, or returns an empty string when it can.
func bodyRejection(module content.Module) string {
	switch {
	case module.HasGitContent():
		return "its content comes from the git source it declares"
	case module.Type == moduleTypeQuiz && len(module.Questions) > 0:
		return "its questions are declared in the directive"
	case module.Type == moduleTypeModules:
		return "a modules index is read from git"
	case module.Type == moduleTypeVideo || module.Type == moduleTypeImage:
		return "a " + module.Type + " module is served from its src URL"
	default:
		return ""
	}
}

// takeDirective splits a section into the YAML of its leading
// <!--pupitre …--> comment (nil when it has none) and the lines below it.
func takeDirective(lines []string) (directive []byte, rest []string, err error) { //nolint:nonamedreturns // named for gocritic(unnamedResult)
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	if start == len(lines) || !opensDirective(lines[start]) {
		return nil, lines, nil
	}

	for index := start + 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) != commentClose {
			continue
		}

		return []byte(strings.Join(lines[start+1:index], "\n")), lines[index+1:], nil
	}

	return nil, nil, errUnterminatedDirective
}

// opensDirective reports whether a line opens a module directive comment.
func opensDirective(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, commentOpen) {
		return false
	}

	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, commentOpen))

	return rest == directiveMarker
}

// firstNonEmpty returns the first of its arguments that is not blank.
func firstNonEmpty(candidates ...string) string {
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}

	return ""
}
