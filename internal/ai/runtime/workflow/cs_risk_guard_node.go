package workflow

import (
	"encoding/json"
	"strings"

	"agent-desk/internal/ai/workflow/dsl"
	"agent-desk/internal/models"
	"agent-desk/internal/services"
)

// ============================================================================
// [AgentDesk-BEGIN] 数据共情者 · 实时风险判定节点
//
// 挂载点：在 conversation_understanding 之后、reply_policy 之前。
//
//   start → conversation_understanding → [risk_guard] → reply_policy → ...
//
// 为什么要在 reply_policy 之前？
//   因为风险等级会改变回复策略：命中 critical 时不允许 AI 自由发挥，
//   必须走"安抚 + 转人工 + 登记"的固定路径。这就是产品的核心主张 ——
//   让"看见风险"发生在"生成回复"之前。
//
// 本节点**不调用 LLM**，纯规则。理由见 cs_risk_service.go 顶部注释。
// ============================================================================

// riskGuardResult 节点输出。
type riskGuardResult struct {
	Level       string
	Categories  []string
	MaxScore    int
	Reasons     []string
	Actions     []string
	HardWords   []string
	NeedHuman   bool
	NeedSafety  bool
	Summary     string
	SceneMajor  string  // 推断出的场景大类
	SceneMinor  string  // 推断出的场景子类
	SceneSource string  // rule / provided / none
	SceneConf   float64 // 场景置信度
}

// executeRiskGuard 实现 risk_guard 节点。
//
// 输入：
//
//	userMessage  —— 客户本轮原声（必填）
//	sceneMinor   —— 上游给定的子场景，可为空
//	ticketType   —— 关联工单类型，可为空
//	historyText  —— 历史买家消息拼接（用于跨轮次风险识别），可为空
//
// [AgentDesk-BEGIN] 场景自动推断
//
// 为什么必须有这一步：`DeriveRisks` 的第 2 层（41 类场景映射）依赖 sceneMinor，
// 而实时聊天时**官方数据里根本没有这个字段** —— 它只在离线导入时存在。
// 如果不推断，第 2 层在线上永远是空的，等于白写。
//
// 优先级：上游显式给定 > 规则分类器推断 > 空（跳过第 2 层）。
// 上游给定优先，是因为导入场景下官方标注一定比规则准。
func (e *Executor) executeRiskGuard(state *runState, node dsl.Node) error {
	userMessage := strings.TrimSpace(toString(state.resolveInput(node, "userMessage")))
	if userMessage == "" {
		userMessage = state.input.UserMessage.Content
	}
	sceneMinor := strings.TrimSpace(toString(state.resolveInput(node, "sceneMinor")))
	ticketType := strings.TrimSpace(toString(state.resolveInput(node, "ticketType")))
	historyText := strings.TrimSpace(toString(state.resolveInput(node, "historyText")))

	// 只对本轮消息 + 可选历史做判定。历史不参与硬词命中会漏跨轮积累
	// （如第 3 轮才说"那我要起诉你们"），所以要带上。
	texts := []string{userMessage}
	if historyText != "" {
		texts = append(texts, historyText)
	}

	// ---- 场景推断：上游没给就从买家原声里分类 ----
	sceneMajor := ""
	sceneSource := "none"
	sceneConf := 0.0
	if sceneMinor != "" {
		// 上游显式给定（离线导入场景），仅补大类
		sceneMajor = services.SceneMajorOf(sceneMinor)
		sceneSource = "provided"
		sceneConf = 1.0
	} else {
		cls := services.ClassifyScene(texts)
		if cls.Source == "rule" {
			sceneMinor = cls.SceneMinor
			sceneMajor = cls.SceneMajor
			sceneSource = "rule"
			sceneConf = cls.Confidence
		}
		// fallback 不采信 —— 宁可不判，也不要拿"使用方法"这种兜底
		// 去污染风险链路（它的等级是 none，不会误报，但也没意义）。
	}

	events := services.CSRiskService.DeriveRisks(texts, sceneMinor, ticketType)
	result := summarizeRiskEvents(events)
	result.SceneMajor = sceneMajor
	result.SceneMinor = sceneMinor
	result.SceneSource = sceneSource
	result.SceneConf = sceneConf

	// 把结构化结果写进节点变量，供下游 condition 节点分流。
	state.setNodeVars(node.ID, map[string]any{
		"riskLevel":      result.Level,
		"riskCategories": result.Categories,
		"riskScore":      result.MaxScore,
		"riskReasons":    result.Reasons,
		"riskActions":    result.Actions,
		"riskHardWords":  result.HardWords,
		"needHuman":      result.NeedHuman,
		"needSafetyFlow": result.NeedSafety,
		"riskSummary":    result.Summary,
		"sceneMajor":     result.SceneMajor,
		"sceneMinor":     result.SceneMinor,
		"sceneSource":    result.SceneSource,
		"sceneConf":      result.SceneConf,
	})

	// 风险事件落库与会话全景快照同步（会话已存在时）。
	if state.input.Conversation.ID > 0 {
		sessionNo := strings.TrimSpace(
			toString(state.resolveInput(node, "sessionNo")),
		)
		_ = services.CSRiskService.PersistEvents(state.input.Conversation.ID, sessionNo, events)
		_ = services.CSRiskService.UpsertSessionContextFromRealtime(
			state.input.Conversation.ID,
			sessionNo,
			result.SceneMajor,
			result.SceneMinor,
			result.Level,
			events,
		)
	}
	return nil
}

