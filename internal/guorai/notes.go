package guorai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxDateRangeDays = 90
	BusinessTypeNote = "note"
	BusinessTypePlan = "plan"
)

type Brand struct {
	XHSBrandID       string `json:"xhsBrandId"`
	XHSBrandName     string `json:"xhsBrandName"`
	BrandName        string `json:"brandName"`
	BrandBindMerFlag bool   `json:"brandBindMerFlag"`
}

func (b Brand) Name() string {
	if b.XHSBrandName != "" {
		return b.XHSBrandName
	}
	return b.BrandName
}

type Rule struct {
	EndDate         string `json:"endDate"`
	TradeDataPeriod any    `json:"tradeDataPeriod"`
	EventType       string `json:"eventType"`
	EventModel      string `json:"eventModel"`
}

type NotesFilter struct {
	BusinessType string
	BeginDate    string
	EndDate      string
	BrandID      string
	MerchantID   string
	SortField    string
	SortOrder    string
	PageSize     int
	Limit        int
}

type ResolvedFilter struct {
	NotesFilter
	EnterpriseID int64  `json:"enterpriseId"`
	BrandName    string `json:"brandName"`
	RuleEndDate  string `json:"ruleEndDate"`
	Rule         Rule   `json:"rule"`
}

type NotesResult struct {
	Filter ResolvedFilter   `json:"filter"`
	Total  int              `json:"total"`
	Data   []map[string]any `json:"data"`
}

type ExportTask struct {
	TaskID     string `json:"taskId"`
	TaskName   string `json:"taskName"`
	Status     int    `json:"status"`
	CreateTime string `json:"createTime"`
}

type ExportRequest struct {
	Filter       NotesFilter
	NoWait       bool
	PollInterval time.Duration
	Timeout      time.Duration
	OutputPath   string
}

type ExportResult struct {
	Task       ExportTask
	OutputPath string
}

func (c *Client) QueryNotes(ctx context.Context, filter NotesFilter) (NotesResult, error) {
	resolved, err := c.ResolveFilter(ctx, filter)
	if err != nil {
		return NotesResult{}, err
	}
	return c.QueryResolved(ctx, resolved)
}

func (c *Client) QueryResolved(ctx context.Context, resolved ResolvedFilter) (NotesResult, error) {
	pageSize := resolved.PageSize
	if pageSize <= 0 {
		pageSize = 200
	}
	if pageSize > 500 {
		pageSize = 500
	}

	result := NotesResult{Filter: resolved, Data: make([]map[string]any, 0)}
	for page := 1; ; page++ {
		data, total, err := c.queryNotesPage(ctx, resolved, page, pageSize)
		if err != nil {
			return NotesResult{}, err
		}
		result.Total = total
		if resolved.Limit > 0 && len(result.Data)+len(data) > resolved.Limit {
			data = data[:resolved.Limit-len(result.Data)]
		}
		result.Data = append(result.Data, data...)
		if len(data) == 0 || len(result.Data) >= total || (resolved.Limit > 0 && len(result.Data) >= resolved.Limit) {
			break
		}
	}
	return result, nil
}

func (c *Client) ResolveFilter(ctx context.Context, filter NotesFilter) (ResolvedFilter, error) {
	businessType, err := normalizeBusinessType(filter.BusinessType)
	if err != nil {
		return ResolvedFilter{}, err
	}
	filter.BusinessType = businessType
	account, err := c.ValidateSession(ctx)
	if err != nil {
		return ResolvedFilter{}, err
	}
	brands, err := c.listBrands(ctx, account.EnterpriseID, businessType)
	if err != nil {
		return ResolvedFilter{}, err
	}
	brand, err := selectBrand(brands, filter.BrandID)
	if err != nil {
		return ResolvedFilter{}, err
	}
	rule, err := c.getRule(ctx, account.EnterpriseID, brand.XHSBrandID, filter.MerchantID, businessType)
	if err != nil {
		return ResolvedFilter{}, err
	}

	if filter.BeginDate == "" && filter.EndDate == "" {
		end, err := time.Parse(time.DateOnly, rule.EndDate)
		if err != nil {
			return ResolvedFilter{}, fmt.Errorf("parse server end date %q: %w", rule.EndDate, err)
		}
		filter.EndDate = end.Format(time.DateOnly)
		filter.BeginDate = end.AddDate(0, 0, -6).Format(time.DateOnly)
	} else if filter.BeginDate == "" || filter.EndDate == "" {
		return ResolvedFilter{}, errors.New("--from and --to must be provided together")
	}
	if err := validateDateRange(filter.BeginDate, filter.EndDate, rule.EndDate); err != nil {
		return ResolvedFilter{}, err
	}
	if filter.SortField == "" {
		filter.SortField = "totalPayAmt"
	}
	if filter.SortOrder == "" {
		filter.SortOrder = "DESC"
	}
	filter.SortOrder = strings.ToUpper(filter.SortOrder)
	if filter.SortOrder != "ASC" && filter.SortOrder != "DESC" {
		return ResolvedFilter{}, errors.New("sort order must be ASC or DESC")
	}
	filter.BrandID = brand.XHSBrandID
	return ResolvedFilter{
		NotesFilter: filter, EnterpriseID: account.EnterpriseID, BrandName: brand.Name(), RuleEndDate: rule.EndDate, Rule: rule,
	}, nil
}

