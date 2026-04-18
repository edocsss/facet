package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var envVarPattern = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)\}|\$([a-zA-Z_][a-zA-Z0-9_]*)`)

// ExpandPath expands ~ and environment variables in a path.
// After expansion, the path must be absolute.
func ExpandPath(path string) (string, error) {
	// Expand ~ at the start
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %w", err)
		}
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %w", err)
		}
		path = home + path[1:]
	}

	// Expand environment variables: $VAR and ${VAR}
	var expandErr error
	expanded := envVarPattern.ReplaceAllStringFunc(path, func(match string) string {
		if expandErr != nil {
			return match
		}
		// Extract variable name
		submatches := envVarPattern.FindStringSubmatch(match)
		var varName string
		if submatches[1] != "" {
			varName = submatches[1] // ${VAR} form
		} else {
			varName = submatches[2] // $VAR form
		}

		value, ok := os.LookupEnv(varName)
		if !ok {
			expandErr = fmt.Errorf("undefined environment variable: $%s in path %q", varName, path)
			return match
		}
		return value
	})

	if expandErr != nil {
		return "", expandErr
	}

	// Must be absolute after expansion
	if !filepath.IsAbs(expanded) {
		return "", fmt.Errorf("config target path must be absolute after expansion, got: %q", expanded)
	}

	return filepath.Clean(expanded), nil
}

// ValidateSourcePath checks that a source path is relative and stays within the config directory.
func ValidateSourcePath(source, configDir string) error {
	if filepath.IsAbs(source) {
		return fmt.Errorf("config source path must be relative to the config directory, got absolute path: %q", source)
	}

	// Resolve the full path and check it's within configDir
	full := filepath.Join(configDir, source)
	resolved, err := filepath.Abs(full)
	if err != nil {
		return fmt.Errorf("cannot resolve source path %q: %w", source, err)
	}

	absConfigDir, err := filepath.Abs(configDir)
	if err != nil {
		return fmt.Errorf("cannot resolve config directory: %w", err)
	}

	if !strings.HasPrefix(resolved, absConfigDir+string(filepath.Separator)) && resolved != absConfigDir {
		return fmt.Errorf("config source path %q escapes the config directory (resolves to %s, outside %s)", source, resolved, absConfigDir)
	}

	return nil
}
