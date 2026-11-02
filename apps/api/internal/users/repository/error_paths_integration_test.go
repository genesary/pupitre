//go:build integration

// error_paths_integration_test.go drives every repository method with a
// cancelled context so the `if err != nil { return fmt.Errorf(...) }`
// wrapping branch of each query is exercised. Behind the `integration`
// build tag; run with TEST_DATABASE_URL set.

package repository_test

import (
	"context"
	"io"
	"testing"

	"github.com/google/uuid"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// wantErr fails the test unless err is non-nil.
func wantErr(t *testing.T, name string, err error) {
	t.Helper()

	if err == nil {
		t.Errorf("%s: expected an error on a cancelled context", name)
	}
}

// TestRepository_CancelledContext runs the read/write methods of every
// repository against a context that is already cancelled.
//
//nolint:paralleltest // shared db
func TestRepository_CancelledContext(t *testing.T) {
	gdb := crudDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	id := uuid.New()

	users := repository.NewGormUserRepository(gdb)
	_, err := users.FindByID(ctx, id)
	wantErr(t, "Users.FindByID", err)
	_, err = users.ExistsByEmailOrUsername(ctx, "a@b.c", "a")
	wantErr(t, "Users.ExistsByEmailOrUsername", err)
	wantErr(t, "Users.Create", users.Create(ctx, &models.User{ID: id, Username: "x", Email: "x@x"}))
	_, err = users.CountByRole(ctx, "admin")
	wantErr(t, "Users.CountByRole", err)
	_, err = users.ListForAdmin(ctx, "")
	wantErr(t, "Users.ListForAdmin", err)
	_, err = users.Search(ctx, "q")
	wantErr(t, "Users.Search", err)
	_, err = users.Leaderboard(ctx)
	wantErr(t, "Users.Leaderboard", err)
	_, err = users.ListAuthProviders(ctx)
	wantErr(t, "Users.ListAuthProviders", err)
	wantErr(t, "Users.TouchLastLogin", users.TouchLastLogin(ctx, id))
	wantErr(t, "Users.Delete", users.Delete(ctx, id))

	settings := repository.NewGormSettingRepository(gdb)
	_, _, err = settings.Get(ctx, "k")
	wantErr(t, "Settings.Get", err)
	_, err = settings.List(ctx)
	wantErr(t, "Settings.List", err)
	wantErr(t, "Settings.Upsert", settings.Upsert(ctx, "k", "v"))

	enr := repository.NewGormEnrollmentRepository(gdb)
	wantErr(t, "Enrollments.Create", enr.Create(ctx, id.String(), "c"))
	_, err = enr.Exists(ctx, id.String(), "c")
	wantErr(t, "Enrollments.Exists", err)
	_, err = enr.MyEnrollments(ctx, id.String())
	wantErr(t, "Enrollments.MyEnrollments", err)
	_, err = enr.CountAll(ctx)
	wantErr(t, "Enrollments.CountAll", err)
	_, err = enr.CountDistinctCourses(ctx)
	wantErr(t, "Enrollments.CountDistinctCourses", err)
	_, err = enr.ListByCourse(ctx, "c")
	wantErr(t, "Enrollments.ListByCourse", err)

	lp := repository.NewGormLessonProgressRepository(gdb)
	wantErr(t, "LessonProgress.MarkComplete", lp.MarkComplete(ctx, id.String(), "c", "l"))
	_, err = lp.ViewedSlugs(ctx, id.String(), "c")
	wantErr(t, "LessonProgress.ViewedSlugs", err)
	_, err = lp.CountViewed(ctx, id.String(), "c")
	wantErr(t, "LessonProgress.CountViewed", err)
	_, err = lp.ViewedKeys(ctx, id.String())
	wantErr(t, "LessonProgress.ViewedKeys", err)
	_, err = lp.CompletedCourseSlugs(ctx, id.String(), []string{"c"})
	wantErr(t, "LessonProgress.CompletedCourseSlugs", err)

	mp := repository.NewGormModuleProgressRepository(gdb)
	wantErr(t, "ModuleProgress.RecordProgress",
		mp.RecordProgress(ctx, id.String(), "c", 0, "m", "quiz", 1, 1, true))
	_, err = mp.TotalScore(ctx, id.String(), "c")
	wantErr(t, "ModuleProgress.TotalScore", err)
	_, err = mp.PassedModuleSlugs(ctx, id.String(), "c")
	wantErr(t, "ModuleProgress.PassedModuleSlugs", err)
	_, err = mp.ListByUserCourse(ctx, id.String(), "c")
	wantErr(t, "ModuleProgress.ListByUserCourse", err)
	_, err = mp.PassedKeys(ctx, id.String())
	wantErr(t, "ModuleProgress.PassedKeys", err)

	pe := repository.NewGormPathEnrollmentRepository(gdb)
	wantErr(t, "PathEnrollment.Enroll", pe.Enroll(ctx, id.String(), "p"))
	wantErr(t, "PathEnrollment.EnrollWithCourses", pe.EnrollWithCourses(ctx, id.String(), "p", nil))
	_, err = pe.EnrolledInAny(ctx, id.String(), []string{"p"})
	wantErr(t, "PathEnrollment.EnrolledInAny", err)
	_, err = pe.MyEnrollments(ctx, id.String(), nil, 0)
	wantErr(t, "PathEnrollment.MyEnrollments", err)
	_, err = pe.ListBySlug(ctx, "p")
	wantErr(t, "PathEnrollment.ListBySlug", err)
	wantErr(t, "PathEnrollment.Unenroll", pe.Unenroll(ctx, id.String(), "p"))

	xp := repository.NewGormXPRepository(gdb)
	wantErr(t, "XP.Award", xp.Award(ctx, id.String(), "s", "slug", 10))
	_, err = xp.Total(ctx, id.String())
	wantErr(t, "XP.Total", err)
	_, err = xp.History(ctx, id.String())
	wantErr(t, "XP.History", err)

	badges := repository.NewGormBadgeRepository(gdb)
	wantErr(t, "Badges.Award", badges.Award(ctx, id.String(), "c"))
	_, err = badges.MyBadges(ctx, id.String())
	wantErr(t, "Badges.MyBadges", err)
	_, err = badges.EarnedCount(ctx, "c")
	wantErr(t, "Badges.EarnedCount", err)
	_, err = badges.Leaderboard(ctx, 10)
	wantErr(t, "Badges.Leaderboard", err)

	sl := repository.NewGormSkillLevelRepository(gdb)
	wantErr(t, "SkillLevels.Upsert", sl.Upsert(ctx, id.String(), "s", "beginner", 1))
	_, err = sl.ListByUser(ctx, id.String())
	wantErr(t, "SkillLevels.ListByUser", err)

	sb := repository.NewGormSessionBookingRepository(gdb)
	wantErr(t, "SessionBookings.Book", sb.Book(ctx, id.String(), "c", "s"))
	_, err = sb.ExistsByUser(ctx, id.String(), "c", "s")
	wantErr(t, "SessionBookings.ExistsByUser", err)
	_, err = sb.ListByUser(ctx, id.String())
	wantErr(t, "SessionBookings.ListByUser", err)
	_, err = sb.CountBySession(ctx, "c", "s")
	wantErr(t, "SessionBookings.CountBySession", err)

	groups := repository.NewGormGroupRepository(gdb)
	_, err = groups.Create(ctx, "g")
	wantErr(t, "Groups.Create", err)
	_, err = groups.List(ctx)
	wantErr(t, "Groups.List", err)
	_, err = groups.ListMappings(ctx)
	wantErr(t, "Groups.ListMappings", err)
	_, err = groups.GetGroupsByUserID(ctx, id.String())
	wantErr(t, "Groups.GetGroupsByUserID", err)
	_, err = groups.GetByID(ctx, id.String())
	wantErr(t, "Groups.GetByID", err)
	wantErr(t, "Groups.AddToDefault", groups.AddToDefault(ctx, id.String()))

	// Group hierarchy, members and mappings.
	_, err = groups.CreateAndJoin(ctx, "g", id.String())
	wantErr(t, "Groups.CreateAndJoin", err)
	_, err = groups.CreateSubgroup(ctx, "g", id.String())
	wantErr(t, "Groups.CreateSubgroup", err)
	_, err = groups.Delete(ctx, id.String())
	wantErr(t, "Groups.Delete", err)
	wantErr(t, "Groups.UpsertMapping", groups.UpsertMapping(ctx, "g", "admin"))
	_, err = groups.DeleteMapping(ctx, "g")
	wantErr(t, "Groups.DeleteMapping", err)
	_, err = groups.GetSubgroups(ctx, id.String())
	wantErr(t, "Groups.GetSubgroups", err)
	_, err = groups.GetAncestors(ctx, id.String())
	wantErr(t, "Groups.GetAncestors", err)
	_, err = groups.GetDescendants(ctx, id.String())
	wantErr(t, "Groups.GetDescendants", err)
	_, err = groups.HasChildren(ctx, id.String())
	wantErr(t, "Groups.HasChildren", err)
	_, err = groups.HasMembers(ctx, id.String())
	wantErr(t, "Groups.HasMembers", err)
	wantErr(t, "Groups.UpdateGroup", groups.UpdateGroup(ctx, id.String(), "n"))
	wantErr(t, "Groups.AddMember", groups.AddMember(ctx, id.String(), id.String()))
	wantErr(t, "Groups.RemoveMember", groups.RemoveMember(ctx, id.String(), id.String()))
	_, err = groups.ListMembers(ctx, id.String())
	wantErr(t, "Groups.ListMembers", err)
	_, err = groups.ListMemberIDs(ctx, id.String())
	wantErr(t, "Groups.ListMemberIDs", err)
	_, err = groups.UserInAnyGroup(ctx, id.String(), []string{id.String()})
	wantErr(t, "Groups.UserInAnyGroup", err)
	_, err = groups.GetGroupCourses(ctx, id.String())
	wantErr(t, "Groups.GetGroupCourses", err)
	_, err = groups.EnrollCourse(ctx, id.String(), "c")
	wantErr(t, "Groups.EnrollCourse", err)
	_, err = groups.ListCourseEnrollments(ctx, "c")
	wantErr(t, "Groups.ListCourseEnrollments", err)
	wantErr(t, "Groups.UnenrollCourse", groups.UnenrollCourse(ctx, id.String(), "c"))
	wantErr(t, "Groups.SyncEnrollments", groups.SyncEnrollments(ctx, id.String()))

	// SSO user methods.
	_, err = users.FindByEmail(ctx, "a@b.c")
	wantErr(t, "Users.FindByEmail", err)
	_, err = users.FindByEmailActive(ctx, "a@b.c")
	wantErr(t, "Users.FindByEmailActive", err)
	_, err = users.FindByProviderIdentity(ctx, "oidc", "x")
	wantErr(t, "Users.FindByProviderIdentity", err)
	_, err = users.GetPasswordHash(ctx, id)
	wantErr(t, "Users.GetPasswordHash", err)
	wantErr(t, "Users.UpdatePasswordHash", users.UpdatePasswordHash(ctx, id, "h"))
	_, err = users.ExistsByUsername(ctx, "x")
	wantErr(t, "Users.ExistsByUsername", err)
	_, err = users.GetForAdmin(ctx, id)
	wantErr(t, "Users.GetForAdmin", err)
	_, err = users.ListByGroupIDs(ctx, []string{id.String()})
	wantErr(t, "Users.ListByGroupIDs", err)
	wantErr(t, "Users.CreateAdminIfAbsent", users.CreateAdminIfAbsent(ctx, "u", "e@e", "h"))
	wantErr(t, "Users.UpsertAdminPassword", users.UpsertAdminPassword(ctx, "u", "e@e", "h"))
	_, err = users.UpsertMockStudent(ctx, "u", "e@e", "h")
	wantErr(t, "Users.UpsertMockStudent", err)

	// Patterns.
	pat := repository.NewGormPatternRepository(gdb)
	_, err = pat.ListGlobal(ctx)
	wantErr(t, "Patterns.ListGlobal", err)
	_, err = pat.ListForCourse(ctx, "c")
	wantErr(t, "Patterns.ListForCourse", err)
	_, err = pat.Get(ctx, id)
	wantErr(t, "Patterns.Get", err)
	_, err = pat.Create(ctx, "n", "l", "d", "p", "h", "", "", "global", id.String())
	wantErr(t, "Patterns.Create", err)
	_, err = pat.DeleteByName(ctx, "n")
	wantErr(t, "Patterns.DeleteByName", err)
	wantErr(t, "Patterns.UpsertFromConfig", pat.UpsertFromConfig(ctx, "n", "l", "d", "p", "h", "", "", "global"))

	exp := repository.NewGormExportRepository(gdb)
	_, _, _, err = exp.FetchRows(ctx, "users", []string{"email"}, nil, repository.Unrestricted, 1) //nolint:dogsled // fixed 4-value signature
	wantErr(t, "Export.FetchRows", err)
	_, err = exp.WriteCSV(ctx, io.Discard, "users", []string{"email"}, nil, repository.Unrestricted)
	wantErr(t, "Export.WriteCSV", err)
	wantErr(t, "Export.LogDownload", exp.LogDownload(ctx, &models.ExportLog{UserID: "u", UserEmail: "e", Category: "users"}))
}
