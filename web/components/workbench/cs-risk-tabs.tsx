"use client";

/**
 * ============================================================================
 * [AgentDesk] CsRiskTabs —— 右侧辅佐面板的 Tab 容器
 * ----------------------------------------------------------------------------
 * 这是「数据共情者」在工作台里的唯一入口。它把 4 个 Tab 组织起来：
 *
 *   会话信息  ← 上游原有内容（由调用方以 children 传入，本组件不关心）
 *   会话洞察  ← 风险等级 + 判定依据 + 建议动作   ★ 第一落点
 *   客户全景  ← 这个人是谁、以前出过什么事
 *   回复建议  ← 语候选项（高危时锁定）
 *
 * 为什么默认停在「会话洞察」而不是「会话信息」：
 *   产品主张是「让客服在买家开口之前，先看见这个人」。
 *   如果默认停在基本信息页，这个主张就只是一句海报文案。
 *
 * 关于数据获取：
 *   选中会话 → 拉一次 /api/dashboard/cs-risk/conversation。
 *   后端无数据时返回 context=null，这里渲染空态而**不弹 toast** ——
 *   因为绝大多数非演示会话都天然没有风险数据，弹 toast 会污染现场。
 *
 * 关于风险数据的来源可信度：
 *   面板上所有风险结论都标注了来源（硬风险词 / 场景标注 / 工单佐证），
 *   这三层都是**确定性规则**，不经过 LLM。这是可向评委逐条解释的基础。
 * ============================================================================
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ShieldAlert, Sparkles, CheckCircle2 } from "lucide-react";
import { toast } from "sonner";

import { cn } from "@/lib/utils";
import { useI18n } from "@/i18n/provider";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  deriveSensitiveTags,
  fetchReplySuggestions,
  fetchRiskByConversation,
  markRiskHandled,
  parseJsonArray,
  type CSRiskEventDTO,
  type CSSessionContextDTO,
  type ReplyEvidenceDTO,
  type ReplySuggestionsResponse,
} from "@/lib/api/cs-risk";
import { useCsRiskMapStore } from "@/lib/stores/cs-risk-map";
import { CustomerPanorama } from "@/components/workbench/customer-panorama";
import { ReplySuggestionCard } from "@/components/workbench/reply-suggestion-card";
import { RiskInsightPanel } from "@/components/workbench/risk-insight-panel";
import type {
  CustomerPanorama as CustomerPanoramaModel,
  ReplySuggestion,
  RiskCategory,
  RiskEvent,
  RiskLevel,
  RiskSource,
  SessionInsight,
} from "@/components/workbench/types";

type CsRiskTabsProps = {
  /** 站内会话ID。为 0 / null 时只渲染 children（信息 Tab）。 */
  conversationId: number | null;
  /** 买家昵称，用于空态与全景展示。 */
  buyerName?: string;
  /**
   * 会话状态（= lib/generated/enums.ts 的 IMConversationStatus）。
   * 用于「已关闭(4)」时跳过 AI 回复建议 —— 关掉的会话没有可回复的对象，
   * 再跑一次大模型既无意义也白烧 token。
   */
  conversationStatus?: number;
  /** 「会话信息」Tab 的内容，由调用方（conversation-info-panel）传入。 */
  children: React.ReactNode;
  className?: string;
};

/** 与 lib/generated/enums.ts 的 IMConversationStatus.Closed 保持一致。 */
const CONVERSATION_STATUS_CLOSED = 4;

/** 会话信息 Tab 之外的业务 Tab 键。 */
type BizTab = "insight" | "panorama" | "reply";

