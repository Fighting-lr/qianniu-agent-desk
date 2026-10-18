/**
 * ============================================================================
 * [AgentDesk] 业务类型定义
 * ----------------------------------------------------------------------------
 * 与后端 cs_session_context / cs_risk_event / cs_reply_suggestion 表结构对齐。
 * 全部为前端展示所需的视图模型，不直接映射数据库行。
 * ============================================================================
 */

/** 风险等级 —— 5 档。与 globals.css 的 --risk-* token 一一对应。 */
export type RiskLevel = "none" | "low" | "medium" | "high" | "critical";

/** 风险类别 —— 五类，与后端 models.RiskCategory* 常量一一对应。 */
export type RiskCategory = "safety" | "opinion" | "experience" | "compliance" | "timeliness";

/**
 * 风险事件的触发来源。
 * 这个字段是「可信 AI」叙事的关键：规则引擎与模型输出必须能被肉眼分开。
 *   · hard_word —— 命中硬风险词库（确定性，可复现）
 *   · scene     —— 官方 41 类 scene_minor 映射（确定性）
 *   · ticket    —— 工单类型佐证（确定性）
 *   · model     —— 预留：模型推断（当前链路不产生）
 */
export type RiskSource = "hard_word" | "scene" | "ticket" | "model";

/** 情绪极性 —— 压成 3 档（正/中/负），比 5 档更好读、更好做色。 */
export type EmotionPolarity = "positive" | "neutral" | "negative";

/** AI 置信度 —— 3 档，不用百分比。 */
export type ConfidenceTier = "high" | "medium" | "low";

/** 回复语气档位 —— 三档候选回复。 */
export type ReplyTone = "professional" | "warm" | "concise";

/**
 * 单条风险事件。
 * 来源：后端 `cs_risk_event` 表 / S2 风险 Agent 输出。
 */
export type RiskEvent = {
  id: number;
  conversationId: number;
  category: RiskCategory;
  level: RiskLevel;
  /** 0-100 分。 */
  score: number;
  /** 触发依据（为什么判成这个等级）。 */
  reason: string;
  /**
   * 触发来源。规则引擎三层各自填值，前端据此区分「确定性结论」与「模型推断」。
   * 缺省时按 hard_word 处理（兼容早期数据）。
   */
  source?: RiskSource;
  /** 佐证引用 —— 工单号 / 订单号 / 会话片段，用于向客服和评委逐条解释。 */
  evidenceRef?: string;
  /** 命中的硬风险词（规则引擎产出，非模型）。 */
  hardWords?: string[];
  /** 建议动作。 */
  suggestedAction?: string;
  /** 是否已由人工处理。 */
  handled?: boolean;
  detectedAt: string;
};

/**
 * 会话理解结果（S1 输出）。
 * 来源：后端 `cs_session_profile` 表。
 */
export type SessionInsight = {
  conversationId: number;
  /** 意图 —— major 对应官方 scene_major。 */
  intentMajor: string;
  /** 意图 —— minor 对应官方 scene_minor（41 类）。 */
  intentMinor: string;
  /** 意图置信度档位。 */
  intentConfidence: ConfidenceTier;
  /** 置信度原因（低置信度时必须给出口）。 */
  confidenceReason?: string;
  /** 情绪极性。 */
  emotion: EmotionPolarity;
  /** 情绪强度 1-5。 */
  emotionIntensity: number;
  /** 买家真实诉求。 */
  buyerNeed: string;

  /** 关键实体。 */
  entities: {
    orderNo?: string;
    ticketNo?: string;
    logisticsNo?: string;
    product?: string;
    amount?: number;
  };

  /** 风险事件列表。 */
  risks: RiskEvent[];

  /** 综合风险等级（取 risks 里最高的）。 */
  overallRisk: RiskLevel;

  /** 图文证据不一致预警（多模态核验）。 */
  evidenceMismatch?: string;

  /** 单会话 token 成本（成本优化加分项的数据来源）。 */
  tokenCost?: {
    promptTokens: number;
    completionTokens: number;
    /** 毫秒。 */
    latencyMs: number;
  };
};

/** 回复建议（S3 输出）。 */
export type ReplySuggestion = {
  id: number;
  conversationId: number;
  tone: ReplyTone;
  text: string;
  /** 引用的知识库条款号。 */
  kbClauseIds?: number[];
  /** 是否被客服采用。 */
  adopted?: boolean;
  /** 置信度档位。 */
  confidence?: "high" | "medium" | "low";
  /** 推荐依据。 */
  rationale?: string;
};

/** 买家全景（右侧 Tab1）。 */
export type CustomerPanorama = {
  customerId: number;
  name: string;
  /** 累计进线次数。 */
  visitCount: number;
  /** 累计实付金额。 */
  totalPaid: number;
  /** 敏感标签，如「过敏史」「已投诉」。 */
  sensitiveTags: string[];
  /** 上次进线结论 —— 这是「先看见这个人」的关键。 */
  lastConclusion?: string;
  lastVisitAt?: string;
  /** 关联订单。 */
  orders: Array<{
    orderNo: string;
    product: string;
    amount: number;
    status: string;
    createdAt: string;
  }>;
  /** 关联工单。 */
  tickets: Array<{
    ticketNo: string;
    type: string;
    status: string;
    createdAt: string;
    summary?: string;
    batchNo?: string;
  }>;
};

/** 风险看板的批次聚类行。 */
export type AdverseClusterRow = {
  product: string;
  batchNo: string | null;
  count: number;
  /** 批次号异常标记（如官方的「00」）。 */
  dataQualityFlag?: string;
};
