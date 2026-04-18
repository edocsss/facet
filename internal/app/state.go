package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"facet/internal/ai"
	"facet/internal/deploy"
	"facet/internal/packages"
)

const stateFile = ".state.json"

// ApplyState records the result of a facet apply run.
type ApplyState struct {
	Profile      string                   `json:"profile"`
	AppliedAt    time.Time                `json:"applied_at"`
	FacetVersion string                   `json:"facet_version"`
	Packages     []packages.PackageResult `json:"packages"`
	Configs      []deploy.ConfigResult    `json:"configs"`
	AI           *ai.AIState              `json:"ai,omitempty"`
}

// FileStateStore handles reading and writing state to the filesystem.
type FileStateStore struct{}

// NewFileStateStore creates a new FileStateStore.
func NewFileStateStore() *FileStateStore {
	return &FileStateStore{}
}

// Write saves the apply state to .state.json in the state directory.
func (s *FileStateStore) Write(stateDir string, st *ApplyState) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	path := filepath.Join(stateDir, stateFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// Read loads the apply state from .state.json.
// Returns nil, nil if the file does not exist (no previous apply).
func (s *FileStateStore) Read(stateDir string) (*ApplyState, error) {
	path := filepath.Join(stateDir, stateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var st ApplyState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &st, nil
}

// CanaryWrite performs an early write to .state.json to detect permission or disk errors
// before doing any real work.
func (s *FileStateStore) CanaryWrite(stateDir string) error {
	path := filepath.Join(stateDir, stateFile)

	// If file already exists, check we can write to it
	if _, err := os.Stat(path); err == nil {
		f, err := os.OpenFile(path, os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("cannot write to %s: %w", path, err)
		}
		f.Close()
		return nil
	}

	// File doesn't exist — try to create it with a minimal state
	canary := &ApplyState{
		Profile:      "_canary",
		AppliedAt:    time.Now(),
		FacetVersion: "0.1.0",
	}
	return s.Write(stateDir, canary)
}
