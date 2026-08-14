package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
)

type SemanticAdvice struct {
	Themes             []string             `json:"themes"`
	KeywordSeeds       []string             `json:"keyword_seeds"`
	NegativeKeywords   []string             `json:"negative_keywords"`
	AudienceHypotheses []AudienceHypothesis `json:"audience_hypotheses"`
	NoteEvidence       []NoteEvidence       `json:"note_evidence"`
	Uncertainties      []string             `json:"uncertainties"`
	Provider           string               `json:"provider"`
	Model              string               `json:"model"`
	Fallback           bool                 `json:"fallback"`
	PromptVersion      string               `json:"prompt_version"`
}

type AudienceHypothesis struct {
	Age        string   `json:"age"`
	Gender     string   `json:"gender,omitempty"`
	Regions    []string `json:"regions,omitempty"`
	Interests  []string `json:"interests,omitempty"`
	Hypothesis string   `json:"hypothesis"`
	Evidence   []string `json:"evidence"`
}

type NoteEvidence struct {
	NoteID   string   `json:"note_id"`
	Themes   []string `json:"themes"`
	Evidence []string `json:"evidence"`
	Risks    []string `json:"risks"`
}

type SemanticRequest struct {
	Objective  string          `json:"objective"`
	Placement  string          `json:"placement"`
	Candidates []CandidateNote `json:"candidates"`
}

type SemanticAdvisor interface {
	Advise(context.Context, SemanticRequest) (SemanticAdvice, error)
	Metadata() map[string]any
}

type RuleSemanticAdvisor struct{}

func (RuleSemanticAdvisor) Metadata() map[string]any {
	return map[string]any{"provider": "local", "model": "rules-semantic/v1", "configured": true, "fallback": true}
}

func (RuleSemanticAdvisor) Advise(_ context.Context, request SemanticRequest) (SemanticAdvice, error) {
	result := SemanticAdvice{
		Themes: []string{}, KeywordSeeds: []string{}, NegativeKeywords: []string{},
		AudienceHypotheses: []AudienceHypothesis{}, NoteEvidence: []NoteEvidence{},
		Uncertainties: []string{"LLM 未配置或调用失败，仅使用稿件标签和确定性词频基线"},
		Provider:      "local", Model: "rules-semantic/v1", Fallback: true,
		PromptVersion: "delivery-semantic/2026-08-13",
	}
	themeCounts := map[string]int{}
	termCounts := map[string]int{}
	for _, candidate := range request.Candidates {
		evidence := NoteEvidence{NoteID: candidate.NoteID, Themes: []string{}, Evidence: []string{}, Risks: []string{}}
		for _, value := range append(append([]string{}, candidate.NoteTypes...), candidate.Scenarios...) {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			themeCounts[value]++
			evidence.Themes = appendUnique(evidence.Themes, value)
		}
		for _, value := range candidate.Audience {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			evidence.Evidence = appendUnique(evidence.Evidence, "稿件受众标签: "+value)
		}
		for _, term := range extractDeterministicTerms(candidate.Title+" "+candidate.Content, 40) {
			termCounts[term]++
		}
		if candidate.HistoricalUsers > 0 {
			evidence.Evidence = append(evidence.Evidence, fmt.Sprintf("历史搜索用户 %d", candidate.HistoricalUsers))
		} else {
			evidence.Risks = append(evidence.Risks, "缺少有效转化样本")
		}
		result.NoteEvidence = append(result.NoteEvidence, evidence)
	}
	result.Themes = topStringCounts(themeCounts, 8)
	result.KeywordSeeds = topStringCounts(termCounts, 20)
	ageSet := map[string]struct{}{}
	for _, candidate := range request.Candidates {
		for _, audience := range candidate.Audience {
			for _, age := range inferAgeLabels(audience) {
				ageSet[age] = struct{}{}
			}
		}
	}
	for _, age := range []string{"18-22", "23-27", "28-32", "32-100"} {
		if _, ok := ageSet[age]; ok {
			result.AudienceHypotheses = append(result.AudienceHypotheses, AudienceHypothesis{
				Age: age, Hypothesis: "由稿件既有受众标签映射，需通过平台人群预估和分层实验验证",
				Evidence: []string{"稿件受众标签"},
			})
		}
	}
	return result, nil
}

