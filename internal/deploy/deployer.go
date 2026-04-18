package deploy

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"facet/internal/profile"
)

// Deploy strategies.
const (
	StrategySymlink  = "symlink"
	StrategyTemplate = "template"
	StrategyCopy     = "copy"
)

// ConfigResult records a single deployed config.
type ConfigResult struct {
	Target     string `json:"target"`
	Source     string `json:"source"`
	SourcePath string `json:"source_path,omitempty"`
	IsDir      bool   `json:"is_dir,omitempty"`
	Strategy   string `json:"strategy"` // StrategySymlink, StrategyTemplate, or StrategyCopy
}

// Service is the interface for deployment operations.
type Service interface {
	DeployOne(targetPath string, source SourceSpec, force bool) (ConfigResult, error)
	Unapply(configs []ConfigResult) error
	Rollback() error
	Deployed() []ConfigResult
}

// Deployer handles deploying config files as symlinks or rendered templates.
type Deployer struct {
	configDir    string
	homeDir      string
	vars         map[string]any
	deployed     []ConfigResult          // tracks deployments for rollback
	ownedTargets map[string]ConfigResult // targets that were managed by facet in a previous run
}

// NewDeployer creates a new Deployer.
// ownedConfigs is the list of configs from a previous apply run (used to identify
// facet-owned files that can be overwritten without --force).
func NewDeployer(configDir, homeDir string, vars map[string]any, ownedConfigs []ConfigResult) *Deployer {
	owned := make(map[string]ConfigResult, len(ownedConfigs))
	for _, c := range ownedConfigs {
		owned[c.Target] = c
	}
	return &Deployer{
		configDir:    configDir,
		homeDir:      homeDir,
		vars:         vars,
		ownedTargets: owned,
	}
}

// Deployed returns the list of configs deployed during this session.
func (d *Deployer) Deployed() []ConfigResult {
	return d.deployed
}

// DetectStrategy determines whether a source path should be symlinked or templated.
func DetectStrategy(sourcePath string, materialize bool) (string, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("cannot stat source %q: %w", sourcePath, err)
	}

	if info.IsDir() {
		if materialize {
			return StrategyCopy, nil
		}
		return StrategySymlink, nil
	}

	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("cannot read source %q: %w", sourcePath, err)
	}

	if strings.Contains(string(content), "${facet:") {
		return StrategyTemplate, nil
	}

	if materialize {
		return StrategyCopy, nil
	}

	return StrategySymlink, nil
}

// DeployOne deploys a single config entry.
// targetPath is the absolute expanded target path.
// source is the relative source path within the config directory.
// force replaces existing non-facet files without prompting.
func (d *Deployer) DeployOne(targetPath string, source SourceSpec, force bool) (ConfigResult, error) {
	sourcePath := source.ResolvedPath
	result := ConfigResult{
		Target:     targetPath,
		Source:     source.DisplaySource,
		SourcePath: source.ResolvedPath,
	}
	if source.Materialize {
		if err := validateMaterializedSource(source.ResolvedPath, source.SourceRoot); err != nil {
			return result, err
		}
	}
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return result, fmt.Errorf("cannot stat source %q: %w", sourcePath, err)
	}
	result.IsDir = sourceInfo.IsDir()

	// Detect strategy
	strategy, err := DetectStrategy(sourcePath, source.Materialize)
	if err != nil {
		return result, err
	}
	result.Strategy = strategy

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return result, fmt.Errorf("cannot create parent directory for %s: %w", targetPath, err)
	}

	// Check existing target
	existingInfo, err := os.Lstat(targetPath)
	if err == nil {
		// Target exists
		if existingInfo.Mode()&os.ModeSymlink != 0 {
			// It's a symlink — check if it points to the right place
			currentTarget, err := os.Readlink(targetPath)
			if err == nil && currentTarget == sourcePath && strategy == StrategySymlink {
				// Already correct — no-op
				d.deployed = append(d.deployed, result)
				return result, nil
			}
			if !force {
				ownedConfig, isOwnedByFacet := d.ownedTargets[targetPath]
				if !isOwnedByFacet || ownedConfig.Strategy != StrategySymlink {
					return result, fmt.Errorf("target %s already exists as a symlink and is not managed by facet — use --force to replace", targetPath)
				}

				expectedTarget := d.expectedSymlinkTarget(ownedConfig)
				if currentTarget != expectedTarget {
					return result, fmt.Errorf("refusing to replace unmanaged symlink %s: current target %s does not match recorded source %s", targetPath, currentTarget, expectedTarget)
				}
			}

			if err := os.Remove(targetPath); err != nil {
				return result, fmt.Errorf("cannot remove existing symlink %s: %w", targetPath, err)
			}
		} else {
			// Regular file or directory exists
			ownedConfig, isOwnedByFacet := d.ownedTargets[targetPath]
			if isOwnedByFacet {
				// File was previously managed by facet — overwrite it
				if existingInfo.IsDir() && !ownedConfig.IsDir {
					return result, fmt.Errorf("refusing to remove unexpected directory at %s for previously file-managed target", targetPath)
				}
				removeFn := os.Remove
				if existingInfo.IsDir() {
					removeFn = os.RemoveAll
				}
				if err := removeFn(targetPath); err != nil {
					return result, fmt.Errorf("cannot remove existing file %s: %w", targetPath, err)
				}
			} else if force {
				// User file, but --force was given — replace
				if err := os.RemoveAll(targetPath); err != nil {
					return result, fmt.Errorf("cannot remove existing file %s: %w", targetPath, err)
				}
			} else {
				return result, fmt.Errorf("target %s already exists and is not managed by facet — use --force to replace", targetPath)
			}
		}
	}

	// Deploy
	switch strategy {
	case StrategySymlink:
		if err := os.Symlink(sourcePath, targetPath); err != nil {
			return result, fmt.Errorf("cannot create symlink %s → %s: %w", targetPath, sourcePath, err)
		}
	case StrategyTemplate:
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			return result, fmt.Errorf("cannot read template source %s: %w", sourcePath, err)
		}
		rendered, err := profile.SubstituteVars(string(content), d.vars)
		if err != nil {
			return result, fmt.Errorf("template rendering failed for %s: %w", source.DisplaySource, err)
		}
		// Preserve source file permissions
		sourceInfo, err := os.Stat(sourcePath)
		if err != nil {
			return result, fmt.Errorf("cannot stat template source %s: %w", sourcePath, err)
		}
		if err := os.WriteFile(targetPath, []byte(rendered), sourceInfo.Mode()); err != nil {
			return result, fmt.Errorf("cannot write rendered template to %s: %w", targetPath, err)
		}
	case StrategyCopy:
		if err := copyPath(sourcePath, targetPath); err != nil {
			return result, err
		}
	}

	d.deployed = append(d.deployed, result)
	return result, nil
}