// summarizeRiskEvents 把风险事件列表压成节点可用的扁平输出。
func summarizeRiskEvents(events []models.CSRiskEvent) riskGuardResult {
	ret := riskGuardResult{Level: models.RiskLevelNone}
	if len(events) == 0 {
		ret.Summary = "未识别到风险信号"
		return ret
	}

	seenCat := map[string]bool{}
	seenReason := map[string]bool{}
	seenAction := map[string]bool{}
	seenWord := map[string]bool{}

	for _, ev := range events {
		if models.RiskLevelRank(ev.Level) > models.RiskLevelRank(ret.Level) {
			ret.Level = ev.Level
		}
		if ev.Score > ret.MaxScore {
			ret.MaxScore = ev.Score
		}
		if !seenCat[ev.Category] {
			seenCat[ev.Category] = true
			ret.Categories = append(ret.Categories, ev.Category)
		}
		if !seenReason[ev.Reason] && ev.Reason != "" {
			seenReason[ev.Reason] = true
			ret.Reasons = append(ret.Reasons, ev.Reason)
		}
		if !seenAction[ev.SuggestedAction] && ev.SuggestedAction != "" {
			seenAction[ev.SuggestedAction] = true
			ret.Actions = append(ret.Actions, ev.SuggestedAction)
		}
		for _, w := range parseJSONArray(ev.HardWords) {
			if !seenWord[w] {
				seenWord[w] = true
				ret.HardWords = append(ret.HardWords, w)
			}
		}
		if ev.Category == models.RiskCategorySafety {
			ret.NeedSafety = true
		}
	}

	// 转人工阈值：high 及以上一律转人工。
	// 理由：high 已经代表"体验受损或合规敏感"，AI 无权承诺赔付；
	// 让 AI 在这种场景继续对话，风险远大于收益。
	ret.NeedHuman = models.RiskLevelRank(ret.Level) >= models.RiskLevelRank(models.RiskLevelHigh)

	ret.Summary = buildRiskSummary(ret)
	return ret
}

func buildRiskSummary(r riskGuardResult) string {
	if r.Level == models.RiskLevelNone {
		return "未识别到风险信号"
	}
	var b strings.Builder
	b.WriteString(riskLevelLabel(r.Level))
	if len(r.Categories) > 0 {
		b.WriteString(" · ")
		b.WriteString(strings.Join(r.Categories, "/"))
	}
	if len(r.HardWords) > 0 {
		b.WriteString(" · 命中词：")
		b.WriteString(strings.Join(limitSlice(r.HardWords, 5), "、"))
	}
	return b.String()
}

func riskLevelLabel(level string) string {
	switch level {
	case models.RiskLevelCritical:
		return "危急风险"
	case models.RiskLevelHigh:
		return "高风险"
	case models.RiskLevelMedium:
		return "中风险"
	case models.RiskLevelLow:
		return "低风险"
	default:
		return "无风险"
	}
}

func parseJSONArray(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func limitSlice(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

// ---------------------------------------------------------------------------
// 关于节点元数据
//
// 本文件只放**执行逻辑**。节点的 InputSchema / OutputSchema 定义在
//   internal/ai/workflow/registry/cs_risk_guard_spec.go
//
// 原因：registry 里的辅助函数（requiredInput / output / enumOutput /
// valueOption）都是**包内未导出函数**，从 runtime/workflow 包无法调用。
// 两个包之间只通过 registry.NodeTypeRiskGuard 这一个常量解耦，
// 互不 import，因此不存在循环依赖。
// ---------------------------------------------------------------------------
