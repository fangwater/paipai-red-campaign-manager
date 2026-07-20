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
