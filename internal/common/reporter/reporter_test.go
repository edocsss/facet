package reporter

import (
	"bytes"
	"errors"
	"testing"
	"time"

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

func TestReporter_Progress_Verbose(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.SetVerbose(true)
	r.Progress("Deploying configs")
	assert.Contains(t, buf.String(), "Deploying configs")
}

func TestReporter_Progress_Silent_WhenNotVerbose(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Progress("Deploying configs")
	assert.Empty(t, buf.String())
}

func TestReporter_SetVerbose_TogglesBehavior(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)

	r.Progress("before enable")
	r.SetVerbose(true)
	r.Progress("after enable")
	r.SetVerbose(false)
	r.Progress("after disable")

	out := buf.String()
	assert.NotContains(t, out, "before enable")
	assert.Contains(t, out, "after enable")
	assert.NotContains(t, out, "after disable")
}

func TestReporter_ProgressDuration_Silent_WhenNotVerbose(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)

	r.ProgressDuration("Loading profile", "ok", 12*time.Millisecond, nil)

	assert.Empty(t, buf.String())
}

func TestReporter_ProgressDuration_PrintsOutcomeAndDuration(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.SetVerbose(true)

	r.ProgressDuration("Loading profile", "ok", 12*time.Millisecond, nil)

	out := buf.String()
	assert.Contains(t, out, "Loading profile ... ok 12ms")
}

func TestReporter_ProgressDuration_PrintsSecondsForLongDurations(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.SetVerbose(true)

	r.ProgressDuration("Installing packages", "done", 1500*time.Millisecond, nil)

	assert.Contains(t, buf.String(), "Installing packages ... done 1.5s")
}

func TestReporter_ProgressDuration_PrintsErrorLine(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.SetVerbose(true)

	r.ProgressDuration("  -> node install", "failed", 21*time.Millisecond, errors.New("exit status 1"))

	out := buf.String()
	assert.Contains(t, out, "  -> node install ... failed 21ms")
	assert.Contains(t, out, "     error: exit status 1")
}

func TestReporter_ProgressStart_PrintsStartAndDone(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.SetVerbose(true)

	done := r.ProgressStart("Deploying configs")
	done("done", nil)

	out := buf.String()
	assert.Contains(t, out, "Deploying configs ... start")
	assert.Regexp(t, `Deploying configs \.\.\. done [0-9]+ms`, out)
}

func TestReporter_ProgressStep_PrintsFailureAndReturnsError(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.SetVerbose(true)
	expectedErr := errors.New("boom")

	err := r.ProgressStep("Resolving extends", func() error {
		return expectedErr
	})

	assert.ErrorIs(t, err, expectedErr)
	out := buf.String()
	assert.Regexp(t, `Resolving extends \.\.\. failed [0-9]+ms`, out)
	assert.Contains(t, out, "     error: boom")
}
