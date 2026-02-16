package plugins

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents a semantic version.
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion parses a semantic version string (e.g., "1.2.3").
// Returns an error if the version string is invalid.
func ParseVersion(v string) (*Version, error) {
	if v == "" {
		return nil, fmt.Errorf("version string is empty")
	}

	// Remove 'v' prefix if present
	v = strings.TrimPrefix(v, "v")

	parts := strings.Split(v, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return nil, fmt.Errorf("invalid version format: %s (expected X.Y.Z)", v)
	}

	ver := &Version{}
	var err error

	// Parse major version
	ver.Major, err = strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", parts[0])
	}

	// Parse minor version (default to 0 if not present)
	if len(parts) > 1 {
		ver.Minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid minor version: %s", parts[1])
		}
	}

	// Parse patch version (default to 0 if not present)
	if len(parts) > 2 {
		ver.Patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid patch version: %s", parts[2])
		}
	}

	return ver, nil
}

// String returns the string representation of the version.
func (v *Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare compares two versions.
// Returns:
//
//	-1 if v < other
//	 0 if v == other
//	+1 if v > other
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// IsCompatibleWith checks if this version is compatible with the required version
// based on semantic versioning rules:
// - Major version must match (breaking changes)
// - Minor version must be >= required (backward compatible features)
// - Patch version is ignored (bug fixes are always compatible)
func (v *Version) IsCompatibleWith(required *Version) bool {
	// Major version must match exactly (breaking changes)
	if v.Major != required.Major {
		return false
	}

	// Minor version must be >= required (new features are backward compatible)
	if v.Minor < required.Minor {
		return false
	}

	// Patch version doesn't matter for compatibility
	// (bug fixes should always be compatible)
	return true
}

// Constraint represents a version constraint (e.g., ">=1.2.0", "<2.0.0").
type Constraint struct {
	Operator string // ">=", ">", "=", "<", "<="
	Version  *Version
}

// ParseConstraint parses a version constraint string.
// Supported operators: >=, >, =, <, <=
func ParseConstraint(s string) (*Constraint, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("constraint string is empty")
	}

	// Check for operators
	operators := []string{">=", "<=", ">", "<", "="}
	for _, op := range operators {
		if strings.HasPrefix(s, op) {
			versionStr := strings.TrimSpace(s[len(op):])
			version, err := ParseVersion(versionStr)
			if err != nil {
				return nil, fmt.Errorf("invalid constraint version: %w", err)
			}
			return &Constraint{
				Operator: op,
				Version:  version,
			}, nil
		}
	}

	// No operator means exact match
	version, err := ParseVersion(s)
	if err != nil {
		return nil, fmt.Errorf("invalid constraint version: %w", err)
	}
	return &Constraint{
		Operator: "=",
		Version:  version,
	}, nil
}

// Check checks if a version satisfies this constraint.
func (c *Constraint) Check(v *Version) bool {
	cmp := v.Compare(c.Version)
	switch c.Operator {
	case ">=":
		return cmp >= 0
	case ">":
		return cmp > 0
	case "=":
		return cmp == 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	default:
		return false
	}
}

// String returns the string representation of the constraint.
func (c *Constraint) String() string {
	return c.Operator + c.Version.String()
}

// ConstraintSet represents a set of version constraints that must all be satisfied.
type ConstraintSet struct {
	Constraints []*Constraint
}

// ParseConstraintSet parses a constraint set string.
// Multiple constraints can be separated by commas (e.g., ">=1.0.0,<2.0.0").
func ParseConstraintSet(s string) (*ConstraintSet, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return &ConstraintSet{}, nil
	}

	parts := strings.Split(s, ",")
	constraints := make([]*Constraint, 0, len(parts))

	for _, part := range parts {
		constraint, err := ParseConstraint(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		constraints = append(constraints, constraint)
	}

	return &ConstraintSet{
		Constraints: constraints,
	}, nil
}

// Check checks if a version satisfies all constraints in the set.
func (cs *ConstraintSet) Check(v *Version) bool {
	for _, constraint := range cs.Constraints {
		if !constraint.Check(v) {
			return false
		}
	}
	return true
}

// String returns the string representation of the constraint set.
func (cs *ConstraintSet) String() string {
	if len(cs.Constraints) == 0 {
		return ""
	}
	parts := make([]string, len(cs.Constraints))
	for i, c := range cs.Constraints {
		parts[i] = c.String()
	}
	return strings.Join(parts, ",")
}
