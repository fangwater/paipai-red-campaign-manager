# PaiPai RED Campaign Manager

PaiPai 小红书投放管理服务。飞书多维表格 Base、稿件正文和服务商电子表格只在显式调用本机 API 时同步到 PostgreSQL。

## 同步行为

- 每次调用 Base 同步接口时发现并分页同步全部数据表，无需逐个配置 `table_id`。
- 记录字段原样保存为 PostgreSQL `JSONB`。
- 使用 `app_token + table_id + record_id` 作为幂等主键。
- 飞书中消失的表和记录在 PostgreSQL 中标记为软删除。
- 从富文本字段提取飞书 Wiki、Docx 和腾讯文档链接。
- 飞书 Wiki 会解析为真实 Docx，并保存标题、版本和纯文本正文。
- 业务数据不使用定时器或启动同步；Base 正文仅在显式同步时按刷新间隔判断是否需要重新拉取。
- 每轮写入和删除在同一个 PostgreSQL 事务中执行。
- 进程内防重入，并使用 PostgreSQL advisory lock 防止多实例重复同步。
- `sync_runs` 保存每轮表、记录、文档和错误数量。
- 从服务商索引中发现已启用的飞书电子表格。
- 在“达人笔记执行表”中按表头名称匹配所需字段，不依赖固定列序。
- 从“稿件”单元格的飞书 Docx 链接拉取有序正文块和图片，并按笔记 ID 保存。
- 单元格存在多个链接时优先选择标注为“定稿、终稿、最终稿”的链接；文档内存在版本章节时仅保存最后一个定稿章节。
- 仅同步格式明确的 24 位十六进制笔记 ID，空值和占位文本整行跳过。
- 稿件按笔记 ID、文档资源键和抽取版本增量抓取；链接及抽取版本未变化时不重复请求，抓取失败的 ID 下轮重试。
- 图片下载后按 SHA-256 去重存入 PostgreSQL，不保存或暴露飞书临时 URL；源图最大 50 MB，超过 2 MB 或边长超过 2560px 的 JPEG/PNG 会先缩放压缩，数据库单图最多 10 MB、单篇优化后最多 100 MB。
- 服务商记录使用快照同步，飞书中消失的行在 PostgreSQL 中标记为软删除。
- “cid数据”从源页签“辅酶q10日数据”按日期和内容哈希增量写入；新增日期插入、历史修订更新、未变化记录跳过，源表暂缺日期不删除本地历史。

腾讯/企微文档不抓取正文。服务会保存其链接和 `auth_required` 状态，并将正文统一存为 `nan`。

## 数据表

- `lark_bitable_tables`：Base 数据表目录。
- `lark_bitable_records`：所有表的原始 JSONB 记录。
- `lark_linked_documents`：链接文档的标题、正文、版本和抓取状态。
- `lark_record_documents`：飞书记录字段与链接文档的多对多关系。
- `sync_runs`：同步运行历史。
- `service_provider_content_tables`：服务商内容执行表索引及最后同步状态。
- `service_provider_note_executions`：服务商达人笔记执行明细，仅保存指定字段。
- `service_provider_notes`：笔记 ID、定稿纯文本、有序内容块及来源版本。
- `manuscript_assets`：按内容哈希去重的稿件图片二进制。
- `service_provider_note_assets`：稿件与图片的有序关联。
- `coenzyme_q10_daily`：cid数据的 19 列日记录，日期为幂等主键。
- `coenzyme_q10_sync_runs`：cid数据每轮读取、插入、更新、未变化数量及失败原因。

## 环境要求

- Go 1.26+
- PostgreSQL 14+
- 飞书自建应用
- 多维表格读取权限
- 新版文档读取权限
- 电子表格读取权限
- 知识库读取权限
- 目标 Base、稿件文档、Wiki 和电子表格已向应用开放访问

## 配置

```bash
cp .env.example .env
```

必填项：

- `LARK_APP_ID`：飞书自建应用 ID
- `LARK_APP_SECRET`：飞书自建应用密钥
- `LARK_APP_TOKEN`：原多维表格 Base 的 App Token
- `LARK_DANDELION_APP_TOKEN`：包含“蒲公英数据”的 Base App Token
- `LARK_DANDELION_TABLE_ID`：“蒲公英数据”真实表 ID
- `DATABASE_URL`：本地 PostgreSQL 连接字符串

飞书手动同步配置：

- `LARK_SYNC_LISTEN`：本机 API 监听地址，默认 `127.0.0.1:18081`
- `SYNC_TIMEOUT`：一次显式同步请求的超时，默认 10 分钟
- `DOCUMENT_REFRESH_INTERVAL`：调用 Base 同步时，已抓取正文的重新拉取间隔，默认 1 小时

