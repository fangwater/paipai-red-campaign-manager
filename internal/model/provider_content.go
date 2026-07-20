package model

import "time"

type ProviderContentTable struct {
	ProviderCode     string
	ProviderName     string
	SourceURL        string
	WikiToken        string
	SpreadsheetToken string
	SheetID          string
	SheetName        string
	LastSyncedAt     *time.Time
}

type ProviderNoteExecution struct {
	RecordKey           string
	SourceRowNumber     int
	SubmissionDate      string
	NoteID              string
	ContentType         string
	CoverType           string
	CommercialIntensity string
	Audience            string
	UserScenario        string
	NoteType            string
	Progress            string
	ReviewFeedback      string
}

type ProviderContentSnapshot struct {
	Table   ProviderContentTable
	Records []ProviderNoteExecution
}

type ProviderSyncResult struct {
	Providers int
	Fetched   int
	Upserted  int
	Deleted   int64
}
