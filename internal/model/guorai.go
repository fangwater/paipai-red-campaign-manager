package model

import "time"

// GuoraiSnapshot is one complete note or plan query window ready for PostgreSQL storage.
type GuoraiSnapshot struct {
	EntityType            string
	EnterpriseID          int64
	XHSBrandID            string
	BrandName             string
	MerchantID            string
	AttributionShop       string
	WindowStart           time.Time
	WindowEnd             time.Time
	SnapshotDate          time.Time
	SourceCutoffDate      time.Time
	AttributionType       string
	AttributionModel      string
	AttributionWindowDays int
	TrafficType           string
	RequestPayload        []byte
	RawResponse           []byte
	Records               []byte
}

type GuoraiStoreResult struct {
	FetchID int64
	Rows    int
}
