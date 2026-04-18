package reporter

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReporter_Success(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Success("test message")
	output := buf.String()
	assert.Contains(t, output, "✓")
	assert.Contains(t, output, "test message")
}

func TestReporter_Warning(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Warning("warning message")
	output := buf.String()
	assert.Contains(t, output, "⚠")
	assert.Contains(t, output, "warning message")
}

func TestReporter_Error(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Error("error message")
	output := buf.String()
	assert.Contains(t, output, "✗")
	assert.Contains(t, output, "error message")
}

func TestReporter_ColorDisabled(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Success("test message")
	output := buf.String()
	assert.NotContains(t, output, "\033[")
	assert.Contains(t, output, "test message")
}

func TestReporter_Header(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Header("Section Title")
	output := buf.String()
	assert.Contains(t, output, "Section Title")
}
