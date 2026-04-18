package app

// Deps holds all dependencies for the App.
type Deps struct {
	Loader          ProfileLoader
	BaseResolver    BaseResolver
	Installer       Installer
	Reporter        Reporter
	StateStore      StateStore
	DeployerFactory DeployerFactory
	AIOrchestrator  AIOrchestrator
	ScriptRunner    ScriptRunner
	SkillsManager   SkillsManager
	Version         string
	OSName          string
}

// App is the application service layer that orchestrates all facet operations.
type App struct {
	loader          ProfileLoader
	baseResolver    BaseResolver
	installer       Installer
	reporter        Reporter
	stateStore      StateStore
	deployerFactory DeployerFactory
	aiOrchestrator  AIOrchestrator
	scriptRunner    ScriptRunner
	skillsManager   SkillsManager
	version         string
	osName          string
}

// New creates a new App with the given dependencies.
func New(deps Deps) *App {
	return &App{
		loader:          deps.Loader,
		baseResolver:    deps.BaseResolver,
		installer:       deps.Installer,
		reporter:        deps.Reporter,
		stateStore:      deps.StateStore,
		deployerFactory: deps.DeployerFactory,
		aiOrchestrator:  deps.AIOrchestrator,
		scriptRunner:    deps.ScriptRunner,
		skillsManager:   deps.SkillsManager,
		version:         deps.Version,
		osName:          deps.OSName,
	}
}
