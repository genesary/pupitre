package fake

import (
	"context"
	"sync"
	"time"

	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
)

// SettingRepository is an in-memory repository.SettingRepository for tests.
type SettingRepository struct {
	mu       sync.Mutex
	settings map[string]models.PlatformSetting

	// Err, when set, is returned by every method call (simulates a DB failure).
	Err error
}

// NewSettingRepository builds a fake SettingRepository seeded with settings.
func NewSettingRepository(seed ...models.PlatformSetting) *SettingRepository {
	m := make(map[string]models.PlatformSetting, len(seed))
	for _, s := range seed {
		m[s.Key] = s
	}

	return &SettingRepository{settings: m}
}

// Get returns the value for key, whether it was found, and any error.
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func (f *SettingRepository) Get(_ context.Context, key string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return "", false, f.Err
	}

	s, ok := f.settings[key]
	if !ok {
		return "", false, nil
	}

	return s.Value, true, nil
}

// List returns all platform settings.
func (f *SettingRepository) List(_ context.Context) ([]models.PlatformSetting, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}

	list := make([]models.PlatformSetting, 0, len(f.settings))
	for _, s := range f.settings {
		list = append(list, s)
	}

	return list, nil
}

// Upsert creates or updates the setting identified by key with value.
func (f *SettingRepository) Upsert(_ context.Context, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}

	f.settings[key] = models.PlatformSetting{Key: key, Value: value, UpdatedAt: time.Now()}

	return nil
}
