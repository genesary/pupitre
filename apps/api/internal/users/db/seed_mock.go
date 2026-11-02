package db

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"golang.org/x/crypto/bcrypt"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// mockStudentPassword is the cleartext password used for all mock students.
// Hash of "Student@1234" (cost 12).
const mockStudentPassword = "Student@1234"

// Mock student usernames. Each is defined once and reused both as the
// seeded username and as the corresponding mockEnrollments map key.
const (
	userAmandineDupont = "amandine.dupont"
	userTheoMartin     = "theo.martin"
	userLenaBernard    = "lena.bernard"
	userKarimBenali    = "karim.benali"
	userSofiaIglesias  = "sofia.iglesias"
	userLucasPetit     = "lucas.petit"
	userYasmineToure   = "yasmine.toure"
	userMaximeLeroy    = "maxime.leroy"
	userInesFontaine   = "ines.fontaine"
	userRayanCherif    = "rayan.cherif"
	userJulieMoreau    = "julie.moreau"
	userBaptisteSimon  = "baptiste.simon"
	userChiaraRusso    = "chiara.russo"
	userAdrienLambert  = "adrien.lambert"
	userFatouDiallo    = "fatou.diallo"
	userHugoRenard     = "hugo.renard"
	userCamilleVidal   = "camille.vidal"
	userOmarNdiaye     = "omar.ndiaye"
	userAliceGarnier   = "alice.garnier"
	userMehdiBoukhari  = "mehdi.boukhari"
	userPaulineAubert  = "pauline.aubert"
	userTomLeclerc     = "tom.leclerc"
	userNadiaKhelifi   = "nadia.khelifi"
	userVincentPerez   = "vincent.perez"
	userElisaNguyen    = "elisa.nguyen"
)

// Mock course slugs referenced by more than one mock student's enrollments.
const (
	courseDockerFundamentals   = "docker-fundamentals"
	courseKubernetesBasics     = "kubernetes-basics"
	courseCybersecurityIntro   = "cybersecurity-intro"
	courseGitAdvanced          = "git-advanced"
	courseLinuxIntro           = "linux-intro"
	courseNetworkingEssentials = "networking-essentials"
	coursePythonBasics         = "python-basics"
)

// mockUser is a mock student record seeded by SeedMockData.
type mockUser struct {
	username string
	email    string
}

// mockUsers is the static list of mock student accounts seeded when
// SEED_MOCK_DATA=true. It is read-only and only ever consumed by
// SeedMockData.
//
//nolint:gochecknoglobals // static seed data table, read-only, used only by SeedMockData
var mockUsers = []mockUser{
	{userAmandineDupont, "amandine.dupont@example.com"},
	{userTheoMartin, "theo.martin@example.com"},
	{userLenaBernard, "lena.bernard@example.com"},
	{userKarimBenali, "karim.benali@example.com"},
	{userSofiaIglesias, "sofia.iglesias@example.com"},
	{userLucasPetit, "lucas.petit@example.com"},
	{userYasmineToure, "yasmine.toure@example.com"},
	{userMaximeLeroy, "maxime.leroy@example.com"},
	{userInesFontaine, "ines.fontaine@example.com"},
	{userRayanCherif, "rayan.cherif@example.com"},
	{userJulieMoreau, "julie.moreau@example.com"},
	{userBaptisteSimon, "baptiste.simon@example.com"},
	{userChiaraRusso, "chiara.russo@example.com"},
	{userAdrienLambert, "adrien.lambert@example.com"},
	{userFatouDiallo, "fatou.diallo@example.com"},
	{userHugoRenard, "hugo.renard@example.com"},
	{userCamilleVidal, "camille.vidal@example.com"},
	{userOmarNdiaye, "omar.ndiaye@example.com"},
	{userAliceGarnier, "alice.garnier@example.com"},
	{userMehdiBoukhari, "mehdi.boukhari@example.com"},
	{userPaulineAubert, "pauline.aubert@example.com"},
	{userTomLeclerc, "tom.leclerc@example.com"},
	{userNadiaKhelifi, "nadia.khelifi@example.com"},
	{userVincentPerez, "vincent.perez@example.com"},
	{userElisaNguyen, "elisa.nguyen@example.com"},
}

