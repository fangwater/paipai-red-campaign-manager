CREATE TABLE IF NOT EXISTS guorai_fetch_runs (
    id BIGSERIAL PRIMARY KEY,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('note', 'plan')),
    enterprise_id BIGINT NOT NULL,
    xhs_brand_id TEXT NOT NULL,
    brand_name TEXT NOT NULL DEFAULT '',
    merchant_id TEXT NOT NULL DEFAULT '',
    attribution_shop TEXT NOT NULL DEFAULT '',
    window_start DATE NOT NULL,
    window_end DATE NOT NULL,
    snapshot_date DATE NOT NULL,
    source_cutoff_date DATE NOT NULL,
    attribution_type TEXT NOT NULL DEFAULT '',
    attribution_model TEXT NOT NULL DEFAULT '',
    attribution_window_days INTEGER,
    traffic_type TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    row_count INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    error_message TEXT,
    request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    raw_response JSONB NOT NULL DEFAULT '{}'::jsonb,
    CHECK (window_start <= window_end),
    CHECK (snapshot_date = window_end)
);

CREATE INDEX IF NOT EXISTS idx_guorai_fetch_runs_snapshot
    ON guorai_fetch_runs (entity_type, snapshot_date DESC, started_at DESC);

CREATE TABLE IF NOT EXISTS guorai_notes (
    note_id TEXT PRIMARY KEY,
    note_name TEXT NOT NULL DEFAULT '',
    note_type SMALLINT,
    note_author_name TEXT NOT NULL DEFAULT '',
    account_name TEXT NOT NULL DEFAULT '',
    note_publish_time TIMESTAMP,
    note_pic TEXT NOT NULL DEFAULT '',
    spu_id TEXT NOT NULL DEFAULT '',
    spu_name TEXT NOT NULL DEFAULT '',
    tag TEXT NOT NULL DEFAULT '',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    raw_dimension_payload JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS guorai_plans (
    plan_id TEXT PRIMARY KEY,
    plan_name TEXT NOT NULL DEFAULT '',
    plan_type TEXT NOT NULL DEFAULT '',
    plan_publish_time TIMESTAMP,
    account_name TEXT NOT NULL DEFAULT '',
    tag TEXT NOT NULL DEFAULT '',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    raw_dimension_payload JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS guorai_plan_notes (
    plan_id TEXT NOT NULL REFERENCES guorai_plans(plan_id),
    note_id TEXT NOT NULL REFERENCES guorai_notes(note_id),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (plan_id, note_id)
);

CREATE INDEX IF NOT EXISTS idx_guorai_plan_notes_note
    ON guorai_plan_notes (note_id, plan_id) WHERE is_active;

CREATE TABLE IF NOT EXISTS guorai_note_snapshots (
    id BIGSERIAL PRIMARY KEY,
    fetch_id BIGINT NOT NULL REFERENCES guorai_fetch_runs(id),
    note_id TEXT NOT NULL REFERENCES guorai_notes(note_id),
    enterprise_id BIGINT NOT NULL,
    xhs_brand_id TEXT NOT NULL,
    merchant_id TEXT NOT NULL DEFAULT '',
    snapshot_date DATE NOT NULL,
    window_start DATE NOT NULL,
    window_end DATE NOT NULL,
    total_pay_user NUMERIC, total_pay_amt NUMERIC, total_pay_order NUMERIC,
    total_uroi NUMERIC, total_oroi NUMERIC, total_roi NUMERIC,
    consume NUMERIC, note_consume NUMERIC, note_ad_cost_volume NUMERIC, note_heat_consume NUMERIC,
    exposure_count BIGINT, click_count BIGINT, click_r NUMERIC,
    interact_count BIGINT, interact_r NUMERIC, click_roi NUMERIC,
    part_pay_user NUMERIC, part_pay_amt NUMERIC, part_pay_order NUMERIC,
    part_pay_user_r NUMERIC, part_pay_amt_r NUMERIC, part_pay_order_r NUMERIC,
    part_uroi NUMERIC, part_oroi NUMERIC, part_roi NUMERIC,
    new_pay_user NUMERIC, new_pay_amt NUMERIC, new_pay_order NUMERIC,
    new_pay_user_r NUMERIC, new_pay_amt_r NUMERIC, new_pay_order_r NUMERIC,
    note_endorse_volume BIGINT, note_comment_volume BIGINT, note_collect_volume BIGINT,
    note_share_volume BIGINT, note_follow_volume BIGINT,
    is_new BOOLEAN,
    raw_payload JSONB NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (fetch_id, note_id),
    CHECK (window_start <= window_end),
    CHECK (snapshot_date = window_end)
);

CREATE INDEX IF NOT EXISTS idx_guorai_note_snapshots_analysis
    ON guorai_note_snapshots (snapshot_date DESC, xhs_brand_id, merchant_id, note_id);
CREATE INDEX IF NOT EXISTS idx_guorai_note_snapshots_raw
    ON guorai_note_snapshots USING GIN (raw_payload);

CREATE TABLE IF NOT EXISTS guorai_plan_snapshots (
    id BIGSERIAL PRIMARY KEY,
    fetch_id BIGINT NOT NULL REFERENCES guorai_fetch_runs(id),
    plan_id TEXT NOT NULL REFERENCES guorai_plans(plan_id),
    enterprise_id BIGINT NOT NULL,
    xhs_brand_id TEXT NOT NULL,
    merchant_id TEXT NOT NULL DEFAULT '',
    snapshot_date DATE NOT NULL,
    window_start DATE NOT NULL,
    window_end DATE NOT NULL,
    total_pay_user NUMERIC, total_pay_amt NUMERIC, total_pay_order NUMERIC,
    total_uroi NUMERIC, total_oroi NUMERIC, total_roi NUMERIC,
    part_pay_user NUMERIC, part_pay_amt NUMERIC, part_pay_order NUMERIC,
    part_pay_user_r NUMERIC, part_pay_amt_r NUMERIC, part_pay_order_r NUMERIC,
    part_uroi NUMERIC, part_oroi NUMERIC, part_roi NUMERIC,
    new_pay_user NUMERIC, new_pay_amt NUMERIC, new_pay_order NUMERIC,
    new_pay_user_r NUMERIC, new_pay_amt_r NUMERIC, new_pay_order_r NUMERIC,
    note_ad_cost_volume NUMERIC,
    exposure_count BIGINT, click_count BIGINT, interact_count BIGINT,
    click_r NUMERIC, interact_r NUMERIC, click_roi NUMERIC,
    note_endorse_volume BIGINT, note_comment_volume BIGINT, note_collect_volume BIGINT,
    note_share_volume BIGINT, note_follow_volume BIGINT,
    is_new BOOLEAN,
    raw_payload JSONB NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (fetch_id, plan_id),
    CHECK (window_start <= window_end),
    CHECK (snapshot_date = window_end)
);

CREATE INDEX IF NOT EXISTS idx_guorai_plan_snapshots_analysis
    ON guorai_plan_snapshots (snapshot_date DESC, xhs_brand_id, merchant_id, plan_id);
CREATE INDEX IF NOT EXISTS idx_guorai_plan_snapshots_raw
    ON guorai_plan_snapshots USING GIN (raw_payload);

COMMENT ON TABLE guorai_fetch_runs IS '薯量关注笔记/计划接口拉取批次及原始响应';
COMMENT ON COLUMN guorai_fetch_runs.id IS '拉取批次主键';
COMMENT ON COLUMN guorai_fetch_runs.entity_type IS '数据类型：note 笔记、plan 计划';
COMMENT ON COLUMN guorai_fetch_runs.enterprise_id IS '薯量企业ID';
COMMENT ON COLUMN guorai_fetch_runs.xhs_brand_id IS '小红书品牌ID';
COMMENT ON COLUMN guorai_fetch_runs.brand_name IS '品牌名称';
COMMENT ON COLUMN guorai_fetch_runs.merchant_id IS '归因店铺ID，空字符串表示全部或默认店铺';
COMMENT ON COLUMN guorai_fetch_runs.attribution_shop IS '归因店铺名称';
COMMENT ON COLUMN guorai_fetch_runs.window_start IS '触达时间查询开始日期，包含当天';
COMMENT ON COLUMN guorai_fetch_runs.window_end IS '触达时间查询结束日期，包含当天';
COMMENT ON COLUMN guorai_fetch_runs.snapshot_date IS 'Rolling 趋势快照日期，等于窗口结束日期';
COMMENT ON COLUMN guorai_fetch_runs.source_cutoff_date IS '薯量当前数据统计截止日期';
COMMENT ON COLUMN guorai_fetch_runs.attribution_type IS '平台归因类型，例如点击';
COMMENT ON COLUMN guorai_fetch_runs.attribution_model IS '平台归因模型，例如末触';
COMMENT ON COLUMN guorai_fetch_runs.attribution_window_days IS '平台归因窗口天数';
COMMENT ON COLUMN guorai_fetch_runs.traffic_type IS '平台流量类型，主要用于笔记';
COMMENT ON COLUMN guorai_fetch_runs.status IS '批次状态：running、succeeded、failed';
COMMENT ON COLUMN guorai_fetch_runs.row_count IS '接口返回的实体记录数';
COMMENT ON COLUMN guorai_fetch_runs.started_at IS '拉取开始时间';
COMMENT ON COLUMN guorai_fetch_runs.finished_at IS '拉取完成时间';
COMMENT ON COLUMN guorai_fetch_runs.error_message IS '失败原因';
COMMENT ON COLUMN guorai_fetch_runs.request_payload IS '发往平台的原始查询参数';
COMMENT ON COLUMN guorai_fetch_runs.raw_response IS '合并分页后的完整原始查询结果';

COMMENT ON TABLE guorai_notes IS '薯量笔记最新维度信息';
COMMENT ON COLUMN guorai_notes.note_id IS '笔记ID';
COMMENT ON COLUMN guorai_notes.note_name IS '笔记名称';
COMMENT ON COLUMN guorai_notes.note_type IS '笔记形式原始代码';
COMMENT ON COLUMN guorai_notes.note_author_name IS '博主名称';
COMMENT ON COLUMN guorai_notes.account_name IS '博主账号ID';
COMMENT ON COLUMN guorai_notes.note_publish_time IS '笔记发布时间，按北京时间解释';
COMMENT ON COLUMN guorai_notes.note_pic IS '笔记封面地址';
COMMENT ON COLUMN guorai_notes.spu_id IS '绑定SPU ID';
COMMENT ON COLUMN guorai_notes.spu_name IS '绑定SPU名称';
COMMENT ON COLUMN guorai_notes.tag IS '笔记标签原始文本';
COMMENT ON COLUMN guorai_notes.first_seen_at IS '首次抓取到该笔记的时间';
COMMENT ON COLUMN guorai_notes.last_seen_at IS '最近抓取到该笔记的时间';
COMMENT ON COLUMN guorai_notes.raw_dimension_payload IS '最近一次笔记基础信息原文';

COMMENT ON TABLE guorai_plans IS '薯量计划最新维度信息';
COMMENT ON COLUMN guorai_plans.plan_id IS '计划ID';
COMMENT ON COLUMN guorai_plans.plan_name IS '计划名称';
COMMENT ON COLUMN guorai_plans.plan_type IS '广告类型';
COMMENT ON COLUMN guorai_plans.plan_publish_time IS '计划创建时间，接口字段为 planPublishTime';
COMMENT ON COLUMN guorai_plans.account_name IS '计划所属账号';
COMMENT ON COLUMN guorai_plans.tag IS '计划标签原始文本';
COMMENT ON COLUMN guorai_plans.first_seen_at IS '首次抓取到该计划的时间';
COMMENT ON COLUMN guorai_plans.last_seen_at IS '最近抓取到该计划的时间';
COMMENT ON COLUMN guorai_plans.raw_dimension_payload IS '最近一次计划基础信息原文';

COMMENT ON TABLE guorai_plan_notes IS '薯量计划与笔记的当前关联关系';
COMMENT ON COLUMN guorai_plan_notes.plan_id IS '计划ID';
COMMENT ON COLUMN guorai_plan_notes.note_id IS '笔记ID';
COMMENT ON COLUMN guorai_plan_notes.first_seen_at IS '首次发现该关联的时间';
COMMENT ON COLUMN guorai_plan_notes.last_seen_at IS '最近发现该关联的时间';
COMMENT ON COLUMN guorai_plan_notes.is_active IS '最近一次计划快照中是否仍存在该关联';
COMMENT ON COLUMN guorai_plan_notes.raw_payload IS '计划内嵌笔记的完整原文';

COMMENT ON TABLE guorai_note_snapshots IS '薯量笔记 Rolling 窗口原始指标快照，不包含自定义计算';
COMMENT ON COLUMN guorai_note_snapshots.id IS '笔记快照主键';
COMMENT ON COLUMN guorai_note_snapshots.fetch_id IS '所属拉取批次ID';
COMMENT ON COLUMN guorai_note_snapshots.note_id IS '笔记ID';
COMMENT ON COLUMN guorai_note_snapshots.enterprise_id IS '薯量企业ID';
COMMENT ON COLUMN guorai_note_snapshots.xhs_brand_id IS '小红书品牌ID';
COMMENT ON COLUMN guorai_note_snapshots.merchant_id IS '归因店铺ID';
COMMENT ON COLUMN guorai_note_snapshots.snapshot_date IS 'Rolling 趋势快照日期';
COMMENT ON COLUMN guorai_note_snapshots.window_start IS '触达时间窗口开始日期，包含当天';
COMMENT ON COLUMN guorai_note_snapshots.window_end IS '触达时间窗口结束日期，包含当天';
COMMENT ON COLUMN guorai_note_snapshots.total_pay_user IS '平台原始估算总体付款人数';
COMMENT ON COLUMN guorai_note_snapshots.total_pay_amt IS '平台原始估算总体付款金额';
COMMENT ON COLUMN guorai_note_snapshots.total_pay_order IS '平台原始估算总体付款订单数';
COMMENT ON COLUMN guorai_note_snapshots.total_uroi IS '平台原始估算总体人均转化成本';
COMMENT ON COLUMN guorai_note_snapshots.total_oroi IS '平台原始估算总体订单转化成本';
COMMENT ON COLUMN guorai_note_snapshots.total_roi IS '平台原始估算总体转化ROI';
COMMENT ON COLUMN guorai_note_snapshots.consume IS '平台原始总消耗';
COMMENT ON COLUMN guorai_note_snapshots.note_consume IS '平台原始推广消费金额';
COMMENT ON COLUMN guorai_note_snapshots.note_ad_cost_volume IS '平台原始笔记合作金额';
COMMENT ON COLUMN guorai_note_snapshots.note_heat_consume IS '平台原始内容加热金额';
COMMENT ON COLUMN guorai_note_snapshots.exposure_count IS '平台原始曝光量';
COMMENT ON COLUMN guorai_note_snapshots.click_count IS '平台原始点击量';
COMMENT ON COLUMN guorai_note_snapshots.click_r IS '平台原始点击率';
COMMENT ON COLUMN guorai_note_snapshots.interact_count IS '平台原始互动量';
COMMENT ON COLUMN guorai_note_snapshots.interact_r IS '平台原始互动率';
COMMENT ON COLUMN guorai_note_snapshots.click_roi IS '平台原始估算点击转化率';
COMMENT ON COLUMN guorai_note_snapshots.part_pay_user IS '平台原始估算本品付款人数';
COMMENT ON COLUMN guorai_note_snapshots.part_pay_amt IS '平台原始估算本品付款金额';
COMMENT ON COLUMN guorai_note_snapshots.part_pay_order IS '平台原始估算本品付款订单数';
COMMENT ON COLUMN guorai_note_snapshots.part_pay_user_r IS '平台原始估算本品付款人数占比';
COMMENT ON COLUMN guorai_note_snapshots.part_pay_amt_r IS '平台原始估算本品付款金额占比';
COMMENT ON COLUMN guorai_note_snapshots.part_pay_order_r IS '平台原始估算本品付款订单数占比';
COMMENT ON COLUMN guorai_note_snapshots.part_uroi IS '平台原始估算本品人均转化成本';
COMMENT ON COLUMN guorai_note_snapshots.part_oroi IS '平台原始估算本品订单转化成本';
COMMENT ON COLUMN guorai_note_snapshots.part_roi IS '平台原始估算本品转化ROI';
COMMENT ON COLUMN guorai_note_snapshots.new_pay_user IS '平台原始估算新客付款人数';
COMMENT ON COLUMN guorai_note_snapshots.new_pay_amt IS '平台原始估算新客付款金额';
COMMENT ON COLUMN guorai_note_snapshots.new_pay_order IS '平台原始估算新客付款订单数';
COMMENT ON COLUMN guorai_note_snapshots.new_pay_user_r IS '平台原始估算新客付款人数占比';
COMMENT ON COLUMN guorai_note_snapshots.new_pay_amt_r IS '平台原始估算新客付款金额占比';
COMMENT ON COLUMN guorai_note_snapshots.new_pay_order_r IS '平台原始估算新客付款订单数占比';
COMMENT ON COLUMN guorai_note_snapshots.note_endorse_volume IS '平台原始点赞量';
COMMENT ON COLUMN guorai_note_snapshots.note_comment_volume IS '平台原始评论量';
COMMENT ON COLUMN guorai_note_snapshots.note_collect_volume IS '平台原始收藏量';
COMMENT ON COLUMN guorai_note_snapshots.note_share_volume IS '平台原始分享量';
COMMENT ON COLUMN guorai_note_snapshots.note_follow_volume IS '平台原始关注量';
COMMENT ON COLUMN guorai_note_snapshots.is_new IS '接口原始新增标识';
COMMENT ON COLUMN guorai_note_snapshots.raw_payload IS '该笔记在本次窗口中的完整接口原文';
COMMENT ON COLUMN guorai_note_snapshots.fetched_at IS '该笔记快照写入时间';

COMMENT ON TABLE guorai_plan_snapshots IS '薯量计划 Rolling 窗口原始指标快照，不包含自定义计算';
COMMENT ON COLUMN guorai_plan_snapshots.id IS '计划快照主键';
COMMENT ON COLUMN guorai_plan_snapshots.fetch_id IS '所属拉取批次ID';
COMMENT ON COLUMN guorai_plan_snapshots.plan_id IS '计划ID';
COMMENT ON COLUMN guorai_plan_snapshots.enterprise_id IS '薯量企业ID';
COMMENT ON COLUMN guorai_plan_snapshots.xhs_brand_id IS '小红书品牌ID';
COMMENT ON COLUMN guorai_plan_snapshots.merchant_id IS '归因店铺ID';
COMMENT ON COLUMN guorai_plan_snapshots.snapshot_date IS 'Rolling 趋势快照日期';
COMMENT ON COLUMN guorai_plan_snapshots.window_start IS '触达时间窗口开始日期，包含当天';
COMMENT ON COLUMN guorai_plan_snapshots.window_end IS '触达时间窗口结束日期，包含当天';
COMMENT ON COLUMN guorai_plan_snapshots.total_pay_user IS '平台原始估算总体付款人数';
COMMENT ON COLUMN guorai_plan_snapshots.total_pay_amt IS '平台原始估算总体付款金额';
COMMENT ON COLUMN guorai_plan_snapshots.total_pay_order IS '平台原始估算总体付款订单数';
COMMENT ON COLUMN guorai_plan_snapshots.total_uroi IS '平台原始估算总体人均转化成本';
COMMENT ON COLUMN guorai_plan_snapshots.total_oroi IS '平台原始估算总体订单转化成本';
COMMENT ON COLUMN guorai_plan_snapshots.total_roi IS '平台原始估算总体转化ROI';
COMMENT ON COLUMN guorai_plan_snapshots.part_pay_user IS '平台原始估算本品付款人数';
COMMENT ON COLUMN guorai_plan_snapshots.part_pay_amt IS '平台原始估算本品付款金额';
COMMENT ON COLUMN guorai_plan_snapshots.part_pay_order IS '平台原始估算本品付款订单数';
COMMENT ON COLUMN guorai_plan_snapshots.part_pay_user_r IS '平台原始估算本品付款人数占比';
COMMENT ON COLUMN guorai_plan_snapshots.part_pay_amt_r IS '平台原始估算本品付款金额占比';
COMMENT ON COLUMN guorai_plan_snapshots.part_pay_order_r IS '平台原始估算本品付款订单数占比';
COMMENT ON COLUMN guorai_plan_snapshots.part_uroi IS '平台原始估算本品人均转化成本';
COMMENT ON COLUMN guorai_plan_snapshots.part_oroi IS '平台原始估算本品订单转化成本';
COMMENT ON COLUMN guorai_plan_snapshots.part_roi IS '平台原始估算本品转化ROI';
COMMENT ON COLUMN guorai_plan_snapshots.new_pay_user IS '平台原始估算新客付款人数';
COMMENT ON COLUMN guorai_plan_snapshots.new_pay_amt IS '平台原始估算新客付款金额';
COMMENT ON COLUMN guorai_plan_snapshots.new_pay_order IS '平台原始估算新客付款订单数';
COMMENT ON COLUMN guorai_plan_snapshots.new_pay_user_r IS '平台原始估算新客付款人数占比';
COMMENT ON COLUMN guorai_plan_snapshots.new_pay_amt_r IS '平台原始估算新客付款金额占比';
COMMENT ON COLUMN guorai_plan_snapshots.new_pay_order_r IS '平台原始估算新客付款订单数占比';
COMMENT ON COLUMN guorai_plan_snapshots.note_ad_cost_volume IS '平台原始推广消费金额';
COMMENT ON COLUMN guorai_plan_snapshots.exposure_count IS '平台原始曝光量';
COMMENT ON COLUMN guorai_plan_snapshots.click_count IS '平台原始点击量';
COMMENT ON COLUMN guorai_plan_snapshots.interact_count IS '平台原始互动量';
COMMENT ON COLUMN guorai_plan_snapshots.click_r IS '平台原始点击率';
COMMENT ON COLUMN guorai_plan_snapshots.interact_r IS '平台原始互动率';
COMMENT ON COLUMN guorai_plan_snapshots.click_roi IS '平台原始估算点击转化率';
COMMENT ON COLUMN guorai_plan_snapshots.note_endorse_volume IS '平台原始点赞量';
COMMENT ON COLUMN guorai_plan_snapshots.note_comment_volume IS '平台原始评论量';
COMMENT ON COLUMN guorai_plan_snapshots.note_collect_volume IS '平台原始收藏量';
COMMENT ON COLUMN guorai_plan_snapshots.note_share_volume IS '平台原始分享量';
COMMENT ON COLUMN guorai_plan_snapshots.note_follow_volume IS '平台原始关注量';
COMMENT ON COLUMN guorai_plan_snapshots.is_new IS '接口原始新增标识';
COMMENT ON COLUMN guorai_plan_snapshots.raw_payload IS '该计划在本次窗口中的完整接口原文';
COMMENT ON COLUMN guorai_plan_snapshots.fetched_at IS '该计划快照写入时间';
