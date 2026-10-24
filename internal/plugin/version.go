package plugin

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/komari-monitor/komari/utils"
)

// CheckKomariVersion validates a manifest komari constraint against the
// running server version. Supported constraints: empty (any version),
// "x.y.z" (exact), and ">=x.y.z", ">x.y.z", "<=x.y.z", "<x.y.z". An optional
// leading "v" is accepted.
func CheckKomariVersion(constraint string) error {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return nil
	}
	rest := constraint
	op := ""
	for _, candidate := range []string{">=", "<=", ">", "<", "="} {
		if strings.HasPrefix(rest, candidate) {
			op = candidate
			rest = strings.TrimSpace(strings.TrimPrefix(rest, candidate))
			break
		}
	}
	want, err := semver.ParseTolerant(rest)
	if err != nil {
		return fmt.Errorf("invalid komari version constraint %q: %w", constraint, err)
	}
	have, err := semver.ParseTolerant(utils.CurrentVersion)
	if err != nil {
		return fmt.Errorf("cannot verify plugin compatibility with server version %q", utils.CurrentVersion)
	}
	if !satisfies(have.Compare(want), op) {
		return fmt.Errorf("plugin requires komari %s, running %s", constraint, utils.CurrentVersion)
	}
	return nil
}

func satisfies(cmp int, op string) bool {
	switch op {
	case ">=":
		return cmp >= 0
	case ">":
		return cmp > 0
	case "<=":
		return cmp <= 0
	case "<":
		return cmp < 0
	default: // exact
		return cmp == 0
	}
}
