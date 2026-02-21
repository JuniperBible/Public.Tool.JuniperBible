package selfcheck

import (
	"testing"

	"github.com/JuniperBible/juniper/core/ir"
)

func TestLossBudgetIsWithinBudget(t *testing.T) {
	tests := []struct {
		name     string
		budget   *LossBudget
		report   *ir.LossReport
		expected bool
	}{
		{
			name:     "nil report within any budget",
			budget:   NewLossBudget(ir.LossL4),
			report:   nil,
			expected: true,
		},
		{
			name:   "L0 report within L0 budget",
			budget: LosslessOnly(),
			report: &ir.LossReport{
				LossClass: ir.LossL0,
			},
			expected: true,
		},
		{
			name:   "L1 report exceeds L0 budget",
			budget: LosslessOnly(),
			report: &ir.LossReport{
				LossClass: ir.LossL1,
			},
			expected: false,
		},
		{
			name:   "L1 report within L1 budget",
			budget: SemanticallyLossless(),
			report: &ir.LossReport{
				LossClass: ir.LossL1,
			},
			expected: true,
		},
		{
			name:   "L2 report exceeds L1 budget",
			budget: SemanticallyLossless(),
			report: &ir.LossReport{
				LossClass: ir.LossL2,
			},
			expected: false,
		},
		{
			name:   "L3 report within L4 budget",
			budget: NewLossBudget(ir.LossL4),
			report: &ir.LossReport{
				LossClass: ir.LossL3,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.budget.IsWithinBudget(tt.report)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestLossBudgetMaxLostElements(t *testing.T) {
	budget := &LossBudget{
		MaxLossClass:    ir.LossL2,
		MaxLostElements: 3,
	}

	tests := []struct {
		name       string
		lostCount  int
		expectedOK bool
	}{
		{"0 lost elements", 0, true},
		{"1 lost element", 1, true},
		{"3 lost elements", 3, true},
		{"4 lost elements", 4, false},
		{"10 lost elements", 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &ir.LossReport{
				LossClass:    ir.LossL1,
				LostElements: make([]ir.LostElement, tt.lostCount),
			}

			result := budget.IsWithinBudget(report)
			if result != tt.expectedOK {
				t.Errorf("expected %v, got %v", tt.expectedOK, result)
			}
		})
	}
}

func TestLossBudgetAllowedElementTypes(t *testing.T) {
	// Budget that only counts "critical" and "important" elements
	budget := &LossBudget{
		MaxLossClass:        ir.LossL2,
		MaxLostElements:     2,
		AllowedElementTypes: []string{"formatting", "whitespace"}, // These are allowed to be lost
	}

	tests := []struct {
		name       string
		lostTypes  []string
		expectedOK bool
	}{
		{
			name:       "no lost elements",
			lostTypes:  []string{},
			expectedOK: true,
		},
		{
			name:       "only allowed types lost",
			lostTypes:  []string{"formatting", "whitespace", "formatting"},
			expectedOK: true,
		},
		{
			name:       "one disallowed type lost",
			lostTypes:  []string{"critical"},
			expectedOK: true, // 1 disallowed is within budget of 2
		},
		{
			name:       "two disallowed types lost",
			lostTypes:  []string{"critical", "important"},
			expectedOK: true, // 2 is within budget
		},
		{
			name:       "three disallowed types lost",
			lostTypes:  []string{"critical", "important", "strongs"},
			expectedOK: false, // 3 exceeds budget of 2
		},
		{
			name:       "mixed types",
			lostTypes:  []string{"formatting", "critical", "whitespace", "important"},
			expectedOK: true, // Only 2 disallowed (critical, important), within budget
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lostElements := make([]ir.LostElement, len(tt.lostTypes))
			for i, elemType := range tt.lostTypes {
				lostElements[i] = ir.LostElement{ElementType: elemType}
			}

			report := &ir.LossReport{
				LossClass:    ir.LossL1,
				LostElements: lostElements,
			}

			result := budget.IsWithinBudget(report)
			if result != tt.expectedOK {
				t.Errorf("expected %v, got %v", tt.expectedOK, result)
			}
		})
	}
}

func TestLossBudgetCheck(t *testing.T) {
	budget := SemanticallyLossless()

	// Test passing case
	t.Run("passing case", func(t *testing.T) {
		report := &ir.LossReport{
			LossClass: ir.LossL0,
		}

		result := budget.Check(report)
		if !result.WithinBudget {
			t.Error("expected within budget")
		}
		if result.ActualLossClass != ir.LossL0 {
			t.Errorf("expected L0, got %s", result.ActualLossClass)
		}
		if len(result.Violations) > 0 {
			t.Errorf("expected no violations, got %v", result.Violations)
		}
	})

	// Test failing case
	t.Run("failing case", func(t *testing.T) {
		report := &ir.LossReport{
			LossClass: ir.LossL3,
		}

		result := budget.Check(report)
		if result.WithinBudget {
			t.Error("expected outside budget")
		}
		if len(result.Violations) == 0 {
			t.Error("expected violations")
		}
	})

	// Test with nil report
	t.Run("nil report", func(t *testing.T) {
		result := budget.Check(nil)
		if !result.WithinBudget {
			t.Error("expected within budget for nil report")
		}
		if result.ActualLossClass != ir.LossL0 {
			t.Errorf("expected L0, got %s", result.ActualLossClass)
		}
	})

	// Test with MaxLostElements violation
	t.Run("max lost elements violation", func(t *testing.T) {
		strictBudget := &LossBudget{
			MaxLossClass:    ir.LossL1,
			MaxLostElements: 2,
		}

		report := &ir.LossReport{
			LossClass: ir.LossL1,
			LostElements: []ir.LostElement{
				{ElementType: "type1"},
				{ElementType: "type2"},
				{ElementType: "type3"},
			},
		}

		result := strictBudget.Check(report)
		if result.WithinBudget {
			t.Error("expected outside budget due to too many lost elements")
		}
		if len(result.Violations) == 0 {
			t.Error("expected violations")
		}
		if result.LostElementCount != 3 {
			t.Errorf("expected 3 lost elements, got %d", result.LostElementCount)
		}
	})
}
