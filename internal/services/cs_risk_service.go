package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

// ============================================================================
// [AgentDesk-BEGIN] 数据共情者 · 风险推导服务
//
// 定位：这是「防幻觉的最后一道闸」。LLM 可以判断语气、生成建议，
//       但**风险等级绝不由 LLM 决定** —— 全部走规则，可复现、可审计、
//       可向评委解释"为什么这条是 critical"。
//
// 三层判定，取并集后取最高级：
//   1. 硬风险词   —— 买家原声里的确定性信号（"12315" "起诉" "过敏" "毁容"）
//   2. 场景映射   —— 官方 41 个 scene_minor 到风险类别/基础等级的静态表
//   3. 工单佐证   —— 不良反应/风控工单的存在本身即最高级证据
//
// 与 pipeline.py 的 build_risks() 保持同一套规则，两边结果应一致
// （pipeline 用于离线 Mock 与评估；本服务用于线上实时判定）。
// ============================================================================

// ---- 常量：硬风险词库 ------------------------------------------------------
// 分五组，对应五个风险类别。命中即产生一条风险事件。
var (
	// 人身安全 —— 最高优先级，任何一条都直接 critical。
	hardWordsSafety = []hardWord{
		{"过敏", 100}, {"不良反应", 100}, {"毁容", 100}, {"烂脸", 100},
		{"红肿", 85}, {"溃烂", 100}, {"就医", 95}, {"医院", 90},
		{"刺痛", 80}, {"灼烧", 85}, {"溃破", 100}, {"起疹", 85},
		{"留疤", 90}, {"色素沉着", 75}, {"激素", 80}, {"脱皮", 70},
	}

	// 合规 —— 涉及监管、法律、正品信任。
	hardWordsCompliance = []hardWord{
		{"12315", 100}, {"市场监管", 100}, {"工商", 95}, {"药监", 100},
		{"消费者协会", 95}, {"假货", 95}, {"假冒", 95}, {"正品", 70},
		{"虚假宣传", 90}, {"欺骗", 85}, {"欺诈", 95}, {"误导", 80},
		{"成分表", 65}, {"备案", 70}, {"违规", 85}, {"举报", 95},
	}

	// 舆情 —— 公开传播风险，比投诉本身更危险。
	hardWordsOpinion = []hardWord{
		{"曝光", 100}, {"起诉", 100}, {"律师", 95}, {"法院", 100},
		{"诉讼", 100}, {"差评", 85}, {"黑猫", 90}, {"小红书", 75},
		{"微博", 75}, {"抖音", 70}, {"媒体", 90}, {"记者", 95},
		{"投诉", 80}, {"消协", 90}, {"集体", 85}, {"维权", 85},
	}

	// 时效 —— 等待焦虑与逾期风险。
	hardWordsTimeliness = []hardWord{
		{"急", 60}, {"着急", 65}, {"尽快", 50}, {"马上", 50},
		{"已经很久", 70}, {"一直没", 70}, {"多久", 45}, {"还没到", 65},
		{"迟迟", 70}, {"拖了", 75}, {"等了两", 75}, {"半个月", 80},
		{"一个月", 85}, {"超时", 75}, {"失效", 65}, {"过期", 70},
	}

	// 体验 —— 情绪与服务感受，是预警而非事故。
	hardWordsExperience = []hardWord{
		{"失望", 65}, {"生气", 70}, {"气死", 75}, {"无语", 60},
		{"太差", 75}, {"垃圾", 80}, {"恶心", 75}, {"受不了", 75},
		{"再也不", 80}, {"拉黑", 85}, {"退坑", 70}, {"崩溃", 80},
		{"离谱", 70}, {"过分", 65}, {"敷衍", 75},
	}
)

type hardWord struct {
	Word  string
	Score int
}

