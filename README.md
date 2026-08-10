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

稿件向量配置：

- `BAILIAN_API_KEY`：百炼 Workspace API Key，也可使用 `DASHSCOPE_API_KEY`
- `BAILIAN_BASE_URL`：Workspace 的 OpenAI 兼容 Base URL
- `BAILIAN_EMBEDDING_MODEL`：默认 `qwen3.7-text-embedding`
- `BAILIAN_EMBEDDING_DIMENSIONS`：默认 1024

聚光 OpenAPI 配置：

- `XHS_JG_APP_ID`：聚光开放平台应用 ID
- `XHS_JG_SECRET`：聚光开放平台应用 Secret
- `XHS_JG_SESSION_FILE`：OAuth Token 会话文件，默认 `.xhs-jg/session.json`

## 初始化数据库

```bash
sudo -u postgres createdb -O "$USER" paipai_red
```

当前机器已创建由 `ubuntu` 角色拥有的 `paipai_red` 数据库。示例连接串通过本地 Unix socket 使用 peer 认证，不需要数据库密码。

服务启动时自动执行 `migrations/` 中全部幂等迁移。

## 前端中台

前端位于 `frontend/`，使用 React、TypeScript 和 Vite。Maituo 客户日报模块支持一次选择或拖放多个 `.xlsx` 文件，本地解析后按报表日期升序执行，并展示服务器中已保存的报表日期和文件状态。
蒲公英数据更新页会按文件内最大的“数据更新日期”展示历史上传，并补齐首尾上传日期间的日历：周五、周六作为非工作日默认缺省，其他未上传日期标记为缺少文件。
系统会为每个非“总体”子账户生成独立文件目录 URL。目录按日期倒序列出该子账户的历史日报，每个日期可单独下载一个只含“笔记明细”和“分子账户”的 Excel；不同子账户不会出现在同一文件中。
目录 URL 使用可逆的子账户标识，不提供身份鉴权；如需限制访问者查看其他已知子账户，应在网关层增加登录认证。

```bash
make frontend-dev
make frontend-build
```

开发服务默认运行在 `http://localhost:5173/paipai/`。生产入口使用 `https://pangutech.online/paipai/`，直接复用根域名现有 DNS 和 HTTPS 证书。构建产物部署至 `/var/www/paipai`，Nginx snippet 位于 `deploy/nginx/paipai-console.conf`。生产站点公开健康检查和固定用途的 Excel 导入入口，其他本机同步 API 仍不可访问。

```bash
make frontend-deploy
```

## Maituo 客户日报导入 API

`POST /v1/imports/maituo-customer-daily` 接收 `multipart/form-data`，唯一字段 `file` 为不超过 50 MB 的 `.xlsx`。前端多选后按日期逐个调用该接口。`GET /v1/imports/maituo-customer-daily` 返回按报表日期倒序排列的已保存文件。`GET /v1/imports/maituo-subaccount-directories` 返回子账户目录；`GET /v1/downloads/maituo-subaccount/{account_id}` 列出该账户的历史日期，追加 `/{YYYY-MM-DD}.xlsx` 下载单日拆分文件。

系统识别以下 5 张目标表。工作簿至少包含其中一张即可；缺少的目标表会跳过且不会修改该表已有数据，其他未知工作表会被忽略。实际存在的目标表，其表名和表头必须与样本一致：

- `总览KPI`，业务键为 `报表日期 + 指标`
- `笔记明细`，业务键为 `报表日期 + 笔记ID + 子账户 + 计划名 + 场域`
- `分SPU总览`，业务键为 `报表日期 + SPU`
- `分子账户`，业务键为 `报表日期 + SPU + 子账户 + 场域`
- `淘搜趋势`，业务键为表内 `日期`

报表日期优先从文件名中的 `YYYY-MM-DD` 提取；文件名没有日期时，使用 `淘搜趋势` 的最大日期。缺少 `淘搜趋势` 且文件名也没有日期时无法导入。成功导入过的文件 SHA-256 会返回 `already_saved=true`，避免重复写入。

