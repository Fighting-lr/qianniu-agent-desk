"use client";

/**
 * ============================================================================
 * [AgentDesk] ReplySuggestionCard —— 右侧「回复建议」Tab
 * ----------------------------------------------------------------------------
 * 设计上最克制的一个组件：它**不是**一个聊天框。
 *
 * 产品定位是「右侧辅佐插件」，不是「替代客服的机器人」。所以这里：
 *   · 给出 2-3 档语气候选，而不是一条"标准答案"
 *   · 每档都标明置信度与依据，让客服判断而不是盲从
 *   · 高危会话下**不生成自由回复**，只显示固定话术指引
 *
 * 最后一条最关键：模型在安全类场景即兴发挥是产品风险，不是能力。
 * ============================================================================
 */

import { useState } from "react";
import { Check, Copy, FileText, Percent, Quote, Wand2 } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/i18n/provider";
import { Button } from "@/components/ui/button";
import { ConfidenceChip } from "./confidence-chip";
import { REPLY_TONE_META, RISK_STYLE, needsImmediateAttention } from "./risk-style";
import type { ReplySuggestion, RiskLevel } from "./types";

type ReplySuggestionCardProps = {
  suggestions: ReplySuggestion[];
  /** 当前会话的综合风险等级 —— 决定是否禁用自由回复。 */
  riskLevel: RiskLevel;
  /** 低置信度的判断理由。 */
  confidenceReason?: string;
  confidenceTier?: "high" | "medium" | "low";
  /** 引用证据，如"订单 6920... 的破损照片"。 */
  evidence?: Array<{ label: string; detail: string }>;
  /** 插入到输入框。 */
  onInsert?: (text: string) => void;
  className?: string;
};

