package model

type ContentAnalysisQuery struct {
	SPU                string
	Agency             string
	Dimension          string
	PublishedStartDate string
	PublishedEndDate   string
}

type ContentAnalysisSources struct {
	DandelionDataDate  string `json:"dandelion_data_date"`
	DandelionSyncedAt  string `json:"dandelion_synced_at"`
	MaituoReportDate   string `json:"maituo_report_date"`
	GuoraiSnapshotDate string `json:"guorai_snapshot_date"`
	GuoraiWindowStart  string `json:"guorai_window_start"`
	GuoraiWindowEnd    string `json:"guorai_window_end"`
	ManuscriptSyncedAt string `json:"manuscript_synced_at"`
}

type ContentAnalysisCoverage struct {
	TotalNotes         int `json:"total_notes"`
	ContentTypeTagged  int `json:"content_type_tagged"`
	AudienceTagged     int `json:"audience_tagged"`
	ScenarioTagged     int `json:"scenario_tagged"`
	DandelionCostNotes int `json:"dandelion_cost_notes"`
	FlowEvaluatedNotes int `json:"flow_evaluated_notes"`
	ROIEvaluatedNotes  int `json:"roi_evaluated_notes"`
	AllMetricsNotes    int `json:"all_metrics_notes"`
}

type ContentAnalysisNote struct {
	NoteID           string   `json:"note_id"`
	Title            string   `json:"title"`
	URL              string   `json:"url"`
	Author           string   `json:"author"`
	PublishedDate    string   `json:"published_date"`
	Agency           string   `json:"agency"`
	ContentType      string   `json:"content_type"`
	Audience         string   `json:"audience"`
	Scenario         string   `json:"scenario"`
	DandelionCost    *float64 `json:"dandelion_cost"`
	Boom             bool     `json:"boom"`
	SearchSpend      float64  `json:"search_spend"`
	SearchCost       *float64 `json:"search_cost"`
	LatestSearchCost *float64 `json:"latest_search_cost"`
	SearchCostChange *float64 `json:"search_cost_change"`
	SearchQualified  bool     `json:"search_qualified"`
	FeedSpend        float64  `json:"feed_spend"`
	FeedCost         *float64 `json:"feed_cost"`
	FeedQualified    bool     `json:"feed_qualified"`
	LatestSpend      float64  `json:"latest_spend"`
	Stopped          bool     `json:"stopped"`
	FlowEvaluated    bool     `json:"flow_evaluated"`
	FlowQualified    bool     `json:"flow_qualified"`
	ROI              *float64 `json:"roi"`
	ROIQualified     bool     `json:"roi_qualified"`
	AllQualified     bool     `json:"all_qualified"`
}

type ContentAnalysisCell struct {
	ContentType       string                `json:"content_type"`
	Dimension         string                `json:"dimension"`
	TotalNotes        int                   `json:"total_notes"`
	DandelionEligible int                   `json:"dandelion_eligible"`
	BoomCount         int                   `json:"boom_count"`
	BoomRate          *float64              `json:"boom_rate"`
	FlowEvaluated     int                   `json:"flow_evaluated"`
	FlowQualified     int                   `json:"flow_qualified"`
	ROIEvaluated      int                   `json:"roi_evaluated"`
	ROIQualified      int                   `json:"roi_qualified"`
	AllQualified      int                   `json:"all_qualified"`
	Notes             []ContentAnalysisNote `json:"notes"`
}

type ContentAnalysis struct {
	SPU                string                  `json:"spu"`
	Agency             string                  `json:"agency"`
	Dimension          string                  `json:"dimension"`
	PublishedStartDate string                  `json:"published_start_date"`
	PublishedEndDate   string                  `json:"published_end_date"`
	Sources            ContentAnalysisSources  `json:"sources"`
	Coverage           ContentAnalysisCoverage `json:"coverage"`
	Types              []string                `json:"types"`
	Dimensions         []string                `json:"dimensions"`
	Cells              []ContentAnalysisCell   `json:"cells"`
}
