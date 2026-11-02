package repository

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/jojo91550/pupitre2/apps/api/internal/platform/csvsafe"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// Category identifier constants for CSV export.
const (
	catUsers           = "users"
	catEnrollments     = "enrollments"
	catModuleProgress  = "module_progress"
	catQuizAttempts    = "quiz_attempts"
	catPathEnrollments = "path_enrollments"
	catXPEvents        = "xp_events"
	catLessonProgress  = "lesson_progress"
)

// Field identifier constants specific to CSV export.
const (
	fieldEnrolledAt  = "enrolledat"
	fieldModuleIndex = "moduleindex"
	fieldModuleSlug  = "moduleslug"
	fieldModuleType  = "moduletype"
	fieldBestScore   = "bestscore"
	fieldAttempts    = "attempts"
	fieldPathSlug    = "path_slug"
	fieldSourceSlug  = "source_slug"
	fieldAmount      = "amount"
	fieldEarnedAt    = "earned_at"
	fieldRole        = "role"
	fieldIsActive    = "isactive"
	fieldTotalXP     = "totalxp"
	fieldViewedAt    = "viewedat"
	fieldLastLoginAt = "lastloginat"
	// fieldCourseTitle and fieldPathTitle are virtual: the SQL layer returns an
	// empty string; the handler enriches them via cross-service calls.
	fieldCourseTitle = "course_title"
	fieldPathTitle   = "path_title"
)

// SQL column expression constants reused across multiple category definitions.
const (
	sqlUserEmail     = "u.email"
	sqlUserUsername  = "u.username"
	sqlTotalXP       = "(SELECT COALESCE(SUM(x.amount),0) FROM user_xp_events x WHERE x.userid = u.id)::text"
	sqlEnrolledAt    = "e.enrolledat"
	sqlECourseSlug   = "e.courseslug"
	sqlMPCourseSlug  = "mp.courseslug"
	sqlMPCompletedAt = "mp.completed_at"
	sqlPEEnrolledAt  = "pe.enrolledat"
	sqlPEPathSlug    = "pe.path_slug"
	sqlLPCourseSlug  = "lp.courseslug"
	sqlLPViewedAt    = "lp.viewed_at"
	sqlXEarnedAt     = "x.earned_at"
	sqlURole         = "u.role"
	sqlUCreatedAt    = "u.createdat"
	sqlXSource       = "x.source"
	// sqlMPFrom is the FROM/JOIN shared by catModuleProgress and catQuizAttempts.
	sqlMPFrom      = "FROM module_progress mp JOIN users u ON u.id = mp.userid"
	sqlMPPassedCol = "mp.passed"
	// sqlMPUserID through sqlMPModuleType are SELECT expressions for
	// module_progress columns, shared to avoid goconst violations.
	sqlMPUserID        = "mp.userid::text"
	sqlMPModuleIndex   = "mp.moduleindex::text"
	sqlMPModuleSlug    = "COALESCE(mp.moduleslug, '')"
	sqlMPBestScore     = "mp.bestscore::text"
	sqlMPMaxScore      = "mp.maxscore::text"
	sqlMPPassed        = "mp.passed::text"
	sqlMPAttempts      = "mp.attempts::text"
	sqlMPCompletedText = "COALESCE(mp.completed_at::text, '')"
	sqlMPModuleType    = "COALESCE(mp.moduletype, '')"
	// sqlVirtual is the SQL expression for virtual fields whose values are
	// populated by the handler via cross-service enrichment after the query.
	sqlVirtual = "''::text"
)

// Filter kind values for filterSpec.Type.
const (
	filterKindDate   = "date"
	filterKindSelect = "select"
	filterKindText   = "text"
)

// SQL cast suffixes appended to ? placeholders.
const (
	castDate    = "::date"
	castBoolean = "::boolean"
)

// Shared filter display labels.
const (
	labelFilterDepuis   = "Inscrit depuis"
	labelFilterJusquAu  = "Inscrit jusqu'au"
	labelFilterCoursSlu = "Cours (slug)"
	labelFilterReussi   = "Réussi"
	labelFilterRole     = "Rôle"
	labelFilterActif    = "Actif"
	labelFilterSource   = "Source"
)

