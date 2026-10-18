package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

type replyCacheEntry struct {
	result    *ReplySuggestionResult
	expiresAt time.Time
}

const (
	replyCacheTTL    = 30 * time.Minute
	replyCacheMaxCap = 500 // 最大缓存条数限制，防止无界占用内存
)

var (
	replyCacheLock sync.RWMutex
	replyCacheMap  = make(map[string]replyCacheEntry)
)

func getReplyCache(key string) (*ReplySuggestionResult, bool) {
	replyCacheLock.RLock()
	entry, ok := replyCacheMap[key]
	replyCacheLock.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		replyCacheLock.Lock()
		delete(replyCacheMap, key)
		replyCacheLock.Unlock()
		return nil, false
	}
	return entry.result, true
}

func setReplyCache(key string, result *ReplySuggestionResult) {
	replyCacheLock.Lock()
	defer replyCacheLock.Unlock()

	if len(replyCacheMap) >= replyCacheMaxCap {
		now := time.Now()
		for k, v := range replyCacheMap {
			if now.After(v.expiresAt) {
				delete(replyCacheMap, k)
			}
		}
		if len(replyCacheMap) >= replyCacheMaxCap {
			for k := range replyCacheMap {
				delete(replyCacheMap, k)
				break
			}
		}
	}

	replyCacheMap[key] = replyCacheEntry{
		result:    result,
		expiresAt: time.Now().Add(replyCacheTTL),
	}
}

// ============================================================================
// [AgentDesk] 数据共情者 · AI 回复建议生成
//
// 这是「右侧辅佐面板」里唯一真正调用大模型的能力 —— 之前的实现是
// 前端写死的空数组（`suggestions={[]}`），演示时看不出任何 AI 成分。
//
// 产品原则（与前端 ReplySuggestionCard 的设计取向严格对齐）：
//
//  1. 输出 2-3 档【语气】候选，而不是一条"标准答案" —— 判断权留给客服。
//  2. 每档都带置信度与依据，让建议可被质疑，而不是黑箱。
//  3. ★ critical / high 会话【不生成自由回复】。
//     安全类场景让模型即兴发挥是产品风险，不是能力。
//     这类会话改为输出「处置要点」——给客服看的内部指引，不是发给买家的话术。
//
// 语气枚举与前端保持一致：professional(专业) / warm(亲切) / concise(简洁)
// ============================================================================

// ReplySuggestion 单条回复建议。字段名与前端 web/components/workbench/types.ts 对齐。
type ReplySuggestion struct {
	Tone       string `json:"tone"`       // professional | warm | concise
	Text       string `json:"text"`       // 建议正文
	Confidence string `json:"confidence"` // high | medium | low
	Rationale  string `json:"rationale"`  // 一句话依据
}