func (c *Client) StartNotesExport(ctx context.Context, request ExportRequest) (ExportResult, error) {
	resolved, err := c.ResolveFilter(ctx, request.Filter)
	if err != nil {
		return ExportResult{}, err
	}
	config := businessConfigFor(resolved.BusinessType)
	taskName := fmt.Sprintf("%s_按触达时间_%s至%s_脚本%s", config.TaskLabel, resolved.BeginDate, resolved.EndDate, time.Now().Format("150405"))
	condition := c.followPayload(resolved)
	condition["dateType"] = 1
	condition["getDate"] = 1
	condition["beginDate"] = resolved.BeginDate
	condition["endDate"] = resolved.EndDate
	condition["exportTypes"] = []string{"summary"}
	condition["sortField"] = resolved.SortField
	condition["outType"] = config.ExportSummary + ",attentionType"
	condition["outTypeList"] = []string{config.ExportSummary, "attentionType"}
	if resolved.BusinessType == BusinessTypePlan {
		condition["planTypeList"] = []string{"plan"}
		condition["exportBizTypeList"] = []string{"plan"}
	}
	encodedCondition, err := json.Marshal(condition)
	if err != nil {
		return ExportResult{}, fmt.Errorf("encode export condition: %w", err)
	}

	start, err := c.request(ctx, http.MethodPost, c.endpoints.MediaBase+"/media-delivery-merchant/startExport", map[string]any{
		"enterpriseId": resolved.EnterpriseID,
		"taskName":     taskName, "excelBizType": config.ExportBizType, "isHandleVal": true, "desc": "",
		"condition": string(encodedCondition),
	}, config.FunctionKey, true)
	if err != nil {
		return ExportResult{}, fmt.Errorf("start export: %w", err)
	}
	if start.Retcode != 0 {
		return ExportResult{}, apiError("start export", start)
	}
	task := ExportTask{TaskName: taskName}
	var content map[string]any
	if json.Unmarshal(start.Content, &content) == nil {
		task.TaskID = valueString(content["taskId"])
	}
	if request.NoWait {
		return ExportResult{Task: task}, nil
	}

	pollInterval := request.PollInterval
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	task, err = c.waitForExport(waitCtx, resolved.EnterpriseID, task, pollInterval)
	if err != nil {
		return ExportResult{Task: task}, err
	}
	if request.OutputPath == "" {
		request.OutputPath = fmt.Sprintf("guorai-%s-%s-%s.xlsx", config.FilenamePrefix, resolved.BeginDate, resolved.EndDate)
	}
	if err := c.downloadExport(ctx, resolved.EnterpriseID, task, request.OutputPath); err != nil {
		return ExportResult{Task: task}, err
	}
	return ExportResult{Task: task, OutputPath: request.OutputPath}, nil
}

func (c *Client) listBrands(ctx context.Context, enterpriseID int64, businessType string) ([]Brand, error) {
	config := businessConfigFor(businessType)
	endpoint := c.endpoints.MediaBase + "/media-delivery-merchant/enterpriseBrand/queryPassEnterpriseBrand?enterpriseId=" + strconv.FormatInt(enterpriseID, 10)
	response, err := c.request(ctx, http.MethodGet, endpoint, nil, config.FunctionKey, true)
	if err != nil {
		return nil, err
	}
	if response.Retcode != 0 {
		return nil, apiError("list brands", response)
	}
	var brands []Brand
	if err := json.Unmarshal(response.Content, &brands); err != nil {
		return nil, fmt.Errorf("decode brands: %w", err)
	}
	return brands, nil
}

