package migrations

import (
	"sort"
	"strconv"
	"strings"
)

// sortMigrations sorts a slice of [Migration] in semver ascending order, in
// place. Uses [compareSemver] as the comparison key.
func sortMigrations(ms []Migration) {
	sort.Slice(ms, func(i, j int) bool {
		return compareSemver(ms[i].Version(), ms[j].Version()) < 0
	})
}

// compareSemver compares two semver strings (MAJOR.MINOR.PATCH).
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
// Any segment that cannot be parsed as an integer is treated as 0.
// Optional leading "v" prefix is stripped before parsing.
func compareSemver(a, b string) int {
	aParts := parseSemver(a)
	bParts := parseSemver(b)
	for i := 0; i < 3; i++ {
		if aParts[i] < bParts[i] {
			return -1
		}
		if aParts[i] > bParts[i] {
			return 1
		}
	}
	return 0
}

// parseSemver splits a version string into [MAJOR, MINOR, PATCH] ints.
// Segments that fail to parse are returned as 0. Handles optional "v" prefix.
func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err == nil {
			result[i] = n
		}
	}
	return result
}
