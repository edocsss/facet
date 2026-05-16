package app

import "time"

// mockReporter captures reporter output for testing.
type mockReporter struct {
	messages []string
}

func (m *mockReporter) Success(msg string)     { m.messages = append(m.messages, "success: "+msg) }
func (m *mockReporter) Warning(msg string)     { m.messages = append(m.messages, "warning: "+msg) }
func (m *mockReporter) Error(msg string)       { m.messages = append(m.messages, "error: "+msg) }
func (m *mockReporter) Header(msg string)      { m.messages = append(m.messages, "header: "+msg) }
func (m *mockReporter) PrintLine(msg string)   { m.messages = append(m.messages, "line: "+msg) }
func (m *mockReporter) Dim(text string) string { return text }
func (m *mockReporter) Progress(msg string)    { m.messages = append(m.messages, "progress: "+msg) }
func (m *mockReporter) ProgressDuration(label, outcome string, _ time.Duration, err error) {
	m.messages = append(m.messages, "progress: "+label+" ... "+outcome)
	if err != nil {
		m.messages = append(m.messages, "progress-error: "+err.Error())
	}
}
func (m *mockReporter) ProgressStart(label string) func(outcome string, err error) {
	m.messages = append(m.messages, "progress: "+label+" ... start")
	return func(outcome string, err error) {
		m.ProgressDuration(label, outcome, 0, err)
	}
}
func (m *mockReporter) ProgressStep(label string, fn func() error) error {
	err := fn()
	outcome := "ok"
	if err != nil {
		outcome = "failed"
	}
	m.ProgressDuration(label, outcome, 0, err)
	return err
}
