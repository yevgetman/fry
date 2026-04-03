package audit

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yevgetman/fry/internal/severity"
)

type remediationCluster struct {
	ID          int
	Label       string
	Reason      string
	Theme       string
	Scope       string
	Findings    []Finding
	TargetFiles []string
}

type clusteringSignals struct {
	fileFamily  string
	subsystem   string
	themeLabel  string
	themeTokens []string
}

func clusterFixFindings(findings []Finding) []remediationCluster {
	if len(findings) == 0 {
		return nil
	}

	sorted := append([]Finding(nil), findings...)
	sortFindingsFIFO(sorted)

	var clusters []remediationCluster
	for _, finding := range sorted {
		assigned := false
		for i := range clusters {
			if clusterIncludesFinding(clusters[i], finding) {
				clusters[i].Findings = append(clusters[i].Findings, finding)
				assigned = true
				break
			}
		}
		if !assigned {
			clusters = append(clusters, remediationCluster{Findings: []Finding{finding}})
		}
	}

	for i := range clusters {
		sortFindingsFIFO(clusters[i].Findings)
		clusters[i] = finalizeRemediationCluster(i+1, clusters[i].Findings)
	}

	sort.SliceStable(clusters, func(i, j int) bool {
		left := clusters[i]
		right := clusters[j]
		leftCycle := left.Findings[0].OriginCycle
		rightCycle := right.Findings[0].OriginCycle
		if leftCycle != rightCycle {
			return leftCycle < rightCycle
		}
		leftSeverity := severity.Rank(maxSeverityForFindings(left.Findings))
		rightSeverity := severity.Rank(maxSeverityForFindings(right.Findings))
		if leftSeverity != rightSeverity {
			return leftSeverity > rightSeverity
		}
		if len(left.Findings) != len(right.Findings) {
			return len(left.Findings) > len(right.Findings)
		}
		return left.Label < right.Label
	})

	for i := range clusters {
		clusters[i].ID = i + 1
	}

	return clusters
}

func orderFindingsByCluster(findings []Finding) []Finding {
	clusters := clusterFixFindings(findings)
	if len(clusters) == 0 {
		return nil
	}

	ordered := make([]Finding, 0, len(findings))
	for _, cluster := range clusters {
		ordered = append(ordered, cluster.Findings...)
	}
	return ordered
}

func clusterIncludesFinding(cluster remediationCluster, candidate Finding) bool {
	for _, existing := range cluster.Findings {
		if findingsBelongToSameCluster(existing, candidate) {
			return true
		}
	}
	return false
}

func findingsBelongToSameCluster(a, b Finding) bool {
	signalsA := clusteringSignalsForFinding(a)
	signalsB := clusteringSignalsForFinding(b)
	sameTheme := themeMatch(a, b) || sharedThemeLabel(signalsA.themeLabel, signalsB.themeLabel) || sharesRemediationTheme(signalsA.themeTokens, signalsB.themeTokens)
	if !sameTheme {
		return false
	}
	if signalsA.fileFamily != "" && signalsA.fileFamily == signalsB.fileFamily {
		return true
	}
	if signalsA.subsystem != "" && signalsA.subsystem == signalsB.subsystem {
		return true
	}
	return signalsA.subsystem == "" || signalsB.subsystem == ""
}

func sharesRemediationTheme(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	if jaccardSimilarity(a, b) >= 0.2 {
		return true
	}
	overlap := 0
	set := make(map[string]struct{}, len(a))
	for _, token := range a {
		set[token] = struct{}{}
	}
	for _, token := range b {
		if _, ok := set[token]; ok {
			overlap++
			if overlap >= 2 {
				return true
			}
		}
	}
	return false
}

func sharedThemeLabel(a, b string) bool {
	return a != "" && b != "" && a == b
}

func finalizeRemediationCluster(id int, findings []Finding) remediationCluster {
	scope := clusterScope(findings)
	theme := clusterTheme(findings)
	label := clusterLabel(scope, theme)
	reason := clusterReason(scope, theme, findings)

	return remediationCluster{
		ID:          id,
		Label:       label,
		Reason:      reason,
		Theme:       theme,
		Scope:       scope,
		Findings:    findings,
		TargetFiles: clusterTargetFiles(findings),
	}
}

func clusteringSignalsForFinding(f Finding) clusteringSignals {
	return clusteringSignals{
		fileFamily:  findingFileFamily(f),
		subsystem:   findingSubsystem(f),
		themeLabel:  findingThemeLabel(f),
		themeTokens: themeTokensForFinding(f),
	}
}

