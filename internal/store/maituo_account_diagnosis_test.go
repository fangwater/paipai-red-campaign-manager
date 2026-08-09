package store

import "testing"

func diagnosisCost(value float64) *float64 {
	return &value
}

func TestDiagnosisPlanAction(t *testing.T) {
	tests := []struct {
		name        string
		cost        *float64
		consecutive int
		want        string
	}{
		{name: "inactive", want: "inactive"},
		{name: "enlarge", cost: diagnosisCost(29.99), want: "enlarge"},
		{name: "observe", cost: diagnosisCost(30), consecutive: 2, want: "observe"},
		{name: "stop", cost: diagnosisCost(30), consecutive: 3, want: "stop"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := diagnosisPlanAction(test.cost, test.consecutive, 30); got != test.want {
				t.Fatalf("action = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDiagnosisConsecutiveOverKPI(t *testing.T) {
	coefficient := diagnosisCost(2)
	history := map[string]diagnosisHistoryRow{
		"2026-07-27": {Placement: "搜索", SearchCost: diagnosisCost(17.5), CorrectionCoefficient: coefficient},
		"2026-07-26": {Placement: "搜索", SearchCost: diagnosisCost(15.5), CorrectionCoefficient: coefficient},
		"2026-07-23": {Placement: "搜索", SearchCost: diagnosisCost(14.5), CorrectionCoefficient: coefficient},
	}
	dates := []string{"2026-07-27", "2026-07-26", "2026-07-23"}
	if got := diagnosisConsecutiveOverKPI(history, dates, 30); got != 2 {
		t.Fatalf("consecutive = %d, want 2", got)
	}
}

func TestDiagnosisCorrectedCost(t *testing.T) {
	if got := diagnosisCorrectedCost(diagnosisCost(39.22), diagnosisCost(2)); got == nil || *got != 78.44 {
		t.Fatalf("corrected cost = %v, want 78.44", got)
	}
	if got := diagnosisCorrectedCost(diagnosisCost(39.22), nil); got != nil {
		t.Fatalf("corrected cost without coefficient = %v, want nil", got)
	}
}
