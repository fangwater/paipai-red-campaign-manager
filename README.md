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
- 从“稿件”单元格的飞书 Docx 链接拉取正文，并按笔记 ID 保存。
- 仅同步格式明确的 24 位十六进制笔记 ID，空值和占位文本整行跳过。
- 正文按笔记 ID 增量抓取；已存在的正文不重复请求，抓取失败的 ID 下轮重试。
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
- `service_provider_notes`：笔记 ID 与飞书稿件正文。

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
- `LARK_APP_TOKEN`：多维表格 Base 的 App Token
- `DATABASE_URL`：本地 PostgreSQL 连接字符串

飞书手动同步配置：

- `LARK_SYNC_LISTEN`：本机 API 监听地址，默认 `127.0.0.1:18081`
- `SYNC_TIMEOUT`：一次显式同步请求的超时，默认 10 分钟
- `DOCUMENT_REFRESH_INTERVAL`：调用 Base 同步时，已抓取正文的重新拉取间隔，默认 1 小时

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

## 飞书手动同步 API

构建并启动本机 API：

```bash
make lark-sync-start
```

服务由 PM2 运行在 `127.0.0.1:18081`，不会定时同步，也不会在启动时同步。接口同步返回结果：`200 OK` 表示数据库写入已经完成，已有同类作业运行时返回 `409 Conflict`。

- `GET /healthz`：进程健康状态。
- `POST /v1/sync/manuscripts`：同步服务商稿件表。
- `GET /v1/sync/manuscripts/status`：查询三张稿件表最近的持久化同步状态。
- `POST /v1/sync/base`：同步 Base 全部数据表和需要刷新的链接正文。

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

请求中的服务商代码会在开始写入前完整校验。未知或未启用代码返回 `400 Bad Request`，不会只执行部分有效目标。Base 快照通过以下命令显式执行：

```bash
make lark-sync-base
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

`cmd/guorai` 复用薯量网页使用的认证和数据接口，同时支持“我的关注笔记”和“我的关注计划”。登录只保存 Cookie，不保存密码；默认会话文件为 `.guorai/session.json`，目录已加入 `.gitignore`，文件权限为 `0600`。两个列表共用这一个会话。

首次登录：

```bash
go run ./cmd/guorai login --username "$GUORAI_USERNAME"
```

命令会隐藏密码输入。也可以临时设置 `GUORAI_PASSWORD` 环境变量用于非交互运行。

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

`guorai sync` 默认同时回刷笔记和计划最近 15 个快照日。每个快照日查询含首尾共 7 天的触达窗口，并保存平台原始 JSON、类型化原始指标、最新维度和计划-笔记关系。

```bash
set -a; . ./.env; set +a
go run ./cmd/guorai sync
```

等价的显式参数：

```bash
go run ./cmd/guorai sync --type all --days 15 --window-days 7 --timeout 30m
```

可使用 `--type note` 或 `--type plan` 只同步一种数据，使用 `--as-of YYYY-MM-DD` 回刷指定截止日期。默认以平台统计截止日期为最新快照日。

PostgreSQL 表：

- `guorai_fetch_runs`：拉取批次、窗口、归因配置、请求和合并后的原始响应。
- `guorai_notes` / `guorai_plans`：最新维度信息。
- `guorai_plan_notes`：计划与笔记当前关系。
- `guorai_note_snapshots` / `guorai_plan_snapshots`：追加式 Rolling 原始指标快照。

所有表和字段均包含 PostgreSQL 中文注释。重复执行会新增抓取批次和原始快照，不覆盖历史；维度表更新为最近一次看到的信息。
