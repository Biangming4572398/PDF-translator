package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu       sync.RWMutex
	path     string
	settings Settings
}

func NewStore() (*Store, error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(configRoot, AppName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	store := &Store{
		path:     filepath.Join(dir, "config.json"),
		settings: NormalizeSettings(DefaultSettings()),
	}

	if _, err := store.Load(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *Store) Load() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		if err := s.persistLocked(s.settings); err != nil {
			return Settings{}, err
		}

		return s.settings, nil
	}
	if err != nil {
		return Settings{}, err
	}

	settings := DefaultSettings()
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, err
	}

	s.settings = NormalizeSettings(settings)
	return s.settings, nil
}

func (s *Store) Save(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings = NormalizeSettings(settings)

	if err := s.persistLocked(settings); err != nil {
		return err
	}

	s.settings = settings
	return nil
}

func (s *Store) Update(mutator func(*Settings)) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	mutator(&s.settings)
	s.settings = NormalizeSettings(s.settings)

	if err := s.persistLocked(s.settings); err != nil {
		return Settings{}, err
	}

	return s.settings, nil
}

func (s *Store) Current() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.settings
}

func (s *Store) Path() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.path
}

func (s *Store) Directory() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return filepath.Dir(s.path)
}

func (s *Store) persistLocked(settings Settings) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0o644)
}