// mockEnrollments maps username → list of course slugs.
//
//nolint:gochecknoglobals // static seed data table, read-only, used only by SeedMockData
var mockEnrollments = map[string][]string{
	userAmandineDupont: {courseLinuxIntro, courseDockerFundamentals, coursePythonBasics},
	userTheoMartin:     {courseLinuxIntro, courseKubernetesBasics, courseDockerFundamentals},
	userLenaBernard:    {coursePythonBasics},
	userKarimBenali:    {courseLinuxIntro, courseNetworkingEssentials, courseCybersecurityIntro},
	userSofiaIglesias:  {coursePythonBasics, courseLinuxIntro},
	userLucasPetit:     {courseLinuxIntro, courseDockerFundamentals, courseGitAdvanced, courseKubernetesBasics},
	userYasmineToure:   {courseNetworkingEssentials, courseCybersecurityIntro},
	userMaximeLeroy:    {courseLinuxIntro, coursePythonBasics, courseDockerFundamentals},
	userInesFontaine:   {courseGitAdvanced, courseDockerFundamentals},
	userRayanCherif:    {courseLinuxIntro, courseNetworkingEssentials},
	userJulieMoreau:    {coursePythonBasics, courseCybersecurityIntro},
	userBaptisteSimon:  {courseLinuxIntro, courseKubernetesBasics, courseNetworkingEssentials, courseGitAdvanced},
	userChiaraRusso:    {coursePythonBasics, courseLinuxIntro},
	userAdrienLambert:  {courseDockerFundamentals, courseKubernetesBasics, courseCybersecurityIntro},
	userFatouDiallo:    {courseLinuxIntro, coursePythonBasics, courseNetworkingEssentials},
	userHugoRenard:     {courseLinuxIntro, courseDockerFundamentals},
	userCamilleVidal:   {coursePythonBasics, courseGitAdvanced},
	userOmarNdiaye:     {courseLinuxIntro, courseNetworkingEssentials, courseCybersecurityIntro},
	userAliceGarnier:   {coursePythonBasics, courseDockerFundamentals, courseKubernetesBasics},
	userMehdiBoukhari:  {courseLinuxIntro, courseCybersecurityIntro},
	userPaulineAubert:  {coursePythonBasics},
	userTomLeclerc:     {courseLinuxIntro, courseDockerFundamentals, courseGitAdvanced},
	userNadiaKhelifi:   {courseNetworkingEssentials, courseCybersecurityIntro, courseLinuxIntro},
	userVincentPerez:   {courseKubernetesBasics, courseDockerFundamentals},
	userElisaNguyen:    {coursePythonBasics, courseLinuxIntro, courseGitAdvanced},
}

// SeedMockData inserts mock student users and their enrollments.
// It is idempotent: existing rows are skipped via ON CONFLICT DO NOTHING.
// Activate with SEED_MOCK_DATA=true at startup.
func SeedMockData(ctx context.Context, users repository.UserRepository, enrollments repository.EnrollmentRepository) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(mockStudentPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("hash mock student password: %w", err)
	}

	passwordHash := string(hash)

	for _, student := range mockUsers {
		userID, err := users.UpsertMockStudent(ctx, student.username, student.email, passwordHash)
		if err != nil {
			zap.L().Error("mock seed: failed to upsert user", zap.String("username", student.username), zap.Error(err))

			continue
		}

		for _, slug := range mockEnrollments[student.username] {
			err := enrollments.Create(ctx, userID.String(), slug)
			if err != nil {
				zap.L().Error("mock seed: failed to enroll user", zap.String("username", student.username), zap.String("course", slug), zap.Error(err))
			}
		}
	}

	zap.L().Info("mock seed: inserted students and enrollments", zap.Int("count", len(mockUsers)))

	return nil
}
