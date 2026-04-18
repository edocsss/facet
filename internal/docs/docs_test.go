package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopics_ReturnsNonEmptyList(t *testing.T) {
	topics := Topics()
	assert.NotEmpty(t, topics, "Topics() should return at least one topic")
}

func TestTopics_EachHasNameAndDescription(t *testing.T) {
	for _, topic := range Topics() {
		assert.NotEmpty(t, topic.Name, "topic name must not be empty")
		assert.NotEmpty(t, topic.Description, "topic %q description must not be empty", topic.Name)
	}
}

func TestRender_ValidTopic(t *testing.T) {
	content, err := Render("quickstart")
	assert.NoError(t, err)
	assert.Contains(t, content, "# Quickstart")
}

func TestRender_InvalidTopic(t *testing.T) {
	_, err := Render("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown topic")
}

func TestRender_AllRegisteredTopicsHaveFiles(t *testing.T) {
	for _, topic := range Topics() {
		content, err := Render(topic.Name)
		assert.NoError(t, err, "topic %q should have an embedded file", topic.Name)
		assert.NotEmpty(t, content, "topic %q should have non-empty content", topic.Name)
	}
}

func TestOverview_ContainsAllTopicNames(t *testing.T) {
	overview := Overview()
	for _, topic := range Topics() {
		assert.Contains(t, overview, topic.Name, "overview should list topic %q", topic.Name)
	}
}

func TestOverview_ContainsUsageInstructions(t *testing.T) {
	overview := Overview()
	assert.Contains(t, overview, "facet docs <topic>")
	assert.Contains(t, overview, "Run \"facet docs <topic>\" to read a specific topic.")
}
