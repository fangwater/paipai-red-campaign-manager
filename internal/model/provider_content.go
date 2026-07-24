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
	LastSyncStatus   string
	LastSyncError    string
}

type ProviderNoteExecution struct {
	RecordKey           string
	SourceRowNumber     int
	SubmissionDate      string
	NoteID              string
	CoverType           string
	CommercialIntensity string
	Audience            string
	UserScenario        string
	NoteType            string
	Progress            string
}

type ProviderNote struct {
	NoteID      string
	NoteContent string
}

type ProviderContentSnapshot struct {
	Table      ProviderContentTable
	Records    []ProviderNoteExecution
	NoteRefs   []DocumentRef
	Notes      []ProviderNote
	NoteErrors int
}

type ProviderSyncResult struct {
	Providers  int   `json:"providers"`
	Fetched    int   `json:"fetched"`
	Upserted   int   `json:"upserted"`
	Deleted    int64 `json:"deleted"`
	Notes      int   `json:"notes"`
	NoteErrors int   `json:"note_errors"`
}
