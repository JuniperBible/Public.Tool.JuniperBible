package plugins

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Version
		wantErr bool
	}{
		{
			name:  "standard version",
			input: "1.2.3",
			want:  &Version{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:  "version with v prefix",
			input: "v1.2.3",
			want:  &Version{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:  "major.minor only",
			input: "1.2",
			want:  &Version{Major: 1, Minor: 2, Patch: 0},
		},
		{
			name:  "major only",
			input: "1",
			want:  &Version{Major: 1, Minor: 0, Patch: 0},
		},
		{
			name:  "zero version",
			input: "0.0.0",
			want:  &Version{Major: 0, Minor: 0, Patch: 0},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "1.2.3.4",
			wantErr: true,
		},
		{
			name:    "non-numeric major",
			input:   "x.2.3",
			wantErr: true,
		},
		{
			name:    "non-numeric minor",
			input:   "1.x.3",
			wantErr: true,
		},
		{
			name:    "non-numeric patch",
			input:   "1.2.x",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Major != tt.want.Major || got.Minor != tt.want.Minor || got.Patch != tt.want.Patch {
					t.Errorf("ParseVersion() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		name    string
		version *Version
		want    string
	}{
		{
			name:    "standard version",
			version: &Version{Major: 1, Minor: 2, Patch: 3},
			want:    "1.2.3",
		},
		{
			name:    "zero version",
			version: &Version{Major: 0, Minor: 0, Patch: 0},
			want:    "0.0.0",
		},
		{
			name:    "major only",
			version: &Version{Major: 5, Minor: 0, Patch: 0},
			want:    "5.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.version.String(); got != tt.want {
				t.Errorf("Version.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		name  string
		v1    *Version
		v2    *Version
		want  int
	}{
		{
			name: "equal versions",
			v1:   &Version{Major: 1, Minor: 2, Patch: 3},
			v2:   &Version{Major: 1, Minor: 2, Patch: 3},
			want: 0,
		},
		{
			name: "major version less",
			v1:   &Version{Major: 1, Minor: 2, Patch: 3},
			v2:   &Version{Major: 2, Minor: 2, Patch: 3},
			want: -1,
		},
		{
			name: "major version greater",
			v1:   &Version{Major: 2, Minor: 2, Patch: 3},
			v2:   &Version{Major: 1, Minor: 2, Patch: 3},
			want: 1,
		},
		{
			name: "minor version less",
			v1:   &Version{Major: 1, Minor: 2, Patch: 3},
			v2:   &Version{Major: 1, Minor: 3, Patch: 3},
			want: -1,
		},
		{
			name: "minor version greater",
			v1:   &Version{Major: 1, Minor: 3, Patch: 3},
			v2:   &Version{Major: 1, Minor: 2, Patch: 3},
			want: 1,
		},
		{
			name: "patch version less",
			v1:   &Version{Major: 1, Minor: 2, Patch: 3},
			v2:   &Version{Major: 1, Minor: 2, Patch: 4},
			want: -1,
		},
		{
			name: "patch version greater",
			v1:   &Version{Major: 1, Minor: 2, Patch: 4},
			v2:   &Version{Major: 1, Minor: 2, Patch: 3},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v1.Compare(tt.v2); got != tt.want {
				t.Errorf("Version.Compare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersionIsCompatibleWith(t *testing.T) {
	tests := []struct {
		name     string
		version  *Version
		required *Version
		want     bool
	}{
		{
			name:     "exact match",
			version:  &Version{Major: 1, Minor: 2, Patch: 3},
			required: &Version{Major: 1, Minor: 2, Patch: 3},
			want:     true,
		},
		{
			name:     "same major, higher minor",
			version:  &Version{Major: 1, Minor: 3, Patch: 0},
			required: &Version{Major: 1, Minor: 2, Patch: 0},
			want:     true,
		},
		{
			name:     "same major and minor, higher patch",
			version:  &Version{Major: 1, Minor: 2, Patch: 5},
			required: &Version{Major: 1, Minor: 2, Patch: 3},
			want:     true,
		},
		{
			name:     "same major and minor, lower patch",
			version:  &Version{Major: 1, Minor: 2, Patch: 1},
			required: &Version{Major: 1, Minor: 2, Patch: 3},
			want:     true, // Patch version doesn't affect compatibility
		},
		{
			name:     "same major, lower minor",
			version:  &Version{Major: 1, Minor: 1, Patch: 0},
			required: &Version{Major: 1, Minor: 2, Patch: 0},
			want:     false,
		},
		{
			name:     "different major version",
			version:  &Version{Major: 2, Minor: 0, Patch: 0},
			required: &Version{Major: 1, Minor: 0, Patch: 0},
			want:     false,
		},
		{
			name:     "lower major version",
			version:  &Version{Major: 1, Minor: 0, Patch: 0},
			required: &Version{Major: 2, Minor: 0, Patch: 0},
			want:     false,
		},
		{
			name:     "version 0.5.0 compatible with 0.4.0",
			version:  &Version{Major: 0, Minor: 5, Patch: 0},
			required: &Version{Major: 0, Minor: 4, Patch: 0},
			want:     true,
		},
		{
			name:     "version 0.4.0 not compatible with 0.5.0",
			version:  &Version{Major: 0, Minor: 4, Patch: 0},
			required: &Version{Major: 0, Minor: 5, Patch: 0},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.version.IsCompatibleWith(tt.required); got != tt.want {
				t.Errorf("Version.IsCompatibleWith() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseConstraint(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Constraint
		wantErr bool
	}{
		{
			name:  "greater than or equal",
			input: ">=1.2.3",
			want:  &Constraint{Operator: ">=", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
		},
		{
			name:  "greater than",
			input: ">1.2.3",
			want:  &Constraint{Operator: ">", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
		},
		{
			name:  "equal",
			input: "=1.2.3",
			want:  &Constraint{Operator: "=", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
		},
		{
			name:  "less than",
			input: "<1.2.3",
			want:  &Constraint{Operator: "<", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
		},
		{
			name:  "less than or equal",
			input: "<=1.2.3",
			want:  &Constraint{Operator: "<=", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
		},
		{
			name:  "no operator defaults to equal",
			input: "1.2.3",
			want:  &Constraint{Operator: "=", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
		},
		{
			name:  "with spaces",
			input: ">= 1.2.3",
			want:  &Constraint{Operator: ">=", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid version",
			input:   ">=invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConstraint(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConstraint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Operator != tt.want.Operator {
					t.Errorf("ParseConstraint() operator = %v, want %v", got.Operator, tt.want.Operator)
				}
				if got.Version.Compare(tt.want.Version) != 0 {
					t.Errorf("ParseConstraint() version = %v, want %v", got.Version, tt.want.Version)
				}
			}
		})
	}
}

func TestConstraintCheck(t *testing.T) {
	tests := []struct {
		name       string
		constraint *Constraint
		version    *Version
		want       bool
	}{
		{
			name:       "greater than or equal - equal",
			constraint: &Constraint{Operator: ">=", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
			version:    &Version{Major: 1, Minor: 2, Patch: 3},
			want:       true,
		},
		{
			name:       "greater than or equal - greater",
			constraint: &Constraint{Operator: ">=", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
			version:    &Version{Major: 1, Minor: 3, Patch: 0},
			want:       true,
		},
		{
			name:       "greater than or equal - less",
			constraint: &Constraint{Operator: ">=", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
			version:    &Version{Major: 1, Minor: 1, Patch: 0},
			want:       false,
		},
		{
			name:       "greater than - greater",
			constraint: &Constraint{Operator: ">", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
			version:    &Version{Major: 1, Minor: 3, Patch: 0},
			want:       true,
		},
		{
			name:       "greater than - equal",
			constraint: &Constraint{Operator: ">", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
			version:    &Version{Major: 1, Minor: 2, Patch: 3},
			want:       false,
		},
		{
			name:       "equal - match",
			constraint: &Constraint{Operator: "=", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
			version:    &Version{Major: 1, Minor: 2, Patch: 3},
			want:       true,
		},
		{
			name:       "equal - no match",
			constraint: &Constraint{Operator: "=", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
			version:    &Version{Major: 1, Minor: 2, Patch: 4},
			want:       false,
		},
		{
			name:       "less than - less",
			constraint: &Constraint{Operator: "<", Version: &Version{Major: 2, Minor: 0, Patch: 0}},
			version:    &Version{Major: 1, Minor: 9, Patch: 9},
			want:       true,
		},
		{
			name:       "less than - equal",
			constraint: &Constraint{Operator: "<", Version: &Version{Major: 2, Minor: 0, Patch: 0}},
			version:    &Version{Major: 2, Minor: 0, Patch: 0},
			want:       false,
		},
		{
			name:       "less than or equal - equal",
			constraint: &Constraint{Operator: "<=", Version: &Version{Major: 2, Minor: 0, Patch: 0}},
			version:    &Version{Major: 2, Minor: 0, Patch: 0},
			want:       true,
		},
		{
			name:       "less than or equal - less",
			constraint: &Constraint{Operator: "<=", Version: &Version{Major: 2, Minor: 0, Patch: 0}},
			version:    &Version{Major: 1, Minor: 9, Patch: 9},
			want:       true,
		},
		{
			name:       "less than or equal - greater",
			constraint: &Constraint{Operator: "<=", Version: &Version{Major: 2, Minor: 0, Patch: 0}},
			version:    &Version{Major: 2, Minor: 1, Patch: 0},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.constraint.Check(tt.version); got != tt.want {
				t.Errorf("Constraint.Check() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseConstraintSet(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr bool
	}{
		{
			name:    "single constraint",
			input:   ">=1.0.0",
			wantLen: 1,
		},
		{
			name:    "multiple constraints",
			input:   ">=1.0.0,<2.0.0",
			wantLen: 2,
		},
		{
			name:    "multiple constraints with spaces",
			input:   ">= 1.0.0, < 2.0.0",
			wantLen: 2,
		},
		{
			name:    "empty string",
			input:   "",
			wantLen: 0,
		},
		{
			name:    "invalid constraint in set",
			input:   ">=1.0.0,invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConstraintSet(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConstraintSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got.Constraints) != tt.wantLen {
				t.Errorf("ParseConstraintSet() len = %v, want %v", len(got.Constraints), tt.wantLen)
			}
		})
	}
}

func TestConstraintString(t *testing.T) {
	tests := []struct {
		name       string
		constraint *Constraint
		want       string
	}{
		{
			name:       "greater than or equal",
			constraint: &Constraint{Operator: ">=", Version: &Version{Major: 1, Minor: 2, Patch: 3}},
			want:       ">=1.2.3",
		},
		{
			name:       "less than",
			constraint: &Constraint{Operator: "<", Version: &Version{Major: 2, Minor: 0, Patch: 0}},
			want:       "<2.0.0",
		},
		{
			name:       "equal",
			constraint: &Constraint{Operator: "=", Version: &Version{Major: 1, Minor: 0, Patch: 0}},
			want:       "=1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.constraint.String(); got != tt.want {
				t.Errorf("Constraint.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConstraintSetString(t *testing.T) {
	tests := []struct {
		name        string
		constraints string
		want        string
	}{
		{
			name:        "single constraint",
			constraints: ">=1.0.0",
			want:        ">=1.0.0",
		},
		{
			name:        "multiple constraints",
			constraints: ">=1.0.0,<2.0.0",
			want:        ">=1.0.0,<2.0.0",
		},
		{
			name:        "empty",
			constraints: "",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs, err := ParseConstraintSet(tt.constraints)
			if err != nil {
				t.Fatalf("ParseConstraintSet() error = %v", err)
			}
			if got := cs.String(); got != tt.want {
				t.Errorf("ConstraintSet.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConstraintCheckInvalidOperator(t *testing.T) {
	// Test the default case in Constraint.Check() with an invalid operator
	constraint := &Constraint{Operator: "invalid", Version: &Version{Major: 1, Minor: 0, Patch: 0}}
	version := &Version{Major: 1, Minor: 0, Patch: 0}

	if got := constraint.Check(version); got != false {
		t.Errorf("Constraint.Check() with invalid operator = %v, want false", got)
	}
}

func TestConstraintSetCheck(t *testing.T) {
	tests := []struct {
		name        string
		constraints string
		version     *Version
		want        bool
	}{
		{
			name:        "single constraint - pass",
			constraints: ">=1.0.0",
			version:     &Version{Major: 1, Minor: 5, Patch: 0},
			want:        true,
		},
		{
			name:        "single constraint - fail",
			constraints: ">=1.0.0",
			version:     &Version{Major: 0, Minor: 9, Patch: 0},
			want:        false,
		},
		{
			name:        "range constraint - within range",
			constraints: ">=1.0.0,<2.0.0",
			version:     &Version{Major: 1, Minor: 5, Patch: 0},
			want:        true,
		},
		{
			name:        "range constraint - below range",
			constraints: ">=1.0.0,<2.0.0",
			version:     &Version{Major: 0, Minor: 9, Patch: 0},
			want:        false,
		},
		{
			name:        "range constraint - above range",
			constraints: ">=1.0.0,<2.0.0",
			version:     &Version{Major: 2, Minor: 0, Patch: 0},
			want:        false,
		},
		{
			name:        "empty constraint set - always pass",
			constraints: "",
			version:     &Version{Major: 0, Minor: 1, Patch: 0},
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs, err := ParseConstraintSet(tt.constraints)
			if err != nil {
				t.Fatalf("ParseConstraintSet() error = %v", err)
			}
			if got := cs.Check(tt.version); got != tt.want {
				t.Errorf("ConstraintSet.Check() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckPluginCompatibility(t *testing.T) {
	tests := []struct {
		name        string
		manifest    *PluginManifest
		hostVersion string
		wantErr     bool
	}{
		{
			name: "no minimum version - compatible",
			manifest: &PluginManifest{
				PluginID:       "test.plugin",
				Version:        "1.0.0",
				MinHostVersion: "",
			},
			hostVersion: "0.5.0",
			wantErr:     false,
		},
		{
			name: "compatible version - exact match",
			manifest: &PluginManifest{
				PluginID:       "test.plugin",
				Version:        "1.0.0",
				MinHostVersion: "0.5.0",
			},
			hostVersion: "0.5.0",
			wantErr:     false,
		},
		{
			name: "compatible version - higher minor",
			manifest: &PluginManifest{
				PluginID:       "test.plugin",
				Version:        "1.0.0",
				MinHostVersion: "0.4.0",
			},
			hostVersion: "0.5.0",
			wantErr:     false,
		},
		{
			name: "incompatible version - lower minor",
			manifest: &PluginManifest{
				PluginID:       "test.plugin",
				Version:        "1.0.0",
				MinHostVersion: "0.6.0",
			},
			hostVersion: "0.5.0",
			wantErr:     true,
		},
		{
			name: "incompatible version - different major",
			manifest: &PluginManifest{
				PluginID:       "test.plugin",
				Version:        "1.0.0",
				MinHostVersion: "1.0.0",
			},
			hostVersion: "0.5.0",
			wantErr:     true,
		},
		{
			name: "invalid host version",
			manifest: &PluginManifest{
				PluginID:       "test.plugin",
				Version:        "1.0.0",
				MinHostVersion: "0.5.0",
			},
			hostVersion: "invalid",
			wantErr:     true,
		},
		{
			name: "invalid min host version",
			manifest: &PluginManifest{
				PluginID:       "test.plugin",
				Version:        "1.0.0",
				MinHostVersion: "invalid",
			},
			hostVersion: "0.5.0",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPluginCompatibility(tt.manifest, tt.hostVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPluginCompatibility() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
