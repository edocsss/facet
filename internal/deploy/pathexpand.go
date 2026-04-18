package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"facet/internal/profile"
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

type SourceSpec struct {
	DisplaySource string
	ResolvedPath  string
	Materialize   bool
	SourceRoot    string
}

func ResolveSourcePath(source string, meta profile.ConfigProvenance, localConfigDir string) (SourceSpec, error) {
	root := meta.SourceRoot
	if root == "" {
		root = localConfigDir
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return SourceSpec{}, fmt.Errorf("cannot resolve source root: %w", err)
	}

	if filepath.IsAbs(source) {
		if meta.Materialize {
			return SourceSpec{}, fmt.Errorf("config source path %q must be relative for git-based bases", source)
		}
		return SourceSpec{
			DisplaySource: source,
			ResolvedPath:  filepath.Clean(source),
			Materialize:   meta.Materialize,
			SourceRoot:    absRoot,
		}, nil
	}

	resolved := filepath.Join(root, source)
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return SourceSpec{}, fmt.Errorf("cannot resolve source path %q: %w", source, err)
	}

	if !strings.HasPrefix(absResolved, absRoot+string(filepath.Separator)) && absResolved != absRoot {
		return SourceSpec{}, fmt.Errorf("config source path %q escapes source root %s", source, absRoot)
	}

	return SourceSpec{
		DisplaySource: source,
		ResolvedPath:  absResolved,
		Materialize:   meta.Materialize,
		SourceRoot:    absRoot,
	}, nil
}
