package store

import (
	"paipai-red-campaign-manager/internal/maituo"
)

func maituoReconcileSpecs(snapshot maituo.Snapshot, applyTrend bool) []maituoReconcileSpec {
	result := make([]maituoReconcileSpec, 0, len(snapshot.PresentSheets))
	for _, sheet := range snapshot.PresentSheets {
		switch sheet {
		case maituo.SheetKPI:
			result = append(result, maituoReconcileSpec{
				key: "kpis", name: sheet, target: "maituo_customer_daily_kpis", stage: "maituo_stage_kpis",
				presence: "t.metric", join: "t.report_date=s.report_date AND t.metric=s.metric",
				deleteScope: "t.report_date=$2", upsert: maituoKPIUpsert,
			})
		case maituo.SheetNotes:
			result = append(result, maituoReconcileSpec{
				key: "notes", name: sheet, target: "maituo_customer_daily_notes", stage: "maituo_stage_notes",
				presence: "t.note_id", join: "t.report_date=s.report_date AND t.note_id=s.note_id AND t.subaccount=s.subaccount AND t.campaign_name=s.campaign_name AND t.placement=s.placement",
				deleteScope: "t.report_date=$2", upsert: maituoNoteUpsert,
			})
		case maituo.SheetSPU:
			result = append(result, maituoReconcileSpec{
				key: "spus", name: sheet, target: "maituo_customer_daily_spus", stage: "maituo_stage_spus",
				presence: "t.spu", join: "t.report_date=s.report_date AND t.spu=s.spu",
				deleteScope: "t.report_date=$2", upsert: maituoSPUUpsert,
			})
		case maituo.SheetSubaccount:
			result = append(result, maituoReconcileSpec{
				key: "subaccounts", name: sheet, target: "maituo_customer_daily_subaccounts", stage: "maituo_stage_subaccounts",
				presence: "t.spu", join: "t.report_date=s.report_date AND t.spu=s.spu AND t.subaccount=s.subaccount AND t.placement=s.placement",
				deleteScope: "t.report_date=$2", upsert: maituoSubaccountUpsert,
			})
		case maituo.SheetTrend:
			if applyTrend {
				result = append(result, maituoReconcileSpec{
					key: "trends", name: sheet, target: "maituo_customer_daily_trends", stage: "maituo_stage_trends",
					presence: "t.report_date", join: "t.report_date=s.report_date", upsert: maituoTrendUpsert,
				})
			}
		}
	}
	return result
}
