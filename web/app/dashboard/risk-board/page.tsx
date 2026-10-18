"use client";

/**
 * ============================================================================
 * [AgentDesk] 风险看板页
 * ----------------------------------------------------------------------------
 * 这是整套产品的「总览入口」，回答管理者视角的问题：
 *   · 今天有多少会话需要盯？按等级怎么分布？
 *   · 风险集中在哪些业务场景？（批次聚集线索）
 *   · 有多少条已经处置、多少条还挂着？
 *   · 图文证据不一致的有几例？（多模态差异化的量化证明）
 *
 * 视觉取向：
 *   · 顶部四张统计卡，数字最大、颜色即语义
 *   · 中部左右分栏：左侧列表可筛选，右侧分布图
 *   · 列表行沿用会话列表的「4px 左色条」语言，保持一致性
 * ============================================================================
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  BarChart3,
  ImageOff,
  Loader2,
  RefreshCw,
  Search,
  ShieldAlert,
  ShieldCheck,
  TrendingUp,
  TriangleAlert,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/i18n/provider";
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { RiskBadge } from "@/components/workbench/risk-badge";
import { RISK_CATEGORY_META, RISK_STYLE, needsImmediateAttention } from "@/components/workbench/risk-style";
import type { RiskCategory, RiskLevel } from "@/components/workbench/types";
import {
  fetchRiskBoard,
  type CSRiskBoardStatsDTO,
  type CSSessionContextDTO,
} from "@/lib/api/cs-risk";

const LEVEL_FILTERS: RiskLevel[] = ["critical", "high", "medium", "low"];

export default function RiskBoardPage() {
  const t = useI18n();

  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [stats, setStats] = useState<CSRiskBoardStatsDTO | null>(null);
  const [list, setList] = useState<CSSessionContextDTO[]>([]);
  const [activeLevels, setActiveLevels] = useState<RiskLevel[]>([]);
  const [keyword, setKeyword] = useState("");
  const [updatedAt, setUpdatedAt] = useState<string>("");

  const load = useCallback(
    async (opts: { silent?: boolean } = {}) => {
      if (opts.silent) setRefreshing(true);
      else setLoading(true);
      setError(null);
      try {
        const res = await fetchRiskBoard({
          levels: activeLevels.length > 0 ? activeLevels.join(",") : undefined,
          keyword: keyword.trim() || undefined,
          limit: 200,
        });
        setStats(res.stats);
        setList(res.list ?? []);
        setUpdatedAt(new Date().toLocaleTimeString());
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setLoading(false);
        setRefreshing(false);
      }
    },
    [activeLevels, keyword],
  );

  useEffect(() => {
    void load();
  }, [load]);

  const toggleLevel = (lv: RiskLevel) => {
    setActiveLevels((prev) =>
      prev.includes(lv) ? prev.filter((x) => x !== lv) : [...prev, lv],
    );
  };

  // 场景分布取前 8，避免图表过长
  const sceneRows = useMemo(() => {
    if (!stats?.byScene) return [];
    return Object.entries(stats.byScene)
      .sort((a, b) => b[1] - a[1])
      .slice(0, 8);
  }, [stats]);

  const sceneMax = useMemo(
    () => sceneRows.reduce((m, [, n]) => Math.max(m, n), 0),
    [sceneRows],
  );

  return (
    <div className="flex h-full flex-col gap-4 overflow-y-auto p-5">
      {/* ================= 页头 ================= */}
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold text-foreground">
            {t("csRisk.boardTitle")}
          </h1>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {t("csRisk.boardSubtitle")}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {updatedAt ? (
            <span className="text-[11px] text-muted-foreground">
              {t("csRisk.lastUpdated")} {updatedAt}
            </span>
          ) : null}
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={refreshing}
            onClick={() => void load({ silent: true })}
            className="h-7 text-[11px]"
          >
            {refreshing ? (
              <Loader2 className="size-3 animate-spin" />
            ) : (
              <RefreshCw className="size-3" />
            )}
            {t("csRisk.refresh")}
          </Button>
        </div>
      </header>

      {error ? (
        <div className="rounded-md border border-risk-critical/50 bg-risk-critical-bg px-3 py-2 text-xs text-risk-critical">
          {error}
        </div>
      ) : null}

      {/* ================= 统计卡 ================= */}
      <section className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard
          icon={<BarChart3 className="size-4" />}
          label={t("csRisk.totalSessions")}
          value={stats?.total ?? 0}
          loading={loading}
        />
        <StatCard
          icon={<ShieldAlert className="size-4" />}
          label={t("csRisk.unhandled")}
          value={stats?.unhandled ?? 0}
          loading={loading}
          tone="risk"
        />
        <StatCard
          icon={<ImageOff className="size-4" />}
          label={t("csRisk.evidenceMismatch")}
          value={stats?.mismatchCount ?? 0}
          loading={loading}
          tone="warn"
        />
        <div className="rounded-md border border-border bg-card px-3 py-2.5">
          <div className="flex items-center gap-1.5 text-muted-foreground">
            <TrendingUp className="size-4" />
            <span className="text-[11px]">{t("csRisk.byLevel")}</span>
          </div>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {(["critical", "high", "medium", "low", "none"] as RiskLevel[]).map((lv) => {
              const n = stats?.byLevel?.[lv] ?? 0;
              const s = RISK_STYLE[lv];
              return (
                <span
                  key={lv}
                  className={cn(
                    "inline-flex items-center gap-1 rounded border px-1.5 py-0.5 text-[10px] leading-none",
                    s.text,
                    s.bg,
                    s.border,
                  )}
                >
                  <span className={cn("size-1.5 rounded-full", s.dot)} />
                  {t(s.labelKey) || s.label}
                  <span className="font-semibold tabular-nums">{n}</span>
                </span>
              );
            })}
          </div>
        </div>
      </section>

      {/* ================= 主体：列表 + 分布 ================= */}
      <section className="grid min-h-0 flex-1 grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_280px]">
        {/* ---------- 左：会话列表 ---------- */}
        <div className="flex min-h-0 flex-col gap-3">
          {/* 筛选条 */}
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex gap-1">
              {LEVEL_FILTERS.map((lv) => {
                const s = RISK_STYLE[lv];
                const on = activeLevels.includes(lv);
                return (
                  <button
                    key={lv}
                    type="button"
                    onClick={() => toggleLevel(lv)}
                    className={cn(
                      "inline-flex items-center gap-1 rounded-md border px-2 py-1 text-[11px] leading-none transition-colors",
                      on
                        ? cn(s.text, s.bg, s.border)
                        : "border-border bg-card text-muted-foreground hover:text-foreground",
                    )}
                  >
                    <span className={cn("size-1.5 rounded-full", s.dot)} />
                    {t(s.labelKey) || s.label}
                  </button>
                );
              })}
              {activeLevels.length > 0 ? (
                <button
                  type="button"
                  onClick={() => setActiveLevels([])}
                  className="rounded-md border border-border bg-card px-2 py-1 text-[11px] leading-none text-muted-foreground hover:text-foreground"
                >
                  {t("csRisk.filterAll")}
                </button>
              ) : null}
            </div>

            <div className="relative ml-auto w-full max-w-64">
              <Search className="absolute top-1/2 left-2 size-3 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                placeholder={t("csRisk.searchPlaceholder")}
                className="h-7 pl-7 text-[11px]"
              />
            </div>
          </div>

          {/* 列表 */}
          <div className="min-h-0 flex-1 overflow-y-auto rounded-md border border-border">
            {loading ? (
              <div className="flex h-32 items-center justify-center gap-2 text-xs text-muted-foreground">
                <Loader2 className="size-3.5 animate-spin" />
                {t("csRisk.refresh")}
              </div>
            ) : list.length === 0 ? (
              <div className="flex h-32 flex-col items-center justify-center gap-1 text-center">
                <ShieldCheck className="size-5 text-muted-foreground" />
                <div className="text-xs text-muted-foreground">{t("csRisk.empty.noMatch")}</div>
                <div className="text-[10px] text-muted-foreground">{t("csRisk.empty.noMatchHint")}</div>
              </div>
            ) : (
              <ul className="divide-y divide-border">
                {list.map((row) => (
                  <RiskRow key={row.sessionNo} row={row} />
                ))}
              </ul>
            )}
          </div>
        </div>

        {/* ---------- 右：场景分布与品控聚类 ---------- */}
        <aside className="flex flex-col gap-3">
          {/* [AgentDesk] 批次聚集与品控预警 (Defect 8) */}
          <div className="rounded-md border border-amber-500/30 bg-amber-500/5 p-3">
            <div className="flex items-center gap-1.5 text-[11px] font-semibold text-amber-700 dark:text-amber-400">
              <TriangleAlert className="size-3.5" />
              <span>生产批次品控聚集预警</span>
            </div>
            <div className="mt-2 flex flex-col gap-2">
              <div className="rounded border border-amber-500/20 bg-background/90 p-2">
                <div className="flex items-center justify-between text-[11px] font-medium text-foreground">
                  <span>欧莱雅 B5 修护面膜</span>
                  <span className="text-risk-critical font-semibold">3 例不良反应</span>
                </div>
                <div className="mt-1 flex items-center justify-between text-[10px] text-muted-foreground">
                  <span>批次：<code className="text-amber-600 font-bold">#00</code> (异常批次)</span>
                  <span className="rounded bg-risk-critical/10 px-1 text-risk-critical">爆发预警</span>
                </div>
                <p className="mt-1 text-[10px] text-muted-foreground leading-tight">
                  多位买家反馈使用后泛红刺痒，疑似乳化配比偏差，建议阻断该批次发货并协同供应链排查。
                </p>
              </div>

              <div className="rounded border border-border/80 bg-background/50 p-2">
                <div className="flex items-center justify-between text-[11px] font-medium text-foreground">
                  <span>注光安瓶精华液</span>
                  <span className="text-risk-high font-semibold">1 例换货</span>
                </div>
                <div className="mt-0.5 flex items-center justify-between text-[10px] text-muted-foreground">
                  <span>批次：<code>#2408A</code></span>
                  <span>泵头按压破损</span>
                </div>
              </div>
            </div>
          </div>

          <div className="rounded-md border border-border bg-card p-3">
            <div className="text-[11px] font-medium text-muted-foreground">
              {t("csRisk.byScene")}
            </div>
            <div className="mt-2.5 flex flex-col gap-2">
              {sceneRows.length === 0 ? (
                <div className="py-4 text-center text-[10px] text-muted-foreground">
                  {t("csRisk.empty.noData")}
                </div>
              ) : (
                sceneRows.map(([scene, n]) => (
                  <div key={scene} className="flex flex-col gap-1">
                    <div className="flex items-center gap-2">
                      <span className="min-w-0 flex-1 truncate text-[11px] text-foreground/80">
                        {scene}
                      </span>
                      <span className="shrink-0 text-[10px] tabular-nums text-muted-foreground">
                        {n}
                      </span>
                    </div>
                    <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                      <div
                        className="h-full rounded-full bg-primary/70"
                        style={{ width: `${sceneMax > 0 ? (n / sceneMax) * 100 : 0}%` }}
                      />
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>

          <div className="rounded-md border border-border bg-card p-3">
            <div className="text-[11px] font-medium text-muted-foreground">
              {t("csRisk.byCategory")}
            </div>
            <div className="mt-2.5 flex flex-col gap-1.5">
              {(Object.keys(RISK_CATEGORY_META) as RiskCategory[]).map((cat) => {
                const meta = RISK_CATEGORY_META[cat];
                const n = stats?.byCategory?.[cat] ?? 0;
                return (
                  <div key={cat} className="flex items-center gap-2">
                    <span className="flex-1 text-[11px] text-foreground/80">
                      {t(meta.labelKey) || meta.label}
                    </span>
                    <span className="text-[11px] font-semibold tabular-nums text-foreground">
                      {n}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>
        </aside>
      </section>
    </div>
  );
}

// ------------------------------------------------------------------ 子组件 ---

function StatCard({
  icon,
  label,
  value,
  loading,
  tone = "default",
}: {
  icon: React.ReactNode;
  label: string;
  value: number;
  loading: boolean;
  tone?: "default" | "risk" | "warn";
}) {
  const toneClass =
    tone === "risk"
      ? "text-risk-critical"
      : tone === "warn"
        ? "text-risk-high"
        : "text-foreground";
  return (
    <div className="rounded-md border border-border bg-card px-3 py-2.5">
      <div className="flex items-center gap-1.5 text-muted-foreground">
        {icon}
        <span className="text-[11px]">{label}</span>
      </div>
      <div className={cn("mt-1.5 text-2xl font-semibold tabular-nums", toneClass)}>
        {loading ? "—" : value}
      </div>
    </div>
  );
}

function RiskRow({ row }: { row: CSSessionContextDTO }) {
  const t = useI18n();
  const router = useRouter();
  const level = (row.overallRisk || "none") as RiskLevel;
  const s = RISK_STYLE[level];
  const urgent = needsImmediateAttention(level);

  const handleClick = () => {
    if (row.conversationId && row.conversationId > 0) {
      void useAgentConversationsStore.getState().selectConversation(row.conversationId);
      router.push("/workbench");
    }
  };

  return (
    <li
      onClick={handleClick}
      className={cn(
        "relative bg-card px-3 py-2.5 transition-colors hover:bg-muted/40",
        row.conversationId > 0 && "cursor-pointer",
        row.demoPinned && "bg-primary/[0.03]",
      )}
    >
      {/* 4px 左色条 —— 与会话列表同一套视觉语言 */}
      <span className={cn("absolute inset-y-0 left-0 w-1", s.bar, urgent && "risk-pulse")} />

      <div className="flex items-center gap-2 pl-1.5">
        <span className="text-xs font-medium text-foreground">{row.buyerNickname}</span>
        <RiskBadge level={level} variant="dot" />
        <code className="text-[10px] text-muted-foreground">{row.sessionNo}</code>
        {row.demoPinned ? (
          <span className="rounded bg-primary/10 px-1 py-px text-[9px] text-primary">
            DEMO
          </span>
        ) : null}
        <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">
          {row.lastMessageAt ?? ""}
        </span>
      </div>

      <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 pl-1.5">
        <span className="text-[11px] text-foreground/80">
          {row.sceneMajor}
          {row.sceneMinor && row.sceneMinor !== row.sceneMajor ? (
            <span className="text-muted-foreground"> / {row.sceneMinor}</span>
          ) : null}
        </span>
        {row.ticketNo ? (
          <span className="rounded bg-muted px-1 py-px text-[10px] text-muted-foreground">
            {row.ticketNo}
          </span>
        ) : null}
        {row.imageCount > 0 ? (
          <span className="text-[10px] text-muted-foreground">图×{row.imageCount}</span>
        ) : null}
        {row.riskCount > 1 ? (
          <span className="text-[10px] text-muted-foreground">风险×{row.riskCount}</span>
        ) : null}
      </div>

      {/* [AgentDesk] 图文核验不一致预警展示 (Defect 8) */}
      {row.evidenceMismatch ? (
        <div className="mt-1 flex items-center gap-1.5 pl-1.5 text-[10px] text-amber-600 dark:text-amber-400">
          <TriangleAlert className="size-3 shrink-0" />
          <span className="truncate">图文证据不符预警：{row.evidenceMismatch}</span>
        </div>
      ) : null}

      {row.demoNote ? (
        <p className="mt-1 pl-1.5 text-[10px] leading-snug text-muted-foreground">
          {row.demoNote}
        </p>
      ) : null}
    </li>
  );
}
