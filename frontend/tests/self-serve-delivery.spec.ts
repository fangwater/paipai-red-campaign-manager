import { expect, test, type Page, type Route } from "@playwright/test";

const now = "2026-08-13T08:00:00Z";
const advertiserID = 710001;
const noteID = "64a1b2c3d4e5f60718293a4b";
const draftID = "drf_0123456789abcdef0123456789abcdef";

const spec = {
  advertiser_id: advertiserID,
  objective: "种草转化",
  placement: "search",
  budget: { daily_limit_fen: 10000, total_limit_fen: 30000, max_bid_fen: 2000, stop_loss_spend_fen: 20000, stop_loss_conversions_min: 1 },
  notes: [noteID],
  campaign: {
    local_key: "campaign", name: "辅酶Q10 搜索验证", marketing_target: 4, placement: 2,
    promotion_target: 1, enable: 0, time_type: 0, time_period_type: 0, bidding_strategy: 2,
    limit_day_budget: 1, day_budget_fen: 10000, optimize_target: 1, pacing_mode: 0,
    target_extension_switch: 0
  },
  units: [{
    local_key: "unit-1", name: "核心词单元", event_bid_fen: 1000, note_ids: [noteID],
    promotion_target: 1, target_type: 1,
    target: { gender: "all", age: "all", device: "all", intelligent_expansion: 0 },
    keywords: [{ keyword: "辅酶Q10", bid_fen: 1200, phrase_match_type: 1 }],
    negative_keywords: [{ keyword: "免费", phrase_match_type: 1 }],
    creativities: [{ local_key: "creative-1-1", name: "抗氧化科普", note_id: noteID }]
  }],
  experiment: { primary_metric: "conversion", guardrails: ["cost"], variables: ["creative"], hold_constant: ["budget"] }
};

function draft(overrides: Record<string, unknown> = {}) {
  return {
    id: draftID, advertiser_id: advertiserID, status: "draft", current_version: 1, spec,
    spec_hash: "a".repeat(64), idempotency_key: "draft-test-key", created_by: "submitter-a",
    updated_by: "submitter-a", created_at: now, updated_at: now, ...overrides
  };
}

function envelope(data: unknown) {
  return { success: true, data };
}

async function fulfill(route: Route, data: unknown, status = 200) {
  await route.fulfill({ status, contentType: "application/json", body: JSON.stringify(status >= 400 ? { success: false, error: String(data) } : envelope(data)) });
}

type MockState = {
  drafts: ReturnType<typeof draft>[];
  recommendation?: Record<string, unknown>;
  validation?: Record<string, unknown>;
  approvals: Record<string, unknown>[];
  jobs: Record<string, unknown>[];
};

