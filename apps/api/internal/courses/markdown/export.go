package markdown

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jojo91550/pupitre2/apps/api/internal/courses/content"
	"github.com/jojo91550/pupitre2/apps/api/internal/courses/definition"
)

// Directive keys that Export never writes, because the document already
// carries them as the heading and the body underneath it.
const (
	moduleNameKey    = "name"
	moduleContentKey = "content"
	moduleTypeKey    = "type"
)

// checkParamsKey holds a lab's provider-defined settings, the one part of
// a module the exporter passes through untouched.
const checkParamsKey = "checkParams"

// meaningfulZeroKeys names directive keys whose false/zero value carries
// information, so pruneZeroes must leave them alone:
//
//   - correctAnswer (*bool) and maxAttemptsPerQuestion (*int): pointer
//     fields where "absent" and "zero" differ. Pruning correctAnswer would
//     turn a true/false question answered "false" into one with no answer
//     key.
//   - partialScoring (*PartialScoring): a pointer to a struct. omitempty
//     only drops it when nil, so its presence already means "configured";
//     an all-false value must survive as a non-nil struct, not collapse to
//     nil, or the import loses the "configured but disabled" state.
//
// All three break the export → import round trip when pruned.
//
//nolint:gochecknoglobals // static lookup table, read-only
var meaningfulZeroKeys = map[string]bool{
	"correctAnswer":          true,
	"maxAttemptsPerQuestion": true,
	"partialScoring":         true,
}

// ExportOptions controls how Export lays out a document.
type ExportOptions struct {
	// Split names the heading level to write each module under. Empty
	// picks a level no module body already uses, so that importing the
	// result back cuts it at exactly the same places.
	Split string
}

// Export renders a stored course definition as the markdown document
// Import reads back into the same definition.
func Export(slug string, spec definition.Course, opts ExportOptions) ([]byte, error) {
	level, err := exportLevel(opts.Split, spec.Modules)
	if err != nil {
		return nil, err
	}

	header := frontmatter{Course: spec, Slug: slug, Split: SplitName(level)}
	header.Modules = nil

	front, err := definition.EncodeYAML(header)
	if err != nil {
		return nil, fmt.Errorf("render frontmatter: %w", err)
	}

	var out strings.Builder

	out.WriteString(frontmatterFence + "\n")
	out.Write(front)
	out.WriteString(frontmatterFence + "\n")

	prefix := strings.Repeat("#", level)

	for _, module := range spec.Modules {
		block, err := exportModule(prefix, module)
		if err != nil {
			return nil, err
		}

		out.WriteString(block)
	}

	return []byte(out.String()), nil
}

// exportModule renders one module as a heading, an optional directive and
// its markdown body.
func exportModule(prefix string, module content.Module) (string, error) {
	attributes, err := directiveAttributes(module)
	if err != nil {
		return "", err
	}

	var out strings.Builder

	out.WriteString("\n" + prefix + " " + headingText(module.Name) + "\n")

	if len(attributes) > 0 {
		directive, err := definition.EncodeYAML(attributes)
		if err != nil {
			return "", fmt.Errorf("render directive for module %q: %w", module.Name, err)
		}

		out.WriteString("\n" + commentOpen + directiveMarker + "\n")
		out.Write(directive)
		out.WriteString(commentClose + "\n")
	}

	if body := strings.TrimSpace(module.InlineContent); body != "" {
		out.WriteString("\n" + body + "\n")
	}

	return out.String(), nil
}

// directiveAttributes returns everything about a module that the heading
// and the body below it cannot express.
//
// Fields sitting at their zero value are dropped: the importer applies the
// same defaults, so writing them out would only bury the two or three
// attributes that matter under a wall of noise. A plain text module with
// an inline body is left with none at all, and so exports as clean
// markdown with no comment above it.
func directiveAttributes(module content.Module) (map[string]any, error) {
	encoded, err := json.Marshal(module)
	if err != nil {
		return nil, fmt.Errorf("encode module %q: %w", module.Name, err)
	}

	var attributes map[string]any

	err = json.Unmarshal(encoded, &attributes)
	if err != nil {
		return nil, fmt.Errorf("decode module %q: %w", module.Name, err)
	}

	delete(attributes, moduleNameKey)
	delete(attributes, moduleContentKey)

	if attributes[moduleTypeKey] == content.ModuleTypeText {
		delete(attributes, moduleTypeKey)
	}

	pruneZeroes(attributes)

	return attributes, nil
}

// pruneZeroes removes every key whose value carries no information —
// blank, false, zero, or an empty list or object once its own zero-valued
// keys are gone.
//
// It walks nested objects and lists so that, say, a question's untouched
// feedback block does not show up as an empty map under every question.
// The one subtree it leaves alone is checkParams: those are opaque
// provider-defined settings, where "absent" and "false" are not ours to
// call equivalent.
func pruneZeroes(values map[string]any) {
	for key, value := range values {
		if meaningfulZeroKeys[key] {
			continue
		}

		if key != checkParamsKey {
			pruneValue(value)
		}

		if isZero(value) {
			delete(values, key)
		}
	}
}

// pruneValue applies pruneZeroes to any objects reachable from value.
func pruneValue(value any) {
	switch typed := value.(type) {
	case map[string]any:
		pruneZeroes(typed)
	case []any:
		for _, element := range typed {
			pruneValue(element)
		}
	}
}

// isZero reports whether a decoded JSON value is its type's zero value.
func isZero(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case bool:
		return !typed
	case float64:
		return typed == 0
	case string:
		return typed == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

// headingText renders a module name as the text of its heading line. Only
// line breaks are neutralised — replaced with a space so a name that
// somehow contains one cannot split the document it names. Every other run
// of whitespace is kept verbatim, so a name like "Intro   to  Linux" or
// one holding a tab survives the import that reads the heading back.
func headingText(name string) string {
	flat := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}

		return r
	}, name)

	flat = strings.TrimSpace(flat)
	if flat == "" {
		return fallbackModuleName
	}

	return flat
}

// exportLevel resolves the heading depth to write modules under.
//
// Left to itself it picks the shallowest depth that no module body already
// uses, so re-importing the document cuts it back into exactly the same
// modules instead of splitting inside one. A course whose bodies somehow
// use all six depths falls back to h1 and accepts the collision.
func exportLevel(requested string, modules []content.Module) (int, error) {
	if strings.TrimSpace(requested) != "" {
		level, ok := SplitLevel(requested)
		if !ok || level == 0 {
			return 0, fmt.Errorf("cannot export at split level %q: expected h1, h2, h3, h4, h5 or h6", requested) //nolint:err113 // the rejected value belongs in the message
		}

		return level, nil
	}

	var used [maxHeadingLevel + 1]bool

	for _, module := range modules {
		markHeadingLevels(module.InlineContent, &used)
	}

	for level := 1; level <= maxHeadingLevel; level++ {
		if !used[level] {
			return level, nil
		}
	}

	return 1, nil
}