// Unapply removes previously deployed configs based on state records.
func (d *Deployer) Unapply(configs []ConfigResult) error {
	for _, cfg := range configs {
		info, err := os.Lstat(cfg.Target)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("cannot inspect %s during unapply: %w", cfg.Target, err)
		}

		if cfg.Strategy == StrategySymlink {
			if info.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("refusing to remove non-symlink at %s for previously symlink-managed target", cfg.Target)
			}

			currentTarget, err := os.Readlink(cfg.Target)
			if err != nil {
				return fmt.Errorf("cannot read symlink target %s during unapply: %w", cfg.Target, err)
			}

			expectedTarget := d.expectedSymlinkTarget(cfg)
			if expectedTarget != "" && currentTarget != expectedTarget {
				return fmt.Errorf("refusing to remove repointed symlink %s: current target %s does not match recorded source %s", cfg.Target, currentTarget, expectedTarget)
			}

			if err := os.Remove(cfg.Target); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("cannot remove %s during unapply: %w", cfg.Target, err)
			}
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to remove unexpected symlink at %s for previously non-symlink-managed target", cfg.Target)
		}

		if info.IsDir() {
			if !cfg.IsDir {
				return fmt.Errorf("refusing to remove unexpected directory at %s for previously file-managed target", cfg.Target)
			}
			if err := os.RemoveAll(cfg.Target); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("cannot remove %s during unapply: %w", cfg.Target, err)
			}
			continue
		}

		if err := os.Remove(cfg.Target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cannot remove %s during unapply: %w", cfg.Target, err)
		}
	}
	return nil
}

// Rollback removes all configs deployed during this session.
func (d *Deployer) Rollback() error {
	return d.Unapply(d.deployed)
}

func (d *Deployer) expectedSymlinkTarget(cfg ConfigResult) string {
	if cfg.SourcePath != "" {
		return filepath.Clean(cfg.SourcePath)
	}
	if cfg.Source == "" {
		return ""
	}
	return filepath.Clean(filepath.Join(d.configDir, cfg.Source))
}

func validateMaterializedSource(sourcePath, sourceRoot string) error {
	if sourceRoot == "" {
		return nil
	}

	realRoot, err := filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return fmt.Errorf("cannot resolve source root %q: %w", sourceRoot, err)
	}

	return filepath.WalkDir(sourcePath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("cannot resolve materialized source %q: %w", path, err)
		}
		if !isWithinRoot(realPath, realRoot) {
			return fmt.Errorf("materialized source %q escapes source root %s", path, realRoot)
		}

		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("materialized source %q must not contain symlinks", path)
		}
		return nil
	})
}

func isWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func copyPath(sourcePath, targetPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot stat source %q: %w", sourcePath, err)
	}

	if info.IsDir() {
		return copyDir(sourcePath, targetPath, info.Mode())
	}
	return copyFile(sourcePath, targetPath, info.Mode())
}

func copyDir(sourceDir, targetDir string, mode os.FileMode) error {
	if err := os.MkdirAll(targetDir, mode); err != nil {
		return fmt.Errorf("cannot create target directory %s: %w", targetDir, err)
	}

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("cannot read source directory %s: %w", sourceDir, err)
	}

	for _, entry := range entries {
		src := filepath.Join(sourceDir, entry.Name())
		dst := filepath.Join(targetDir, entry.Name())

		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("cannot stat source directory %s: %w", src, err)
			}
			if err := copyDir(src, dst, info.Mode()); err != nil {
				return err
			}
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("cannot stat source file %s: %w", src, err)
		}
		if err := copyFile(src, dst, info.Mode()); err != nil {
			return err
		}
	}

	return nil
}

func copyFile(sourcePath, targetPath string, mode os.FileMode) error {
	src, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot open source file %s: %w", sourcePath, err)
	}
	defer src.Close()

	dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("cannot create target file %s: %w", targetPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("cannot copy %s to %s: %w", sourcePath, targetPath, err)
	}

	return nil
}
