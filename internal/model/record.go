package model

import "time"

type Record struct {
	ID        string
	Fields    []byte
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

type Table struct {
	ID       string
	Name     string
	Revision int
	Records  []Record
}

type DocumentRef struct {
	TableID     string
	RecordID    string
	FieldName   string
	Provider    string
	ResourceKey string
	SourceURL   string
}

type Document struct {
	Provider     string
	ResourceKey  string
	SourceURL    string
	DocumentType string
	Title        string
	Content      string
	RevisionID   int
	Status       string
	ErrorMessage string
}

type Snapshot struct {
	Tables       []Table
	DocumentRefs []DocumentRef
}

type SyncResult struct {
	Tables         int   `json:"tables"`
	Fetched        int   `json:"fetched"`
	Upserted       int   `json:"upserted"`
	Deleted        int64 `json:"deleted"`
	Documents      int   `json:"documents"`
	DocumentErrors int   `json:"document_errors"`
}
