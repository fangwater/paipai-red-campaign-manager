import { useEffect, useMemo, useRef, useState } from "react";
import { AlertCircle, ArrowDownWideNarrow, ChevronLeft, ChevronRight, ExternalLink, LoaderCircle, Search, X } from "lucide-react";
import { Link } from "react-router-dom";
import "./content-analysis.css";
import "./placement-note-performance.css";

type ServiceState = "checking" | "online" | "offline";
type Placement = "search" | "feed";
type SPUOption = "辅酶" | "磷虾油";
type AgencyOption = "全部" | "曼杰" | "有一有二" | "智元";
type DimensionOption = "audience" | "scenario";
type NoteSortOption = "search_spend" | "feed_spend" | "search_cost_change";
type CostFilter = "search" | "feed" | "search_stopped" | "feed_stopped";

type ContentCampaign = {
  name: string;
  spend: number;
  cost: number | null;
  latest_spend: number;
  campaign_id: number | null;
  filter_state: number | null;
  enable: number | null;
};

const CAMPAIGN_FILTER_STATES: Record<number, string> = {
  1: "有效",
  2: "暂停",
  3: "已删除",
  4: "计划预算不足",
  5: "现金余额不足",
  7: "账户日预算不足",
  8: "暂停阶段",
  10: "未投放"
};

type ContentNote = {
  note_id: string;
  title: string;
  url: string;
  author: string;
  published_date: string;
  agency: string;
  content_type: string;
  audience: string;
  scenario: string;
  dandelion_cost: number | null;
  boom: boolean;
  search_spend: number;
  search_cost: number | null;
  latest_search_cost: number | null;
  search_cost_change: number | null;
  search_qualified: boolean;
  feed_spend: number;
  feed_cost: number | null;
  feed_qualified: boolean;
  latest_search_spend: number;
  latest_feed_spend: number;
  search_campaigns: ContentCampaign[];
  feed_campaigns: ContentCampaign[];
  search_stopped: boolean;
  feed_stopped: boolean;
};

type ContentCell = {
  content_type: string;
  dimension: string;
  notes: ContentNote[];
};

type ContentResult = {
  sources: {
    dandelion_data_date: string;
    maituo_report_date: string;
  };
  cells: ContentCell[];
};

const SPU_OPTIONS: SPUOption[] = ["辅酶", "磷虾油"];
const AGENCY_OPTIONS: AgencyOption[] = ["全部", "曼杰", "有一有二", "智元"];
const NOTE_PAGE_SIZE = 20;
const DEFAULT_SEARCH_COST_LIMIT = 30;
const DEFAULT_FEED_COST_LIMIT = 70;
const NOTE_SORT_OPTIONS: Array<{ value: NoteSortOption; label: string; description: string }> = [
  { value: "search_spend", label: "搜索累计消耗", description: "按搜索累计消耗从高到低排序" },
  { value: "feed_spend", label: "信息流累计消耗", description: "按信息流累计消耗从高到低排序" },
  { value: "search_cost_change", label: "回搜成本变化", description: "按当日回搜成本 − 累计回搜成本从高到低排序" }
];
const integer = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });
const money = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

function safeURL(value: string): string | null {
  try {
    const parsed = new URL(value);
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? parsed.toString() : null;
  } catch {
    return null;
  }
}

function metricCost(spend: number, cost: number | null): string {
  if (spend <= 0) return "--";
  return cost === null ? "¥" + money.format(spend) + " · 暂无成本" : "¥" + money.format(spend) + " · ¥" + money.format(cost);
}

function formatCostChange(value: number | null): string {
  if (value === null) return "--";
  if (value > 0) return "+¥" + money.format(value);
  if (value < 0) return "-¥" + money.format(Math.abs(value));
  return "¥" + money.format(0);
}

function parseCostLimit(value: string, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback;
}

function costUnqualified(spend: number, cost: number | null, limit: number): boolean {
  return spend > 0 && (cost === null || cost > limit);
}