func (c *Client) getRule(ctx context.Context, enterpriseID int64, brandID, merchantID, businessType string) (Rule, error) {
	config := businessConfigFor(businessType)
	payload := map[string]any{"enterpriseId": enterpriseID, "xhsBrandId": brandID}
	if merchantID != "" {
		payload["merchantId"] = merchantID
	}
	response, err := c.request(ctx, http.MethodPost, c.endpoints.MainGateway+"/dbapi-access/bigdata/shuliangattention/getRule", payload, config.FunctionKey, true)
	if err != nil {
		return Rule{}, err
	}
	if response.Retcode != 0 {
		return Rule{}, apiError("get attribution rule", response)
	}
	var rule Rule
	if err := json.Unmarshal(response.Content, &rule); err != nil {
		return Rule{}, fmt.Errorf("decode attribution rule: %w", err)
	}
	if rule.EndDate == "" {
		return Rule{}, errors.New("attribution rule returned an empty endDate")
	}
	return rule, nil
}

func (c *Client) queryNotesPage(ctx context.Context, filter ResolvedFilter, pageNo, pageSize int) ([]map[string]any, int, error) {
	config := businessConfigFor(filter.BusinessType)
	payload := c.followPayload(filter)
	payload["pageNo"] = pageNo
	payload["pageSize"] = pageSize
	response, err := c.request(ctx, http.MethodPost, c.endpoints.MediaBase+config.QueryPath, payload, config.FunctionKey, true)
	if err != nil {
		return nil, 0, err
	}
	if response.Retcode != 0 {
		return nil, 0, apiError("query "+filter.BusinessType, response)
	}
	var content struct {
		Data       []map[string]any `json:"data"`
		TotalCount int              `json:"totalCount"`
		Total      int              `json:"total"`
	}
	decoder := json.NewDecoder(bytes.NewReader(response.Content))
	decoder.UseNumber()
	if err := decoder.Decode(&content); err != nil {
		return nil, 0, fmt.Errorf("decode notes: %w", err)
	}
	total := content.TotalCount
	if total == 0 {
		total = content.Total
	}
	return content.Data, total, nil
}

func (c *Client) followPayload(filter ResolvedFilter) map[string]any {
	config := businessConfigFor(filter.BusinessType)
	payload := map[string]any{
		"enterpriseId": filter.EnterpriseID, "xhsBrandId": filter.BrandID,
		"dateType": 1, "getDate": filter.BeginDate, "endDate": filter.EndDate,
		"isAd": config.IsAd, "sortColumn": filter.SortField, "sortOrder": filter.SortOrder,
	}
	if filter.MerchantID != "" {
		payload["merchantId"] = filter.MerchantID
	}
	return payload
}

func (c *Client) waitForExport(ctx context.Context, enterpriseID int64, target ExportTask, interval time.Duration) (ExportTask, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		tasks, err := c.listExportTasks(ctx, enterpriseID)
		if err != nil {
			return target, err
		}
		for _, task := range tasks {
			if (target.TaskID != "" && task.TaskID == target.TaskID) || task.TaskName == target.TaskName {
				target = task
				switch task.Status {
				case 2:
					return task, nil
				case 3:
					return task, errors.New("export task failed")
				}
				break
			}
		}
		select {
		case <-ctx.Done():
			return target, fmt.Errorf("wait for export: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (c *Client) listExportTasks(ctx context.Context, enterpriseID int64) ([]ExportTask, error) {
	response, err := c.request(ctx, http.MethodPost, c.endpoints.MediaBase+"/media-delivery-merchant/export/list", map[string]any{
		"enterpriseId": enterpriseID, "pageNo": 1, "pageSize": 100,
	}, functionKey, true)
	if err != nil {
		return nil, err
	}
	if response.Retcode != 0 {
		return nil, apiError("list export tasks", response)
	}
	var content struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Content, &content); err != nil {
		return nil, fmt.Errorf("decode export tasks: %w", err)
	}
	tasks := make([]ExportTask, 0, len(content.Data))
	for _, item := range content.Data {
		tasks = append(tasks, ExportTask{
			TaskID: valueString(item["taskId"]), TaskName: valueString(item["taskName"]),
			Status: valueInt(item["status"]), CreateTime: valueString(item["createTime"]),
		})
	}
	sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].CreateTime > tasks[j].CreateTime })
	return tasks, nil
}