- `LARK_COENZYME_Q10_WIKI_TOKEN`：cid数据源日报所在 Wiki 节点，默认当前“脉拓辅酶日报表”。
- `LARK_COENZYME_Q10_SHEET_ID`：日报页签 ID，默认 `a961f7`。
- `LARK_COENZYME_Q10_SHEET_NAME`：日报页签名，默认“辅酶q10日数据”；同步时名称优先于 ID。
稿件向量配置：

- `BAILIAN_API_KEY`：百炼 Workspace API Key，也可使用 `DASHSCOPE_API_KEY`
- `BAILIAN_BASE_URL`：Workspace 的 OpenAI 兼容 Base URL
- `BAILIAN_EMBEDDING_MODEL`：默认 `qwen3.7-text-embedding`
- `BAILIAN_EMBEDDING_DIMENSIONS`：默认 1024

聚光 OpenAPI 配置：

- `XHS_JG_APP_ID`：聚光开放平台应用 ID
- `XHS_JG_SECRET`：聚光开放平台应用 Secret
- `XHS_JG_SESSION_FILE`：OAuth Token 会话文件，默认 `.xhs-jg/session.json`
- `XHS_JG_AUTHD_URL`：投流服务访问 Token Broker 的环回地址，默认 `http://127.0.0.1:18080`
- `XHS_JG_INTERNAL_API_KEY`：Token Broker 内部网关密钥，使用至少 32 字符的随机值
- `DELIVERY_API_CREDENTIALS_JSON`：可选。脚本或独立审批身份的 API Key 到固定 `actor`、`role` 和广告主范围的服务端绑定；浏览器控制台使用共享 `delivery-console/operator` 身份直接进入
- `DELIVERY_MEDIA_WRITES_ENABLED`：聚光写入总开关；生产已开启，信息流/搜索页可真实启停计划
- `DELIVERY_LLM_BASE_URL`、`DELIVERY_LLM_MODEL`：可选 OpenAI-compatible 语义服务；缺失时使用确定性规则降级
- `DELIVERY_RANKER_URL`、`DELIVERY_RANKER_API_KEY`、`DELIVERY_RANKER_MODEL`：可选 LightGBM/LambdaMART 推理服务；缺失时使用可解释启发式排序

## 初始化数据库

```bash
sudo -u postgres createdb -O "$USER" paipai_red
```

当前机器已创建由 `ubuntu` 角色拥有的 `paipai_red` 数据库。示例连接串通过本地 Unix socket 使用 peer 认证，不需要数据库密码。

服务启动时自动执行 `migrations/` 中全部幂等迁移。

## 前端中台

前端位于 `frontend/`，使用 React、TypeScript 和 Vite。Maituo 客户日报模块支持一次选择或拖放多个 `.xlsx` 文件，本地解析后按报表日期升序执行，并展示服务器中已保存的报表日期和文件状态。
蒲公英数据更新页会按文件内最大的“数据更新日期”展示历史上传，并补齐首尾上传日期间的日历：周五、周六作为非工作日默认缺省，其他未上传日期标记为缺少文件。
历史列表中的已保存日期可打开该 `report_date` 对应的合并后笔记明细。明细只保留笔记和场域维度，不展示或推断子账户、广告账户和计划归属。

```bash
make frontend-dev
make frontend-build
```

开发服务默认运行在 `http://localhost:5173/paipai/`。生产入口使用 `https://pangutech.online/paipai/`，直接复用根域名现有 DNS 和 HTTPS 证书。构建产物部署至 `/var/www/paipai`，Nginx snippet 位于 `deploy/nginx/paipai-console.conf`。生产站点公开健康检查和固定用途的 Excel 导入入口，其他本机同步 API 仍不可访问。

```bash
make frontend-deploy
```

## Maituo 客户日报导入 API

`POST /v1/imports/maituo-customer-daily` 接收 `multipart/form-data`，唯一字段 `file` 为不超过 50 MB 的 `.xlsx`。前端多选后按日期逐个调用该接口。`GET /v1/imports/maituo-customer-daily` 返回按报表日期倒序排列的已保存文件；传入 `report_date=YYYY-MM-DD` 时返回该日期按正式业务键合并后的笔记明细。

系统识别以下 5 张目标表。工作簿至少包含其中一张即可；缺少的目标表会跳过且不会修改该表已有数据，其他未知工作表会被忽略。实际存在的目标表，其表名和表头必须与样本一致：