function noteHasPlacement(note: ContentNote, placement: Placement): boolean {
  if (placement === "search") {
    return note.search_spend > 0 || note.search_cost !== null || note.latest_search_spend > 0 || note.search_stopped;
  }
  return note.feed_spend > 0 || note.feed_cost !== null || note.latest_feed_spend > 0 || note.feed_stopped;
}

function noteCampaignAnalysisPath(noteID: string): string {
  return "/note-campaign-analysis?" + new URLSearchParams({ q: noteID }).toString();
}

function noteCampaigns(note: ContentNote, placement: Placement): ContentCampaign[] {
  return placement === "search" ? note.search_campaigns ?? [] : note.feed_campaigns ?? [];
}

function campaignKey(noteID: string, campaignName: string): string {
  return noteID + "\u0000" + campaignName;
}

function campaignFilterStateLabel(state: number | null | undefined): string {
  if (state === null || state === undefined) return "未匹配";
  return CAMPAIGN_FILTER_STATES[state] ?? ("状态 " + state);
}

function campaignFilterStateTone(state: number | null | undefined): string {
  if (state === null || state === undefined) return "unknown";
  if (state === 1) return "healthy";
  if (state === 2 || state === 8 || state === 10) return "paused";
  return "warning";
}

function PlacementMetric({ spend, cost, qualified, stopped }: {
  spend: number;
  cost: number | null;
  qualified: boolean;
  stopped: boolean;
}) {
  return <div className={"content-note-metric" + (qualified && !stopped ? " metric-good" : "")}>
    <span>{metricCost(spend, cost)}</span>
    {stopped ? <em className="content-note-stopped">已停投</em> : null}
  </div>;
}

function PlacementCampaigns({ noteID, campaigns, selectedKeys, onToggleCampaign }: {
  noteID: string;
  campaigns: ContentCampaign[];
  selectedKeys: Set<string>;
  onToggleCampaign: (noteID: string, campaignName: string, checked: boolean) => void;
}) {
  if (campaigns.length === 0) {
    return <div className="placement-note-campaigns"><span className="placement-note-campaigns-empty">暂无计划</span></div>;
  }
  return <ul className="placement-note-campaigns">
    {campaigns.map((campaign) => {
      const key = campaignKey(noteID, campaign.name);
      const checked = selectedKeys.has(key);
      const stopped = campaign.latest_spend <= 0;
      const stateLabel = campaignFilterStateLabel(campaign.filter_state);
      const stateTone = campaignFilterStateTone(campaign.filter_state);
      const stateTitle = campaign.campaign_id
        ? "聚光计划 " + campaign.campaign_id + " · " + stateLabel + (campaign.enable === 0 ? " · 开关关闭" : campaign.enable === 1 ? " · 开关开启" : "")
        : "未匹配到本地同步的聚光计划";
      return <li key={campaign.name}>
        <label className={"placement-campaign-card" + (checked ? " selected" : "") + (stopped ? " stopped" : "")}>
          <input
            type="checkbox"
            checked={checked}
            aria-label={"选择计划 " + campaign.name}
            onChange={(event) => onToggleCampaign(noteID, campaign.name, event.target.checked)}
          />
          <span className="placement-campaign-body">
            <span className="placement-campaign-title">
              <span className={"placement-campaign-state " + stateTone} title={stateTitle}>{stateLabel}</span>
              <strong title={campaign.name}>{campaign.name}</strong>
            </span>
            <small>
              <span>消耗 ¥{money.format(campaign.spend)}</span>
              <span>{campaign.cost === null ? "成本 --" : "成本 ¥" + money.format(campaign.cost)}</span>
              {stopped ? <span className="placement-campaign-spend-stopped">近一天 0</span> : <span>近一天 ¥{money.format(campaign.latest_spend)}</span>}
            </small>
          </span>
        </label>
      </li>;
    })}
  </ul>;
}

