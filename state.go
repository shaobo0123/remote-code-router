package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const stateFileName = "remote-code-router.state.yaml"

type routerState struct {
	ActiveCandidate string `yaml:"active_candidate" json:"active_candidate"`
}

type stateStore struct {
	mu   sync.RWMutex
	path string
	data routerState
}

func newStateStore(pluginDir string, cfg PluginConfig) *stateStore {
	path := statePath(pluginDir)
	store := &stateStore{
		path: path,
		data: routerState{ActiveCandidate: cfg.ActiveCandidate},
	}
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		var loaded routerState
		if yaml.Unmarshal(raw, &loaded) == nil && strings.TrimSpace(loaded.ActiveCandidate) != "" {
			store.data.ActiveCandidate = strings.ToLower(strings.TrimSpace(loaded.ActiveCandidate))
		}
	}
	return store
}

func statePath(pluginDir string) string {
	pluginDir = strings.TrimSpace(pluginDir)
	if pluginDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			pluginDir = cwd
		}
	}
	if pluginDir == "" {
		return stateFileName
	}
	return filepath.Join(pluginDir, stateFileName)
}

func (s *stateStore) activeCandidate(fallback string) string {
	if s == nil {
		return normalizeActiveCandidate(fallback)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if strings.TrimSpace(s.data.ActiveCandidate) == "" {
		return normalizeActiveCandidate(fallback)
	}
	return normalizeActiveCandidate(s.data.ActiveCandidate)
}

func (s *stateStore) setActiveCandidate(candidate string) error {
	if s == nil {
		return nil
	}
	candidate = normalizeActiveCandidate(candidate)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.ActiveCandidate = candidate
	if dir := filepath.Dir(s.path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	raw, err := yaml.Marshal(s.data)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

func normalizeActiveCandidate(candidate string) string {
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	if candidate == "" {
		return activeCandidateAuto
	}
	return candidate
}
