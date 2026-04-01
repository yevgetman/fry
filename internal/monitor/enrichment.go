package monitor

import (
	"fmt"
	"time"

	"github.com/yevgetman/fry/internal/agent"
)

// phaseMap maps event types to build phase names.
var phaseMap = map[string]string{
	"triage_start":        "triage",
	"triage_complete":     "triage",
	"prepare_start":       "prepare",
	"prepare_complete":    "prepare",
	"build_start":         "sprint",
	"sprint_start":        "sprint",
	"sprint_complete":     "sprint",
	"alignment_complete":  "sprint",
	"agent_deploy":        "sprint",
	"audit_cycle_start":   "audit",
	"audit_fix_start":     "audit",
	"audit_verify_start":  "audit",
	"audit_complete":      "audit",
	"review_start":        "review",
	"review_complete":     "review",
	"build_audit_start":   "audit",
	"build_audit_done":    "audit",
	"build_end":           "complete",
	"team_start":          "team",
	"team_task_started":   "team",
	"team_task_completed": "team",
	"team_task_failed":    "team",
	"team_shutdown":       "complete",
	"team_complete":       "complete",
}

// EnrichEvents takes raw BuildEvents and a total sprint count and returns
// EnrichedEvents with computed context (elapsed times, sprint fractions,
// phase transitions). This is a pure function with no side effects.
func EnrichEvents(events []agent.BuildEvent, totalSprints int) []EnrichedEvent {
	if len(events) == 0 {
		return nil
	}

	enriched := make([]EnrichedEvent, len(events))
	var buildStart time.Time
	var sprintStart time.Time
	var prevPhase string

	// Find the build_start event for elapsed time baseline.
	// Fall back to the first event if monitoring starts mid-build.
	for _, evt := range events {
		if evt.Type == "build_start" {
			buildStart = evt.Timestamp
			break
		}
	}
	if buildStart.IsZero() && len(events) > 0 {
		buildStart = events[0].Timestamp
	}

	for i, evt := range events {
		e := EnrichedEvent{BuildEvent: evt}

		if !buildStart.IsZero() {
			e.ElapsedBuild = evt.Timestamp.Sub(buildStart)
		}

		// Track sprint start time.
		if evt.Type == "sprint_start" {
			sprintStart = evt.Timestamp
		}
		if !sprintStart.IsZero() {
			e.ElapsedSprint = evt.Timestamp.Sub(sprintStart)
		}

		// Sprint fraction.
		if evt.Sprint > 0 && totalSprints > 0 {
			e.SprintOf = fmt.Sprintf("%d/%d", evt.Sprint, totalSprints)
		}

		// Phase transition detection.
		curPhase := phaseForEvent(evt.Type)
		if curPhase != "" && prevPhase != "" && curPhase != prevPhase {
			e.PhaseChange = prevPhase + " -> " + curPhase
		}
		if curPhase != "" {
			prevPhase = curPhase
		}

		// Terminal event.
		e.IsTerminal = evt.Type == "build_end"
		if _, ok := verboseMonitorEventTypes[evt.Type]; ok {
			e.Synthetic = true
		}

		enriched[i] = e
	}
	return enriched
}

// EnrichNewEvents enriches only new events, given the full event history for
// context (build start time, current sprint start, previous phase).
func EnrichNewEvents(allEvents []agent.BuildEvent, newStart int, totalSprints int) []EnrichedEvent {
	if newStart >= len(allEvents) {
		return nil
	}
	all := EnrichEvents(allEvents, totalSprints)
	return all[newStart:]
}

func phaseForEvent(eventType string) string {
	if p, ok := phaseMap[eventType]; ok {
		return p
	}
	return ""
}
