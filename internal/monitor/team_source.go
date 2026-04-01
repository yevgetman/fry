package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/yevgetman/fry/internal/team"
)

type TeamSource struct {
	projectDir string
	snap       *team.Snapshot
	lastHash   string
}

func NewTeamSource(projectDir string) *TeamSource {
	return &TeamSource{projectDir: projectDir}
}

func (s *TeamSource) Name() string { return "team" }

func (s *TeamSource) Poll() (bool, error) {
	snap, err := team.ActiveSnapshot(context.Background(), s.projectDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, team.ErrNoActiveTeam) {
			changed := s.snap != nil
			s.snap = nil
			s.lastHash = ""
			return changed, nil
		}
		return false, err
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return false, err
	}
	hash := string(data)
	changed := hash != s.lastHash
	s.lastHash = hash
	s.snap = snap
	return changed, nil
}

func (s *TeamSource) Snapshot() *team.Snapshot { return s.snap }
