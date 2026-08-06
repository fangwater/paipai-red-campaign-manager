package dandelion

import "time"

const (
	FieldNoteID            = "笔记ID"
	FieldTitle             = "笔记标题"
	FieldNoteURL           = "笔记链接"
	FieldAuthor            = "达人/发布账号"
	FieldPublishedAt       = "发布时间"
	FieldOrderingAccount   = "下单账号"
	FieldSPUName           = "spu名称"
	FieldDataUpdatedAt     = "数据更新日期"
	FieldNoteType          = "笔记类型"
	FieldContentTag        = "内容标签"
	FieldDandelionAmount   = "蒲公英金额"
	FieldOffsiteActiveCost = "站外活跃成本（15天设备归因）"
	FieldImpressions       = "曝光量"
	FieldReads             = "阅读量"
	FieldInteractions      = "互动量"
	FieldReadCost          = "阅读单价"
	FieldInteractionCost   = "互动单价"
)

type Record struct {
	RecordID    string
	SourceRow   int
	NoteID      string
	DataUpdated time.Time
	Fields      []byte
}

type Snapshot struct {
	FileName      string
	FileSHA256    string
	SheetName     string
	HeaderRow     int
	MatchedFields []string
	Records       []Record
}

type ImportResult struct {
	RunID       int64  `json:"run_id"`
	FileName    string `json:"file_name"`
	FileSHA256  string `json:"file_sha256"`
	SheetName   string `json:"sheet_name"`
	HeaderRow   int    `json:"header_row"`
	Fetched     int    `json:"fetched"`
	Inserted    int    `json:"inserted"`
	Updated     int    `json:"updated"`
	Unchanged   int    `json:"unchanged"`
	Deleted     int    `json:"deleted"`
	CompletedAt string `json:"completed_at,omitempty"`
}
