package selfcheck

import (
	"github.com/JuniperBible/juniper/core/ir"
)

// LossBudget defines acceptable loss thresholds for a conversion.
type LossBudget struct {
	// MaxLossClass is the maximum acceptable loss class (e.g., L1 means L0 and L1 are ok).
	MaxLossClass ir.LossClass `json:"max_loss_class"`

	// MaxLostElements is the maximum number of lost elements allowed (0 = any).
	MaxLostElements int `json:"max_lost_elements,omitempty"`

	// AllowedElementTypes specifies which element types can be lost.
	// If empty, all types are checked. If set, only listed types are counted.
	AllowedElementTypes []string `json:"allowed_element_types,omitempty"`
}

// NewLossBudget creates a budget allowing up to the specified loss class.
func NewLossBudget(maxClass ir.LossClass) *LossBudget {
	return &LossBudget{
		MaxLossClass: maxClass,
	}
}

// LosslessOnly creates a budget that only allows L0 (lossless) conversions.
func LosslessOnly() *LossBudget {
	return &LossBudget{
		MaxLossClass: ir.LossL0,
	}
}

// SemanticallyLossless creates a budget allowing L0-L1 (content preserved).
func SemanticallyLossless() *LossBudget {
	return &LossBudget{
		MaxLossClass: ir.LossL1,
	}
}

// IsWithinBudget checks if a loss report is within the budget constraints.
func (b *LossBudget) IsWithinBudget(report *ir.LossReport) bool {
	if report == nil {
		return true
	}

	// Check loss class
	if report.LossClass.Level() > b.MaxLossClass.Level() {
		return false
	}

	// Check lost element count if limit is set
	if b.MaxLostElements > 0 {
		count := b.countRelevantLostElements(report)
		if count > b.MaxLostElements {
			return false
		}
	}

	return true
}

// countRelevantLostElements counts lost elements that match the allowed types.
func (b *LossBudget) countRelevantLostElements(report *ir.LossReport) int {
	if len(b.AllowedElementTypes) == 0 {
		return len(report.LostElements)
	}

	count := 0
	allowed := make(map[string]bool)
	for _, t := range b.AllowedElementTypes {
		allowed[t] = true
	}

	for _, elem := range report.LostElements {
		if !allowed[elem.ElementType] {
			// This type is not in the allowed list, so it counts against the budget
			count++
		}
	}

	return count
}

// LossBudgetResult describes the result of checking a loss report against a budget.
type LossBudgetResult struct {
	// WithinBudget is true if the report is within the budget.
	WithinBudget bool `json:"within_budget"`

	// ActualLossClass is the loss class from the report.
	ActualLossClass ir.LossClass `json:"actual_loss_class"`

	// MaxAllowedClass is the maximum allowed from the budget.
	MaxAllowedClass ir.LossClass `json:"max_allowed_class"`

	// LostElementCount is the number of lost elements.
	LostElementCount int `json:"lost_element_count"`

	// Violations lists specific violations.
	Violations []string `json:"violations,omitempty"`
}

// Check performs a detailed check and returns a result.
func (b *LossBudget) Check(report *ir.LossReport) *LossBudgetResult {
	result := &LossBudgetResult{
		MaxAllowedClass: b.MaxLossClass,
		WithinBudget:    true,
	}

	if report == nil {
		result.ActualLossClass = ir.LossL0
		return result
	}

	result.ActualLossClass = report.LossClass
	result.LostElementCount = len(report.LostElements)

	// Check loss class
	if report.LossClass.Level() > b.MaxLossClass.Level() {
		result.WithinBudget = false
		result.Violations = append(result.Violations,
			"loss class "+string(report.LossClass)+" exceeds budget "+string(b.MaxLossClass))
	}

	// Check lost element count
	if b.MaxLostElements > 0 {
		count := b.countRelevantLostElements(report)
		if count > b.MaxLostElements {
			result.WithinBudget = false
			result.Violations = append(result.Violations,
				"lost element count exceeds budget")
		}
	}

	return result
}