func (c *Client) downloadExport(ctx context.Context, enterpriseID int64, task ExportTask, outputPath string) error {
	response, err := c.request(ctx, http.MethodPost, c.endpoints.MediaBase+"/media-delivery-merchant/export/url", map[string]any{
		"enterpriseId": enterpriseID, "taskId": task.TaskID, "taskName": task.TaskName,
	}, functionKey, true)
	if err != nil {
		return fmt.Errorf("get export URL: %w", err)
	}
	if response.Retcode != 0 {
		return apiError("get export URL", response)
	}
	var downloadURL string
	if err := json.Unmarshal(response.Content, &downloadURL); err != nil || downloadURL == "" {
		return errors.New("export URL was empty")
	}
	parsed, err := url.Parse(downloadURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return errors.New("export URL was invalid")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create export download request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download export: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download export: HTTP %d", resp.StatusCode)
	}
	return writeFileAtomic(outputPath, resp.Body)
}

func validateDateRange(beginDate, endDate, ruleEndDate string) error {
	begin, err := time.Parse(time.DateOnly, beginDate)
	if err != nil {
		return fmt.Errorf("parse --from %q: %w", beginDate, err)
	}
	end, err := time.Parse(time.DateOnly, endDate)
	if err != nil {
		return fmt.Errorf("parse --to %q: %w", endDate, err)
	}
	if begin.After(end) {
		return errors.New("--from cannot be later than --to")
	}
	days := int(end.Sub(begin).Hours()/24) + 1
	if days > maxDateRangeDays {
		return fmt.Errorf("date range is %d days; Guorai allows at most %d days", days, maxDateRangeDays)
	}
	if ruleEndDate != "" {
		latest, err := time.Parse(time.DateOnly, ruleEndDate)
		if err == nil && end.After(latest) {
			return fmt.Errorf("--to %s is later than the current statistics cutoff %s", endDate, ruleEndDate)
		}
	}
	return nil
}

type businessConfig struct {
	QueryPath      string
	FunctionKey    string
	ExportBizType  string
	ExportSummary  string
	TaskLabel      string
	FilenamePrefix string
	IsAd           int
}

func normalizeBusinessType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return BusinessTypeNote, nil
	}
	if value != BusinessTypeNote && value != BusinessTypePlan {
		return "", errors.New("--type must be note or plan")
	}
	return value, nil
}

func businessConfigFor(businessType string) businessConfig {
	if businessType == BusinessTypePlan {
		return businessConfig{
			QueryPath:      "/media-delivery-merchant/followList/getFollowPlanList",
			FunctionKey:    "MyFollowPlansList",
			ExportBizType:  "followPlanList",
			ExportSummary:  "followPlanSummary",
			TaskLabel:      "关注计划（我的关注）",
			FilenamePrefix: "plans",
			IsAd:           2,
		}
	}
	return businessConfig{
		QueryPath:      "/media-delivery-merchant/followList/getFollowNoteList",
		FunctionKey:    functionKey,
		ExportBizType:  "followNoteList",
		ExportSummary:  "followNoteSummary",
		TaskLabel:      "关注笔记（我的关注）",
		FilenamePrefix: "notes",
		IsAd:           1,
	}
}

func selectBrand(brands []Brand, requested string) (Brand, error) {
	if requested != "" {
		for _, brand := range brands {
			if brand.XHSBrandID == requested {
				return brand, nil
			}
		}
		return Brand{}, fmt.Errorf("brand %s was not found in this account", requested)
	}
	for _, brand := range brands {
		if brand.BrandBindMerFlag {
			return brand, nil
		}
	}
	if len(brands) > 0 {
		return brands[0], nil
	}
	return Brand{}, errors.New("no accessible XHS brands were found")
}

func valueString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func valueInt(value any) int {
	switch typed := value.(type) {
	case json.Number:
		result, _ := strconv.Atoi(typed.String())
		return result
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		result, _ := strconv.Atoi(typed)
		return result
	default:
		return 0
	}
}

func writeFileAtomic(path string, source io.Reader) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".guorai-export-*")
	if err != nil {
		return fmt.Errorf("create temporary export: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, source); err != nil {
		tmp.Close()
		return fmt.Errorf("write export: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close export: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace export: %w", err)
	}
	return nil
}
