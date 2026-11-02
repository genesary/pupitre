package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// EnrollCourse enrolls groupID in courseSlug and backfills enrollments for
// every existing member, returning the number of members newly enrolled.
func (r *gormGroupRepository) EnrollCourse(ctx context.Context, groupID, courseSlug string) (int64, error) {
	groupUUID, err := uuid.Parse(groupID)
	if err != nil {
		return 0, fmt.Errorf("parse group id: %w", err)
	}

	enrollment := models.GroupEnrollment{GroupID: groupUUID, CourseSlug: courseSlug}

	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&enrollment).Error
	if err != nil {
		return 0, fmt.Errorf("enroll group in course: %w", err)
	}

	var userIDs []string

	err = r.db.WithContext(ctx).Model(&models.UserGroup{}).
		Where("groupid = ?::uuid", groupID).
		Pluck("userid", &userIDs).Error
	if err != nil {
		return 0, fmt.Errorf("list group members: %w", err)
	}

	if len(userIDs) == 0 {
		return 0, nil
	}

	enrollments := make([]models.Enrollment, len(userIDs))
	for i, uid := range userIDs {
		enrollments[i] = models.Enrollment{UserID: uid, CourseSlug: courseSlug}
	}

	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&enrollments)
	if result.Error != nil {
		return 0, fmt.Errorf("backfill group course enrollments: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// ListCourseEnrollments mirrors the original hand-rolled join exactly.
func (r *gormGroupRepository) ListCourseEnrollments(ctx context.Context, courseSlug string) ([]GroupEnrollmentRow, error) {
	var rows []GroupEnrollmentRow

	err := r.db.WithContext(ctx).Table("group_enrollments AS ge").
		Select("g.id::text AS id, g.name, g.source, COUNT(ug.userid) AS member_count, ge.createdat::text AS enrolled_at").
		Joins("JOIN groups g ON g.id = ge.groupid").
		Joins("LEFT JOIN user_groups ug ON ug.groupid = ge.groupid").
		Where("ge.courseslug = ?", courseSlug).
		Group("g.id, g.name, g.source, ge.createdat").
		Order("g.name").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list course group enrollments: %w", err)
	}

	return rows, nil
}

// UnenrollCourse removes groupID's enrollment in courseSlug.
func (r *gormGroupRepository) UnenrollCourse(ctx context.Context, groupID, courseSlug string) error {
	err := r.db.WithContext(ctx).
		Where("groupid = ?::uuid AND courseslug = ?", groupID, courseSlug).
		Delete(&models.GroupEnrollment{}).Error
	if err != nil {
		return fmt.Errorf("unenroll group from course: %w", err)
	}

	return nil
}

// GetGroupCourses returns the course slugs that groupID is enrolled in.
func (r *gormGroupRepository) GetGroupCourses(ctx context.Context, groupID string) ([]string, error) {
	var slugs []string

	err := r.db.WithContext(ctx).Table("group_enrollments").
		Where("groupid = ?::uuid", groupID).
		Pluck("courseslug", &slugs).Error
	if err != nil {
		return nil, fmt.Errorf("get group courses: %w", err)
	}

	return slugs, nil
}

// GetGroupCoursesByGroups returns the course slugs each group in groupIDs
// is enrolled in, keyed by group ID.
func (r *gormGroupRepository) GetGroupCoursesByGroups(ctx context.Context, groupIDs []string) (map[string][]string, error) {
	return r.groupedByGroupID(ctx, groupIDs, "group_enrollments", "courseslug", "get group courses by group")
}
