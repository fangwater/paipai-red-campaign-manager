package store

import (
	"strings"
	"testing"
)

func TestGuoraiMetricAssignmentAcceptsNumericStrings(t *testing.T) {
	assignment := guoraiMetricAssignment(guoraiMetricField{
		column: "total_pay_amt",
		json:   "totalPayAmt",
		cast:   "numeric",
	})
	for _, expected := range []string{"jsonb_typeof", "'number'", "'string'", "::numeric"} {
		if !strings.Contains(assignment, expected) {
			t.Fatalf("assignment %q does not contain %q", assignment, expected)
		}
	}
}

func TestGuoraiMetricAssignmentConvertsBigIntThroughNumeric(t *testing.T) {
	assignment := guoraiMetricAssignment(guoraiMetricField{
		column: "exposure_count",
		json:   "exposureCount",
		cast:   "bigint",
	})
	if !strings.Contains(assignment, "::numeric::bigint") {
		t.Fatalf("assignment %q does not preserve bigint conversion", assignment)
	}
}

func TestGuoraiMetricAssignmentAcceptsBooleanStrings(t *testing.T) {
	assignment := guoraiMetricAssignment(guoraiMetricField{
		column: "is_new",
		json:   "isNew",
		cast:   "boolean",
	})
	for _, expected := range []string{"'boolean'", "'string'", "LOWER", "::boolean"} {
		if !strings.Contains(assignment, expected) {
			t.Fatalf("assignment %q does not contain %q", assignment, expected)
		}
	}
}
