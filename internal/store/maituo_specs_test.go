package store

import (
	"strings"
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

func TestMaituoNoteReconcileSpecUsesCanonicalPlacementKey(t *testing.T) {
	specs := maituoReconcileSpecs(maituo.Snapshot{PresentSheets: []string{maituo.SheetNotes}}, true)
	if len(specs) != 1 {
		t.Fatalf("specs = %+v", specs)
	}
	spec := specs[0]
	if spec.join != "t.report_date=s.report_date AND t.note_id=s.note_id AND t.placement=s.placement" {
		t.Fatalf("join = %q", spec.join)
	}
	for _, removed := range []string{"subaccount", "campaign_name"} {
		if strings.Contains(spec.join, removed) || strings.Contains(spec.upsert, removed) {
			t.Fatalf("canonical note spec contains %q: join=%q upsert=%q", removed, spec.join, spec.upsert)
		}
	}
}

func TestMaituoReconcileSpecsSkipsTrendForOlderWorkbook(t *testing.T) {
	snapshot := maituo.Snapshot{PresentSheets: []string{maituo.SheetTrend}}
	if specs := maituoReconcileSpecs(snapshot, false); len(specs) != 0 {
		t.Fatalf("specs = %+v, want no trend reconciliation", specs)
	}
}
