package services

import (
	"sort"
	"strings"
)

// ============================================================================
// [AgentDesk-BEGIN] 数据共情者 · 41 类场景分类器
//
// 定位：这是 `DeriveRisks` 第 2 层（场景映射）的**前置输入**。
//       原本 sceneMinor 只能从官方数据里直接读到；客服实时聊天时没有这个字段，
//       必须从买家原声里**推断**出来，否则风险链路的第 2 层永远是空的。
//
// 设计原则（与 messageIntent 正交，互不干扰）：
//   - messageIntent（10 类）回答「买家在干什么」→ 决定回复策略
//     例：greeting / complaint / handoff_request / ticket_request
//   - sceneCategory（41 类）回答「问题属于什么业务场景」→ 决定风险判定
//     例：泛红刺痒 / 物流停滞疑似丢件 / 退款迟迟不到账
//
//   一条消息可以同时是「投诉」（意图）+「过敏就医」（场景），
//   两者各自驱动不同的下游逻辑，不能合并。
//
// 纯规则实现，**不调用 LLM** —— 保证可复现、可审计、可离线评估。
// 词表优先匹配长词（避免「过敏」抢走「过敏就医」的匹配）。
// ============================================================================

// SceneCategoryRule 一条场景匹配规则。
type SceneCategoryRule struct {
	SceneMajor string   // 官方 10 大类之一
	SceneMinor string   // 官方 41 子类之一
	Keywords   []string // 命中任一即认为属于该场景
	Excludes   []string // 命中任一则排除（用于消歧，如"退款"vs"仅退款疑似异常"）
	Priority   int      // 越大越优先；同分时按关键词长度降序
}

// sceneRiskFloor 返回「最不该被压过」的场景强度下限。
//
// ★ 为什么需要这条下限（血泪教训）：
// 初版打分是 `命中词长度和 × Priority/50`，两个量纲相乘后不可比 ——
// 「用了过敏了，我要退货」里 `用了过敏`(4字) 命中「使用不适退货」得 6 分，
// 而 `过敏`(2字) 命中「过敏就医」只有 4 分，**长词直接压过优先级**，
// 安全类场景被降级成普通退货。
//
// 修正思路：安全/合规类属于「宁可误报不可漏报」的场景，
// 命中即给一个**分数下限**，保证优先级语义不被词长稀释。
func sceneRiskFloor(sceneMinor string) int {
	switch sceneMinor {
	case "过敏就医":
		return 200 // 任何情况下都是最高优先，不允许被任何场景压过
	case "售后期关闭过敏退款":
		return 130 // 过敏 + 售后超期，双重叠加
	case "使用不适退货":
		return 120 // 过敏退货 ≠ 普通退货
	case "泛红刺痒", "闷痘爆痘":
		return 110 // 已发生不良反应，高于一般合规
	case "质疑正品退货":
		return 100
	case "仅退款疑似异常":
		return 95
	case "孕妇可用咨询":
		return 90 // 安全口径问答，错答后果严重
	case "物流停滞疑似丢件", "显示签收未收到":
		return 88
	case "直播间承诺纠纷":
		return 86
	}
	return 0
}

// SceneCategoryResult 分类结果。
type SceneCategoryResult struct {
	SceneMajor string   `json:"sceneMajor"`
	SceneMinor string   `json:"sceneMinor"`
	Matched    []string `json:"matched"`    // 命中的关键词
	Confidence float64  `json:"confidence"` // 0-1，基于命中数与关键词长度
	Source     string   `json:"source"`     // rule / fallback
}

