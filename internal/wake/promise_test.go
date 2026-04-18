package wake

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractPromise_PlainText(t *testing.T) {
	out := []byte("did some work\n===WAKE_DONE===\n")
	found, text := ExtractPromise(out)
	assert.True(t, found)
	assert.Contains(t, text, "did some work")
}

func TestExtractPromise_JSON(t *testing.T) {
	jsonOut := []byte(`{"type":"result","result":"did work\n===WAKE_DONE===","total_cost_usd":0.01}`)
	found, text := ExtractPromise(jsonOut)
	assert.True(t, found)
	assert.Contains(t, text, "===WAKE_DONE===")
}

func TestExtractPromise_NotFound(t *testing.T) {
	out := []byte("did some work without token\n")
	found, _ := ExtractPromise(out)
	assert.False(t, found)
}

func TestExtractPromise_ExtraWhitespace(t *testing.T) {
	out := []byte("work\n  ===WAKE_DONE===  \n")
	found, _ := ExtractPromise(out)
	assert.True(t, found)
}

func TestParseCostUSD(t *testing.T) {
	out := []byte(`{"type":"result","total_cost_usd":0.07860825,"result":"hello"}`)
	cost := ParseCostUSD(out)
	assert.InDelta(t, 0.07860825, cost, 1e-6)
}

func TestParseCostUSD_NoJSON(t *testing.T) {
	out := []byte("plain text output")
	cost := ParseCostUSD(out)
	assert.Equal(t, 0.0, cost)
}

func TestExtractStatusTransition(t *testing.T) {
	out := []byte("done with work\nFRY_STATUS_TRANSITION=complete\n===WAKE_DONE===\n")
	status, ok := ExtractStatusTransition(out)
	assert.True(t, ok)
	assert.Equal(t, "complete", status)
}

func TestExtractStatusTransition_NotFound(t *testing.T) {
	out := []byte("just normal output\n===WAKE_DONE===\n")
	_, ok := ExtractStatusTransition(out)
	assert.False(t, ok)
}