// ReplyEvidence 证据/指引条目。
type ReplyEvidence struct {
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

// ReplySuggestionResult 生成结果。
type ReplySuggestionResult struct {
	ConversationID int64             `json:"conversationId"`
	OverallRisk    string            `json:"overallRisk"`
	Locked         bool              `json:"locked"`      // 高危锁定：不给自由话术
	Suggestions    []ReplySuggestion `json:"suggestions"` // 锁定态为空
	Evidence       []ReplyEvidence   `json:"evidence"`    // 锁定态承载 AI 处置要点
	ModelName      string            `json:"modelName"`
	ElapsedMS      int64             `json:"elapsedMs"`
	Fallback       bool              `json:"fallback"` // true = 模型不可用，走了规则兜底
	DegradedReason string            `json:"degradedReason,omitempty"`
}

type csReplyService struct{}

// CSReplyService 生成式建议服务。命名与 CSRiskService 保持一致的包级单例风格。
var CSReplyService = &csReplyService{}

const (
	replyToneProfessional = "professional"
	replyToneWarm         = "warm"
	replyToneConcise      = "concise"

	// 历史消息条数上限。太多会稀释信号、推高成本；5-12 条足以覆盖一次售前售后对话。
	replyHistoryLimit = 12
)

// Generate 为指定会话生成回复建议。
//
// 设计取舍：模型不可用时【不报错】，而是返回规则兜底结果并标记 fallback。
// 理由：这是客服工作台的辅助面板，宁可降级也不能让现场弹错误、白屏。
func (s *csReplyService) Generate(ctx context.Context, conversationID int64) (*ReplySuggestionResult, error) {
	if conversationID <= 0 {
		return nil, errorsx.InvalidParamI18n("error.e0064")
	}

	started := time.Now()
	overview := CSRiskService.GetContextByConversationID(conversationID)
	events := CSRiskService.ListEventsByConversationID(conversationID)

	risk := "none"
	if overview != nil && strings.TrimSpace(overview.OverallRisk) != "" {
		risk = strings.ToLower(strings.TrimSpace(overview.OverallRisk))
	}
	locked := risk == "critical" || risk == "high"

	result := &ReplySuggestionResult{
		ConversationID: conversationID,
		OverallRisk:    risk,
		Locked:         locked,
	}

	// ---- 取证：会话历史 + 风险命中点 ----
	history, lastMsgID := s.loadHistory(conversationID)
	hitWords := collectHitWords(events)
	sceneMajor, sceneMinor, buyerNick := "", "", ""
	if overview != nil {
		sceneMajor = overview.SceneMajor
		sceneMinor = overview.SceneMinor
		buyerNick = overview.BuyerNickname
	}

	// 缓存键：按 (会话ID, 最后消息ID, 风险等级) 缓存，带 TTL 自动过期与容量保护
	cacheKey := fmt.Sprintf("%d_%d_%s", conversationID, lastMsgID, risk)
	if cached, ok := getReplyCache(cacheKey); ok {
		return cached, nil
	}

	// ★ 已关闭的会话不生成建议。
	if conversation := ConversationService.Get(conversationID); conversation != nil &&
		conversation.Status == enums.IMConversationStatusClosed {
		result.Locked = true
		result.Fallback = true
		result.DegradedReason = "会话已关闭，无需生成回复建议"
		result.Evidence = buildRuleEvidence(events, hitWords)
		return result, nil
	}

	// 没有任何对话内容时不必调模型
	if strings.TrimSpace(history) == "" {
		result.Fallback = true
		result.DegradedReason = "该会话没有可用于分析的买家消息"
		result.Evidence = buildRuleEvidence(events, hitWords)
		return result, nil
	}

	// ---- 调用模型 ----
	config, err := ai.GetEnabledAIConfig(enums.AIModelTypeLLM)
	if err != nil {
		result.Fallback = true
		result.DegradedReason = "未配置可用的 LLM（后台 → AI 配置里启用一条）"
		if locked {
			result.Suggestions = []ReplySuggestion{
				buildLockedWhitelistSuggestion(sceneMajor, sceneMinor, hitWords),
			}
			result.Evidence = append(result.Evidence,
				ReplyEvidence{Label: "AI 处置要点", Detail: "高危/危急会话锁定：立即阻断自由承诺，核实订单与客诉凭证并引导专员跟进"})
		}
		result.Evidence = append(result.Evidence, buildRuleEvidence(events, hitWords)...)
		return result, nil
	}
	result.ModelName = config.ModelName

	systemPrompt := buildReplySystemPrompt(locked)
	userPrompt := buildReplyUserPrompt(buildReplyPromptInput{
		BuyerNick:  buyerNick,
		Risk:       risk,
		SceneMajor: sceneMajor,
		SceneMinor: sceneMinor,
		HitWords:   hitWords,
		History:    history,
		Locked:     locked,
	})

	resp, err := ai.LLM.ChatWithConfig(ctx, *config, systemPrompt, userPrompt)
	result.ElapsedMS = time.Since(started).Milliseconds()
	if err != nil {
		// 降级而不是失败
		result.Fallback = true
		result.DegradedReason = "模型调用失败：" + trimErr(err)
		if locked {
			result.Suggestions = []ReplySuggestion{
				buildLockedWhitelistSuggestion(sceneMajor, sceneMinor, hitWords),
			}
			result.Evidence = append(result.Evidence,
				ReplyEvidence{Label: "AI 处置要点", Detail: "高危/危急会话锁定：立即阻断自由承诺，核实订单与客诉凭证并引导专员跟进"})
		}
		result.Evidence = append(result.Evidence, buildRuleEvidence(events, hitWords)...)
		return result, nil
	}

	parsed, perr := parseReplyJSON(resp.Content)
	if perr != nil {
		result.Fallback = true
		result.DegradedReason = "模型返回的内容不是合法 JSON：" + perr.Error()
		if locked {
			result.Suggestions = []ReplySuggestion{
				buildLockedWhitelistSuggestion(sceneMajor, sceneMinor, hitWords),
			}
			result.Evidence = append(result.Evidence,
				ReplyEvidence{Label: "AI 处置要点", Detail: "高危/危急会话锁定：立即阻断自由承诺，核实订单与客诉凭证并引导专员跟进"})
		}
		result.Evidence = append(result.Evidence, buildRuleEvidence(events, hitWords)...)
		return result, nil
	}

	if locked {
		// [AgentDesk] 高危合规锁定态：返回品牌法务审定白名单安抚话术，严格阻断自由生成防幻觉
		result.Suggestions = []ReplySuggestion{
			buildLockedWhitelistSuggestion(sceneMajor, sceneMinor, hitWords),
		}
		if hint := strings.TrimSpace(parsed.LockedHint); hint != "" {
			result.Evidence = append(result.Evidence,
				ReplyEvidence{Label: "AI 处置要点", Detail: hint})
		} else {
			result.Evidence = append(result.Evidence,
				ReplyEvidence{Label: "AI 处置要点", Detail: "高危/危急会话锁定：立即阻断自由承诺，核实订单与客诉凭证并引导专员跟进"})
		}
		result.Evidence = append(result.Evidence, buildRuleEvidence(events, hitWords)...)

		setReplyCache(cacheKey, result)
		return result, nil
	}

	suggestions := normalizeSuggestions(parsed.Suggestions)
	if len(suggestions) == 0 {
		result.Fallback = true
		result.DegradedReason = "模型未产出可用话术"
		result.Evidence = buildRuleEvidence(events, hitWords)
		return result, nil
	}
	result.Suggestions = suggestions
	result.Evidence = buildRuleEvidence(events, hitWords)

	setReplyCache(cacheKey, result)

	return result, nil
}

// buildLockedWhitelistSuggestion 构建高危锁定下的合规白名单安抚模板（安全/合规/舆情三类法务审定标准话术）
func buildLockedWhitelistSuggestion(sceneMajor, sceneMinor string, hitWords []string) ReplySuggestion {
	joined := strings.ToLower(sceneMajor + " " + sceneMinor + " " + strings.Join(hitWords, " "))
	text := "【合规白名单·法务审定安抚模板】亲亲，非常抱歉给您带来如此不愉快的体验，我们十分重视您的诉求。为了最高效妥善处理您的问题，本会话已由客服主管协同督办。请您先平复心情，我们绝不推诿责任，专员正在加急核实情况，必定在最短时间内给您一个满意的答复！"

	if strings.Contains(joined, "不良反应") || strings.Contains(joined, "泛红") || strings.Contains(joined, "刺痒") ||
		strings.Contains(joined, "红肿") || strings.Contains(joined, "医院") || strings.Contains(joined, "就医") ||
		strings.Contains(joined, "过敏") || strings.Contains(joined, "烂脸") {
		text = "【合规白名单·法务审定安抚模板】亲亲，非常抱歉给您带来不适体验！请您立即停用产品，并用温清水轻柔清洁面部。若伴有红肿刺痒，建议您优先前往正规医院皮肤科就诊，我们已为您开通绿色健康关怀通道，稍后会有不良反应专员联系您协助就医凭证登记与后续处理，请您千万不要慌张！"
	} else if strings.Contains(joined, "假货") || strings.Contains(joined, "假一") || strings.Contains(joined, "正品") ||
		strings.Contains(joined, "仿冒") || strings.Contains(joined, "报关") {
		text = "【合规白名单·法务审定安抚模板】亲亲，请您消消气！本店为品牌直营官方旗舰店，所有在售商品均有官方全链路授权与权威检验报告，支持防伪溯源。您的反馈我们高度重视，已将订单反馈至品控督办组，稍后将由资深主管为您提供全链路溯源凭证及一对一专项解答。"
	}

	return ReplySuggestion{
		Tone:       replyToneProfessional,
		Text:       text,
		Confidence: "high",
		Rationale:  "高危合规锁定：系统已阻断大模型自由生成，由品牌法务白名单标准模板兜底安抚",
	}
}

// loadHistory 取该会话最近的双向消息（客服与买家），拼成模型可读的上下文，并返回最新消息ID供缓存使用
func (s *csReplyService) loadHistory(conversationID int64) (string, int64) {
	cnd := sqls.NewCnd().
		Eq("conversation_id", conversationID).
		Asc("id")
	msgs := repositories.MessageRepository.Find(sqls.DB(), cnd)
	if len(msgs) > replyHistoryLimit {
		msgs = msgs[len(msgs)-replyHistoryLimit:]
	}

	var lastMsgID int64
	var sb strings.Builder
	for _, m := range msgs {
		if m.ID > lastMsgID {
			lastMsgID = m.ID
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		prefix := "[买家]"
		if m.SenderType == enums.IMSenderTypeAgent || m.SenderType == enums.IMSenderTypeSystem {
			prefix = "[客服]"
		}
		sb.WriteString(prefix)
		sb.WriteString(" ")
		sb.WriteString(content)
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String()), lastMsgID
}

func collectHitWords(events []models.CSRiskEvent) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range events {
		raw := strings.TrimSpace(e.HardWords)
		if raw == "" || raw == "null" {
			continue
		}
		var arr []string
		if err := json.Unmarshal([]byte(raw), &arr); err != nil {
			continue
		}
		for _, w := range arr {
			w = strings.TrimSpace(w)
			if w != "" && !seen[w] {
				seen[w] = true
				out = append(out, w)
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Prompt 构造
// ---------------------------------------------------------------------------

type buildReplyPromptInput struct {
	BuyerNick  string
	Risk       string
	SceneMajor string
	SceneMinor string
	HitWords   []string
	History    string
	Locked     bool
}

func buildReplySystemPrompt(locked bool) string {
	var sb strings.Builder
	sb.WriteString("你是电商客服团队的「回复建议助手」。你的输出会被客服参考后**手动改写并发送**给买家。\n\n")
	sb.WriteString("你只输出 JSON，不要 markdown 围栏，不要任何解释性文字。\n")

	if locked {
		sb.WriteString("\n【本次为高危会话】只输出处置要点，不输出可直接发送的话术。\n")
		return sb.String()
	}

	sb.WriteString(`
【硬性约束】
1. 不得编造事实：不知道订单号、金额、时间就笼统表述，绝不虚构。
2. 不得承诺具体赔偿金额、赔偿时限、责任认定。
3. 不得使用「保证」「一定」「绝对」「百分之百」等绝对化表述。
4. 每条话术 40~120 字，口语化，像真人客服在打字，不要书面腔、不要列表符号。
5. 三档语气各一条，必须真的换一种说法，不能只改开头。
`)

	if locked {
		sb.WriteString(`
【本次为高危会话的额外约束】
该会话已被系统判定为高风险，**不得给出可直接发送给买家的自由话术**。
只需输出一段「给客服看的处置要点」，包含：
  · 这个买家此刻最在意什么
  · 客服接下来该做的 2~3 个具体动作（如先安抚、引导留证、走哪个流程）
  · 哪些话绝对不能说
`)
	}
	return sb.String()
}

func buildReplyUserPrompt(in buildReplyPromptInput) string {
	var sb strings.Builder
	sb.WriteString("【会话信息】\n")
	sb.WriteString("- 买家昵称：" + orDash(in.BuyerNick) + "\n")
	sb.WriteString("- 综合风险等级：" + in.Risk + "\n")
	sb.WriteString("- 业务场景：" + orDash(in.SceneMajor) + " / " + orDash(in.SceneMinor) + "\n")
	if len(in.HitWords) > 0 {
		sb.WriteString("- 命中的风险点：" + strings.Join(in.HitWords, "、") + "\n")
	}
	sb.WriteString("\n【买家原话（按时间顺序）】\n")
	sb.WriteString(in.History)
	sb.WriteString("\n\n")

	if in.Locked {
		sb.WriteString(`请输出 JSON：
{"locked_hint":"给客服的处置要点，150 字以内","suggestions":[]}`)
	} else {
		sb.WriteString(`请输出 JSON：
{"suggestions":[
  {"tone":"professional","text":"...","confidence":"high","rationale":"依据一句话"},
  {"tone":"warm","text":"...","confidence":"medium","rationale":"..."},
  {"tone":"concise","text":"...","confidence":"medium","rationale":"..."}
]}`)
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// 解析与兜底
// ---------------------------------------------------------------------------

type replyJSON struct {
	LockedHint  string            `json:"locked_hint"`
	Suggestions []ReplySuggestion `json:"suggestions"`
}

// parseReplyJSON 宽容解析：模型偶尔会带 ```json 围栏或前后缀文字。
func parseReplyJSON(content string) (*replyJSON, error) {
	text := strings.TrimSpace(content)
	if text == "" {
		return nil, fmt.Errorf("内容为空")
	}
	text = stripCodeFence(text)
	// 截取第一个 { 到最后一个 }，容忍模型加了前后缀
	if i := strings.Index(text, "{"); i > 0 {
		text = text[i:]
	}
	if j := strings.LastIndex(text, "}"); j >= 0 && j < len(text)-1 {
		text = text[:j+1]
	}
	var out replyJSON
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}

// normalizeSuggestions 过滤非法语气、去空、限制档数，保证前端渲染不会踩空。
func normalizeSuggestions(in []ReplySuggestion) []ReplySuggestion {
	valid := map[string]bool{
		replyToneProfessional: true,
		replyToneWarm:         true,
		replyToneConcise:      true,
	}
	seen := map[string]bool{}
	out := make([]ReplySuggestion, 0, 3)
	for _, s := range in {
		tone := strings.ToLower(strings.TrimSpace(s.Tone))
		text := strings.TrimSpace(s.Text)
		if text == "" || seen[tone] {
			continue
		}
		if !valid[tone] {
			// 模型偶尔会自创语气名，映射到专业档而不是丢弃整条
			tone = replyToneProfessional
			if seen[tone] {
				continue
			}
		}
		conf := strings.ToLower(strings.TrimSpace(s.Confidence))
		if conf != "high" && conf != "medium" && conf != "low" {
			conf = "medium"
		}
		seen[tone] = true
		out = append(out, ReplySuggestion{
			Tone:       tone,
			Text:       text,
			Confidence: conf,
			Rationale:  strings.TrimSpace(s.Rationale),
		})
		if len(out) >= 3 {
			break
		}
	}
	return out
}

// buildRuleEvidence 规则兜底证据：模型不可用时，至少让客服看到"为什么被判定有风险"。
// 这是"降级但不空屏"的最后一道保险。
func buildRuleEvidence(events []models.CSRiskEvent, hitWords []string) []ReplyEvidence {
	var out []ReplyEvidence
	for _, e := range events {
		if len(out) >= 3 {
			break
		}
		detail := strings.TrimSpace(e.Reason)
		if detail == "" {
			detail = strings.TrimSpace(e.SuggestedAction)
		}
		label := categoryOrLevel(e)
		if detail == "" {
			continue
		}
		out = append(out, ReplyEvidence{Label: label, Detail: detail})
	}
	if len(hitWords) > 0 {
		out = append(out, ReplyEvidence{
			Label:  "命中风险词",
			Detail: strings.Join(hitWords, "、"),
		})
	}
	return out
}

func categoryOrLevel(e models.CSRiskEvent) string {
	if s := strings.TrimSpace(categoryLabel(string(e.Category))); s != "" && s != string(e.Category) {
		return s
	}
	return string(e.Level)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func trimErr(err error) string {
	s := err.Error()
	if len(s) > 160 {
		s = s[:160] + "..."
	}
	return s
}
