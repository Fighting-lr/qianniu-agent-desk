"use client";

/**
 * ============================================================================
 * [AgentDesk] RiskAlertBar —— 会话窗口顶部告警条
 * ----------------------------------------------------------------------------
 * 为什么要有它？
 *   右侧面板的风险信息需要"客服主动去看"，而顶部告警条是"被动看到"。
 *   高危会话不能指望客服先点开右栏 —— 告警必须在视线第一落点。
 *
 * 设计约束：
 *   · 高度固定 32px，不挤压消息区（客服对布局变化极度敏感）
 *   · 只在 high / critical 出现，其余等级不渲染（避免"狼来了"）
 *   · critical 用脉冲，high 不用 —— 视觉强度必须拉开
 *   · 右侧提供直达动作，不做纯信息展示
 * ============================================================================
 */

import { ArrowRight, ShieldAlert } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/i18n/provider";
import { RISK_STYLE, needsImmediateAttention } from "./risk-style";
import type { RiskLevel } from "./types";

type RiskAlertBarProps = {
  level: RiskLevel;
  /** 一句话风险概要。 */
  summary?: string;
  /** 已有多少条同类高危会话 —— 支撑"批次聚集"叙事。 */
  clusterCount?: number;
  /** 点击「查看全部」跳看板。 */
  onViewBoard?: () => void;
  className?: string;
};

export function RiskAlertBar({
  level,
  summary,
  clusterCount,
  onViewBoard,
  className,
}: RiskAlertBarProps) {
  const t = useI18n();

  // 只有 high / critical 才渲染 —— 低等级告警会训练客服忽略它。
  if (!needsImmediateAttention(level)) return null;

  const style = RISK_STYLE[level];
  const critical = level === "critical";
  const headline = critical ? t("csRisk.alertBar.critical") : t("csRisk.alertBar.high");

  return (
    <div
      role="alert"
      className={cn(
        "flex h-8 shrink-0 items-center gap-2 border-b px-3 text-xs",
        style.bg,
        style.border,
        className,
      )}
    >
      <ShieldAlert className={cn("size-3.5 shrink-0", style.text, critical && "risk-pulse")} />

      <span className={cn("shrink-0 font-medium", style.text)}>{headline}</span>

      {summary ? (
        <>
          <span className="shrink-0 text-muted-foreground">·</span>
          <span className="min-w-0 flex-1 truncate text-muted-foreground">{summary}</span>
        </>
      ) : (
        <span className="flex-1" />
      )}

      {typeof clusterCount === "number" && clusterCount > 0 ? (
        <span
          className={cn(
            "shrink-0 rounded border px-1.5 py-px text-[10px] tabular-nums",
            style.border,
            style.text,
          )}
        >
          {clusterCount}
        </span>
      ) : null}

      {onViewBoard ? (
        <button
          type="button"
          onClick={onViewBoard}
          className={cn(
            "inline-flex shrink-0 items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-medium transition-colors hover:bg-background/60",
            style.text,
          )}
        >
          {t("csRisk.alertBar.viewAll")}
          <ArrowRight className="size-3" />
        </button>
      ) : null}
    </div>
  );
}