// Shared filter ID constants.
const (
	filterIDEnrolledFrom = "enrolled_from"
	filterIDEnrolledTo   = "enrolled_to"
	filterIDCoursSlug    = "course_slug"
)

// Boolean filter option values.
const (
	optTrue  = "true"
	optFalse = "false"
)

// Shared field label constants.
const (
	labelEmail       = "Email"
	labelUsername    = "Pseudo"
	labelUserID      = "ID utilisateur"
	labelEnrolledAt  = "Inscrit le"
	labelCourseSlug  = "Slug cours"
	labelModuleIndex = "Index module"
	labelModuleSlug  = "Slug module"
	labelModuleType  = "Type module"
	labelBestScore   = "Meilleur score"
	labelMaxScore    = "Score max"
	labelAttempts    = "Tentatives"
	labelCompletedAt = "Complété le"
	labelCourseTitle = "Titre cours"
	labelPathTitle   = "Titre parcours"
	labelOui         = "Oui"
	labelNon         = "Non"
)

// Shared filter ID and label constants for module completion filters.
const (
	filterIDCompletedFrom = "completed_from"
	filterIDCompletedTo   = "completed_to"
	labelCompletedDepuis  = "Complété depuis"
	labelCompletedJusquAu = "Complété jusqu'au"
)

// fieldDef maps a field identifier to its display label and SQL expression.
type fieldDef struct {
	Label string
	Expr  string
}

// filterSpec describes one filterable dimension of a data category.
type filterSpec struct {
	ID          string
	Label       string
	Type        string         // "date", "select", "text"
	Expr        string         // SQL column expression (left of comparison)
	Op          string         // SQL operator: ">=", "<=", "="
	ParamSuffix string         // optional SQL cast appended to ?, e.g. "::boolean", "::date"
	Options     []filterOption // for type="select"
}

// filterOption is a (value, label) pair for select-type filters.
type filterOption struct {
	Value string
	Label string
}

// categoryDef describes a data category available for CSV export.
type categoryDef struct {
	Label         string
	FromClause    string
	BaseWhere     string // optional static WHERE condition (e.g. "mp.moduletype = 'quiz'")
	Fields        map[string]fieldDef
	DefaultFields []string
	FilterSpecs   []filterSpec
}

// ExportCategoryMeta is the JSON-serializable description of a data category.
type ExportCategoryMeta struct {
	ID            string             `json:"id"`
	Label         string             `json:"label"`
	Fields        []ExportFieldMeta  `json:"fields"`
	DefaultFields []string           `json:"defaultFields"`
	Filters       []ExportFilterMeta `json:"filters"`
}

// ExportFieldMeta is the JSON-serializable description of an exportable field.
type ExportFieldMeta struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// ExportFilterMeta is the JSON-serializable description of an available filter.
type ExportFilterMeta struct {
	ID      string               `json:"id"`
	Label   string               `json:"label"`
	Type    string               `json:"type"`
	Options []ExportFilterOption `json:"options,omitempty"`
}

