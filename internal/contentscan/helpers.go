package contentscan

import (
	"regexp"
	"strings"
)

// countMatches returns len(re.FindAllString(body, cap)), capped at cap.
// cap=0 means unlimited.
func countMatches(re *regexp.Regexp, body string, cap int) int {
	if cap <= 0 {
		return len(re.FindAllString(body, -1))
	}
	m := re.FindAllString(body, cap+1)
	if len(m) > cap {
		return cap
	}
	return len(m)
}

// containsAnyFold returns true when any needle (case-insensitive) is
// a substring of body.
func containsAnyFold(body string, needles ...string) bool {
	lb := strings.ToLower(body)
	for _, n := range needles {
		if strings.Contains(lb, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

// countAnyFold counts distinct needles that appear in body (case-insensitive).
// Useful for "number of city names mentioned" metrics.
func countAnyFold(body string, needles []string, cap int) int {
	lb := strings.ToLower(body)
	count := 0
	for _, n := range needles {
		if strings.Contains(lb, strings.ToLower(n)) {
			count++
			if cap > 0 && count >= cap {
				return cap
			}
		}
	}
	return count
}

// mustCompile is a short regexp.MustCompile alias for readability.
func mustCompile(pattern string) *regexp.Regexp { return regexp.MustCompile(pattern) }
