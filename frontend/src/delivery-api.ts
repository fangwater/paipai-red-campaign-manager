const DELIVERY_BASE = `${import.meta.env.BASE_URL}api/delivery`;

export type JSONValue = null | boolean | number | string | JSONValue[] | { [key: string]: JSONValue };
export type JSONObject = Record<string, unknown>;

type APIEnvelope<T> = {
  success: boolean;
  data?: T;
  error?: string;
};

export class DeliveryAPIError extends Error {
  status: number;
  details: unknown;

  constructor(message: string, status: number, details?: unknown) {
    super(message);
    this.name = "DeliveryAPIError";
    this.status = status;
    this.details = details;
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body !== undefined && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  const response = await fetch(`${DELIVERY_BASE}${path}`, {
    ...init,
    headers,
    credentials: "same-origin"
  });
  let envelope: APIEnvelope<T> | undefined;
  try {
    envelope = await response.json() as APIEnvelope<T>;
  } catch {
    throw new DeliveryAPIError(`投放服务返回了无法解析的响应（HTTP ${response.status}）`, response.status);
  }
  if (!response.ok || !envelope.success) {
    throw new DeliveryAPIError(envelope.error || `请求失败（HTTP ${response.status}）`, response.status, envelope.data);
  }
  return envelope.data as T;
}

function post<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: "POST",
    body: body === undefined ? undefined : JSON.stringify(body)
  });
}

export type ActorRole = "viewer" | "analyst" | "operator" | "budget_owner" | "admin";

export type Actor = {
  id: string;
  role: ActorRole;
};

export type Advertiser = {
  advertiser_id: number;
  advertiser_name: string;
};

export type DeliverySession = {
  actor: Actor;
  advertisers: Advertiser[];
  all_advertisers: boolean;
};

export type Capability = {
  advertiser_id: number;
  authorized: boolean;
  advertiser_allowed: boolean;
  scopes: string[];
  required_scopes: string[];
  missing_scopes: string[];
  advertiser_count: number;
  media_writes_enabled: boolean;
  contract_version: string;
  operations: JSONObject;
  checked_at: string;
};

export type BudgetPolicy = {
  daily_limit_fen: number;
  total_limit_fen: number;
  advertiser_daily_cap_fen?: number;
  max_bid_fen?: number;
  stop_loss_spend_fen?: number;
  stop_loss_conversions_min?: number;
};

export type CampaignSpec = {
  local_key: string;
  name: string;
  marketing_target: number;
  placement: number;
  promotion_target: number;
  enable: number;
  time_type: number;
  start_time?: string;
  expire_time?: string;
  time_period_type: number;
  time_period?: Record<string, string>;
  bidding_strategy: number;
  limit_day_budget: number;
  day_budget_fen?: number;
  optimize_target: number;
  constraint_type?: number;
  smart_switch?: number;
  pacing_mode?: number;
  feed_flag?: number;
  build_type?: number;
  event_asset_id?: number;
  asset_event?: number;
  asset_event_id?: number;
  page_category?: number;
  search_flag?: number;
  target_extension_switch?: number;
  search_bid_ratio?: number;
  deeplink_id?: number;
  universal_link_id?: number;
  detect_url_link?: string;
};

export type CodeName = { code: string; name: string };

export type TargetSpec = {
  gender?: string;
  age?: string;
  device?: string;
  cities?: string;
  content_interests?: CodeName[];
  shopping_interests?: CodeName[];
  crowd_packages?: JSONObject[];
  behavior_keywords?: string[];
  interest_keywords?: string[];
  keyword_target_period?: number;
  keyword_target_actions?: number[];
  intelligent_expansion?: number;
  exclude_blogger_fans?: boolean;
  exclude_blogger_purchasers?: boolean;
  include_brand_recognition?: boolean;
  include_category_interested?: boolean;
};

export type KeywordBid = {
  keyword: string;
  bid_fen: number;
  feed_bid_fen?: number;
  keyword_source?: number;
  phrase_match_type: number;
};

export type NegativeKeyword = {
  keyword: string;
  phrase_match_type: number;
};

export type CreativitySpec = {
  local_key: string;
  name: string;
  note_id: string;
  click_urls?: string[];
  expo_urls?: string[];
  mask_prefer?: number;
  title_mask_prefer?: number;
  conversion_type?: number;
  jump_url?: string;
  landing_page_type?: number;
  bar_content?: string;
  conversion_component_types?: number[];
  comment?: string;
  app_component_icon?: string;
  fallback_jump_url?: string;
  qualification?: JSONObject;
};

