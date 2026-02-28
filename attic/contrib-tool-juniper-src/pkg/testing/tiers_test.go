package testing

import (
	"testing"
	"time"
)

func TestTestTier_String(t *testing.T) {
	tests := []struct {
		tier     TestTier
		expected string
	}{
		{TierQuick, "quick"},
		{TierComprehensive, "comprehensive"},
		{TierExtensive, "extensive"},
		{TestTier(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.tier.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestTestTier_Timeout(t *testing.T) {
	tests := []struct {
		tier     TestTier
		expected time.Duration
	}{
		{TierQuick, 60 * time.Second},
		{TierComprehensive, 5 * time.Minute},
		{TierExtensive, 30 * time.Minute},
		{TestTier(99), time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.tier.String(), func(t *testing.T) {
			if got := tt.tier.Timeout(); got != tt.expected {
				t.Errorf("Timeout() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLoadBibleSets(t *testing.T) {
	config, err := LoadBibleSets()
	if err != nil {
		t.Skipf("bible_sets.yaml not found: %v", err)
	}

	// Check quick tier
	if config.Quick.Description == "" {
		t.Error("Quick tier should have a description")
	}
	if len(config.Quick.Bibles) == 0 {
		t.Error("Quick tier should have at least one Bible")
	}

	// Check comprehensive tier
	if config.Comprehensive.Description == "" {
		t.Error("Comprehensive tier should have a description")
	}
	if len(config.Comprehensive.EnglishMajor) == 0 {
		t.Error("Comprehensive tier should have major English Bibles")
	}

	// Check extensive tier
	if config.Extensive.Description == "" {
		t.Error("Extensive tier should have a description")
	}
}

func TestGetBiblesForTier_Quick(t *testing.T) {
	bibles := GetBiblesForTier(TierQuick)
	if len(bibles) == 0 {
		t.Error("Quick tier should return at least one Bible")
	}

	// Check that default quick Bibles are present
	expected := map[string]bool{
		"KJV":                true,
		"Tyndale":            true,
		"Geneva1599":         true,
		"DRC":                true,
		"VulgClementwordsof": true,
	}

	for _, bible := range bibles {
		if expected[bible] {
			delete(expected, bible)
		}
	}

	if len(expected) > 0 {
		t.Errorf("Missing expected Bibles in quick tier: %v", expected)
	}
}

func TestGetBiblesForTier_Comprehensive(t *testing.T) {
	bibles := GetBiblesForTier(TierComprehensive)
	if len(bibles) == 0 {
		t.Error("Comprehensive tier should return at least one Bible")
	}

	// Comprehensive should have more Bibles than quick
	quickBibles := GetBiblesForTier(TierQuick)
	if len(bibles) <= len(quickBibles) {
		t.Errorf("Comprehensive (%d) should have more Bibles than quick (%d)",
			len(bibles), len(quickBibles))
	}
}

func TestGetBiblesForTier_Extensive(t *testing.T) {
	bibles := GetBiblesForTier(TierExtensive)
	if len(bibles) == 0 {
		t.Error("Extensive tier should return at least one Bible")
	}

	// Extensive should have at least as many as comprehensive
	comprehensiveBibles := GetBiblesForTier(TierComprehensive)
	if len(bibles) < len(comprehensiveBibles) {
		t.Errorf("Extensive (%d) should have at least as many Bibles as comprehensive (%d)",
			len(bibles), len(comprehensiveBibles))
	}
}

func TestGetCommentariesForTier(t *testing.T) {
	// Quick tier should have no commentaries
	quickCommentaries := GetCommentariesForTier(TierQuick)
	if len(quickCommentaries) != 0 {
		t.Errorf("Quick tier should have no commentaries, got %d", len(quickCommentaries))
	}

	// Comprehensive tier should have commentaries
	comprehensiveCommentaries := GetCommentariesForTier(TierComprehensive)
	if len(comprehensiveCommentaries) == 0 {
		t.Log("Comprehensive tier has no commentaries (may be intentional)")
	}
}

func TestGetDictionariesForTier(t *testing.T) {
	// Quick tier should have no dictionaries
	quickDictionaries := GetDictionariesForTier(TierQuick)
	if len(quickDictionaries) != 0 {
		t.Errorf("Quick tier should have no dictionaries, got %d", len(quickDictionaries))
	}

	// Comprehensive tier should have dictionaries
	comprehensiveDictionaries := GetDictionariesForTier(TierComprehensive)
	if len(comprehensiveDictionaries) == 0 {
		t.Log("Comprehensive tier has no dictionaries (may be intentional)")
	}
}

func TestGetValidationRules(t *testing.T) {
	rules := GetValidationRules()

	// Check min verses
	if rules.MinVerses == nil {
		t.Error("Validation rules should have min_verses")
	}
	if rules.MinVerses["full_bible"] < 20000 {
		t.Errorf("Expected full_bible min verses > 20000, got %d", rules.MinVerses["full_bible"])
	}

	// Check expected books
	if rules.ExpectedBooks == nil {
		t.Error("Validation rules should have expected_books")
	}
	if rules.ExpectedBooks["protestant"] != 66 {
		t.Errorf("Expected protestant canon = 66 books, got %d", rules.ExpectedBooks["protestant"])
	}

	// Check required references
	if len(rules.RequiredReferences) == 0 {
		t.Error("Validation rules should have required_references")
	}
}

func TestIsQuickBible(t *testing.T) {
	tests := []struct {
		id       string
		expected bool
	}{
		{"KJV", true},
		{"Tyndale", true},
		{"Geneva1599", true},
		{"DRC", true},
		{"VulgClementwordsof", true},
		{"NonExistent", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := IsQuickBible(tt.id); got != tt.expected {
				t.Errorf("IsQuickBible(%q) = %v, want %v", tt.id, got, tt.expected)
			}
		})
	}
}

func TestQuickBibles(t *testing.T) {
	if len(QuickBibles) != 5 {
		t.Errorf("Expected 5 quick Bibles, got %d", len(QuickBibles))
	}

	expected := []string{"KJV", "Tyndale", "Geneva1599", "DRC", "VulgClementwordsof"}
	for i, bible := range expected {
		if QuickBibles[i] != bible {
			t.Errorf("QuickBibles[%d] = %q, want %q", i, QuickBibles[i], bible)
		}
	}
}
