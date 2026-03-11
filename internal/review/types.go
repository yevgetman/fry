package review

type ReviewVerdict string

const (
	VerdictContinue ReviewVerdict = "CONTINUE"
	VerdictDeviate  ReviewVerdict = "DEVIATE"
)

type DeviationSpec struct {
	Trigger         string
	RiskAssessment  string
	AffectedSprints []int
	Details         string
	RawText         string
}

type ReviewResult struct {
	Verdict   ReviewVerdict
	Deviation *DeviationSpec
	RawOutput string
}