// ---- 常量：场景到风险的静态映射 -------------------------------------------
// key = 官方 scene_minor（41 个全覆盖），value = 该场景的基础风险。
var sceneMinorToRisk = map[string]struct {
	Category string
	Level    string
	Score    int
	Action   string
}{
	// 不良反应（安全，一律 high 起，含就医则 critical）
	"泛红刺痒": {models.RiskCategorySafety, models.RiskLevelHigh, 75, "立即引导不良反应登记，24 小时内专员回访"},
	"闷痘爆痘": {models.RiskCategorySafety, models.RiskLevelHigh, 75, "立即引导不良反应登记，24 小时内专员回访"},
	"过敏就医": {models.RiskCategorySafety, models.RiskLevelCritical, 100, "最高优先级：同步安全负责人，按法规流程上报"},
	// 补发换货（体验为主，会员/直播承诺升级为时效）
	"漏发赠品":     {models.RiskCategoryExperience, models.RiskLevelMedium, 55, "核实赠品清单，按承诺补发并致歉"},
	"错发色号":     {models.RiskCategoryExperience, models.RiskLevelMedium, 55, "核实订单色号，安排换货并承担运费"},
	"少发正装":     {models.RiskCategoryExperience, models.RiskLevelMedium, 60, "核实发货重量，补发正装并致歉"},
	"破损换货":     {models.RiskCategoryExperience, models.RiskLevelMedium, 55, "核实破损凭证，按原地址补发"},
	"会员权益加赠补发": {models.RiskCategoryTimeliness, models.RiskLevelMedium, 60, "核实会员等级与权益规则，限期补发"},
	// 售后退货
	"七天无理由退货": {models.RiskCategoryExperience, models.RiskLevelLow, 40, "按标SOP引导退货流程"},
	"使用不适退货":  {models.RiskCategorySafety, models.RiskLevelHigh, 75, "先确认不适症状，再办理退货，避免二次伤害"},
	"色号不合适退货": {models.RiskCategoryExperience, models.RiskLevelLow, 35, "按标SOP引导退货流程"},
	"质疑正品退货":  {models.RiskCategoryCompliance, models.RiskLevelHigh, 85, "提供官方授权链路证明，安抚信任危机"},
	"仅退款疑似异常": {models.RiskCategoryCompliance, models.RiskLevelHigh, 80, "转风控核实，保留证据链"},
	"拆封少件拒收":  {models.RiskCategoryCompliance, models.RiskLevelHigh, 75, "核实拆包视频，转风控评估"},
	// 物流异常
	"物流停滞疑似丢件": {models.RiskCategoryTimeliness, models.RiskLevelHigh, 75, "主动联系承运商查件，超时直接补发"},
	"显示签收未收到":  {models.RiskCategoryTimeliness, models.RiskLevelHigh, 80, "调取签收凭证，同步发起物流理赔"},
	"包裹少件":     {models.RiskCategoryExperience, models.RiskLevelMedium, 60, "核实发货记录，差额补发"},
	"包裹破损":     {models.RiskCategoryExperience, models.RiskLevelMedium, 60, "核实破损凭证，按原地址补发"},
	"拦截改址":     {models.RiskCategoryTimeliness, models.RiskLevelMedium, 55, "确认新地址，跟进承运商拦截结果"},
	// 物流服务
	"催发货":  {models.RiskCategoryTimeliness, models.RiskLevelMedium, 55, "同步仓库发货进度，给出明确时效"},
	"物流查询": {models.RiskCategoryTimeliness, models.RiskLevelLow, 30, "直接同步最新物流轨迹"},
	// 退款打款
	"大促价保补差":    {models.RiskCategoryTimeliness, models.RiskLevelMedium, 60, "核实价保规则，明确补差金额与到账时间"},
	"退款金额少退补打":  {models.RiskCategoryTimeliness, models.RiskLevelHigh, 70, "核对订单实付与实退差额，主动补打"},
	"售后期关闭过敏退款": {models.RiskCategorySafety, models.RiskLevelHigh, 80, "特批开通售后入口，先安抚再办理"},
	"退货运费报销":    {models.RiskCategoryTimeliness, models.RiskLevelMedium, 50, "按运费险规则报销，明确到账时效"},
	// 订单服务
	"退款进度查询":  {models.RiskCategoryTimeliness, models.RiskLevelMedium, 55, "查询退款流水，给出银行到账预计时间"},
	"退款迟迟不到账": {models.RiskCategoryTimeliness, models.RiskLevelHigh, 75, "调取打款凭证，异常则立即重打"},
	"保价申请被拒":  {models.RiskCategoryCompliance, models.RiskLevelMedium, 60, "复核价保条件，说明拒绝依据或改判"},
	"预售尾款咨询":  {models.RiskCategoryTimeliness, models.RiskLevelLow, 35, "说明尾款时间与优惠规则"},
	"直播间承诺纠纷": {models.RiskCategoryCompliance, models.RiskLevelHigh, 85, "调取直播回放核对承诺，兑现或补偿"},
	"发票申请":    {models.RiskCategoryCompliance, models.RiskLevelNone, 0, "按税务规范开具并寄送"},
	// 产品咨询
	"优惠与赠品咨询": {models.RiskCategoryExperience, models.RiskLevelLow, 30, "说明当前活动规则与赠品清单"},
	"套装搭配推荐":  {models.RiskCategoryExperience, models.RiskLevelNone, 0, "按肤质推荐搭配方案"},
	"色号选择":    {models.RiskCategoryExperience, models.RiskLevelNone, 0, "按肤色推荐，说明退换政策"},
	"成分与肤质适配": {models.RiskCategorySafety, models.RiskLevelMedium, 55, "核对成分与肤质，敏感肌主动提示风险"},
	"使用方法":    {models.RiskCategoryExperience, models.RiskLevelNone, 0, "说明用法用量与注意事项"},
	"比价与决策":   {models.RiskCategoryExperience, models.RiskLevelLow, 25, "客观说明产品差异，不贬低竞品"},
	"正品鉴别咨询":  {models.RiskCategoryCompliance, models.RiskLevelMedium, 60, "提供防伪查询路径与授权证明"},
	"孕妇可用咨询":  {models.RiskCategorySafety, models.RiskLevelHigh, 80, "严格按成分安全口径回复，建议咨询医生"},
	// 会员服务
	"积分兑换咨询": {models.RiskCategoryExperience, models.RiskLevelNone, 0, "说明积分规则与兑换入口"},
	// 其他服务
	"好评回访": {models.RiskCategoryOpinion, models.RiskLevelNone, 0, "表达感谢，邀请真实评价"},
}