- `总览KPI`，业务键为 `报表日期 + 指标`
- `笔记明细`，业务键为 `report_date + note_id + placement`（报表日期 + 笔记ID + 场域）
- `分SPU总览`，业务键为 `报表日期 + SPU`
- `分子账户`，业务键为 `报表日期 + SPU + 子账户 + 场域`
- `淘搜趋势`，业务键为表内 `日期`

“笔记明细”中的同日、同笔记、同场域行会合并保存，正式数据不保留子账户或计划名。“分子账户”是工作簿提供的另一套独立汇总，表内没有可验证的笔记归属，不能用它把笔记反推到子账户、计划或 SPU。

报表日期优先从文件名中的 `YYYY-MM-DD` 提取；文件名没有日期时，使用 `淘搜趋势` 的最大日期。缺少 `淘搜趋势` 且文件名也没有日期时无法导入。成功导入过的文件 SHA-256 会返回 `already_saved=true`，避免重复写入。

前四张业务表按报表日期独立保存：同一日期重复上传时仅更新差异，不同日期互不覆盖。`淘搜趋势` 不使用工作簿报表日期，而是按表内日期维护一套趋势序列；补传早于当前最新报表日期的历史文件时不会回退趋势。每个文件只处理实际存在的目标表，解析、比较和写入位于同一个事务中。GET 结果中的 `present_sheets` 和 `missing_sheets` 用于前端展示表覆盖情况。历史列表会补齐日期序列：周五、周六识别为业务周末并标记“无需日报”，其他缺失日期标记为“缺少报表”。

## 蒲公英 Excel 导入 API

`POST /v1/imports/dandelion-excel` 接收不超过 50 MB 的 `.xlsx`，以文件内最大的“数据更新日期”作为本次数据日期。`GET /v1/imports/dandelion-excel` 按数据日期倒序返回每个日期最新一次成功上传，供前端展示历史上传与日期缺口。

## 笔记场域分析 API

`GET /v1/analytics/maituo/note-campaigns` 按 `笔记ID + 场域` 聚合笔记明细。公网入口为 `/paipai/api/analytics/maituo/note-campaigns`；路径中的 `campaigns` 仅为兼容旧客户端保留，不表示日报存在计划维度。查询参数：

- `window`：`3d`、`7d` 或 `all`，默认 `7d`；3D/7D 取最近实际存在的报表日，周五、周六自然跳过
- `q`：按笔记ID或场域模糊搜索
- `sort`：`daily_spend` 按最新报表日消耗排序，`cumulative_spend` 按所选范围累计消耗排序，`search_cost_change` 按“最新报表日回搜成本 - 上一实际报表日回搜成本”排序，默认 `cumulative_spend`
- `page`、`page_size`：分页参数，每页最多 100 个笔记场域组合

每个笔记场域组合返回所选报表日内的累计消耗、累计回搜人数、最新回搜成本及其较上一实际报表日的差值，以及逐日报表中的当天回搜成本。某日报日未投放时补零日增量，使累计曲线保持水平；回搜成本不累加，差值计算同样将未投放日视为 0。仅有一个报表日时差值为 0。分析结果不返回子账户或日报计划归因。

## 内容分析 API

`GET /v1/analytics/content-analysis` 汇总蒲公英、Maituo 客户日报、薯量笔记快照和服务商稿件标签。公网入口为 `/paipai/api/analytics/content-analysis`，前端页面为 `/paipai/content-analysis`。查询参数：

- `spu`：`辅酶` 或 `磷虾油`，默认 `辅酶`
- `agency`：`全部`、`曼杰`、`有一有二` 或 `智元`，默认 `全部`
- `dimension`：`audience`（人群标签）或 `scenario`（用户场景），默认 `audience`
- `published_start_date`、`published_end_date`：可选的笔记发布时间起止日期，格式为 `YYYY-MM-DD`，包含边界日期；仅传一端时按开放区间筛选

机构仅包含以下蒲公英下单账号映射；`全部`也是这三家机构的合集：

- 江苏拾光宝盒信息技术有限公司 → 曼杰
- 上海有一有二网络技术有限公司 → 有一有二
- 杭州智元文化传播有限公司 → 智元

热力图以内容类型为行，以选定的人群标签或用户场景为列。爆文率分母只计入蒲公英“站外活跃成本（15天设备归因）”大于 0 的笔记，成本不高于 20 判定为爆文。投流按笔记和场域汇总所有已保存日报：单场域累计消耗不少于 200，且搜索回搜成本不高于 30 或信息流预计回流后成本不高于 70，即判定投流达标。薯量最新笔记快照 `total_roi` 不低于 1.2 判定 ROI 达标。

