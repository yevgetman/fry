package team

import (
	"fmt"
	"path/filepath"

	"github.com/yevgetman/fry/internal/config"
)

const (
	activeTeamFile        = "active-team.txt"
	defaultHeartbeatGrace = 15
)

func RootDir(projectDir string) string {
	return filepath.Join(projectDir, config.FryDir, "team")
}

func ActiveTeamPath(projectDir string) string {
	return filepath.Join(RootDir(projectDir), activeTeamFile)
}

func TeamDir(projectDir, teamID string) string {
	return filepath.Join(RootDir(projectDir), teamID)
}

func ConfigPath(projectDir, teamID string) string {
	return filepath.Join(TeamDir(projectDir, teamID), "config.json")
}

func ManifestPath(projectDir, teamID string) string {
	return filepath.Join(TeamDir(projectDir, teamID), "manifest.json")
}

func TeamEventsPath(projectDir, teamID string) string {
	return filepath.Join(TeamDir(projectDir, teamID), "events.jsonl")
}

func TasksDir(projectDir, teamID string) string {
	return filepath.Join(TeamDir(projectDir, teamID), "tasks")
}

func TaskPath(projectDir, teamID, taskID string) string {
	return filepath.Join(TasksDir(projectDir, teamID), fmt.Sprintf("%s.json", taskID))
}

func WorkersDir(projectDir, teamID string) string {
	return filepath.Join(TeamDir(projectDir, teamID), "workers")
}

func WorkerDir(projectDir, teamID, workerID string) string {
	return filepath.Join(WorkersDir(projectDir, teamID), workerID)
}

func WorkerIdentityPath(projectDir, teamID, workerID string) string {
	return filepath.Join(WorkerDir(projectDir, teamID, workerID), "identity.json")
}

func WorkerHeartbeatPath(projectDir, teamID, workerID string) string {
	return filepath.Join(WorkerDir(projectDir, teamID, workerID), "heartbeat.json")
}

func WorkerStatusPath(projectDir, teamID, workerID string) string {
	return filepath.Join(WorkerDir(projectDir, teamID, workerID), "status.json")
}

func WorkerMailboxPath(projectDir, teamID, workerID string) string {
	return filepath.Join(WorkerDir(projectDir, teamID, workerID), "mailbox.json")
}

func LocksDir(projectDir, teamID string) string {
	return filepath.Join(TeamDir(projectDir, teamID), "locks")
}

func AssignmentLockPath(projectDir, teamID string) string {
	return filepath.Join(LocksDir(projectDir, teamID), "assignment.lock")
}

func FinalizeLockPath(projectDir, teamID string) string {
	return filepath.Join(LocksDir(projectDir, teamID), "finalize.lock")
}

func ArtifactsDir(projectDir, teamID string) string {
	return filepath.Join(TeamDir(projectDir, teamID), "artifacts")
}

func PlanSummaryPath(projectDir, teamID string) string {
	return filepath.Join(ArtifactsDir(projectDir, teamID), "plan-summary.md")
}

func MergeReportPath(projectDir, teamID string) string {
	return filepath.Join(ArtifactsDir(projectDir, teamID), "merge-report.md")
}

func IntegratedOutputDir(projectDir, teamID string) string {
	return filepath.Join(ArtifactsDir(projectDir, teamID), "integrated-output")
}

func TaskLogPath(projectDir, teamID, workerID, taskID string) string {
	return filepath.Join(ArtifactsDir(projectDir, teamID), fmt.Sprintf("%s-%s.log", workerID, taskID))
}