// ExportFilterOption is a selectable value for a filter of type "select".
type ExportFilterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// exportCategoryDefs holds every exportable category definition.
//
//nolint:gochecknoglobals // static export schema, read-only after package init
var exportCategoryDefs = map[string]categoryDef{
	catUsers: {
		Label:      "Utilisateurs",
		FromClause: "FROM users u",
		Fields: map[string]fieldDef{
			"id":             {"ID", "u.id::text"},
			colEmail:         {labelEmail, sqlUserEmail},
			colUsername:      {labelUsername, sqlUserUsername},
			fieldRole:        {labelFilterRole, sqlURole},
			fieldIsActive:    {labelFilterActif, "u.isactive::text"},
			colCreatedAt:     {"Inscrit le", "u.createdat::text"},
			fieldLastLoginAt: {"Dernière connexion", "u.lastloginat::text"},
			fieldTotalXP:     {"XP total", sqlTotalXP},
		},
		DefaultFields: []string{colEmail, colUsername, fieldRole, colCreatedAt},
		FilterSpecs: []filterSpec{
			{ID: "created_from", Label: labelFilterDepuis, Type: filterKindDate, Expr: sqlUCreatedAt, Op: ">=", ParamSuffix: castDate},
			{ID: "created_to", Label: labelFilterJusquAu, Type: filterKindDate, Expr: sqlUCreatedAt, Op: "<=", ParamSuffix: castDate},
			{
				ID: fieldRole, Label: labelFilterRole, Type: filterKindSelect, Expr: sqlURole, Op: "=",
				Options: []filterOption{
					{Value: "student", Label: "Étudiant"},
					{Value: "manager", Label: "Manager"},
					{Value: "admin", Label: "Admin"},
				},
			},
			{
				ID: "active", Label: "Statut", Type: filterKindSelect, Expr: "u.isactive", Op: "=", ParamSuffix: castBoolean,
				Options: []filterOption{
					{Value: optTrue, Label: labelFilterActif},
					{Value: optFalse, Label: "Inactif"},
				},
			},
		},
	},
	catEnrollments: {
		Label:      "Inscriptions aux cours",
		FromClause: "FROM enrollments e JOIN users u ON u.id = e.userid",
		Fields: map[string]fieldDef{
			colUserID:        {labelUserID, "e.userid::text"},
			colEmail:         {labelEmail, sqlUserEmail},
			colUsername:      {labelUsername, sqlUserUsername},
			colCourseSlug:    {labelCourseSlug, sqlECourseSlug},
			fieldEnrolledAt:  {labelEnrolledAt, "e.enrolledat::text"},
			fieldCourseTitle: {labelCourseTitle, sqlVirtual},
		},
		DefaultFields: []string{colEmail, colUsername, colCourseSlug, fieldEnrolledAt},
		FilterSpecs: []filterSpec{
			{ID: filterIDEnrolledFrom, Label: labelFilterDepuis, Type: filterKindDate, Expr: sqlEnrolledAt, Op: ">=", ParamSuffix: castDate},
			{ID: filterIDEnrolledTo, Label: labelFilterJusquAu, Type: filterKindDate, Expr: sqlEnrolledAt, Op: "<=", ParamSuffix: castDate},
			{ID: filterIDCoursSlug, Label: labelFilterCoursSlu, Type: filterKindText, Expr: sqlECourseSlug, Op: "="},
		},
	},
	catModuleProgress: {
		Label:      "Progression modules",
		FromClause: sqlMPFrom,
		Fields: map[string]fieldDef{
			colUserID:        {labelUserID, sqlMPUserID},
			colEmail:         {labelEmail, sqlUserEmail},
			colCourseSlug:    {labelCourseSlug, sqlMPCourseSlug},
			fieldModuleIndex: {labelModuleIndex, sqlMPModuleIndex},
			fieldModuleSlug:  {labelModuleSlug, sqlMPModuleSlug},
			fieldModuleType:  {labelModuleType, sqlMPModuleType},
			fieldBestScore:   {labelBestScore, sqlMPBestScore},
			colMaxScore:      {labelMaxScore, sqlMPMaxScore},
			colPassed:        {labelFilterReussi, sqlMPPassed},
			fieldAttempts:    {labelAttempts, sqlMPAttempts},
			colCompletedAt:   {labelCompletedAt, sqlMPCompletedText},
			fieldCourseTitle: {labelCourseTitle, sqlVirtual},
		},
		DefaultFields: []string{colEmail, colCourseSlug, fieldModuleIndex, colPassed, fieldBestScore, fieldAttempts},
		FilterSpecs: []filterSpec{
			{ID: filterIDCoursSlug, Label: labelFilterCoursSlu, Type: filterKindText, Expr: sqlMPCourseSlug, Op: "="},
			{ID: filterIDCompletedFrom, Label: labelCompletedDepuis, Type: filterKindDate, Expr: sqlMPCompletedAt, Op: ">=", ParamSuffix: castDate},
			{ID: filterIDCompletedTo, Label: labelCompletedJusquAu, Type: filterKindDate, Expr: sqlMPCompletedAt, Op: "<=", ParamSuffix: castDate},
			{
				ID: colPassed, Label: labelFilterReussi, Type: filterKindSelect, Expr: sqlMPPassedCol, Op: "=", ParamSuffix: castBoolean,
				Options: []filterOption{
					{Value: optTrue, Label: labelOui},
					{Value: optFalse, Label: labelNon},
				},
			},
		},
	},
	catQuizAttempts: {
		Label:      "Tentatives quiz",
		FromClause: sqlMPFrom,
		BaseWhere:  "mp.moduletype = 'quiz'",
		Fields: map[string]fieldDef{
			colUserID:        {labelUserID, sqlMPUserID},
			colEmail:         {labelEmail, sqlUserEmail},
			colCourseSlug:    {labelCourseSlug, sqlMPCourseSlug},
			fieldModuleIndex: {labelModuleIndex, sqlMPModuleIndex},
			fieldModuleSlug:  {labelModuleSlug, sqlMPModuleSlug},
			fieldBestScore:   {labelBestScore, sqlMPBestScore},
			colMaxScore:      {labelMaxScore, sqlMPMaxScore},
			colPassed:        {labelFilterReussi, sqlMPPassed},
			fieldAttempts:    {labelAttempts, sqlMPAttempts},
			colCompletedAt:   {labelCompletedAt, sqlMPCompletedText},
			fieldCourseTitle: {labelCourseTitle, sqlVirtual},
		},
		DefaultFields: []string{colEmail, colCourseSlug, fieldModuleIndex, colPassed, fieldBestScore, fieldAttempts},
		FilterSpecs: []filterSpec{
			{ID: filterIDCoursSlug, Label: labelFilterCoursSlu, Type: filterKindText, Expr: sqlMPCourseSlug, Op: "="},
			{ID: filterIDCompletedFrom, Label: labelCompletedDepuis, Type: filterKindDate, Expr: sqlMPCompletedAt, Op: ">=", ParamSuffix: castDate},
			{ID: filterIDCompletedTo, Label: labelCompletedJusquAu, Type: filterKindDate, Expr: sqlMPCompletedAt, Op: "<=", ParamSuffix: castDate},
			{
				ID: colPassed, Label: labelFilterReussi, Type: filterKindSelect, Expr: sqlMPPassedCol, Op: "=", ParamSuffix: castBoolean,
				Options: []filterOption{
					{Value: optTrue, Label: labelOui},
					{Value: optFalse, Label: labelNon},
				},
			},
		},
	},
	catPathEnrollments: {
		Label:      "Inscriptions aux parcours",
		FromClause: "FROM path_enrollments pe JOIN users u ON u.id = pe.userid",
		Fields: map[string]fieldDef{
			colUserID:       {labelUserID, "pe.userid::text"},
			colEmail:        {labelEmail, sqlUserEmail},
			colUsername:     {labelUsername, sqlUserUsername},
			fieldPathSlug:   {"Slug parcours", sqlPEPathSlug},
			fieldEnrolledAt: {labelEnrolledAt, "pe.enrolledat::text"},
			fieldPathTitle:  {labelPathTitle, sqlVirtual},
		},
		DefaultFields: []string{colEmail, colUsername, fieldPathSlug, fieldEnrolledAt},
		FilterSpecs: []filterSpec{
			{ID: filterIDEnrolledFrom, Label: labelFilterDepuis, Type: filterKindDate, Expr: sqlPEEnrolledAt, Op: ">=", ParamSuffix: castDate},
			{ID: filterIDEnrolledTo, Label: labelFilterJusquAu, Type: filterKindDate, Expr: sqlPEEnrolledAt, Op: "<=", ParamSuffix: castDate},
			{ID: "path_slug", Label: "Parcours (slug)", Type: filterKindText, Expr: sqlPEPathSlug, Op: "="},
		},
	},
	catXPEvents: {
		Label:      "Événements XP",
		FromClause: "FROM user_xp_events x JOIN users u ON u.id = x.userid",
		Fields: map[string]fieldDef{
			colUserID:       {labelUserID, "x.userid::text"},
			colEmail:        {labelEmail, sqlUserEmail},
			colSource:       {labelFilterSource, sqlXSource},
			fieldSourceSlug: {"Slug source", "x.source_slug"},
			fieldAmount:     {"Points XP", "x.amount::text"},
			fieldEarnedAt:   {"Gagné le", "x.earned_at::text"},
		},
		DefaultFields: []string{colEmail, colSource, fieldSourceSlug, fieldAmount, fieldEarnedAt},
		FilterSpecs: []filterSpec{
			{ID: "earned_from", Label: "Depuis", Type: filterKindDate, Expr: sqlXEarnedAt, Op: ">=", ParamSuffix: castDate},
			{ID: "earned_to", Label: "Jusqu'au", Type: filterKindDate, Expr: sqlXEarnedAt, Op: "<=", ParamSuffix: castDate},
			{
				ID: colSource, Label: labelFilterSource, Type: filterKindSelect, Expr: sqlXSource, Op: "=",
				Options: []filterOption{
					{Value: "lesson", Label: "Leçon"},
					{Value: "module", Label: "Module"},
					{Value: "course", Label: "Cours"},
				},
			},
		},
	},
	catLessonProgress: {
		Label:      "Historique leçons",
		FromClause: "FROM lesson_progress lp JOIN users u ON u.id = lp.userid",
		Fields: map[string]fieldDef{
			colUserID:     {labelUserID, "lp.userid::text"},
			colEmail:      {labelEmail, sqlUserEmail},
			colUsername:   {labelUsername, sqlUserUsername},
			colCourseSlug: {labelCourseSlug, sqlLPCourseSlug},
			colLessonSlug: {"Slug leçon", "lp.lessonslug"},
			fieldViewedAt: {"Vu le", "lp.viewed_at::text"},
		},
		DefaultFields: []string{colEmail, colCourseSlug, colLessonSlug, fieldViewedAt},
		FilterSpecs: []filterSpec{
			{ID: filterIDCoursSlug, Label: labelFilterCoursSlu, Type: filterKindText, Expr: sqlLPCourseSlug, Op: "="},
			{ID: "viewed_from", Label: "Vu depuis", Type: filterKindDate, Expr: sqlLPViewedAt, Op: ">=", ParamSuffix: castDate},
			{ID: "viewed_to", Label: "Vu jusqu'au", Type: filterKindDate, Expr: sqlLPViewedAt, Op: "<=", ParamSuffix: castDate},
		},
	},
}

