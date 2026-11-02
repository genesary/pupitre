package markdown

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// courseSlug derives a URL-safe slug from a course title, for a document
// that names neither a slug of its own nor one in the request.
//
// Unlike a module slug, which only has to be unique inside its course,
// this ends up in the catalog URL and in every admin form that validates
// against [a-z0-9-] — so accents are folded to their base letter and
// anything still outside that set becomes a separator. A title with
// nothing to fold (a purely non-Latin one) yields an empty slug, and the
// caller asks for an explicit one instead of inventing something.
func courseSlug(title string) string {
	folded, _, err := transform.String(
		transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC),
		title,
	)
	if err != nil {
		folded = title
	}

	var builder strings.Builder

	for _, char := range strings.ToLower(folded) {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			builder.WriteRune(char)
		default:
			builder.WriteByte('-')
		}
	}

	return strings.Trim(collapseDashes(builder.String()), "-")
}

// collapseDashes squeezes runs of separators left by stripped characters
// into a single dash.
func collapseDashes(slug string) string {
	var builder strings.Builder

	previousDash := false

	for _, char := range slug {
		if char == '-' {
			if !previousDash {
				builder.WriteRune(char)
			}

			previousDash = true

			continue
		}

		previousDash = false

		builder.WriteRune(char)
	}

	return builder.String()
}
