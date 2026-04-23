package app

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
