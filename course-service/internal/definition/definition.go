// Package definition holds the wire representation of a course or learning
// path: the shape the admin API accepts and returns, and the shape the dev
// seed files are written in.
//
// It lives apart from internal/content (the domain types the rest of the
// service works with) so that the admin API and the seeder cannot drift
// from one another: both go through the conversions below.
package definition

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/genesary/pupitre/course-service/internal/content"
)

// Session is one scheduled in-person session inside a course definition.
// Sessions are keyed by ID in the enclosing map so that retrying a failed
// write overwrites the same slot rather than appending a duplicate.
type Session struct {
	Title    string `json:"title"`
	Date     string `json:"date"`
	Location string `json:"location,omitempty"`
	Capacity int    `json:"capacity,omitempty"`
}

// Course is the wire representation of a course definition, used for both
// the admin read and write endpoints so that a client can round-trip a
// course it just fetched.
type Course struct {
	Title         string                       `json:"title,omitempty"`
	Description   string                       `json:"description,omitempty"`
	Public        bool                         `json:"public,omitempty"`
	Hidden        bool                         `json:"hidden,omitempty"`
	Category      string                       `json:"category,omitempty"`
	Difficulty    string                       `json:"difficulty,omitempty"`
	Scope         string                       `json:"scope,omitempty"`
	XPRequired    int                          `json:"xpRequired,omitempty"`
	InPerson      bool                         `json:"inPerson,omitempty"`
	Badge         *content.Badge               `json:"badge,omitempty"`
	Modules       []content.Module             `json:"modules,omitempty"`
	Prerequisites []content.CoursePrerequisite `json:"prerequisites,omitempty"`
	Sessions      map[string]Session           `json:"sessions,omitempty"`
}

// ToCourse converts a wire definition into the domain course the
// repository persists.
func (d Course) ToCourse(slug string) *content.Course {
	course := &content.Course{
		Slug:          slug,
		Title:         d.Title,
		Description:   d.Description,
		Category:      d.Category,
		Difficulty:    d.Difficulty,
		IsPublic:      d.Public,
		Hidden:        d.Hidden,
		Scope:         d.Scope,
		XPRequired:    d.XPRequired,
		InPerson:      d.InPerson,
		Badge:         d.Badge,
		Modules:       d.Modules,
		Prerequisites: d.Prerequisites,
	}

	for sessionID, session := range d.Sessions {
		course.Sessions = append(course.Sessions, content.Session{
			ID:       sessionID,
			Title:    session.Title,
			Date:     session.Date,
			Location: session.Location,
			Capacity: session.Capacity,
		})
	}

	// Map iteration order is random; sort so a course written twice from
	// the same payload produces the same rows.
	sort.Slice(course.Sessions, func(i, j int) bool {
		return course.Sessions[i].ID < course.Sessions[j].ID
	})

	return course
}

// FromCourse converts a stored course back into its wire definition.
func FromCourse(course *content.Course) Course {
	spec := Course{
		Title:         course.Title,
		Description:   course.Description,
		Public:        course.IsPublic,
		Hidden:        course.Hidden,
		Category:      course.Category,
		Difficulty:    course.Difficulty,
		Scope:         course.Scope,
		XPRequired:    course.XPRequired,
		InPerson:      course.InPerson,
		Badge:         course.Badge,
		Modules:       course.Modules,
		Prerequisites: course.Prerequisites,
	}

	if len(course.Sessions) > 0 {
		spec.Sessions = make(map[string]Session, len(course.Sessions))
		for _, session := range course.Sessions {
			spec.Sessions[session.ID] = Session{
				Title:    session.Title,
				Date:     session.Date,
				Location: session.Location,
				Capacity: session.Capacity,
			}
		}
	}

	return spec
}

// Path is the wire representation of a learning path definition.
type Path struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Kind        string   `json:"kind,omitempty"`
	Level       string   `json:"level,omitempty"`
	Courses     []string `json:"courses,omitempty"`
	Skills      []string `json:"skills,omitempty"`
}

// ToPath converts a wire definition into the domain path the repository
// persists.
func (d Path) ToPath(slug string) *content.Path {
	return &content.Path{
		Slug:        slug,
		Title:       d.Title,
		Description: d.Description,
		Kind:        d.Kind,
		Level:       d.Level,
		Courses:     d.Courses,
		Skills:      d.Skills,
	}
}

// FromPath converts a stored path back into its wire definition, the
// inverse of ToPath. The admin UI reads a path through this rather than
// through the public endpoint, which substitutes the aggregated skills of
// the member courses for the ones actually stored.
func FromPath(path *content.Path) Path {
	return Path{
		Title:       path.Title,
		Description: path.Description,
		Kind:        path.Kind,
		Level:       path.Level,
		Courses:     path.Courses,
		Skills:      path.Skills,
	}
}

// errMissingSlug is returned for a definition file that names no slug.
var errMissingSlug = errors.New("course definition has no slug")

// File is a definition read from a YAML file: the slug the resource is
// stored under, plus the definition itself.
type File struct {
	Slug string `json:"slug"`
	Spec Course `json:"spec"`
}

// DecodeYAML decodes YAML into target through JSON, so the `json` tags on
// the types above stay the single description of the wire shape.
//
// YAML is the authoring format wherever a human writes a definition by
// hand — seed files, markdown frontmatter, the per-module directives of an
// imported document — because long markdown bodies read far better as
// block scalars than as escaped JSON strings. Routing the decode through
// JSON keeps those files and the admin API from drifting apart.
func DecodeYAML(data []byte, target any) error {
	var intermediate any

	err := yaml.Unmarshal(data, &intermediate)
	if err != nil {
		return fmt.Errorf("parse YAML: %w", err)
	}

	encoded, err := json.Marshal(intermediate)
	if err != nil {
		return fmt.Errorf("re-encode definition: %w", err)
	}

	err = json.Unmarshal(encoded, target)
	if err != nil {
		return fmt.Errorf("decode definition: %w", err)
	}

	return nil
}

// EncodeYAML renders a definition value as YAML keyed by its `json` tags,
// the inverse of DecodeYAML. Mapping keys come out sorted, so the same
// value always produces the same bytes.
func EncodeYAML(source any) ([]byte, error) {
	encoded, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("encode definition: %w", err)
	}

	var intermediate any

	err = json.Unmarshal(encoded, &intermediate)
	if err != nil {
		return nil, fmt.Errorf("re-decode definition: %w", err)
	}

	out, err := yaml.Marshal(intermediate)
	if err != nil {
		return nil, fmt.Errorf("render YAML: %w", err)
	}

	return out, nil
}

// PathFile is a path definition read from a YAML seed file.
type PathFile struct {
	Slug string `json:"slug"`
	Spec Path   `json:"spec"`
}

// ParsePathFile decodes a learning-path definition file.
func ParsePathFile(data []byte) (PathFile, error) {
	var file PathFile

	err := DecodeYAML(data, &file)
	if err != nil {
		return PathFile{}, fmt.Errorf("parse path file: %w", err)
	}

	if file.Slug == "" {
		return PathFile{}, errMissingSlug
	}

	return file, nil
}

// ParseCourseFile decodes a course definition file.
func ParseCourseFile(data []byte) (File, error) {
	var file File

	err := DecodeYAML(data, &file)
	if err != nil {
		return File{}, fmt.Errorf("parse course file: %w", err)
	}

	if file.Slug == "" {
		return File{}, errMissingSlug
	}

	return file, nil
}