// exportCategoryOrder controls the display order of categories in the UI.
//
//nolint:gochecknoglobals // static export schema, read-only after package init
var exportCategoryOrder = []string{
	catUsers,
	catEnrollments,
	catModuleProgress,
	catQuizAttempts,
	catPathEnrollments,
	catXPEvents,
	catLessonProgress,
}

// ExportScope limits an export to the users of some groups: a manager
// exports the members of the groups they manage, an admin everything
// (docs/specs/190-backend-defects.md B6).
type ExportScope struct {
	// Restricted turns the limit on; GroupIDs may then be empty, which
	// exports nothing.
	Restricted bool
	GroupIDs   []string
}

// Unrestricted is the scope of an admin: every row.
//
//nolint:gochecknoglobals // immutable zero value, named for readability
var Unrestricted = ExportScope{}

// ExportRepository handles CSV data extraction and audit logging.
type ExportRepository interface {
	// Categories returns the ordered list of available export categories
	// with their field and filter descriptors.
	Categories() []ExportCategoryMeta

	// FetchRows returns the first limit rows for category filtered to
	// fields and filters, plus the total matching count.
	FetchRows(ctx context.Context, category string, fields []string, filters map[string]string, scope ExportScope, limit int) ([]string, []map[string]string, int64, error)

	// StreamRows runs the same query unbounded and hands each row to visit
	// as it is read, so a large export never has to fit in memory.
	StreamRows(ctx context.Context, category string, fields []string, filters map[string]string, scope ExportScope, visit func(row map[string]string) error) ([]string, error)

	// WriteCSV streams the full dataset for category/fields/filters into
	// writer as CSV.
	WriteCSV(ctx context.Context, writer io.Writer, category string, fields []string, filters map[string]string, scope ExportScope) (int, error)

	// LogDownload records a download event in the export_logs audit table.
	LogDownload(ctx context.Context, log *models.ExportLog) error
}

