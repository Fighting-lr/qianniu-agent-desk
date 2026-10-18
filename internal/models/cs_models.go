package models

import "time"

// ============================================================================
// [AgentDesk-BEGIN] 数据共情者 · 风险与全景模型
//
// 本文件为「L'Oréal 2nd Beauty Tech Hackathon · 赛题1 数据共情者」参赛新增。
// 设计原则（见《架构与迭代维护规划.md》L4 层）：
//   1. 全新增文件，不改动 models.go 里任何既有结构体；
//   2. 只在 models.go 的 Models 数组里追加 2 行注册（唯一的上游改动）；
//   3. 表名前缀 cs_ 与其他业务表物理隔离，未来上游升级不会冲突。
//
// 两张表的分工：
//   cs_session_context —— 会话级全景。三向 join（会话ID/订单号/工单号）的
//                          结果 + 聚合后的最高风险。一条会话一行，供工作台
//                          右侧面板与左侧列表的角标直接读取。
//   cs_risk_event     —— 风险事件明细。一条会话可有多类风险（如既命中
//                          "安全"又命中"时效"），逐条落库，支撑风险看板的
//                          分类统计与钻取。
//
// ★ 血泪教训（2026-09-27）：结构体**必须带 json tag**。
//   Go 的 encoding/json 在字段没有 json tag 时，会直接使用**字段名原文**
//   （即 PascalCase：`SessionNo`），而前端 TypeScript 类型定义用的是
//   camelCase（`sessionNo`）。两边对不上，前端拿到的是 undefined ——
//   表现为**列表每行只有色条、文字全空白**，且接口返回 200、无任何报错。
//   上游 models.go 的既有结构体都带 json tag，本文件当初漏了。
//   新增字段时请照抄下面的写法：
//       FieldName T `gorm:"..." json:"fieldName"`
//   字段名映射规则：首字母小写；结尾的 ID 写成 Id（如 ConversationID → conversationId）。
// ============================================================================

// 风险等级常量。数值越大越严重，方便排序与取 max。
// 与前端 web/components/workbench/risk-style.ts 的 RiskLevel 严格对齐。
const (
	RiskLevelNone     = "none"     // 无风险：常规咨询
	RiskLevelLow      = "low"      // 低：情绪偏负或轻承诺，仅留痕不打扰
	RiskLevelMedium   = "medium"   // 中：涉及金额/时效承诺，需留痕
	RiskLevelHigh     = "high"     // 高：体验受损或合规敏感，需主管关注
	RiskLevelCritical = "critical" // 危急：人身安全 / 监管 / 舆情，必须立即介入
)

// 风险等级的语义口径（写下来是为了防止 41 个场景各自"拍脑袋"定级）：
//
//	none     —— 纯咨询，答完即结束，**不出现在风险看板上**
//	low      —— 需要留痕，但**不需要主动介入**（如"能开发票吗"）
//	medium   —— 有实质诉求或承诺义务，需在时限内跟进
//	high     —— 买家已有损失或品牌信任受损，需主管关注
//	critical —— 人身安全 / 监管介入 / 舆情扩散，必须立即上报
//
// ★ 血泪教训：`发票申请` 曾被定为 low + score 30，而 `大促价保补差` 定为
// medium + score 60 —— 同为"咨询类诉求"却一个不上板、一个上板。
// 场景等级是**业务判断**，不能靠零散赋值，必须对齐上面这张表。

// 风险类别常量。对应官方 10 大类场景的业务语义域。
const (
	RiskCategorySafety     = "safety"     // 安全：不良反应、过敏、就医
	RiskCategoryCompliance = "compliance" // 合规：正品质疑、虚假宣传、隐私
	RiskCategoryTimeliness = "timeliness" // 时效：物流停滞、退款不到账、催发货
	RiskCategoryOpinion    = "opinion"    // 舆情：差评、曝光、投诉升级、起诉
	RiskCategoryExperience = "experience" // 体验：色号错发、漏发、破损
)

// RiskLevelRank 把等级映射为可比较的整数，用于取 max 与排序。
func RiskLevelRank(level string) int {
	switch level {
	case RiskLevelCritical:
		return 5
	case RiskLevelHigh:
		return 4
	case RiskLevelMedium:
		return 3
	case RiskLevelLow:
		return 2
	case RiskLevelNone:
		return 1
	default:
		return 0
	}
}

