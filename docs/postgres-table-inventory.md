# Paipai PostgreSQL table inventory

Database: `paipai_red`. Inventory date: 2026-07-29.

Public access uses the login role `paipai_reader` and the dedicated
`paipai_readonly` schema. The role has no privileges on the `public` schema's
base tables. Read-only views omit operational identifiers, import hashes, raw
API payloads and soft-deleted rows.

## Public read-only views

| View | Source table | Purpose |
| --- | --- | --- |
| `lark_bitable_tables` | `public.lark_bitable_tables` | Active Feishu Bitable table catalog |
| `lark_bitable_records` | `public.lark_bitable_records` | Active Feishu record fields, including Dandelion data |
| `lark_linked_documents` | `public.lark_linked_documents` | Linked Feishu document title, content and fetch metadata |
| `maituo_customer_daily_kpis` | same name in `public` | Daily KPI metrics |
| `maituo_customer_daily_notes` | same name in `public` | Note and campaign delivery metrics |
| `maituo_customer_daily_spus` | same name in `public` | SPU daily summaries |
| `maituo_customer_daily_subaccounts` | same name in `public` | Subaccount and placement summaries |
| `maituo_customer_daily_trends` | same name in `public` | Search and order trend metrics |
| `service_provider_note_executions` | same name in `public` | Active provider manuscript execution metadata |
| `service_provider_notes` | same name in `public` | Manuscript text keyed by note ID |
| `xhs_jg_advertisers` | same name in `public` | Spotlight advertiser catalog |
| `xhs_jg_campaigns` | same name in `public` | Active Spotlight campaigns without raw payloads |
| `xhs_jg_units` | same name in `public` | Active Spotlight units without raw payloads |
| `xhs_jg_creativities` | same name in `public` | Active Spotlight creatives without raw payloads |

Query these views with a qualified name, for example:

```sql
SELECT report_date, subaccount, placement, spend
FROM paipai_readonly.maituo_customer_daily_subaccounts
ORDER BY report_date DESC, spend DESC
LIMIT 20;
```

## Internal base tables

| Group | Tables | Public reader access |
| --- | --- | --- |
| Guorai analytics | `guorai_notes`, `guorai_note_snapshots`, `guorai_plans`, `guorai_plan_snapshots`, `guorai_plan_notes` | No |
| Guorai operations | `guorai_fetch_runs` | No |
| Feishu Bitable | `lark_bitable_tables`, `lark_bitable_records` | Through filtered views |
| Feishu documents | `lark_linked_documents`, `lark_record_documents` | Linked document cache through a read-only view; record-to-document relation is not exposed |
| Maituo daily data | `maituo_customer_daily_kpis`, `maituo_customer_daily_notes`, `maituo_customer_daily_spus`, `maituo_customer_daily_subaccounts`, `maituo_customer_daily_trends` | Through filtered views |
| Maituo operations | `maituo_customer_daily_import_runs` | No |
| Provider content | `service_provider_content_tables`, `service_provider_note_executions`, `service_provider_notes` | Manuscripts and active execution rows through filtered views; provider source configuration is not exposed |
| Provider embeddings | `service_provider_note_embeddings`, `service_provider_note_embedding_runs` | No |
| Feishu operations | `sync_runs` | No |
| Spotlight data | `xhs_jg_advertisers`, `xhs_jg_campaigns`, `xhs_jg_units`, `xhs_jg_creativities` | Through filtered views |
| Spotlight operations | `xhs_jg_sync_runs` | No |

There are 27 base tables in total. Expanding the public set requires adding a
filtered view and explicitly granting `SELECT`; future base tables are not
automatically exposed.

## Connection

Credentials are generated during deployment and stored only on the server at
`/home/ubuntu/.config/paipai/reader.env` with mode `0600`.

```bash
set -a
. /home/ubuntu/.config/paipai/reader.env
set +a
psql
```

Remote clients connect to `pangutech.online:5432`, database `paipai_red`, user
`paipai_reader`. Use `sslmode=verify-full` and a trusted CA bundle:

```text
postgresql://paipai_reader:PASSWORD@pangutech.online:5432/paipai_red?sslmode=verify-full&sslrootcert=/path/to/ca-bundle.crt
```

## Security controls

- Public IPv4/IPv6 access is limited to database `paipai_red` and login role
  `paipai_reader`; other roles have no public HBA rule.
- TLS 1.2 or newer is mandatory, with the `pangutech.online` Let's Encrypt
  certificate and SCRAM-SHA-256 authentication.
- `NOSUPERUSER`, `NOCREATEDB`, `NOCREATEROLE`, `NOREPLICATION`, `NOINHERIT`.
- Five concurrent connections maximum.
- Read-only transactions, 15-second statement timeout and 5-minute idle timeout.
- No direct privileges on `public` tables or sequences.
