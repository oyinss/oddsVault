package syncjob

import (
	"regexp"
	"strconv"
	"strings"
)

var clockMinute = regexp.MustCompile(`(\d{1,3})(?:\+\d{1,2})?`)

// parseClockMinute reads a match minute from either provider clock: "46 minutes",
// "68'", "45'+3'" or "HT". Anything else stays unknown.
func parseClockMinute(raw string) *int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if trimmed == "HT" || trimmed == "Half Time" {
		value := 45
		return &value
	}
	match := clockMinute.FindStringSubmatch(trimmed)
	if len(match) < 2 {
		return nil
	}
	value, err := strconv.Atoi(match[1])
	if err != nil || value < 0 || value > 120 {
		return nil
	}
	return &value
}
