package app

import (
	"fmt"
	"os"
	"path/filepath"

	"facet/internal/deploy"
)

// StatusOpts holds options for the Status operation.
type StatusOpts struct {
	ConfigDir string
	StateDir  string
}

// Status displays the current facet status.
func (a *App) Status(opts StatusOpts) error {
	s, err := a.stateStore.Read(opts.StateDir)
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	if s == nil {
		a.printNoState()
		return nil
	}

	checks := runValidityChecks(s, opts.ConfigDir)
	a.printStatus(s, checks)

	return nil
}

// runValidityChecks checks that deployed configs are still valid.
func runValidityChecks(s *ApplyState, cfgDir string) []ValidityCheck {
	var checks []ValidityCheck

	for _, cfg := range s.Configs {
		check := ValidityCheck{Target: cfg.Target}

		info, err := os.Lstat(cfg.Target)
		if err != nil {
			check.Valid = false
			check.Error = "file missing"
			checks = append(checks, check)
			continue
		}

		switch cfg.Strategy {
		case deploy.StrategySymlink:
			if info.Mode()&os.ModeSymlink == 0 {
				check.Valid = false
				check.Error = "expected symlink, found regular file"
			} else {
				symlinkTarget, err := os.Readlink(cfg.Target)
				if err != nil {
					check.Valid = false
					check.Error = "cannot read symlink target"
				} else if _, err := os.Stat(symlinkTarget); err != nil {
					check.Valid = false
					check.Error = "symlink target does not exist (broken symlink)"
				} else if cfgDir != "" {
					expectedSource := filepath.Join(cfgDir, cfg.Source)
					if symlinkTarget != expectedSource {
						check.Valid = false
						check.Error = fmt.Sprintf("symlink points to wrong source: got %s, want %s", symlinkTarget, expectedSource)
					} else {
						check.Valid = true
					}
				} else {
					check.Valid = true
				}
			}
		case deploy.StrategyTemplate:
			check.Valid = true
		default:
			check.Valid = true
		}

		checks = append(checks, check)
	}

	return checks
}
