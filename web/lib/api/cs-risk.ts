/**
 * ============================================================================
 * [AgentDesk] 风险看板 API 客户端
 * ----------------------------------------------------------------------------
 * 对接后端新增的三条 dashboard 路由：
 *   GET  /api/dashboard/cs-risk/board
 *   GET  /api/dashboard/cs-risk/session/:no
 *   POST /api/dashboard/cs-risk/handle
 *
 * 复用 agent-desk 既有的 request() 基座，自动带上站内登录态，
 * 不引入任何新的网络层依赖。
 * ============================================================================
 */

import { request } from "@/lib/api/client";

/** 风险等级 —— 与 components/workbench/types.ts 保持一致。 */
export type CSRiskLevel = "none" | "low" | "medium" | "high" | "critical";

/** 风险类别。 */
export type CSRiskCategory =
  | "safety"
  | "compliance"
  | "timeliness"
  | "opinion"
  | "experience";

/** 会话全景 —— 对应后端 models.CSSessionContext 的 JSON 序列化。 */
export type CSSessionContextDTO = {
  id: number;
  sessionNo: string;
  conversationId: number;
  orderNo: string;
  ticketNo: string;
  ticketType: string;
  // [AgentDesk] 订单与工单业务详情
  orderProduct?: string;
  orderAmount?: number;
  orderStatus?: string;
  ticketSummary?: string;
  batchNo?: string;
  buyerNickname: string;
  sceneMajor: string;
  sceneMinor: string;
  overallRisk: CSRiskLevel;
  riskCount: number;
  imageCount: number;
  /** JSON 字符串，需二次解析。 */
  imageTypes: string;
  /** JSON 字符串，需二次解析。 */
  evidenceMismatch: string;
  buyerVisitCount: number;
  buyerTotalPaid: number;
  messageCount: number;
  buyerMessageCount: number;
  agentMessageCount: number;
  firstMessageAt: string | null;
  lastMessageAt: string | null;
  demoPinned: boolean;
  demoNote: string;
};

/** 风险事件明细 —— 对应后端 models.CSRiskEvent。 */
export type CSRiskEventDTO = {
  id: number;
  conversationId: number;
  sessionNo: string;
  category: CSRiskCategory;
  level: CSRiskLevel;
  score: number;
  source: string;
  reason: string;
  /** JSON 字符串数组。 */
  hardWords: string;
  evidenceRef: string;
  suggestedAction: string;
  handled: boolean;
  handledAt: string | null;
};

/** 看板统计。 */
export type CSRiskBoardStatsDTO = {
  total: number;
  byLevel: Record<string, number>;
  byCategory: Record<string, number>;
  byScene: Record<string, number>;
  unhandled: number;
  mismatchCount: number;
};

export type CSRiskBoardResponse = {
  stats: CSRiskBoardStatsDTO;
  list: CSSessionContextDTO[] | null;
};

export type CSRiskSessionResponse = {
  /** 无数据时为 null（后端约定，不是错误）。 */
  context: CSSessionContextDTO | null;
  risks: CSRiskEventDTO[] | null;
};

export type CSRiskBoardQuery = {
  /** 风险等级，逗号分隔，如 "critical,high"。 */
  levels?: string;
  /** 大类场景，如 "不良反应"。 */
  scene?: string;
  /** 关键词：买家昵称 / 会话号。 */
  keyword?: string;
  limit?: number;
};

/** 拉取风险看板数据（统计 + 列表，单请求完成）。 */
export async function fetchRiskBoard(query: CSRiskBoardQuery = {}) {
  const qs = new URLSearchParams();
  if (query.levels) qs.set("levels", query.levels);
  if (query.scene) qs.set("scene", query.scene);
  if (query.keyword) qs.set("keyword", query.keyword);
  if (query.limit) qs.set("limit", String(query.limit));
  const suffix = qs.toString() ? `?${qs.toString()}` : "";
  return request<CSRiskBoardResponse>(`/api/dashboard/cs-risk/board${suffix}`);
}

/** 拉取单会话全景 + 风险明细（按官方 session_no）。 */
export async function fetchRiskSession(sessionNo: string) {
  return request<CSRiskSessionResponse>(
    `/api/dashboard/cs-risk/session/${encodeURIComponent(sessionNo)}`
  );
}

/**
 * 拉取单会话全景（按站内 conversationId）。
 *
 * 这是工作台右侧辅佐面板的主调用：选中会话即拉一次。
 * 约定：没有风险数据时后端返回 context=null（而非报错），
 * 调用方据此渲染「暂无洞察」空态，不弹 toast。
 */
export async function fetchRiskByConversation(conversationId: number) {
  return request<CSRiskSessionResponse>(
    `/api/dashboard/cs-risk/conversation?conversationId=${conversationId}`
  );
}

