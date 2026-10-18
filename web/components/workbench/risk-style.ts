/**
 * ============================================================================
 * [AgentDesk] 风险视觉映射表
 * ----------------------------------------------------------------------------
 * 单一数据源：所有风险相关的颜色 / 文案 / 图标都从这里取，避免散落在各组件里。
 * 色值全部走 globals.css 的语义 token，不写死 hex —— 这样深色模式与
 * 四套调色板自动跟随。
 *
 * 「不刺眼但一定被看到」的三个手段（竞品调研结论）：
 *   ① 高危不用大面积红填充 → 1px 描边 + 浅底 + 4px 左色条
 *   ② 顶部告警条，比侧栏更早进入视野
 *   ③ risk-pulse 脉冲动画（幅度极小）
 * ============================================================================
 */

import type {
  ConfidenceTier,
  EmotionPolarity,
  ReplyTone,
  RiskCategory,
  RiskLevel,
  RiskSource,
} from "./types";

/** 风险等级的视觉配置。 */
export const RISK_STYLE: Record<
  RiskLevel,
  {
    /** i18n key，由调用方 t() 翻译。 */
    labelKey: string;
    /** 兜底中文，i18n 未就绪时用。 */
    label: string;
    /** 文字颜色类。 */
    text: string;
    /** 圆点背景类。 */
    dot: string;
    /** 左侧色条类（贯穿行/卡）。 */
    bar: string;
    /** 浅色底类。 */
    bg: string;
    /** 描边类。 */
    border: string;
    /** 是否启用脉冲。 */
    pulse: boolean;
  }
> = {
  none: {
    labelKey: "csRisk.level.none",
    label: "无风险",
    text: "text-risk-none",
    dot: "bg-risk-none",
    bar: "bg-risk-none",
    bg: "bg-risk-none-bg",
    border: "border-risk-none/30",
    pulse: false,
  },
  low: {
    labelKey: "csRisk.level.low",
    label: "低",
    text: "text-risk-low",
    dot: "bg-risk-low",
    bar: "bg-risk-low",
    bg: "bg-risk-low-bg",
    border: "border-risk-low/30",
    pulse: false,
  },
  medium: {
    labelKey: "csRisk.level.medium",
    label: "中",
    text: "text-risk-medium",
    dot: "bg-risk-medium",
    bar: "bg-risk-medium",
    bg: "bg-risk-medium-bg",
    border: "border-risk-medium/40",
    pulse: false,
  },
  high: {
    labelKey: "csRisk.level.high",
    label: "高",
    text: "text-risk-high",
    dot: "bg-risk-high",
    bar: "bg-risk-high",
    bg: "bg-risk-high-bg",
    border: "border-risk-high/50",
    pulse: false,
  },
  critical: {
    labelKey: "csRisk.level.critical",
    label: "高危",
    text: "text-risk-critical",
    dot: "bg-risk-critical",
    bar: "bg-risk-critical",
    bg: "bg-risk-critical-bg",
    border: "border-risk-critical/60",
    pulse: true,
  },
};

/**
 * 风险类别 → 业务含义。
 * 分级原则：安全 > 舆情 > 合规 > 体验 > 时效。
 */
export const RISK_CATEGORY_META: Record<
  RiskCategory,
  { labelKey: string; label: string; /** 同类风险的最高可接受等级上限提示。 */ hint: string }
> = {
  safety: {
    labelKey: "csRisk.category.safety",
    label: "安全风险",
    hint: "涉及人身安全，最高优先级",
  },
  opinion: {
    labelKey: "csRisk.category.opinion",
    label: "舆情风险",
    hint: "投诉举报、平台曝光",
  },
  compliance: {
    labelKey: "csRisk.category.compliance",
    label: "合规风险",
    hint: "赔付承诺、监管口径",
  },
  experience: {
    labelKey: "csRisk.category.experience",
    label: "体验风险",
    hint: "服务过程不满",
  },
  timeliness: {
    labelKey: "csRisk.category.timeliness",
    label: "时效风险",
    hint: "超时未响应、催促",
  },
};

/** 情绪极性的视觉配置。 */
export const EMOTION_STYLE: Record<
  EmotionPolarity,
  { labelKey: string; label: string; text: string; bg: string; icon: string }
