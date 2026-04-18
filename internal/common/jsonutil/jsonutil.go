package jsonutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ReadFile reads a JSON object from disk. Missing files return an empty object.
func ReadFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return result, nil
}

// WriteFile writes a JSON object to disk with stable indentation.
func WriteFile(path string, data map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	content = append(content, '\n')
	return os.WriteFile(path, content, 0o644)
}

// GetOrCreateObject returns the object stored under key or creates one.
func GetOrCreateObject(data map[string]any, key string) map[string]any {
	if existing, ok := data[key].(map[string]any); ok {
		return existing
	}
	obj := make(map[string]any)
	data[key] = obj
	return obj
}
