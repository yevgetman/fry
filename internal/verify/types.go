// Package verify implements the sanity check system (post-sprint checks).
// Despite the package name, the user-facing term is "sanity checks" — not "verification."
// The package name is retained for import stability.
package verify

type CheckType int

const (
	CheckFile CheckType = iota
	CheckFileContains
	CheckCmd
	CheckCmdOutput
	CheckTest
)

func (t CheckType) String() string {
	switch t {
	case CheckFile:
		return "FILE"
	case CheckFileContains:
		return "FILE_CONTAINS"
	case CheckCmd:
		return "CMD"
	case CheckCmdOutput:
		return "CMD_OUTPUT"
	case CheckTest:
		return "TEST"
	default:
		return "UNKNOWN"
	}
}

type Check struct {
	Sprint  int
	Type    CheckType
	Path    string
	Pattern string
	Command string
}

type CheckResult struct {
	Check         Check
	Passed        bool
	Output        string
	TestPassCount int
	TestFailCount int
	TestSkipCount int
	TestFramework string
}

type VerificationOutcome struct {
	Results          []CheckResult
	PassCount        int
	TotalCount       int
	FailPercent      float64
	WithinThreshold  bool
	DeferredFailures []CheckResult
}