// sceneCategoryRules 官方 41 子类 → 关键词规则。
//
// 顺序无关紧要（靠 Priority + 关键词长度决胜），但按 10 大类分组便于维护。
var sceneCategoryRules = []SceneCategoryRule{
	// ── 不良反应（官方 3 类，安全类，最高优先级）────────────────
	{SceneMajor: "不良反应", SceneMinor: "过敏就医", Priority: 100,
		Keywords: []string{"过敏", "就医", "去医院", "看医生", "医生", "医院", "急诊", "就诊", "就医记录"},
		// ★ 消歧方向要正确：是「过敏就医」退出鉴别/咨询/存疑语境，
		//   不是让「正品鉴别咨询」排除掉自己所有的真实语料。
		//   ★ 「是不是过敏了」「会不会过敏」是**怀疑**（官方归泛红刺痒等），
		//     只有「过敏了/去医院/就医」才是确诊。故排除疑问句式。
		Excludes: []string{"孕妇", "怀孕", "备孕", "哺乳期", "孕期",
			"专柜买", "味道不太一样", "味道不一样", "不会是假", "会不会是假",
			"是正品吗", "正品鉴别", "怎么验", "教教我",
			"是不是过敏", "会不会过敏", "是不是过", "会不会过"}},
	{SceneMajor: "不良反应", SceneMinor: "泛红刺痒", Priority: 80,
		Keywords: []string{"泛红", "刺痒", "发红", "刺痛", "痒", "红肿", "红血丝", "灼烧", "火辣辣", "起皮"}},
	{SceneMajor: "不良反应", SceneMinor: "闷痘爆痘", Priority: 80,
		Keywords: []string{"闷痘", "爆痘", "长痘", "起痘", "痘痘", "闭口", "粉刺"}},

	// ── 补发换货（官方 5 类）────────────────────────────────────
	{SceneMajor: "补发换货", SceneMinor: "会员权益加赠补发", Priority: 75,
		Keywords: []string{"会员", "加赠", "赠品没", "权益", "会员礼", "专属赠品"}},
	{SceneMajor: "补发换货", SceneMinor: "漏发赠品", Priority: 70,
		Keywords: []string{"赠品", "小样", "试用装", "没送", "漏发", "少发赠"}},
	// ★ 错发色号：真实语料是「我买的#01，发来个#05」「拍的是#01，收到咋是#05」
	//   必须比「色号不合适退货」优先，靠「买的是X发来Y」这种对照结构消歧。
	//   ★ 且要排掉「不适合我」——那是色号不合适退货的落点，不是发错货。
	//   ★ 注意：匹配用的是 strings.Contains（字面匹配），**不要写正则**。
	{SceneMajor: "补发换货", SceneMinor: "错发色号", Priority: 71,
		Keywords: []string{"错发", "发错", "色号不对", "颜色不对", "不是我买",
			"发来个", "发来的", "收到咋是", "拍的是", "我买的是", "收到的不是",
			"我要的是", "给我发成", "底标写的是", "仓库发货", "开什么玩笑",
			"开啥玩笑", "发货能走点心"},
		Excludes: []string{"想要", "想买", "打算买"}},
	{SceneMajor: "补发换货", SceneMinor: "少发正装", Priority: 70,
		Keywords: []string{"少发", "少件", "少了", "只收到一", "数量不对", "缺了", "两件咋只收"}},
	{SceneMajor: "补发换货", SceneMinor: "破损换货", Priority: 70,
		Keywords: []string{"破损", "碎了", "漏了", "洒了", "摔坏", "裂了", "外包装坏", "瓶子碎"}},

	// ── 售后退货（官方 6 类）────────────────────────────────────
	{SceneMajor: "售后退货", SceneMinor: "仅退款疑似异常", Priority: 90,
		Keywords: []string{"仅退款", "只退款", "不退货", "直接退钱", "光退款"}},
	// ★ 使用不适退货：真实语料是「用着不太舒服」「用着有点紧绷刺刺的」「用了两次感觉有点闷痘」
	//   ★ 不要放「不适合我」——那与「色号不合适退货」重叠，会把错发色号引到这里。
	//     本类的落点是**身体不适/用不惯**，不是颜色不合适。
	{SceneMajor: "售后退货", SceneMinor: "使用不适退货", Priority: 85,
		Keywords: []string{"用了不适", "用完难受", "用了过敏", "不敢用", "用了不舒服",
			"不太舒服", "用着不舒服", "感觉不舒服", "用不惯", "用完难受",
			"用着难受", "不想用了", "用着不太", "停用就好了", "敷上刺",
			"脸有点紧", "紧绷刺", "刺刺的"}},
	// ★ 质疑正品退货：**已认定是假并要求退**（结论语气）
	//   与「正品鉴别咨询」的区别：鉴别是「不会是假的吧」（疑问），
	//   这里是「假货 / 就是假的 / 仿的」（断言）。故排除鉴别类语气词。
	{SceneMajor: "售后退货", SceneMinor: "质疑正品退货", Priority: 88,
		Keywords: []string{"假货", "假冒", "不是正品", "怀疑正品", "假", "仿的", "非正品"},
		Excludes: []string{"不会是假", "会不会是假", "是正品吗", "怎么验", "教教我", "正品鉴别"}},
	{SceneMajor: "售后退货", SceneMinor: "拆封少件拒收", Priority: 82,
		Keywords: []string{"拒收", "拆封", "已拆", "开封", "少件拒收"}},
	// ★ 色号不合适退货：真实语料是「上脸有点显黑，想退了换#05」「涂上不好看」
	//   ★ 「显黑」要加"已到手"语境（到手了/上脸/涂上），否则会抢走
	//     「会不会显黑」（还没买，属色号选择）的咨询类语料。
	//   ★ Excludes 排掉"发错货"语境——那是错发色号，不是颜色不合适。
	{SceneMajor: "售后退货", SceneMinor: "色号不合适退货", Priority: 72,
		Keywords: []string{"色号不合适", "颜色不合适", "色差", "不合适想退", "涂上不好看",
			"到手了", "上脸有点", "显黑", "想退了换", "不适合我",
			"就试色了", "试色了一下"},
		Excludes: []string{"发来个", "发来的", "拍的是", "我买的是", "错发", "发错",
			"收到咋是", "底标写的是"}},
	{SceneMajor: "售后退货", SceneMinor: "七天无理由退货", Priority: 60,
		Keywords: []string{"七天无理由", "无理由退货", "想退货", "退货", "不想要了"}},

	// ── 物流异常（官方 5 类）────────────────────────────────────
	{SceneMajor: "物流异常", SceneMinor: "物流停滞疑似丢件", Priority: 88,
		Keywords: []string{"丢件", "丢了", "停滞", "一直不动", "没更新", "卡住", "物流停", "找不到包裹"}},
	{SceneMajor: "物流异常", SceneMinor: "显示签收未收到", Priority: 86,
		Keywords: []string{"显示签收", "已签收", "没收到", "没拿到", "放哪了", "代收点", "驿站没有"},
		// ★ 运费/退款语境下的「没收到」不是物流签收问题
		Excludes: []string{"运费", "邮费", "报销", "退款", "钱", "单号"}},
	// ★ 包裹破损：真实语料是「刚拆的包裹，面霜瓶子直接碎了，玻璃渣混着精华」
	//   与「破损换货」是**同一件事的两个视角**——买家报破损（物流异常），
	//   诉求是换货（补发换货）。官方标注归物流异常，故此处优先。
	{SceneMajor: "物流异常", SceneMinor: "包裹破损", Priority: 79,
		Keywords: []string{"包裹破损", "箱子破", "快递破损", "外箱坏", "压扁",
			"瓶子碎", "碎了", "玻璃渣", "这还咋用", "刚拆的包裹"},
		Excludes: []string{"换货", "补发"}},
	// ★ 包裹少件：真实语料是「包裹到了但里面少了洗发水，箱子也没破」
	//   与「少发正装」的区别：包裹少件说的是**收到的包裹内件数少了**，
	//   少发正装是**订单里少发了一件商品**。官方分属物流异常 / 补发换货。
	{SceneMajor: "物流异常", SceneMinor: "包裹少件", Priority: 77,
		Keywords: []string{"包裹少", "箱子里少", "收到少了", "件数不对",
			"里面少了", "包裹到了但", "少了洗发水", "少了眼影", "箱子也没破"},
		Excludes: []string{"赠品", "小样"}},
	{SceneMajor: "物流异常", SceneMinor: "拦截改址", Priority: 74,
		Keywords: []string{"改地址", "拦截", "换地址", "地址填错", "改收货"}},
	{SceneMajor: "物流服务", SceneMinor: "催发货", Priority: 68,
		Keywords: []string{"催", "什么时候发", "还没发", "多久发", "发货慢", "抓紧发", "快点发"}},
	// ★ 物流查询：曾试过把「单号」「快递」「物流」收窄成"纯查询运踪"表述，
	//   准确率虽涨 1.4pp，但**覆盖率掉 3 条、漏报从 4 涨到 5**，故回退。
	//   教训：漏报权重高于误报，覆盖率是硬约束，不能为了精确率牺牲召回。
	{SceneMajor: "物流服务", SceneMinor: "物流查询", Priority: 55,
		Keywords: []string{"物流", "快递", "到哪了", "运单", "单号", "什么时候到", "查快递"}},

	// ── 退款打款（官方 4 类）────────────────────────────────────
	{SceneMajor: "退款打款", SceneMinor: "退款金额少退补打", Priority: 80,
		Keywords: []string{"少退", "退少了", "金额不对", "少打", "补打", "钱少了"}},
	// ★ 保价申请被拒：真实语料是「我刚买没几天就降价40块，太坑了吧」
	//   —— **全文不含「保价」二字**，买家是在抱怨「刚买就降价且不给补」。
	//   与「大促价保补差」的区别：这里是**被拒**（已拿到拒绝结论），
	//   后者是**提出申请**（还在争取，如「刚买就降价了能保价吗」）。
	//   ★ 「刚买就降」本身只是陈述降价事实，两方都可能用；
	//     必须搭配"抱怨/被拒结果"词才归本类，否则会抢走「能保价吗」的申请语气。
	{SceneMajor: "订单服务", SceneMinor: "保价申请被拒", Priority: 78,
		Keywords: []string{"保价被拒", "价保被拒", "不给保价", "拒绝保价", "保价失败",
			"太坑了", "谁还敢原价买", "破规则", "摆明了坑", "坑老客户",
			"就差几天", "算了算了"},
		Excludes: []string{"能保价吗", "可以保价吗", "能不能保价", "怎么保价", "如何保价"}},
	{SceneMajor: "退款打款", SceneMinor: "大促价保补差", Priority: 76,
		Keywords: []string{"价保", "补差价", "降价", "保价", "买贵了", "差价"}},
	// ★ 退货运费报销：真实语料是「说好运费你们报销的，钱一直都没收到」
	//   「退货运费8.8元啥时候给我」。与「退款迟迟不到账」的区别：
	//   报销针对的是**退货运费**这一笔，不是商品货款。
	{SceneMajor: "退款打款", SceneMinor: "退货运费报销", Priority: 75,
		Keywords: []string{"运费", "邮费", "报销", "运费险", "快递费"}},

	// ── 订单服务 · 退款相关（官方 6 类中的 2 类）──────────────
	// ★ 退款迟迟不到账：真实语料是「说好的原路退回呢？都一个礼拜了一分没到」
	//   「退款显示成功五天了，我卡里根本没收到钱」
	//   注意：必须比「退款进度查询」优先，且要能吃下**不含"退款"二字**的说法
	//   （「都一个礼拜了一分没到」「钱到底去哪了」）。
	{SceneMajor: "订单服务", SceneMinor: "退款迟迟不到账", Priority: 83,
		Keywords: []string{"退款没到", "退款迟迟", "退款多久", "钱没到", "还没退", "退款慢", "一直没退款",
			"一分没到", "一个礼拜", "都一星期", "钱一直", "根本没收到", "退款去哪了",
			"退款到底", "别踢皮球", "显示成功", "卡里", "啥时候到", "到底啥时候"}},
	// ★ 售后期关闭过敏退款：真实语料是「退货你们仓库都签收了，但售后单关闭了钱退不出来」
	//   现有词表写的是「售后期关闭」，真实说法是「售后单关闭」，差一个字就全漏。
	{SceneMajor: "退款打款", SceneMinor: "售后期关闭过敏退款", Priority: 85,
		Keywords: []string{"售后期关闭", "超出售后", "售后关闭", "过期不能退",
			"售后单关闭", "售后单", "钱退不出来", "退不出来", "关闭了钱"}},
	{SceneMajor: "订单服务", SceneMinor: "退款进度查询", Priority: 60,
		Keywords: []string{"退款进度", "退款状态", "退款到哪", "退款"}},

	// ── 订单服务 · 承诺与预售（官方 6 类中的 2 类）────────────
	{SceneMajor: "订单服务", SceneMinor: "直播间承诺纠纷", Priority: 84,
		Keywords: []string{"直播间", "主播说", "直播说", "秒杀承诺", "直播承诺", "直播"}},
	{SceneMajor: "订单服务", SceneMinor: "预售尾款咨询", Priority: 70,
		Keywords: []string{"预售", "尾款", "定金", "预付款", "定金不退"}},

	// ── 产品咨询（官方 8 类）────────────────────────────────────
	{SceneMajor: "产品咨询", SceneMinor: "孕妇可用咨询", Priority: 66,
		Keywords: []string{"孕妇", "怀孕", "哺乳期", "孕期", "备孕"}},
	// ★ 正品鉴别咨询：真实语料是「和专柜买的味道不太一样，不会是假的吧」
	//   与「质疑正品退货」的区别：这里是**询问/鉴别**（还没下结论），
	//   后者是**已经认定是假并要退**。关键词「不会是假的吧」是疑问语气。
	//   ★ 曾因 Excludes 写过头（把「专柜买」「不会是假」都排除）导致 3 条全漏。
	//     消歧要**反向**做：由「质疑正品退货」排除疑问语气，而非本类排除自己。
	{SceneMajor: "产品咨询", SceneMinor: "正品鉴别咨询", Priority: 65,
		Keywords: []string{"怎么验真", "防伪", "查真伪", "正品鉴别", "真假", "怎么辨别",
			"味道不太一样", "味道不一样", "和专柜买", "专柜买的味道", "不会是假",
			"会不会是假", "是正品吗", "你们这是正品", "怎么验", "教教我", "验证一下",
			"旗舰店也有假", "显示正品"}},
	{SceneMajor: "产品咨询", SceneMinor: "成分与肤质适配", Priority: 62,
		Keywords: []string{"成分", "烟酰胺", "香精", "酒精", "敏感肌", "适合什么肤质", "肤质", "致痘成分"}},
	// ★ 色号选择：真实语料是「暖黄皮适合#05吗？会不会显黑」「我黄二白，选#01还是#05呀」
	//   特征是全国尚未下单的**疑问语气**（适合吗/会不会/还是/帮我看看）。
	{SceneMajor: "产品咨询", SceneMinor: "色号选择", Priority: 59,
		Keywords: []string{"选色号", "什么色号", "哪个色", "黄皮", "白皮", "显白", "色号推荐",
			"适合吗", "会不会显黑", "会不会拔干", "帮我看看", "还是", "纠结",
			"黄二白", "暖黄皮", "冷白皮", "来一支"}},
	{SceneMajor: "产品咨询", SceneMinor: "套装搭配推荐", Priority: 56,
		Keywords: []string{"搭配", "套装", "组合", "配一套", "怎么配"}},
	{SceneMajor: "产品咨询", SceneMinor: "使用方法", Priority: 54,
		Keywords: []string{"怎么用", "使用方法", "用法", "步骤", "一天几次", "早晚"}},
	// ★ 比价与决策：真实语料是「同样是精华，你们比隔壁贵一百，好在哪」
	//   「你家洁面乳和别家比贵不少啊，凭啥卖这个价」
	{SceneMajor: "产品咨询", SceneMinor: "比价与决策", Priority: 53,
		Keywords: []string{"哪个好", "推荐", "区别", "对比", "值得买", "选哪个", "哪个值",
			"比隔壁贵", "比别家贵", "贵一百", "贵不少", "凭什么卖", "凭啥卖",
			"好在哪", "好在哪儿", "划算", "值不值"}},
	{SceneMajor: "产品咨询", SceneMinor: "优惠与赠品咨询", Priority: 50,
		Keywords: []string{"优惠", "活动", "满减", "券", "折扣", "满赠", "有什么赠品"}},
	{SceneMajor: "会员服务", SceneMinor: "积分兑换咨询", Priority: 48,
		Keywords: []string{"积分", "兑换", "会员积分", "积分换"}},
	{SceneMajor: "订单服务", SceneMinor: "发票申请", Priority: 46,
		Keywords: []string{"发票", "开票", "报销凭证", "电子发票"}},

	// ── 其他服务（官方 1 类）────────────────────────────────────
	{SceneMajor: "其他服务", SceneMinor: "好评回访", Priority: 40,
		Keywords: []string{"好评", "评价", "晒图", "返现", "五星", "评价有礼"}},
}