接口同时返回标签覆盖率、各指标可评估样本数、单元格聚合数据和笔记明细。每篇笔记的 `search_campaigns` / `feed_campaigns` 直接通过聚光创意 `note_id` 和计划 `placement` 关联当前未删除的聚光计划，返回广告主、计划 ID、名称、状态、开关和同步时间；数组不包含也不推断计划级日报消耗。信息流和搜索页据此展示并启停真实聚光计划，消耗、成本及停投筛选仍保持 Maituo 日报的“笔记 + 场域”口径。无标签笔记保留为“未标注”，前端默认隐藏并可手动显示；设置发布时间范围后，无有效发布时间的笔记不参与汇总。

## 自建投流后端 API

中台 `/paipai/self-serve-delivery` 直接使用共享 `delivery-console/operator` 身份进入聚光自建投流工作台，不要求浏览器输入 API Key。接口根目录为 `/v1/delivery`，公网代理为 `/paipai/api/delivery`，完整机器可读契约位于 `/paipai/api/delivery/openapi.json`。脚本可选用 `X-Delivery-API-Key` 切换到服务端绑定的独立身份；客户端伪造的身份头不会生效。媒体写入总开关、确定性校验和双人审批规则不受直通模式影响。

当前后端包括：

- 能力、资产与工具：广告主能力快照、本地/平台资产、定向字典、关键词与否词、人群预估，以及计划、单元、创意只读查询。
- 版本化工作流：草稿创建/列表/修订、结构化建议、确定性与上游校验、运营和预算责任人双审批。
- 发布编排：`dry_run` 或异步 `execute`，固定按计划、单元、否词、创意执行；计划强制暂停，三层创建后读回 ID、父子关系、状态和关键字段。
- 状态与报表：计划状态更新（开启、暂停、删除，最多 20 个计划 ID）、已映射实体人工启停，以及账户、计划、单元、创意、关键词五层实时/离线报表和原始快照。
- 审计与隔离：草稿哈希、审批历史、幂等作业、媒体 ID 映射、脱敏上游调用记录、广告主级 RBAC 和双重媒体写开关。

算法职责固定分离：LLM 只抽取语义、候选词和证据；LightGBM/LambdaMART 只对已批准的数值特征排序；贝叶斯接口只估计稀疏分群后验与不确定性；约束优化只在人工上限内返回 `executable=false` 的预算建议；Bandit 只返回 `shadow_only=true` 的影子选择。平台枚举、权限、预算、审批、发布和启停始终由确定性规则、编排器和人工角色决定。

当前生产 OAuth 已包含 `ad_manage`、`ad_query`、`report_service` 和 `account_manage`，并授权 59 个广告主。`DELIVERY_MEDIA_WRITES_ENABLED=true`，信息流/搜索页的一键暂停和双击改状态会真实调用聚光启停接口。

## 子账户汇总诊断 API

`GET /v1/analytics/maituo/account-plan-diagnosis` 返回日报的子账户汇总诊断，公网入口为 `/paipai/api/analytics/maituo/account-plan-diagnosis`，前端页面为 `/paipai/account-plan-diagnosis`。接口和页面路径为兼容旧客户端保留；可选参数 `spu` 默认取 `辅酶`。

诊断只读取“分子账户”独立汇总表，统一使用 KPI 70；搜索使用回搜成本，信息流使用预计回流后成本。接口按“子账户 + 场域”返回诊断数据，并按子账户汇总最近 30 个自然日的搜索与信息流趋势。前端可选择子账户及 7/14/30 日周期，展示总消耗、搜索消耗与回搜成本、信息流消耗与预计回流后成本、搜索 CPC 与 CTR、信息流 CPC 与 CTR、搜索与信息流回搜率六张图；7 日模式显示点位数值。

“分子账户”行没有笔记 ID，正式“笔记明细”行也没有子账户或计划名，因此两表之间不存在可验证的归因键，不能通过日期、场域、SPU 或金额把笔记反推到子账户或计划。数据库视图 `maituo_customer_daily_search_user_overlap` 只保留分子账户与 SPU 汇总之间可直接计算的指标；为兼容旧读取方而保留的笔记重叠字段不再计算并返回 `NULL`。诊断接口中的 `cost` 和兼容字段 `original_cost` 均为分子账户日报的原始成本，`correction_coefficient` 返回 `null`，KPI 状态也只按该原始成本判断。

数据总览接口的 `cid` 区块独立读取 `coenzyme_q10_daily`，以 CID 最新数据日为终点返回所选 7/14/30 个自然日的 `spend` 和 `coenzyme_roi`，缺失日期保留为空值。前端“cid数据 · 辅酶”使用左右双轴折线图按日期展示消耗和辅酶成交ROI，不随总览的 SPU 选择变化。