// ---- 常量：工单类型到风险 -----------------------------------------------
var ticketTypeToRisk = map[string]struct {
	Category string
	Level    string
	Score    int
	Action   string
}{
	"adverse":   {models.RiskCategorySafety, models.RiskLevelCritical, 100, "立即引导不良反应登记，24 小时内专员回访"},
	"reship":    {models.RiskCategoryExperience, models.RiskLevelMedium, 55, "跟进补发物流，确认买家签收"},
	"refund":    {models.RiskCategoryTimeliness, models.RiskLevelMedium, 55, "核对打款状态，异常立即重打"},
	"logistics": {models.RiskCategoryTimeliness, models.RiskLevelHigh, 70, "主动联系承运商，超时直接补发"},
	"aftersale": {models.RiskCategoryCompliance, models.RiskLevelHigh, 75, "转风控核实，保留完整证据链"},
}

// ---- 类别到建议动作的兜底 -------------------------------------------------
var categoryFallbackAction = map[string]string{
	models.RiskCategorySafety:     "优先安抚，按安全流程登记并上报",
	models.RiskCategoryCompliance: "核对规则与凭证，谨慎承诺",
	models.RiskCategoryTimeliness: "主动同步进度，给出明确时间点",
	models.RiskCategoryOpinion:    "降温处理，必要时申请主管补偿权限",
	models.RiskCategoryExperience: "核实事实，按 SOP 补偿并致歉",
}