前四张业务表按报表日期独立保存：同一日期重复上传时仅更新差异，不同日期互不覆盖。`淘搜趋势` 不使用工作簿报表日期，而是按表内日期维护一套趋势序列；补传早于当前最新报表日期的历史文件时不会回退趋势。每个文件只处理实际存在的目标表，解析、比较和写入位于同一个事务中。GET 结果中的 `present_sheets` 和 `missing_sheets` 用于前端展示表覆盖情况。历史列表会补齐日期序列：周五、周六识别为业务周末并标记“无需日报”，其他缺失日期标记为“缺少报表”。

## 蒲公英 Excel 导入 API

`POST /v1/imports/dandelion-excel` 接收不超过 50 MB 的 `.xlsx`，以文件内最大的“数据更新日期”作为本次数据日期。`GET /v1/imports/dandelion-excel` 按数据日期倒序返回每个日期最新一次成功上传，供前端展示历史上传与日期缺口。

## 笔记计划分析 API

`GET /v1/analytics/maituo/note-campaigns` 按 `笔记ID + 计划名 + 场域` 聚合笔记明细。公网入口为 `/paipai/api/analytics/maituo/note-campaigns`。查询参数：

- `window`：`3d`、`7d` 或 `all`，默认 `7d`；3D/7D 取最近实际存在的报表日，周五、周六自然跳过
- `q`：按笔记ID、计划名或场域模糊搜索
- `sort`：`daily_spend` 按最新报表日消耗排序，`cumulative_spend` 按所选范围累计消耗排序，默认 `cumulative_spend`
- `page`、`page_size`：分页参数，每页最多 100 个组合

每个组合返回所选报表日内的累计消耗、累计回搜人数，以及逐日报表中的当天回搜成本。某日报日未投放时补零日增量，使累计曲线保持水平；回搜成本不累加。分析结果不返回笔记 URL、分类或子账户。

## 内容分析 API

`GET /v1/analytics/content-analysis` 汇总蒲公英、Maituo 客户日报、薯量笔记快照和服务商稿件标签。公网入口为 `/paipai/api/analytics/content-analysis`，前端页面为 `/paipai/content-analysis`。查询参数：

- `spu`：`辅酶` 或 `磷虾油`，默认 `辅酶`
- `agency`：`全部`、`曼杰`、`有一有二` 或 `智元`，默认 `全部`
- `dimension`：`audience`（人群标签）或 `scenario`（用户场景），默认 `audience`

机构仅包含以下蒲公英下单账号映射；`全部`也是这三家机构的合集：

- 江苏拾光宝盒信息技术有限公司 → 曼杰
- 上海有一有二网络技术有限公司 → 有一有二
- 杭州智元文化传播有限公司 → 智元

热力图以内容类型为行，以选定的人群标签或用户场景为列。爆文率分母只计入蒲公英“站外活跃成本（15天设备归因）”大于 0 的笔记，成本不高于 20 判定为爆文。投流按笔记和场域汇总所有已保存日报：单场域累计消耗不少于 200，且搜索回搜成本不高于 30 或信息流预计回流后成本不高于 70，即判定投流达标。薯量最新笔记快照 `total_roi` 不低于 1.2 判定 ROI 达标。

接口同时返回标签覆盖率、各指标可评估样本数、单元格聚合数据和笔记明细。无标签笔记保留为“未标注”，前端默认隐藏并可手动显示。

## 子账户与计划诊断 API

`GET /v1/analytics/maituo/account-plan-diagnosis` 复刻日报看板的“子账户与计划诊断”，公网入口为 `/paipai/api/analytics/maituo/account-plan-diagnosis`，前端页面为 `/paipai/account-plan-diagnosis`。可选参数 `spu` 默认取 `辅酶`。

子账户层读取最新完整日报的“分子账户”表，统一使用 KPI 70；搜索使用回搜成本，信息流使用预计回流后成本。接口保留按“子账户 + 场域”的诊断表和计划明细，同时按子账户汇总最近 30 个自然日的搜索与信息流趋势。前端可选择子账户及 7/14/30 日周期，直接展示总消耗、搜索消耗与回搜成本、信息流消耗与预计回流后成本、搜索 CPC 与 CTR、信息流 CPC 与 CTR、搜索与信息流回搜率六张图；7 日模式显示点位数值。计划层读取同日报的“笔记明细”表，搜索 KPI 为 30，信息流 KPI 为 70：