export type UnitSpec = {
  local_key: string;
  name: string;
  event_bid_fen?: number;
  note_ids?: string[];
  promotion_target: number;
  target_type: number;
  target: TargetSpec;
  keyword_target_period?: number;
  keyword_target_actions?: number[];
  business_tree_name?: string;
  spu_notes?: { spu_id: string; note_ids: string[] }[];
  keywords?: KeywordBid[];
  negative_keywords?: NegativeKeyword[];
  substituted_user_id?: string;
  keyword_gen_type?: number;
  page_id?: string;
  landing_page_url?: string;
  external_page_url?: string;
  landing_page_desc?: string;
  target_template_id?: string;
  creativities: CreativitySpec[];
};

export type DraftSpec = {
  advertiser_id: number;
  objective: string;
  placement: string;
  budget: BudgetPolicy;
  notes?: string[];
  campaign: CampaignSpec;
  units: UnitSpec[];
  experiment: {
    primary_metric: string;
    guardrails?: string[];
    variables?: string[];
    hold_constant?: string[];
  };
};

export type Draft = {
  id: string;
  advertiser_id: number;
  status: string;
  current_version: number;
  spec: DraftSpec;
  spec_hash: string;
  idempotency_key: string;
  created_by: string;
  updated_by: string;
  created_at: string;
  updated_at: string;
};

export type Recommendation = {
  id: string;
  draft_id: string;
  draft_version: number;
  schema_version: string;
  llm_provider: string;
  llm_model: string;
  ranker_family: string;
  ranker_version: string;
  rules_version: string;
  payload: JSONObject;
  warnings: string[];
  created_by: string;
  created_at: string;
};

export type ValidationIssue = {
  code: string;
  path: string;
  message: string;
  severity: string;
};

export type Validation = {
  id: string;
  draft_id: string;
  draft_version: number;
  spec_hash: string;
  rules_version: string;
  contract_version: string;
  valid: boolean;
  errors: ValidationIssue[];
  warnings: ValidationIssue[];
  capability_snapshot: JSONObject;
  valid_until: string;
  created_by: string;
  created_at: string;
};

export type Approval = {
  id: string;
  draft_id: string;
  draft_version: number;
  spec_hash: string;
  role: "operator" | "budget_owner";
  decision: "approved" | "rejected";
  actor: string;
  comment: string;
  approved_budget_fen: number;
  expires_at: string;
  created_at: string;
};

export type PublishJob = {
  id: string;
  draft_id: string;
  draft_version: number;
  advertiser_id: number;
  mode: "dry_run" | "execute";
  status: string;
  current_step: string;
  idempotency_key: string;
  request_preview: JSONObject;
  result: JSONObject;
  error_code?: string;
  error_message?: string;
  retry_count: number;
  requested_by: string;
  requested_role: string;
  created_at: string;
  started_at?: string;
  completed_at?: string;
  updated_at: string;
};

export type MediaEntity = {
  id: string;
  job_id: string;
  draft_id: string;
  advertiser_id: number;
  entity_type: "campaign" | "unit" | "creativity";
  local_key: string;
  parent_local_key?: string;
  media_id: number;
  parent_media_id?: number;
  desired_status: string;
  observed_status: string;
  upstream_payload: JSONObject;
  created_at: string;
  updated_at: string;
};

export type Workflow = {
  draft: Draft;
  recommendation?: Recommendation;
  validation?: Validation;
  approvals: Approval[];
  jobs: PublishJob[];
  entities: MediaEntity[];
};

export type CandidateNote = {
  note_id: string;
  title: string;
  content?: string;
  audience: string[];
  scenarios: string[];
  note_types: string[];
  historical_spend: number;
  historical_search_users: number;
  historical_search_cost?: number;
  published: boolean;
  creativity_count: number;
};

export type Assets = {
  advertiser_id: number;
  notes: CandidateNote[];
  count: number;
  generated_at: string;
};

export type GatewayResponse = {
  operation: string;
  data: JSONObject;
  request_id?: string;
  request_hash: string;
  latency_ms: number;
};

