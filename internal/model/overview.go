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

type BusinessOverview struct {
	Days     int              `json:"days"`
	SPU      string           `json:"spu"`
	Trend    OverviewTrend    `json:"trend"`
	NewNotes OverviewNewNotes `json:"new_notes"`
}
