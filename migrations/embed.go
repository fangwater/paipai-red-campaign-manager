package migrations

import _ "embed"

// InitSQL contains the idempotent PostgreSQL schema migration.
//
//go:embed 001_init.sql
var InitSQL string

// ProviderContentSQL contains the service-provider content sync schema.
//
//go:embed 002_provider_content.sql
var ProviderContentSQL string

// GuoraiSQL contains the Guorai rolling-snapshot schema.
//
//go:embed 003_guorai.sql
var GuoraiSQL string

// XHSJGCampaignsSQL contains the XHS Spotlight campaign snapshot schema.
//
//go:embed 004_xhs_jg_campaigns.sql
var XHSJGCampaignsSQL string

// XHSJGDeliverySQL contains the XHS Spotlight unit and creativity snapshot schema.
//
//go:embed 005_xhs_jg_units_creativities.sql
var XHSJGDeliverySQL string

// XHSJGManualSyncSQL contains the XHS Spotlight manual sync run schema.
//
//go:embed 006_xhs_jg_manual_sync.sql
var XHSJGManualSyncSQL string

// MaituoCustomerDailySQL contains the fixed Maituo workbook schema.
//
//go:embed 008_maituo_customer_daily.sql
var MaituoCustomerDailySQL string

// MaituoNoteReportDatesSQL adds report-date history for note details.
//
//go:embed 009_maituo_note_report_dates.sql
var MaituoNoteReportDatesSQL string

// MaituoPartialWorkbooksSQL records the known sheets found in each workbook.
//
//go:embed 010_maituo_partial_workbooks.sql
var MaituoPartialWorkbooksSQL string

// MaituoDatedSummaryTablesSQL adds report-date history to the first four workbook tables.
//
//go:embed 011_maituo_dated_summary_tables.sql
var MaituoDatedSummaryTablesSQL string

// MaituoRemoveImportVersionSQL removes the obsolete import-version marker.
//
//go:embed 012_remove_maituo_import_version.sql
var MaituoRemoveImportVersionSQL string

// SimilarNoteEmbeddingsSQL stores external content embeddings and explicit refresh runs.
//
//go:embed 013_similar_note_embeddings.sql
var SimilarNoteEmbeddingsSQL string

// GuoraiCredentialsSQL stores the account used to renew an expired Guorai session.
//
//go:embed 014_guorai_credentials.sql
var GuoraiCredentialsSQL string

// DandelionExcelHistorySQL stores dated Dandelion Excel upload runs.
//
//go:embed 015_dandelion_excel_history.sql
var DandelionExcelHistorySQL string

// ManuscriptAssetsSQL stores ordered manuscript blocks and deduplicated image assets.
//
//go:embed 016_manuscript_assets.sql
var ManuscriptAssetsSQL string

// MaituoSearchUserOverlapSQL calculates daily attribution overlap against SPU totals.
//
//go:embed 017_maituo_search_user_overlap.sql
var MaituoSearchUserOverlapSQL string

// ReferenceNoteIDsSQL stores valid note IDs referenced by final manuscripts.
//
//go:embed 019_reference_note_ids.sql
var ReferenceNoteIDsSQL string

// ReferenceMaterialContentsSQL stores manually maintained reference-note content.
//
//go:embed 020_reference_material_contents.sql
var ReferenceMaterialContentsSQL string

// ProviderNoteSourceTitleSQL stores the title of each source manuscript document.
//
//go:embed 021_provider_note_source_title.sql
var ProviderNoteSourceTitleSQL string