// gormExportRepository is the GORM-backed ExportRepository implementation.
type gormExportRepository struct {
	db *gorm.DB
}

// NewGormExportRepository returns an ExportRepository backed by gdb.
//
//nolint:ireturn // consistent with all other repository constructors in this package
func NewGormExportRepository(db *gorm.DB) ExportRepository {
	return &gormExportRepository{db: db}
}

// toFilterMetas converts filterSpec slice to JSON-serializable values.
func toFilterMetas(specs []filterSpec) []ExportFilterMeta {
	metas := make([]ExportFilterMeta, 0, len(specs))

	for _, spec := range specs {
		opts := make([]ExportFilterOption, 0, len(spec.Options))

		for _, opt := range spec.Options {
			opts = append(opts, ExportFilterOption(opt))
		}

		metas = append(metas, ExportFilterMeta{
			ID:      spec.ID,
			Label:   spec.Label,
			Type:    spec.Type,
			Options: opts,
		})
	}

	return metas
}

// Categories returns the ordered list of export category descriptors.
func (r *gormExportRepository) Categories() []ExportCategoryMeta {
	result := make([]ExportCategoryMeta, 0, len(exportCategoryDefs))

	for _, catID := range exportCategoryOrder {
		def, exists := exportCategoryDefs[catID]
		if !exists {
			continue
		}

		seen := make(map[string]bool, len(def.Fields))
		fields := make([]ExportFieldMeta, 0, len(def.Fields))

		for _, fid := range def.DefaultFields {
			if fdef, ok := def.Fields[fid]; ok {
				fields = append(fields, ExportFieldMeta{ID: fid, Label: fdef.Label})
				seen[fid] = true
			}
		}

		extras := make([]string, 0, len(def.Fields))

		for key := range def.Fields {
			if !seen[key] {
				extras = append(extras, key)
			}
		}

		sort.Strings(extras)

		for _, fid := range extras {
			fields = append(fields, ExportFieldMeta{ID: fid, Label: def.Fields[fid].Label})
		}

		result = append(result, ExportCategoryMeta{
			ID:            catID,
			Label:         def.Label,
			Fields:        fields,
			DefaultFields: def.DefaultFields,
			Filters:       toFilterMetas(def.FilterSpecs),
		})
	}

	return result
}

