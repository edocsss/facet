package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"facet/internal/ai"
	"facet/internal/deploy"
	"facet/internal/packages"
	"facet/internal/profile"
)

// validStages returns the ordered list of stage names that --stages accepts.
func validStages() []string {
	return []string{"configs", "pre_apply", "packages", "post_apply", "ai"}
}

// ValidStagesCSV returns the valid stage names as a comma-separated string for help text.
func ValidStagesCSV() string {
	return strings.Join(validStages(), ", ")
}

// parseStages parses a comma-separated stages string into a set.
// An empty string means all stages. Returns an error for invalid stage names.
func parseStages(raw string) (map[string]bool, error) {
	all := validStages()
	if raw == "" {
		result := make(map[string]bool, len(all))
		for _, s := range all {
			result[s] = true
		}
		return result, nil
	}

	result := make(map[string]bool)
	valid := make(map[string]bool, len(all))
	for _, s := range all {
		valid[s] = true
	}

	for _, part := range strings.Split(raw, ",") {
		s := strings.TrimSpace(part)
		if s == "" {
			continue
		}
		if !valid[s] {
			return nil, fmt.Errorf("unknown stage %q; valid stages: %s", s, strings.Join(all, ", "))
		}
		result[s] = true
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid stages specified; valid stages: %s", strings.Join(all, ", "))
	}

	return result, nil
}

// ApplyOpts holds options for the Apply operation.
type ApplyOpts struct {
	ConfigDir   string
	StateDir    string
	Force       bool
	SkipFailure bool
	DryRun      bool
	Stages      string
}

