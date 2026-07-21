# PaiPai RED Campaign Manager

PaiPai 小红书投放管理服务。服务定时将飞书多维表格 Base、稿件正文和服务商电子表格同步到本地 PostgreSQL。

## 同步行为

- 自动发现并分页同步 Base 内全部数据表，无需逐个配置 `table_id`。
- 记录字段原样保存为 PostgreSQL `JSONB`。
- 使用 `app_token + table_id + record_id` 作为幂等主键。
- 飞书中消失的表和记录在 PostgreSQL 中标记为软删除。
- 从富文本字段提取飞书 Wiki、Docx 和腾讯文档链接。
- 飞书 Wiki 会解析为真实 Docx，并保存标题、版本和纯文本正文。
- 稿件正文默认每小时刷新，普通表数据默认每 5 分钟同步。
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

调度配置：

- `SYNC_CRON`：标准五段 Cron，默认每 5 分钟
- `SYNC_TIMEZONE`：Cron 时区，默认 `Asia/Shanghai`
- `SYNC_ON_START`：启动时立即同步，默认 `true`
- `SYNC_TIMEOUT`：单轮同步超时，默认 10 分钟
- `DOCUMENT_REFRESH_INTERVAL`：稿件正文刷新间隔，默认 1 小时

## 初始化数据库

```bash
sudo -u postgres createdb -O "$USER" paipai_red
```

当前机器已创建由 `ubuntu` 角色拥有的 `paipai_red` 数据库。示例连接串通过本地 Unix socket 使用 peer 认证，不需要数据库密码。

服务启动时自动执行幂等迁移 [migrations/001_init.sql](migrations/001_init.sql) 和 [migrations/002_provider_content.sql](migrations/002_provider_content.sql)。

## 运行

```bash
make run
```

列出 Base 内全部数据表：

```bash
set -a
source .env
set +a
go run ./cmd/list-tables
```

停止服务时发送 `SIGINT` 或 `SIGTERM`。

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