export function ReplySuggestionCard({
  suggestions,
  riskLevel,
  confidenceReason,
  confidenceTier = "medium",
  evidence,
  onInsert,
  className,
}: ReplySuggestionCardProps) {
  const t = useI18n();
  const [activeIdx, setActiveIdx] = useState(0);
  const [copied, setCopied] = useState(false);

  const locked = needsImmediateAttention(riskLevel);
  const style = RISK_STYLE[riskLevel];

  // ---- 高危锁定态：不给自由回复，只给固定指引 ----
  if (locked && suggestions.length === 0) {
    return (
      <div className={cn("flex flex-col gap-3", className)}>
        <div
          className={cn(
            "relative overflow-hidden rounded-md border",
            style.bg,
            style.border,
          )}
        >
          <span className={cn("absolute inset-y-0 left-0 w-1", style.bar, "risk-pulse")} />
          <div className="flex items-start gap-2 py-3 pr-3 pl-4">
            <Wand2 className={cn("mt-0.5 size-4 shrink-0", style.text)} />
            <div className="min-w-0 flex-1">
              <div className={cn("text-sm font-medium", style.text)}>
                {t("csRisk.reply.empty")}
              </div>
              <p className="mt-1 text-[11px] leading-snug text-muted-foreground">
                {t("csRisk.reply.emptyHint")}
              </p>
              {evidence && evidence.length > 0 ? (
                <div className="mt-2 flex flex-col gap-1">
                  {evidence.map((e, i) => (
                    <div key={i} className="flex items-start gap-1.5">
                      <Quote className="mt-px size-2.5 shrink-0 text-muted-foreground" />
                      <span className="text-[10px] leading-snug text-muted-foreground">
                        <span className="text-foreground/80">{e.label}</span>
                        {e.detail ? ` · ${e.detail}` : ""}
                      </span>
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (suggestions.length === 0) {
    return (
      <div
        className={cn(
          "rounded-md border border-dashed border-border px-3 py-4 text-center text-[11px] text-muted-foreground",
          className,
        )}
      >
        {t("csRisk.reply.empty")}
      </div>
    );
  }

  const active = suggestions[Math.min(activeIdx, suggestions.length - 1)];

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(active.text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1600);
    } catch {
      // 剪贴板不可用时静默失败 —— 不打断客服工作流
    }
  };

  return (
    <div className={cn("flex flex-col gap-2.5", className)}>
      {/* ---------- 标题 ---------- */}
      <div className="flex items-center gap-2">
        <Wand2 className="size-3.5 text-muted-foreground" />
        <span className="text-[11px] font-medium text-foreground">
          {t("csRisk.reply.title")}
        </span>
        <ConfidenceChip
          tier={active.confidence || confidenceTier}
          reason={confidenceReason || active.rationale}
          className="ml-auto"
        />
      </div>

      {locked ? (
        <div className="rounded border border-risk-high/40 bg-risk-high-bg px-2.5 py-1 text-[11px] font-medium text-risk-high">
          🛡️ 系统已锁定自由生成 · 现提供法务白名单标准安抚模板
        </div>
      ) : null}

      {/* ---------- 语气切换 ---------- */}
      {suggestions.length > 1 ? (
        <div className="flex gap-1">
          {suggestions.map((s, i) => {
            const meta = REPLY_TONE_META[s.tone];
            const on = i === activeIdx;
            return (
              <button
                key={`${s.tone}-${s.id}`}
                type="button"
                onClick={() => setActiveIdx(i)}
                className={cn(
                  "rounded-md border px-2 py-1 text-[11px] transition-colors",
                  on
                    ? "border-primary/60 bg-primary/10 text-primary"
                    : "border-border bg-card text-muted-foreground hover:text-foreground",
                )}
              >
                {t(meta.labelKey) || meta.label}
              </button>
            );
          })}
        </div>
      ) : null}

      {/* ---------- 正文 ---------- */}
      <div className="rounded-md border border-border bg-card px-3 py-2.5">
        <p className="whitespace-pre-wrap text-[12px] leading-relaxed text-foreground">
          {active.text}
        </p>
        {active.rationale ? (
          <div className="mt-2.5 border-t border-border/60 pt-2 flex items-start gap-1.5 text-[11px] text-muted-foreground">
            <span className="font-medium text-foreground/80 shrink-0">依据：</span>
            <span className="leading-snug">{active.rationale}</span>
          </div>
        ) : null}
      </div>

      <ConfidenceExplain
        tier={active.confidence || confidenceTier}
        reason={confidenceReason || (active.confidence === "low" ? active.rationale : undefined)}
      />

      {/* ---------- 引用证据 ---------- */}
      {evidence && evidence.length > 0 ? (
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-1.5 text-[10px] font-medium text-muted-foreground">
            <FileText className="size-3" />
            {t("csRisk.reply.evidence")}
          </div>
          {evidence.map((e, i) => (
            <div key={i} className="flex items-start gap-1.5 pl-4">
              <Quote className="mt-px size-2.5 shrink-0 text-muted-foreground" />
              <span className="text-[10px] leading-snug text-muted-foreground">
                <span className="text-foreground/80">{e.label}</span>
                {e.detail ? ` · ${e.detail}` : ""}
              </span>
            </div>
          ))}
        </div>
      ) : null}

      {/* ---------- 操作 ---------- */}
      <div className="flex gap-2">
        {onInsert ? (
          <Button
            type="button"
            size="sm"
            variant="secondary"
            onClick={() => onInsert(active.text)}
            className="h-7 flex-1 text-[11px]"
          >
            <Wand2 className="size-3" />
            {t("csRisk.reply.insert")}
          </Button>
        ) : null}
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={handleCopy}
          className="h-7 text-[11px]"
        >
          {copied ? <Check className="size-3 text-risk-low" /> : <Copy className="size-3" />}
          {copied ? t("csRisk.reply.copied") : t("csRisk.reply.copy")}
        </Button>
      </div>
    </div>
  );
}

/**
 * 置信度解释条 —— 低置信度时必须给客服一个"为什么不确定"的出口。
 * 这是把 AI 建议变成可被质疑的东西，而不是黑箱。
 */
export function ConfidenceExplain({
  tier,
  reason,
  className,
}: {
  tier: "high" | "medium" | "low";
  reason?: string;
  className?: string;
}) {
  const t = useI18n();
  if (tier === "high" || !reason) return null;
  return (
    <div
      className={cn(
        "flex items-start gap-2 rounded-md border border-border bg-muted/40 px-2.5 py-1.5",
        className,
      )}
    >
      <Percent className="mt-px size-3 shrink-0 text-muted-foreground" />
      <div className="min-w-0 flex-1">
        <div className="text-[10px] font-medium text-muted-foreground">
          {t("csRisk.reply.confidenceReason")}
        </div>
        <p className="mt-0.5 text-[10px] leading-snug text-foreground/80">{reason}</p>
      </div>
    </div>
  );
}
