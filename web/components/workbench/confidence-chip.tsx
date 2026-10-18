"use client";

/**
 * ============================================================================
 * [AgentDesk] ConfidenceChip —— AI 置信度胶囊
 * ----------------------------------------------------------------------------
 * 硬规则（竞品调研结论）：
 *   1) 只用 3 档：高 / 中 / 低。不显示百分比。
 *      —— 未校准的百分比是负债，业务评委会追问「你怎么校准的」。
 *   2) 高置信度弱化为灰字，不上色。不要每条都挂彩色徽章。
 *   3) 低置信度必须带原因，例如「低 · 知识库无同类记录，建议人工核对」。
 *      —— 只说「AI 不确定」是给用户焦虑不给出口。
 * ============================================================================
 */

import { cn } from "@/lib/utils";
import { useI18n } from "@/i18n/provider";
import { CONFIDENCE_STYLE } from "./risk-style";
import type { ConfidenceTier } from "./types";

type ConfidenceChipProps = {
  tier: ConfidenceTier;
  /** 低置信度时必须给原因。 */
  reason?: string;
  className?: string;
};

export function ConfidenceChip({ tier, reason, className }: ConfidenceChipProps) {
  const t = useI18n();
  const style = CONFIDENCE_STYLE[tier];
  const label = t(style.labelKey) ?? style.label;

  // 低置信度：胶囊 + 原因，两行结构
  if (tier === "low" && reason) {
    return (
      <div className={cn("flex flex-col gap-1", className)}>
        <span
          className={cn(
            "inline-flex w-fit items-center rounded-md px-1.5 py-0.5 text-[11px] font-medium leading-none",
            style.text,
            style.bg,
          )}
        >
          {label}
        </span>
        <span className="text-[11px] leading-snug text-muted-foreground">{reason}</span>
      </div>
    );
  }

  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center rounded-md px-1.5 py-0.5 text-[11px] leading-none",
        style.text,
        style.muted ? "" : cn(style.bg, "font-medium"),
        className,
      )}
      title={reason}
    >
      {label}
    </span>
  );
}