export function createDefaultDraftSpec(advertiserID: number): DraftSpec {
  return {
    advertiser_id: advertiserID,
    objective: "种草转化",
    placement: "search",
    budget: {
      daily_limit_fen: 10000,
      total_limit_fen: 30000,
      max_bid_fen: 2000,
      stop_loss_spend_fen: 20000,
      stop_loss_conversions_min: 1
    },
    notes: [],
    campaign: {
      local_key: "campaign",
      name: `自建投流-${new Date().toISOString().slice(0, 10)}`,
      marketing_target: 4,
      placement: 2,
      promotion_target: 1,
      enable: 0,
      time_type: 0,
      time_period_type: 0,
      bidding_strategy: 2,
      limit_day_budget: 1,
      day_budget_fen: 10000,
      optimize_target: 1,
      pacing_mode: 0,
      target_extension_switch: 0
    },
    units: [createDefaultUnit(1)],
    experiment: {
      primary_metric: "conversion",
      guardrails: ["cost", "daily_budget"],
      variables: ["creative"],
      hold_constant: ["budget", "targeting"]
    }
  };
}

export function createDefaultUnit(index: number): UnitSpec {
  return {
    local_key: `unit-${index}`,
    name: `广告单元 ${index}`,
    event_bid_fen: 1000,
    note_ids: [],
    promotion_target: 1,
    target_type: 1,
    target: { gender: "all", age: "all", device: "all", intelligent_expansion: 0 },
    keywords: [],
    negative_keywords: [],
    creativities: [{ local_key: `creative-${index}-1`, name: `创意 ${index}-1`, note_id: "" }]
  };
}

export function createClientKey(prefix: string): string {
  return `${prefix}-${crypto.randomUUID()}`;
}

export const deliveryAPI = {
	session: () => request<DeliverySession>("/session"),
	capabilities: (advertiserID: number) => request<Capability>(`/capabilities?advertiser_id=${advertiserID}`),
  assets: (advertiserID: number, search = "", limit = 50) => {
    const query = new URLSearchParams({ advertiser_id: String(advertiserID), search, limit: String(limit) });
    return request<Assets>(`/assets?${query}`);
  },
  drafts: (advertiserID: number, limit = 100) => request<{ items: Draft[]; count: number }>(`/drafts?advertiser_id=${advertiserID}&limit=${limit}`),
  workflow: (draftID: string) => request<Workflow>(`/drafts/${encodeURIComponent(draftID)}/workflow`),
  createDraft: (spec: DraftSpec, idempotencyKey: string, changeReason: string) => post<Draft>("/drafts", {
    spec,
    idempotency_key: idempotencyKey,
    change_reason: changeReason
  }),
  updateDraft: (draftID: string, spec: DraftSpec, expectedVersion: number, changeReason: string) => request<Draft>(`/drafts/${encodeURIComponent(draftID)}`, {
    method: "PUT",
    body: JSON.stringify({ spec, expected_version: expectedVersion, change_reason: changeReason })
  }),
  recommend: (draftID: string) => post<Recommendation>(`/drafts/${encodeURIComponent(draftID)}/recommendations`, {}),
  validate: (draftID: string) => post<Validation>(`/drafts/${encodeURIComponent(draftID)}/validate`, {}),
  approve: (draftID: string, input: { role: "operator" | "budget_owner"; decision: "approved" | "rejected"; comment: string; approved_budget_fen: number; expires_in_minutes: number }) => post<Approval>(`/drafts/${encodeURIComponent(draftID)}/approve`, input),
  publish: (draftID: string, mode: "dry_run" | "execute", idempotencyKey: string) => post<PublishJob>(`/drafts/${encodeURIComponent(draftID)}/publish`, { mode, idempotency_key: idempotencyKey }),
  job: (jobID: string) => request<PublishJob>(`/jobs/${encodeURIComponent(jobID)}`),
  updateEntityStatus: (entity: MediaEntity, status: "paused" | "active") => post<GatewayResponse>(`/entities/${entity.entity_type}/${entity.media_id}/status`, {
    advertiser_id: entity.advertiser_id,
    status
  }),
  performance: (input: JSONObject) => post<JSONObject>("/performance", input),
  platformTool: (path: string, body: JSONObject) => post<GatewayResponse>(path, body),
  intelligenceCapabilities: () => request<JSONObject>("/intelligence/capabilities"),
  bayesian: (input: JSONObject) => post<JSONObject>("/intelligence/bayesian", input),
  optimizeBudget: (input: JSONObject) => post<JSONObject>("/intelligence/optimize-budget", input),
  banditShadow: (input: JSONObject) => post<JSONObject>("/intelligence/bandit-shadow", input)
};
