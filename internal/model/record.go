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
	Label       string
	Provider    string
	ResourceKey string
	SourceURL   string
}

type ManuscriptBlock struct {
	Type             string   `json:"type"`
	Text             string   `json:"text,omitempty"`
	Level            int      `json:"level,omitempty"`
	AssetID          string   `json:"asset_id,omitempty"`
	Width            int      `json:"width,omitempty"`
	Height           int      `json:"height,omitempty"`
	Caption          string   `json:"caption,omitempty"`
	SourceToken      string   `json:"-"`
	ReferenceNoteIDs []string `json:"-"`
}

type ManuscriptAsset struct {
	AssetID     string
	ContentType string
	ByteSize    int64
	Width       int
	Height      int
	Content     []byte
}

type Document struct {
	Provider         string
	ResourceKey      string
	SourceURL        string
	DocumentType     string
	Title            string
	Content          string
	RevisionID       int
	Status           string
	ErrorMessage     string
	Blocks           []ManuscriptBlock
	ReferenceNoteIDs []string
	Assets           []ManuscriptAsset
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