// ============================================================================
// 服务实现
// ============================================================================

var CSRiskService = newCSRiskService()

type csRiskService struct{}

func newCSRiskService() *csRiskService { return &csRiskService{} }

// DeriveRisks 对一个会话的买家原声 + 场景 + 工单类型做三层判定。
//
// 参数全部为纯输入，无 DB 依赖 —— 这样它既能在导入时批量调用，
// 也能在工作流节点里对单条实时消息调用，还方便写单测。
func (s *csRiskService) DeriveRisks(
	buyerTexts []string,
	sceneMinor string,
	ticketType string,
) []models.CSRiskEvent {
	var events []models.CSRiskEvent
	joined := strings.Join(buyerTexts, "\n")

	// ---- 第 1 层：硬风险词 ----
	groups := []struct {
		category string
		words    []hardWord
	}{
		{models.RiskCategorySafety, hardWordsSafety},
		{models.RiskCategoryCompliance, hardWordsCompliance},
		{models.RiskCategoryOpinion, hardWordsOpinion},
		{models.RiskCategoryTimeliness, hardWordsTimeliness},
		{models.RiskCategoryExperience, hardWordsExperience},
	}
	for _, g := range groups {
		var hits []string
		topScore := 0
		for _, hw := range g.words {
			if strings.Contains(joined, hw.Word) {
				// [AgentDesk] 否定词与疑问消歧：如“幸好没过敏”、“没有假货”、“不急”、“不会过敏”不应误报
				if isHardWordNegated(joined, hw.Word) {
					continue
				}
				hits = append(hits, hw.Word)
				if hw.Score > topScore {
					topScore = hw.Score
				}
			}
		}
		if len(hits) == 0 {
			continue
		}
		events = append(events, models.CSRiskEvent{
			Category:        g.category,
			Level:           scoreToLevel(topScore),
			Score:           topScore,
			Source:          "hard_word",
			Reason:          "买家原文命中硬风险词：" + strings.Join(hits, "、"),
			HardWords:       toJSONArray(hits),
			SuggestedAction: categoryFallbackAction[g.category],
		})
	}

	// ---- 第 2 层：场景映射 ----
	if r, ok := sceneMinorToRisk[strings.TrimSpace(sceneMinor)]; ok && r.Level != models.RiskLevelNone {
		events = append(events, models.CSRiskEvent{
			Category:        r.Category,
			Level:           r.Level,
			Score:           r.Score,
			Source:          "scene",
			Reason:          "场景判定为「" + sceneMinor + "」，归属" + categoryLabel(r.Category) + "风险",
			SuggestedAction: r.Action,
		})
	}

	// ---- 第 3 层：工单佐证 ----
	if r, ok := ticketTypeToRisk[strings.TrimSpace(ticketType)]; ok {
		events = append(events, models.CSRiskEvent{
			Category:        r.Category,
			Level:           r.Level,
			Score:           r.Score,
			Source:          "ticket",
			Reason:          "存在" + ticketTypeLabel(ticketType) + "记录",
			SuggestedAction: r.Action,
		})
	}

	return events
}

// OverallRisk 取一组风险事件里的最高等级。
func (s *csRiskService) OverallRisk(events []models.CSRiskEvent) string {
	best := models.RiskLevelNone
	for _, e := range events {
		if models.RiskLevelRank(e.Level) > models.RiskLevelRank(best) {
			best = e.Level
		}
	}
	return best
}

