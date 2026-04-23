package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCmd_DocsOverviewWritesToConfiguredOutput(t *testing.T) {
	rootCmd := NewRootCmd(nil, nil)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"docs"})

	err := rootCmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "# facet")
	assert.Contains(t, stdout.String(), "quickstart")
	assert.Empty(t, stderr.String())
}

func TestRootCmd_DocsTopicWritesToConfiguredOutput(t *testing.T) {
	rootCmd := NewRootCmd(nil, nil)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"docs", "quickstart"})

	err := rootCmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "# Quickstart")
	assert.Empty(t, stderr.String())
}

func TestRootCmd_DocsUnknownTopicReturnsError(t *testing.T) {
	rootCmd := NewRootCmd(nil, nil)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"docs", "unknown"})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown topic")
	assert.Contains(t, stdout.String(), "Usage:")
	assert.Contains(t, stderr.String(), "Error:")
}
