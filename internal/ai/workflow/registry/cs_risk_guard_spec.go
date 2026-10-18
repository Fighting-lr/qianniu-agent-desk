package registry

// ============================================================================
// [AgentDesk-BEGIN] 数据共情者 · risk_guard 节点元数据
//
// 为什么单独一个文件？
//   registry 里的 output/enumOutput/valueOption 等辅助函数是包内未导出的，
//   把 spec 写在 registry 包内才能复用它们；而执行逻辑在 runtime/workflow 包。
//   两边通过 NodeTypeRiskGuard 常量解耦，互不 import，避免循环依赖。
// ============================================================================

// RiskGuardNodeSpec 返回 risk_guard 节点的元数据定义。
// 在 DefaultRegistry() 里用一行追加即可完成注册。
func RiskGuardNodeSpec() NodeSpec {
	return NodeSpec{
		Type:        NodeTypeRiskGuard,
		Title:       "Risk Guard",
		Description: "Rule-based risk detection that runs before reply generation. Never calls an LLM, so results stay reproducible and auditable.",
		Icon:        "ShieldAlertIcon",
		RiskLevel:   NodeRiskLevelLow,
		InputSchema: []VariableSpec{
			requiredInput("userMessage", "用户消息", VariableTypeString, "客户本轮发送的原始消息内容。"),
			optionalInput("sceneMinor", "子场景", VariableTypeString, "上游判定的业务子场景，如「泛红刺痒」。"),
			optionalInput("ticketType", "工单类型", VariableTypeString, "关联工单类型：adverse/reship/refund/logistics/aftersale。"),
			optionalInput("historyText", "历史消息", VariableTypeString, "历史买家消息拼接，用于识别跨轮次累积风险。"),
			optionalInput("sessionNo", "会话编号", VariableTypeString, "外部会话编号，用于风险事件落库。"),
		},
		OutputSchema: []VariableSpec{
			enumOutput("riskLevel", "风险等级", "规则引擎判定的最高风险等级。", []VariableValueOption{
				valueOption("critical", "危急", "人身安全、监管或舆情风险，必须立即介入。"),
				valueOption("high", "高", "体验受损或合规敏感，需转人工。"),
				valueOption("medium", "中", "涉及金额或时效承诺，需留痕处理。"),
				valueOption("low", "低", "情绪偏负但无实质损失。"),
				valueOption("none", "无", "常规咨询，AI 可正常处理。"),
			}),
			output("riskCategories", "风险类别", VariableTypeStringArray, "命中的风险类别列表。"),
			output("riskScore", "风险分值", VariableTypeInteger, "0-100 的风险分值。"),
			output("riskReasons", "判定理由", VariableTypeStringArray, "可向客服解释的判定依据。"),
			output("riskActions", "处置建议", VariableTypeStringArray, "建议客服采取的动作。"),
			output("riskHardWords", "命中风险词", VariableTypeStringArray, "买家原文里命中的硬风险词。"),
			output("needHuman", "需转人工", VariableTypeBoolean, "high 及以上为 true。"),
			output("needSafetyFlow", "需安全流程", VariableTypeBoolean, "是否命中安全类风险。"),
			output("riskSummary", "风险摘要", VariableTypeString, "一句话风险概要，可直接展示在会话列表角标。"),
		},
	}
}
