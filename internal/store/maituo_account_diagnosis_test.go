package store

import "testing"

func diagnosisCost(value float64) *float64 {
	return &value
}

func TestDiagnosisAccountStatus(t *testing.T) {
	tests := []struct {
		name string
		cost *float64
		want string
	}{
		{name: "unattributed", want: "unattributed"},
		{name: "good", cost: diagnosisCost(69.99), want: "good"},
		{name: "over", cost: diagnosisCost(70), want: "over"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := diagnosisAccountStatus(test.cost); got != test.want {
				t.Fatalf("status = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDiagnosisCostMetric(t *testing.T) {
	if got := diagnosisCostMetric("信息流"); got != "预计回流后成本" {
		t.Fatalf("feed metric = %q", got)
	}
	if got := diagnosisCostMetric("搜索"); got != "回搜成本" {
		t.Fatalf("search metric = %q", got)
	}
}
