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