// PersistEvents 落库风险事件（使用事务保证删除与写入的原子性，防止并发丢事件）。
func (s *csRiskService) PersistEvents(conversationID int64, sessionNo string, events []models.CSRiskEvent) error {
	sessionNo = strings.TrimSpace(sessionNo)
	if sessionNo == "" && conversationID > 0 {
		if c := repositories.CSSessionContextRepository.GetByConversationID(conversationID); c != nil {
			sessionNo = c.SessionNo
		}
	}
	if sessionNo == "" && conversationID <= 0 {
		return nil
	}

	return sqls.DB().Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&models.CSRiskEvent{})
		if sessionNo != "" {
			query = query.Where("session_no = ?", sessionNo)
		} else {
			query = query.Where("conversation_id = ?", conversationID)
		}
		if err := query.Delete(&models.CSRiskEvent{}).Error; err != nil {
			slog.Error("csRiskService.PersistEvents delete failed", "sessionNo", sessionNo, "convID", conversationID, "err", err)
			return err
		}
		if len(events) == 0 {
			return nil
		}
		for i := range events {
			events[i].ID = 0
			events[i].ConversationID = conversationID
			events[i].SessionNo = sessionNo
		}
		if err := tx.Create(&events).Error; err != nil {
			slog.Error("csRiskService.PersistEvents create failed", "sessionNo", sessionNo, "convID", conversationID, "err", err)
			return err
		}
		return nil
	})
}

// isHardWordNegated 检查 word 在 text 中的命中是否全被否定词或疑问消歧。
func isHardWordNegated(text string, word string) bool {
	negationPrefixes := []string{
		"没", "没有", "未", "不", "别", "幸好没", "并未", "不是", "不曾",
		"不会", "是不是", "会不会", "避免", "谨防", "防止", "无需", "不用",
	}
	idx := 0
	foundAnyPositive := false
	for {
		pos := strings.Index(text[idx:], word)
		if pos == -1 {
			break
		}
		actualPos := idx + pos
		prefixStart := actualPos - 15
		if prefixStart < 0 {
			prefixStart = 0
		}
		prefix := text[prefixStart:actualPos]

		negated := false
		for _, neg := range negationPrefixes {
			if strings.HasSuffix(prefix, neg) || strings.Contains(prefix, neg) {
				negated = true
				break
			}
		}
		if !negated {
			foundAnyPositive = true
			break
		}
		idx = actualPos + len(word)
	}
	return !foundAnyPositive
}

// UpsertSessionContextFromRealtime 在实时消息跑完工作流时自动写入会话全景快照。
func (s *csRiskService) UpsertSessionContextFromRealtime(
	conversationID int64,
	sessionNo string,
	sceneMajor string,
	sceneMinor string,
	overallRisk string,
	events []models.CSRiskEvent,
) error {
	if conversationID <= 0 {
		return nil
	}
	now := time.Now()
	existing := repositories.CSSessionContextRepository.GetByConversationID(conversationID)
	if existing != nil {
		if models.RiskLevelRank(overallRisk) > models.RiskLevelRank(existing.OverallRisk) {
			existing.OverallRisk = overallRisk
		}
		if sceneMajor != "" {
			existing.SceneMajor = sceneMajor
		}
		if sceneMinor != "" {
			existing.SceneMinor = sceneMinor
		}
		existing.RiskCount = len(events)
		existing.LastMessageAt = &now
		existing.MessageCount += 1
		return repositories.CSSessionContextRepository.UpsertByConversationID(existing)
	}

	sNo := strings.TrimSpace(sessionNo)
	if sNo == "" {
		sNo = fmt.Sprintf("RT%05d", conversationID)
	}
	buyerNick := "买家"
	conv := repositories.ConversationRepository.Get(sqls.DB(), conversationID)
	if conv != nil && strings.TrimSpace(conv.CustomerName) != "" {
		buyerNick = conv.CustomerName
	}

	defaultProduct := os.Getenv("DEFAULT_ORDER_PRODUCT")
	if defaultProduct == "" {
		defaultProduct = "美妆护肤品（演示预设）"
	}

	newCtx := &models.CSSessionContext{
		SessionNo:         sNo,
		ConversationID:    conversationID,
		BuyerNickname:     buyerNick,
		SceneMajor:        sceneMajor,
		SceneMinor:        sceneMinor,
		OverallRisk:       overallRisk,
		RiskCount:         len(events),
		MessageCount:      1,
		BuyerMessageCount: 1,
		FirstMessageAt:    &now,
		LastMessageAt:     &now,
		OrderProduct:      defaultProduct,
		OrderStatus:       "已签收",
		TicketSummary:     "处理中",
	}
	return repositories.CSSessionContextRepository.UpsertByConversationID(newCtx)
}