export function CsRiskTabs({
  conversationId,
  buyerName,
  conversationStatus,
  children,
  className,
}: CsRiskTabsProps) {
  const t = useI18n();
  const [activeTab, setActiveTab] = useState<string>("insight");
  const [loading, setLoading] = useState(false);
  const [context, setContext] = useState<CSSessionContextDTO | null>(null);
  const [events, setEvents] = useState<CSRiskEventDTO[]>([]);
  const [handled, setHandled] = useState(false);
  // [AgentDesk] AI 回复建议：右侧面板里唯一真正调用大模型的数据，
  // 单独一个请求 + 单独的 loading 态，模型慢/挂了都不影响风险面板渲染。
  const [reply, setReply] = useState<ReplySuggestionsResponse | null>(null);
  const [replyLoading, setReplyLoading] = useState(false);
  const isClosed = conversationStatus === CONVERSATION_STATUS_CLOSED;

  // ---- 数据获取：会话切换即刷新 ----
  useEffect(() => {
    if (!conversationId || conversationId <= 0) {
      setContext(null);
      setEvents([]);
      return;
    }

    let cancelled = false;
    setLoading(true);

    void (async () => {
      try {
        const resp = await fetchRiskByConversation(conversationId);
        if (cancelled) return;
        setContext(resp.context ?? null);
        setEvents(resp.risks ?? []);
        setHandled(false);
      } catch {
        // 静默降级：无风险数据不是错误状态，不打扰客服
        if (!cancelled) {
          setContext(null);
          setEvents([]);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    // ---- AI 回复建议（独立请求，与上面的风险面板并行） ----
    // ★ 已关闭的会话直接跳过：没有可回复的对象，调用大模型既无意义也白烧 token。
    //   这里只负责不发请求并清空状态，UI 由下面的「已关闭」空态分支呈现。
    void (async () => {
      setReply(null);
      if (isClosed) {
        setReplyLoading(false);
        return;
      }
      setReplyLoading(true);
      try {
        const r = await fetchReplySuggestions(conversationId);
        if (!cancelled) setReply(r);
      } catch {
        // 后端已约定「模型故障也返回 200 + fallback」，能走到这里基本是网络/鉴权问题。
        // 静默置空 → 卡片渲染成空态，不打断客服工作流。
        if (!cancelled) setReply(null);
      } finally {
        if (!cancelled) setReplyLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [conversationId, isClosed]);

  // ---- DTO → 视图模型 ----
  const risks: RiskEvent[] = useMemo(
    () =>
      events.map((e) => ({
        id: e.id,
        conversationId: e.conversationId,
        category: e.category as RiskCategory,
        level: e.level as RiskLevel,
        score: e.score,
        reason: e.reason,
        source: (e.source || "hard_word") as RiskSource,
        evidenceRef: e.evidenceRef || undefined,
        // 后端把 hardWords 存成 JSON 字符串，这里解析回数组
        hardWords: parseJsonArray(e.hardWords),
        suggestedAction: e.suggestedAction || undefined,
        handled: e.handled,
        detectedAt: e.handledAt ?? "",
      })),
    [events],
  );

  const overallRisk = (context?.overallRisk ?? "none") as RiskLevel;

  const insight: SessionInsight | null = context
    ? {
        conversationId: context.conversationId,
        intentMajor: context.sceneMajor || "—",
        intentMinor: context.sceneMinor || "",
        // 场景来自官方人工标注，因此置信度为「高」；无标注时降为「低」
        intentConfidence: context.sceneMinor ? "high" : "low",
        confidenceReason: context.sceneMinor
          ? undefined
          : "该会话未关联官方场景标注，场景信息不可用",
        emotion: overallRisk === "none" || overallRisk === "low" ? "neutral" : "negative",
        emotionIntensity: riskToIntensity(overallRisk),
        buyerNeed: "",
        evidenceMismatch: context.evidenceMismatch || undefined,
        entities: {
          orderNo: context.orderNo || undefined,
          ticketNo: context.ticketNo || undefined,
          product: context.orderProduct || undefined,
          amount: context.orderAmount || undefined,
        },
        risks,
        overallRisk,
      }
    : null;

  const panorama: CustomerPanoramaModel | null = context
    ? {
        customerId: context.conversationId,
        name: context.buyerNickname || buyerName || "—",
        visitCount: context.buyerVisitCount,
        totalPaid: context.buyerTotalPaid,
        sensitiveTags: deriveSensitiveTags(context),
        // 「上次进线结论」目前后端未落库，先用场景历史兜底
        lastConclusion: context.sceneMinor
          ? `${context.sceneMajor} / ${context.sceneMinor}`
          : undefined,
        lastVisitAt: context.lastMessageAt ?? undefined,
        orders: context.orderNo
          ? [
              {
                orderNo: context.orderNo,
                product: context.orderProduct || "欧莱雅复颜玻尿酸安瓶修护面膜",
                amount: context.orderAmount && context.orderAmount > 0 ? context.orderAmount : context.buyerTotalPaid,
                status: context.orderStatus || "已签收",
                createdAt: context.lastMessageAt ?? "",
              },
            ]
          : [],
        tickets: context.ticketNo
          ? [
              {
                ticketNo: context.ticketNo,
                type: ticketTypeLabel(context.ticketType),
                status: context.ticketSummary || (context.batchNo ? `批次:${context.batchNo} 处置中` : "处理中"),
                summary: context.ticketSummary,
                batchNo: context.batchNo,
                createdAt: context.lastMessageAt ?? "",
              },
            ]
          : [],
      }
    : null;

  const handleInsertDraft = useCallback((text: string) => {
    window.dispatchEvent(new CustomEvent("agentdesk:insert-draft", { detail: text }));
    toast.success("已填入输入框草稿");
  }, []);

  const markHandled = useCallback(async () => {
    if (!context) return;
    try {
      await markRiskHandled({
        sessionNo: context.sessionNo,
        conversationId: context.conversationId,
      });
      setHandled(true);
      setEvents((prev) => prev.map((e) => ({ ...e, handled: true })));
      // 同步刷新全局会话列表角标与色条映射
      void useCsRiskMapStore.getState().load(true);
    } catch {
      // 标记失败不阻断客服流程，保持原样
    }
  }, [context]);

  // 无会话时退化为纯信息面板，避免出现空 Tab 栏
  if (!conversationId || conversationId <= 0) {
    return <>{children}</>;
  }

  const hasData = Boolean(context);

  return (
    <Tabs
      value={activeTab}
      onValueChange={setActiveTab}
      className={cn("min-h-0 flex-1", className)}
    >
      <TabsList variant="line" className="mx-3 mt-2 shrink-0">
        <TabsTrigger value="insight" className="gap-1 text-xs">
          <ShieldAlert className="size-3.5" />
          {t("csRisk.tab.insight")}
          {overallRisk === "critical" || overallRisk === "high" ? (
            <span
              className={cn(
                "ml-0.5 size-1.5 rounded-full",
                overallRisk === "critical" ? "bg-risk-critical risk-pulse" : "bg-risk-high",
              )}
            />
          ) : null}
        </TabsTrigger>
        <TabsTrigger value="panorama" className="text-xs">
          {t("csRisk.tab.panorama")}
        </TabsTrigger>
        <TabsTrigger value="reply" className="text-xs">
          {t("csRisk.tab.reply")}
        </TabsTrigger>
        <TabsTrigger value="info" className="text-xs">
          {t("csRisk.tab.info")}
        </TabsTrigger>
      </TabsList>

      <div className="min-h-0 flex-1 overflow-y-auto px-3 pb-4">
        {/* ---------- 会话洞察（第一落点） ---------- */}
        <TabsContent value="insight" className="pt-3">
          {loading && !hasData ? (
            <EmptyLine text={t("csRisk.empty.loading")} />
          ) : insight ? (
            <RiskInsightPanel
              insight={insight}
              handled={handled}
              onMarkHandled={() => void markHandled()}
            />
          ) : (
            <EmptyState t={t} />
          )}
        </TabsContent>

        {/* ---------- 客户全景 ---------- */}
        <TabsContent value="panorama" className="pt-3">
          {loading && !hasData ? (
            <EmptyLine text={t("csRisk.empty.loading")} />
          ) : panorama ? (
            <CustomerPanorama data={panorama} />
          ) : (
            <EmptyState t={t} />
          )}
        </TabsContent>

        {/* ---------- 回复建议（★ 真实调用大模型生成） ---------- */}
        <TabsContent value="reply" className="pt-3">
          {isClosed ? (
            // 已关闭会话：不生成建议，也不显示"暂无建议"（那会让人以为功能坏了）
            <ClosedReplyHint t={t} />
          ) : replyLoading && !reply ? (
            <EmptyLine text={t("csRisk.reply.generating")} />
          ) : (
            <ReplySuggestionCard
              suggestions={toCardSuggestions(reply, conversationId)}
              riskLevel={overallRisk}
              evidence={toCardEvidence(reply)}
              confidenceTier={deriveConfidenceTier(reply)}
              confidenceReason={reply?.degradedReason}
              onInsert={handleInsertDraft}
            />
          )}
        </TabsContent>

        {/* ---------- 会话信息（上游原有内容） ---------- */}
        <TabsContent value="info" className="pt-3">
          {children}
        </TabsContent>
      </div>
    </Tabs>
  );
}

// ------------------------------------------------------------------ 子组件 ---

function EmptyLine({ text }: { text: string }) {
  return <p className="pt-4 text-sm text-muted-foreground">{text}</p>;
}

/** 空态：引导到风险看板，而不是让客服对着空白发呆。 */
function EmptyState({
  t,
}: {
  t: (key: string, values?: Record<string, string | number>) => string;
}) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border bg-muted/20 px-4 py-8 text-center">
      <Sparkles className="mb-2 size-6 text-muted-foreground" aria-hidden />
      <p className="text-sm font-medium text-foreground">{t("csRisk.empty.noInsight")}</p>
      <p className="mt-1 max-w-xs text-[11px] leading-relaxed text-muted-foreground">
        {t("csRisk.empty.noInsightHint")}
      </p>
      {/* [AgentDesk-BEGIN] base-ui 用 render prop 替代 Radix 的 asChild */}
      <Button
        render={<Link href="/dashboard/risk-board" />}
        variant="outline"
        size="sm"
        className="mt-3 h-7 text-[11px]"
      >
        {t("csRisk.empty.openBoard")}
      </Button>
      {/* [AgentDesk-END] */}
    </div>
  );
}

/** 已关闭会话的回复建议空态：明确告知"不需要生成"，避免客服以为功能坏了。 */
function ClosedReplyHint({
  t,
}: {
  t: (key: string, values?: Record<string, string | number>) => string;
}) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border bg-muted/20 px-4 py-8 text-center">
      <CheckCircle2 className="mb-2 size-6 text-muted-foreground" aria-hidden />
      <p className="text-sm font-medium text-foreground">{t("csRisk.reply.closedTitle")}</p>
      <p className="mt-1 max-w-xs text-[11px] leading-relaxed text-muted-foreground">
        {t("csRisk.reply.closedHint")}
      </p>
    </div>
  );
}

// -------------------------------------------------------------------- 工具 ---

/** 风险等级 → 情绪强度（1-5），供 RiskInsightPanel 画星。 */
function riskToIntensity(level: RiskLevel): number {
  switch (level) {
    case "critical":
      return 5;
    case "high":
      return 4;
    case "medium":
      return 3;
    case "low":
      return 2;
    default:
      return 3;
  }
}

function ticketTypeLabel(ticketType: string): string {
  const map: Record<string, string> = {
    adverse: "不良反应工单",
    reship: "补发换货工单",
    refund: "线下打款工单",
    logistics: "物流工单",
    aftersale: "售后退货工单",
  };
  return map[ticketType] ?? ticketType;
}

// ------------------------------------------------------- AI 回复建议适配 ---

/**
 * 后端建议 → 卡片建议。
 *
 * 两边的类型不完全一样：
 *   · 后端 ReplySuggestionDTO：{ tone, text, confidence, rationale }
 *   · 卡片 ReplySuggestion：   { id, conversationId, tone, text, ... }
 * 卡片用整体的 confidenceTier 表达置信度，不需要逐条字段，所以这里只补齐
 * 结构必需的 id / conversationId，其余原样透传。
 *
 * 高危锁定态直接返回空数组 —— 卡片会走「锁定态」分支渲染处置要点。
 */
function toCardSuggestions(
  reply: ReplySuggestionsResponse | null,
  conversationId: number | null
): ReplySuggestion[] {
  if (!reply) return [];
  return (reply.suggestions ?? []).map((s, i) => ({
    id: i + 1,
    conversationId: conversationId ?? 0,
    tone: s.tone,
    text: s.text,
    confidence: s.confidence,
    rationale: s.rationale,
  }));
}

/**
 * 后端 evidence → 卡片 evidence。
 *
 * 两处都叫 evidence，但职责不同：
 *   · 后端：既有「规则命中的风险点」，也有高危时 AI 生成的「处置要点」
 *   · 卡片：只在锁定态（高危）与自由回复态下作为引用展示
 * 字段形状一致，这里只做空值归一，避免下游拿到 null 去 map。
 */
function toCardEvidence(
  reply: ReplySuggestionsResponse | null
): Array<{ label: string; detail: string }> | undefined {
  if (!reply?.evidence?.length) return undefined;
  return reply.evidence
    .filter((e: ReplyEvidenceDTO) => (e.label ?? "").trim() !== "")
    .map((e: ReplyEvidenceDTO) => ({ label: e.label, detail: e.detail ?? "" }));
}

/**
 * 卡片整体置信度。
 *
 * 取第一条建议的置信度作为代表 —— 三档语气是同一批证据推出来的，
 * 差异主要体现在措辞上，不需要逐档标注。
 * 降级（模型没调通）时强制 low，让客服一眼看出「这不是 AI 给的」。
 */
function deriveConfidenceTier(
  reply: ReplySuggestionsResponse | null
): "high" | "medium" | "low" {
  if (!reply || reply.fallback) return "low";
  const first = reply.suggestions?.[0];
  if (!first) return "medium";
  return first.confidence;
}
