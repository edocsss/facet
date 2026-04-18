package app

import "fmt"

// AISkillsCheck delegates to the SkillsManager to check for available skill updates.
func (a *App) AISkillsCheck() error {
	if a.skillsManager == nil {
		return fmt.Errorf("skills manager not available (is npx installed?)")
	}
	return a.skillsManager.Check()
}

// AISkillsUpdate delegates to the SkillsManager to update all installed skills.
func (a *App) AISkillsUpdate() error {
	if a.skillsManager == nil {
		return fmt.Errorf("skills manager not available (is npx installed?)")
	}
	return a.skillsManager.Update()
}
