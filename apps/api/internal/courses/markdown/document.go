// Package markdown converts a course definition to and from a single
// markdown document, so a course can be authored by writing (or pasting)
// markdown instead of laying out a git repository.
//
// A document is an optional YAML frontmatter block carrying the course
// fields, followed by markdown that is cut into modules at a chosen
// heading level. Anything a module needs that markdown cannot express —
// its type, a git source, quiz questions — goes in an HTML comment
// directly under the heading, where no renderer will show it:
//
//	---
//	slug: kubernetes-basics
//	title: Kubernetes Basics
//	split: h2
//	---
//
//	## What is Kubernetes
//
//	A container orchestrator…
//
//	## Knowledge check
//
//	<!--pupitre
//	type: quiz
//	passingScore: 80
//	questions: […]
//	-->
package markdown

import (
	"strings"
)

// Split levels name the heading depth at which a new module starts.
// SplitNone keeps the whole document as one module.
const (
	SplitNone = "none"
	SplitH1   = "h1"
	SplitH2   = "h2"
	SplitH3   = "h3"
	SplitH4   = "h4"
	SplitH5   = "h5"
	SplitH6   = "h6"
)

// maxHeadingLevel is the deepest ATX heading markdown defines.
const maxHeadingLevel = 6

// maxHeadingIndent is the widest indent an ATX heading may carry before
// markdown reads the line as an indented code block instead.
const maxHeadingIndent = 3

// minFenceLength is the shortest run of backticks or tildes that opens a
// fenced code block.
const minFenceLength = 3

// splitNames indexes every accepted split name by the heading depth it
// stands for, so the two directions stay in step by construction.
//
//nolint:gochecknoglobals // static lookup table, read-only
var splitNames = [maxHeadingLevel + 1]string{SplitNone, SplitH1, SplitH2, SplitH3, SplitH4, SplitH5, SplitH6}

// SplitLevel resolves a split name to its heading depth. A blank name and
// SplitNone both resolve to a depth of zero — do not split — while a name
// that is not one of the Split constants reports ok=false.
func SplitLevel(name string) (int, bool) {
	wanted := strings.ToLower(strings.TrimSpace(name))
	if wanted == "" {
		return 0, true
	}

	for level, candidate := range splitNames {
		if candidate == wanted {
			return level, true
		}
	}

	return 0, false
}

// SplitName returns the split constant for a heading depth, so a level
// chosen by the exporter can be written back into the frontmatter.
func SplitName(level int) string {
	if level < 0 || level > maxHeadingLevel {
		return SplitNone
	}

	return splitNames[level]
}

// section is one chunk of a document: the heading text that started it
// (empty for the text preceding the first heading) and the lines below it.
type section struct {
	Heading string
	Lines   []string
}

// normalize strips a UTF-8 BOM and collapses CRLF/CR line endings, so the
// scanner below only ever has to reason about "\n".
func normalize(document []byte) string {
	text := strings.TrimPrefix(string(document), "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")

	return strings.ReplaceAll(text, "\r", "\n")
}

// fence tracks whether the scanner is currently inside a fenced code
// block, so that a "## " line in a shell snippet never cuts a module in
// half.
type fence struct {
	marker byte
	length int
	open   bool
}

// step feeds one line to the tracker and reports whether that line is
// inside a code fence (the fence delimiters themselves count as inside).
func (f *fence) step(line string) bool {
	marker, length := fenceDelimiter(line)

	switch {
	case !f.open && length > 0:
		f.marker, f.length, f.open = marker, length, true
	case f.open && marker == f.marker && length >= f.length:
		f.open = false
	default:
		return f.open
	}

	return true
}

// fenceDelimiter returns the character and run length of a ``` or ~~~
// fence line, or a zero length when the line is not a fence.
func fenceDelimiter(line string) (marker byte, length int) { //nolint:nonamedreturns // named for gocritic(unnamedResult)
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > maxHeadingIndent || trimmed == "" {
		return 0, 0
	}

	marker = trimmed[0]
	if marker != '`' && marker != '~' {
		return 0, 0
	}

	length = len(trimmed) - len(strings.TrimLeft(trimmed, string(marker)))
	if length < minFenceLength {
		return 0, 0
	}

	return marker, length
}

// heading returns the depth and text of an ATX heading line, or a depth of
// zero when the line is not one. Setext headings ("===" underlines) are
// deliberately not recognised: they cannot express a depth beyond two and
// would make a stray divider line cut a module in half.
func heading(line string) (level int, text string) { //nolint:nonamedreturns // named for gocritic(unnamedResult)
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > maxHeadingIndent {
		return 0, ""
	}

	level = len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
	if level == 0 || level > maxHeadingLevel {
		return 0, ""
	}

	rest := trimmed[level:]
	if rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\t") {
		return 0, ""
	}

	core := strings.TrimSpace(rest)

	// Strip an ATX closing sequence — a run of trailing '#' — only when it
	// is preceded by whitespace or is all that is left. Otherwise a module
	// named "C#" would come back as "C" on re-import.
	if stripped := strings.TrimRight(core, "#"); stripped == "" ||
		strings.HasSuffix(stripped, " ") || strings.HasSuffix(stripped, "\t") {
		core = strings.TrimSpace(stripped)
	}

	return level, core
}

// sections cuts a document body at every heading of the given depth. A
// depth of zero yields a single section holding the whole body.
//
// The text before the first matching heading always comes back as the
// leading section with an empty Heading, whether or not it is blank; the
// caller decides what to do with it.
func sections(body string, level int) []section {
	lines := strings.Split(body, "\n")
	if level == 0 {
		return []section{{Lines: lines}}
	}

	out := []section{{}}
	tracker := &fence{}

	for _, line := range lines {
		if tracker.step(line) {
			out[len(out)-1].Lines = append(out[len(out)-1].Lines, line)

			continue
		}

		depth, text := heading(line)
		if depth == level {
			out = append(out, section{Heading: text})

			continue
		}

		out[len(out)-1].Lines = append(out[len(out)-1].Lines, line)
	}

	return out
}

// headingCounts tallies how many ATX headings of each depth a body uses,
// skipping fenced code. Index zero is unused; the depth is the index.
func headingCounts(text string) [maxHeadingLevel + 1]int {
	var counts [maxHeadingLevel + 1]int

	tracker := &fence{}

	for line := range strings.SplitSeq(text, "\n") {
		if tracker.step(line) {
			continue
		}

		if depth, _ := heading(line); depth > 0 {
			counts[depth]++
		}
	}

	return counts
}

// markHeadingLevels records every heading depth text uses, so the exporter
// can pick a depth for its own headings that no body will collide with.
// Fenced code is skipped.
func markHeadingLevels(text string, used *[maxHeadingLevel + 1]bool) {
	for depth, count := range headingCounts(text) {
		if count > 0 {
			used[depth] = true
		}
	}
}

// trimBlank drops leading and trailing blank lines and joins the rest, so
// a module body never starts or ends with the whitespace that separated it
// from its heading.
func trimBlank(lines []string) string {
	start, end := 0, len(lines)

	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	return strings.Join(lines[start:end], "\n")
}
