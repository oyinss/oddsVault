package store

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	linePattern = regexp.MustCompile(`\(\s*([+-]?\d+(?:[.,]\d+)?)\s*\)`)
	sidePattern = regexp.MustCompile(`\b(over|under)\b`)
)

// ParsedMarket is the structure the journal keeps for each leg.
type ParsedMarket struct {
	Family string
	Side   string
	Line   *float64
}

// ParseMarketText extracts a market family, side and line from provider text.
//
//	"Total. Under (7). Corners 1.39"
//	  -> Family "corners_total", Side "under", Line 7
//	"Handicap. (+1). Corners"
//	  -> Family "corners_handicap", Line 1
//
// Anything it cannot classify keeps family "unknown"; the raw text is always
// stored alongside.
func ParseMarketText(raw string) ParsedMarket {
	cleaned := strings.Join(strings.Fields(raw), " ")
	lower := strings.ToLower(cleaned)
	result := ParsedMarket{}

	switch {
	case strings.Contains(lower, "corner"):
		if strings.Contains(lower, "handicap") {
			result.Family = "corners_handicap"
		} else {
			result.Family = "corners_total"
		}
	case strings.Contains(lower, "both teams"):
		result.Family = "both_teams_to_score"
	case strings.Contains(lower, "correct score"):
		result.Family = "correct_score"
	case strings.Contains(lower, "total"):
		result.Family = "total_goals"
	case strings.Contains(lower, "handicap"):
		result.Family = "handicap"
	case strings.Contains(lower, "winner") || strings.Contains(lower, "1x2") ||
		strings.Contains(lower, "w1") || strings.Contains(lower, "w2") ||
		strings.Contains(lower, "result"):
		result.Family = "match_result"
	default:
		result.Family = "unknown"
	}

	if match := sidePattern.FindStringSubmatch(lower); len(match) == 2 {
		result.Side = match[1]
	}
	if match := linePattern.FindStringSubmatch(cleaned); len(match) == 2 {
		if value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", "."), 64); err == nil {
			result.Line = &value
		}
	}
	return result
}