function PlacementNoteTable({ notes, placement, label, selectedCampaignKeys, onToggleNote, onToggleCampaign, onToggleAllNotes }: {
  notes: ContentNote[];
  placement: Placement;
  label: string;
  selectedCampaignKeys: Set<string>;
  onToggleNote: (note: ContentNote, checked: boolean) => void;
  onToggleCampaign: (noteID: string, campaignName: string, checked: boolean) => void;
  onToggleAllNotes: (checked: boolean) => void;
}) {
  const pageCampaignKeys = useMemo(() => {
    const keys: string[] = [];
    for (const note of notes) {
      for (const campaign of noteCampaigns(note, placement)) {
        keys.push(campaignKey(note.note_id, campaign.name));
      }
    }
    return keys;
  }, [notes, placement]);
  const selectedOnPage = pageCampaignKeys.filter((key) => selectedCampaignKeys.has(key)).length;
  const allChecked = pageCampaignKeys.length > 0 && selectedOnPage === pageCampaignKeys.length;
  const someChecked = selectedOnPage > 0 && !allChecked;

  return <div className="content-note-table-wrap"><table className={"content-note-table content-note-table-summary placement-note-table placement-note-table-" + placement} aria-label={label}>
    <thead><tr>
      <th className="placement-note-select-col">
        <input
          type="checkbox"
          checked={allChecked}
          ref={(element) => {
            if (element) element.indeterminate = someChecked;
          }}
          disabled={pageCampaignKeys.length === 0}
          aria-label="全选本页笔记计划"
          onChange={(event) => onToggleAllNotes(event.target.checked)}
        />
      </th>
      <th>笔记</th>
      <th>机构与标签</th>
      <th>计划</th>
      <th>站外成本 15 天</th>
      {placement === "search" ? <th>搜索累计消耗 · 成本</th> : <th>信息流累计消耗 · 成本</th>}
      {placement === "search" ? <th>回搜成本变化</th> : null}
    </tr></thead>
    <tbody>{notes.map((note) => {
      const noteURL = safeURL(note.url);
      const campaigns = noteCampaigns(note, placement);
      const noteCampaignKeys = campaigns.map((campaign) => campaignKey(note.note_id, campaign.name));
      const selectedCount = noteCampaignKeys.filter((key) => selectedCampaignKeys.has(key)).length;
      const noteChecked = noteCampaignKeys.length > 0 && selectedCount === noteCampaignKeys.length;
      const notePartial = selectedCount > 0 && !noteChecked;
      return <tr key={note.note_id}>
        <td className="placement-note-select-col">
          <input
            type="checkbox"
            checked={noteChecked}
            ref={(element) => {
              if (element) element.indeterminate = notePartial;
            }}
            disabled={noteCampaignKeys.length === 0}
            aria-label={"选择笔记 " + note.note_id}
            onChange={(event) => onToggleNote(note, event.target.checked)}
          />
        </td>
        <td><div className="content-note-identity">
          {noteURL ? <a className="content-note-title" href={noteURL} target="_blank" rel="noreferrer" title={note.title}>{note.title}<ExternalLink size={12} /></a> : <strong className="content-note-title" title={note.title}>{note.title}</strong>}
          <Link className="content-note-id" to={noteCampaignAnalysisPath(note.note_id)} title="查看笔记计划分析" aria-label={"查看笔记计划分析 " + note.note_id}>{note.note_id}</Link>
          <small>{note.author || "未知达人"} · {note.published_date || "发布时间未知"}</small>
        </div></td>
        <td><div className="content-note-labels placement-note-labels"><strong>{note.agency}</strong><span>{note.content_type}</span><span>{note.audience}</span><span>{note.scenario}</span></div></td>
        <td><PlacementCampaigns noteID={note.note_id} campaigns={campaigns} selectedKeys={selectedCampaignKeys} onToggleCampaign={onToggleCampaign} /></td>
        <td className={note.boom ? "metric-good" : ""}>{note.dandelion_cost === null || note.dandelion_cost <= 0 ? "--" : "¥" + money.format(note.dandelion_cost)}</td>
        {placement === "search"
          ? <td><PlacementMetric spend={note.search_spend} cost={note.search_cost} qualified={note.search_qualified} stopped={note.search_stopped} /></td>
          : <td><PlacementMetric spend={note.feed_spend} cost={note.feed_cost} qualified={note.feed_qualified} stopped={note.feed_stopped} /></td>}
        {placement === "search" ? <td className={"search-cost-change" + (note.search_cost_change !== null && note.search_cost_change > 0 ? " increase" : note.search_cost_change !== null && note.search_cost_change < 0 ? " decrease" : "")} title={note.latest_search_cost === null || note.search_cost === null ? "暂无完整回搜成本" : "当日回搜成本 ¥" + money.format(note.latest_search_cost) + " − 累计回搜成本 ¥" + money.format(note.search_cost)}>{formatCostChange(note.search_cost_change)}</td> : null}
      </tr>;
    })}</tbody>
  </table></div>;
}

