package delivery

import (
	"context"
	"time"
)

type AssetQuery struct {
	AdvertiserID int64
	Search       string
	Limit        int
}

type CandidateNote struct {
	NoteID          string   `json:"note_id"`
	Title           string   `json:"title"`
	Content         string   `json:"content,omitempty"`
	Audience        []string `json:"audience"`
	Scenarios       []string `json:"scenarios"`
	NoteTypes       []string `json:"note_types"`
	HistoricalSpend float64  `json:"historical_spend"`
	HistoricalUsers int64    `json:"historical_search_users"`
	HistoricalCost  *float64 `json:"historical_search_cost,omitempty"`
	Published       bool     `json:"published"`
	CreativityCount int      `json:"creativity_count"`
}

type Assets struct {
	AdvertiserID int64           `json:"advertiser_id"`
	Notes        []CandidateNote `json:"notes"`
	Count        int             `json:"count"`
	GeneratedAt  time.Time       `json:"generated_at"`
}

type APIAttempt struct {
	JobID             string
	AdvertiserID      int64
	Operation         string
	ContractVersion   string
	RequestHash       string
	RequestSummary    map[string]any
	ResponseSummary   map[string]any
	UpstreamRequestID string
	Success           bool
	ErrorCode         string
	ErrorMessage      string
	LatencyMS         int64
}

type Store interface {
	CreateDraft(context.Context, CreateDraftInput, Actor) (Draft, error)
	UpdateDraft(context.Context, string, UpdateDraftInput, Actor) (Draft, error)
	Draft(context.Context, string) (Draft, error)
	Drafts(context.Context, int64, int) ([]Draft, error)
	SaveRecommendation(context.Context, Recommendation) (Recommendation, error)
	LatestRecommendation(context.Context, string, int) (Recommendation, error)
	SaveValidation(context.Context, Validation) (Validation, error)
	LatestValidation(context.Context, string, int) (Validation, error)
	SaveApproval(context.Context, Approval) (Approval, error)
	Approvals(context.Context, string, int) ([]Approval, error)
	CreatePublishJob(context.Context, PublishJob) (PublishJob, error)
	PublishJobByIdempotency(context.Context, string) (PublishJob, error)
	PublishJob(context.Context, string) (PublishJob, error)
	PublishJobs(context.Context, string, int, int) ([]PublishJob, error)
	ClaimPublishJob(context.Context) (PublishJob, bool, error)
	UpdatePublishJob(context.Context, PublishJob) error
	SaveMediaEntity(context.Context, MediaEntity) (MediaEntity, error)
	MediaEntity(context.Context, int64, string, int64) (MediaEntity, error)
	MediaEntities(context.Context, string) ([]MediaEntity, error)
	UpdateMediaEntityStatus(context.Context, string, string) error
	SaveAPIAttempt(context.Context, APIAttempt) error
	SavePerformanceSnapshot(context.Context, PerformanceQuery, map[string]any, string) error
	Assets(context.Context, AssetQuery) (Assets, error)
	RecommendationCandidates(context.Context, []string) ([]CandidateNote, error)
	Audit(context.Context, Actor, string, string, string, int64, map[string]any) error
}