## 投流情况对比 API

`GET /v1/analytics/maituo/traffic-comparisons` 是为旧客户端保留的兼容入口。公网入口为 `/paipai/api/analytics/maituo/traffic-comparisons`，前端页面为 `/paipai/traffic-comparison`。查询参数：

- `window`：`3d`、`7d` 或 `all`，默认 `7d`；用于笔记场域趋势和区间指标
- `q`：按笔记ID或场域模糊搜索
- `page`、`page_size`：分页参数，每页最多 100 个笔记场域组合

接口只返回可用报表日期，不再返回计划对比项。日报只能提供同一笔记、同一场域的合并指标，不提供账户或计划拆分，也不能将合并消耗、回搜人数或成本分摊给某个计划。

`GET /v1/analytics/maituo/traffic-comparison-delivery?note_id=...&placement=信息流|搜索` 返回选中笔记场域可从聚光/XHS 实体关系中查到的计划、单元及投放配置，公网入口为 `/paipai/api/analytics/maituo/traffic-comparison-delivery`。这些计划关系和配置来自 XHS 实体表，不是 Maituo 日报的计划归因；接口不得把日报合并指标拆到计划上。


## 飞书手动同步 API

构建并启动本机 API：

```bash
make lark-sync-start
```

服务由 PM2 运行在 `127.0.0.1:18081`，API 进程自身不会定时同步或在启动时同步。接口同步返回结果：`200 OK` 表示数据库写入已经完成，已有同类作业运行时返回 `409 Conflict`。

- `GET /healthz`：进程健康状态。
- `GET /v1/manuscript-assets/{sha256}`：读取仍被稿件引用的图片，支持 ETag 和私有缓存。
- `POST /v1/sync/manuscripts`：同步服务商稿件表。
- `GET /v1/sync/manuscripts/status`：查询三张稿件表最近的持久化同步状态。
- `POST /v1/sync/dandelion`：只同步配置 Base 内的“蒲公英数据”表。
- `GET /v1/sync/dandelion/status`：查询蒲公英最近 10 次持久化同步结果。
- `POST /v1/sync/cid`：按日期增量同步 cid数据。
- `GET /v1/sync/cid/status`：查询 cid数据当前日期范围和最近 10 次持久化同步结果。

前端入口为 `/paipai/data-sync/cid`；原 `coenzyme-q10` 前端和 API 路径保留兼容。公网只开放各目标固定的状态查询与显式同步触发路径；飞书凭据、Token 与源表地址均不返回前端。

稿件请求体为空或 `{}` 时同步全部三个服务商：`manjie`（曼杰）、`youyiyouer`（有一有二）、`zhiyuan`（智元）：

```bash
make lark-sync-manuscripts
make lark-sync-status
```

只同步指定稿件表：

```bash
curl -sS http://127.0.0.1:18081/v1/sync/manuscripts \
  -H "Content-Type: application/json" \
  --data "{\"provider_codes\":[\"manjie\",\"zhiyuan\"]}"
```

请求中的服务商代码会在开始写入前完整校验。未知或未启用代码返回 `400 Bad Request`，不会只执行部分有效目标。

“蒲公英数据”是独立目标，只读取配置的单表，不会扫描同 Base 的其他数据表：

```bash
make lark-sync-dandelion
```

该目标当前对应 Base `ULhXbXkAGaiNARsahfcclBX4nWe` 内的 `tbl3djNUVT4WANi0`，接口成功结果中的 `tables` 固定为 `1`。记录使用 `app_token + table_id + record_id` 与其他飞书目标隔离。

“cid数据”可手动同步，也可安装每天上海时间 11:00 的 systemd timer：

```bash
make lark-sync-cid
make lark-sync-cid-daily-install
```

timer 使用 `OnCalendar=*-*-* 11:00:00 Asia/Shanghai` 和 `Persistent=true`；主机在 11:00 停机时会在恢复后补跑。同步失败后每 10 分钟重试，2 小时内最多启动 3 次。

立即执行、查看下次触发时间和日志：

```bash
make lark-sync-cid-daily-now
make lark-sync-cid-daily-status
make lark-sync-cid-daily-logs
```

也可直接确认 timer 日历：

```bash
systemctl list-timers paipai-coenzyme-q10-sync.timer --all
```

列出 Base 内全部数据表：

```bash
set -a
source .env
set +a
go run ./cmd/list-tables
```

停止飞书手动同步服务：

```bash
make lark-sync-stop
```

## 稿件向量