// virtualExportFieldSet is the set of field IDs whose SQL expression returns an
// empty string and whose values are filled in by the handler via a
// cross-service call to course-service after the SQL fetch.
//
//nolint:gochecknoglobals // static read-only enrichment metadata
var virtualExportFieldSet = map[string]bool{
	fieldCourseTitle: true,
	fieldPathTitle:   true,
}

// IsVirtualExportField reports whether fieldID needs cross-service enrichment.
func IsVirtualExportField(fieldID string) bool {
	return virtualExportFieldSet[fieldID]
}

// combineWhere prepends baseWhere into whereClause so that baseWhere is always
// the first condition. Returns an empty string when both inputs are empty.
func combineWhere(baseWhere, whereClause string) string {
	if baseWhere == "" {
		return whereClause
	}

	if whereClause == "" {
		return "WHERE " + baseWhere
	}

	return "WHERE " + baseWhere + " AND " + whereClause[len("WHERE "):]
}

// buildWhere constructs a parameterized WHERE clause from active filters.
//
//nolint:gocritic // unnamedResult: string+[]any are distinct types; nonamedreturns bans named returns
func buildWhere(specs []filterSpec, filters map[string]string) (string, []any) {
	var conditions []string

	var args []any

	for _, spec := range specs {
		filterVal, ok := filters[spec.ID]
		if !ok || filterVal == "" {
			continue
		}

		placeholder := "?" + spec.ParamSuffix
		conditions = append(conditions, spec.Expr+" "+spec.Op+" "+placeholder)
		args = append(args, filterVal)
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return "WHERE " + strings.Join(conditions, " AND "), args
}

// scopedBaseWhere adds the scope's condition to a category's base WHERE
// condition. Every category joins the users table as u.
//
//nolint:gocritic // unnamedResult conflicts with project nonamedreturns policy
func scopedBaseWhere(baseWhere string, scope ExportScope) (string, []any) {
	if !scope.Restricted {
		return baseWhere, nil
	}

	condition := "FALSE"

	var args []any

	if len(scope.GroupIDs) > 0 {
		condition = "u.id IN (SELECT ug.userid FROM user_groups ug WHERE ug.groupid::text IN ?)"
		args = []any{scope.GroupIDs}
	}

	if baseWhere == "" {
		return condition, args
	}

	return baseWhere + " AND " + condition, args
}

// buildQuery validates category/fields/filters and returns column headers,
// the SELECT … FROM … WHERE … query and its bound arguments.
//
//nolint:gocritic // unnamedResult conflicts with project nonamedreturns policy
func buildQuery(category string, fields []string, filters map[string]string, scope ExportScope) ([]string, string, []any, error) {
	def, exists := exportCategoryDefs[category]
	if !exists {
		return nil, "", nil, fmt.Errorf("unknown category: %s", category)
	}

	if len(fields) == 0 {
		fields = def.DefaultFields
	}

	selects := make([]string, 0, len(fields))
	headers := make([]string, 0, len(fields))

	for _, fieldKey := range fields {
		fdef, ok := def.Fields[fieldKey]
		if !ok {
			return nil, "", nil, fmt.Errorf("unknown field %q in category %q", fieldKey, category)
		}

		selects = append(selects, fdef.Expr+" AS "+fieldKey)
		headers = append(headers, fieldKey)
	}

	baseWhere, scopeArgs := scopedBaseWhere(def.BaseWhere, scope)
	rawWhere, filterArgs := buildWhere(def.FilterSpecs, filters)
	whereClause := combineWhere(baseWhere, rawWhere)

	args := append(scopeArgs, filterArgs...)

	query := "SELECT " + strings.Join(selects, ", ") + " " + def.FromClause
	if whereClause != "" {
		query += " " + whereClause
	}

	return headers, query, args, nil
}

// visitSQLRows scans sqlRows one row at a time, handing each to visit as a
// string map keyed by column name. Only one row is ever materialized, so a
// caller that writes rows out as they arrive uses constant memory whatever
// the size of the result.
func visitSQLRows(sqlRows *sql.Rows, visit func(map[string]string) error) error {
	cols, colErr := sqlRows.Columns()
	if colErr != nil {
		return fmt.Errorf("columns: %w", colErr)
	}

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))

	for idx := range vals {
		ptrs[idx] = &vals[idx]
	}

	for sqlRows.Next() {
		scanErr := sqlRows.Scan(ptrs...)
		if scanErr != nil {
			return fmt.Errorf("scan: %w", scanErr)
		}

		row := make(map[string]string, len(cols))

		for idx, col := range cols {
			if vals[idx] != nil {
				row[col] = fmt.Sprintf("%v", vals[idx])
			}
		}

		visitErr := visit(row)
		if visitErr != nil {
			return visitErr
		}
	}

	rowErr := sqlRows.Err()
	if rowErr != nil {
		return fmt.Errorf("rows: %w", rowErr)
	}

	return nil
}