async function mockDeliveryAPI(page: Page, initialDrafts: ReturnType<typeof draft>[] = []) {
  const state: MockState = { drafts: initialDrafts, approvals: [], jobs: [] };
  await page.route("**/paipai/api/delivery/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace("/paipai/api/delivery", "");
    const method = request.method();

    if (path === "/session" && method === "GET") {
      return fulfill(route, {
        actor: { id: "delivery-console", role: "operator" },
        advertisers: [{ advertiser_id: advertiserID, advertiser_name: "暂停态验收广告主" }],
        all_advertisers: true
      });
    }
    if (path === "/capabilities") return fulfill(route, {
      advertiser_id: advertiserID, authorized: true, advertiser_allowed: true,
      scopes: ["ad_manage", "ad_data"], required_scopes: ["ad_manage", "ad_data"], missing_scopes: [],
      advertiser_count: 1, media_writes_enabled: false, contract_version: "xhs-jg/2026-05-candidate",
      operations: {}, checked_at: now
    });
    if (path === "/drafts" && method === "GET") return fulfill(route, { items: state.drafts, count: state.drafts.length });
    if (path === "/drafts" && method === "POST") {
      const body = request.postDataJSON() as { spec: typeof spec };
      const created = draft({ spec: body.spec, updated_by: "submitter-a" });
      state.drafts = [created];
      return fulfill(route, created, 201);
    }
    if (path === `/drafts/${draftID}` && method === "PUT") {
      const body = request.postDataJSON() as { spec: typeof spec };
      state.drafts = [draft({ spec: body.spec, current_version: 2, updated_by: "submitter-b" })];
      state.recommendation = undefined; state.validation = undefined; state.approvals = []; state.jobs = [];
      return fulfill(route, state.drafts[0]);
    }
    if (path === `/drafts/${draftID}/workflow`) return fulfill(route, {
      draft: state.drafts[0], recommendation: state.recommendation, validation: state.validation,
      approvals: state.approvals, jobs: state.jobs, entities: []
    });
    if (path === `/drafts/${draftID}/recommendations`) {
      state.recommendation = {
        id: "rec_0123456789abcdef0123456789abcdef", draft_id: draftID, draft_version: state.drafts[0].current_version,
        schema_version: "delivery-recommendation/v1", llm_provider: "local", llm_model: "rules-semantic/v1",
        ranker_family: "deterministic-baseline", ranker_version: "note-ranker/v1", rules_version: "delivery-rules/2026-08-13",
        payload: { ranked_notes: [{ note_id: noteID, score: 0.87, rank: 1 }], themes: ["抗氧化"], keyword_seeds: ["辅酶Q10"], requires_human_review: true, executable: false },
        warnings: ["样本量有限"], created_by: "admin-reviewer", created_at: now
      };
      return fulfill(route, state.recommendation);
    }
    if (path === `/drafts/${draftID}/validate`) {
      state.validation = {
        id: "val_0123456789abcdef0123456789abcdef", draft_id: draftID, draft_version: state.drafts[0].current_version,
        spec_hash: "a".repeat(64), rules_version: "delivery-rules/2026-08-13", contract_version: "xhs-jg/2026-05-candidate",
        valid: true, errors: [], warnings: [{ code: "media_writes_disabled", path: "advertiser_id", message: "媒体写入关闭，dry_run 可用", severity: "warning" }],
        capability_snapshot: { media_writes_enabled: false }, valid_until: "2026-08-13T08:15:00Z", created_by: "admin-reviewer", created_at: now
      };
      return fulfill(route, state.validation);
    }
    if (path === `/drafts/${draftID}/approve`) {
      const body = request.postDataJSON() as Record<string, unknown>;
      const approval = { id: `apr_${state.approvals.length + 1}`, draft_id: draftID, draft_version: state.drafts[0].current_version, spec_hash: "a".repeat(64), actor: "admin-reviewer", expires_at: "2026-08-13T09:00:00Z", created_at: now, ...body };
      state.approvals.push(approval);
      return fulfill(route, approval);
    }
    if (path === `/drafts/${draftID}/publish`) {
      const body = request.postDataJSON() as { mode: string; idempotency_key: string };
      const job = {
        id: "job_0123456789abcdef0123456789abcdef", draft_id: draftID, draft_version: state.drafts[0].current_version,
        advertiser_id: advertiserID, mode: body.mode, status: "succeeded", current_step: "preview_complete",
        idempotency_key: body.idempotency_key, request_preview: { campaign: state.drafts[0].spec.campaign, units: state.drafts[0].spec.units },
        result: { dry_run: true }, retry_count: 0, requested_by: "admin-reviewer", requested_role: "admin",
        created_at: now, updated_at: now
      };
      state.jobs = [job];
      return fulfill(route, job);
    }
    if (path === "/assets" && method === "GET") return fulfill(route, {
      advertiser_id: advertiserID, count: 1, generated_at: now,
      notes: [{ note_id: noteID, title: "抗氧化与精力管理", content: "稿件正文", audience: ["28-32"], scenarios: ["职场"], note_types: ["科普"], historical_spend: 138.5, historical_search_users: 42, historical_search_cost: 3.3, published: true, creativity_count: 2 }]
    });
    if (["/assets/platform", "/target-options", "/audience-estimates", "/keyword-candidates", "/negative-keywords", "/campaigns/query", "/units/query", "/creativities/query"].includes(path)) return fulfill(route, {
      operation: "asset.note_list", data: { list: [{ note_id: noteID, title: "平台笔记" }] }, request_id: "req-test", request_hash: "b".repeat(64), latency_ms: 23
    });
    if (path === "/performance") return fulfill(route, { list: [{ campaign_id: "9001", campaign_name: "辅酶Q10 搜索验证", spend: 123.45, impressions: 5600 }] });
    if (path === "/intelligence/capabilities") return fulfill(route, {
      llm: { provider: "local", model: "rules-semantic/v1", configured: true },
      ranker: { family: "deterministic-baseline", version: "note-ranker/v1", configured: true },
      bayesian: { method: "beta-binomial-normal-approximation/v1" }, optimizer: { method: "constrained-greedy-marginal-value/v1", executable: false },
      bandit: { method: "contextual-ucb-shadow/v1", shadow_only: true },
      responsibility_boundary: {
        llm: "semantic extraction, candidate keywords, and evidence summaries only",
        lightgbm_lambdamart: "ranking over approved numeric features only",
        bayesian: "uncertainty intervals and shrinkage for sparse segments",
        constraint_optimizer: "allocation suggestions inside operator-approved caps",
        bandit: "shadow suggestions only; never activates or changes media state",
        rules: "permissions, platform enums, budget caps, approvals, and safety checks",
        human: "final targeting, budget, publish, activation, and stop-loss decisions"
      }
    });
    if (path === "/intelligence/bayesian") return fulfill(route, { posterior_alpha: 9, posterior_beta: 93, mean: 0.088235, variance: 0.00078, credible_low_95: 0.0335, credible_high_95: 0.143, method: "beta-binomial-normal-approximation/v1" });
    if (path === "/intelligence/optimize-budget") return fulfill(route, { total_fen: 30000, allocated_fen: 30000, unallocated_fen: 0, executable: false, allocations: [{ key: "search-a", amount_fen: 20000, score: 0.77 }, { key: "search-b", amount_fen: 10000, score: 0.55 }] });
    if (path === "/intelligence/bandit-shadow") return fulfill(route, { selected_key: "creative-b", scores: { "creative-a": 0.32, "creative-b": 0.46 }, method: "contextual-ucb-shadow/v1", shadow_only: true });
    return fulfill(route, `unmocked delivery route: ${method} ${path}`, 404);
  });
  return state;
}

