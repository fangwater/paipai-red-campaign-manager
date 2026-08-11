package model

import "time"

type OverviewMetricPoint struct {
	Date  string   `json:"date"`
	Value *float64 `json:"value"`
}

type OverviewMetric struct {
	Key           string                `json:"key"`
	Label         string                `json:"label"`
	Unit          string                `json:"unit"`
	CurrentValue  *float64              `json:"current_value"`
	PreviousValue *float64              `json:"previous_value"`
	ChangePct     *float64              `json:"change_pct"`
	Points        []OverviewMetricPoint `json:"points"`
}

type OverviewTrend struct {
	StartDate         string           `json:"start_date"`
	EndDate           string           `json:"end_date"`
	PreviousStartDate string           `json:"previous_start_date"`
	PreviousEndDate   string           `json:"previous_end_date"`
	AvailableDays     int              `json:"available_days"`
	Metrics           []OverviewMetric `json:"metrics"`
}

type OverviewDailyNotes struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type OverviewNote struct {
	NoteID        string `json:"note_id"`
	Title         string `json:"title"`
	URL           string `json:"url"`
	Author        string `json:"author"`
	PublishedDate string `json:"published_date"`
	Agency        string `json:"agency"`
	Audience      string `json:"audience"`
	NoteType      string `json:"note_type"`
	ContentTag    string `json:"content_tag"`
}

type OverviewAgency struct {
	Agency       string         `json:"agency"`
	Count        int            `json:"count"`
	AudienceTags []string       `json:"audience_tags"`
	Notes        []OverviewNote `json:"notes"`
}

type OverviewNewNotes struct {
	StartDate      string               `json:"start_date"`
	EndDate        string               `json:"end_date"`
	Total          int                  `json:"total"`
	Daily          []OverviewDailyNotes `json:"daily"`
	Agencies       []OverviewAgency     `json:"agencies"`
	SourceSyncedAt *time.Time           `json:"source_synced_at"`
}

type SearchUserPlacementCoefficient struct {
	Placement             string   `json:"placement"`
	SearchUsers           int64    `json:"search_users"`
	NoteSearchUsers       int64    `json:"note_search_users"`
	SubaccountSearchUsers int64    `json:"subaccount_search_users"`
	SPUSearchUsers        int64    `json:"spu_search_users"`
	Coefficient           *float64 `json:"coefficient"`
	NoteSPUCoefficient    *float64 `json:"note_spu_coefficient"`
}

type SearchUserOverlapPoint struct {
	ReportDate              string                           `json:"report_date"`
	SPUSearchUsers          *int64                           `json:"spu_search_users"`
	SubaccountSearchUsers   *int64                           `json:"subaccount_search_users"`
	OverlapUsers            *int64                           `json:"overlap_users"`
	OverlapCoefficient      *float64                         `json:"overlap_coefficient"`
	DeduplicationFactor     *float64                         `json:"deduplication_factor"`
	NoteSearchUsers         *int64                           `json:"note_search_users"`
	NoteOverlapUsers        *int64                           `json:"note_overlap_users"`
	NoteOverlapCoefficient  *float64                         `json:"note_overlap_coefficient"`
	NoteDeduplicationFactor *float64                         `json:"note_deduplication_factor"`
	PlacementCoefficients   []SearchUserPlacementCoefficient `json:"placement_coefficients"`
}

type BusinessOverview struct {
	Days          int                      `json:"days"`
	SPU           string                   `json:"spu"`
	OverlapPoints []SearchUserOverlapPoint `json:"overlap_points"`
	Trend         OverviewTrend            `json:"trend"`
	NewNotes      OverviewNewNotes         `json:"new_notes"`
}
