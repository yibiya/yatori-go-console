"use client"

import {useMemo, useState} from "react"
import {Button} from "@/components/ui/button"
import {cn} from "@/lib/utils"

/**
 * 账号日志视图。
 *
 * 原始日志是刷课内核打印的行式文本，一个视频任务点会连续输出几十条
 * “提交状态：false 观看时间：58/278 …”进度行，直接铺在 <pre> 里没法看。
 * 这里把行解析成结构化条目，按 (类型·课程·小节·任务点) 聚合只保留最新状态，
 * 给出进度条/观看秒数/类型着色，同时保留“原始日志”视图用于排查。
 */

export type LogKind = "video" | "audio" | "doc" | "bbs" | "exam" | "work" | "course" | "error" | "info"

export type LogEntry = {
    id: string
    kind: LogKind
    /** MM-DD HH:mm，无时间戳的行为空 */
    time: string
    course?: string
    section?: string
    title?: string
    /** 0-100，仅进度类日志有 */
    progress?: number
    /** 观看秒数 当前/总数 */
    watch?: string
    state: "done" | "running" | "warn" | "fail"
    message: string
    /** 聚合的原始行数 */
    count: number
}

const KIND_META: Record<LogKind, {label: string; chip: string}> = {
    video:   {label: "视频", chip: "border-sky-500/25 bg-sky-500/10 text-sky-700 dark:text-sky-300"},
    audio:   {label: "音频", chip: "border-cyan-500/25 bg-cyan-500/10 text-cyan-700 dark:text-cyan-300"},
    doc:     {label: "文档", chip: "border-violet-500/25 bg-violet-500/10 text-violet-700 dark:text-violet-300"},
    bbs:     {label: "讨论", chip: "border-teal-500/25 bg-teal-500/10 text-teal-700 dark:text-teal-300"},
    exam:    {label: "考试", chip: "border-rose-500/25 bg-rose-500/10 text-rose-700 dark:text-rose-300"},
    work:    {label: "作业", chip: "border-amber-500/25 bg-amber-500/10 text-amber-700 dark:text-amber-300"},
    course:  {label: "课程", chip: "border-border bg-muted text-muted-foreground"},
    error:   {label: "异常", chip: "border-destructive/30 bg-destructive/10 text-destructive"},
    info:    {label: "系统", chip: "border-border bg-muted text-muted-foreground"},
}

const TS_RE = /^\[(\d{4}-\d{2}-\d{2}) ([\d:]+)\]\s+\[([^\]]+)\]\s*(.*)$/
const CHEV = /【([^】]*)】/g

function classify(rest: string): LogKind {
    if (/异常|失败|错误|不可用|未找到|无法|触发风控/.test(rest)) return "error"
    if (/提交状态|观看时间/.test(rest)) return "video"
    if (/音频/.test(rest)) return "audio"
    if (/文档阅览/.test(rest)) return "doc"
    if (/讨论任务点/.test(rest)) return "bbs"
    if (/考试|试卷/.test(rest)) return "exam"
    if (/作业|答题|章节测验|章测/.test(rest)) return "work"
    if (/获取课程章节|正在学习该课程|课程学习完毕|章节为空|非任务点/.test(rest)) return "course"
    return "info"
}

function toEntries(text: string): LogEntry[] {
    const out: LogEntry[] = []
    if (!text) return out
    text.split(/\r?\n/).forEach((rawLine, i) => {
        const line = rawLine.trim()
        if (!line) return
        const ts = TS_RE.exec(line)
        const date = ts?.[1] ?? ""
        const time = ts?.[2] ?? ""
        const rest = ts?.[4] ?? line
        const kind = classify(rest)
        // >>> 之后是内核真正想表达的状态段；没有 >>> 时整行都是正文
        const afterChevron = line.includes(">>>") ? line.split(">>>")[1].trim() : rest
        const chevrons = [...rest.matchAll(CHEV)].map((m) => m[1])

        const progress = /观看进度：([\d.]+)%/.exec(afterChevron)?.[1]
        const watch = /观看时间：(\d+\s*\/\s*\d+)/.exec(afterChevron)?.[1].replace(/\s/g, "")
        const boolState = /(?:提交状态|阅览状态|状态)：(true|false)/.exec(afterChevron)?.[1]

        let state: LogEntry["state"] = "running"
        if (kind === "error") state = "fail"
        else if (/过超提交成功|回复发表成功|提交成功|学习完毕|已完成/.test(afterChevron)) state = "done"
        else if (boolState === "true" && (progress === "100.00" || /阅览状态：true/.test(afterChevron))) state = "done"
        else if (boolState === "true") state = "running"
        else if (/跳过|为空/.test(afterChevron)) state = "warn"

        // 摘要：剥掉已单独展示的指标与书名号路径，避免一行里重复三遍
        let message = afterChevron
            .replace(/提交状态：(true|false)\s*/, "")
            .replace(/观看时间：\S+\s*/, "")
            .replace(/过超时间：\S+\s*/, "")
            .replace(/观看进度：[\d.]+%\s*/, "")
            .replace(/文档阅览状态：(true|false)\s*/, "")
            .replace(/【[^】]*】/g, "")
            .trim()
        if (!message) message = chevrons[chevrons.length - 1] ?? rest

        out.push({
            id: `${i}`,
            kind,
            time: time ? `${date.slice(5)} ${time.slice(0, 5)}` : "",
            course: chevrons[0],
            section: chevrons[1],
            title: chevrons[2],
            progress: progress ? Number(progress) : undefined,
            watch,
            state,
            message,
            count: 1,
        })
    })
    return out
}

