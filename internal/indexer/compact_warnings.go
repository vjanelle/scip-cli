package indexer

import (
	"slices"
	"strings"
)

func buildCompactDependencies(deps []ModuleDependency, table *stringTablesBuilder) []CompactDependency {
	compact := make([]CompactDependency, 0, len(deps))
	for _, dep := range deps {
		compact = append(compact, CompactDependency{
			PathID:    table.add("paths", normalizePathKey(dep.Path)),
			VersionID: table.add("misc", dep.Version),
			ReplaceID: table.add("misc", dep.Replace),
			DirID:     table.add("paths", normalizePathKey(dep.Dir)),
			Main:      dep.Main,
			Indirect:  dep.Indirect,
		})
	}
	return compact
}

func compactWarnings(warnings []string, table *stringTablesBuilder) ([]string, []WarningNotice) {
	if len(warnings) == 0 {
		return nil, nil
	}

	type warningBucket struct {
		message string
		count   int
		score   int
	}
	buckets := map[string]*warningBucket{}
	for _, warning := range warnings {
		code := warningCode(warning)
		entry := buckets[code]
		if entry == nil {
			entry = &warningBucket{message: warning, score: warningScore(warning)}
			buckets[code] = entry
		}
		entry.count++
		if warningScore(warning) > entry.score {
			entry.message = warning
			entry.score = warningScore(warning)
		}
	}

	codes := make([]string, 0, len(buckets))
	for code := range buckets {
		codes = append(codes, code)
	}
	slices.SortFunc(codes, func(left, right string) int {
		return buckets[right].score - buckets[left].score
	})

	notices := make([]WarningNotice, 0, min(12, len(codes)))
	inline := make([]string, 0, 2)
	for _, code := range codes {
		bucket := buckets[code]
		notices = append(notices, WarningNotice{
			Code:      code,
			MessageID: table.add("misc", bucket.message),
			Count:     bucket.count,
		})
		if len(inline) < 2 && isCriticalWarningCode(code) {
			inline = append(inline, bucket.message)
		}
		if len(notices) == 12 {
			break
		}
	}
	return inline, notices
}

func warningCode(warning string) string {
	lowered := strings.ToLower(warning)
	switch {
	case strings.Contains(lowered, "security warning"), strings.Contains(lowered, "security:"):
		return "security"
	case strings.Contains(lowered, "workspace changed during indexing"):
		return "workspace_dirty"
	case strings.Contains(lowered, "dependency scan failed"):
		return "dependency_scan_failed"
	case strings.Contains(lowered, "failed"):
		return "indexing_failed"
	case strings.Contains(lowered, "indexed"):
		return "indexer_info"
	default:
		return "general"
	}
}

func warningScore(warning string) int {
	switch warningCode(warning) {
	case "security":
		return 100
	case "indexing_failed":
		return 90
	case "dependency_scan_failed":
		return 80
	case "general":
		return 40
	default:
		return 10
	}
}

func isCriticalWarningCode(code string) bool {
	switch code {
	case "security", "indexing_failed", "dependency_scan_failed", "workspace_dirty":
		return true
	default:
		return false
	}
}
