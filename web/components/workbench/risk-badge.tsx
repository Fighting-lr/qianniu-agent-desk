"use client";

/**
 * ============================================================================
 * [AgentDesk] RiskBadge —— 风险徽章
 * ----------------------------------------------------------------------------
 * 四种形态（由 variant 控制）：
 *   dot   仅色点 —— 用于会话列表行内，最省空间
 *   badge 色点 + 文字标签 —— 用于卡片标题
 *   card  4px 左色条 + 浅底 —— 用于右侧面板区块
 *   bar   顶部告警条 —— 用于会话窗口顶部，比侧栏更早进入视野
 *
 * 高危（critical）自动附加 risk-pulse 脉冲动画。
 * 颜色全部走语义 token，不写死 hex。
 * ============================================================================
 */

import { AlertTriangle, Info, ShieldAlert } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/i18n/provider";
import { RISK_STYLE, needsImmediateAttention } from "./risk-style";
import type { RiskLevel } from "./types";

type RiskBadgeProps = {
  level: RiskLevel;
  variant?: "dot" | "badge" | "card" | "bar";
  /** 覆盖默认文案，如显示「高危 · 过敏」 */
  label?: string;
  /** 高危详情（触发依据），仅在 card / bar 形态展示 */
  reason?: string;
  className?: string;
};

export function RiskBadge({
  level,
  variant = "badge",
  label,
  reason,
  className,
}: RiskBadgeProps) {
  const t = useI18n();
  const style = RISK_STYLE[level];
  const displayLabel = label ?? t(style.labelKey) ?? style.label;
  const urgent = needsImmediateAttention(level);

  // ---- 仅色点 ----
  if (variant === "dot") {
    return (
      <span
        className={cn("inline-block size-1.5 shrink-0 rounded-full", style.dot, urgent && "risk-pulse", className)}
        role="img"
        aria-label={displayLabel}
      />
    );
  }

  // ---- 色点 + 文字 ----
  if (variant === "badge") {
    return (
      <span
        className={cn(
          "inline-flex shrink-0 items-center gap-1 rounded-md border px-1.5 py-0.5 text-[11px] leading-none",
          style.text,
          style.bg,
          style.border,
          className,
        )}
      >
        <span className={cn("size-1.5 rounded-full", style.dot, urgent && "risk-pulse")} />
        {displayLabel}
      </span>
    );
  }

  // ---- 顶部告警条 ----
  if (variant === "bar") {
    return (
      <div
        role="alert"
        className={cn(
          "flex items-center gap-2 border-b px-3 py-1.5 text-xs",
          style.bg,
          style.border,
          style.text,
          className,
        )}
      >
        <ShieldAlert className={cn("size-3.5 shrink-0", urgent && "risk-pulse")} />
        <span className="font-medium">{displayLabel}</span>
        {reason ? (
          <span className="min-w-0 flex-1 truncate opacity-80">{reason}</span>
        ) : (
          <span className="flex-1" />
        )}
      </div>
    );
  }

  // ---- 卡片 ----
  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-md border",
        style.bg,
        style.border,
        className,
      )}
    >
      {/* 4px 左色条 —— 比徽章更快被扫到 */}
      <span className={cn("absolute inset-y-0 left-0 w-1", style.bar, urgent && "risk-pulse")} />

      <div className="flex items-start gap-2 py-2 pr-2.5 pl-3.5">
        <AlertTriangle className={cn("mt-px size-3.5 shrink-0", style.text)} />
        <div className="min-w-0 flex-1">
          <div className={cn("text-xs font-medium leading-tight", style.text)}>{displayLabel}</div>
          {reason ? (
            <p className="mt-1 text-[11px] leading-snug text-muted-foreground">{reason}</p>
          ) : null}
        </div>
      </div>
    </div>
  );
}

/**
 * 空状态占位 —— 无风险时显示，避免区块塌陷。
 */
export function RiskBadgeEmpty({ className }: { className?: string }) {
  const t = useI18n();
  return (
    <div
      className={cn(
        "flex items-center gap-2 rounded-md border border-dashed border-border px-3 py-2 text-[11px] text-muted-foreground",
        className,
      )}
    >
      <Info className="size-3.5 shrink-0" />
      {t("csRisk.panel.noRisk") ?? "未检测到风险信号"}
    </div>
  );
}