/** 按 类型+课程+小节+标题 聚合，连续进度行折叠成一条（×N） */
function collapse(entries: LogEntry[]): LogEntry[] {
    // 运行期动态键（课程名/标题不可预先枚举），需保留插入顺序，用对象 + order 数组承载
    const byKey: Record<string, LogEntry> = {}
    const order: string[] = []
    for (const e of entries) {
        const key = e.course ? `${e.kind}|${e.course}|${e.section ?? ""}|${e.title ?? e.message}` : `single|${e.id}`
        const prev = byKey[key]
        if (prev) {
            byKey[key] = {...e, count: prev.count + 1}
        } else {
            byKey[key] = e
            order.push(key)
        }
    }
    return order.map((k) => byKey[k])
}

type Filter = "all" | LogKind

const FILTERS: {id: Filter; label: string}[] = [
    {id: "all", label: "全部"},
    {id: "video", label: "视频"},
    {id: "doc", label: "文档"},
    {id: "bbs", label: "讨论"},
    {id: "exam", label: "作业考试"},
    {id: "course", label: "课程"},
    {id: "error", label: "异常"},
]

export function LogView({logs, loading}: {logs: string; loading: boolean}) {
    const [filter, setFilter] = useState<Filter>("all")
    const [grouped, setGrouped] = useState(true)
    const [rawMode, setRawMode] = useState(false)

    const parsed = useMemo(() => toEntries(logs), [logs])
    const view = useMemo(() => {
        let list = grouped ? collapse(parsed) : parsed
        if (filter === "exam") list = list.filter((e) => e.kind === "exam" || e.kind === "work")
        else if (filter !== "all") list = list.filter((e) => e.kind === filter)
        // 原始视图直接给全量文本，不做截断
        return grouped ? list : list.slice(-400)
    }, [parsed, filter, grouped])

    const stats = useMemo(() => {
        const done = parsed.filter((e) => e.state === "done").length
        const fail = parsed.filter((e) => e.state === "fail").length
        return {done, fail, total: parsed.length}
    }, [parsed])

    if (rawMode) {
        return (
            <div className="overflow-hidden rounded-lg border bg-card">
                <div className="flex items-center justify-between gap-3 border-b bg-muted/30 px-3 py-2">
                    <span className="text-xs text-muted-foreground">原始日志 · 最近 {stats.total} 条</span>
                    <Button variant="ghost" size="sm" onClick={() => setRawMode(false)}>结构化视图</Button>
                </div>
                <pre className="max-h-[32rem] min-h-[12rem] overflow-auto whitespace-pre-wrap break-all p-3 font-mono text-[11px] leading-5 text-muted-foreground">
{loading && !logs ? "日志加载中…" : logs || "暂无日志"}
                </pre>
            </div>
        )
    }

    return (
        <div className="overflow-hidden rounded-lg border bg-card">
            {/* 工具栏 */}
            <div className="flex flex-wrap items-center gap-x-3 gap-y-2 border-b bg-muted/30 px-3 py-2">
                <div className="flex min-w-0 flex-wrap items-center gap-1">
                    {FILTERS.map((f) => {
                        const n = f.id === "all" ? parsed.length
                            : parsed.filter((e) => (f.id === "exam" ? e.kind === "exam" || e.kind === "work" : e.kind === f.id)).length
                        return (
                            <button
                                key={f.id}
                                onClick={() => setFilter(f.id)}
                                className={cn(
                                    "rounded-md px-2 py-1 text-xs font-medium transition-colors",
                                    filter === f.id
                                        ? "bg-primary text-primary-foreground"
                                        : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                                )}
                            >
                                {f.label}
                                <span className="ml-1 tabular-nums opacity-70">{n}</span>
                            </button>
                        )
                    })}
                </div>
                <div className="ml-auto flex shrink-0 items-center gap-3 text-xs text-muted-foreground">
                    <span className="flex items-center gap-1">
                        <span className="h-1.5 w-1.5 rounded-full bg-emerald-500"/>
                        完成 <b className="font-medium tabular-nums text-foreground">{stats.done}</b>
                    </span>
                    <span className="flex items-center gap-1">
                        <span className="h-1.5 w-1.5 rounded-full bg-red-500"/>
                        异常 <b className="font-medium tabular-nums text-foreground">{stats.fail}</b>
                    </span>
                    <label className="flex cursor-pointer select-none items-center gap-1.5">
                        <input type="checkbox" checked={grouped} onChange={(e) => setGrouped(e.target.checked)}
                               className="h-3.5 w-3.5 accent-primary"/>
                        聚合任务点
                    </label>
                    <Button variant="ghost" size="sm" onClick={() => setRawMode(true)}>原始日志</Button>
                </div>
            </div>

            {/* 列表 */}
            <div className="max-h-[32rem] min-h-[12rem] divide-y overflow-auto">
                {loading && parsed.length === 0 ? (
                    <div className="p-10 text-center text-sm text-muted-foreground">日志加载中…</div>
                ) : view.length === 0 ? (
                    <div className="p-10 text-center text-sm text-muted-foreground">
                        {stats.total === 0 ? "该账号暂无日志" : "该筛选条件下暂无日志"}
                    </div>
                ) : (
                    view.map((e) => <LogRow key={`${e.id}-${e.count}`} entry={e}/>)
                )}
            </div>
        </div>
    )
}

