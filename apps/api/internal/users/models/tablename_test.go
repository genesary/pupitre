package models

import "testing"

// tableNamer is implemented by every GORM model that pins its table name.
type tableNamer interface {
	TableName() string
}

// TestTableNames pins the table name of every model so an accidental struct
// rename cannot silently change the mapped table.
func TestTableNames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		model tableNamer
		want  string
	}{
		{Enrollment{}, "enrollments"},
		{GroupRoleMapping{}, "group_role_mappings"},
		{GroupEnrollment{}, "group_enrollments"},
		{Group{}, "groups"},
		{ModuleProgress{}, "module_progress"},
		{MarkdownPattern{}, "markdown_patterns"},
		{PathEnrollment{}, "path_enrollments"},
		{SessionBooking{}, "session_bookings"},
		{PlatformSetting{}, "platform_settings"},
		{UserGroup{}, "user_groups"},
		{LessonProgress{}, "lesson_progress"},
		{UserBadge{}, "user_badges"},
		{User{}, "users"},
		{UserXPEvent{}, "user_xp_events"},
		{UserSkillLevel{}, "user_skill_levels"},
	}

	for _, tc := range cases {
		got := tc.model.TableName()
		if got != tc.want {
			t.Errorf("%T.TableName() = %q, want %q", tc.model, got, tc.want)
		}
	}
}
