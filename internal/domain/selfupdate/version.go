package selfupdate

import (
	"fmt"
	"strconv"
	"strings"
)

type Version struct {
	Major int
	Minor int
	Patch int
}

func ParseVersion(s string) (Version, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.SplitN(trimmed, ".", 3)
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("selfupdate: invalid version %q", s)
	}

	nums := make([]int, 3)
	for i, part := range parts {
		part = strings.SplitN(part, "-", 2)[0]
		part = strings.SplitN(part, "+", 2)[0]
		n, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, fmt.Errorf("selfupdate: invalid version %q: %w", s, err)
		}
		nums[i] = n
	}

	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}, nil
}

func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		return sign(v.Major - other.Major)
	}
	if v.Minor != other.Minor {
		return sign(v.Minor - other.Minor)
	}
	return sign(v.Patch - other.Patch)
}

func (v Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	default:
		return 0
	}
}

func IsNewer(current, latest string) (bool, error) {
	currentVersion, err := ParseVersion(current)
	if err != nil {
		return false, err
	}
	latestVersion, err := ParseVersion(latest)
	if err != nil {
		return false, err
	}
	return latestVersion.Compare(currentVersion) > 0, nil
}