type OpenAICompatibleAdvisor struct {
	apiKey     string
	endpoint   string
	model      string
	httpClient *http.Client
}

func NewOpenAICompatibleAdvisor(apiKey, baseURL, model string, httpClient *http.Client) (*OpenAICompatibleAdvisor, error) {
	apiKey, baseURL, model = strings.TrimSpace(apiKey), strings.TrimSpace(baseURL), strings.TrimSpace(model)
	if apiKey == "" || baseURL == "" || model == "" {
		return nil, errors.New("LLM api key, base URL, and model are required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("LLM base URL is invalid")
	}
	if parsed.Scheme != "https" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" {
		return nil, errors.New("LLM base URL must use HTTPS")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}
	return &OpenAICompatibleAdvisor{
		apiKey: apiKey, endpoint: strings.TrimRight(baseURL, "/") + "/chat/completions",
		model: model, httpClient: httpClient,
	}, nil
}

func (advisor *OpenAICompatibleAdvisor) Metadata() map[string]any {
	return map[string]any{"provider": "openai-compatible", "model": advisor.model, "configured": true, "fallback": false}
}

func (advisor *OpenAICompatibleAdvisor) Advise(ctx context.Context, request SemanticRequest) (SemanticAdvice, error) {
	if len(request.Candidates) == 0 || len(request.Candidates) > 100 {
		return SemanticAdvice{}, errors.New("semantic candidates must contain between 1 and 100 notes")
	}
	type safeCandidate struct {
		NoteID    string   `json:"note_id"`
		Title     string   `json:"title"`
		Content   string   `json:"content"`
		Audience  []string `json:"audience"`
		Scenarios []string `json:"scenarios"`
		NoteTypes []string `json:"note_types"`
	}
	safe := make([]safeCandidate, 0, len(request.Candidates))
	for _, candidate := range request.Candidates {
		safe = append(safe, safeCandidate{
			NoteID: candidate.NoteID, Title: limitRunes(candidate.Title, 160),
			Content: limitRunes(candidate.Content, 1600), Audience: candidate.Audience,
			Scenarios: candidate.Scenarios, NoteTypes: candidate.NoteTypes,
		})
	}
	input, err := json.Marshal(map[string]any{
		"objective": request.Objective, "placement": request.Placement, "candidates": safe,
	})
	if err != nil {
		return SemanticAdvice{}, err
	}
	system := `你是投放语义分析器。只做候选提取与证据归纳，不决定预算、出价、启停，也不得编造平台枚举或受众规模。只返回 JSON，字段必须是 themes、keyword_seeds、negative_keywords、audience_hypotheses、note_evidence、uncertainties。年龄只能使用 18-22、23-27、28-32、32-100、all；每条建议必须引用输入证据，证据不足写入 uncertainties。`
	payload := map[string]any{
		"model":           advisor.model,
		"temperature":     0.1,
		"response_format": map[string]string{"type": "json_object"},
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": string(input)},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return SemanticAdvice{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, advisor.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return SemanticAdvice{}, fmt.Errorf("create LLM request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+advisor.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := advisor.httpClient.Do(httpRequest)
	if err != nil {
		return SemanticAdvice{}, fmt.Errorf("call LLM: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20+1))
	if err != nil {
		return SemanticAdvice{}, fmt.Errorf("read LLM response: %w", err)
	}
	if len(data) > 2<<20 {
		return SemanticAdvice{}, errors.New("LLM response exceeds size limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return SemanticAdvice{}, fmt.Errorf("LLM returned HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || len(envelope.Choices) == 0 {
		return SemanticAdvice{}, errors.New("LLM returned an invalid completion envelope")
	}
	var result SemanticAdvice
	resultDecoder := json.NewDecoder(strings.NewReader(envelope.Choices[0].Message.Content))
	resultDecoder.DisallowUnknownFields()
	if err := resultDecoder.Decode(&result); err != nil {
		return SemanticAdvice{}, fmt.Errorf("LLM did not return valid recommendation JSON: %w", err)
	}
	var trailing any
	if err := resultDecoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return SemanticAdvice{}, errors.New("LLM recommendation must contain exactly one JSON object")
	}
	if err := validateSemanticAdvice(result, request.Candidates); err != nil {
		return SemanticAdvice{}, err
	}
	result.Provider = "openai-compatible"
	result.Model = advisor.model
	result.Fallback = false
	result.PromptVersion = "delivery-semantic/2026-08-13"
	return result, nil
}

type FallbackAdvisor struct {
	Primary  SemanticAdvisor
	Fallback SemanticAdvisor
}

func (advisor FallbackAdvisor) Metadata() map[string]any {
	return map[string]any{"primary": advisor.Primary.Metadata(), "fallback": advisor.Fallback.Metadata()}
}

func (advisor FallbackAdvisor) Advise(ctx context.Context, request SemanticRequest) (SemanticAdvice, error) {
	result, err := advisor.Primary.Advise(ctx, request)
	if err == nil {
		return result, nil
	}
	fallback, fallbackErr := advisor.Fallback.Advise(ctx, request)
	if fallbackErr != nil {
		return SemanticAdvice{}, errors.Join(err, fallbackErr)
	}
	fallback.Uncertainties = append(fallback.Uncertainties, "LLM 调用失败: "+limitRunes(err.Error(), 240))
	return fallback, nil
}

type RankFeatures struct {
	NoteID          string  `json:"note_id"`
	HistoricalSpend float64 `json:"historical_spend"`
	SearchUsers     int64   `json:"search_users"`
	SearchCost      float64 `json:"search_cost"`
	HasSearchCost   bool    `json:"has_search_cost"`
	AudienceTags    int     `json:"audience_tags"`
	ScenarioTags    int     `json:"scenario_tags"`
	ContentLength   int     `json:"content_length"`
}

type RankedNote struct {
	NoteID      string   `json:"note_id"`
	Score       float64  `json:"score"`
	Rank        int      `json:"rank"`
	Evidence    []string `json:"evidence"`
	Uncertainty float64  `json:"uncertainty"`
}

type RankRequest struct {
	Objective string         `json:"objective"`
	Items     []RankFeatures `json:"items"`
}

type RankResult struct {
	Items    []RankedNote `json:"items"`
	Family   string       `json:"family"`
	Version  string       `json:"version"`
	Fallback bool         `json:"fallback"`
	Warnings []string     `json:"warnings"`
}

type Ranker interface {
	Rank(context.Context, RankRequest) (RankResult, error)
	Metadata() map[string]any
}

type HeuristicRanker struct{}

func (HeuristicRanker) Metadata() map[string]any {
	return map[string]any{"family": "deterministic-baseline", "version": "note-ranker/v1", "configured": true, "fallback": true}
}

func (HeuristicRanker) Rank(_ context.Context, request RankRequest) (RankResult, error) {
	if len(request.Items) == 0 {
		return RankResult{}, errors.New("rank request has no items")
	}
	maxUsers, maxSpend, maxLength := int64(1), 1.0, 1
	for _, item := range request.Items {
		maxUsers = maxInt64(maxUsers, item.SearchUsers)
		maxSpend = math.Max(maxSpend, item.HistoricalSpend)
		if item.ContentLength > maxLength {
			maxLength = item.ContentLength
		}
	}
	result := RankResult{Family: "deterministic-baseline", Version: "note-ranker/v1", Fallback: true, Items: make([]RankedNote, 0, len(request.Items)), Warnings: []string{"LightGBM/LambdaMART 未配置或不可用，使用可解释基线排序"}}
	for _, item := range request.Items {
		users := math.Log1p(float64(item.SearchUsers)) / math.Log1p(float64(maxUsers))
		spend := math.Log1p(item.HistoricalSpend) / math.Log1p(maxSpend)
		quality := math.Min(1, float64(item.AudienceTags+item.ScenarioTags)/4)
		content := math.Min(1, float64(item.ContentLength)/float64(maxLength))
		costScore := 0.25
		uncertainty := 0.7
		if item.HasSearchCost && item.SearchCost > 0 {
			costScore = 1 / (1 + item.SearchCost/50)
			uncertainty = 1 / math.Sqrt(float64(maxInt64(item.SearchUsers, 1)))
		}
		score := 0.4*users + 0.2*costScore + 0.15*quality + 0.1*content + 0.15*spend
		result.Items = append(result.Items, RankedNote{
			NoteID: item.NoteID, Score: roundScore(score), Uncertainty: roundScore(math.Min(1, uncertainty)),
			Evidence: []string{"历史搜索用户", "历史搜索成本", "稿件标签完整度", "内容完整度"},
		})
	}
	sortRankedNotes(result.Items)
	return result, nil
}

type RemoteRanker struct {
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewRemoteRanker(endpoint, apiKey, model string, httpClient *http.Client) (*RemoteRanker, error) {
	endpoint, model = strings.TrimSpace(endpoint), strings.TrimSpace(model)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || model == "" {
		return nil, errors.New("ranker endpoint and model are required")
	}
	if parsed.Scheme != "https" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" {
		return nil, errors.New("ranker endpoint must use HTTPS")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &RemoteRanker{endpoint: endpoint, apiKey: strings.TrimSpace(apiKey), model: model, httpClient: httpClient}, nil
}

func (ranker *RemoteRanker) Metadata() map[string]any {
	return map[string]any{"family": "lightgbm-lambdamart", "model": ranker.model, "configured": true, "fallback": false}
}

func (ranker *RemoteRanker) Rank(ctx context.Context, request RankRequest) (RankResult, error) {
	payload := map[string]any{"schema_version": "delivery-rank-features/v1", "model": ranker.model, "objective": request.Objective, "items": request.Items}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return RankResult{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, ranker.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return RankResult{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if ranker.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+ranker.apiKey)
	}
	response, err := ranker.httpClient.Do(httpRequest)
	if err != nil {
		return RankResult{}, fmt.Errorf("call LightGBM/LambdaMART ranker: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	if err != nil {
		return RankResult{}, err
	}
	if len(data) > 1<<20 || response.StatusCode < 200 || response.StatusCode >= 300 {
		return RankResult{}, fmt.Errorf("ranker returned HTTP %d or an oversized response", response.StatusCode)
	}
	var result RankResult
	resultDecoder := json.NewDecoder(bytes.NewReader(data))
	resultDecoder.DisallowUnknownFields()
	if err := resultDecoder.Decode(&result); err != nil {
		return RankResult{}, fmt.Errorf("decode ranker response: %w", err)
	}
	var trailing any
	if err := resultDecoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return RankResult{}, errors.New("ranker response must contain exactly one JSON object")
	}
	if result.Family != "lightgbm" && result.Family != "lambdamart" && result.Family != "lightgbm-lambdamart" {
		return RankResult{}, errors.New("ranker response family is not LightGBM/LambdaMART")
	}
	if result.Version == "" || len(result.Items) != len(request.Items) {
		return RankResult{}, errors.New("ranker response is incomplete")
	}
	expected := map[string]struct{}{}
	for _, item := range request.Items {
		expected[item.NoteID] = struct{}{}
	}
	for _, item := range result.Items {
		if _, ok := expected[item.NoteID]; !ok || math.IsNaN(item.Score) || math.IsInf(item.Score, 0) {
			return RankResult{}, errors.New("ranker response contains an unknown note or invalid score")
		}
		delete(expected, item.NoteID)
	}
	if len(expected) != 0 {
		return RankResult{}, errors.New("ranker response omitted candidates")
	}
	result.Fallback = false
	sortRankedNotes(result.Items)
	return result, nil
}

type FallbackRanker struct {
	Primary  Ranker
	Fallback Ranker
}

func (ranker FallbackRanker) Metadata() map[string]any {
	return map[string]any{"primary": ranker.Primary.Metadata(), "fallback": ranker.Fallback.Metadata()}
}

func (ranker FallbackRanker) Rank(ctx context.Context, request RankRequest) (RankResult, error) {
	result, err := ranker.Primary.Rank(ctx, request)
	if err == nil {
		return result, nil
	}
	fallback, fallbackErr := ranker.Fallback.Rank(ctx, request)
	if fallbackErr != nil {
		return RankResult{}, errors.Join(err, fallbackErr)
	}
	fallback.Warnings = append(fallback.Warnings, "LightGBM/LambdaMART 调用失败: "+limitRunes(err.Error(), 240))
	return fallback, nil
}

func BuildRankFeatures(candidates []CandidateNote) []RankFeatures {
	result := make([]RankFeatures, 0, len(candidates))
	for _, candidate := range candidates {
		features := RankFeatures{
			NoteID: candidate.NoteID, HistoricalSpend: candidate.HistoricalSpend,
			SearchUsers: candidate.HistoricalUsers, AudienceTags: len(candidate.Audience),
			ScenarioTags: len(candidate.Scenarios), ContentLength: len([]rune(candidate.Content)),
		}
		if candidate.HistoricalCost != nil {
			features.HasSearchCost = true
			features.SearchCost = *candidate.HistoricalCost
		}
		result = append(result, features)
	}
	return result
}

func validateSemanticAdvice(value SemanticAdvice, candidates []CandidateNote) error {
	if len(value.KeywordSeeds) > 100 || len(value.AudienceHypotheses) > 20 || len(value.NoteEvidence) > len(candidates) {
		return errors.New("LLM recommendation exceeds schema limits")
	}
	notes := map[string]struct{}{}
	for _, candidate := range candidates {
		notes[candidate.NoteID] = struct{}{}
	}
	for _, evidence := range value.NoteEvidence {
		if _, ok := notes[evidence.NoteID]; !ok {
			return errors.New("LLM recommendation cites an unknown note")
		}
	}
	for _, hypothesis := range value.AudienceHypotheses {
		if hypothesis.Age != "" && hypothesis.Age != "all" && !oneOfString(hypothesis.Age, "18-22", "23-27", "28-32", "32-100") {
			return errors.New("LLM recommendation invented an age enum")
		}
	}
	return nil
}

func sortRankedNotes(items []RankedNote) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].NoteID < items[j].NoteID
		}
		return items[i].Score > items[j].Score
	})
	for index := range items {
		items[index].Rank = index + 1
	}
}

