package compact

import (
	"strings"
	"testing"
)

func TestCompactSummaryValidatorValidateAcceptsValidSummary(t *testing.T) {
	t.Parallel()

	summary, err := (compactSummaryValidator{}).Validate(validSemanticSummary(), 1200)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !strings.Contains(summary, "done:") {
		t.Fatalf("expected validated summary to preserve sections, got %q", summary)
	}
}

func TestCompactSummaryValidatorValidateRejectsBrokenStructure(t *testing.T) {
	t.Parallel()

	_, err := (compactSummaryValidator{}).Validate("[compact_summary]\ndone:\n- ok", 1200)
	if err == nil {
		t.Fatalf("expected invalid summary error")
	}
}

func TestCompactSummaryValidatorValidateTrimsBeforeCheckingStructure(t *testing.T) {
	t.Parallel()

	validator := compactSummaryValidator{}
	longSummary := validSemanticSummary() + "\n\n"
	got, err := validator.Validate(longSummary, len([]rune(validSemanticSummary())))
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got != validSemanticSummary() {
		t.Fatalf("expected normalized summary, got %q", got)
	}
}
