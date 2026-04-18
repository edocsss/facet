package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CommandRunner interface {
	Run(name string, args ...string) error
}

type ResolvedBase struct {
	Config  *FacetConfig
	Cleanup func() error
}

type BaseResolver struct {
	loader *Loader
	runner CommandRunner
}

func NewBaseResolver(loader *Loader, runner CommandRunner) *BaseResolver {
	return &BaseResolver{
		loader: loader,
		runner: runner,
	}
}

func (r *BaseResolver) Resolve(rawExtends, localConfigDir string) (*ResolvedBase, error) {
	spec, err := ParseExtends(rawExtends)
	if err != nil {
		return nil, err
	}
	spec = reinterpretExistingFileURL(rawExtends, spec)

	switch spec.Kind {
	case ExtendsFile:
		path, err := resolveLocalPath(localConfigDir, spec.Locator)
		if err != nil {
			return nil, err
		}
		cfg, err := r.loader.LoadConfig(path)
		if err != nil {
			return nil, err
		}
		AnnotateLayer(cfg, filepath.Dir(path), false)
		return &ResolvedBase{
			Config:  cfg,
			Cleanup: noopCleanup,
		}, nil
	case ExtendsDir:
		root, err := resolveLocalPath(localConfigDir, spec.Locator)
		if err != nil {
			return nil, err
		}
		cfg, err := r.loader.LoadConfig(filepath.Join(root, "base.yaml"))
		if err != nil {
			return nil, err
		}
		AnnotateLayer(cfg, root, false)
		return &ResolvedBase{
			Config:  cfg,
			Cleanup: noopCleanup,
		}, nil
	case ExtendsGit:
		if r.runner == nil {
			return nil, fmt.Errorf("git extends requires a command runner")
		}

		tmpDir, err := os.MkdirTemp("", "facet-extends-*")
		if err != nil {
			return nil, err
		}
		cleanup := func() error {
			return os.RemoveAll(tmpDir)
		}

		if spec.Ref == "" {
			err = r.runner.Run("git", "clone", "--depth", "1", spec.Locator, tmpDir)
		} else {
			err = r.runner.Run("git", "clone", spec.Locator, tmpDir)
		}
		if err != nil {
			_ = cleanup()
			return nil, fmt.Errorf("git clone failed for %q: %w", spec.Locator, err)
		}

		if spec.Ref != "" {
			if err := r.runner.Run("git", "-C", tmpDir, "checkout", spec.Ref); err != nil {
				_ = cleanup()
				return nil, fmt.Errorf("git checkout failed for ref %q: %w", spec.Ref, err)
			}
		}

		cfg, err := r.loader.LoadConfig(filepath.Join(tmpDir, "base.yaml"))
		if err != nil {
			_ = cleanup()
			return nil, err
		}
		AnnotateLayer(cfg, tmpDir, true)
		return &ResolvedBase{
			Config:  cfg,
			Cleanup: cleanup,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported extends kind %q", spec.Kind)
	}
}

func resolveLocalPath(localConfigDir, locator string) (string, error) {
	if locator == "~" || strings.HasPrefix(locator, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~ in extends locator %q: %w", locator, err)
		}
		if locator == "~" {
			return homeDir, nil
		}
		return filepath.Join(homeDir, locator[2:]), nil
	}
	if filepath.IsAbs(locator) {
		return filepath.Clean(locator), nil
	}
	path := filepath.Join(localConfigDir, locator)
	return filepath.Abs(path)
}

func noopCleanup() error {
	return nil
}

func reinterpretExistingFileURL(rawExtends string, spec ExtendsSpec) ExtendsSpec {
	trimmed := strings.TrimSpace(rawExtends)
	if spec.Kind != ExtendsGit || spec.Ref == "" || !strings.HasPrefix(trimmed, "file://") {
		return spec
	}

	path := strings.TrimPrefix(trimmed, "file://")
	if path == "" {
		return spec
	}

	if _, err := os.Stat(path); err == nil {
		return ExtendsSpec{
			Raw:     rawExtends,
			Kind:    ExtendsGit,
			Locator: trimmed,
		}
	}

	return spec
}