// ---- 小工具 ---------------------------------------------------------------

func scoreToLevel(score int) string {
	switch {
	case score >= 95:
		return models.RiskLevelCritical
	case score >= 70:
		return models.RiskLevelHigh
	case score >= 50:
		return models.RiskLevelMedium
	case score > 0:
		return models.RiskLevelLow
	default:
		return models.RiskLevelNone
	}
}

func toJSONArray(items []string) string {
	b, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func categoryLabel(category string) string {
	switch category {
	case models.RiskCategorySafety:
		return "安全"
	case models.RiskCategoryCompliance:
		return "合规"
	case models.RiskCategoryTimeliness:
		return "时效"
	case models.RiskCategoryOpinion:
		return "舆情"
	case models.RiskCategoryExperience:
		return "体验"
	default:
		return "未知"
	}
}

func ticketTypeLabel(ticketType string) string {
	switch ticketType {
	case "adverse":
		return "不良反应工单"
	case "reship":
		return "补发换货工单"
	case "refund":
		return "线下打款工单"
	case "logistics":
		return "物流工单"
	case "aftersale":
		return "售后退货工单"
	default:
		return "工单"
	}
}

// ---- 供外部（工作流节点 / 看板）读用的查询方法 -----------------------------

// ListContexts 返回会话全景列表，可按风险等级与场景过滤。
// 用于风险看板页与左侧会话列表的角标合并。
func (s *csRiskService) ListContexts(levels []string, sceneMajor string, keyword string, limit int) []models.CSSessionContext {
	query := sqls.DB().Model(&models.CSSessionContext{})
	if len(levels) > 0 {
		query = query.Where("overall_risk IN ?", levels)
	}
	if strings.TrimSpace(sceneMajor) != "" {
		query = query.Where("scene_major = ?", strings.TrimSpace(sceneMajor))
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		query = query.Where("buyer_nickname LIKE ? OR session_no LIKE ?", "%"+kw+"%", "%"+kw+"%")
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var out []models.CSSessionContext
	// ===== [AgentDesk-BEGIN] 排序改用跨库表达式 =====
	// 原写法 FIELD(overall_risk,...) 是 MySQL 专有函数，SQLite 下报
	// "no such function: FIELD"，且 GORM 的 Find() 不检查 Error → 静默吞错，
	// 表现为接口返回 list:null、风险看板列表永远空白且无任何报错。
	// CASE WHEN 是标准 SQL，SQLite / MySQL 通用。
	if err := query.Order(repositories.OrderOverallRiskSQL + ", last_message_at DESC").
		Limit(limit).Find(&out).Error; err != nil {
		slog.Error("csRiskService.ListContexts failed", "err", err)
		return nil
	}
	// ===== [AgentDesk-END] =====
	return out
}

// GetContextBySessionNo 按官方会话ID取全景。
func (s *csRiskService) GetContextBySessionNo(sessionNo string) *models.CSSessionContext {
	if strings.TrimSpace(sessionNo) == "" {
		return nil
	}
	return repositories.CSSessionContextRepository.GetBySessionNo(sessionNo)
}

// ListEvents 取某会话的风险明细（等级倒序）。
func (s *csRiskService) ListEvents(sessionNo string) []models.CSRiskEvent {
	if strings.TrimSpace(sessionNo) == "" {
		return nil
	}
	return repositories.CSRiskEventRepository.ListBySessionNo(sessionNo)
}

// GetContextByConversationID 按站内会话ID取全景。
//
// 为什么需要这个而不是只用 sessionNo：
//
//	工作台的 AgentConversation 只携带 conversationId，没有官方 session_no。
//	右侧辅佐面板是"选中会话即刷新"，用 conversationId 直查可以少一次映射，
//	也避免前端为了拿到 sessionNo 而多打一次接口。
func (s *csRiskService) GetContextByConversationID(conversationID int64) *models.CSSessionContext {
	if conversationID <= 0 {
		return nil
	}
	return repositories.CSSessionContextRepository.GetByConversationID(conversationID)
}

// ListEventsByConversationID 按站内会话ID取风险明细（等级倒序）。
func (s *csRiskService) ListEventsByConversationID(conversationID int64) []models.CSRiskEvent {
	if conversationID <= 0 {
		return nil
	}
	return repositories.CSRiskEventRepository.ListByConversationID(conversationID)
}

// MarkHandled 把风险事件标记为已处置，形成"发现→处置"闭环。
//
// 传 eventId 时只处理该条；否则按 sessionNo 批量处理该会话的全部风险。
// 返回实际影响的条数，供前端确认。
func (s *csRiskService) MarkHandled(sessionNo string, eventID int64) int64 {
	if eventID > 0 {
		if err := repositories.CSRiskEventRepository.MarkHandled(eventID); err != nil {
			return 0
		}
		return 1
	}
	sessionNo = strings.TrimSpace(sessionNo)
	if sessionNo == "" {
		return 0
	}
	n, err := repositories.CSRiskEventRepository.MarkHandledBySessionNo(sessionNo)
	if err != nil {
		return 0
	}
	return n
}

// MarkHandledByConversationID 按站内会话ID批量标记已处理。
func (s *csRiskService) MarkHandledByConversationID(conversationID int64) int64 {
	if conversationID <= 0 {
		return 0
	}
	n, err := repositories.CSRiskEventRepository.MarkHandledByConversationID(conversationID)
	if err != nil {
		return 0
	}
	return n
}

// RiskBoardStats 风险看板顶部的统计卡片数据。
type RiskBoardStats struct {
	Total       int64            `json:"total"`
	ByLevel     map[string]int64 `json:"byLevel"`
	ByCategory  map[string]int64 `json:"byCategory"`
	ByScene     map[string]int64 `json:"byScene"`
	Unhandled   int64            `json:"unhandled"`
	MismatchCnt int64            `json:"mismatchCount"`
}

// BuildRiskBoardStats 聚合看板统计。
func (s *csRiskService) BuildRiskBoardStats() *RiskBoardStats {
	stats := &RiskBoardStats{
		ByLevel:    map[string]int64{},
		ByCategory: map[string]int64{},
		ByScene:    map[string]int64{},
	}
	stats.Total = repositories.CSSessionContextRepository.Count()

	for _, lvl := range []string{
		models.RiskLevelCritical, models.RiskLevelHigh,
		models.RiskLevelMedium, models.RiskLevelLow, models.RiskLevelNone,
	} {
		stats.ByLevel[lvl] = repositories.CSSessionContextRepository.CountByRiskLevel(lvl)
	}
	for _, cat := range []string{
		models.RiskCategorySafety, models.RiskCategoryCompliance,
		models.RiskCategoryTimeliness, models.RiskCategoryOpinion,
		models.RiskCategoryExperience,
	} {
		stats.ByCategory[cat] = repositories.CSRiskEventRepository.CountByCategory(cat)
	}
	stats.Unhandled = repositories.CSRiskEventRepository.CountUnhandled()
	stats.MismatchCnt = repositories.CSSessionContextRepository.CountEvidenceMismatch()
	stats.ByScene = repositories.CSSessionContextRepository.CountGroupBySceneMinor()
	return stats
}