// Apply loads, merges, and applies a configuration profile.
func (a *App) Apply(profileName string, opts ApplyOpts) error {
	stages, err := parseStages(opts.Stages)
	if err != nil {
		return err
	}

	// Step 1: Load facet.yaml
	_, err = a.loader.LoadMeta(opts.ConfigDir)
	if err != nil {
		return fmt.Errorf("Not a facet config directory (facet.yaml not found). Use -c to specify the config directory, or run facet scaffold to create one.\n  detail: %w", err)
	}

	// Step 2: Load base.yaml
	baseCfg, err := a.loader.LoadConfig(filepath.Join(opts.ConfigDir, "base.yaml"))
	if err != nil {
		return err
	}

	// Step 3: Load profile
	profilePath := filepath.Join(opts.ConfigDir, "profiles", profileName+".yaml")
	profileCfg, err := a.loader.LoadConfig(profilePath)
	if err != nil {
		return fmt.Errorf("cannot load profile %q: %w", profileName, err)
	}
	if err := profile.ValidateProfile(profileCfg); err != nil {
		return err
	}

	// Step 4: Load .local.yaml
	localPath := filepath.Join(opts.StateDir, ".local.yaml")
	localCfg, err := a.loader.LoadConfig(localPath)
	if err != nil {
		return fmt.Errorf(".local.yaml is required in %s: %w", opts.StateDir, err)
	}

	// Step 5: Merge layers
	merged, err := profile.Merge(baseCfg, profileCfg)
	if err != nil {
		return fmt.Errorf("merge error: %w", err)
	}
	merged, err = profile.Merge(merged, localCfg)
	if err != nil {
		return fmt.Errorf("merge error with .local.yaml: %w", err)
	}
	if err := profile.ValidateMergedConfig(merged); err != nil {
		return err
	}

	// Step 6: Resolve variables
	resolved, err := profile.Resolve(merged)
	if err != nil {
		return err
	}

	// Dry-run: preview what would happen without side effects
	if opts.DryRun {
		return a.printDryRun(profileName, resolved, opts)
	}

	// Step 7: Canary write to .state.json
	if err := os.MkdirAll(opts.StateDir, 0o755); err != nil {
		return fmt.Errorf("cannot create state directory %s: %w", opts.StateDir, err)
	}
	if err := a.stateStore.CanaryWrite(opts.StateDir); err != nil {
		return fmt.Errorf("cannot write state file: %w", err)
	}

	// Read previous state for unapply
	prevState, err := a.stateStore.Read(opts.StateDir)
	if err != nil {
		a.reporter.Warning(fmt.Sprintf("Could not read previous state: %v", err))
	}

	var prevConfigs []deploy.ConfigResult
	var prevAIState *ai.AIState
	if prevState != nil {
		prevConfigs = prevState.Configs
		prevAIState = prevState.AI
	}

	// Unapply previous state if needed
	if prevState != nil {
		shouldUnapply := opts.Force || prevState.Profile != profileName
		if shouldUnapply {
			if a.aiOrchestrator != nil && prevState.AI != nil {
				if err := a.aiOrchestrator.Unapply(prevState.AI); err != nil {
					a.reporter.Error(fmt.Sprintf("AI unapply failed: %v", err))
				}
			}
			unapplyDeployer := a.deployerFactory(opts.ConfigDir, "", resolved.Vars, nil)
			if err := unapplyDeployer.Unapply(prevState.Configs); err != nil {
				a.reporter.Warning(fmt.Sprintf("Unapply warning: %v", err))
			}
			prevConfigs = nil
			prevAIState = nil
		} else {
			// Same profile reapply — find orphaned configs to clean up
			newTargets := make(map[string]bool)
			for target := range resolved.Configs {
				expanded, err := deploy.ExpandPath(target)
				if err == nil {
					newTargets[expanded] = true
				}
			}
			var orphans []deploy.ConfigResult
			for _, cfg := range prevState.Configs {
				if !newTargets[cfg.Target] {
					orphans = append(orphans, cfg)
				}
			}
			if len(orphans) > 0 {
				orphanDeployer := a.deployerFactory(opts.ConfigDir, "", nil, nil)
				if err := orphanDeployer.Unapply(orphans); err != nil {
					a.reporter.Warning(fmt.Sprintf("Orphan cleanup warning: %v", err))
				}
			}
		}
	}

	// Step 8: Deploy configs (sorted for deterministic order)
	deployer := a.deployerFactory(opts.ConfigDir, "", resolved.Vars, prevConfigs)

	if stages["configs"] {
		var deployErr error

		targets := make([]string, 0, len(resolved.Configs))
		for target := range resolved.Configs {
			targets = append(targets, target)
		}
		sort.Strings(targets)

		for _, target := range targets {
			source := resolved.Configs[target]

			if err := deploy.ValidateSourcePath(source, opts.ConfigDir); err != nil {
				if opts.SkipFailure {
					a.reporter.Warning(fmt.Sprintf("Skipping config %s: %v", target, err))
					continue
				}
				deployer.Rollback()
				return fmt.Errorf("config deployment failed: %w", err)
			}

			expandedTarget, err := deploy.ExpandPath(target)
			if err != nil {
				if opts.SkipFailure {
					a.reporter.Warning(fmt.Sprintf("Skipping config %s: %v", target, err))
					continue
				}
				deployer.Rollback()
				return fmt.Errorf("config deployment failed: %w", err)
			}

			_, err = deployer.DeployOne(expandedTarget, source, opts.Force)
			if err != nil {
				if opts.SkipFailure {
					a.reporter.Warning(fmt.Sprintf("Config deploy warning: %v", err))
					continue
				}
				deployErr = err
				break
			}
		}

		if deployErr != nil {
			a.reporter.Error(fmt.Sprintf("Config deployment failed: %v", deployErr))
			a.reporter.Warning("Rolling back deployed configs...")
			deployer.Rollback()
			if prevState == nil {
				os.Remove(filepath.Join(opts.StateDir, stateFile))
			}
			return fmt.Errorf("config deployment failed (rolled back): %w", deployErr)
		}
	}

	// Stage: pre_apply
	if stages["pre_apply"] {
		if err := a.runScripts(resolved.PreApply, opts.ConfigDir, "pre_apply"); err != nil {
			return err
		}
	}

	// Stage: packages
	var pkgResults []packages.PackageResult
	if stages["packages"] {
		pkgResults = a.installer.InstallAll(resolved.Packages)
	}

	// Stage: post_apply
	if stages["post_apply"] {
		if err := a.runScripts(resolved.PostApply, opts.ConfigDir, "post_apply"); err != nil {
			return err
		}
	}

	// Stage: ai
	var aiState *ai.AIState
	if stages["ai"] {
		if a.aiOrchestrator != nil {
			effectiveAI := ai.Resolve(resolved.AI)
			if effectiveAI != nil || prevAIState != nil {
				var aiErr error
				aiState, aiErr = a.aiOrchestrator.Apply(effectiveAI, prevAIState)
				if aiErr != nil {
					a.reporter.Error(fmt.Sprintf("AI configuration failed: %v", aiErr))
				}
				if isEmptyAIState(aiState) {
					aiState = nil
				}
			}
		}
	}

	// Carry forward results from previous state for skipped stages
	configResults := deployer.Deployed()
	if !stages["configs"] && prevState != nil {
		configResults = prevState.Configs
	}
	if !stages["packages"] && prevState != nil {
		pkgResults = prevState.Packages
	}
	if !stages["ai"] && prevState != nil {
		aiState = prevState.AI
	}

	// Write final state
	applyState := &ApplyState{
		Profile:      profileName,
		AppliedAt:    time.Now().UTC(),
		FacetVersion: a.version,
		Packages:     pkgResults,
		Configs:      configResults,
		AI:           aiState,
	}

	if err := a.stateStore.Write(opts.StateDir, applyState); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	// Step 12: Report
	a.printApplyReport(applyState)

	return nil
}

// runScripts executes a list of scripts sequentially. Fails fast on first error.
func (a *App) runScripts(scripts []profile.ScriptEntry, configDir, stageName string) error {
	if len(scripts) == 0 {
		return nil
	}
	if a.scriptRunner == nil {
		return fmt.Errorf("%s script runner is not configured", stageName)
	}

	for _, script := range scripts {
		if err := a.scriptRunner.Run(script.Run, configDir); err != nil {
			return fmt.Errorf("%s script %q failed: %w", stageName, script.Name, err)
		}
	}

	return nil
}

func isEmptyAIState(state *ai.AIState) bool {
	if state == nil {
		return true
	}
	return len(state.Skills) == 0 && len(state.MCPs) == 0 && len(state.Permissions) == 0
}