func extractDeterministicTerms(value string, limit int) []string {
	counts := map[string]int{}
	var current []rune
	flush := func() {
		if len(current) >= 2 && len(current) <= 12 {
			term := string(current)
			if !oneOfString(term, "我们", "这个", "可以", "一个", "就是", "真的", "比较") {
				counts[term]++
			}
		}
		current = current[:0]
	}
	for _, char := range []rune(value) {
		if unicode.IsLetter(char) || unicode.IsNumber(char) {
			current = append(current, unicode.ToLower(char))
			if len(current) == 12 {
				flush()
			}
		} else {
			flush()
		}
	}
	flush()
	return topStringCounts(counts, limit)
}

func topStringCounts(counts map[string]int, limit int) []string {
	type item struct {
		value string
		count int
	}
	values := make([]item, 0, len(counts))
	for value, count := range counts {
		values = append(values, item{value: value, count: count})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].count == values[j].count {
			return values[i].value < values[j].value
		}
		return values[i].count > values[j].count
	})
	if len(values) > limit {
		values = values[:limit]
	}
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.value
	}
	return result
}

func inferAgeLabels(value string) []string {
	value = strings.ToLower(value)
	result := []string{}
	for _, item := range []struct {
		age  string
		keys []string
	}{
		{"18-22", []string{"大学", "学生", "年轻"}},
		{"23-27", []string{"职场新人", "初入职场", "白领", "熬夜"}},
		{"28-32", []string{"宝妈", "成熟", "管理层", "家庭"}},
		{"32-100", []string{"中年", "银发", "父母", "养生"}},
	} {
		for _, key := range item.keys {
			if strings.Contains(value, key) {
				result = appendUnique(result, item.age)
			}
		}
	}
	return result
}

func appendUnique(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func limitRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func oneOfString(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func roundScore(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}