// sceneMajorOf 由 sceneMinor 反查所属大类。
//
// ★ 单一真相源：一律查 `sceneMinorToMajor`（由官方 xlsx 生成），
//
//	不再从 sceneCategoryRules 里读 SceneMajor 字段。
//	原因：规则表的 SceneMajor 字段曾按「业务直觉」自行分组，
//	导致 41 条里 33 条大类归错，大类准确率只有 16.9%。
//	权威映射生成出来之后，规则表里的 SceneMajor 只作展示/调试用，
//	所有**对外输出**的大类都必须经这里查表。
func sceneMajorOf(sceneMinor string) string {
	if mj, ok := sceneMinorToMajor[strings.TrimSpace(sceneMinor)]; ok {
		return mj
	}
	return ""
}

// ClassifyScene 从买家原声推断官方 41 类场景。
//
// 匹配策略：
//  1. 先算每条规则的得分：命中关键词长度之和 × 该规则的 Priority 权重
//  2. 排除词命中则该规则直接出局
//  3. 安全/合规类场景再取一次 riskFloor 下限，防止被长词压过
//  4. 取最高分；完全相同则按 SceneMinor 字典序（保证结果稳定可复现）
//
// 返回结果永远有效：匹配不到时回落到「咨询 / 使用方法」，
// 由调用方决定是否采信（看 Source 字段）。
func ClassifyScene(buyerTexts []string) SceneCategoryResult {
	joined := strings.Join(buyerTexts, "\n")
	if strings.TrimSpace(joined) == "" {
		return SceneCategoryResult{
			SceneMajor: "咨询",
			SceneMinor: "使用方法",
			Confidence: 0,
			Source:     "fallback",
		}
	}

	type scored struct {
		rule  *SceneCategoryRule
		score int
		hits  []string
	}
	var candidates []scored

	for i := range sceneCategoryRules {
		r := &sceneCategoryRules[i]

		// 排除词优先
		excluded := false
		for _, ex := range r.Excludes {
			if ex != "" && strings.Contains(joined, ex) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		var hits []string
		score := 0
		for _, kw := range r.Keywords {
			if strings.Contains(joined, kw) {
				hits = append(hits, kw)
				// 长关键词更具判别力，按 rune 数加权
				score += len([]rune(kw))
			}
		}
		if len(hits) == 0 {
			continue
		}
		// Priority 作为权重放大：priority=100 时权重 2.0，priority=40 时约 1.4
		score = score * r.Priority / 50
		// 安全/合规类场景设下限，保证「宁可误报不可漏报」
		if floor := sceneRiskFloor(r.SceneMinor); floor > score {
			score = floor
		}
		candidates = append(candidates, scored{rule: r, score: score, hits: hits})
	}

	if len(candidates) == 0 {
		return SceneCategoryResult{
			SceneMajor: "咨询",
			SceneMinor: "使用方法",
			Confidence: 0.1,
			Source:     "fallback",
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].rule.SceneMinor < candidates[j].rule.SceneMinor
	})

	best := candidates[0]
	// 置信度：命中越多、分越高越可信；上限 0.95（规则法不给满分，留出 LLM 复核空间）
	conf := 0.5 + float64(len(best.hits))*0.1
	if conf > 0.95 {
		conf = 0.95
	}

	return SceneCategoryResult{
		SceneMajor: sceneMajorOf(best.rule.SceneMinor),
		SceneMinor: best.rule.SceneMinor,
		Matched:    best.hits,
		Confidence: conf,
		Source:     "rule",
	}
}

// SceneMajorOf 暴露给外部（handler / 测试）由子类查大类。
func SceneMajorOf(sceneMinor string) string {
	return sceneMajorOf(sceneMinor)
}

// AllSceneMinors 返回全部 41 个子类，供校验与前端下拉。
func AllSceneMinors() []string {
	seen := make(map[string]bool, len(sceneCategoryRules))
	out := make([]string, 0, 41)
	for _, r := range sceneCategoryRules {
		if seen[r.SceneMinor] {
			continue
		}
		seen[r.SceneMinor] = true
		out = append(out, r.SceneMinor)
	}
	return out
}

// [AgentDesk-END]
