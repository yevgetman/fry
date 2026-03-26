package verify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckTypeString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "FILE", CheckFile.String())
	assert.Equal(t, "FILE_CONTAINS", CheckFileContains.String())
	assert.Equal(t, "CMD", CheckCmd.String())
	assert.Equal(t, "CMD_OUTPUT", CheckCmdOutput.String())
	assert.Equal(t, "TEST", CheckTest.String())
	assert.NotEmpty(t, CheckType(99).String())
}