// scanSQLRows drains sqlRows into a slice of string maps keyed by column name.
func scanSQLRows(sqlRows *sql.Rows) ([]map[string]string, error) {
	var result []map[string]string

	err := visitSQLRows(sqlRows, func(row map[string]string) error {
		result = append(result, row)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// StreamRows runs the query for category/fields/filters and hands each row
// to visit as it arrives, returning the result's column headers.
//
// It is the unbounded counterpart of FetchRows: no LIMIT, no COUNT, and no
// accumulated slice. An export of every enrollment on the platform is
// written out as it is read rather than assembled in memory first.
func (r *gormExportRepository) StreamRows(
	ctx context.Context,
	category string,
	fields []string,
	filters map[string]string,
	scope ExportScope,
	visit func(row map[string]string) error,
) ([]string, error) {
	headers, query, args, buildErr := buildQuery(category, fields, filters, scope)
	if buildErr != nil {
		return nil, buildErr
	}

	sqlRows, fetchErr := r.db.WithContext(ctx).Raw(query, args...).Rows()
	if fetchErr != nil {
		return nil, fmt.Errorf("fetch rows: %w", fetchErr)
	}

	defer func() { _ = sqlRows.Close() }()

	err := visitSQLRows(sqlRows, visit)
	if err != nil {
		return nil, err
	}

	return headers, nil
}

// FetchRows returns up to limit rows for the given category/fields/filters,
// plus the total matching row count.
//
//nolint:gocritic // unnamedResult conflicts with project nonamedreturns policy
func (r *gormExportRepository) FetchRows(
	ctx context.Context,
	category string,
	fields []string,
	filters map[string]string,
	scope ExportScope,
	limit int,
) ([]string, []map[string]string, int64, error) {
	headers, query, args, buildErr := buildQuery(category, fields, filters, scope)
	if buildErr != nil {
		return nil, nil, 0, buildErr
	}

	def := exportCategoryDefs[category]
	countBase, countArgs := scopedBaseWhere(def.BaseWhere, scope)
	rawCountWhere, countFilterArgs := buildWhere(def.FilterSpecs, filters)
	countWhere := combineWhere(countBase, rawCountWhere)

	countArgs = append(countArgs, countFilterArgs...)

	countQuery := "SELECT COUNT(*) " + def.FromClause
	if countWhere != "" {
		countQuery += " " + countWhere
	}

	var total int64

	countErr := r.db.WithContext(ctx).Raw(countQuery, countArgs...).Scan(&total).Error
	if countErr != nil {
		return nil, nil, 0, fmt.Errorf("count: %w", countErr)
	}

	limitedQuery := query
	if limit > 0 {
		limitedQuery += fmt.Sprintf(" LIMIT %d", limit)
	}

	sqlRows, fetchErr := r.db.WithContext(ctx).Raw(limitedQuery, args...).Rows()
	if fetchErr != nil {
		return nil, nil, 0, fmt.Errorf("fetch rows: %w", fetchErr)
	}

	defer func() { _ = sqlRows.Close() }()

	rows, scanErr := scanSQLRows(sqlRows)
	if scanErr != nil {
		return nil, nil, 0, scanErr
	}

	return headers, rows, total, nil
}

// writeCSVRows streams each row from sqlRows into csvWriter.
func writeCSVRows(sqlRows *sql.Rows, cols []string, csvWriter *csv.Writer) (int, error) {
	rowCount := 0

	for sqlRows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))

		for idx := range vals {
			ptrs[idx] = &vals[idx]
		}

		scanErr := sqlRows.Scan(ptrs...)
		if scanErr != nil {
			return rowCount, fmt.Errorf("scan: %w", scanErr)
		}

		record := make([]string, len(cols))

		for idx := range cols {
			if vals[idx] != nil {
				record[idx] = fmt.Sprintf("%v", vals[idx])
			}
		}

		writeErr := csvWriter.Write(csvsafe.Record(record))
		if writeErr != nil {
			return rowCount, fmt.Errorf("write row: %w", writeErr)
		}

		rowCount++
	}

	return rowCount, nil
}

