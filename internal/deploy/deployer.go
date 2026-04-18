package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"facet/internal/profile"
)

// Deploy strategies.
const (
	StrategySymlink  = "symlink"
	StrategyTemplate = "template"
)

// ConfigResult records a single deployed config.
type ConfigResult struct {
	Target   string `json:"target"`
	Source   string `json:"source"`
	Strategy string `json:"strategy"` // StrategySymlink or StrategyTemplate
}

// Service is the interface for deployment operations.
type Service interface {
	DeployOne(targetPath, source string, force bool) (ConfigResult, error)
	Unapply(configs []ConfigResult) error
	Rollback() error
	Deployed() []ConfigResult
}

// Deployer handles deploying config files as symlinks or rendered templates.
type Deployer struct {
	configDir    string
	homeDir      string
	vars         map[string]any
	deployed     []ConfigResult     // tracks deployments for rollback
	ownedTargets map[string]bool    // targets that were managed by facet in a previous run
}

// NewDeployer creates a new Deployer.
// ownedConfigs is the list of configs from a previous apply run (used to identify
// facet-owned files that can be overwritten without --force).
func NewDeployer(configDir, homeDir string, vars map[string]any, ownedConfigs []ConfigResult) *Deployer {
	owned := make(map[string]bool, len(ownedConfigs))
	for _, c := range ownedConfigs {
		owned[c.Target] = true
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
func DetectStrategy(sourcePath string) (string, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("cannot stat source %q: %w", sourcePath, err)
	}

	if info.IsDir() {
		return StrategySymlink, nil
	}

	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("cannot read source %q: %w", sourcePath, err)
	}

	if strings.Contains(string(content), "${facet:") {
		return StrategyTemplate, nil
	}

	return StrategySymlink, nil
}

// DeployOne deploys a single config entry.
// targetPath is the absolute expanded target path.
// source is the relative source path within the config directory.
// force replaces existing non-facet files without prompting.
func (d *Deployer) DeployOne(targetPath, source string, force bool) (ConfigResult, error) {
	sourcePath := filepath.Join(d.configDir, source)
	result := ConfigResult{
		Target: targetPath,
		Source: source,
	}

	// Detect strategy
	strategy, err := DetectStrategy(sourcePath)
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
			// Wrong target or different strategy — remove and re-create
			if err := os.Remove(targetPath); err != nil {
				return result, fmt.Errorf("cannot remove existing symlink %s: %w", targetPath, err)
			}
		} else {
			// Regular file or directory exists
			isOwnedByFacet := d.ownedTargets[targetPath]
			if isOwnedByFacet {
				// File was previously managed by facet — overwrite it
				if err := os.Remove(targetPath); err != nil {
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
			return result, fmt.Errorf("template rendering failed for %s: %w", source, err)
		}
		// Preserve source file permissions
		sourceInfo, err := os.Stat(sourcePath)
		if err != nil {
			return result, fmt.Errorf("cannot stat template source %s: %w", sourcePath, err)
		}
		if err := os.WriteFile(targetPath, []byte(rendered), sourceInfo.Mode()); err != nil {
			return result, fmt.Errorf("cannot write rendered template to %s: %w", targetPath, err)
		}
	}

	d.deployed = append(d.deployed, result)
	return result, nil
}

// Unapply removes previously deployed configs based on state records.
func (d *Deployer) Unapply(configs []ConfigResult) error {
	for _, cfg := range configs {
		if err := os.Remove(cfg.Target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cannot remove %s during unapply: %w", cfg.Target, err)
		}
	}
	return nil
}

// Rollback removes all configs deployed during this session.
func (d *Deployer) Rollback() error {
	return d.Unapply(d.deployed)
}