首次补齐或手动增量刷新稿件向量：

```bash
make embeddings-refresh
```

刷新按正文 SHA-256、模型和维度判断变化，未变化的稿件不会调用模型。需要全部重算时使用：

```bash
make embeddings-force-refresh
```

每次 `POST /v1/sync/manuscripts` 成功后会自动运行增量刷新。向量保存在 PostgreSQL 的 `service_provider_note_embeddings`，运行记录保存在 `service_provider_note_embedding_runs`。

## 验证

```bash
go test ./...
go vet ./...
go build ./cmd/sync
```

查询各表有效记录数：

```sql
SELECT tables.name, COUNT(*) AS records
FROM lark_bitable_records AS records
JOIN lark_bitable_tables AS tables
  ON tables.app_token = records.app_token
 AND tables.table_id = records.table_id
WHERE records.deleted_at IS NULL
GROUP BY tables.name
ORDER BY tables.name;
```

查询稿件正文：

```sql
SELECT tables.name, refs.field_name, documents.title, documents.content
FROM lark_record_documents AS refs
JOIN lark_bitable_tables AS tables
  ON tables.app_token = refs.app_token
 AND tables.table_id = refs.table_id
JOIN lark_linked_documents AS documents
  ON documents.provider = refs.provider
 AND documents.resource_key = refs.resource_key
WHERE documents.fetch_status = 'succeeded';
```

查询正文抓取状态：

```sql
SELECT provider, fetch_status, COUNT(*)
FROM lark_linked_documents
GROUP BY provider, fetch_status
ORDER BY provider, fetch_status;
```

查询服务商索引和有效笔记数：

```sql
SELECT tables.provider_name, tables.enabled, tables.last_sync_status,
       tables.last_synced_at, COUNT(records.*) AS active_records
FROM service_provider_content_tables AS tables
LEFT JOIN service_provider_note_executions AS records
  ON records.provider_code = tables.provider_code
 AND records.deleted_at IS NULL
GROUP BY tables.provider_code
ORDER BY tables.provider_name;
```

## 小红书聚光 OpenAPI

应用 ID 和 Secret 只保存在已被 Git 忽略的 `.env` 中。独立二进制 `bin/xhs-jg-authd` 负责授权、Token 持久化、本机 HTTP 服务和 Token 自动续期。

构建并交给 PM2 后台运行：

```bash
make xhs-authd-start
```

服务允许在没有 OAuth 会话时启动。取得十分钟内有效的 `auth_code` 后，用同一个二进制隐藏输入并提交一次授权：

```bash
make xhs-authd-authorize
```

授权成功后，会话保存到 `.xhs-jg/session.json`。服务默认在 access token 到期前 10 分钟刷新，失败后每分钟重试；刷新成功才会原子保存小红书返回的新 access token 和 refresh token。查看状态和日志：

```bash
make xhs-authd-status
make xhs-authd-logs
```

HTTP 服务仅监听 `127.0.0.1:18080`：

- `GET /healthz`：进程健康状态。
- `GET /readyz`：是否已有可用 access token。
- `GET /v1/oauth/status`：授权账户、广告主和过期时间，不包含 Token。
- `POST /v1/oauth/authorize`：提交首次或重新授权的 `auth_code`。
- `POST /v1/oauth/refresh`：立即触发一次 Token 刷新。
- `POST /v1/campaigns/list`：查询一页聚光推广计划。
- `POST /v1/campaigns/all`：以每页 100 条自动翻页，返回全部匹配的推广计划。
- `POST /v1/units/list`：查询一页聚光单元；分页参数为顶层 `page`、`page_size`。
- `POST /v1/units/all`：从第 1 页自动翻页，返回全部匹配的单元。
- `POST /v1/creativities/list`：查询一页聚光创意。
- `POST /v1/creativities/all`：从第 1 页自动翻页，返回全部匹配的创意。
- `GET /v1/sync/status`：当前手动作业和最近 10 次运行记录。
- `POST /v1/sync/campaigns`：显式刷新推广计划；默认增量，也支持完整刷新。
- `POST /v1/sync/units`：显式刷新推广单元；默认增量，也支持完整刷新。
- `POST /v1/sync/creativities`：显式完整刷新推广创意。

前端入口为 `/paipai/xhs-jg-sync/campaigns`、`/paipai/xhs-jg-sync/units` 和 `/paipai/xhs-jg-sync/creativities`。公网只开放同步状态与三个显式触发路径，不开放 Token、授权或原始查询接口。

控制接口不返回原始 Token，也不允许监听非回环地址。聚光业务接口由该服务内部取得有效 access token 后代为请求，不向调用方暴露 Token。

