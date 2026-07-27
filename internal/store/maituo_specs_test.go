package store

import (
	"testing"

	"paipai-red-campaign-manager/internal/maituo"
)

func TestMaituoReconcileSpecsOnlyIncludePresentSheets(t *testing.T) {
	snapshot := maituo.Snapshot{PresentSheets: []string{maituo.SheetKPI, maituo.SheetSubaccount}}
	specs := maituoReconcileSpecs(snapshot, true)
	if len(specs) != 2 || specs[0].key != "kpis" || specs[1].key != "subaccounts" {
		t.Fatalf("specs = %+v", specs)
	}
	for _, spec := range specs {
		if spec.deleteScope != "t.report_date=$2" {
			t.Fatalf("delete scope for %s = %q", spec.key, spec.deleteScope)
		}
	}
}

func TestMaituoReconcileSpecsSkipsTrendForOlderWorkbook(t *testing.T) {
	snapshot := maituo.Snapshot{PresentSheets: []string{maituo.SheetTrend}}
	if specs := maituoReconcileSpecs(snapshot, false); len(specs) != 0 {
		t.Fatalf("specs = %+v, want no trend reconciliation", specs)
	}
}