function LogRow({entry}: {entry: LogEntry}) {
    const meta = KIND_META[entry.kind]
    const showBar = entry.progress !== undefined && entry.progress < 100
    return (
        <div className="relative flex gap-3 px-4 py-2.5 transition-colors hover:bg-muted/40">
            <span
                className={cn(
                    "absolute inset-y-2 left-1.5 w-0.5 rounded-full",
                    entry.state === "done" && "bg-emerald-500/70",
                    entry.state === "running" && "bg-sky-500/60",
                    entry.state === "warn" && "bg-zinc-400/40",
                    entry.state === "fail" && "bg-red-500/70",
                )}
            />
            <div className="flex w-full min-w-0 flex-col gap-1">
                <div className="flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0.5 text-[13px] leading-5">
                    <span className="hidden shrink-0 font-mono text-[11px] tabular-nums text-muted-foreground/70 sm:inline">
                        {entry.time}
                    </span>
                    <span className={cn("shrink-0 rounded-md border px-1.5 py-0.5 text-[10px] font-medium", meta.chip)}>
                        {meta.label}
                    </span>
                    {entry.course ? (
                        <span className="max-w-[16rem] truncate font-medium text-foreground" title={entry.course}>
                            {entry.course}
                        </span>
                    ) : null}
                    {entry.section ? (
                        <>
                            <span className="text-muted-foreground/40">/</span>
                            <span className="max-w-[14rem] truncate text-muted-foreground" title={entry.section}>
                                {entry.section}
                            </span>
                        </>
                    ) : null}
                    {entry.title ? (
                        <>
                            <span className="text-muted-foreground/40">/</span>
                            <span className="max-w-[18rem] truncate" title={entry.title}>{entry.title}</span>
                        </>
                    ) : null}
                    <span className={cn("min-w-0 max-w-[40rem] flex-1 truncate",
                        entry.state === "fail" ? "text-destructive" : "text-muted-foreground")}
                          title={entry.message}>
                        {entry.message}
                    </span>
                </div>

                {showBar ? (
                    <div className="flex items-center gap-2">
                        <div className="h-1 w-full max-w-xs overflow-hidden rounded-full bg-muted">
                            <div className="h-full rounded-full bg-sky-500/80 transition-[width] duration-500"
                                 style={{width: `${entry.progress}%`}}/>
                        </div>
                        <span className="font-mono text-[11px] tabular-nums text-muted-foreground">{entry.progress}%</span>
                    </div>
                ) : null}
            </div>

            <div className="flex shrink-0 items-center gap-3">
                {entry.watch ? (
                    <span className="font-mono text-[11px] tabular-nums text-muted-foreground">
                        <b className={cn("font-medium", entry.state === "done" && "text-emerald-600 dark:text-emerald-400")}>
                            {entry.watch.split("/")[0]}
                        </b>
                        <span className="text-muted-foreground/60">/{entry.watch.split("/")[1] ?? ""}s</span>
                    </span>
                ) : null}
                {entry.progress === 100 ? (
                    <span className="font-mono text-[11px] tabular-nums text-emerald-600 dark:text-emerald-400">100%</span>
                ) : null}
                {entry.count > 1 ? (
                    <span className="rounded-md bg-muted px-1.5 py-0.5 font-mono text-[10px] tabular-nums text-muted-foreground"
                          title={`该任务点累计输出 ${entry.count} 条进度日志`}>
                        ×{entry.count}
                    </span>
                ) : null}
            </div>
        </div>
    )
}
