import { useEffect, useMemo, useState } from "react";
import { BookOpenCheck, CheckCircle2, ExternalLink, FileSearch, Search, ShieldCheck, X } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import {
  SPOTLIGHT_DOC_SOURCES, SPOTLIGHT_EVIDENCE_LABELS, SPOTLIGHT_FIELD_DOCS, SPOTLIGHT_LEVEL_LABELS,
  type SpotlightDocLevel, type SpotlightFieldDoc
} from "./spotlight-config-docs";
import "./spotlight-config-helper.css";

type LevelFilter = "all" | SpotlightDocLevel;

const levelFilters: { value: LevelFilter; label: string }[] = [
  { value: "all", label: "全部" },
  { value: "campaign", label: "计划" },
  { value: "unit", label: "单元" },
  { value: "creativity", label: "创意" }
];

function searchableText(doc: SpotlightFieldDoc): string {
  return [
    doc.field, doc.label, doc.summary, doc.interpretation, doc.decision, doc.applies,
    ...(doc.keywords ?? []), ...(doc.related ?? []),
    ...(doc.options ?? []).flatMap((option) => [String(option.code), option.label, option.meaning, option.decision ?? ""])
  ].join(" ").toLowerCase();
}

function SpotlightConfigHelper() {
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedField = searchParams.get("field") ?? "";
  const requestedLevel = searchParams.get("level") as SpotlightDocLevel | null;
  const [query, setQuery] = useState(searchParams.get("q") ?? requestedField);
  const [level, setLevel] = useState<LevelFilter>(requestedLevel && SPOTLIGHT_LEVEL_LABELS[requestedLevel] ? requestedLevel : "all");

  useEffect(() => {
    if (!requestedField) return;
    const timer = window.setTimeout(() => document.getElementById(`spotlight-doc-${requestedField}`)?.scrollIntoView({ block: "start" }), 80);
    return () => window.clearTimeout(timer);
  }, [requestedField]);

  const docs = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return SPOTLIGHT_FIELD_DOCS.filter((doc) => (level === "all" || doc.levels.includes(level)) && (!normalized || searchableText(doc).includes(normalized)));
  }, [level, query]);

  const changeQuery = (value: string) => {
    setQuery(value);
    const params = new URLSearchParams(searchParams);
    params.delete("field");
    if (value.trim()) params.set("q", value.trim()); else params.delete("q");
    setSearchParams(params, { replace: true });
  };

  const changeLevel = (next: LevelFilter) => {
    setLevel(next);
    const params = new URLSearchParams(searchParams);
    if (next === "all") params.delete("level"); else params.set("level", next);
    setSearchParams(params, { replace: true });
  };

  const selectRelated = (field: string) => {
    setLevel("all");
    setQuery(field);
    setSearchParams(new URLSearchParams({ field }), { replace: true });
    window.setTimeout(() => document.getElementById(`spotlight-doc-${field}`)?.scrollIntoView({ block: "start", behavior: "smooth" }), 50);
  };

  return <div className="spotlight-helper">
    <section className="page-heading spotlight-helper-heading">
      <div><h1>聚光配置助手</h1><p>投放管理 · 字段字典、配置影响与状态排障</p></div>
      <div className="heading-status"><span className="status-dot" />官方文档与真实快照校验</div>
    </section>

    <section className="spotlight-helper-tools">
      <label><Search size={16} /><input aria-label="搜索聚光字段或码值" value={query} onChange={(event) => changeQuery(event.target.value)} placeholder="搜索字段、中文含义、码值或场景" /></label>
      {query ? <button className="spotlight-helper-clear" type="button" aria-label="清除配置搜索" title="清除" onClick={() => changeQuery("")}><X size={15} /></button> : null}
      <div className="spotlight-helper-levels" role="group" aria-label="配置层级筛选">{levelFilters.map((item) => <button className={level === item.value ? "active" : ""} type="button" key={item.value} onClick={() => changeLevel(item.value)}>{item.label}</button>)}</div>
      <span>{docs.length} / {SPOTLIGHT_FIELD_DOCS.length} 个字段</span>
    </section>

    <section className="spotlight-helper-source-band">
      <div><BookOpenCheck size={18} /><div><strong>文档依据</strong><span>字段枚举以官方接口文档为主；运营解释由项目快照验证补充，不替代聚光账户实时可选项。</span></div></div>
      <nav aria-label="聚光官方文档">{Object.values(SPOTLIGHT_DOC_SOURCES).map((source) => <a href={source.url} target="_blank" rel="noreferrer" key={source.url}>{source.label}<ExternalLink size={12} /></a>)}</nav>
    </section>

    {docs.length === 0 ? <section className="spotlight-helper-empty"><FileSearch size={24} /><strong>没有匹配字段</strong><span>当前字典尚未收录该关键词</span></section> : <div className="spotlight-helper-layout">
      <aside aria-label="配置字段目录"><strong>字段目录</strong>{docs.map((doc) => <a href={`#spotlight-doc-${doc.field}`} key={doc.field}><span>{doc.label}</span><code>{doc.field}</code></a>)}</aside>
      <main className="spotlight-helper-docs">{docs.map((doc) => {
        const source = SPOTLIGHT_DOC_SOURCES[doc.source];
        return <article id={`spotlight-doc-${doc.field}`} className={requestedField === doc.field ? "selected" : ""} key={doc.field}>
          <header><div><div className="spotlight-helper-badges">{doc.levels.map((item) => <span className={`level ${item}`} key={item}>{SPOTLIGHT_LEVEL_LABELS[item]}</span>)}<span className={doc.configurable ? "configurable" : "readonly"}>{doc.configurable ? "可配置" : "系统状态 / 来源"}</span><span className={`evidence ${doc.evidence}`}>{SPOTLIGHT_EVIDENCE_LABELS[doc.evidence]}</span></div><h2>{doc.label}</h2><code>{doc.field}</code></div><a href={source.url} target="_blank" rel="noreferrer">查看原文<ExternalLink size={13} /></a></header>
          <p className="spotlight-helper-summary">{doc.summary}</p>
          <dl className="spotlight-helper-explanation"><div><dt>怎么理解</dt><dd>{doc.interpretation}</dd></div><div><dt>决策影响</dt><dd>{doc.decision}</dd></div><div><dt>适用条件</dt><dd>{doc.applies}</dd></div></dl>
          {doc.options?.length ? <div className="spotlight-helper-options"><div className="spotlight-helper-options-head"><strong>{doc.configurable ? "码值与候选项" : "状态码与处理动作"}</strong><span>{doc.options.length} 项</span></div><div className="spotlight-helper-options-table" role="table" aria-label={`${doc.label}码值说明`}><div className="table-head" role="row"><span>码值</span><span>名称</span><span>含义</span><span>决策 / 处理</span></div>{doc.options.map((option) => <div role="row" key={String(option.code)}><code>{option.code}</code><strong>{option.label}</strong><span>{option.meaning}</span><span>{option.decision || "按当前对象配置与上级状态综合判断。"}</span></div>)}</div></div> : null}
          <footer><div><ShieldCheck size={13} /><span>{source.label}</span></div>{doc.related?.length ? <nav aria-label={`${doc.label}关联字段`}><span>关联字段</span>{doc.related.map((field) => <button type="button" onClick={() => selectRelated(field)} key={field}>{field}</button>)}</nav> : <div><CheckCircle2 size={13} /><span>暂无关联字段</span></div>}</footer>
        </article>;
      })}</main>
    </div>}
  </div>;
}

export default SpotlightConfigHelper;