数据库视图 `maituo_customer_daily_search_user_overlap` 按“报表日期 + SPU”以分 SPU 总览回搜人数为统一基准，分别计算“子账户合计 / SPU”和“笔记合计 / SPU”两个综合重合系数，校准系数为各自倒数。笔记中的多账户字段会先拆分映射到 SPU，同一笔记行在同一 SPU 内只计算一次。数据总览接口的 `overlap_points` 返回所选 SPU 与 7/14/30 日周期内的三层人数、两组重合人数及系数；整体回搜重合图按有效报表日展示这两个综合系数，不归属于任何单一子账户。子账户与计划诊断不使用上述修正系数：接口中的 `cost` 和兼容字段 `original_cost` 均为日报原始成本，`correction_coefficient` 返回 `null`，所有 KPI 状态、连续超标和动作建议也均按日报原始成本判断。

- 成本为空：今日未投放
- 成本低于 KPI：建议放大
- 成本达到或超过 KPI 且连续不足 3 个有效报表日：正常观察
- 成本达到或超过 KPI 且连续满 3 个有效报表日：建议停止

计划明细按笔记 ID 精确关联“蒲公英数据”，同一笔记存在多条快照时取“数据更新日期”最新的一条，补充标题、达人、类型、内容标签、发布时间、合作金额、曝光、阅读、互动及单价。响应同时返回蒲公英最近同步时间和匹配/缺失数量；未匹配笔记保留日报诊断结果，不以相似内容替代。

## 投流情况对比 API

`GET /v1/analytics/maituo/traffic-comparisons` 按 `笔记ID + 场域` 聚合最新报表日仍在投放的计划。公网入口为 `/paipai/api/analytics/maituo/traffic-comparisons`，前端页面为 `/paipai/traffic-comparison`。查询参数：

- `window`：`3d`、`7d` 或 `all`，默认 `7d`；用于计划趋势和区间指标
- `q`：按笔记ID、计划名或场域模糊搜索；命中计划名时仍返回同笔记、同场域下的全部计划
- `page`、`page_size`：分页参数，每页最多 100 个笔记场域组合

列表固定按“当天有效回搜成本差异降序、最高当天回搜成本降序”排列。单计划组合继续返回，但差异为 0；当天回搜人数为 0 的计划标记为无有效成本，不以 0 元参与差异计算。详情返回同一笔记、同一场域下各计划的当天指标、所选区间汇总和逐日报表点；周末不补日期，回搜成本不累加。

`GET /v1/analytics/maituo/traffic-comparison-delivery?note_id=...&placement=信息流|搜索` 返回选中笔记场域下各计划关联到的聚光计划、单元及投放配置，公网入口为 `/paipai/api/analytics/maituo/traffic-comparison-delivery`。前端据此横向比较出价与优化目标、单元定向、人群、地域、设备和搜索关键词，不展示日报子账户、广告账户和计划日预算。默认只展示不同配置，列表型配置还会移除各计划共有值；地域归并到省级后比较。该接口按需加载，不增加列表响应体积。


## 飞书手动同步 API

构建并启动本机 API：

```bash
make lark-sync-start
```

服务由 PM2 运行在 `127.0.0.1:18081`，不会定时同步，也不会在启动时同步。接口同步返回结果：`200 OK` 表示数据库写入已经完成，已有同类作业运行时返回 `409 Conflict`。

- `GET /healthz`：进程健康状态。
- `GET /v1/manuscript-assets/{sha256}`：读取仍被稿件引用的图片，支持 ETag 和私有缓存。
- `POST /v1/sync/manuscripts`：同步服务商稿件表。
- `GET /v1/sync/manuscripts/status`：查询三张稿件表最近的持久化同步状态。
- `POST /v1/sync/dandelion`：只同步配置 Base 内的“蒲公英数据”表。
- `GET /v1/sync/dandelion/status`：查询蒲公英最近 10 次持久化同步结果。

前端入口为 `/paipai/data-sync/dandelion` 和 `/paipai/data-sync/manuscripts`。公网只开放四个固定路径：两个状态查询和两个显式同步触发；飞书凭据、Base Token 与源表地址均不返回前端。

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

业务数据不会在定时器或服务启动时自动刷新。只有显式调用计划、单元或创意接口时，目标表才会更新。三个 Make 命令分别刷新全部授权广告主，计划和单元默认使用增量模式，创意使用完整模式：

```bash
make xhs-sync-campaigns
make xhs-sync-units
make xhs-sync-creativities
make xhs-sync-status
```

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
