package verify

type CheckType int

const (
	CheckFile CheckType = iota
	CheckFileContains
	CheckCmd
	CheckCmdOutput
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
	Check  Check
	Passed bool
	Output string
}