function PlacementNotePerformance({ placement, serviceState }: { placement: Placement; serviceState: ServiceState }) {
  const pageTitle = placement === "search" ? "搜索" : "信息流";
  const defaultSort: NoteSortOption = placement === "search" ? "search_spend" : "feed_spend";
  const [spu, setSPU] = useState<SPUOption>("辅酶");
  const [agency, setAgency] = useState<AgencyOption>("全部");
  const [publishedStartDate, setPublishedStartDate] = useState("");
  const [publishedEndDate, setPublishedEndDate] = useState("");
  const [includeUnlabeled, setIncludeUnlabeled] = useState(false);
  const [noteSort, setNoteSort] = useState<NoteSortOption>(defaultSort);
  const [costFilters, setCostFilters] = useState<CostFilter[]>([]);
  const [searchCostLimitInput, setSearchCostLimitInput] = useState(String(DEFAULT_SEARCH_COST_LIMIT));
  const [feedCostLimitInput, setFeedCostLimitInput] = useState(String(DEFAULT_FEED_COST_LIMIT));
  const [noteIDQuery, setNoteIDQuery] = useState("");
  const [notePage, setNotePage] = useState(1);
  const noteSectionRef = useRef<HTMLElement>(null);
  const [result, setResult] = useState<ContentResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [selectedCampaignKeys, setSelectedCampaignKeys] = useState<Set<string>>(() => new Set());

  useEffect(() => {
    setNoteSort(defaultSort);
    setCostFilters([]);
    setNoteIDQuery("");
    setNotePage(1);
    setSelectedCampaignKeys(new Set());
  }, [defaultSort, placement]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    const params = new URLSearchParams({ spu, agency, dimension: "audience" satisfies DimensionOption });
    if (publishedStartDate) params.set("published_start_date", publishedStartDate);
    if (publishedEndDate) params.set("published_end_date", publishedEndDate);
    fetch(import.meta.env.BASE_URL + "api/analytics/content-analysis?" + params, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: ContentResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || pageTitle + "笔记读取失败");
        setResult(payload.data);
        setSelectedCampaignKeys(new Set());
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : pageTitle + "笔记读取失败");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [agency, pageTitle, publishedEndDate, publishedStartDate, spu]);

  const searchCostLimit = parseCostLimit(searchCostLimitInput, DEFAULT_SEARCH_COST_LIMIT);
  const feedCostLimit = parseCostLimit(feedCostLimitInput, DEFAULT_FEED_COST_LIMIT);
  const normalizedNoteIDQuery = noteIDQuery.trim().toLowerCase();
  const placementNotes = useMemo(() => {
    const notesByID = new Map<string, ContentNote>();
    for (const cell of result?.cells ?? []) {
      if (!includeUnlabeled && (cell.content_type === "未标注" || cell.dimension === "未标注")) continue;
      for (const note of cell.notes) {
        if (!includeUnlabeled && (note.content_type === "未标注" || note.audience === "未标注" || note.scenario === "未标注")) continue;
        if (!noteHasPlacement(note, placement)) continue;
        notesByID.set(note.note_id, note);
      }
    }
    return Array.from(notesByID.values());
  }, [includeUnlabeled, placement, result]);
  const visibleNotes = useMemo(() => {
    return placementNotes.filter((note) => {
      if (normalizedNoteIDQuery && !note.note_id.toLowerCase().includes(normalizedNoteIDQuery)) return false;
      if (costFilters.length === 0) return true;
      return costFilters.some((filter) => {
        if (filter === "search") return costUnqualified(note.search_spend, note.search_cost, searchCostLimit);
        if (filter === "feed") return costUnqualified(note.feed_spend, note.feed_cost, feedCostLimit);
        if (filter === "search_stopped") return note.search_stopped;
        return note.feed_stopped;
      });
    }).sort((left, right) => {
      const sortValue = (note: ContentNote) => {
        if (noteSort === "feed_spend") return note.feed_spend;
        if (noteSort === "search_cost_change") return note.search_cost_change ?? Number.NEGATIVE_INFINITY;
        return note.search_spend;
      };
      return sortValue(right) - sortValue(left) || left.note_id.localeCompare(right.note_id);
    });
  }, [costFilters, feedCostLimit, normalizedNoteIDQuery, noteSort, placementNotes, searchCostLimit]);
  const notePageCount = Math.max(1, Math.ceil(visibleNotes.length / NOTE_PAGE_SIZE));
  const pagedVisibleNotes = useMemo(() => {
    const start = (notePage - 1) * NOTE_PAGE_SIZE;
    return visibleNotes.slice(start, start + NOTE_PAGE_SIZE);
  }, [notePage, visibleNotes]);
  const unqualifiedCounts = useMemo(() => ({
    search: placementNotes.filter((note) => costUnqualified(note.search_spend, note.search_cost, searchCostLimit)).length,
    feed: placementNotes.filter((note) => costUnqualified(note.feed_spend, note.feed_cost, feedCostLimit)).length,
    searchStopped: placementNotes.filter((note) => note.search_stopped).length,
    feedStopped: placementNotes.filter((note) => note.feed_stopped).length
  }), [feedCostLimit, placementNotes, searchCostLimit]);
  const sortLabel = NOTE_SORT_OPTIONS.find((option) => option.value === noteSort)?.label ?? (placement === "search" ? "搜索累计消耗" : "信息流累计消耗");
  const selectedCampaignCount = selectedCampaignKeys.size;

  const toggleCostFilter = (value: CostFilter) => {
    setCostFilters((current) => current.includes(value) ? current.filter((item) => item !== value) : [...current, value]);
  };

  const toggleCampaign = (noteID: string, campaignName: string, checked: boolean) => {
    const key = campaignKey(noteID, campaignName);
    setSelectedCampaignKeys((current) => {
      const next = new Set(current);
      if (checked) next.add(key);
      else next.delete(key);
      return next;
    });
  };

  const toggleNote = (note: ContentNote, checked: boolean) => {
    const keys = noteCampaigns(note, placement).map((campaign) => campaignKey(note.note_id, campaign.name));
    setSelectedCampaignKeys((current) => {
      const next = new Set(current);
      for (const key of keys) {
        if (checked) next.add(key);
        else next.delete(key);
      }
      return next;
    });
  };

  const toggleAllNotes = (checked: boolean) => {
    setSelectedCampaignKeys((current) => {
      const next = new Set(current);
      for (const note of pagedVisibleNotes) {
        for (const campaign of noteCampaigns(note, placement)) {
          const key = campaignKey(note.note_id, campaign.name);
          if (checked) next.add(key);
          else next.delete(key);
        }
      }
      return next;
    });
  };

  useEffect(() => {
    setNotePage(1);
  }, [agency, costFilters, feedCostLimit, includeUnlabeled, normalizedNoteIDQuery, noteSort, placement, publishedEndDate, publishedStartDate, searchCostLimit, spu]);

  useEffect(() => {
    setNotePage((current) => Math.min(current, notePageCount));
  }, [notePageCount]);

  const changeNotePage = (page: number) => {
    setNotePage(page);
    window.requestAnimationFrame(() => {
      noteSectionRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
    });
  };

  return <>
    <section className="page-heading content-page-heading">
      <div><h1>{pageTitle}</h1><p>投放管理 · 按{pageTitle}累计消耗查看笔记表现</p></div>
      <div className="content-heading-actions">
        <div className="segmented-control content-spu-control" aria-label={pageTitle + " SPU"}>
          {SPU_OPTIONS.map((option) => <button type="button" className={spu === option ? "active" : ""} key={option} onClick={() => setSPU(option)}>{option}</button>)}
        </div>
        <div className="heading-status"><span className={"status-dot " + serviceState} />{serviceState === "online" ? "数据服务已连接" : serviceState === "offline" ? "数据服务未连接" : "正在检查连接"}</div>
      </div>
    </section>

    <section className="content-toolbar">
      <div className="content-filter-group"><span>机构</span><div className="segmented-control content-agency-control" aria-label={pageTitle + "机构"}>
        {AGENCY_OPTIONS.map((option) => <button type="button" className={agency === option ? "active" : ""} key={option} onClick={() => setAgency(option)}>{option === "全部" ? "全部机构" : option}</button>)}
      </div></div>
      <div className="content-filter-group content-date-filter"><span>发布时间</span><div className="content-date-range">
        <input type="date" aria-label="发布时间开始" value={publishedStartDate} max={publishedEndDate || undefined} onChange={(event) => setPublishedStartDate(event.target.value)} />
        <span aria-hidden="true">至</span>
        <input type="date" aria-label="发布时间结束" value={publishedEndDate} min={publishedStartDate || undefined} onChange={(event) => setPublishedEndDate(event.target.value)} />
        <button type="button" title="清除发布时间范围" aria-label="清除发布时间范围" disabled={!publishedStartDate && !publishedEndDate} onClick={() => { setPublishedStartDate(""); setPublishedEndDate(""); }}><X size={14} /></button>
      </div></div>
      <label className="content-unlabeled-toggle"><input type="checkbox" checked={includeUnlabeled} onChange={(event) => setIncludeUnlabeled(event.target.checked)} />包含未标注</label>
    </section>

    {error ? <div className="analysis-error content-error"><AlertCircle size={17} />{error}</div> : null}
    {loading ? <div className="content-loading"><LoaderCircle size={19} className="spin" />正在汇总{pageTitle}笔记</div> : null}

    {!loading && result ? <section className="content-note-section" ref={noteSectionRef}>
      <header>
        <div><h2>笔记表现</h2><p>默认按{pageTitle}累计消耗降序；回搜成本变化 = 当日回搜成本 − 累计回搜成本</p></div>
        <span>{integer.format(visibleNotes.length)} 篇笔记{selectedCampaignCount > 0 ? " · 已选 " + integer.format(selectedCampaignCount) + " 个计划" : ""}</span>
      </header>
      <div className="content-note-controls">
        <div className="content-note-sort">
          <ArrowDownWideNarrow size={14} />
          <span>排序</span>
          <div className="content-note-sort-buttons" aria-label="笔记排序方式">
            {NOTE_SORT_OPTIONS.map((option) => <button type="button" className={noteSort === option.value ? "active" : ""} aria-pressed={noteSort === option.value} key={option.value} title={option.description} onClick={() => setNoteSort(option.value)}>{option.label}</button>)}
          </div>
        </div>
        <div className="content-note-filter-row">
          <div className="content-note-filters" aria-label="笔记表现筛选">
            {placement === "search" ? <>
              <button type="button" className={"content-note-filter-card" + (costFilters.includes("search") ? " active" : "")} aria-pressed={costFilters.includes("search")} onClick={() => toggleCostFilter("search")}>
                <span>搜索成本不达标</span>
                <strong>{integer.format(unqualifiedCounts.search)}</strong>
                <small>累计回搜成本 &gt; {searchCostLimit} 或暂无成本</small>
              </button>
              <button type="button" className={"content-note-filter-card" + (costFilters.includes("search_stopped") ? " active" : "")} aria-pressed={costFilters.includes("search_stopped")} onClick={() => toggleCostFilter("search_stopped")}>
                <span>搜索已停投</span>
                <strong>{integer.format(unqualifiedCounts.searchStopped)}</strong>
                <small>近一天搜索消耗为 0</small>
              </button>
              <label className="content-note-threshold">搜索阈值<input type="number" min="0" step="1" aria-label="搜索成本不达标阈值" value={searchCostLimitInput} onChange={(event) => setSearchCostLimitInput(event.target.value)} /></label>
            </> : <>
              <button type="button" className={"content-note-filter-card" + (costFilters.includes("feed") ? " active" : "")} aria-pressed={costFilters.includes("feed")} onClick={() => toggleCostFilter("feed")}>
                <span>信息流成本不达标</span>
                <strong>{integer.format(unqualifiedCounts.feed)}</strong>
                <small>累计成本 &gt; {feedCostLimit} 或暂无成本</small>
              </button>
              <button type="button" className={"content-note-filter-card" + (costFilters.includes("feed_stopped") ? " active" : "")} aria-pressed={costFilters.includes("feed_stopped")} onClick={() => toggleCostFilter("feed_stopped")}>
                <span>信息流已停投</span>
                <strong>{integer.format(unqualifiedCounts.feedStopped)}</strong>
                <small>近一天信息流消耗为 0</small>
              </button>
              <label className="content-note-threshold">信息流阈值<input type="number" min="0" step="1" aria-label="信息流成本不达标阈值" value={feedCostLimitInput} onChange={(event) => setFeedCostLimitInput(event.target.value)} /></label>
            </>}
          </div>
          <label className="content-note-id-search">按笔记 ID 搜索
            <span>
              <Search size={14} />
              <input type="search" value={noteIDQuery} placeholder="输入笔记 ID" aria-label="按笔记 ID 搜索" onChange={(event) => setNoteIDQuery(event.target.value)} />
              {noteIDQuery ? <button type="button" title="清除笔记 ID 搜索" aria-label="清除笔记 ID 搜索" onClick={() => setNoteIDQuery("")}><X size={12} /></button> : null}
            </span>
          </label>
        </div>
      </div>
      {visibleNotes.length === 0 ? <div className="content-note-section-empty">当前筛选条件下暂无{pageTitle}笔记</div> : <>
        <PlacementNoteTable
          notes={pagedVisibleNotes}
          placement={placement}
          label={"按" + sortLabel + "排序的笔记"}
          selectedCampaignKeys={selectedCampaignKeys}
          onToggleNote={toggleNote}
          onToggleCampaign={toggleCampaign}
          onToggleAllNotes={toggleAllNotes}
        />
        <nav className="content-note-pagination" aria-label="笔记分页">
          <span>共 {integer.format(visibleNotes.length)} 篇 · 每页 {NOTE_PAGE_SIZE} 篇</span>
          <div className="content-note-pagination-controls">
            <button type="button" title="上一页" aria-label="上一页" disabled={notePage === 1} onClick={() => changeNotePage(notePage - 1)}><ChevronLeft size={15} /></button>
            <select aria-label="选择笔记页码" value={notePage} onChange={(event) => changeNotePage(Number(event.target.value))}>
              {Array.from({ length: notePageCount }, (_, index) => <option value={index + 1} key={index + 1}>第 {index + 1} / {notePageCount} 页</option>)}
            </select>
            <button type="button" title="下一页" aria-label="下一页" disabled={notePage === notePageCount} onClick={() => changeNotePage(notePage + 1)}><ChevronRight size={15} /></button>
          </div>
        </nav>
      </>}
    </section> : null}
  </>;
}

export default PlacementNotePerformance;