> = {
  negative: {
    labelKey: "csRisk.emotion.negative",
    label: "不满",
    text: "text-emotion-negative",
    bg: "bg-risk-critical-bg",
    icon: "😠",
  },
  neutral: {
    labelKey: "csRisk.emotion.neutral",
    label: "中性",
    text: "text-emotion-neutral",
    bg: "bg-risk-none-bg",
    icon: "😐",
  },
  positive: {
    labelKey: "csRisk.emotion.positive",
    label: "满意",
    text: "text-emotion-positive",
    bg: "bg-risk-low-bg",
    icon: "🙂",
  },
};

/**
 * AI 置信度三档。
 *
 * 设计决策：不使用百分比。
 * 未校准的百分比是负债 —— 业务评委看到「92.4%」第一反应是「你怎么校准的」。
 * 用「高 / 中 / 低」不会有这个问题。
 */
export const CONFIDENCE_STYLE: Record<
  ConfidenceTier,
  { labelKey: string; label: string; text: string; bg: string; /** 高置信度弱化为灰字。 */ muted: boolean }
> = {
  high: {
    labelKey: "csRisk.confidence.high",
    label: "高",
    text: "text-muted-foreground",
    bg: "bg-muted",
    muted: true,
  },
  medium: {
    labelKey: "csRisk.confidence.medium",
    label: "中",
    text: "text-risk-medium",
    bg: "bg-risk-medium-bg",
    muted: false,
  },
  low: {
    labelKey: "csRisk.confidence.low",
    label: "低",
    text: "text-confidence-low",
    bg: "bg-risk-critical-bg",
    muted: false,
  },
};

/** 回复语气档位。 */
export const REPLY_TONE_META: Record<ReplyTone, { labelKey: string; label: string }> = {
  professional: { labelKey: "csRisk.tone.professional", label: "专业" },
  warm: { labelKey: "csRisk.tone.warm", label: "亲切" },
  concise: { labelKey: "csRisk.tone.concise", label: "简洁" },
};

/**
 * 风险触发来源 → 展示元数据。
 *
 * 产品意义：客服看到「高危」时，必须能一眼分辨这是**规则算出来的**还是
 * **模型猜出来的**。前者可以直接采信并升级处理，后者需要人为复核。
 * 这也是答辩时最容易被追问的一点。
 */
export const RISK_SOURCE_META: Record<
  RiskSource,
  { labelKey: string; label: string; /** 是否确定性（可复现、可审计）。 */ deterministic: boolean }
> = {
  hard_word: { labelKey: "csRisk.source.hardWord", label: "硬风险词", deterministic: true },
  scene: { labelKey: "csRisk.source.scene", label: "场景标注", deterministic: true },
  ticket: { labelKey: "csRisk.source.ticket", label: "工单佐证", deterministic: true },
  model: { labelKey: "csRisk.source.model", label: "模型推断", deterministic: false },
};

/** 风险等级排序权重，用于取最高等级。 */
const RISK_ORDER: Record<RiskLevel, number> = {
  none: 0,
  low: 1,
  medium: 2,
  high: 3,
  critical: 4,
};

/** 从一组风险事件里取最高等级。 */
export function highestRiskLevel(levels: RiskLevel[]): RiskLevel {
  if (levels.length === 0) {
    return "none";
  }
  return levels.reduce((acc, cur) => (RISK_ORDER[cur] > RISK_ORDER[acc] ? cur : acc), "none");
}

/** 判断是否达到「需要立即处理」的门槛。 */
export function needsImmediateAttention(level: RiskLevel): boolean {
  return level === "high" || level === "critical";
}

/**
 * 风险分数 → 等级。
 *
 * ⚠️ 这里的阈值必须与后端 `cs_risk_service.go::scoreToLevel` **完全一致**：
 *     >=95 critical / >=70 high / >=50 medium / >0 low / else none
 * 任何一侧单独改动都会导致「客服看到高危、看板显示中等」的演示事故。
 * 改动时请同时改两处，并跑 Go 侧 TestScoreToLevel 与前端本函数的对照测试。
 */
export function scoreToRiskLevel(score: number): RiskLevel {
  if (score >= 95) return "critical";
  if (score >= 70) return "high";
  if (score >= 50) return "medium";
  if (score > 0) return "low";
  return "none";
}

/** 情绪强度 1-5 → 极性。 */
export function intensityToPolarity(intensity: number): EmotionPolarity {
  if (intensity <= 2) return "negative";
  if (intensity === 3) return "neutral";
  return "positive";
}
