package verify

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	frylog "github.com/yevgetman/fry/internal/log"
)

// logCaptureMu serializes tests that redirect the package-level frylog logger.
// frylog.SetStdout mutates a global logger, so parallel tests would race on
// which buffer receives log output. Holding this mutex for the duration of each
// log-capturing test keeps the tests reliable while still allowing them to run
// in parallel with unrelated tests in the package.
var logCaptureMu sync.Mutex

func TestParseVerificationCheckBeforeSprintFileContains(t *testing.T) {
	t.Parallel()
	logCaptureMu.Lock()
	t.Cleanup(logCaptureMu.Unlock)

	var buf strings.Builder
	frylog.SetStdout(&buf)
	t.Cleanup(func() { frylog.SetStdout(nil) })

	path := writeVerificationFile(t, "@check_file_contains README.md \"hello\"\n@sprint 1\n")
	checks, err := ParseVerification(path)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, 0, checks[0].Sprint, "check before @sprint should have Sprint=0")
	assert.Contains(t, buf.String(), "WARNING")
	assert.Contains(t, buf.String(), "check before any @sprint directive")
}

func TestParseVerificationCheckBeforeSprintFile(t *testing.T) {
	t.Parallel()
	logCaptureMu.Lock()
	t.Cleanup(logCaptureMu.Unlock)

	var buf strings.Builder
	frylog.SetStdout(&buf)
	t.Cleanup(func() { frylog.SetStdout(nil) })

	path := writeVerificationFile(t, "@check_file go.mod\n@sprint 1\n")
	checks, err := ParseVerification(path)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, 0, checks[0].Sprint, "check before @sprint should have Sprint=0")
	assert.Contains(t, buf.String(), "WARNING")
	assert.Contains(t, buf.String(), "check before any @sprint directive")
}

func TestParseVerificationCheckBeforeSprintCmd(t *testing.T) {
	t.Parallel()
	logCaptureMu.Lock()
	t.Cleanup(logCaptureMu.Unlock)

	var buf strings.Builder
	frylog.SetStdout(&buf)
	t.Cleanup(func() { frylog.SetStdout(nil) })

	path := writeVerificationFile(t, "@check_cmd go build ./...\n@sprint 1\n")
	checks, err := ParseVerification(path)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, 0, checks[0].Sprint, "check before @sprint should have Sprint=0")
	assert.Contains(t, buf.String(), "WARNING")
	assert.Contains(t, buf.String(), "check before any @sprint directive")
}

func TestParseVerificationCheckBeforeSprintCmdOutput(t *testing.T) {
	t.Parallel()
	logCaptureMu.Lock()
	t.Cleanup(logCaptureMu.Unlock)

	var buf strings.Builder
	frylog.SetStdout(&buf)
	t.Cleanup(func() { frylog.SetStdout(nil) })

	path := writeVerificationFile(t, "@check_cmd_output go version | \"go1\\.\"\n@sprint 1\n")
	checks, err := ParseVerification(path)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, 0, checks[0].Sprint, "check before @sprint should have Sprint=0")
	assert.Contains(t, buf.String(), "WARNING")
	assert.Contains(t, buf.String(), "check before any @sprint directive")
}

func TestParseVerificationCheckBeforeSprintTest(t *testing.T) {
	t.Parallel()
	logCaptureMu.Lock()
	t.Cleanup(logCaptureMu.Unlock)

	var buf strings.Builder
	frylog.SetStdout(&buf)
	t.Cleanup(func() { frylog.SetStdout(nil) })

	path := writeVerificationFile(t, "@check_test go test ./...\n@sprint 1\n")
	checks, err := ParseVerification(path)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, 0, checks[0].Sprint, "check before @sprint should have Sprint=0")
	assert.Contains(t, buf.String(), "WARNING")
	assert.Contains(t, buf.String(), "check before any @sprint directive")
}

func TestParseVerificationCheckBeforeSprintAllTypes(t *testing.T) {
	t.Parallel()
	logCaptureMu.Lock()
	t.Cleanup(logCaptureMu.Unlock)

	var buf strings.Builder
	frylog.SetStdout(&buf)
	t.Cleanup(func() { frylog.SetStdout(nil) })

	path := writeVerificationFile(t,
		"@check_file_contains README.md \"hello\"\n"+
			"@check_file go.mod\n"+
			"@check_cmd go build ./...\n"+
			"@check_cmd_output go version | \"go1\\.\"\n"+
			"@check_test go test ./...\n"+
			"@sprint 1\n")
	checks, err := ParseVerification(path)
	require.NoError(t, err)
	require.Len(t, checks, 5)
	for i, c := range checks {
		assert.Equal(t, 0, c.Sprint, "check[%d] should have Sprint=0", i)
	}
	assert.Equal(t, 5, strings.Count(buf.String(), "WARNING"), "each check before @sprint should emit exactly one warning")
}