// WriteCSV streams the full dataset for category/fields/filters as CSV.
func (r *gormExportRepository) WriteCSV(
	ctx context.Context,
	writer io.Writer,
	category string,
	fields []string,
	filters map[string]string,
	scope ExportScope,
) (int, error) {
	headers, query, args, buildErr := buildQuery(category, fields, filters, scope)
	if buildErr != nil {
		return 0, buildErr
	}

	sqlRows, fetchErr := r.db.WithContext(ctx).Raw(query, args...).Rows()
	if fetchErr != nil {
		return 0, fmt.Errorf("fetch rows: %w", fetchErr)
	}

	defer func() { _ = sqlRows.Close() }()

	cols, colErr := sqlRows.Columns()
	if colErr != nil {
		return 0, fmt.Errorf("columns: %w", colErr)
	}

	csvWriter := csv.NewWriter(writer)
	csvWriter.Comma = ';'

	headerErr := csvWriter.Write(headers)
	if headerErr != nil {
		return 0, fmt.Errorf("write header: %w", headerErr)
	}

	rowCount, rowErr := writeCSVRows(sqlRows, cols, csvWriter)
	if rowErr != nil {
		return rowCount, rowErr
	}

	csvWriter.Flush()

	rowsErr := sqlRows.Err()
	if rowsErr != nil {
		return rowCount, fmt.Errorf("rows: %w", rowsErr)
	}

	flushErr := csvWriter.Error()
	if flushErr != nil {
		return rowCount, fmt.Errorf("csv flush: %w", flushErr)
	}

	return rowCount, nil
}

// LogDownload inserts an audit record for a completed CSV download.
func (r *gormExportRepository) LogDownload(ctx context.Context, log *models.ExportLog) error {
	log.DownloadedAt = time.Now()

	err := r.db.WithContext(ctx).Create(log).Error
	if err != nil {
		return fmt.Errorf("create export log: %w", err)
	}

	return nil
}