async function openConsole(page: Page) {
  await page.goto("/paipai/self-serve-delivery");
  await expect(page.getByLabel("选择广告主")).toHaveValue(String(advertiserID));
  await expect(page.getByText("delivery-console", { exact: true })).toBeVisible();
  await expect(page.getByText("控制台直通")).toBeVisible();
  await expect(page.getByText("投放身份验证", { exact: true })).toHaveCount(0);
}

test("creates a complete draft and restores its persisted workflow", async ({ page }) => {
  await mockDeliveryAPI(page);
  await openConsole(page);

  await expect(page.getByRole("heading", { name: "新建投放草稿" })).toBeVisible();
  await expect(page.getByText("媒体写入关闭", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: /加入稿件 抗氧化与精力管理/ }).click();
  await page.getByPlaceholder("辅酶Q10|12.00|1").fill("辅酶Q10|12.00|1\n抗氧化|10.00|2");
  await page.getByPlaceholder("免费|1").fill("免费|1");
  await page.getByRole("button", { name: "创建草稿" }).click();

  await expect(page.getByText("草稿已创建", { exact: true })).toBeVisible();
  await expect(page.getByText(draftID, { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: /校验与审批/ })).toBeVisible();
});

test("opens a linked quick-plan draft directly in review", async ({ page }) => {
  await mockDeliveryAPI(page, [draft()]);
  await page.goto(`/paipai/self-serve-delivery?advertiser_id=${advertiserID}&draft=${draftID}&view=review`);
  await expect(page.getByText(draftID, { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: /校验与审批/ })).toHaveClass(/active/);
  await expect(page.getByRole("button", { name: "执行校验" })).toBeVisible();
});

test("runs recommendations, validation, approval and dry-run while execute remains locked", async ({ page }) => {
  await mockDeliveryAPI(page, [draft()]);
  await openConsole(page);
  await page.getByRole("button", { name: /校验与审批/ }).click();

  await page.getByRole("button", { name: "生成建议" }).click();
  await expect(page.getByText("deterministic-baseline/note-ranker/v1", { exact: false })).toBeVisible();
  await expect(page.getByText("抗氧化", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "执行校验" }).click();
  await expect(page.getByText("当前版本通过校验")).toBeVisible();
  await expect(page.getByText("media_writes_disabled", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "提交审批" }).click();
  await expect(page.getByText("审批已记录", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: /发布与实体/ }).click();
  await expect(page.getByRole("button", { name: "真实发布" })).toBeDisabled();
  await page.getByRole("button", { name: "发布演练" }).click();
  await expect(page.getByText("发布演练已生成", { exact: true })).toBeVisible();
  await expect(page.getByText("preview_complete", { exact: true })).toBeVisible();
});

test("queries assets, platform objects, reports and all decision-support calculators", async ({ page }) => {
  await mockDeliveryAPI(page, [draft()]);
	await openConsole(page);

  await page.getByRole("button", { name: "资产与报表" }).click();
  await expect(page.getByText("抗氧化与精力管理", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "平台查询" }).click();
  await page.getByRole("button", { name: "执行查询" }).click();
  await expect(page.getByRole("cell", { name: "平台笔记" })).toBeVisible();
  await page.getByRole("button", { name: "效果报表" }).click();
  await page.getByRole("button", { name: "查询报表" }).click();
  await expect(page.getByRole("cell", { name: "辅酶Q10 搜索验证" })).toBeVisible();

  await page.getByRole("button", { name: "算法实验室" }).click();
  await expect(page.getByText("LightGBM / LambdaMART", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "计算后验" }).click();
  await expect(page.getByText("8.82%", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "约束优化" }).click();
  await page.getByRole("button", { name: "生成分配建议" }).click();
  await expect(page.getByText("¥200.00", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Bandit 影子" }).click();
  await page.getByRole("button", { name: "计算影子建议" }).click();
  await expect(page.getByText("creative-b", { exact: true })).toBeVisible();
});

test("keeps the operational console and retained blueprint usable on mobile", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockDeliveryAPI(page, [draft()]);
	await openConsole(page);
  await expect(page.getByRole("heading", { name: "辅酶Q10 搜索验证" })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);

  await page.getByRole("button", { name: "设计与边界" }).click();
  await expect(page.getByText("后端接口已落地，上游写入待验收")).toBeVisible();
  await page.getByRole("button", { name: "API 目标" }).click();
  await expect(page.getByRole("heading", { name: "接口台账" })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
});