查询全部未删除计划（`status=6`）：

```bash
curl -sS http://127.0.0.1:18080/v1/campaigns/all \
  -H 'Content-Type: application/json' \
  --data '{"advertiser_id":123,"status":6}'
```

单页接口支持 `campaign_ids`、创建日期、更新日期和状态过滤。上游实际分页字段使用 `page_index`、`page_size`；本地接口也兼容官方示例中的 camelCase 写法：

```json
{
  "advertiser_id": 123,
  "update_start_date": "2026-07-01",
  "update_end_date": "2026-07-21",
  "page": {"page_index": 1, "page_size": 100}
}
```

`/v1/campaigns/all` 会忽略传入的分页值并从第 1 页开始拉取。它返回所有符合过滤条件的计划；若要明确排除已删除计划，请传 `status=6`。上游接口为 `POST https://adapi.xiaohongshu.com/api/open/jg/campaign/list`，参见[小红书官方“查询计划”文档](https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3150)。

查询广告主下的全部单元不需要逐计划请求，因为 `campaign_id` 是可选参数：

```bash
curl -sS http://127.0.0.1:18080/v1/units/all \
  -H 'Content-Type: application/json' \
  --data '{"advertiser_id":123}'
```

单元查询接口的请求状态只支持 `1=投放中`、`2=暂停`，没有“全部未删除”的请求状态。结构同步因此不传 `status` 拉取全部结果，再排除返回值中 `unit_filter_state=1` 的已删除单元。上游接口为 `POST https://adapi.xiaohongshu.com/api/open/jg/unit/list`，参见[小红书官方“获取单元列表接口”文档](https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3044)。

查询全部未删除创意（`status=2`）：

```bash
curl -sS http://127.0.0.1:18080/v1/creativities/all \
  -H 'Content-Type: application/json' \
  --data '{"advertiser_id":123,"status":2}'
```

创意上游接口为 `POST https://adapi.xiaohongshu.com/api/open/jg/creativity/search`，分页位于 `page.page_index`、`page.page_size`，参见[小红书官方“创意查询”文档](https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3158)。

计划、单元和创意支持自动与显式刷新。三个手动 Make 命令分别刷新全部授权广告主，计划和单元默认使用增量模式，创意使用完整模式：

```bash
make xhs-sync-campaigns
make xhs-sync-units
make xhs-sync-creativities
make xhs-sync-status
```

每日生产任务在 03:30（Asia/Shanghai）依次完整刷新计划、单元和创意，只有上一步成功后才会继续。安装后可立即执行、查看 timer 状态和日志：

```bash
make xhs-sync-daily
make xhs-sync-daily-install
make xhs-sync-daily-now
make xhs-sync-daily-status
make xhs-sync-daily-logs
```

`sync-daily` 会轮询每个运行记录直到完成，任一目标失败都会以非零状态退出。systemd timer 使用 `Persistent=true`，失败后每 10 分钟重试，最多 3 次。

HTTP 请求体为空或 `{}` 时处理全部授权广告主；传 `advertiser_id` 时只处理指定的已授权广告主。计划和单元的 `mode` 可选 `incremental` 或 `full`，省略时默认为 `incremental`：

```bash
curl -sS http://127.0.0.1:18080/v1/sync/campaigns \
  -H "Content-Type: application/json" \
  --data "{\"advertiser_id\":123,\"mode\":\"incremental\"}"

curl -sS http://127.0.0.1:18080/v1/sync/units \
  -H "Content-Type: application/json" \
  --data "{\"mode\":\"full\"}"

curl -sS http://127.0.0.1:18080/v1/sync/creativities \
  -H "Content-Type: application/json" \
  --data "{\"advertiser_id\":123}"

curl -sS http://127.0.0.1:18080/v1/sync/status
```

触发接口返回 `202 Accepted` 和包含 `target` 的运行记录，作业在后台继续执行。同一时刻只允许一个目标运行，已有作业时返回 `409 Conflict`。状态持久化到 `xhs_jg_sync_runs`，服务重启后仍可查询最近结果。

计划和单元各自维护独立增量游标，使用更新时间窗口并额外重叠一天以覆盖日期粒度边界。增量模式只 upsert 本次返回记录，不删除窗口外数据；完整模式核对全部未删除记录，并将本次结果中缺失的数据设置 `deleted_at`。官方创意接口没有更新时间过滤，因此创意只支持 `full`，传 `incremental` 会返回 `400 Bad Request`。

