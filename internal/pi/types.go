package pi

// Config is the effective Pi configuration to apply.
type Config struct {
	Extensions []string
}

// PiState records Pi extensions managed by Facet.
type PiState struct {
	Extensions []string `json:"extensions,omitempty"`
}