// CSSessionContext 会话级全景上下文。
//
// 这是「数据共情者」的核心数据载体：把原本散落在聊天记录 / 订单 /
// 5 类工单里的信息，按会话聚合成一张"这个人此刻的处境"的快照，
// 让客服在点开会话之前就能看见风险。
type CSSessionContext struct {
	ID int64 `gorm:"primaryKey;autoIncrement" json:"id"`

	// —— 三方主键 ——
	SessionNo      string `gorm:"type:varchar(64);not null;default:'';index" json:"sessionNo"`  // 官方数据里的「会话ID」，如 S00010
	ConversationID int64  `gorm:"type:bigint;not null;default:0;index" json:"conversationId"`   // 对应 agent-desk 内部会话 ID（导入后回填）
	OrderNo        string `gorm:"type:varchar(64);not null;default:'';index" json:"orderNo"`    // 关联订单号
	TicketNo       string `gorm:"type:varchar(64);not null;default:'';index" json:"ticketNo"`   // 关联工单号
	TicketType     string `gorm:"type:varchar(32);not null;default:'';index" json:"ticketType"` // 工单类型：adverse/reship/refund/logistics/aftersale

	// [AgentDesk-BEGIN] —— 订单与工单业务详情（客户全景真实化展示）——
	OrderProduct  string  `gorm:"type:varchar(255);not null;default:''" json:"orderProduct"`  // 商品名称（如 "欧莱雅复颜玻尿酸安瓶面膜"）
	OrderAmount   float64 `gorm:"type:decimal(12,2);not null;default:0" json:"orderAmount"`   // 单笔订单金额
	OrderStatus   string  `gorm:"type:varchar(64);not null;default:''" json:"orderStatus"`    // 订单状态（如 "已签收" / "运输中"）
	TicketSummary string  `gorm:"type:varchar(255);not null;default:''" json:"ticketSummary"` // 工单状态与处置摘要
	BatchNo       string  `gorm:"type:varchar(64);not null;default:''" json:"batchNo"`       // 商品/不良反应批次号（如 "#00"）
	// [AgentDesk-END]

	// —— 买家身份 ——
	BuyerNickname string `gorm:"type:varchar(100);not null;default:'';index" json:"buyerNickname"` // 买家昵称（脱敏后，如 "喵e**"）

	// —— 场景分类：官方 10 大类 / 41 子类 ——
	SceneMajor string `gorm:"type:varchar(50);not null;default:'';index" json:"sceneMajor"` // 如「不良反应」
	SceneMinor string `gorm:"type:varchar(50);not null;default:'';index" json:"sceneMinor"` // 如「泛红刺痒」

	// —— 风险聚合（冗余，避免每次查询都聚合 cs_risk_event）——
	OverallRisk string `gorm:"type:varchar(20);not null;default:'none';index" json:"overallRisk"` // 该会话最高风险等级
	RiskCount   int    `gorm:"type:int;not null;default:0" json:"riskCount"`                      // 命中的风险类别数

	// —— 证据链（多模态差异化的落点）——
	ImageCount       int    `gorm:"type:int;not null;default:0" json:"imageCount"` // 买家上传的图片数
	ImageTypes       string `gorm:"type:text" json:"imageTypes"`                   // 图片类型 JSON 数组，如 ["产品整体","破损特写"]
	EvidenceMismatch string `gorm:"type:text" json:"evidenceMismatch"`             // 图文证据不一致的记录 JSON

	// —— 买家价值（「先看见这个人」的量化基础）——
	BuyerVisitCount int     `gorm:"type:int;not null;default:0" json:"buyerVisitCount"`          // 该买家历史会话数
	BuyerTotalPaid  float64 `gorm:"type:decimal(12,2);not null;default:0" json:"buyerTotalPaid"` // 该买家累计实付金额

	// —— 会话统计 ——
	MessageCount      int        `gorm:"type:int;not null;default:0" json:"messageCount"`
	BuyerMessageCount int        `gorm:"type:int;not null;default:0" json:"buyerMessageCount"`
	AgentMessageCount int        `gorm:"type:int;not null;default:0" json:"agentMessageCount"`
	FirstMessageAt    *time.Time `gorm:"type:datetime;index" json:"firstMessageAt"`
	LastMessageAt     *time.Time `gorm:"type:datetime;index" json:"lastMessageAt"`

	// —— 演示辅助 ——
	DemoPinned bool   `gorm:"not null;default:false;index" json:"demoPinned"`        // 是否在演示中置顶高亮
	DemoNote   string `gorm:"type:varchar(255);not null;default:''" json:"demoNote"` // 演示解说词

	AuditFields
}

// TableName 固定表名，避免 GORM 复数化带来的不确定性。
func (CSSessionContext) TableName() string { return "cs_session_context" }

// CSRiskEvent 风险事件明细。
//
// 一次会话可产生多条记录（不同类别 / 不同触发来源），
// 风险看板按 category 分组统计，右侧面板按 level 倒序列出。
type CSRiskEvent struct {
	ID int64 `gorm:"primaryKey;autoIncrement" json:"id"`

	ConversationID int64  `gorm:"type:bigint;not null;default:0;index" json:"conversationId"` // 关联会话
	SessionNo      string `gorm:"type:varchar(64);not null;default:'';index" json:"sessionNo"`

	Category string `gorm:"type:varchar(30);not null;default:'';index" json:"category"` // safety/compliance/timeliness/opinion/experience
	Level    string `gorm:"type:varchar(20);not null;default:'low';index" json:"level"` // none/low/medium/high/critical
	Score    int    `gorm:"type:int;not null;default:0" json:"score"`                   // 0-100，用于同级别内排序

	// —— 触发来源与依据（可解释性）——
	Source      string `gorm:"type:varchar(30);not null;default:'';index" json:"source"` // 触发来源：hard_word/scene/ticket/image
	Reason      string `gorm:"type:varchar(500);not null;default:''" json:"reason"`      // 人能读懂的判定理由
	HardWords   string `gorm:"type:text" json:"hardWords"`                               // 命中的硬风险词 JSON 数组
	EvidenceRef string `gorm:"type:varchar(255);not null;default:''" json:"evidenceRef"` // 证据引用（消息序号 / 图片路径 / 工单号）

	// —— 处置建议 ——
	SuggestedAction string `gorm:"type:varchar(255);not null;default:''" json:"suggestedAction"` // 建议客服采取的动作

	// —— 处置状态（闭环用）——
	Handled   bool       `gorm:"not null;default:false;index" json:"handled"` // 客服是否已处理
	HandledAt *time.Time `gorm:"type:datetime" json:"handledAt"`

	AuditFields
}

// TableName 固定表名。
func (CSRiskEvent) TableName() string { return "cs_risk_event" }