/** 标记风险已处置。传 eventId 处理单条，否则处理整个会话。 */
export async function markRiskHandled(payload: {
  sessionNo?: string;
  conversationId?: number;
  eventId?: number;
}) {
  return request<{ handled: number }>("/api/dashboard/cs-risk/handle", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

// -------------------------------------------------------------- AI 回复建议 ---

/** 单条回复建议。语气枚举与 components/workbench/types.ts 的 ReplyTone 一致。 */
export type ReplySuggestionDTO = {
  tone: "professional" | "warm" | "concise";
  text: string;
  confidence: "high" | "medium" | "low";
  rationale: string;
};

/** 证据 / 处置要点条目。 */
export type ReplyEvidenceDTO = {
  label: string;
  detail: string;
};

/** 后端 `services.ReplySuggestionResult` 的 JSON 序列化。 */
export type ReplySuggestionsResponse = {
  conversationId: number;
  overallRisk: CSRiskLevel;
  /**
   * 高危锁定：critical / high 时后端**不返回自由话术**（suggestions 为空），
   * 改为在 evidence 里给出「AI 处置要点」。这是产品原则，不是 bug。
   */
  locked: boolean;
  suggestions: ReplySuggestionDTO[] | null;
  evidence: ReplyEvidenceDTO[] | null;
  modelName: string;
  elapsedMs: number;
  /** true = 模型未配置 / 调用失败，走的是规则兜底，UI 应提示降级。 */
  fallback: boolean;
  degradedReason?: string;
};

/**
 * 拉取 AI 回复建议 —— 右侧「回复建议」Tab 的真实数据源。
 *
 * 这是整个右侧面板里**唯一真正调用大模型**的接口：
 *   · 中低风险 → 返回 3 档语气候选，客服可一键插入输入框
 *   · critical / high → locked=true，不给自由话术，只给处置要点
 *
 * 后端保证不会因模型故障返回错误码（降级为 fallback=true），
 * 所以这里不需要额外的错误兜底分支。
 */
export async function fetchReplySuggestions(conversationId: number) {
  return request<ReplySuggestionsResponse>(
    `/api/dashboard/cs-risk/reply_suggestions?conversationId=${conversationId}`
  );
}

// ------------------------------------------------------------------ 工具 ---

/** 安全解析后端返回的 JSON 字符串数组。 */
export function parseJsonArray(raw: string | null | undefined): string[] {
  if (!raw) return [];
  try {
    const v = JSON.parse(raw);
    return Array.isArray(v) ? v.map((x) => String(x)) : [];
  } catch {
    return [];
  }
}

/** 安全解析后端返回的 JSON 字符串对象数组。 */
export function parseJsonObjectArray<T>(raw: string | null | undefined): T[] {
  if (!raw) return [];
  try {
    const v = JSON.parse(raw);
    return Array.isArray(v) ? (v as T[]) : [];
  } catch {
    return [];
  }
}

// ------------------------------------------------------------ 视图模型适配 ---

/**
 * 把后端 DTO 适配成右侧面板用的视图模型。
 *
 * 为什么要这一层而不是让组件直接用 DTO：
 *   1. 后端把 imageTypes / evidenceMismatch / hardWords 都存成 JSON 字符串，
 *      解析逻辑集中在这里，组件里就不该出现 JSON.parse；
 *   2. 后端字段是 `ticketType: "adverse"`，前端要显示「不良反应工单」，
 *      这类文案映射属于展示层；
 *   3. 后端目前不返回「上次进线结论」「敏感标签」这类派生字段，
 *      由这里按现有数据推导，未来后端补齐时只改这一处。
 */

/** 工单类型 → 中文标签。与后端 services.ticketTypeLabel 保持一致。 */
const TICKET_TYPE_LABEL: Record<string, string> = {
  adverse: "不良反应工单",
  reship: "补发换货工单",
  refund: "线下打款工单",
  logistics: "物流工单",
  aftersale: "售后退货工单",
};

/** 图片类型 → 中文标签。 */
const IMAGE_TYPE_LABEL: Record<string, string> = {
  damaged: "破损",
  wrong: "错发",
  effect: "使用效果",
  skin: "皮肤状况",
  packaging: "包装",
  other: "其它",
};

/**
 * 由会话全景推导「敏感标签」。
 *
 * 注意：这里是**派生展示**，不是判定。判定在服务端规则引擎里已经完成，
 * 本函数只是把已有的结构化事实翻译成客服好扫读的标签。
 */
export function deriveSensitiveTags(ctx: CSSessionContextDTO): string[] {
  const tags: string[] = [];
  if (ctx.overallRisk === "critical") tags.push("高危会话");
  else if (ctx.overallRisk === "high") tags.push("高风险会话");
  if (ctx.ticketType) tags.push(TICKET_TYPE_LABEL[ctx.ticketType] ?? ctx.ticketType);
  if (ctx.evidenceMismatch && parseJsonObjectArray(ctx.evidenceMismatch).length > 0) {
    tags.push("图文证据不一致");
  }
  if (ctx.buyerVisitCount >= 3) tags.push(`多次进线 ×${ctx.buyerVisitCount}`);
  return tags;
}

/** 图片类型 JSON → 中文标签数组。 */
export function describeImageTypes(ctx: CSSessionContextDTO): string[] {
  return parseJsonArray(ctx.imageTypes).map((t) => IMAGE_TYPE_LABEL[t] ?? t);
}
