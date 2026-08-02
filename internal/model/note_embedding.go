package model

type NoteEmbeddingSource struct {
	NoteID       string
	NoteContent  string
	ExistingHash string
}

type NoteEmbeddingRecord struct {
	NoteID      string
	ContentHash string
	Embedding   []float32
}

type NoteEmbeddingRefreshResult struct {
	RunID      int64  `json:"run_id"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
	Total      int    `json:"total"`
	Candidates int    `json:"candidates"`
	Embedded   int    `json:"embedded"`
	Skipped    int    `json:"skipped"`
	Failed     int    `json:"failed"`
	Requests   int    `json:"requests"`
	Tokens     int64  `json:"tokens"`
}

type SimilarProviderNote struct {
	NoteID      string  `json:"note_id"`
	NoteContent string  `json:"note_content"`
	Similarity  float64 `json:"similarity"`
}
