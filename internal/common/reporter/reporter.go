package reporter

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// Reporter handles formatted terminal output.
type Reporter struct {
	w       io.Writer
	color   bool
	verbose bool
}

// New creates a new Reporter.
func New(w io.Writer, color bool) *Reporter {
	return &Reporter{w: w, color: color}
}

// NewDefault creates a Reporter that writes to stdout with auto-detected color support.
func NewDefault() *Reporter {
	color := os.Getenv("TERM") != "" && os.Getenv("TERM") != "dumb" && os.Getenv("NO_COLOR") == ""
	return &Reporter{w: os.Stdout, color: color}
}

func (r *Reporter) colorize(color, text string) string {
	if !r.color {
		return text
	}
	return color + text + colorReset
}

// Success prints a success message with a checkmark.
func (r *Reporter) Success(msg string) {
	fmt.Fprintf(r.w, "  %s %s\n", r.colorize(colorGreen, "✓"), msg)
}

// Warning prints a warning message.
func (r *Reporter) Warning(msg string) {
	fmt.Fprintf(r.w, "  %s %s\n", r.colorize(colorYellow, "⚠"), msg)
}

// Error prints an error message.
func (r *Reporter) Error(msg string) {
	fmt.Fprintf(r.w, "  %s %s\n", r.colorize(colorRed, "✗"), msg)
}

// Header prints a section header.
func (r *Reporter) Header(msg string) {
	fmt.Fprintf(r.w, "\n%s\n", r.colorize(colorBold, msg))
}

// PrintLine prints a formatted line.
func (r *Reporter) PrintLine(msg string) {
	fmt.Fprintf(r.w, "%s\n", msg)
}

// SetVerbose enables or disables progress output.
func (r *Reporter) SetVerbose(verbose bool) {
	r.verbose = verbose
}

// Progress prints a progress message when verbose mode is enabled.
func (r *Reporter) Progress(msg string) {
	if !r.verbose {
		return
	}
	fmt.Fprintf(r.w, "%s\n", msg)
}

// ProgressDuration prints a completed timed operation when verbose mode is enabled.
func (r *Reporter) ProgressDuration(label, outcome string, elapsed time.Duration, err error) {
	if !r.verbose {
		return
	}
	fmt.Fprintf(r.w, "%s ... %s %s\n", label, outcome, formatDuration(elapsed))
	if err != nil {
		fmt.Fprintf(r.w, "     error: %v\n", err)
	}
}

// ProgressStart prints a grouped operation start line and returns a completion function.
func (r *Reporter) ProgressStart(label string) func(outcome string, err error) {
	if !r.verbose {
		return func(string, error) {}
	}
	start := time.Now()
	fmt.Fprintf(r.w, "%s ... start\n", label)
	return func(outcome string, err error) {
		r.ProgressDuration(label, outcome, time.Since(start), err)
	}
}

// ProgressStep runs fn and prints the operation outcome and duration.
func (r *Reporter) ProgressStep(label string, fn func() error) error {
	start := time.Now()
	err := fn()
	outcome := "ok"
	if err != nil {
		outcome = "failed"
	}
	r.ProgressDuration(label, outcome, time.Since(start), err)
	return err
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		ms := d.Round(time.Millisecond).Milliseconds()
		if ms < 0 {
			ms = 0
		}
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// Dim returns the text with dim styling (for use in formatted output).
func (r *Reporter) Dim(text string) string {
	return r.colorize(colorDim, text)
}

// Separator prints a visual separator line.
func (r *Reporter) Separator() {
	fmt.Fprintf(r.w, "%s\n", strings.Repeat("─", 60))
}