func findingFileFamily(f Finding) string {
	if family := fileFamily(f.Location); family != "" {
		return family
	}
	files := findingTargetFiles(f)
	if len(files) == 0 {
		return ""
	}
	path := files[0]
	ext := filepath.Ext(path)
	if ext != "" {
		path = strings.TrimSuffix(path, ext)
	}
	return path
}

func findingSubsystem(f Finding) string {
	files := findingTargetFiles(f)
	if len(files) == 0 {
		return ""
	}
	path := filepath.Clean(files[0])
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		return base
	}
	parts := strings.Split(filepath.ToSlash(dir), "/")
	switch {
	case len(parts) >= 2:
		return filepath.ToSlash(filepath.Join(parts[0], parts[1]))
	default:
		return parts[0]
	}
}

func findingTargetFiles(f Finding) []string {
	if len(f.AffectedFiles) > 0 {
		return append([]string(nil), f.AffectedFiles...)
	}
	return targetFilesForFinding(f)
}

func findingThemeLabel(f Finding) string {
	return summarizeThemeTokens(themeTokensForFinding(f), 2)
}

func themeTokensForFinding(f Finding) []string {
	text := strings.TrimSpace(strings.Join([]string{f.Description, f.RecommendedFix, f.BlockerDetails}, " "))
	if text == "" {
		return nil
	}
	return descriptionTokens(text)
}

func clusterScope(findings []Finding) string {
	if len(findings) == 0 {
		return ""
	}
	if family := sharedClusterValue(findings, func(f Finding) string {
		return clusteringSignalsForFinding(f).fileFamily
	}); family != "" {
		return family
	}
	return sharedClusterValue(findings, func(f Finding) string {
		return clusteringSignalsForFinding(f).subsystem
	})
}

func clusterTheme(findings []Finding) string {
	tokenCounts := make(map[string]int)
	for _, finding := range findings {
		seen := make(map[string]struct{})
		for _, token := range themeTokensForFinding(finding) {
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			tokenCounts[token]++
		}
	}

	type scoredToken struct {
		token string
		count int
	}

	var scored []scoredToken
	for token, count := range tokenCounts {
		scored = append(scored, scoredToken{token: token, count: count})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].count != scored[j].count {
			return scored[i].count > scored[j].count
		}
		if len(scored[i].token) != len(scored[j].token) {
			return len(scored[i].token) > len(scored[j].token)
		}
		return scored[i].token < scored[j].token
	})

	limit := 2
	if len(scored) < limit {
		limit = len(scored)
	}
	if limit == 0 && len(findings) > 0 {
		return findingThemeLabel(findings[0])
	}
	parts := make([]string, 0, limit)
	for _, token := range scored[:limit] {
		parts = append(parts, token.token)
	}
	return strings.Join(parts, " ")
}

func summarizeThemeTokens(tokens []string, limit int) string {
	if len(tokens) == 0 || limit <= 0 {
		return ""
	}
	if len(tokens) < limit {
		limit = len(tokens)
	}
	return strings.Join(tokens[:limit], " ")
}

func sharedClusterValue(findings []Finding, valueFn func(Finding) string) string {
	if len(findings) == 0 {
		return ""
	}
	first := strings.TrimSpace(valueFn(findings[0]))
	if first == "" {
		return ""
	}
	for _, finding := range findings[1:] {
		if strings.TrimSpace(valueFn(finding)) != first {
			return ""
		}
	}
	return first
}

func clusterLabel(scope, theme string) string {
	switch {
	case scope != "" && theme != "":
		return fmt.Sprintf("%s: %s", scope, theme)
	case scope != "":
		return scope
	case theme != "":
		return theme
	default:
		return "related remediation scope"
	}
}

func clusterReason(scope, theme string, findings []Finding) string {
	switch {
	case scope != "" && theme != "":
		return fmt.Sprintf("shared scope `%s` and requirement theme `%s`", scope, theme)
	case scope != "":
		return fmt.Sprintf("shared scope `%s`", scope)
	case theme != "":
		return fmt.Sprintf("shared requirement theme `%s`", theme)
	case len(findings) > 1:
		return "related remediation scope"
	default:
		return "single issue cluster"
	}
}

func clusterTargetFiles(findings []Finding) []string {
	seen := make(map[string]struct{})
	var files []string
	for _, finding := range findings {
		for _, file := range findingTargetFiles(finding) {
			if file == "" {
				continue
			}
			if _, ok := seen[file]; ok {
				continue
			}
			seen[file] = struct{}{}
			files = append(files, file)
		}
	}
	sort.Strings(files)
	return files
}