同步结果写入 `xhs_jg_advertisers`、`xhs_jg_campaigns`、`xhs_jg_units` 和 `xhs_jg_creativities`。三个业务表均保留稳定检索字段和完整 `raw_payload`，通过 `advertiser_id`、`campaign_id`、`unit_id` 关联。旧的 `go run ./cmd/xhs-jg-campaign-sync` 命令仅用于一次性排查。


仍可使用一次性 CLI 完成初始授权或手动刷新：

```bash
make xhs-token
make xhs-refresh
```

OAuth 接口：

- 获取：`POST https://adapi.xiaohongshu.com/api/open/oauth2/access_token`
- 刷新：`POST https://adapi.xiaohongshu.com/api/open/oauth2/refresh_token`

## 薯量笔记/计划触达查询与导出

`cmd/guorai` 复用薯量网页使用的认证和数据接口，同时支持“我的关注笔记”和“我的关注计划”。默认会话文件为 `.guorai/session.json`，目录已加入 `.gitignore`，文件权限为 `0600`。登录成功后，账号和密码同时保存到 PostgreSQL 的 `guorai_credentials` 表，两个列表共用这一套登录状态。

首次登录：

```bash
make guorai-login
```

在 `.env` 中配置 `GUORAI_USERNAME` 和品牌绑定店铺的 `GUORAI_MERCHANT_ID`。登录命令会隐藏密码输入、更新会话文件并把验证通过的账号密码写入数据库。若只想更新 Cookie，可传 `--store-credentials=false`。`GUORAI_PASSWORD` 仍只用于临时非交互输入，无需写进 `.env`。

按笔记触达时间查询并输出 JSON（`note` 是默认类型）：

```bash
go run ./cmd/guorai query --type note --from 2026-07-09 --to 2026-07-16 --output notes.json
```

按计划触达时间查询并输出 CSV：

```bash
go run ./cmd/guorai query --type plan --from 2026-07-09 --to 2026-07-16 --format csv --output plans.csv
```

创建后台笔记导出任务，等待完成并下载 Excel：

```bash
go run ./cmd/guorai export --type note --from 2026-07-09 --to 2026-07-16 --output notes.xlsx
```

导出关注计划：

```bash
go run ./cmd/guorai export --type plan --from 2026-07-09 --to 2026-07-16 --output plans.xlsx
```

省略 `--from` 和 `--to` 时，使用系统统计截止日期及其前 6 天（含首尾共 7 天）。薯量限制单次查询或导出最多 90 天，且结束日期不能晚于当前统计截止日期。Cookie 失效时重新运行 `login` 即可。

### Rolling 快照入库

`guorai sync` 默认只刷新平台当前截止日的一份最新快照，同时处理笔记和计划。查询会使用 `GUORAI_MERCHANT_ID` 指定店铺，否则薯量仅返回关注列表维度而不返回投放指标。笔记和计划快照均查询含首尾共 14 天的触达窗口，并保存平台原始 JSON、类型化原始指标、最新维度和计划-笔记关系。

```bash
set -a; . ./.env; set +a
go run ./cmd/guorai sync
```

等价的显式参数：

```bash
go run ./cmd/guorai sync --type all --days 1 --note-window-days 14 --plan-window-days 14 --timeout 30m
```

可使用 `--type note` 或 `--type plan` 只同步一种数据，使用 `--as-of YYYY-MM-DD` 回刷指定截止日期。默认以平台统计截止日期为最新快照日。`--window-days N` 可在临时回刷时统一覆盖笔记和计划窗口。

手动执行同一套生产参数：

```bash
make guorai-sync
```

安装每天 09:00（Asia/Shanghai）运行的 systemd timer：

```bash
make guorai-sync-install
```

安装后也可以人工触发、查看状态和日志：

```bash
make guorai-sync-now
make guorai-sync-status
make guorai-sync-logs
```

自动与手动同步共用 PostgreSQL 全局锁，不会并行执行。同步发现 Cookie 失效时，会使用 `guorai_credentials` 中的账号密码自动登录并重试当前操作一次。systemd timer 使用 `Persistent=true`，机器错过计划时间后会在恢复时补跑。

PostgreSQL 表：

- `guorai_fetch_runs`：拉取批次、窗口、归因配置、请求和合并后的原始响应。
- `guorai_credentials`：自动续登录使用的单账户账号和密码。
- `guorai_notes` / `guorai_plans`：最新维度信息。
- `guorai_plan_notes`：计划与笔记当前关系。
- `guorai_note_snapshots` / `guorai_plan_snapshots`：追加式 Rolling 原始指标快照。

所有表和字段均包含 PostgreSQL 中文注释。重复执行会新增抓取批次和原始快照，不覆盖历史；维度表更新为最近一次看到的信息。
